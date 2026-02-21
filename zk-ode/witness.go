package zkode

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
)

// StepWitness contains all data needed to create a circuit assignment for one step.
type StepWitness struct {
	PreState  *ODEState
	PostState *ODEState
	StepSize  *big.Int
	Rates     []*big.Int
}

// NativeTsit5Step performs one Tsit5 ODE integration step using native big.Int
// field arithmetic. This mirrors the circuit computation exactly, ensuring the
// witness values satisfy all constraints.
//
// Returns the new marking after one step of size h.
func NativeTsit5Step(
	net NetConfig,
	marking []*big.Int,
	h *big.Int,
	rates []*big.Int,
) []*big.Int {
	N := net.NumPlaces
	M := net.NumTransitions

	// k[stage][place] = derivative at each RK stage
	k := make([][]*big.Int, 7)
	for s := 0; s < 7; s++ {
		k[s] = make([]*big.Int, N)
		for p := 0; p < N; p++ {
			k[s][p] = new(big.Int)
		}
	}

	for stage := 0; stage < 7; stage++ {
		// Compute stage state: yStage[p] = marking[p] + h * sum(A[stage][j] * k[j][p])
		yStage := make([]*big.Int, N)
		for p := 0; p < N; p++ {
			yStage[p] = new(big.Int).Set(marking[p])
		}

		for j := 0; j < len(tsit5A[stage]); j++ {
			hA := NativeFixMul(h, tsit5A[stage][j])
			for p := 0; p < N; p++ {
				contrib := NativeFixMul(hA, k[j][p])
				yStage[p] = NativeFixAdd(yStage[p], contrib)
			}
		}

		// Evaluate mass-action rates at stage state
		// rate[t] = Rates[t] * product(yStage[p] for p in InputArcs[t])
		massRates := make([]*big.Int, M)
		for t := 0; t < M; t++ {
			r := new(big.Int).Set(rates[t])
			for _, p := range net.InputArcs[t] {
				r = NativeFixMul(r, yStage[p])
			}
			massRates[t] = r
		}

		// Compute derivatives: k[stage][p] = sum(S[p][t] * rate[t])
		for p := 0; p < N; p++ {
			k[stage][p] = new(big.Int)
			for t := 0; t < M; t++ {
				s := net.Stoichiometry[p][t]
				if s == 0 {
					continue
				} else if s == 1 {
					k[stage][p] = NativeFixAdd(k[stage][p], massRates[t])
				} else if s == -1 {
					k[stage][p] = NativeFixSub(k[stage][p], massRates[t])
				} else {
					sBI := big.NewInt(int64(s))
					scaled := new(big.Int).Mul(massRates[t], sBI)
					scaled.Mod(scaled, fieldModulus)
					k[stage][p] = NativeFixAdd(k[stage][p], scaled)
				}
			}
		}
	}

	// Final weighted sum: post[p] = marking[p] + h * sum(B[j] * k[j][p])
	post := make([]*big.Int, N)
	for p := 0; p < N; p++ {
		post[p] = new(big.Int).Set(marking[p])
	}

	for j := 0; j < 7; j++ {
		if tsit5B[j].Sign() == 0 {
			continue
		}
		hB := NativeFixMul(h, tsit5B[j])
		for p := 0; p < N; p++ {
			contrib := NativeFixMul(hB, k[j][p])
			post[p] = NativeFixAdd(post[p], contrib)
		}
	}

	return post
}

// ComputeStep runs one Tsit5 step and generates a full witness for the circuit.
func ComputeStep(net NetConfig, state *ODEState, h *big.Int, rates []*big.Int) *StepWitness {
	postMarking := NativeTsit5Step(net, state.Marking, h, rates)

	postState := &ODEState{
		Marking: postMarking,
		Root:    ComputeRoot(postMarking),
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
	numPlaces := len(w.PreState.Marking)
	numTrans := len(w.Rates)

	c := &Tsit5StepCircuit{
		PreStateRoot:  w.PreState.Root,
		PostStateRoot: w.PostState.Root,
		StepSize:      w.StepSize,
		Rates:         make([]frontend.Variable, numTrans),
		PreMarking:    make([]frontend.Variable, numPlaces),
		PostMarking:   make([]frontend.Variable, numPlaces),
	}

	for t := 0; t < numTrans; t++ {
		c.Rates[t] = w.Rates[t]
	}
	for p := 0; p < numPlaces; p++ {
		c.PreMarking[p] = w.PreState.Marking[p]
		c.PostMarking[p] = w.PostState.Marking[p]
	}

	return c
}

// ComputeSteps runs N consecutive Tsit5 steps, returning all witnesses.
// The output root of step i is the input root of step i+1.
func ComputeSteps(net NetConfig, initial *ODEState, h *big.Int, rates []*big.Int, n int) []*StepWitness {
	witnesses := make([]*StepWitness, n)
	state := initial

	for i := 0; i < n; i++ {
		w := ComputeStep(net, state, h, rates)
		witnesses[i] = w
		state = w.PostState
	}

	return witnesses
}
