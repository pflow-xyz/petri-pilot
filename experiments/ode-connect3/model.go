// The declared 4x4 Connect-3 net and its exact discrete semantics.
//
// Gravity lives in the topology. Each column has exactly one p<rc> token:
// the next cell a counter can land in. A play consumes that token and emits
// the cell above it, so the enabled play transitions are precisely the legal
// drops. Win detectors are catalytic on every length-3 row, column, or
// diagonal segment and halt play by consuming the opponent's turn token plus
// game_active. A weight-16 move counter declares the full-board draw.
package main

import (
	"fmt"
	"sort"
)

const (
	boardRows = 4
	boardCols = 4
	connectN  = 3
)

var cells = func() []string {
	out := make([]string, 0, boardRows*boardCols)
	for r := 0; r < boardRows; r++ {
		for c := 0; c < boardCols; c++ {
			out = append(out, fmt.Sprintf("%d%d", r, c))
		}
	}
	return out
}()

var winLines = func() map[string][]string {
	out := map[string][]string{}
	cell := func(r, c int) string { return fmt.Sprintf("%d%d", r, c) }
	for r := 0; r < boardRows; r++ {
		for c := 0; c <= boardCols-connectN; c++ {
			out[fmt.Sprintf("row%d_%d", r, c)] = []string{cell(r, c), cell(r, c+1), cell(r, c+2)}
		}
	}
	for c := 0; c < boardCols; c++ {
		for r := 0; r <= boardRows-connectN; r++ {
			out[fmt.Sprintf("col%d_%d", c, r)] = []string{cell(r, c), cell(r+1, c), cell(r+2, c)}
		}
	}
	for r := 0; r <= boardRows-connectN; r++ {
		for c := 0; c <= boardCols-connectN; c++ {
			out[fmt.Sprintf("diag%d_%d", r, c)] = []string{cell(r, c), cell(r+1, c+1), cell(r+2, c+2)}
		}
		for c := connectN - 1; c < boardCols; c++ {
			out[fmt.Sprintf("anti%d_%d", r, c)] = []string{cell(r, c), cell(r+1, c-1), cell(r+2, c-2)}
		}
	}
	return out
}()

type arc struct {
	from, to string
	weight   int
}

type model struct {
	places      []string
	transitions []string
	rates       map[string]float64
	inputs      map[string][]arc
	outputs     map[string][]arc
	initial     map[string]int
	minimaxMemo map[string]int
}

func buildModel() *model {
	m := &model{
		rates:       map[string]float64{},
		inputs:      map[string][]arc{},
		outputs:     map[string][]arc{},
		initial:     map[string]int{},
		minimaxMemo: map[string]int{},
	}
	addPlace := func(id string, init int) { m.places = append(m.places, id); m.initial[id] = init }
	addT := func(id string, rate float64) { m.transitions = append(m.transitions, id); m.rates[id] = rate }
	in := func(t, p string, w int) { m.inputs[t] = append(m.inputs[t], arc{p, t, w}) }
	out := func(t, p string, w int) { m.outputs[t] = append(m.outputs[t], arc{t, p, w}) }

	for _, c := range cells {
		r := int(c[0] - '0')
		addPlace("p"+c, boolInt(r == boardRows-1))
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
		r, col := int(c[0]-'0'), int(c[1]-'0')
		for _, side := range []string{"x", "o"} {
			other := "o"
			if side == "o" {
				other = "x"
			}
			t := side + "_play_" + c
			addT(t, 1)
			in(t, "p"+c, 1)
			in(t, side+"_turn", 1)
			out(t, side+c, 1)
			out(t, other+"_turn", 1)
			out(t, "move_tokens", 1)
			if r > 0 {
				out(t, fmt.Sprintf("p%d%d", r-1, col), 1)
			}
		}
	}

	for _, name := range sortedLineNames() {
		for _, side := range []string{"x", "o"} {
			other, win := "o", "win_x"
			if side == "o" {
				other, win = "x", "win_o"
			}
			t := side + "_win_" + name
			addT(t, 1)
			for _, c := range winLines[name] {
				in(t, side+c, 1)
				out(t, side+c, 1)
			}
			in(t, other+"_turn", 1)
			in(t, "game_active", 1)
			out(t, win, 1)
		}
	}
	addT("draw", 1)
	in("draw", "move_tokens", boardRows*boardCols)
	in("draw", "game_active", 1)
	out("draw", "win_o", 1)
	return m
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func sortedLineNames() []string {
	names := make([]string, 0, len(winLines))
	for name := range winLines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

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
	return prefix, moves, maximizes, len(moves) > 0
}

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
