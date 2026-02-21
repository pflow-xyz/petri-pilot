package zkode

import (
	"math/big"
)

// Heatmap scoring constants (fixed-point).
var (
	HeatmapBonus   = FixFromFloat(10.0) // Winning move bonus (dominates any base rate)
	HeatmapPenalty = FixFromFloat(1.5)  // Leaving opponent a win threat
)

// TTTHeatmapWitness contains all data for one TTT heatmap circuit assignment.
type TTTHeatmapWitness struct {
	PreState      *TTTODEState
	PostState     *TTTODEState
	StepSize      *big.Int
	HeatmapScores [9]*big.Int // Tactical scores per cell position
}

// ComputeTTTHeatmapStep runs one Tsit5 ODE step and computes tactical heatmap scores.
func ComputeTTTHeatmapStep(state *TTTODEState, h *big.Int) *TTTHeatmapWitness {
	// Compute all 34 transition rates
	var actualRates [TTTNumTransitions]*big.Int
	for t := 0; t < TTTNumTransitions; t++ {
		actualRates[t] = nativeMultiInputRate(state.Marking, t)
	}

	// Run ODE step
	postMarking := NativeTTTStep(state.Marking, h)
	postState := &TTTODEState{
		Marking: postMarking,
		Root:    ComputeRoot(postMarking[:]),
		Step:    state.Step + 1,
	}

	// Determine current player
	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)
	isXTurn := state.Marking[XTurn].Cmp(zero) > 0

	// Extract board state as boolean arrays
	var currentPiece [9]*big.Int
	var opponentPiece [9]*big.Int
	var cellEmpty [9]*big.Int
	for i := 0; i < 9; i++ {
		if isXTurn {
			currentPiece[i] = state.Marking[X00+i]
			opponentPiece[i] = state.Marking[O00+i]
		} else {
			currentPiece[i] = state.Marking[O00+i]
			opponentPiece[i] = state.Marking[X00+i]
		}
		cellEmpty[i] = state.Marking[P00+i]
	}

	// Compute heatmap scores for each cell
	var scores [9]*big.Int
	for i := 0; i < 9; i++ {
		// Base rate: select x_play or o_play rate based on turn
		var baseRate *big.Int
		if isXTurn {
			baseRate = new(big.Int).Set(actualRates[TXPlay00+i])
		} else {
			baseRate = new(big.Int).Set(actualRates[TOPlay00+i])
		}

		// Only score empty cells (occupied cells have rate 0 already)
		if cellEmpty[i].Cmp(zero) == 0 {
			scores[i] = new(big.Int).Set(zero)
			continue
		}

		// Win detection: does placing here complete 3-in-a-row for current player?
		winFlag := nativeWinFlag(i, currentPiece[:], one)

		// Block detection: after placing here, does opponent have an unblocked threat?
		blockFlag := nativeBlockFlag(i, opponentPiece[:], cellEmpty[:], one)

		// score = base_rate + BONUS * win_flag - PENALTY * block_flag * (1 - win_flag)
		score := new(big.Int).Set(baseRate)

		if winFlag {
			score = NativeFixAdd(score, HeatmapBonus)
		} else if blockFlag {
			score = NativeFixSub(score, HeatmapPenalty)
		}

		scores[i] = score
	}

	return &TTTHeatmapWitness{
		PreState:      state,
		PostState:     postState,
		StepSize:      h,
		HeatmapScores: scores,
	}
}

// nativeWinFlag checks if placing current player's piece at cell i completes a win line.
func nativeWinFlag(cell int, currentPiece []*big.Int, one *big.Int) bool {
	for _, lineIdx := range TTTCellWinLines[cell] {
		line := TTTWinLines[lineIdx]
		allOwned := true
		for _, c := range line {
			if c == cell {
				continue // This is where we're placing
			}
			if currentPiece[c].Cmp(one) != 0 {
				allOwned = false
				break
			}
		}
		if allOwned {
			return true
		}
	}
	return false
}

// nativeBlockFlag checks if after placing at cell i, the opponent has an unblocked winning threat.
// An unblocked threat is a win line where opponent has 2 of 3 and the missing cell is empty and != i.
func nativeBlockFlag(cell int, opponentPiece []*big.Int, cellEmpty []*big.Int, one *big.Int) bool {
	for _, line := range TTTWinLines {
		oppCount := 0
		missingCell := -1
		for _, c := range line {
			if opponentPiece[c].Cmp(one) == 0 {
				oppCount++
			} else if cellEmpty[c].Cmp(one) == 0 {
				missingCell = c
			}
		}
		// Threat: opponent has 2, and the missing cell is empty and not blocked by our move
		if oppCount == 2 && missingCell >= 0 && missingCell != cell {
			return true
		}
	}
	return false
}

// ToHeatmapCircuitAssignment converts a TTTHeatmapWitness into a gnark circuit assignment.
func (w *TTTHeatmapWitness) ToHeatmapCircuitAssignment() *TTTHeatmapCircuit {
	c := &TTTHeatmapCircuit{
		PreStateRoot:  w.PreState.Root,
		PostStateRoot: w.PostState.Root,
		StepSize:      w.StepSize,
	}

	for i := 0; i < 9; i++ {
		c.HeatmapScores[i] = w.HeatmapScores[i]
	}
	for p := 0; p < TTTNumPlaces; p++ {
		c.PreMarking[p] = w.PreState.Marking[p]
		c.PostMarking[p] = w.PostState.Marking[p]
	}

	return c
}
