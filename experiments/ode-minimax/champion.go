// The winning evaluator, and nothing else. Everything this experiment
// iterated over — draw wirings, threat coordinates, one-knob biases,
// incidence weightings — lives in the git history and in README.md's
// findings; what survived is below.
//
// The champion evaluation net is 31 places and 82 transitions:
//
//   places       the 27 board cells (p/x/o per square), x_turn, o_turn,
//                win_x, win_o
//   plays        18 ordinary move transitions (rate 1)
//   detectors    16 win detectors, catalytic on their line's cells,
//                consuming the opposing turn token (halting)
//   block bias   48 forced-reply copies of the plays — the same move,
//                additionally catalyzed by the opponent holding the
//                other two cells of a line through it, at rate
//                championBlockBias. The opponent's policy ("threats are
//                answered") as structure.
//
// No draw transition, no move_tokens counter, no game_active gate: the
// undecided outcome is the turn-token mass still circulating at the
// horizon, and halting is already the detectors eating the turn tokens.
//
// Scoring, one ODE solve per candidate move: X maximizes win_x - win_o;
// O maximizes x_turn + o_turn + championLambda*win_o. The two constants
// were fitted by Nelder-Mead against minimax labels (fit.go) and the
// whole evaluator is exhaustively minimax-equivalent on both seats
// (main.go verify).
package main

import (
	"math/rand"
	"sort"
	"strconv"

	"github.com/pflow-xyz/go-pflow/petri"
)

const (
	championBlockBias = 2.724
	championLambda    = 1.872
)

// evalNet is an evaluation net plus the rates for its transitions.
type evalNet struct {
	net   *petri.PetriNet
	rates map[string]float64
}

// toPetriChampion builds the champion net at the given block bias.
func (m *model) toPetriChampion(blockBias float64) evalNet {
	net := petri.NewPetriNet()
	rates := map[string]float64{}
	for _, c := range cells {
		net.AddPlace("p"+c, 1, nil, 0, 0, nil)
		net.AddPlace("x"+c, 0, nil, 0, 0, nil)
		net.AddPlace("o"+c, 0, nil, 0, 0, nil)
	}
	net.AddPlace("x_turn", 1, nil, 0, 0, nil)
	net.AddPlace("o_turn", 0, nil, 0, 0, nil)
	net.AddPlace("win_x", 0, nil, 0, 0, nil)
	net.AddPlace("win_o", 0, nil, 0, 0, nil)

	play := func(t, side, opp, c string, rate float64) {
		net.AddTransition(t, "", 0, 0, nil)
		rates[t] = rate
		net.AddArc("p"+c, t, 1, false)
		net.AddArc(side+"_turn", t, 1, false)
		net.AddArc(t, side+c, 1, false)
		net.AddArc(t, opp+"_turn", 1, false)
	}
	for _, c := range cells {
		play("x_play_"+c, "x", "o", c, 1)
		play("o_play_"+c, "o", "x", c, 1)
	}
	for _, name := range sortedLineNames() {
		line := winLines[name]
		for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
			side, opp := sides[0], sides[1]
			t := side + "_win_" + name
			net.AddTransition(t, "", 0, 0, nil)
			rates[t] = 1
			for _, c := range line {
				net.AddArc(side+c, t, 1, false)
				net.AddArc(t, side+c, 1, false)
			}
			net.AddArc(opp+"_turn", t, 1, false)
			net.AddArc(t, "win_"+side, 1, false)
		}
		// forced-reply copies: playing cell i of this line, catalyzed by
		// the opponent holding the line's other two cells
		for i, c := range line {
			for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
				side, opp := sides[0], sides[1]
				t := "blk_" + side + "_" + name + "_" + strconvItoa(i)
				play(t, side, opp, c, blockBias)
				for j, oc := range line {
					if j != i {
						net.AddArc(opp+oc, t, 1, false)
						net.AddArc(t, opp+oc, 1, false)
					}
				}
			}
		}
	}
	return evalNet{net, rates}
}

// championPlayer: one ODE solve per candidate; argmax of the seat's
// objective over the final state.
func championPlayer(ev evalNet, lam float64) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			f := m.odeFinal(ev.net, m.fire(mv, mk), ev.rates)
			score := f["win_x"] - f["win_o"]
			if !maximizes {
				score = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
			}
			if i == 0 || score > bestScore {
				best, bestScore = mv, score
			}
		}
		return best
	}
}

func sortedLineNames() []string {
	names := make([]string, 0, len(winLines))
	for name := range winLines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func strconvItoa(i int) string { return strconv.Itoa(i) }
