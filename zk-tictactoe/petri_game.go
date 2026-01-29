package zktictactoe

import (
	"fmt"
	"math/big"
)

// PetriGame tracks the full Petri net state for ZK proof generation.
// Unlike Game which only tracks the board, this tracks all 33 places.
type PetriGame struct {
	Marking Marking
	Roots   []*big.Int // state root after each transition
}

// NewPetriGame creates a new game with the initial Petri net marking.
func NewPetriGame() *PetriGame {
	m := InitialMarking()
	root := ComputeMarkingRoot(m)
	return &PetriGame{
		Marking: m,
		Roots:   []*big.Int{root},
	}
}

// CurrentRoot returns the current marking's state root.
func (g *PetriGame) CurrentRoot() *big.Int {
	return g.Roots[len(g.Roots)-1]
}

// CurrentPlayer returns whose turn it is (1=X, 2=O) based on Petri net state.
func (g *PetriGame) CurrentPlayer() uint8 {
	if g.Marking[PlaceXTurn] > 0 {
		return 1 // X
	}
	if g.Marking[PlaceOTurn] > 0 {
		return 2 // O
	}
	return 0 // game over or invalid state
}

// PetriTransitionWitness contains all values needed to generate a ZK proof.
type PetriTransitionWitness struct {
	PreStateRoot  *big.Int
	PostStateRoot *big.Int
	Transition    int
	PreMarking    Marking
	PostMarking   Marking
}

// FireTransition fires a transition and returns the witness for proof generation.
func (g *PetriGame) FireTransition(t int) (*PetriTransitionWitness, error) {
	if t < 0 || t >= NumTransitions {
		return nil, fmt.Errorf("invalid transition index: %d", t)
	}

	preMarking := g.Marking
	preRoot := g.CurrentRoot()

	newMarking, err := Fire(g.Marking, t)
	if err != nil {
		return nil, fmt.Errorf("transition %s failed: %w", TransitionNames[t], err)
	}

	postRoot := ComputeMarkingRoot(newMarking)

	witness := &PetriTransitionWitness{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Transition:    t,
		PreMarking:    preMarking,
		PostMarking:   newMarking,
	}

	g.Marking = newMarking
	g.Roots = append(g.Roots, postRoot)

	return witness, nil
}

// MakeMove is a convenience method that fires the appropriate play transition.
// pos: 0-8 (row*3+col)
func (g *PetriGame) MakeMove(pos int) (*PetriTransitionWitness, error) {
	player := g.CurrentPlayer()
	if player == 0 {
		return nil, fmt.Errorf("game is over, no current player")
	}

	t := TransitionForMove(player, pos)
	if t < 0 {
		return nil, fmt.Errorf("invalid position %d for player %d", pos, player)
	}

	return g.FireTransition(t)
}

// CheckWin checks if any win transition is enabled and fires it.
// Returns the witness if a win occurred, nil otherwise.
func (g *PetriGame) CheckWin() (*PetriTransitionWitness, error) {
	// Check X win transitions
	for t := TXWinRow0; t <= TXWinAnti; t++ {
		if IsEnabled(g.Marking, t) {
			return g.FireTransition(t)
		}
	}
	// Check O win transitions
	for t := TOWinRow0; t <= TOWinAnti; t++ {
		if IsEnabled(g.Marking, t) {
			return g.FireTransition(t)
		}
	}
	return nil, nil
}

// Winner returns the winner (1=X, 2=O) or 0 if no winner yet.
func (g *PetriGame) Winner() uint8 {
	if g.Marking[PlaceWinX] > 0 {
		return 1
	}
	if g.Marking[PlaceWinO] > 0 {
		return 2
	}
	return 0
}

// IsGameActive returns true if the game is still in progress.
func (g *PetriGame) IsGameActive() bool {
	return g.Marking[PlaceGameActive] > 0
}

// EnabledMoves returns the positions (0-8) where the current player can move.
func (g *PetriGame) EnabledMoves() []int {
	var moves []int
	player := g.CurrentPlayer()
	if player == 0 {
		return moves
	}

	for pos := 0; pos < 9; pos++ {
		t := TransitionForMove(player, pos)
		if IsEnabled(g.Marking, t) {
			moves = append(moves, pos)
		}
	}
	return moves
}

// PetriWinWitness contains values for proving a win occurred.
type PetriWinWitness struct {
	StateRoot *big.Int
	Winner    int // PlaceWinX or PlaceWinO
	Marking   Marking
}

// GetWinWitness returns a witness for proving the current winner.
// Returns nil if no winner.
func (g *PetriGame) GetWinWitness() *PetriWinWitness {
	if g.Marking[PlaceWinX] > 0 {
		return &PetriWinWitness{
			StateRoot: g.CurrentRoot(),
			Winner:    PlaceWinX,
			Marking:   g.Marking,
		}
	}
	if g.Marking[PlaceWinO] > 0 {
		return &PetriWinWitness{
			StateRoot: g.CurrentRoot(),
			Winner:    PlaceWinO,
			Marking:   g.Marking,
		}
	}
	return nil
}

// ToPetriTransitionAssignment converts a witness to a circuit assignment.
func (w *PetriTransitionWitness) ToPetriTransitionAssignment() *PetriTransitionCircuit {
	c := &PetriTransitionCircuit{
		PreStateRoot:  w.PreStateRoot,
		PostStateRoot: w.PostStateRoot,
		Transition:    w.Transition,
	}
	for i := 0; i < NumPlaces; i++ {
		c.PreMarking[i] = int(w.PreMarking[i])
		c.PostMarking[i] = int(w.PostMarking[i])
	}
	return c
}

// ToPetriWinAssignment converts a witness to a circuit assignment.
func (w *PetriWinWitness) ToPetriWinAssignment() *PetriWinCircuit {
	c := &PetriWinCircuit{
		StateRoot: w.StateRoot,
		Winner:    w.Winner,
	}
	for i := 0; i < NumPlaces; i++ {
		c.Marking[i] = int(w.Marking[i])
	}
	return c
}

// String returns a human-readable representation of the marking.
func (m Marking) String() string {
	// Show the board state
	board := [3][3]rune{}
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			pos := row*3 + col
			if m[X00+pos] > 0 {
				board[row][col] = 'X'
			} else if m[O00+pos] > 0 {
				board[row][col] = 'O'
			} else if m[P00+pos] > 0 {
				board[row][col] = '.'
			} else {
				board[row][col] = '?'
			}
		}
	}

	s := fmt.Sprintf(" %c | %c | %c \n", board[0][0], board[0][1], board[0][2])
	s += "-----------\n"
	s += fmt.Sprintf(" %c | %c | %c \n", board[1][0], board[1][1], board[1][2])
	s += "-----------\n"
	s += fmt.Sprintf(" %c | %c | %c \n", board[2][0], board[2][1], board[2][2])

	// Show control state
	if m[PlaceXTurn] > 0 {
		s += "Turn: X\n"
	} else if m[PlaceOTurn] > 0 {
		s += "Turn: O\n"
	}
	if m[PlaceWinX] > 0 {
		s += "Winner: X\n"
	}
	if m[PlaceWinO] > 0 {
		s += "Winner: O\n"
	}

	return s
}
