package zktictactoe

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

func TestPetriTransitionCircuit_Compiles(t *testing.T) {
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &PetriTransitionCircuit{})
	if err != nil {
		t.Fatalf("PetriTransitionCircuit compilation failed: %v", err)
	}
}

func TestPetriWinCircuit_Compiles(t *testing.T) {
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &PetriWinCircuit{})
	if err != nil {
		t.Fatalf("PetriWinCircuit compilation failed: %v", err)
	}
}

func TestPetriTransitionCircuit_ValidFirstMove(t *testing.T) {
	game := NewPetriGame()

	// X plays center (position 4)
	witness, err := game.MakeMove(4)
	if err != nil {
		t.Fatal(err)
	}

	assignment := witness.ToPetriTransitionAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&PetriTransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestPetriTransitionCircuit_ValidSecondMove(t *testing.T) {
	game := NewPetriGame()

	// X plays center
	_, err := game.MakeMove(4)
	if err != nil {
		t.Fatal(err)
	}

	// O plays top-left
	witness, err := game.MakeMove(0)
	if err != nil {
		t.Fatal(err)
	}

	assignment := witness.ToPetriTransitionAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&PetriTransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestPetriTransitionCircuit_InvalidOccupiedCell(t *testing.T) {
	game := NewPetriGame()

	// X plays center
	_, err := game.MakeMove(4)
	if err != nil {
		t.Fatal(err)
	}

	// O tries to play center (should fail at game level)
	_, err = game.MakeMove(4)
	if err == nil {
		t.Fatal("expected error for playing on occupied cell")
	}
}

func TestPetriTransitionCircuit_WrongTurn(t *testing.T) {
	// Manually craft a bad witness where X tries to play on O's turn
	game := NewPetriGame()

	// X plays center
	_, err := game.MakeMove(4)
	if err != nil {
		t.Fatal(err)
	}

	// Now it's O's turn, but let's try to prove X playing
	preMarking := game.Marking
	preRoot := game.CurrentRoot()

	// Try to fire an X transition (should fail because x_turn is empty)
	_, err = Fire(game.Marking, TXPlay00)
	if err == nil {
		t.Fatal("expected error for wrong turn")
	}

	// Manually craft invalid witness
	badPost := preMarking
	badPost[P00]--       // consume cell
	badPost[PlaceOTurn]-- // wrong - consuming o_turn
	badPost[X00]++
	badPost[PlaceOTurn]++ // producing o_turn again? This is nonsense
	postRoot := ComputeMarkingRoot(badPost)

	assignment := &PetriTransitionCircuit{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Transition:    TXPlay00, // X play
	}
	for i := 0; i < NumPlaces; i++ {
		assignment.PreMarking[i] = int(preMarking[i])
		assignment.PostMarking[i] = int(badPost[i])
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&PetriTransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestPetriTransitionCircuit_FullGameXWins(t *testing.T) {
	// X wins with left column: X plays 0, 3, 6; O plays 1, 4
	game := NewPetriGame()
	assert := test.NewAssert(t)

	moves := []int{0, 1, 3, 4, 6} // X: 0,3,6 (column), O: 1,4

	for i, pos := range moves {
		witness, err := game.MakeMove(pos)
		if err != nil {
			t.Fatalf("move %d (pos %d) failed: %v", i, pos, err)
		}

		assignment := witness.ToPetriTransitionAssignment()
		assert.ProverSucceeded(&PetriTransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))
	}

	// Now X has won via left column - check if win transition is enabled
	if !IsEnabled(game.Marking, TXWinCol0) {
		t.Log("Marking state:")
		t.Log(game.Marking.String())
		t.Fatal("expected x_win_col0 to be enabled")
	}

	// Fire win transition
	winWitness, err := game.FireTransition(TXWinCol0)
	if err != nil {
		t.Fatalf("win transition failed: %v", err)
	}

	assignment := winWitness.ToPetriTransitionAssignment()
	assert.ProverSucceeded(&PetriTransitionCircuit{}, assignment, test.WithCurves(ecc.BN254))

	// Verify winner
	if game.Winner() != 1 {
		t.Fatalf("expected X to win, got %d", game.Winner())
	}
}

func TestPetriWinCircuit_XWins(t *testing.T) {
	game := NewPetriGame()

	// Play to X wins with left column
	moves := []int{0, 1, 3, 4, 6}
	for _, pos := range moves {
		_, err := game.MakeMove(pos)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Fire win transition
	_, err := game.FireTransition(TXWinCol0)
	if err != nil {
		t.Fatal(err)
	}

	// Get win witness
	winWitness := game.GetWinWitness()
	if winWitness == nil {
		t.Fatal("expected win witness")
	}

	assignment := winWitness.ToPetriWinAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&PetriWinCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestPetriWinCircuit_OWins(t *testing.T) {
	game := NewPetriGame()

	// X plays poorly, O wins with middle row
	// X: 0, 6, 2 (corners, no line)
	// O: 3, 4, 5 (middle row)
	moves := []int{0, 3, 6, 4, 2, 5}
	for _, pos := range moves {
		_, err := game.MakeMove(pos)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Fire O win transition
	_, err := game.FireTransition(TOWinRow1)
	if err != nil {
		t.Fatal(err)
	}

	// Verify O won
	if game.Winner() != 2 {
		t.Fatalf("expected O to win, got %d", game.Winner())
	}

	winWitness := game.GetWinWitness()
	assignment := winWitness.ToPetriWinAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&PetriWinCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestPetriWinCircuit_NoWinnerFails(t *testing.T) {
	game := NewPetriGame()

	// Play a few moves but no winner
	_, _ = game.MakeMove(4) // X center
	_, _ = game.MakeMove(0) // O top-left

	// Craft false claim that X won
	assignment := &PetriWinCircuit{
		StateRoot: game.CurrentRoot(),
		Winner:    PlaceWinX, // false claim
	}
	for i := 0; i < NumPlaces; i++ {
		assignment.Marking[i] = int(game.Marking[i])
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&PetriWinCircuit{}, assignment, test.WithCurves(ecc.BN254))
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
	m2[P00] = 0
	m2[X00] = 1
	r3 := ComputeMarkingRoot(m2)

	if r1.Cmp(r3) == 0 {
		t.Fatal("different markings produced same root")
	}
}

func TestTopologyMatchesModel(t *testing.T) {
	// Verify some known transitions match expected behavior

	// X play at 00: consumes p00, x_turn; produces x00, o_turn
	def := Topology[TXPlay00]
	if len(def.Inputs) != 2 || def.Inputs[0] != P00 || def.Inputs[1] != PlaceXTurn {
		t.Errorf("TXPlay00 inputs: got %v, expected [P00, PlaceXTurn]", def.Inputs)
	}
	if len(def.Outputs) != 2 || def.Outputs[0] != X00 || def.Outputs[1] != PlaceOTurn {
		t.Errorf("TXPlay00 outputs: got %v, expected [X00, PlaceOTurn]", def.Outputs)
	}

	// X win row 0: consumes x00, x01, x02, game_active; produces win_x + returns pieces
	defWin := Topology[TXWinRow0]
	if len(defWin.Inputs) != 4 {
		t.Errorf("TXWinRow0 inputs: got %d, expected 4", len(defWin.Inputs))
	}
	// Should contain X00, X01, X02, PlaceGameActive
	inputMap := make(map[int]bool)
	for _, p := range defWin.Inputs {
		inputMap[p] = true
	}
	if !inputMap[X00] || !inputMap[X01] || !inputMap[X02] || !inputMap[PlaceGameActive] {
		t.Errorf("TXWinRow0 inputs missing expected places: %v", defWin.Inputs)
	}
}

func TestInitialMarking(t *testing.T) {
	m := InitialMarking()

	// All board cells should have 1 token
	for i := P00; i <= P22; i++ {
		if m[i] != 1 {
			t.Errorf("expected p%d to have 1 token, got %d", i, m[i])
		}
	}

	// No pieces placed yet
	for i := X00; i <= X22; i++ {
		if m[i] != 0 {
			t.Errorf("expected x piece place %d to be empty", i)
		}
	}
	for i := O00; i <= O22; i++ {
		if m[i] != 0 {
			t.Errorf("expected o piece place %d to be empty", i)
		}
	}

	// Control places
	if m[PlaceXTurn] != 1 {
		t.Error("expected x_turn to have 1 token")
	}
	if m[PlaceOTurn] != 0 {
		t.Error("expected o_turn to be empty")
	}
	if m[PlaceGameActive] != 1 {
		t.Error("expected game_active to have 1 token")
	}
}
