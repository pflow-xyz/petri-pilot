package zkode

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// Tsit5StepCircuit proves that one fixed-step Tsit5 ODE integration was computed
// correctly over a 3-place Petri net with mass-action kinetics.
//
// The private witness is the token marking (place values). The public inputs are
// the state root commitments, step size, and rate constants.
//
// Proof chain: the PostStateRoot of step N becomes the PreStateRoot of step N+1.
type Tsit5StepCircuit struct {
	// Public inputs
	PreStateRoot  frontend.Variable `gnark:",public"`
	PostStateRoot frontend.Variable `gnark:",public"`
	StepSize      frontend.Variable `gnark:",public"` // h, fixed-point
	Rates         [NumTransitions]frontend.Variable `gnark:",public"`

	// Private witness
	PreMarking  [NumPlaces]frontend.Variable
	PostMarking [NumPlaces]frontend.Variable
}

// Define declares the R1CS constraints for one Tsit5 ODE step.
func (c *Tsit5StepCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches private marking
	preRoot := mimcHash(api, c.PreMarking[:])
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Compute 7 Tsit5 stages
	// k[stage][place] stores the derivative at each stage
	var k [7][NumPlaces]frontend.Variable

	for stage := 0; stage < 7; stage++ {
		// Compute stage state: y_stage[p] = Pre[p] + h * sum(A[stage][j] * k[j][p])
		var yStage [NumPlaces]frontend.Variable
		for p := 0; p < NumPlaces; p++ {
			yStage[p] = c.PreMarking[p]
		}

		// Add contributions from previous stages
		for j := 0; j < len(tsit5A[stage]); j++ {
			// hA = h * A[stage][j] (step size times RK coefficient)
			hA := FixMul(api, c.StepSize, tsit5A[stage][j])
			for p := 0; p < NumPlaces; p++ {
				// yStage[p] += hA * k[j][p]
				contrib := FixMul(api, hA, k[j][p])
				yStage[p] = api.Add(yStage[p], contrib)
			}
		}

		// Evaluate mass-action rates at stage state
		// rate[t] = Rates[t] * yStage[InputPlace[t]]
		var rates [NumTransitions]frontend.Variable
		for t := 0; t < NumTransitions; t++ {
			rates[t] = FixMul(api, c.Rates[t], yStage[InputPlaces[t]])
		}

		// Compute derivatives: k[stage][p] = sum over transitions of S[p][t] * rate[t]
		for p := 0; p < NumPlaces; p++ {
			k[stage][p] = frontend.Variable(0)
			for t := 0; t < NumTransitions; t++ {
				s := Stoichiometry[p][t]
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

	// 3. Compute expected post state: Post[p] = Pre[p] + h * sum(B[j] * k[j][p])
	var postExpected [NumPlaces]frontend.Variable
	for p := 0; p < NumPlaces; p++ {
		postExpected[p] = c.PreMarking[p]
	}

	for j := 0; j < 7; j++ {
		if tsit5B[j].Sign() == 0 {
			continue // B[6] = 0
		}
		hB := FixMul(api, c.StepSize, tsit5B[j])
		for p := 0; p < NumPlaces; p++ {
			contrib := FixMul(api, hB, k[j][p])
			postExpected[p] = api.Add(postExpected[p], contrib)
		}
	}

	// 4. Assert actual post marking matches expected
	for p := 0; p < NumPlaces; p++ {
		api.AssertIsEqual(c.PostMarking[p], postExpected[p])
	}

	// 5. Verify post-state root matches private marking
	postRoot := mimcHash(api, c.PostMarking[:])
	api.AssertIsEqual(postRoot, c.PostStateRoot)

	return nil
}

// mimcHash computes MiMC hash over a slice of field elements.
func mimcHash(api frontend.API, values []frontend.Variable) frontend.Variable {
	h, _ := mimc.NewMiMC(api)
	for _, v := range values {
		h.Write(v)
	}
	return h.Sum()
}
