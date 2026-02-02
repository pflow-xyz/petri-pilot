package zkpoker

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

func TestActionCircuitCompiles(t *testing.T) {
	var circuit ActionCircuit
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Circuit compilation failed: %v", err)
	}

	t.Logf("ActionCircuit compiled successfully")
	t.Logf("  Constraints: %d", cs.GetNbConstraints())
	t.Logf("  Public inputs: %d", cs.GetNbPublicVariables())
	t.Logf("  Secret inputs: %d", cs.GetNbSecretVariables())
}

func TestActionCircuitStartHand(t *testing.T) {
	// Start with initial marking
	preMark := InitialMarking()

	// Fire start_hand
	postMark, ok := Fire(preMark, TransitionStartHand)
	if !ok {
		t.Fatal("Failed to fire start_hand")
	}

	// Create witness
	witness := PrepareActionWitness(preMark, postMark, TransitionStartHand)

	// Test the circuit
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestActionCircuitPlayerFold(t *testing.T) {
	// Start hand first
	m := InitialMarking()
	m, _ = Fire(m, TransitionStartHand)

	// Player 0 folds
	preMark := m
	postMark, ok := Fire(m, ActionTransition(0, ActionFold))
	if !ok {
		t.Fatal("Failed to fire player 0 fold")
	}

	// Create witness
	witness := PrepareActionWitness(preMark, postMark, ActionTransition(0, ActionFold))

	// Test the circuit
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestActionCircuitPlayerCheck(t *testing.T) {
	// Start hand first
	m := InitialMarking()
	m, _ = Fire(m, TransitionStartHand)

	// Player 0 checks
	preMark := m
	postMark, ok := Fire(m, ActionTransition(0, ActionCheck))
	if !ok {
		t.Fatal("Failed to fire player 0 check")
	}

	witness := PrepareActionWitness(preMark, postMark, ActionTransition(0, ActionCheck))

	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestActionCircuitInvalidTransition(t *testing.T) {
	// Start with initial marking
	preMark := InitialMarking()

	// Try to fire deal_flop without being in preflop (should fail)
	// We manually create an invalid post-state
	postMark := preMark
	postMark[PhaseFlop] = 1

	witness := PrepareActionWitness(preMark, postMark, TransitionDealFlop)

	// This should fail because deal_flop is not enabled
	assert := test.NewAssert(t)
	assert.ProverFailed(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestActionCircuitWrongPostState(t *testing.T) {
	// Start hand correctly
	preMark := InitialMarking()
	postMark, _ := Fire(preMark, TransitionStartHand)

	// But then claim a different post-state
	wrongPostMark := postMark
	wrongPostMark[PhaseFlop] = 1 // Incorrectly claim we're in flop

	witness := PrepareActionWitness(preMark, wrongPostMark, TransitionStartHand)

	// This should fail because post-state doesn't match transition effect
	assert := test.NewAssert(t)
	assert.ProverFailed(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestActionCircuitDealCard(t *testing.T) {
	// Deal the ace of spades (card index 51)
	preMark := InitialMarking()

	postMark, ok := Fire(preMark, DealTransitionStart+51)
	if !ok {
		t.Fatal("Failed to deal As")
	}

	witness := PrepareActionWitness(preMark, postMark, DealTransitionStart+51)

	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestActionCircuitFullBettingRound(t *testing.T) {
	m := InitialMarking()

	// Start hand
	m, _ = Fire(m, TransitionStartHand)

	// Each player checks in sequence
	for p := 0; p < NumPlayers; p++ {
		preMark := m
		postMark, ok := Fire(m, ActionTransition(p, ActionCheck))
		if !ok {
			t.Fatalf("Player %d check failed", p)
		}

		witness := PrepareActionWitness(preMark, postMark, ActionTransition(p, ActionCheck))

		assert := test.NewAssert(t)
		assert.ProverSucceeded(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))

		m = postMark
	}

	// Now betting should be complete, can deal flop
	if m[PlaceBettingComplete] != 1 {
		t.Fatal("Betting should be complete")
	}

	preMark := m
	postMark, ok := Fire(m, TransitionDealFlop)
	if !ok {
		t.Fatal("Deal flop failed")
	}

	witness := PrepareActionWitness(preMark, postMark, TransitionDealFlop)

	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ActionCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

// BenchmarkActionCircuit measures constraint count and proving time
func BenchmarkActionCircuitCompile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var circuit ActionCircuit
		_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
		if err != nil {
			b.Fatal(err)
		}
	}
}
