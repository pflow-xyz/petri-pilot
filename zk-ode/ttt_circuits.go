package zkode

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// TTTStepCircuit proves one fixed-step Tsit5 ODE integration over the full
// tic-tac-toe Petri net (32 places, 34 transitions) with multi-input mass-action
// kinetics.
//
// Public inputs: PreStateRoot, PostStateRoot, StepSize, ActualRates[34] = 37 total.
// The ActualRates are the initial mass-action rates (product of input markings),
// exposed so the on-chain contract can enforce optimal play by comparing rates.
type TTTStepCircuit struct {
	// Public inputs
	PreStateRoot  frontend.Variable                       `gnark:",public"`
	PostStateRoot frontend.Variable                       `gnark:",public"`
	StepSize      frontend.Variable                       `gnark:",public"`
	ActualRates   [TTTNumTransitions]frontend.Variable     `gnark:",public"`

	// Private witness
	PreMarking  [TTTNumPlaces]frontend.Variable
	PostMarking [TTTNumPlaces]frontend.Variable
}

// Define declares the R1CS constraints for one Tsit5 ODE step over the TTT net.
func (c *TTTStepCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches private marking
	preRoot := tttMimcHash(api, c.PreMarking[:])
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Compute and assert initial rates = product(PreMarking[inputs[t]])
	for t := 0; t < TTTNumTransitions; t++ {
		rate := computeMultiInputRate(api, c.PreMarking[:], t)
		api.AssertIsEqual(rate, c.ActualRates[t])
	}

	// 3. Compute 7 Tsit5 stages with multi-input mass-action kinetics
	var k [7][TTTNumPlaces]frontend.Variable

	for stage := 0; stage < 7; stage++ {
		// Compute stage state: yStage[p] = Pre[p] + h * sum(A[stage][j] * k[j][p])
		var yStage [TTTNumPlaces]frontend.Variable
		for p := 0; p < TTTNumPlaces; p++ {
			yStage[p] = c.PreMarking[p]
		}

		for j := 0; j < len(tsit5A[stage]); j++ {
			hA := FixMul(api, c.StepSize, tsit5A[stage][j])
			for p := 0; p < TTTNumPlaces; p++ {
				contrib := FixMul(api, hA, k[j][p])
				yStage[p] = api.Add(yStage[p], contrib)
			}
		}

		// Evaluate multi-input mass-action rates at stage state
		var rates [TTTNumTransitions]frontend.Variable
		for t := 0; t < TTTNumTransitions; t++ {
			rates[t] = computeMultiInputRate(api, yStage[:], t)
		}

		// Compute derivatives: k[stage][p] = sum(S[p][t] * rate[t])
		for p := 0; p < TTTNumPlaces; p++ {
			k[stage][p] = frontend.Variable(0)
			for t := 0; t < TTTNumTransitions; t++ {
				s := TTTStoichiometry[p][t]
				if s == 0 {
					continue
				}
				if s == 1 {
					k[stage][p] = api.Add(k[stage][p], rates[t])
				} else if s == -1 {
					k[stage][p] = api.Sub(k[stage][p], rates[t])
				}
			}
		}
	}

	// 4. Compute expected post state: Post[p] = Pre[p] + h * sum(B[j] * k[j][p])
	var postExpected [TTTNumPlaces]frontend.Variable
	for p := 0; p < TTTNumPlaces; p++ {
		postExpected[p] = c.PreMarking[p]
	}

	for j := 0; j < 7; j++ {
		if tsit5B[j].Sign() == 0 {
			continue // B[6] = 0
		}
		hB := FixMul(api, c.StepSize, tsit5B[j])
		for p := 0; p < TTTNumPlaces; p++ {
			contrib := FixMul(api, hB, k[j][p])
			postExpected[p] = api.Add(postExpected[p], contrib)
		}
	}

	// 5. Assert actual post marking matches expected
	for p := 0; p < TTTNumPlaces; p++ {
		api.AssertIsEqual(c.PostMarking[p], postExpected[p])
	}

	// 6. Verify post-state root matches private marking
	postRoot := tttMimcHash(api, c.PostMarking[:])
	api.AssertIsEqual(postRoot, c.PostStateRoot)

	return nil
}

// computeMultiInputRate computes the mass-action rate for transition t:
// rate = k[t] * product(marking[input]) for all input places.
func computeMultiInputRate(api frontend.API, marking []frontend.Variable, t int) frontend.Variable {
	inputs := TTTTransitionInputs[t]
	rate := marking[inputs[0]]
	for i := 1; i < len(inputs); i++ {
		rate = FixMul(api, rate, marking[inputs[i]])
	}
	rate = FixMul(api, rate, TTTRateConstants[t])
	return rate
}

// tttMimcHash computes MiMC hash over TTT marking values.
func tttMimcHash(api frontend.API, values []frontend.Variable) frontend.Variable {
	h, _ := mimc.NewMiMC(api)
	for _, v := range values {
		h.Write(v)
	}
	return h.Sum()
}
