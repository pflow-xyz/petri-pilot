package zkode

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestCircuitCompiles(t *testing.T) {
	net := CascadeNet()
	circuit := NewTsit5StepCircuit(net)
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("circuit compilation failed: %v", err)
	}

	t.Logf("Constraints: %d", cs.GetNbConstraints())
	t.Logf("Public variables: %d", cs.GetNbPublicVariables())
	t.Logf("Secret variables: %d", cs.GetNbSecretVariables())
}

func TestSingleStepProof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping proof test in short mode")
	}

	net := CascadeNet()

	// Setup: compile circuit and run trusted setup
	circuit := NewTsit5StepCircuit(net)
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}
	t.Logf("Compiled: %d constraints", cs.GetNbConstraints())

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Compute a single step witness
	h := FixFromFloat(0.01) // small step size
	rates := DefaultRates(net)
	initial := NewODEState(DefaultInitialMarking())

	w := ComputeStep(net, initial, h, rates)
	assignment := w.ToCircuitAssignment()

	// Create witness
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

	t.Log("Single step proof verified successfully")
}

func TestChainedProofs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chained proof test in short mode")
	}

	net := CascadeNet()

	// Compile and setup once
	circuit := NewTsit5StepCircuit(net)
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Run 10 chained steps
	h := FixFromFloat(0.01)
	rates := DefaultRates(net)
	initial := NewODEState(DefaultInitialMarking())

	steps := ComputeSteps(net, initial, h, rates, 10)

	for i, w := range steps {
		// Verify chain: post root of step i-1 == pre root of step i
		if i > 0 {
			prevPost := steps[i-1].PostState.Root
			curPre := w.PreState.Root
			if prevPost.Cmp(curPre) != 0 {
				t.Fatalf("chain broken at step %d: prev post root != cur pre root", i)
			}
		}

		assignment := w.ToCircuitAssignment()
		witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
		if err != nil {
			t.Fatalf("step %d: witness creation failed: %v", i, err)
		}

		proof, err := groth16.Prove(cs, pk, witness)
		if err != nil {
			t.Fatalf("step %d: prove failed: %v", i, err)
		}

		pubWit, err := witness.Public()
		if err != nil {
			t.Fatalf("step %d: public witness failed: %v", i, err)
		}

		err = groth16.Verify(proof, vk, pubWit)
		if err != nil {
			t.Fatalf("step %d: verification failed: %v", i, err)
		}
	}

	t.Logf("All %d chained proofs verified", len(steps))

	// Log final state
	final := steps[len(steps)-1].PostState
	t.Logf("Final marking after %d steps:", len(steps))
	for p := 0; p < net.NumPlaces; p++ {
		f := fixedToFloat(final.Marking[p])
		t.Logf("  %s: %.6f", net.PlaceNames[p], f)
	}
}

func TestNativeSolverAccuracy(t *testing.T) {
	// Compare native fixed-point Tsit5 against float64 reference
	// for the cascade A->B->C with k0=k1=1, y0=[1,0,0]
	net := CascadeNet()
	h := FixFromFloat(0.01)
	rates := DefaultRates(net)
	state := NewODEState(DefaultInitialMarking())

	// Run 100 steps (t=0 to t=1.0)
	nSteps := 100
	for i := 0; i < nSteps; i++ {
		w := ComputeStep(net, state, h, rates)
		state = w.PostState
	}

	// Analytical solution at t=1:
	//   A(t) = e^(-t)
	//   B(t) = t * e^(-t)
	//   C(t) = 1 - (1+t) * e^(-t)
	t1 := 1.0
	exactA := math.Exp(-t1)
	exactB := t1 * math.Exp(-t1)
	exactC := 1 - (1+t1)*math.Exp(-t1)

	gotA := fixedToFloat(state.Marking[0])
	gotB := fixedToFloat(state.Marking[1])
	gotC := fixedToFloat(state.Marking[2])

	t.Logf("At t=1.0 (100 steps of h=0.01):")
	t.Logf("  A: got=%.10f exact=%.10f err=%.2e", gotA, exactA, math.Abs(gotA-exactA))
	t.Logf("  B: got=%.10f exact=%.10f err=%.2e", gotB, exactB, math.Abs(gotB-exactB))
	t.Logf("  C: got=%.10f exact=%.10f err=%.2e", gotC, exactC, math.Abs(gotC-exactC))

	// Tsit5 with h=0.01 should be very accurate (5th order method)
	tol := 1e-8
	if math.Abs(gotA-exactA) > tol {
		t.Errorf("A(1.0) error too large: %e > %e", math.Abs(gotA-exactA), tol)
	}
	if math.Abs(gotB-exactB) > tol {
		t.Errorf("B(1.0) error too large: %e > %e", math.Abs(gotB-exactB), tol)
	}
	if math.Abs(gotC-exactC) > tol {
		t.Errorf("C(1.0) error too large: %e > %e", math.Abs(gotC-exactC), tol)
	}

	// Conservation law: A + B + C = 1 (tokens are conserved in cascade)
	sum := gotA + gotB + gotC
	if math.Abs(sum-1.0) > 1e-10 {
		t.Errorf("conservation violated: A+B+C = %.15f (expected 1.0)", sum)
	}
}

func TestTicTacToeCircuitCompiles(t *testing.T) {
	net := TicTacToeNet()
	circuit := NewTsit5StepCircuit(net)
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("tic-tac-toe circuit compilation failed: %v", err)
	}

	t.Logf("Tic-Tac-Toe circuit:")
	t.Logf("  Places: %d, Transitions: %d", net.NumPlaces, net.NumTransitions)
	t.Logf("  Constraints: %d", cs.GetNbConstraints())
	t.Logf("  Public variables: %d", cs.GetNbPublicVariables())
	t.Logf("  Secret variables: %d", cs.GetNbSecretVariables())
}

func TestTicTacToeProof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping tic-tac-toe proof test in short mode")
	}

	net := TicTacToeNet()

	// Compile and setup
	circuit := NewTsit5StepCircuit(net)
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}
	t.Logf("Compiled: %d constraints", cs.GetNbConstraints())

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Compute a single step
	h := FixFromFloat(0.01)
	rates := DefaultRates(net)
	initial := NewODEState(TicTacToeInitialMarking())

	w := ComputeStep(net, initial, h, rates)
	assignment := w.ToCircuitAssignment()

	// Create witness
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

	t.Log("Tic-tac-toe single step proof verified successfully")
}

func TestExportCascadeVerifier(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping verifier export in short mode")
	}

	sol, err := ExportVerifier(CascadeNet())
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	t.Logf("Exported Solidity verifier (%d bytes)", len(sol))

	// Write to solidity directory for inspection
	outPath := "../solidity/src/Groth16Verifier.sol"
	if err := os.WriteFile(outPath, []byte(sol), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("Written to %s", outPath)

	// Log the function selector for the adapter
	// For cascade: verifyProof(uint256[8],uint256[5]) — 5 = 3 + 2 transitions
	net := CascadeNet()
	numPublicInputs := 3 + net.NumTransitions // preRoot, postRoot, stepSize, rates...
	sig := fmt.Sprintf("verifyProof(uint256[8],uint256[%d])", numPublicInputs)
	t.Logf("gnark verifier function signature: %s", sig)
	t.Logf("Use this signature to compute the selector for Groth16VerifierAdapter")
}

// fixedToFloat converts a fixed-point field element back to float64 for display.
func fixedToFloat(v *big.Int) float64 {
	// If value is in upper half of field, it's negative
	halfField := new(big.Int).Rsh(fieldModulus, 1)
	val := new(big.Int).Set(v)
	if val.Cmp(halfField) > 0 {
		val.Sub(val, fieldModulus) // make it negative
	}

	f := new(big.Float).SetPrec(128).SetInt(val)
	s := new(big.Float).SetPrec(128).SetInt(Scale)
	f.Quo(f, s)
	result, _ := f.Float64()
	return result
}
