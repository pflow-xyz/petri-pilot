package zkode

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// Tsit5StepCircuit proves that one fixed-step Tsit5 ODE integration was computed
// correctly over a Petri net with mass-action kinetics.
//
// The private witness is the token marking (place values). The public inputs are
// the state root commitments, step size, and rate constants.
//
// Proof chain: the PostStateRoot of step N becomes the PreStateRoot of step N+1.
type Tsit5StepCircuit struct {
	// Public inputs
	PreStateRoot  frontend.Variable   `gnark:",public"`
	PostStateRoot frontend.Variable   `gnark:",public"`
	StepSize      frontend.Variable   `gnark:",public"` // h, fixed-point
	Rates         []frontend.Variable `gnark:",public"`

	// Private witness
	PreMarking  []frontend.Variable
	PostMarking []frontend.Variable

	// Compile-time config (not circuit variables)
	Net NetConfig `gnark:"-"`
}

// NewTsit5StepCircuit creates a circuit template sized for the given net.
// Pass the result to frontend.Compile() as the template.
func NewTsit5StepCircuit(net NetConfig) *Tsit5StepCircuit {
	return &Tsit5StepCircuit{
		Rates:       make([]frontend.Variable, net.NumTransitions),
		PreMarking:  make([]frontend.Variable, net.NumPlaces),
		PostMarking: make([]frontend.Variable, net.NumPlaces),
		Net:         net,
	}
}

// Define declares the R1CS constraints for one Tsit5 ODE step.
func (c *Tsit5StepCircuit) Define(api frontend.API) error {
	N := c.Net.NumPlaces
	M := c.Net.NumTransitions

	// 1. Verify pre-state root matches private marking
	preRoot := mimcHash(api, c.PreMarking)
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Compute 7 Tsit5 stages
	// k[stage][place] stores the derivative at each stage
	k := make([][]frontend.Variable, 7)
	for s := 0; s < 7; s++ {
		k[s] = make([]frontend.Variable, N)
	}

	for stage := 0; stage < 7; stage++ {
		// Compute stage state: y_stage[p] = Pre[p] + h * sum(A[stage][j] * k[j][p])
		yStage := make([]frontend.Variable, N)
		for p := 0; p < N; p++ {
			yStage[p] = c.PreMarking[p]
		}

		// Add contributions from previous stages
		for j := 0; j < len(tsit5A[stage]); j++ {
			// hA = h * A[stage][j] (step size times RK coefficient)
			hA := FixMul(api, c.StepSize, tsit5A[stage][j])
			for p := 0; p < N; p++ {
				// yStage[p] += hA * k[j][p]
				contrib := FixMul(api, hA, k[j][p])
				yStage[p] = api.Add(yStage[p], contrib)
			}
		}

		// Evaluate mass-action rates at stage state
		// rate[t] = Rates[t] * product(yStage[p] for p in InputArcs[t])
		rates := make([]frontend.Variable, M)
		for t := 0; t < M; t++ {
			r := c.Rates[t]
			for _, p := range c.Net.InputArcs[t] {
				r = FixMul(api, r, yStage[p])
			}
			rates[t] = r
		}

		// Compute derivatives: k[stage][p] = sum over transitions of S[p][t] * rate[t]
		for p := 0; p < N; p++ {
			k[stage][p] = frontend.Variable(0)
			for t := 0; t < M; t++ {
				s := c.Net.Stoichiometry[p][t]
				if s == 0 {
					continue
				} else if s == 1 {
					k[stage][p] = api.Add(k[stage][p], rates[t])
				} else if s == -1 {
					k[stage][p] = api.Sub(k[stage][p], rates[t])
				} else if s > 0 {
					k[stage][p] = api.Add(k[stage][p], api.Mul(rates[t], s))
				} else {
					k[stage][p] = api.Sub(k[stage][p], api.Mul(rates[t], -s))
				}
			}
		}
	}

	// 3. Compute expected post state: Post[p] = Pre[p] + h * sum(B[j] * k[j][p])
	postExpected := make([]frontend.Variable, N)
	for p := 0; p < N; p++ {
		postExpected[p] = c.PreMarking[p]
	}

	for j := 0; j < 7; j++ {
		if tsit5B[j].Sign() == 0 {
			continue // B[6] = 0
		}
		hB := FixMul(api, c.StepSize, tsit5B[j])
		for p := 0; p < N; p++ {
			contrib := FixMul(api, hB, k[j][p])
			postExpected[p] = api.Add(postExpected[p], contrib)
		}
	}

	// 4. Assert actual post marking matches expected
	for p := 0; p < N; p++ {
		api.AssertIsEqual(c.PostMarking[p], postExpected[p])
	}

	// 5. Verify post-state root matches private marking
	postRoot := mimcHash(api, c.PostMarking)
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
