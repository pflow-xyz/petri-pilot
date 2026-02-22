package zkode

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// TTTHeatmapCircuit proves one ODE step + tactical win/block evaluation.
//
// Public inputs (12): PreStateRoot, PostStateRoot, StepSize, HeatmapScores[9]
// Private witness (64): PreMarking[32], PostMarking[32]
//
// The heatmap scores incorporate base ODE rates plus tactical adjustments:
//   score[i] = base_rate[i] + BONUS * win_flag[i] - PENALTY * block_flag[i] * (1 - win_flag[i])
type TTTHeatmapCircuit struct {
	// Public inputs
	PreStateRoot  frontend.Variable    `gnark:",public"`
	PostStateRoot frontend.Variable    `gnark:",public"`
	StepSize      frontend.Variable    `gnark:",public"`
	HeatmapScores [9]frontend.Variable `gnark:",public"`

	// Private witness
	PreMarking  [TTTNumPlaces]frontend.Variable
	PostMarking [TTTNumPlaces]frontend.Variable
}

func (c *TTTHeatmapCircuit) Define(api frontend.API) error {
	// === 1. Verify pre-state root ===
	preH, _ := mimc.NewMiMC(api)
	for _, v := range c.PreMarking {
		preH.Write(v)
	}
	api.AssertIsEqual(preH.Sum(), c.PreStateRoot)

	// === 2. Compute initial rates and run ODE step ===
	var initialRates [TTTNumTransitions]frontend.Variable
	for t := 0; t < TTTNumTransitions; t++ {
		initialRates[t] = ComputeMultiInputRate(api, c.PreMarking[:], t)
	}

	// 7-stage Tsit5 ODE integration (reuses existing logic)
	var k [7][TTTNumPlaces]frontend.Variable

	for stage := 0; stage < 7; stage++ {
		var yStage [TTTNumPlaces]frontend.Variable
		for p := 0; p < TTTNumPlaces; p++ {
			yStage[p] = c.PreMarking[p]
		}

		for j := 0; j < len(Tsit5A[stage]); j++ {
			hA := FixMul(api, c.StepSize, Tsit5A[stage][j])
			for p := 0; p < TTTNumPlaces; p++ {
				contrib := FixMul(api, hA, k[j][p])
				yStage[p] = api.Add(yStage[p], contrib)
			}
		}

		var rates [TTTNumTransitions]frontend.Variable
		for t := 0; t < TTTNumTransitions; t++ {
			rates[t] = ComputeMultiInputRate(api, yStage[:], t)
		}

		for p := 0; p < TTTNumPlaces; p++ {
			k[stage][p] = frontend.Variable(0)
			for t := 0; t < TTTNumTransitions; t++ {
				s := TTTStoichiometry[p][t]
				if s == 0 {
					continue
				}
				switch {
				case s == 1:
					k[stage][p] = api.Add(k[stage][p], rates[t])
				case s == -1:
					k[stage][p] = api.Sub(k[stage][p], rates[t])
				case s > 1:
					k[stage][p] = api.Add(k[stage][p], api.Mul(rates[t], s))
				case s < -1:
					k[stage][p] = api.Sub(k[stage][p], api.Mul(rates[t], -s))
				}
			}
		}
	}

	// Final weighted sum
	var postExpected [TTTNumPlaces]frontend.Variable
	for p := 0; p < TTTNumPlaces; p++ {
		postExpected[p] = c.PreMarking[p]
	}
	for j := 0; j < 7; j++ {
		if Tsit5B[j].Sign() == 0 {
			continue
		}
		hB := FixMul(api, c.StepSize, Tsit5B[j])
		for p := 0; p < TTTNumPlaces; p++ {
			contrib := FixMul(api, hB, k[j][p])
			postExpected[p] = api.Add(postExpected[p], contrib)
		}
	}

	for p := 0; p < TTTNumPlaces; p++ {
		api.AssertIsEqual(c.PostMarking[p], postExpected[p])
	}

	// === 3. Verify post-state root ===
	postH, _ := mimc.NewMiMC(api)
	for _, v := range c.PostMarking {
		postH.Write(v)
	}
	api.AssertIsEqual(postH.Sum(), c.PostStateRoot)

	// === 4. Tactical heatmap evaluation ===

	// Determine current player from XTurn marking.
	// isXTurn = 1 (raw) if PreMarking[XTurn] != 0, else 0 — for api.Select
	xTurnNonZero := IsNonZeroBool(api, c.PreMarking[XTurn])

	// Select current/opponent pieces and base rates per cell
	var currentPiece [9]frontend.Variable
	var opponentPiece [9]frontend.Variable
	var baseRate [9]frontend.Variable
	var cellEmpty [9]frontend.Variable

	for i := 0; i < 9; i++ {
		currentPiece[i] = api.Select(xTurnNonZero, c.PreMarking[X00+i], c.PreMarking[O00+i])
		opponentPiece[i] = api.Select(xTurnNonZero, c.PreMarking[O00+i], c.PreMarking[X00+i])
		baseRate[i] = api.Select(xTurnNonZero, initialRates[TXPlay00+i], initialRates[TOPlay00+i])
		cellEmpty[i] = c.PreMarking[P00+i]
	}

	// For each cell, compute win flag, block flag, and final score
	for i := 0; i < 9; i++ {
		// Win detection: check each win line through cell i
		winFlag := circuitWinFlag(api, i, currentPiece[:])

		// Block detection: check opponent threats not blocked by placing at i
		blockFlag := circuitBlockFlag(api, i, opponentPiece[:], cellEmpty[:])

		// score = base_rate + BONUS * win_flag - PENALTY * block_flag * (1 - win_flag)
		bonusTerm := FixMul(api, HeatmapBonus, winFlag)
		oneMinusWin := api.Sub(FixFromFloat(1.0), winFlag)
		penaltyTerm := FixMul(api, HeatmapPenalty, FixMul(api, blockFlag, oneMinusWin))
		score := api.Add(baseRate[i], bonusTerm)
		score = api.Sub(score, penaltyTerm)

		// Mask occupied cells: score = cellEmpty[i] > 0 ? score : 0
		cellNonZero := IsNonZeroFP(api, cellEmpty[i])
		score = FixMul(api, score, cellNonZero)

		api.AssertIsEqual(score, c.HeatmapScores[i])
	}

	return nil
}

// ComputeMultiInputRate computes the mass-action rate for transition t:
// rate = k[t] * product(marking[input]) for all input places.
func ComputeMultiInputRate(api frontend.API, marking []frontend.Variable, t int) frontend.Variable {
	inputs := TTTTransitionInputs[t]
	rate := marking[inputs[0]]
	for i := 1; i < len(inputs); i++ {
		rate = FixMul(api, rate, marking[inputs[i]])
	}
	rate = FixMul(api, rate, TTTRateConstants[t])
	return rate
}

// IsNonZeroBool returns raw 0 or 1 (for api.Select which requires boolean 0/1).
func IsNonZeroBool(api frontend.API, v frontend.Variable) frontend.Variable {
	iz := api.IsZero(v) // raw 1 if v==0, raw 0 if v!=0
	return api.Sub(1, iz)
}

// IsNonZeroFP returns fixed-point 0 or 1.0 (Scale) for use in FixMul arithmetic.
func IsNonZeroFP(api frontend.API, v frontend.Variable) frontend.Variable {
	iz := api.IsZero(v) // raw 0 or 1
	izFP := api.Mul(iz, Scale)
	return api.Sub(FixFromFloat(1.0), izFP)
}

// circuitWinFlag computes a fixed-point flag (0 or 1.0) indicating if placing
// the current player's piece at cell completes a win line.
func circuitWinFlag(api frontend.API, cell int, currentPiece []frontend.Variable) frontend.Variable {
	one := FixFromFloat(1.0)
	// Sum of line_wins across all win lines through this cell
	winSum := frontend.Variable(0)
	for _, lineIdx := range TTTCellWinLines[cell] {
		line := TTTWinLines[lineIdx]
		// Product of current player's pieces at the other two cells in this line
		lineWin := frontend.Variable(one)
		for _, c := range line {
			if c == cell {
				continue
			}
			lineWin = FixMul(api, lineWin, currentPiece[c])
		}
		winSum = api.Add(winSum, lineWin)
	}
	// win_flag = isNonZero(winSum) → 1.0 if any line completes
	return IsNonZeroFP(api, winSum)
}

// circuitBlockFlag computes a fixed-point flag (0 or 1.0) indicating if after
// placing at cell, the opponent has an unblocked winning threat.
//
// For each win line, a threat exists if opponent has 2 of 3 cells and the missing
// cell is empty and not equal to our placement cell. The "not equal to cell" check
// is resolved at compile time since the topology is fixed.
func circuitBlockFlag(api frontend.API, cell int, opponentPiece []frontend.Variable, cellEmpty []frontend.Variable) frontend.Variable {
	one := FixFromFloat(1.0)
	threatSum := frontend.Variable(0)

	for _, line := range TTTWinLines {
		// For each cell in the line, check if it could be the "missing" cell
		// where the opponent needs to complete the line.
		for missingIdx := 0; missingIdx < 3; missingIdx++ {
			missing := line[missingIdx]
			if missing == cell {
				// Our move blocks this threat — skip at compile time
				continue
			}

			// The other two cells in the line must have opponent pieces
			otherProduct := frontend.Variable(one)
			for checkIdx := 0; checkIdx < 3; checkIdx++ {
				if checkIdx == missingIdx {
					continue
				}
				c := line[checkIdx]
				otherProduct = FixMul(api, otherProduct, opponentPiece[c])
			}

			// The missing cell must be empty
			threat := FixMul(api, otherProduct, cellEmpty[missing])
			threatSum = api.Add(threatSum, threat)
		}
	}

	return IsNonZeroFP(api, threatSum)
}
