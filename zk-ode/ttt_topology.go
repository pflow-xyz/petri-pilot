package zkode

import "math/big"

// TTT topology constants for the full tic-tac-toe Petri net.
// 32 places: 9 empty cells, 9 X pieces, 9 O pieces, x_turn, o_turn, win_x, win_o, game_active
// 34 transitions: 9 x_play, 9 o_play, 8 x_win, 8 o_win

const TTTNumPlaces = 32
const TTTNumTransitions = 34
const TTTMaxInputsPerTransition = 5

// Place indices: empty cells (0-8)
const (
	P00 = iota
	P01
	P02
	P10
	P11
	P12
	P20
	P21
	P22
)

// Place indices: X pieces (9-17)
const (
	X00 = 9 + iota
	X01
	X02
	X10
	X11
	X12
	X20
	X21
	X22
)

// Place indices: O pieces (18-26)
const (
	O00 = 18 + iota
	O01
	O02
	O10
	O11
	O12
	O20
	O21
	O22
)

// Place indices: control places (27-31)
const (
	XTurn      = 27
	OTurn      = 28
	WinX       = 29
	WinO       = 30
	GameActive = 31
)

// TTTPlaceNames maps indices to human-readable names.
var TTTPlaceNames = [TTTNumPlaces]string{
	"p00", "p01", "p02", "p10", "p11", "p12", "p20", "p21", "p22",
	"x00", "x01", "x02", "x10", "x11", "x12", "x20", "x21", "x22",
	"o00", "o01", "o02", "o10", "o11", "o12", "o20", "o21", "o22",
	"x_turn", "o_turn", "win_x", "win_o", "game_active",
}

// Transition indices: X play (0-8)
const (
	TXPlay00 = iota
	TXPlay01
	TXPlay02
	TXPlay10
	TXPlay11
	TXPlay12
	TXPlay20
	TXPlay21
	TXPlay22
)

// Transition indices: O play (9-17)
const (
	TOPlay00 = 9 + iota
	TOPlay01
	TOPlay02
	TOPlay10
	TOPlay11
	TOPlay12
	TOPlay20
	TOPlay21
	TOPlay22
)

// Transition indices: X win (18-25)
const (
	TXWinRow0 = 18 + iota
	TXWinRow1
	TXWinRow2
	TXWinCol0
	TXWinCol1
	TXWinCol2
	TXWinDiag
	TXWinAnti
)

// Transition indices: O win (26-33)
const (
	TOWinRow0 = 26 + iota
	TOWinRow1
	TOWinRow2
	TOWinCol0
	TOWinCol1
	TOWinCol2
	TOWinDiag
	TOWinAnti
)

// TTTTransitionNames maps transition indices to human-readable names.
var TTTTransitionNames = [TTTNumTransitions]string{
	"x_play_00", "x_play_01", "x_play_02", "x_play_10", "x_play_11", "x_play_12", "x_play_20", "x_play_21", "x_play_22",
	"o_play_00", "o_play_01", "o_play_02", "o_play_10", "o_play_11", "o_play_12", "o_play_20", "o_play_21", "o_play_22",
	"x_win_row0", "x_win_row1", "x_win_row2", "x_win_col0", "x_win_col1", "x_win_col2", "x_win_diag", "x_win_anti",
	"o_win_row0", "o_win_row1", "o_win_row2", "o_win_col0", "o_win_col1", "o_win_col2", "o_win_diag", "o_win_anti",
}

// TTTRateConstants holds the rate constant k[t] for each transition.
// Play transitions use position-based strategic weights:
//   - Center (1,1): k=4 (participates in 4 win lines: row, col, diag, anti-diag)
//   - Corners: k=3 (participates in 3 win lines: row, col, one diagonal)
//   - Edges: k=2 (participates in 2 win lines: row, col)
//   - Win transitions: k=1 (unchanged)
//
// Rate formula: rate[t] = k[t] * product(marking[inputs[t]])
var TTTRateConstants [TTTNumTransitions]*big.Int

// TTTStoichiometry is the net-change matrix S[place][transition].
// S = Output - Input for each (place, transition) pair.
//
// x_play_RC: consumes cell + x_turn, produces piece + o_turn
// o_play_RC: consumes cell + o_turn, produces piece + x_turn
// x_win_pattern: consumes o_turn + game_active, produces win_x (pieces net to 0)
// o_win_pattern: consumes x_turn + game_active, produces win_o (pieces net to 0)
var TTTStoichiometry [TTTNumPlaces][TTTNumTransitions]int

// TTTTransitionInputs lists the input place indices for each transition.
// Mass-action rate = product(marking[input]) for all inputs.
var TTTTransitionInputs [TTTNumTransitions][]int

// TTTNumInputs is the number of input places per transition.
var TTTNumInputs [TTTNumTransitions]int

// TTTWinLines defines the 8 winning lines (rows, cols, diagonals) as cell indices 0-8.
var TTTWinLines = [8][3]int{
	{0, 1, 2}, // row 0
	{3, 4, 5}, // row 1
	{6, 7, 8}, // row 2
	{0, 3, 6}, // col 0
	{1, 4, 7}, // col 1
	{2, 5, 8}, // col 2
	{0, 4, 8}, // diag
	{2, 4, 6}, // anti-diag
}

// TTTCellWinLines maps each cell (0-8) to the indices of win lines passing through it.
var TTTCellWinLines [9][]int

func init() {
	initTTTCellWinLines()
	initTTTRateConstants()
	initTTTStoichiometry()
	initTTTTransitionInputs()
}

func initTTTCellWinLines() {
	for cell := 0; cell < 9; cell++ {
		for lineIdx, line := range TTTWinLines {
			for _, c := range line {
				if c == cell {
					TTTCellWinLines[cell] = append(TTTCellWinLines[cell], lineIdx)
					break
				}
			}
		}
	}
}

func initTTTStoichiometry() {
	// X play transitions: consume cell + x_turn, produce piece + o_turn
	for i := 0; i < 9; i++ {
		t := TXPlay00 + i
		cell := P00 + i
		piece := X00 + i
		TTTStoichiometry[cell][t] = -1
		TTTStoichiometry[XTurn][t] = -1
		TTTStoichiometry[piece][t] = +1
		TTTStoichiometry[OTurn][t] = +1
	}

	// O play transitions: consume cell + o_turn, produce piece + x_turn
	for i := 0; i < 9; i++ {
		t := TOPlay00 + i
		cell := P00 + i
		piece := O00 + i
		TTTStoichiometry[cell][t] = -1
		TTTStoichiometry[OTurn][t] = -1
		TTTStoichiometry[piece][t] = +1
		TTTStoichiometry[XTurn][t] = +1
	}

	// Win pattern piece indices (3 pieces per pattern)
	winPatterns := [8][3]int{
		{0, 1, 2}, // row0
		{3, 4, 5}, // row1
		{6, 7, 8}, // row2
		{0, 3, 6}, // col0
		{1, 4, 7}, // col1
		{2, 5, 8}, // col2
		{0, 4, 8}, // diag
		{2, 4, 6}, // anti-diag
	}

	// X win transitions: pieces net to 0, consume o_turn + game_active, produce win_x
	for i := 0; i < 8; i++ {
		t := TXWinRow0 + i
		// Pieces are consumed and returned (net 0), so no stoichiometry entry
		TTTStoichiometry[OTurn][t] = -1
		TTTStoichiometry[GameActive][t] = -1
		TTTStoichiometry[WinX][t] = +1
		_ = winPatterns[i] // pieces have net 0 change
	}

	// O win transitions: pieces net to 0, consume x_turn + game_active, produce win_o
	for i := 0; i < 8; i++ {
		t := TOWinRow0 + i
		TTTStoichiometry[XTurn][t] = -1
		TTTStoichiometry[GameActive][t] = -1
		TTTStoichiometry[WinO][t] = +1
	}
}

func initTTTTransitionInputs() {
	// X play: inputs are [cell, x_turn]
	for i := 0; i < 9; i++ {
		t := TXPlay00 + i
		cell := P00 + i
		TTTTransitionInputs[t] = []int{cell, XTurn}
		TTTNumInputs[t] = 2
	}

	// O play: inputs are [cell, o_turn]
	for i := 0; i < 9; i++ {
		t := TOPlay00 + i
		cell := P00 + i
		TTTTransitionInputs[t] = []int{cell, OTurn}
		TTTNumInputs[t] = 2
	}

	// Win pattern piece indices
	winPatterns := [8][3]int{
		{0, 1, 2}, // row0
		{3, 4, 5}, // row1
		{6, 7, 8}, // row2
		{0, 3, 6}, // col0
		{1, 4, 7}, // col1
		{2, 5, 8}, // col2
		{0, 4, 8}, // diag
		{2, 4, 6}, // anti-diag
	}

	// X win: inputs are [piece0, piece1, piece2, o_turn, game_active]
	for i := 0; i < 8; i++ {
		t := TXWinRow0 + i
		p := winPatterns[i]
		TTTTransitionInputs[t] = []int{X00 + p[0], X00 + p[1], X00 + p[2], OTurn, GameActive}
		TTTNumInputs[t] = 5
	}

	// O win: inputs are [piece0, piece1, piece2, x_turn, game_active]
	for i := 0; i < 8; i++ {
		t := TOWinRow0 + i
		p := winPatterns[i]
		TTTTransitionInputs[t] = []int{O00 + p[0], O00 + p[1], O00 + p[2], XTurn, GameActive}
		TTTNumInputs[t] = 5
	}
}

func initTTTRateConstants() {
	// Position weights based on number of win lines each cell participates in.
	// Grid positions: 0=corner, 1=edge, 2=corner, 3=edge, 4=center, etc.
	positionWeights := [9]float64{
		3, 2, 3, // (0,0) corner, (0,1) edge, (0,2) corner
		2, 4, 2, // (1,0) edge, (1,1) center, (1,2) edge
		3, 2, 3, // (2,0) corner, (2,1) edge, (2,2) corner
	}

	// X play transitions (0-8): use position weights
	for i := 0; i < 9; i++ {
		TTTRateConstants[TXPlay00+i] = FixFromFloat(positionWeights[i])
	}

	// O play transitions (9-17): use same position weights
	for i := 0; i < 9; i++ {
		TTTRateConstants[TOPlay00+i] = FixFromFloat(positionWeights[i])
	}

	// Win transitions (18-33): k=1
	for i := 0; i < 8; i++ {
		TTTRateConstants[TXWinRow0+i] = FixFromFloat(1.0)
	}
	for i := 0; i < 8; i++ {
		TTTRateConstants[TOWinRow0+i] = FixFromFloat(1.0)
	}
}

// TTTDefaultInitialMarking returns the empty board with X's turn as fixed-point elements.
// Places: all cells=1, all pieces=0, x_turn=1, o_turn=0, win_x=0, win_o=0, game_active=1
func TTTDefaultInitialMarking() [TTTNumPlaces]*big.Int {
	var m [TTTNumPlaces]*big.Int
	zero := FixFromFloat(0.0)
	one := FixFromFloat(1.0)

	// Empty cells: 1.0 each
	for i := P00; i <= P22; i++ {
		m[i] = new(big.Int).Set(one)
	}
	// X pieces: 0
	for i := X00; i <= X22; i++ {
		m[i] = new(big.Int).Set(zero)
	}
	// O pieces: 0
	for i := O00; i <= O22; i++ {
		m[i] = new(big.Int).Set(zero)
	}
	// Control
	m[XTurn] = new(big.Int).Set(one)
	m[OTurn] = new(big.Int).Set(zero)
	m[WinX] = new(big.Int).Set(zero)
	m[WinO] = new(big.Int).Set(zero)
	m[GameActive] = new(big.Int).Set(one)

	return m
}
