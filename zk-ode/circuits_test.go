package zkode

import (
	"math"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestCircuitCompiles(t *testing.T) {
	circuit := &Tsit5StepCircuit{}
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

	// Setup: compile circuit and run trusted setup
	circuit := &Tsit5StepCircuit{}
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
	rates := DefaultRates()
	initial := NewODEState(DefaultInitialMarking())

	w := ComputeStep(initial, h, rates)
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

	// Compile and setup once
	circuit := &Tsit5StepCircuit{}
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
	rates := DefaultRates()
	initial := NewODEState(DefaultInitialMarking())

	steps := ComputeSteps(initial, h, rates, 10)

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
	for p := 0; p < NumPlaces; p++ {
		f := FixToFloat(final.Marking[p])
		t.Logf("  %s: %.6f", PlaceNames[p], f)
	}
}

func TestNativeSolverAccuracy(t *testing.T) {
	// Compare native fixed-point Tsit5 against float64 reference
	// for the cascade A→B→C with k0=k1=1, y0=[1,0,0]
	h := FixFromFloat(0.01)
	rates := DefaultRates()
	state := NewODEState(DefaultInitialMarking())

	// Run 100 steps (t=0 to t=1.0)
	nSteps := 100
	for i := 0; i < nSteps; i++ {
		w := ComputeStep(state, h, rates)
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

	gotA := FixToFloat(state.Marking[PlaceA])
	gotB := FixToFloat(state.Marking[PlaceB])
	gotC := FixToFloat(state.Marking[PlaceC])

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
