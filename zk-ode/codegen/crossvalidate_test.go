package codegen

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

// TestCrossValidate_TTTTopology verifies that the generator produces topology
// data identical to the hand-coded ttt_topology.go values.
func TestCrossValidate_TTTTopology(t *testing.T) {
	model := loadTTTModel(t)
	ctx, err := NewContext(model, "ttt", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	// Verify dimensions
	if ctx.NumPlaces != zkode.TTTNumPlaces {
		t.Fatalf("NumPlaces: generated=%d, hand-coded=%d", ctx.NumPlaces, zkode.TTTNumPlaces)
	}
	if ctx.NumTransitions != zkode.TTTNumTransitions {
		t.Fatalf("NumTransitions: generated=%d, hand-coded=%d", ctx.NumTransitions, zkode.TTTNumTransitions)
	}

	// Verify stoichiometry matrix matches exactly
	mismatches := 0
	for p := 0; p < zkode.TTTNumPlaces; p++ {
		for tr := 0; tr < zkode.TTTNumTransitions; tr++ {
			gen := ctx.Stoichiometry[p][tr]
			hand := zkode.TTTStoichiometry[p][tr]
			if gen != hand {
				t.Errorf("S[%s][%s]: generated=%d, hand-coded=%d",
					zkode.TTTPlaceNames[p], zkode.TTTTransitionNames[tr], gen, hand)
				mismatches++
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d stoichiometry mismatches", mismatches)
	}
	t.Logf("Stoichiometry: %dx%d matrix matches exactly", ctx.NumPlaces, ctx.NumTransitions)

	// Verify transition inputs match
	for tr := 0; tr < zkode.TTTNumTransitions; tr++ {
		genInputs := ctx.Transitions[tr].Inputs
		handInputs := zkode.TTTTransitionInputs[tr]

		if len(genInputs) != len(handInputs) {
			t.Errorf("TransitionInputs[%s]: generated=%d inputs, hand-coded=%d inputs",
				zkode.TTTTransitionNames[tr], len(genInputs), len(handInputs))
			continue
		}

		// Build sets for order-independent comparison
		genSet := make(map[int]bool)
		for _, inp := range genInputs {
			genSet[inp] = true
		}
		for _, inp := range handInputs {
			if !genSet[inp] {
				t.Errorf("TransitionInputs[%s]: hand-coded has input %d (%s) not in generated",
					zkode.TTTTransitionNames[tr], inp, zkode.TTTPlaceNames[inp])
			}
		}
	}
	t.Logf("TransitionInputs: all %d transitions match", ctx.NumTransitions)

	// Verify rate constants match
	for tr := 0; tr < zkode.TTTNumTransitions; tr++ {
		genRate := ctx.Transitions[tr].Rate
		handRate := zkode.FixToFloat(zkode.TTTRateConstants[tr])
		diff := genRate - handRate
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("Rate[%s]: generated=%.4f, hand-coded=%.4f",
				zkode.TTTTransitionNames[tr], genRate, handRate)
		}
	}
	t.Logf("RateConstants: all %d transitions match", ctx.NumTransitions)

	// Verify initial marking
	handMarking := zkode.TTTDefaultInitialMarking()
	for p := 0; p < zkode.TTTNumPlaces; p++ {
		genInit := ctx.Places[p].Initial
		handInit := zkode.FixToFloat(handMarking[p])
		diff := genInit - handInit
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("Initial[%s]: generated=%.4f, hand-coded=%.4f",
				zkode.TTTPlaceNames[p], genInit, handInit)
		}
	}
	t.Logf("InitialMarking: all %d places match", ctx.NumPlaces)
}

// TestCrossValidate_TTTNativeStep verifies that the generated NativeStep
// function produces identical output to the hand-coded NativeTTTStep.
func TestCrossValidate_TTTNativeStep(t *testing.T) {
	model := loadTTTModel(t)
	ctx, err := NewContext(model, "ttt", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	// Build rate constants in the same order
	genRates := make([]*big.Int, ctx.NumTransitions)
	for tr := 0; tr < ctx.NumTransitions; tr++ {
		genRates[tr] = zkode.FixFromFloat(ctx.Transitions[tr].Rate)
	}

	// Build initial marking
	genMarking := make([]*big.Int, ctx.NumPlaces)
	for p := 0; p < ctx.NumPlaces; p++ {
		genMarking[p] = zkode.FixFromFloat(ctx.Places[p].Initial)
	}

	h := zkode.FixFromFloat(0.01)

	// Run hand-coded step
	handMarking := zkode.TTTDefaultInitialMarking()
	handPost := zkode.NativeTTTStep(handMarking, h)

	// Run the same ODE step manually using generated topology data
	// (mirrors what the generated NativeStep function would do)
	genPost := nativeStepFromContext(ctx, genMarking, genRates, h)

	// Compare outputs
	mismatches := 0
	for p := 0; p < zkode.TTTNumPlaces; p++ {
		if handPost[p].Cmp(genPost[p]) != 0 {
			handFloat := zkode.FixToFloat(handPost[p])
			genFloat := zkode.FixToFloat(genPost[p])
			t.Errorf("Post[%s]: hand=%.10f, gen=%.10f (delta=%.2e)",
				zkode.TTTPlaceNames[p], handFloat, genFloat, genFloat-handFloat)
			mismatches++
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d post-marking mismatches", mismatches)
	}
	t.Log("NativeStep: generated topology produces identical ODE output to hand-coded NativeTTTStep")
}

// TestCrossValidate_TTTStateRoot verifies that the MiMC state root from
// generated topology matches the hand-coded genesis root.
func TestCrossValidate_TTTStateRoot(t *testing.T) {
	model := loadTTTModel(t)
	ctx, err := NewContext(model, "ttt", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	// Build marking from generated context
	genMarking := make([]*big.Int, ctx.NumPlaces)
	for p := 0; p < ctx.NumPlaces; p++ {
		genMarking[p] = zkode.FixFromFloat(ctx.Places[p].Initial)
	}
	genRoot := zkode.ComputeRoot(genMarking)

	// Build from hand-coded
	handMarking := zkode.TTTDefaultInitialMarking()
	handRoot := zkode.ComputeRoot(handMarking[:])

	if genRoot.Cmp(handRoot) != 0 {
		t.Fatalf("Genesis root mismatch:\n  generated:  0x%s\n  hand-coded: 0x%s",
			genRoot.Text(16), handRoot.Text(16))
	}
	t.Logf("Genesis root matches: 0x%s", genRoot.Text(16))
}

// TestCrossValidate_CascadeNativeStep verifies that the generated topology
// for the cascade model produces identical ODE output to the hand-coded
// NativeTsit5Step.
func TestCrossValidate_CascadeNativeStep(t *testing.T) {
	model := cascadeModel()
	ctx, err := NewContext(model, "cascade", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	genRates := make([]*big.Int, ctx.NumTransitions)
	for tr := 0; tr < ctx.NumTransitions; tr++ {
		genRates[tr] = zkode.FixFromFloat(ctx.Transitions[tr].Rate)
	}

	genMarking := make([]*big.Int, ctx.NumPlaces)
	for p := 0; p < ctx.NumPlaces; p++ {
		genMarking[p] = zkode.FixFromFloat(ctx.Places[p].Initial)
	}

	h := zkode.FixFromFloat(0.1)

	// Hand-coded cascade step
	handMarking := zkode.DefaultInitialMarking()
	handRates := zkode.DefaultRates()
	handPost := zkode.NativeTsit5Step(handMarking, h, handRates)

	// Generated topology step
	genPost := nativeStepFromContext(ctx, genMarking, genRates, h)

	for p := 0; p < zkode.NumPlaces; p++ {
		if handPost[p].Cmp(genPost[p]) != 0 {
			t.Errorf("Post[%s]: hand=%.10f, gen=%.10f",
				zkode.PlaceNames[p],
				zkode.FixToFloat(handPost[p]),
				zkode.FixToFloat(genPost[p]))
		}
	}

	t.Log("Cascade NativeStep: generated topology produces identical ODE output")
}

// TestCrossValidate_MultiStep verifies multi-step ODE chaining produces
// identical results between hand-coded and generated topology.
func TestCrossValidate_MultiStep(t *testing.T) {
	model := loadTTTModel(t)
	ctx, err := NewContext(model, "ttt", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	genRates := make([]*big.Int, ctx.NumTransitions)
	for tr := 0; tr < ctx.NumTransitions; tr++ {
		genRates[tr] = zkode.FixFromFloat(ctx.Transitions[tr].Rate)
	}

	h := zkode.FixFromFloat(0.01)

	// Run 5 steps with hand-coded
	handMarking := zkode.TTTDefaultInitialMarking()
	for step := 0; step < 5; step++ {
		handMarking = zkode.NativeTTTStep(handMarking, h)
	}

	// Run 5 steps with generated topology
	genMarking := make([]*big.Int, ctx.NumPlaces)
	for p := 0; p < ctx.NumPlaces; p++ {
		genMarking[p] = zkode.FixFromFloat(ctx.Places[p].Initial)
	}
	for step := 0; step < 5; step++ {
		genMarking = nativeStepFromContext(ctx, genMarking, genRates, h)
	}

	// Compare final state
	mismatches := 0
	for p := 0; p < zkode.TTTNumPlaces; p++ {
		if handMarking[p].Cmp(genMarking[p]) != 0 {
			t.Errorf("After 5 steps, Post[%s]: hand=%.10f, gen=%.10f",
				zkode.TTTPlaceNames[p],
				zkode.FixToFloat(handMarking[p]),
				zkode.FixToFloat(genMarking[p]))
			mismatches++
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches after 5 ODE steps", mismatches)
	}

	// Compare state roots
	handRoot := zkode.ComputeRoot(handMarking[:])
	genRoot := zkode.ComputeRoot(genMarking)
	if handRoot.Cmp(genRoot) != 0 {
		t.Fatalf("Root mismatch after 5 steps:\n  hand: 0x%s\n  gen:  0x%s",
			handRoot.Text(16), genRoot.Text(16))
	}
	t.Logf("5-step chain matches: root=0x%s", genRoot.Text(16))
}

// --- helpers ---

func loadTTTModel(t *testing.T) *metamodel.Model {
	t.Helper()
	data, err := os.ReadFile("../../services/tic-tac-toe.json")
	if err != nil {
		t.Skipf("tic-tac-toe.json not found: %v", err)
	}
	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("parsing model: %v", err)
	}
	return &model
}

// nativeStepFromContext runs a Tsit5 ODE step using topology data from
// a generated Context, exactly mirroring what the generated NativeStep
// function would do.
func nativeStepFromContext(
	ctx *Context,
	marking []*big.Int,
	rates []*big.Int,
	h *big.Int,
) []*big.Int {
	np := ctx.NumPlaces
	nt := ctx.NumTransitions

	// k[stage][place]
	k := make([][]*big.Int, 7)
	zero := big.NewInt(0)
	for s := 0; s < 7; s++ {
		k[s] = make([]*big.Int, np)
		for p := 0; p < np; p++ {
			k[s][p] = new(big.Int).Set(zero)
		}
	}

	for stage := 0; stage < 7; stage++ {
		yStage := make([]*big.Int, np)
		for p := 0; p < np; p++ {
			yStage[p] = new(big.Int).Set(marking[p])
		}

		for j := 0; j < len(zkode.Tsit5A[stage]); j++ {
			hA := zkode.NativeFixMul(h, zkode.Tsit5A[stage][j])
			for p := 0; p < np; p++ {
				contrib := zkode.NativeFixMul(hA, k[j][p])
				yStage[p] = zkode.NativeFixAdd(yStage[p], contrib)
			}
		}

		// Mass-action rates
		massRates := make([]*big.Int, nt)
		for tr := 0; tr < nt; tr++ {
			inputs := ctx.Transitions[tr].Inputs
			if len(inputs) == 0 {
				massRates[tr] = new(big.Int).Set(rates[tr])
				continue
			}
			r := new(big.Int).Set(yStage[inputs[0]])
			for i := 1; i < len(inputs); i++ {
				r = zkode.NativeFixMul(r, yStage[inputs[i]])
			}
			r = zkode.NativeFixMul(r, rates[tr])
			massRates[tr] = r
		}

		// Derivatives
		for p := 0; p < np; p++ {
			k[stage][p] = new(big.Int).Set(zero)
			for tr := 0; tr < nt; tr++ {
				s := ctx.Stoichiometry[p][tr]
				if s == 0 {
					continue
				}
				switch {
				case s == 1:
					k[stage][p] = zkode.NativeFixAdd(k[stage][p], massRates[tr])
				case s == -1:
					k[stage][p] = zkode.NativeFixSub(k[stage][p], massRates[tr])
				case s > 1:
					for i := 0; i < s; i++ {
						k[stage][p] = zkode.NativeFixAdd(k[stage][p], massRates[tr])
					}
				case s < -1:
					for i := 0; i < -s; i++ {
						k[stage][p] = zkode.NativeFixSub(k[stage][p], massRates[tr])
					}
				}
			}
		}
	}

	// Final weighted sum
	post := make([]*big.Int, np)
	for p := 0; p < np; p++ {
		post[p] = new(big.Int).Set(marking[p])
	}
	for j := 0; j < 7; j++ {
		if zkode.Tsit5B[j].Sign() == 0 {
			continue
		}
		hB := zkode.NativeFixMul(h, zkode.Tsit5B[j])
		for p := 0; p < np; p++ {
			contrib := zkode.NativeFixMul(hB, k[j][p])
			post[p] = zkode.NativeFixAdd(post[p], contrib)
		}
	}

	return post
}
