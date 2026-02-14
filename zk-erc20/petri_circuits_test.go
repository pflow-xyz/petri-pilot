package zkerc20

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

func TestERC20TransitionCircuit_Compiles(t *testing.T) {
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &ERC20TransitionCircuit{})
	if err != nil {
		t.Fatalf("ERC20TransitionCircuit compilation failed: %v", err)
	}
}

func TestERC20InvariantCircuit_Compiles(t *testing.T) {
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &ERC20InvariantCircuit{})
	if err != nil {
		t.Fatalf("ERC20InvariantCircuit compilation failed: %v", err)
	}
}

func TestERC20TransitionCircuit_ValidMint(t *testing.T) {
	ts := NewTokenState()

	// Mint 1000 tokens to Alice
	witness, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	assignment := witness.ToTransitionAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestERC20TransitionCircuit_ValidTransfer(t *testing.T) {
	ts := NewTokenState()

	// Mint 1000 to Alice
	_, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Alice transfers 100 to Bob
	witness, err := ts.Transfer(0, 1, 100)
	if err != nil {
		t.Fatal(err)
	}

	assignment := witness.ToTransitionAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestERC20TransitionCircuit_ValidApproveAndTransferFrom(t *testing.T) {
	ts := NewTokenState()
	assert := test.NewAssert(t)

	// Mint 1000 to Alice
	_, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Alice approves Bob to spend 500
	approveWitness, err := ts.Approve(0, 1, 500)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, approveWitness.ToTransitionAssignment(), test.WithCurves(ecc.BN254))

	// Bob spends 200 from Alice's allowance
	tfWitness, err := ts.TransferFrom(0, 1, 1, 200)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, tfWitness.ToTransitionAssignment(), test.WithCurves(ecc.BN254))

	// Verify state
	if ts.Marking[PlaceBalance0] != 800 {
		t.Errorf("expected Alice balance 800, got %d", ts.Marking[PlaceBalance0])
	}
	if ts.Marking[PlaceBalance1] != 200 {
		t.Errorf("expected Bob balance 200, got %d", ts.Marking[PlaceBalance1])
	}
	if ts.Marking[PlaceAllow01] != 300 {
		t.Errorf("expected allowance 300, got %d", ts.Marking[PlaceAllow01])
	}
}

func TestERC20TransitionCircuit_InvalidOverdraft(t *testing.T) {
	ts := NewTokenState()

	// Mint 100 to Alice
	_, err := ts.Mint(0, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Try to transfer 200 (overdraft) — should fail at Fire level
	_, err = ts.Transfer(0, 1, 200)
	if err == nil {
		t.Fatal("expected error for overdraft")
	}
}

func TestERC20TransitionCircuit_InvalidOverdraftProverFails(t *testing.T) {
	ts := NewTokenState()

	// Mint 100 to Alice
	_, err := ts.Mint(0, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Manually craft an invalid witness: transfer 200 from Alice who has 100
	preMarking := ts.Marking
	preRoot := ts.CurrentRoot()

	badPost := preMarking
	badPost[PlaceBalance0] -= 200 // underflow
	badPost[PlaceBalance1] += 200
	postRoot := ComputeMarkingRoot(badPost)

	assignment := &ERC20TransitionCircuit{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Transition:    TTransfer01,
		Amount:        200,
	}
	for i := 0; i < NumPlaces; i++ {
		assignment.PreMarking[i] = preMarking[i]
		assignment.PostMarking[i] = badPost[i]
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&ERC20TransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestERC20TransitionCircuit_InvalidAllowance(t *testing.T) {
	ts := NewTokenState()

	// Mint 1000 to Alice
	_, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Bob tries transferFrom without approval — should fail at Fire level
	_, err = ts.TransferFrom(0, 1, 1, 100)
	if err == nil {
		t.Fatal("expected error for unauthorized transferFrom")
	}
}

func TestERC20InvariantCircuit_ConservationAfterOperations(t *testing.T) {
	ts := NewTokenState()
	assert := test.NewAssert(t)

	// Mint 1000 to Alice
	_, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Transfer 300 to Bob
	_, err = ts.Transfer(0, 1, 300)
	if err != nil {
		t.Fatal(err)
	}

	// Verify conservation: balance[0] + balance[1] == totalSupply
	if ts.Marking[PlaceBalance0]+ts.Marking[PlaceBalance1] != ts.Marking[PlaceTotalSupply] {
		t.Fatalf("conservation violated: %d + %d != %d",
			ts.Marking[PlaceBalance0], ts.Marking[PlaceBalance1], ts.Marking[PlaceTotalSupply])
	}

	// Prove invariant
	invWitness := ts.GetInvariantWitness()
	assert.ProverSucceeded(&ERC20InvariantCircuit{}, invWitness.ToInvariantAssignment(), test.WithCurves(ecc.BN254))
}

func TestERC20InvariantCircuit_ViolationFails(t *testing.T) {
	ts := NewTokenState()

	// Mint 1000 to Alice
	_, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Craft a false witness where balances don't sum to totalSupply
	badMarking := ts.Marking
	badMarking[PlaceBalance0] = 999 // off by one
	badRoot := ComputeMarkingRoot(badMarking)

	assignment := &ERC20InvariantCircuit{
		StateRoot: badRoot,
	}
	for i := 0; i < NumPlaces; i++ {
		assignment.Marking[i] = badMarking[i]
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&ERC20InvariantCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestERC20FullScenario(t *testing.T) {
	ts := NewTokenState()
	assert := test.NewAssert(t)

	// 1. Mint 1000 to Alice
	w1, err := ts.Mint(0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, w1.ToTransitionAssignment(), test.WithCurves(ecc.BN254))
	t.Logf("After mint: %s", ts.Marking)

	// 2. Transfer 300 from Alice to Bob
	w2, err := ts.Transfer(0, 1, 300)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, w2.ToTransitionAssignment(), test.WithCurves(ecc.BN254))
	t.Logf("After transfer: %s", ts.Marking)

	// 3. Alice approves Bob for 200
	w3, err := ts.Approve(0, 1, 200)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, w3.ToTransitionAssignment(), test.WithCurves(ecc.BN254))
	t.Logf("After approve: %s", ts.Marking)

	// 4. Bob uses transferFrom to move 150 from Alice
	w4, err := ts.TransferFrom(0, 1, 1, 150)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, w4.ToTransitionAssignment(), test.WithCurves(ecc.BN254))
	t.Logf("After transferFrom: %s", ts.Marking)

	// 5. Bob burns 100
	w5, err := ts.Burn(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	assert.ProverSucceeded(&ERC20TransitionCircuit{}, w5.ToTransitionAssignment(), test.WithCurves(ecc.BN254))
	t.Logf("After burn: %s", ts.Marking)

	// Verify final state
	// Alice: 1000 - 300 - 150 = 550
	// Bob: 300 + 150 - 100 = 350
	// TotalSupply: 1000 - 100 = 900
	// Allowance[0→1]: 200 - 150 = 50
	if ts.Marking[PlaceBalance0] != 550 {
		t.Errorf("Alice balance: expected 550, got %d", ts.Marking[PlaceBalance0])
	}
	if ts.Marking[PlaceBalance1] != 350 {
		t.Errorf("Bob balance: expected 350, got %d", ts.Marking[PlaceBalance1])
	}
	if ts.Marking[PlaceTotalSupply] != 900 {
		t.Errorf("totalSupply: expected 900, got %d", ts.Marking[PlaceTotalSupply])
	}
	if ts.Marking[PlaceAllow01] != 50 {
		t.Errorf("allowance[0→1]: expected 50, got %d", ts.Marking[PlaceAllow01])
	}

	// 6. Verify conservation invariant
	invWitness := ts.GetInvariantWitness()
	assert.ProverSucceeded(&ERC20InvariantCircuit{}, invWitness.ToInvariantAssignment(), test.WithCurves(ecc.BN254))

	// 7. Verify state root chain is unbroken
	if len(ts.Roots) != 6 { // initial + 5 transitions
		t.Errorf("expected 6 roots, got %d", len(ts.Roots))
	}
	for i := 1; i < len(ts.Roots); i++ {
		if ts.Roots[i].Cmp(ts.Roots[i-1]) == 0 {
			t.Errorf("roots %d and %d are identical", i-1, i)
		}
	}
}

func TestMarkingRootConsistency(t *testing.T) {
	m1 := InitialMarking()
	r1 := ComputeMarkingRoot(m1)
	r2 := ComputeMarkingRoot(m1)

	if r1.Cmp(r2) != 0 {
		t.Fatal("marking root not deterministic")
	}

	// Different marking gives different root
	m2 := m1
	m2[PlaceBalance0] = 100
	r3 := ComputeMarkingRoot(m2)

	if r1.Cmp(r3) == 0 {
		t.Fatal("different markings produced same root")
	}
}

func TestTopologyMatchesModel(t *testing.T) {
	// transfer_01: consumes balance_0, produces balance_1
	def := Topology[TTransfer01]
	if len(def.Inputs) != 1 || def.Inputs[0] != PlaceBalance0 {
		t.Errorf("TTransfer01 inputs: got %v, expected [PlaceBalance0]", def.Inputs)
	}
	if len(def.Outputs) != 1 || def.Outputs[0] != PlaceBalance1 {
		t.Errorf("TTransfer01 outputs: got %v, expected [PlaceBalance1]", def.Outputs)
	}
	if def.IsApprove {
		t.Error("TTransfer01 should not be approve")
	}

	// approve_01: allowance_01 in both inputs and outputs, IsApprove=true
	defApprove := Topology[TApprove01]
	if !defApprove.IsApprove {
		t.Error("TApprove01 should be approve")
	}
	if len(defApprove.Inputs) != 1 || defApprove.Inputs[0] != PlaceAllow01 {
		t.Errorf("TApprove01 inputs: got %v, expected [PlaceAllow01]", defApprove.Inputs)
	}

	// mint_0: no inputs, outputs balance_0 and totalSupply
	defMint := Topology[TMint0]
	if len(defMint.Inputs) != 0 {
		t.Errorf("TMint0 inputs: got %v, expected []", defMint.Inputs)
	}
	if len(defMint.Outputs) != 2 {
		t.Errorf("TMint0 outputs: expected 2, got %d", len(defMint.Outputs))
	}

	// transferFrom_01: consumes balance_0 and allowance_01, produces balance_1
	defTF := Topology[TTransferFrom01]
	if len(defTF.Inputs) != 2 {
		t.Errorf("TTransferFrom01 inputs: expected 2, got %d", len(defTF.Inputs))
	}
	if len(defTF.Outputs) != 1 || defTF.Outputs[0] != PlaceBalance1 {
		t.Errorf("TTransferFrom01 outputs: got %v, expected [PlaceBalance1]", defTF.Outputs)
	}

	// burn_0: consumes balance_0 and totalSupply, no outputs
	defBurn := Topology[TBurn0]
	if len(defBurn.Inputs) != 2 {
		t.Errorf("TBurn0 inputs: expected 2, got %d", len(defBurn.Inputs))
	}
	if len(defBurn.Outputs) != 0 {
		t.Errorf("TBurn0 outputs: expected 0, got %d", len(defBurn.Outputs))
	}
}

func TestInitialMarking(t *testing.T) {
	m := InitialMarking()

	// All places should be zero
	for i := 0; i < NumPlaces; i++ {
		if m[i] != 0 {
			t.Errorf("expected place %d (%s) to be 0, got %d", i, PlaceNames[i], m[i])
		}
	}
}
