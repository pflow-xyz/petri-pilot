package zkode

import (
	"math/big"
)

// StepWitness contains all data needed to create a circuit assignment for one step.
type StepWitness struct {
	PreState  *ODEState
	PostState *ODEState
	StepSize  *big.Int
	Rates     [NumTransitions]*big.Int
}

// NativeTsit5Step performs one Tsit5 ODE integration step using native big.Int
// field arithmetic. This mirrors the circuit computation exactly, ensuring the
// witness values satisfy all constraints.
//
// Returns the new marking after one step of size h.
func NativeTsit5Step(
	marking [NumPlaces]*big.Int,
	h *big.Int,
	rates [NumTransitions]*big.Int,
) [NumPlaces]*big.Int {
	// k[stage][place] = derivative at each RK stage
	var k [7][NumPlaces]*big.Int

	// Initialize all k values to zero
	zero := big.NewInt(0)
	for s := 0; s < 7; s++ {
		for p := 0; p < NumPlaces; p++ {
			k[s][p] = new(big.Int).Set(zero)
		}
	}

	for stage := 0; stage < 7; stage++ {
		// Compute stage state: yStage[p] = marking[p] + h * sum(A[stage][j] * k[j][p])
		var yStage [NumPlaces]*big.Int
		for p := 0; p < NumPlaces; p++ {
			yStage[p] = new(big.Int).Set(marking[p])
		}

		for j := 0; j < len(Tsit5A[stage]); j++ {
			hA := NativeFixMul(h, Tsit5A[stage][j])
			for p := 0; p < NumPlaces; p++ {
				contrib := NativeFixMul(hA, k[j][p])
				yStage[p] = NativeFixAdd(yStage[p], contrib)
			}
		}

		// Evaluate mass-action rates at stage state
		var massRates [NumTransitions]*big.Int
		for t := 0; t < NumTransitions; t++ {
			massRates[t] = NativeFixMul(rates[t], yStage[InputPlaces[t]])
		}

		// Compute derivatives: k[stage][p] = sum(S[p][t] * rate[t])
		for p := 0; p < NumPlaces; p++ {
			k[stage][p] = new(big.Int).Set(zero)
			for t := 0; t < NumTransitions; t++ {
				s := Stoichiometry[p][t]
				if s == 0 {
					continue
				}
				if s == 1 {
					k[stage][p] = NativeFixAdd(k[stage][p], massRates[t])
				} else if s == -1 {
					k[stage][p] = NativeFixSub(k[stage][p], massRates[t])
				}
			}
		}
	}

	// Final weighted sum: post[p] = marking[p] + h * sum(B[j] * k[j][p])
	var post [NumPlaces]*big.Int
	for p := 0; p < NumPlaces; p++ {
		post[p] = new(big.Int).Set(marking[p])
	}

	for j := 0; j < 7; j++ {
		if Tsit5B[j].Sign() == 0 {
			continue
		}
		hB := NativeFixMul(h, Tsit5B[j])
		for p := 0; p < NumPlaces; p++ {
			contrib := NativeFixMul(hB, k[j][p])
			post[p] = NativeFixAdd(post[p], contrib)
		}
	}

	return post
}

// ComputeStep runs one Tsit5 step and generates a full witness for the circuit.
func ComputeStep(state *ODEState, h *big.Int, rates [NumTransitions]*big.Int) *StepWitness {
	postMarking := NativeTsit5Step(state.Marking, h, rates)

	postState := &ODEState{
		Marking: postMarking,
		Root:    ComputeRoot(postMarking[:]),
		Step:    state.Step + 1,
	}

	return &StepWitness{
		PreState:  state,
		PostState: postState,
		StepSize:  h,
		Rates:     rates,
	}
}

// ToCircuitAssignment converts a StepWitness into a gnark circuit assignment.
func (w *StepWitness) ToCircuitAssignment() *Tsit5StepCircuit {
	c := &Tsit5StepCircuit{
		PreStateRoot:  w.PreState.Root,
		PostStateRoot: w.PostState.Root,
		StepSize:      w.StepSize,
	}

	for t := 0; t < NumTransitions; t++ {
		c.Rates[t] = w.Rates[t]
	}
	for p := 0; p < NumPlaces; p++ {
		c.PreMarking[p] = w.PreState.Marking[p]
		c.PostMarking[p] = w.PostState.Marking[p]
	}

	return c
}

// ComputeSteps runs N consecutive Tsit5 steps, returning all witnesses.
// The output root of step i is the input root of step i+1.
func ComputeSteps(initial *ODEState, h *big.Int, rates [NumTransitions]*big.Int, n int) []*StepWitness {
	witnesses := make([]*StepWitness, n)
	state := initial

	for i := 0; i < n; i++ {
		w := ComputeStep(state, h, rates)
		witnesses[i] = w
		state = w.PostState
	}

	return witnesses
}
