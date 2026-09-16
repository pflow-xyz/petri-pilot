// The declared Connect Four net and its discrete semantics.
//
// Board: 7 columns (0..6) x 6 rows (0..5), row 0 = bottom (gravity).
//
// Places:
//
//	open_c_r    (42) — the next-open-slot chain per column. open_c_0 starts
//	                    marked; playing at (c,r) consumes open_c_r and, if
//	                    r<5, produces open_c_{r+1} — "you can only play the
//	                    next open slot in a column," which tic-tac-toe never
//	                    needed to model.
//	x_c_r, o_c_r (84) — cell ownership, one place per side per cell.
//	x_turn, o_turn    — whose move (X moves first).
//	game_active       — consumed by whichever win detector fires first, so a
//	                    finished game is an absorbing state (mirrors the
//	                    tic-tac-toe declared net before findings 12/15 pruned
//	                    it from the *evaluation* net — it stays in the
//	                    *declared* net here).
//	win_x, win_o      — the two outcome coordinates.
//
// Transitions:
//
//	x_play_c_r, o_play_c_r (84) — one per cell per side, gated on the
//	                    column's current open slot.
//	{x,o}_win_<line>   (138) — one per side per 4-in-a-row line (69 lines:
//	                    24 horizontal + 21 vertical + 12+12 diagonal),
//	                    catalytic self-loops on the line's 4 cells,
//	                    consuming the opponent's turn token and game_active.
//
// Deliberately NOT modelled: a draw place/transition. Tic-tac-toe's findings
// 12 and 15 found the declared weight-9 draw counter actively harmful under
// mass action (its rate is linear in a token count, not a threshold) and
// removed it from the winning evaluation net entirely; here it is dropped
// from the outset. "Board full, no winner" is simply legalMoves reporting no
// moves with neither win place marked, at both the discrete and the ODE
// layer — the discrete referee (oracle.go) scores that outcome as an exact
// draw (0) with no continuous counter standing in for it.
package main

import (
	"fmt"
	"sort"
)

const (
	Width  = 7
	Height = 6
)

type arc struct {
	from, to string
	weight   int
}

// line is a 4-in-a-row: the coordinates in geometric order (matters only for
// readability; the Petri net and the oracle both treat it as a set).
type line struct {
	id    string
	cells [4][2]int // (col, row)
}

func cellKey(c, r int) string { return fmt.Sprintf("%d_%d", c, r) }

func cellPlace(side string, c, r int) string { return side + "_" + cellKey(c, r) }

func openPlace(c, r int) string { return "open_" + cellKey(c, r) }

func playTrans(side string, c, r int) string { return side + "_play_" + cellKey(c, r) }

// allLines enumerates the 69 four-in-a-row lines on a 7x6 board.
func allLines() []line {
	var out []line
	add := func(id string, cells [4][2]int) { out = append(out, line{id, cells}) }

	// horizontal: 6 rows * 4 starting columns
	for r := 0; r < Height; r++ {
		for c := 0; c <= Width-4; c++ {
			add(fmt.Sprintf("h_%d_%d", r, c), [4][2]int{{c, r}, {c + 1, r}, {c + 2, r}, {c + 3, r}})
		}
	}
	// vertical: 7 cols * 3 starting rows
	for c := 0; c < Width; c++ {
		for r := 0; r <= Height-4; r++ {
			add(fmt.Sprintf("v_%d_%d", c, r), [4][2]int{{c, r}, {c, r + 1}, {c, r + 2}, {c, r + 3}})
		}
	}
	// diagonal up-right (/): 4 starting cols * 3 starting rows
	for c := 0; c <= Width-4; c++ {
		for r := 0; r <= Height-4; r++ {
			add(fmt.Sprintf("d1_%d_%d", c, r), [4][2]int{{c, r}, {c + 1, r + 1}, {c + 2, r + 2}, {c + 3, r + 3}})
		}
	}
	// diagonal down-right (\): 4 starting cols * 3 starting (high) rows
	for c := 0; c <= Width-4; c++ {
		for r := 3; r < Height; r++ {
			add(fmt.Sprintf("d2_%d_%d", c, r), [4][2]int{{c, r}, {c + 1, r - 1}, {c + 2, r - 2}, {c + 3, r - 3}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

var winLines = allLines() // 69 lines, sorted by id

type model struct {
	places      []string
	transitions []string
	rates       map[string]float64
	inputs      map[string][]arc
	outputs     map[string][]arc
	initial     map[string]int
}

func buildModel() *model {
	m := &model{
		rates:   map[string]float64{},
		inputs:  map[string][]arc{},
		outputs: map[string][]arc{},
		initial: map[string]int{},
	}
	addPlace := func(id string, init int) { m.places = append(m.places, id); m.initial[id] = init }
	addT := func(id string, rate float64) { m.transitions = append(m.transitions, id); m.rates[id] = rate }
	in := func(t, p string, w int) { m.inputs[t] = append(m.inputs[t], arc{p, t, w}) }
	out := func(t, p string, w int) { m.outputs[t] = append(m.outputs[t], arc{t, p, w}) }

	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			init := 0
			if r == 0 {
				init = 1
			}
			addPlace(openPlace(c, r), init)
			addPlace(cellPlace("x", c, r), 0)
			addPlace(cellPlace("o", c, r), 0)
		}
	}
	addPlace("x_turn", 1)
	addPlace("o_turn", 0)
	addPlace("game_active", 1)
	addPlace("win_x", 0)
	addPlace("win_o", 0)

	// gravity-chain play transitions
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			for _, side := range []string{"x", "o"} {
				other := "o_turn"
				mine := "x_turn"
				if side == "o" {
					mine, other = "o_turn", "x_turn"
				}
				t := playTrans(side, c, r)
				addT(t, 1)
				in(t, openPlace(c, r), 1)
				in(t, mine, 1)
				out(t, cellPlace(side, c, r), 1)
				out(t, other, 1)
				if r < Height-1 {
					out(t, openPlace(c, r+1), 1)
				}
			}
		}
	}

	// catalytic win detectors, 2 per line (69*2 = 138)
	for _, ln := range winLines {
		for _, side := range []string{"x", "o"} {
			opp := "o_turn"
			winPlace := "win_x"
			if side == "o" {
				opp, winPlace = "x_turn", "win_o"
			}
			t := side + "_win_" + ln.id
			addT(t, 1)
			for _, rc := range ln.cells {
				p := cellPlace(side, rc[0], rc[1])
				in(t, p, 1)
				out(t, p, 1)
			}
			in(t, opp, 1)
			in(t, "game_active", 1)
			out(t, winPlace, 1)
		}
	}
	return m
}

// ---- discrete firing rule (identical pattern to ode-minimax's model.go) ----

type marking map[string]int

func (m *model) start() marking {
	mk := make(marking, len(m.initial))
	for k, v := range m.initial {
		mk[k] = v
	}
	return mk
}

func (m *model) enabled(t string, mk marking) bool {
	for _, a := range m.inputs[t] {
		if mk[a.from] < a.weight {
			return false
		}
	}
	return true
}

func (m *model) fire(t string, mk marking) marking {
	next := make(marking, len(mk))
	for k, v := range mk {
		next[k] = v
	}
	for _, a := range m.inputs[t] {
		next[a.from] -= a.weight
	}
	for _, a := range m.outputs[t] {
		next[a.to] += a.weight
	}
	return next
}

var playerOwned = func() map[string]bool {
	owned := map[string]bool{}
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			owned[playTrans("x", c, r)] = true
			owned[playTrans("o", c, r)] = true
		}
	}
	return owned
}()

// fireHouse fires unowned transitions (win detectors) to quiescence.
func (m *model) fireHouse(mk marking) marking {
	for {
		fired := false
		for _, t := range m.transitions {
			if !playerOwned[t] && m.enabled(t, mk) {
				mk = m.fire(t, mk)
				fired = true
			}
		}
		if !fired {
			return mk
		}
	}
}

// legalMoves returns the mover's enabled play transitions (at most one per
// column — the gravity chain guarantees only the current open slot is
// enabled), whether the mover maximizes (X), and ok=false if the game is
// over (a win is already marked, or the board is full).
func (m *model) legalMoves(mk marking) (moves []string, maximizes, ok bool) {
	switch {
	case mk["x_turn"] > 0:
		maximizes = true
	case mk["o_turn"] > 0:
		maximizes = false
	default:
		return nil, false, false
	}
	side := "o"
	if maximizes {
		side = "x"
	}
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			t := playTrans(side, c, r)
			if m.enabled(t, mk) {
				moves = append(moves, t)
				break
			}
		}
	}
	if len(moves) == 0 {
		return nil, false, false
	}
	return moves, maximizes, true
}

// moveColumn extracts the column a play transition targets.
func moveColumn(t string) int {
	var c, r int
	// t is "x_play_c_r" or "o_play_c_r"
	var side string
	fmt.Sscanf(t, "%1s_play_%d_%d", &side, &c, &r)
	return c
}

// terminal reports whether the (house-quiesced) marking is a decided game,
// and which outcome: +1 X won, -1 O won, 0 drawn board (no legal moves and
// no win). ok=false means the game continues.
func (m *model) terminal(mk marking) (value int, ok bool) {
	if mk["win_x"] > 0 {
		return 1, true
	}
	if mk["win_o"] > 0 {
		return -1, true
	}
	if _, _, playable := m.legalMoves(mk); !playable {
		return 0, true
	}
	return 0, false
}
