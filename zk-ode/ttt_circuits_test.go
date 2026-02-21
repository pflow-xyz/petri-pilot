package zkode

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestTTTCircuitCompiles(t *testing.T) {
	circuit := &TTTStepCircuit{}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("TTT circuit compilation failed: %v", err)
	}

	t.Logf("TTT circuit constraints: %d", cs.GetNbConstraints())
	t.Logf("TTT public variables: %d", cs.GetNbPublicVariables())
	t.Logf("TTT secret variables: %d", cs.GetNbSecretVariables())

	// Expect ~100k-120k constraints
	constraints := cs.GetNbConstraints()
	if constraints < 50000 {
		t.Errorf("unexpectedly few constraints: %d (expected ~100k+)", constraints)
	}

	// Public: PreStateRoot + PostStateRoot + StepSize + 34 ActualRates = 37
	// gnark adds 1 for the constant "1" wire, so public vars = 38
	pubVars := cs.GetNbPublicVariables()
	if pubVars != 38 {
		t.Errorf("expected 38 public variables (37 + constant), got %d", pubVars)
	}
}

func TestTTTSingleStepProof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTT proof test in short mode (compilation takes ~minutes)")
	}

	// Compile
	circuit := &TTTStepCircuit{}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}
	t.Logf("Compiled: %d constraints", cs.GetNbConstraints())

	// Setup
	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Compute witness from empty board
	h := FixFromFloat(0.01)
	state := NewTTTODEState(TTTDefaultInitialMarking())
	w := ComputeTTTStep(state, h)
	assignment := w.ToCircuitAssignment()

	// Create gnark witness
	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness creation failed: %v", err)
	}

	// Prove
	proof, err := groth16.Prove(cs, pk, witness)
	if err != nil {
		t.Fatalf("prove failed: %v", err)
	}

	// Verify
	publicWitness, err := witness.Public()
	if err != nil {
		t.Fatalf("public witness extraction failed: %v", err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	t.Log("TTT single step proof verified successfully")
}

func TestTTTNativeMatchesCircuit(t *testing.T) {
	// Verify that the native computation produces the same results
	// that the circuit would verify. We do this by checking that the
	// witness satisfies all public input assertions.
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)

	w := ComputeTTTStep(state, h)

	// Pre-state root should match MiMC of pre-marking
	expectedPreRoot := ComputeRoot(w.PreState.Marking[:])
	if expectedPreRoot.Cmp(w.PreState.Root) != 0 {
		t.Error("pre-state root mismatch")
	}

	// Post-state root should match MiMC of post-marking
	expectedPostRoot := ComputeRoot(w.PostState.Marking[:])
	if expectedPostRoot.Cmp(w.PostState.Root) != 0 {
		t.Error("post-state root mismatch")
	}

	// Actual rates should match native computation
	for tr := 0; tr < TTTNumTransitions; tr++ {
		rate := nativeMultiInputRate(w.PreState.Marking, tr)
		if rate.Cmp(w.ActualRates[tr]) != 0 {
			t.Errorf("rate[%d] mismatch: computed=%s, witness=%s",
				tr, rate.Text(10), w.ActualRates[tr].Text(10))
		}
	}
}

func TestTTTProofWithBoardState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTT proof test in short mode")
	}

	// Test with a board that has pieces placed
	board := Board{
		{"X", "", ""},
		{"", "O", ""},
		{"", "", ""},
	}

	// Compile and setup
	circuit := &TTTStepCircuit{}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Build state from board position
	state := BoardToTTTODEState(board, "X")
	h := FixFromFloat(0.01)
	w := ComputeTTTStep(state, h)
	assignment := w.ToCircuitAssignment()

	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness creation failed: %v", err)
	}

	proof, err := groth16.Prove(cs, pk, witness)
	if err != nil {
		t.Fatalf("prove failed: %v", err)
	}

	publicWitness, err := witness.Public()
	if err != nil {
		t.Fatalf("public witness extraction failed: %v", err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	t.Log("TTT proof with board state verified successfully")

	// Log the rates to see which transitions are enabled
	t.Log("Transition rates:")
	for tr := 0; tr < TTTNumTransitions; tr++ {
		rate := FixToFloat(w.ActualRates[tr])
		if rate > 0.001 {
			t.Logf("  %s: %.6f", TTTTransitionNames[tr], rate)
		}
	}
}
