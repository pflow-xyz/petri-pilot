package zktictactoe

import (
	"fmt"
	"math/big"
)

// Game tracks the full state of a tic-tac-toe match including state roots
// for ZK proof generation.
type Game struct {
	Board     BoardState
	TurnCount int
	Roots     []*big.Int // state root after each move (index 0 = initial empty board)
}

// NewGame creates a new game with an empty board.
func NewGame() *Game {
	var board BoardState
	root := ComputeStateRoot(board)
	return &Game{
		Board:     board,
		TurnCount: 0,
		Roots:     []*big.Int{root},
	}
}

// CurrentPlayer returns whose turn it is (X=1 or O=2).
func (g *Game) CurrentPlayer() uint8 {
	if g.TurnCount%2 == 0 {
		return X
	}
	return O
}

// CurrentRoot returns the current board's state root.
func (g *Game) CurrentRoot() *big.Int {
	return g.Roots[len(g.Roots)-1]
}

// MoveWitness contains all values needed to generate a ZK proof for a move.
type MoveWitness struct {
	PreStateRoot  *big.Int
	PostStateRoot *big.Int
	Position      int
	Player        uint8
	Board         BoardState // pre-move board
	TurnCount     int
}

// MakeMove validates and applies a move, returning the witness for proof generation.
func (g *Game) MakeMove(pos int) (*MoveWitness, error) {
	player := g.CurrentPlayer()
	preBoard := g.Board
	preRoot := g.CurrentRoot()

	newBoard, postRoot, err := ApplyMove(g.Board, pos, player)
	if err != nil {
		return nil, fmt.Errorf("invalid move: %w", err)
	}

	witness := &MoveWitness{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Position:      pos,
		Player:        player,
		Board:         preBoard,
		TurnCount:     g.TurnCount,
	}

	g.Board = newBoard
	g.TurnCount++
	g.Roots = append(g.Roots, postRoot)

	return witness, nil
}

// WinWitness contains all values needed to generate a ZK proof of a win.
type WinWitness struct {
	StateRoot *big.Int
	Winner    uint8
	Board     BoardState
}

// CheckWin checks if the current board has a winner and returns the witness.
// Returns nil if no winner.
func (g *Game) CheckWin() *WinWitness {
	winner := CheckWinner(g.Board)
	if winner == Empty {
		return nil
	}
	return &WinWitness{
		StateRoot: g.CurrentRoot(),
		Winner:    winner,
		Board:     g.Board,
	}
}

// IsOver returns true if the game has a winner or all cells are filled.
func (g *Game) IsOver() bool {
	if CheckWinner(g.Board) != Empty {
		return true
	}
	for _, cell := range g.Board {
		if cell == Empty {
			return false
		}
	}
	return true // draw
}

// ToMoveAssignment converts a MoveWitness to a circuit assignment.
func (w *MoveWitness) ToMoveAssignment() *MoveCircuit {
	c := &MoveCircuit{
		PreStateRoot:  w.PreStateRoot,
		PostStateRoot: w.PostStateRoot,
		Position:      w.Position,
		Player:        w.Player,
		TurnCount:     w.TurnCount,
	}
	for i := 0; i < 9; i++ {
		c.Board[i] = int(w.Board[i])
	}
	return c
}

// ToWinAssignment converts a WinWitness to a circuit assignment.
func (w *WinWitness) ToWinAssignment() *WinCircuit {
	c := &WinCircuit{
		StateRoot: w.StateRoot,
		Winner:    w.Winner,
	}
	for i := 0; i < 9; i++ {
		c.Board[i] = int(w.Board[i])
	}
	return c
}
