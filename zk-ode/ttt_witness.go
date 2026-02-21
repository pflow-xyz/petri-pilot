package zkode

import (
	"math/big"
)

// TTTODEState tracks the current marking and MiMC state root for the TTT net.
type TTTODEState struct {
	Marking [TTTNumPlaces]*big.Int
	Root    *big.Int
	Step    int
}

// NewTTTODEState creates an initial TTT state from a marking.
func NewTTTODEState(marking [TTTNumPlaces]*big.Int) *TTTODEState {
	s := &TTTODEState{
		Marking: marking,
		Step:    0,
	}
	s.Root = ComputeRoot(marking[:])
	return s
}

// nativeMultiInputRate computes rate = k[t] * product(marking[inputs[t]]) in native big.Int.
func nativeMultiInputRate(marking [TTTNumPlaces]*big.Int, t int) *big.Int {
	inputs := TTTTransitionInputs[t]
	rate := new(big.Int).Set(marking[inputs[0]])
	for i := 1; i < len(inputs); i++ {
		rate = NativeFixMul(rate, marking[inputs[i]])
	}
	rate = NativeFixMul(rate, TTTRateConstants[t])
	return rate
}

// NativeTTTStep performs one Tsit5 ODE integration step over the TTT net using
// native big.Int field arithmetic. Mirrors the circuit computation exactly.
func NativeTTTStep(
	marking [TTTNumPlaces]*big.Int,
	h *big.Int,
) [TTTNumPlaces]*big.Int {
	var k [7][TTTNumPlaces]*big.Int

	zero := big.NewInt(0)
	for s := 0; s < 7; s++ {
		for p := 0; p < TTTNumPlaces; p++ {
			k[s][p] = new(big.Int).Set(zero)
		}
	}

	for stage := 0; stage < 7; stage++ {
		// yStage[p] = marking[p] + h * sum(A[stage][j] * k[j][p])
		var yStage [TTTNumPlaces]*big.Int
		for p := 0; p < TTTNumPlaces; p++ {
			yStage[p] = new(big.Int).Set(marking[p])
		}

		for j := 0; j < len(tsit5A[stage]); j++ {
			hA := NativeFixMul(h, tsit5A[stage][j])
			for p := 0; p < TTTNumPlaces; p++ {
				contrib := NativeFixMul(hA, k[j][p])
				yStage[p] = NativeFixAdd(yStage[p], contrib)
			}
		}

		// Multi-input mass-action rates
		var rates [TTTNumTransitions]*big.Int
		for t := 0; t < TTTNumTransitions; t++ {
			rates[t] = nativeMultiInputRate(yStage, t)
		}

		// Derivatives: k[stage][p] = sum(S[p][t] * rate[t])
		for p := 0; p < TTTNumPlaces; p++ {
			k[stage][p] = new(big.Int).Set(zero)
			for t := 0; t < TTTNumTransitions; t++ {
				s := TTTStoichiometry[p][t]
				if s == 0 {
					continue
				}
				if s == 1 {
					k[stage][p] = NativeFixAdd(k[stage][p], rates[t])
				} else if s == -1 {
					k[stage][p] = NativeFixSub(k[stage][p], rates[t])
				}
			}
		}
	}

	// Final weighted sum: post[p] = marking[p] + h * sum(B[j] * k[j][p])
	var post [TTTNumPlaces]*big.Int
	for p := 0; p < TTTNumPlaces; p++ {
		post[p] = new(big.Int).Set(marking[p])
	}

	for j := 0; j < 7; j++ {
		if tsit5B[j].Sign() == 0 {
			continue
		}
		hB := NativeFixMul(h, tsit5B[j])
		for p := 0; p < TTTNumPlaces; p++ {
			contrib := NativeFixMul(hB, k[j][p])
			post[p] = NativeFixAdd(post[p], contrib)
		}
	}

	return post
}

// ApplyDiscreteMove applies a transition to a discrete marking using the
// stoichiometry matrix. For each place where S[p][t]==-1, subtracts 1.0;
// where S[p][t]==+1, adds 1.0. Returns a new marking with clean integer values.
func ApplyDiscreteMove(marking [TTTNumPlaces]*big.Int, transition int) [TTTNumPlaces]*big.Int {
	one := FixFromFloat(1.0)
	var result [TTTNumPlaces]*big.Int
	for p := 0; p < TTTNumPlaces; p++ {
		result[p] = new(big.Int).Set(marking[p])
		s := TTTStoichiometry[p][transition]
		if s == -1 {
			result[p] = NativeFixSub(result[p], one)
		} else if s == 1 {
			result[p] = NativeFixAdd(result[p], one)
		}
	}
	return result
}

// BoardToTTTODEState converts a Board + player turn into a TTTODEState.
func BoardToTTTODEState(board Board, currentPlayer string) *TTTODEState {
	var marking [TTTNumPlaces]*big.Int
	zero := FixFromFloat(0.0)
	one := FixFromFloat(1.0)

	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			idx := r*3 + c
			cellPlace := P00 + idx
			xPlace := X00 + idx
			oPlace := O00 + idx

			switch board[r][c] {
			case "X":
				marking[cellPlace] = new(big.Int).Set(zero)
				marking[xPlace] = new(big.Int).Set(one)
				marking[oPlace] = new(big.Int).Set(zero)
			case "O":
				marking[cellPlace] = new(big.Int).Set(zero)
				marking[xPlace] = new(big.Int).Set(zero)
				marking[oPlace] = new(big.Int).Set(one)
			default:
				marking[cellPlace] = new(big.Int).Set(one)
				marking[xPlace] = new(big.Int).Set(zero)
				marking[oPlace] = new(big.Int).Set(zero)
			}
		}
	}

	if currentPlayer == "X" {
		marking[XTurn] = new(big.Int).Set(one)
		marking[OTurn] = new(big.Int).Set(zero)
	} else {
		marking[XTurn] = new(big.Int).Set(zero)
		marking[OTurn] = new(big.Int).Set(one)
	}

	marking[WinX] = new(big.Int).Set(zero)
	marking[WinO] = new(big.Int).Set(zero)
	marking[GameActive] = new(big.Int).Set(one)

	return NewTTTODEState(marking)
}
