package zktictactoe

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

// NumPlaces is the number of places in the tic-tac-toe Petri net.
const NumPlaces = 33

// NumTransitions is the number of transitions in the tic-tac-toe Petri net.
const NumTransitions = 35

// Place indices for the tic-tac-toe Petri net (derived from model).
const (
	// Board positions (cells available to play)
	P00 = 0
	P01 = 1
	P02 = 2
	P10 = 3
	P11 = 4
	P12 = 5
	P20 = 6
	P21 = 7
	P22 = 8

	// X pieces placed on board
	X00 = 9
	X01 = 10
	X02 = 11
	X10 = 12
	X11 = 13
	X12 = 14
	X20 = 15
	X21 = 16
	X22 = 17

	// O pieces placed on board
	O00 = 18
	O01 = 19
	O02 = 20
	O10 = 21
	O11 = 22
	O12 = 23
	O20 = 24
	O21 = 25
	O22 = 26

	// Control places
	PlaceXTurn      = 27
	PlaceOTurn      = 28
	PlaceWinX       = 29
	PlaceWinO       = 30
	PlaceCanReset   = 31
	PlaceGameActive = 32
)

// PlaceNames maps place indices to their IDs from the model.
var PlaceNames = [NumPlaces]string{
	"p00", "p01", "p02", "p10", "p11", "p12", "p20", "p21", "p22",
	"x00", "x01", "x02", "x10", "x11", "x12", "x20", "x21", "x22",
	"o00", "o01", "o02", "o10", "o11", "o12", "o20", "o21", "o22",
	"x_turn", "o_turn", "win_x", "win_o", "can_reset", "game_active",
}

// Transition indices for the tic-tac-toe Petri net.
const (
	TXPlay00 = 0
	TXPlay01 = 1
	TXPlay02 = 2
	TXPlay10 = 3
	TXPlay11 = 4
	TXPlay12 = 5
	TXPlay20 = 6
	TXPlay21 = 7
	TXPlay22 = 8

	TOPlay00 = 9
	TOPlay01 = 10
	TOPlay02 = 11
	TOPlay10 = 12
	TOPlay11 = 13
	TOPlay12 = 14
	TOPlay20 = 15
	TOPlay21 = 16
	TOPlay22 = 17

	TReset = 18

	TXWinRow0 = 19
	TXWinRow1 = 20
	TXWinRow2 = 21
	TXWinCol0 = 22
	TXWinCol1 = 23
	TXWinCol2 = 24
	TXWinDiag = 25
	TXWinAnti = 26

	TOWinRow0 = 27
	TOWinRow1 = 28
	TOWinRow2 = 29
	TOWinCol0 = 30
	TOWinCol1 = 31
	TOWinCol2 = 32
	TOWinDiag = 33
	TOWinAnti = 34
)

// TransitionNames maps transition indices to their IDs from the model.
var TransitionNames = [NumTransitions]string{
	"x_play_00", "x_play_01", "x_play_02", "x_play_10", "x_play_11", "x_play_12", "x_play_20", "x_play_21", "x_play_22",
	"o_play_00", "o_play_01", "o_play_02", "o_play_10", "o_play_11", "o_play_12", "o_play_20", "o_play_21", "o_play_22",
	"reset",
	"x_win_row0", "x_win_row1", "x_win_row2", "x_win_col0", "x_win_col1", "x_win_col2", "x_win_diag", "x_win_anti",
	"o_win_row0", "o_win_row1", "o_win_row2", "o_win_col0", "o_win_col1", "o_win_col2", "o_win_diag", "o_win_anti",
}

// ArcDef represents input and output arcs for a transition.
// All arc weights are 1 in tic-tac-toe.
type ArcDef struct {
	Inputs  []int // places consumed (arc weight = 1)
	Outputs []int // places produced (arc weight = 1)
}

// Topology defines the Petri net arcs (derived from model JSON).
// This is the core data structure that would be auto-generated.
var Topology = [NumTransitions]ArcDef{
	// X plays: consume cell + x_turn, produce x_piece + o_turn
	TXPlay00: {[]int{P00, PlaceXTurn}, []int{X00, PlaceOTurn}},
	TXPlay01: {[]int{P01, PlaceXTurn}, []int{X01, PlaceOTurn}},
	TXPlay02: {[]int{P02, PlaceXTurn}, []int{X02, PlaceOTurn}},
	TXPlay10: {[]int{P10, PlaceXTurn}, []int{X10, PlaceOTurn}},
	TXPlay11: {[]int{P11, PlaceXTurn}, []int{X11, PlaceOTurn}},
	TXPlay12: {[]int{P12, PlaceXTurn}, []int{X12, PlaceOTurn}},
	TXPlay20: {[]int{P20, PlaceXTurn}, []int{X20, PlaceOTurn}},
	TXPlay21: {[]int{P21, PlaceXTurn}, []int{X21, PlaceOTurn}},
	TXPlay22: {[]int{P22, PlaceXTurn}, []int{X22, PlaceOTurn}},

	// O plays: consume cell + o_turn, produce o_piece + x_turn
	TOPlay00: {[]int{P00, PlaceOTurn}, []int{O00, PlaceXTurn}},
	TOPlay01: {[]int{P01, PlaceOTurn}, []int{O01, PlaceXTurn}},
	TOPlay02: {[]int{P02, PlaceOTurn}, []int{O02, PlaceXTurn}},
	TOPlay10: {[]int{P10, PlaceOTurn}, []int{O10, PlaceXTurn}},
	TOPlay11: {[]int{P11, PlaceOTurn}, []int{O11, PlaceXTurn}},
	TOPlay12: {[]int{P12, PlaceOTurn}, []int{O12, PlaceXTurn}},
	TOPlay20: {[]int{P20, PlaceOTurn}, []int{O20, PlaceXTurn}},
	TOPlay21: {[]int{P21, PlaceOTurn}, []int{O21, PlaceXTurn}},
	TOPlay22: {[]int{P22, PlaceOTurn}, []int{O22, PlaceXTurn}},

	// Reset: consume can_reset, produce can_reset (no-op for now)
	TReset: {[]int{PlaceCanReset}, []int{PlaceCanReset}},

	// X wins: consume 3 x_pieces + game_active, produce win_x + return pieces
	TXWinRow0: {[]int{X00, X01, X02, PlaceGameActive}, []int{PlaceWinX, X00, X01, X02}},
	TXWinRow1: {[]int{X10, X11, X12, PlaceGameActive}, []int{PlaceWinX, X10, X11, X12}},
	TXWinRow2: {[]int{X20, X21, X22, PlaceGameActive}, []int{PlaceWinX, X20, X21, X22}},
	TXWinCol0: {[]int{X00, X10, X20, PlaceGameActive}, []int{PlaceWinX, X00, X10, X20}},
	TXWinCol1: {[]int{X01, X11, X21, PlaceGameActive}, []int{PlaceWinX, X01, X11, X21}},
	TXWinCol2: {[]int{X02, X12, X22, PlaceGameActive}, []int{PlaceWinX, X02, X12, X22}},
	TXWinDiag: {[]int{X00, X11, X22, PlaceGameActive}, []int{PlaceWinX, X00, X11, X22}},
	TXWinAnti: {[]int{X02, X11, X20, PlaceGameActive}, []int{PlaceWinX, X02, X11, X20}},

	// O wins: consume 3 o_pieces + game_active, produce win_o + return pieces
	TOWinRow0: {[]int{O00, O01, O02, PlaceGameActive}, []int{PlaceWinO, O00, O01, O02}},
	TOWinRow1: {[]int{O10, O11, O12, PlaceGameActive}, []int{PlaceWinO, O10, O11, O12}},
	TOWinRow2: {[]int{O20, O21, O22, PlaceGameActive}, []int{PlaceWinO, O20, O21, O22}},
	TOWinCol0: {[]int{O00, O10, O20, PlaceGameActive}, []int{PlaceWinO, O00, O10, O20}},
	TOWinCol1: {[]int{O01, O11, O21, PlaceGameActive}, []int{PlaceWinO, O01, O11, O21}},
	TOWinCol2: {[]int{O02, O12, O22, PlaceGameActive}, []int{PlaceWinO, O02, O12, O22}},
	TOWinDiag: {[]int{O00, O11, O22, PlaceGameActive}, []int{PlaceWinO, O00, O11, O22}},
	TOWinAnti: {[]int{O02, O11, O20, PlaceGameActive}, []int{PlaceWinO, O02, O11, O20}},
}

// Marking represents the token counts for all places in the Petri net.
type Marking [NumPlaces]uint8

// InitialMarking returns the initial marking from the model.
func InitialMarking() Marking {
	var m Marking
	// Board cells start with 1 token each (available)
	for i := P00; i <= P22; i++ {
		m[i] = 1
	}
	// Control places
	m[PlaceXTurn] = 1
	m[PlaceCanReset] = 1
	m[PlaceGameActive] = 1
	return m
}

// ComputeMarkingRoot computes a MiMC hash of the full marking.
func ComputeMarkingRoot(m Marking) *big.Int {
	h := mimc.NewMiMC()
	for _, tokens := range m {
		var elem fr.Element
		elem.SetUint64(uint64(tokens))
		b := elem.Bytes()
		h.Write(b[:])
	}
	sum := h.Sum(nil)
	return new(big.Int).SetBytes(sum)
}

// IsEnabled checks if a transition can fire with the current marking.
func IsEnabled(m Marking, t int) bool {
	if t < 0 || t >= NumTransitions {
		return false
	}
	for _, p := range Topology[t].Inputs {
		if m[p] < 1 {
			return false
		}
	}
	return true
}

// Fire applies a transition and returns the new marking.
// Returns an error if the transition is not enabled.
func Fire(m Marking, t int) (Marking, error) {
	if !IsEnabled(m, t) {
		return m, fmt.Errorf("transition %s is not enabled", TransitionNames[t])
	}

	newM := m
	// Consume from input places
	for _, p := range Topology[t].Inputs {
		newM[p]--
	}
	// Produce to output places
	for _, p := range Topology[t].Outputs {
		newM[p]++
	}
	return newM, nil
}

// EnabledTransitions returns all transitions that can fire with the current marking.
func EnabledTransitions(m Marking) []int {
	var enabled []int
	for t := 0; t < NumTransitions; t++ {
		if IsEnabled(m, t) {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// HasWinner returns the winner (PlaceWinX or PlaceWinO) or -1 if no winner.
func HasWinner(m Marking) int {
	if m[PlaceWinX] > 0 {
		return PlaceWinX
	}
	if m[PlaceWinO] > 0 {
		return PlaceWinO
	}
	return -1
}

// TransitionForMove returns the transition index for a player move at position.
// player: 1=X, 2=O; pos: 0-8 (row*3+col)
func TransitionForMove(player uint8, pos int) int {
	if pos < 0 || pos > 8 {
		return -1
	}
	if player == 1 { // X
		return TXPlay00 + pos
	}
	if player == 2 { // O
		return TOPlay00 + pos
	}
	return -1
}
