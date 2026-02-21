package zkode

import "math/big"

// NetConfig describes the topology of a Petri net for ODE simulation.
// Sizes are fixed at circuit compile time; the topology is baked into
// the circuit as constants since it doesn't change between proofs.
type NetConfig struct {
	NumPlaces      int
	NumTransitions int
	Stoichiometry  [][]int // [place][transition] = net token change
	InputArcs      [][]int // [transition] = input place indices (for mass-action rates)
	PlaceNames     []string
}

// arcDef describes the input/output arcs of a single transition.
type arcDef struct {
	inputs  []int
	outputs []int
}

// netConfigFromArcs builds a NetConfig from arc definitions.
func netConfigFromArcs(numPlaces int, placeNames []string, arcs []arcDef) NetConfig {
	numTrans := len(arcs)
	stoich := make([][]int, numPlaces)
	for p := range stoich {
		stoich[p] = make([]int, numTrans)
	}
	inputArcs := make([][]int, numTrans)

	for t, arc := range arcs {
		for _, p := range arc.inputs {
			stoich[p][t]--
		}
		for _, p := range arc.outputs {
			stoich[p][t]++
		}
		inputArcs[t] = append([]int{}, arc.inputs...)
	}

	return NetConfig{
		NumPlaces:      numPlaces,
		NumTransitions: numTrans,
		Stoichiometry:  stoich,
		InputArcs:      inputArcs,
		PlaceNames:     placeNames,
	}
}

// CascadeNet returns the 3-place cascade net A -> B -> C.
func CascadeNet() NetConfig {
	return netConfigFromArcs(3, []string{"A", "B", "C"}, []arcDef{
		{inputs: []int{0}, outputs: []int{1}}, // t0: A->B
		{inputs: []int{1}, outputs: []int{2}}, // t1: B->C
	})
}

// DefaultRates returns uniform rate constants (k=1.0) as fixed-point field elements.
func DefaultRates(net NetConfig) []*big.Int {
	rates := make([]*big.Int, net.NumTransitions)
	for t := range rates {
		rates[t] = FixFromFloat(1.0)
	}
	return rates
}

// DefaultInitialMarking returns the cascade initial marking [1, 0, 0] as fixed-point.
func DefaultInitialMarking() []*big.Int {
	return []*big.Int{
		FixFromFloat(1.0),
		FixFromFloat(0.0),
		FixFromFloat(0.0),
	}
}

// TicTacToeNet returns the 33-place, 35-transition tic-tac-toe net.
func TicTacToeNet() NetConfig {
	// Place indices (matching zk-tictactoe/petri_state.go)
	const (
		p00 = 0
		p01 = 1
		p02 = 2
		p10 = 3
		p11 = 4
		p12 = 5
		p20 = 6
		p21 = 7
		p22 = 8

		x00 = 9
		x01 = 10
		x02 = 11
		x10 = 12
		x11 = 13
		x12 = 14
		x20 = 15
		x21 = 16
		x22 = 17

		o00 = 18
		o01 = 19
		o02 = 20
		o10 = 21
		o11 = 22
		o12 = 23
		o20 = 24
		o21 = 25
		o22 = 26

		xTurn      = 27
		oTurn      = 28
		winX       = 29
		winO       = 30
		canReset   = 31
		gameActive = 32
	)

	placeNames := []string{
		"p00", "p01", "p02", "p10", "p11", "p12", "p20", "p21", "p22",
		"x00", "x01", "x02", "x10", "x11", "x12", "x20", "x21", "x22",
		"o00", "o01", "o02", "o10", "o11", "o12", "o20", "o21", "o22",
		"x_turn", "o_turn", "win_x", "win_o", "can_reset", "game_active",
	}

	arcs := []arcDef{
		// X plays (t0-t8): consume cell + x_turn, produce x_piece + o_turn
		{[]int{p00, xTurn}, []int{x00, oTurn}},
		{[]int{p01, xTurn}, []int{x01, oTurn}},
		{[]int{p02, xTurn}, []int{x02, oTurn}},
		{[]int{p10, xTurn}, []int{x10, oTurn}},
		{[]int{p11, xTurn}, []int{x11, oTurn}},
		{[]int{p12, xTurn}, []int{x12, oTurn}},
		{[]int{p20, xTurn}, []int{x20, oTurn}},
		{[]int{p21, xTurn}, []int{x21, oTurn}},
		{[]int{p22, xTurn}, []int{x22, oTurn}},
		// O plays (t9-t17): consume cell + o_turn, produce o_piece + x_turn
		{[]int{p00, oTurn}, []int{o00, xTurn}},
		{[]int{p01, oTurn}, []int{o01, xTurn}},
		{[]int{p02, oTurn}, []int{o02, xTurn}},
		{[]int{p10, oTurn}, []int{o10, xTurn}},
		{[]int{p11, oTurn}, []int{o11, xTurn}},
		{[]int{p12, oTurn}, []int{o12, xTurn}},
		{[]int{p20, oTurn}, []int{o20, xTurn}},
		{[]int{p21, oTurn}, []int{o21, xTurn}},
		{[]int{p22, oTurn}, []int{o22, xTurn}},
		// Reset (t18)
		{[]int{canReset}, []int{canReset}},
		// X wins (t19-t26)
		{[]int{x00, x01, x02, gameActive}, []int{winX, x00, x01, x02}},
		{[]int{x10, x11, x12, gameActive}, []int{winX, x10, x11, x12}},
		{[]int{x20, x21, x22, gameActive}, []int{winX, x20, x21, x22}},
		{[]int{x00, x10, x20, gameActive}, []int{winX, x00, x10, x20}},
		{[]int{x01, x11, x21, gameActive}, []int{winX, x01, x11, x21}},
		{[]int{x02, x12, x22, gameActive}, []int{winX, x02, x12, x22}},
		{[]int{x00, x11, x22, gameActive}, []int{winX, x00, x11, x22}},
		{[]int{x02, x11, x20, gameActive}, []int{winX, x02, x11, x20}},
		// O wins (t27-t34)
		{[]int{o00, o01, o02, gameActive}, []int{winO, o00, o01, o02}},
		{[]int{o10, o11, o12, gameActive}, []int{winO, o10, o11, o12}},
		{[]int{o20, o21, o22, gameActive}, []int{winO, o20, o21, o22}},
		{[]int{o00, o10, o20, gameActive}, []int{winO, o00, o10, o20}},
		{[]int{o01, o11, o21, gameActive}, []int{winO, o01, o11, o21}},
		{[]int{o02, o12, o22, gameActive}, []int{winO, o02, o12, o22}},
		{[]int{o00, o11, o22, gameActive}, []int{winO, o00, o11, o22}},
		{[]int{o02, o11, o20, gameActive}, []int{winO, o02, o11, o20}},
	}

	return netConfigFromArcs(33, placeNames, arcs)
}

// TicTacToeInitialMarking returns the TTT initial marking as fixed-point.
// Board cells (0-8) start with 1 token, plus x_turn, can_reset, game_active.
func TicTacToeInitialMarking() []*big.Int {
	m := make([]*big.Int, 33)
	for i := range m {
		m[i] = FixFromFloat(0.0)
	}
	// Board cells available
	for i := 0; i <= 8; i++ {
		m[i] = FixFromFloat(1.0)
	}
	m[27] = FixFromFloat(1.0) // x_turn
	m[31] = FixFromFloat(1.0) // can_reset
	m[32] = FixFromFloat(1.0) // game_active
	return m
}

// Tsit5 Butcher tableau coefficients, pre-converted to fixed-point field elements.
// Reference: Tsitouras 5(4) RK method.

// tsit5C are the node coefficients.
var tsit5C = [7]*big.Int{
	FixFromFloat(0),
	FixFromFloat(0.161),
	FixFromFloat(0.327),
	FixFromFloat(0.9),
	FixFromFloat(0.9800255409045097),
	FixFromFloat(1),
	FixFromFloat(1),
}

// tsit5A is the Runge-Kutta coefficient matrix. Only lower-triangular entries
// are non-zero. Stored as a ragged array matching the reference implementation.
var tsit5A = [7][]*big.Int{
	{},
	{FixFromFloat(0.161)},
	{FixFromFloat(-0.008480655492356924), FixFromFloat(0.335480655492357)},
	{FixFromFloat(2.8971530571054935), FixFromFloat(-6.359448489975075), FixFromFloat(4.362295432869581)},
	{FixFromFloat(5.325864828439257), FixFromFloat(-11.748883564062828), FixFromFloat(7.4955393428898365), FixFromFloat(-0.09249506636175525)},
	{FixFromFloat(5.86145544294642), FixFromFloat(-12.92096931784711), FixFromFloat(8.159367898576159), FixFromFloat(-0.071584973281401), FixFromFloat(-0.028269050394068383)},
	{FixFromFloat(0.09646076681806523), FixFromFloat(0.01), FixFromFloat(0.4798896504144996), FixFromFloat(1.379008574103742), FixFromFloat(-3.290069515436081), FixFromFloat(2.324710524099774)},
}

// tsit5B are the 5th-order solution weights.
var tsit5B = [7]*big.Int{
	FixFromFloat(0.09646076681806523),
	FixFromFloat(0.01),
	FixFromFloat(0.4798896504144996),
	FixFromFloat(1.379008574103742),
	FixFromFloat(-3.290069515436081),
	FixFromFloat(2.324710524099774),
	FixFromFloat(0),
}
