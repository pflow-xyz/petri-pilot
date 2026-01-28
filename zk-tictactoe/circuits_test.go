package zktictactoe

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

func TestMoveCircuit_ValidFirstMove(t *testing.T) {
	game := NewGame()
	witness, err := game.MakeMove(4) // X plays center
	if err != nil {
		t.Fatal(err)
	}

	assignment := witness.ToMoveAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&MoveCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestMoveCircuit_ValidSecondMove(t *testing.T) {
	game := NewGame()
	_, err := game.MakeMove(4) // X plays center
	if err != nil {
		t.Fatal(err)
	}

	witness, err := game.MakeMove(0) // O plays top-left
	if err != nil {
		t.Fatal(err)
	}

	assignment := witness.ToMoveAssignment()
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&MoveCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestMoveCircuit_InvalidOccupiedCell(t *testing.T) {
	// Try to prove a move on an occupied cell — should fail.
	var board BoardState
	board[4] = X // center already taken

	preRoot := ComputeStateRoot(board)
	// Manually craft a bad witness: claim cell 4 is playable
	badBoard := board
	badBoard[4] = O
	postRoot := ComputeStateRoot(badBoard)

	assignment := &MoveCircuit{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Position:      4,
		Player:        2, // O
		TurnCount:     1,
	}
	for i := 0; i < 9; i++ {
		assignment.Board[i] = int(board[i])
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&MoveCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestMoveCircuit_WrongTurn(t *testing.T) {
	// X tries to play on O's turn.
	var board BoardState
	preRoot := ComputeStateRoot(board)

	board[0] = X // pretend X already played
	postRoot := ComputeStateRoot(board)

	assignment := &MoveCircuit{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Position:      0,
		Player:        1, // X
		TurnCount:     1, // turn 1 should be O's turn
	}
	for i := 0; i < 9; i++ {
		assignment.Board[i] = 0 // empty board
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&MoveCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestWinCircuit_XWinsTopRow(t *testing.T) {
	board := BoardState{X, X, X, O, O, 0, 0, 0, 0}
	root := ComputeStateRoot(board)

	assignment := &WinCircuit{
		StateRoot: root,
		Winner:    X,
	}
	for i := 0; i < 9; i++ {
		assignment.Board[i] = int(board[i])
	}

	assert := test.NewAssert(t)
	assert.ProverSucceeded(&WinCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestWinCircuit_OWinsDiagonal(t *testing.T) {
	board := BoardState{O, X, X, X, O, 0, 0, 0, O}
	root := ComputeStateRoot(board)

	assignment := &WinCircuit{
		StateRoot: root,
		Winner:    O,
	}
	for i := 0; i < 9; i++ {
		assignment.Board[i] = int(board[i])
	}

	assert := test.NewAssert(t)
	assert.ProverSucceeded(&WinCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestWinCircuit_NoWinnerFails(t *testing.T) {
	board := BoardState{X, O, 0, 0, X, 0, 0, 0, 0}
	root := ComputeStateRoot(board)

	assignment := &WinCircuit{
		StateRoot: root,
		Winner:    X, // X hasn't won
	}
	for i := 0; i < 9; i++ {
		assignment.Board[i] = int(board[i])
	}

	assert := test.NewAssert(t)
	assert.ProverFailed(&WinCircuit{}, assignment, test.WithCurves(ecc.BN254))
}

func TestStateRootConsistency(t *testing.T) {
	// Verify that computing the state root twice gives the same result.
	board := BoardState{X, O, X, O, X, O, 0, 0, 0}
	r1 := ComputeStateRoot(board)
	r2 := ComputeStateRoot(board)
	if r1.Cmp(r2) != 0 {
		t.Fatalf("state root not deterministic: %s != %s", r1, r2)
	}

	// Different boards give different roots.
	board2 := BoardState{X, O, X, O, X, 0, O, 0, 0}
	r3 := ComputeStateRoot(board2)
	if r1.Cmp(r3) == 0 {
		t.Fatal("different boards produced same state root")
	}
}

func TestMoveCircuit_Compiles(t *testing.T) {
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &MoveCircuit{})
	if err != nil {
		t.Fatalf("MoveCircuit compilation failed: %v", err)
	}
}

func TestWinCircuit_Compiles(t *testing.T) {
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &WinCircuit{})
	if err != nil {
		t.Fatalf("WinCircuit compilation failed: %v", err)
	}
}

func TestFullGameWithProofs(t *testing.T) {
	// Play a full game: X wins with a column.
	// X: 0, 3, 6 (left column)
	// O: 1, 4
	game := NewGame()
	assert := test.NewAssert(t)

	moves := []int{0, 1, 3, 4, 6} // X wins on move 5 (index 4)
	for _, pos := range moves {
		w, err := game.MakeMove(pos)
		if err != nil {
			t.Fatalf("move %d failed: %v", pos, err)
		}
		assignment := w.ToMoveAssignment()
		assert.ProverSucceeded(&MoveCircuit{}, assignment, test.WithCurves(ecc.BN254))
	}

	// Verify win
	winWitness := game.CheckWin()
	if winWitness == nil {
		t.Fatal("expected X to win")
	}
	if winWitness.Winner != X {
		t.Fatalf("expected winner X, got %d", winWitness.Winner)
	}

	winAssignment := winWitness.ToWinAssignment()
	assert.ProverSucceeded(&WinCircuit{}, winAssignment, test.WithCurves(ecc.BN254))
}
