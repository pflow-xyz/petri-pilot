// The draw-aware halting tic-tac-toe net and its discrete semantics — the
// net shipped as sim-pflow-xyz/services/tic-tac-toe.json.
//
// Places: p<rc> (cell open), x<rc>/o<rc> (claimed), x_turn/o_turn,
// game_active, move_tokens, win_x, win_o. Win detectors are catalytic
// on the claimed cells and absorb the opponent's turn token plus
// game_active, so a finished game is an absorbing state; a weight-9 arc
// from move_tokens fires the draw transition, which pays win_o. The
// declared objective is win_x - win_o: a tie counts fully against X.
// Whether win_o holds a real O win or a called draw is decided by the
// board itself (hasLine), not by a separate place.
package main

import "sort"

type arc struct {
	from, to string
	weight   int
}

var cells = []string{"00", "01", "02", "10", "11", "12", "20", "21", "22"}

var winLines = map[string][]string{
	"row0": {"00", "01", "02"}, "row1": {"10", "11", "12"}, "row2": {"20", "21", "22"},
	"col0": {"00", "10", "20"}, "col1": {"01", "11", "21"}, "col2": {"02", "12", "22"},
	"diag": {"00", "11", "22"}, "anti": {"02", "11", "20"},
}

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

	for _, c := range cells {
		addPlace("p"+c, 1)
		addPlace("x"+c, 0)
		addPlace("o"+c, 0)
	}
	addPlace("x_turn", 1)
	addPlace("o_turn", 0)
	addPlace("game_active", 1)
	addPlace("move_tokens", 0)
	addPlace("win_x", 0)
	addPlace("win_o", 0)

	for _, c := range cells {
		xt, ot := "x_play_"+c, "o_play_"+c
		addT(xt, 1)
		in(xt, "p"+c, 1)
		in(xt, "x_turn", 1)
		out(xt, "x"+c, 1)
		out(xt, "o_turn", 1)
		out(xt, "move_tokens", 1)
		addT(ot, 1)
		in(ot, "p"+c, 1)
		in(ot, "o_turn", 1)
		out(ot, "o"+c, 1)
		out(ot, "x_turn", 1)
		out(ot, "move_tokens", 1)
	}
	names := make([]string, 0, len(winLines))
	for name := range winLines {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		xs := "x_win_" + name
		addT(xs, 1)
		for _, c := range winLines[name] {
			in(xs, "x"+c, 1)
			out(xs, "x"+c, 1)
		}
		in(xs, "o_turn", 1)
		in(xs, "game_active", 1)
		out(xs, "win_x", 1)

		os := "o_win_" + name
		addT(os, 1)
		for _, c := range winLines[name] {
			in(os, "o"+c, 1)
			out(os, "o"+c, 1)
		}
		in(os, "x_turn", 1)
		in(os, "game_active", 1)
		out(os, "win_o", 1)
	}
	addT("draw", 1)
	in("draw", "move_tokens", 9)
	in("draw", "game_active", 1)
	out("draw", "win_o", 1)
	return m
}

// ---- discrete firing rule ----

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
	for _, c := range cells {
		owned["x_play_"+c] = true
		owned["o_play_"+c] = true
	}
	return owned
}()

// fireHouse fires unowned transitions (win detectors, call_draw) to
// quiescence — the referee.
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

// legalMoves returns the mover's prefix, enabled moves and whether the
// mover maximizes the objective; ok=false means the game is over.
func (m *model) legalMoves(mk marking) (prefix string, moves []string, maximizes, ok bool) {
	switch {
	case mk["x_turn"] > 0:
		prefix, maximizes = "x_play_", true
	case mk["o_turn"] > 0:
		prefix = "o_play_"
	default:
		return "", nil, false, false
	}
	for _, c := range cells {
		if m.enabled(prefix+c, mk) {
			moves = append(moves, prefix+c)
		}
	}
	if len(moves) == 0 {
		return "", nil, false, false // board full, draw already called
	}
	return prefix, moves, maximizes, true
}

// declaredObjective is the model's objective: win_x - win_o. A called draw
// pays win_o, so a tie scores -1 for X — that folding is part of the net.
func declaredObjective(mk marking) int {
	return mk["win_x"] - mk["win_o"]
}

// hasLine reports whether side ("x" or "o") has a completed win line — how
// the board itself distinguishes a real win from a called draw.
func (m *model) hasLine(mk marking, side string) bool {
	for _, line := range winLines {
		complete := true
		for _, c := range line {
			if mk[side+c] == 0 {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}
