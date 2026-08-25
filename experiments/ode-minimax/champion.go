// The winning evaluator, derived rather than hand-built. Everything this
// experiment iterated over — draw wirings, threat coordinates, one-knob
// biases, incidence weightings — lives in the git history and in
// README.md's findings; what survived is below, and it is now expressed
// as go-pflow/derive transforms applied to the declared net:
//
//	net := declared tic-tac-toe net
//	derive.DropTransitions(net, "draw")               // finding 12
//	derive.DropPlaces(net, "move_tokens", "game_active") // findings 12, 15
//	derive.AddCatalyzedCopy(net, play, blk, opponent-marks) x48 // finding 9
//
// The result is 31 places and 82 transitions: the 27 board cells, the
// turn tokens, the win places; 18 plays, 16 catalytic win detectors
// (halting via turn-token consumption), and 48 forced-reply copies —
// the opponent's policy ("threats are answered") as structure, at rate
// championBlockBias.
//
// Scoring, one ODE solve per candidate move: X maximizes win_x - win_o;
// O maximizes x_turn + o_turn + championLambda*win_o (the undecided
// outcome is the turn-token mass still circulating at the horizon).
// The two constants were fitted with learn.Minimize against minimax
// labels (fit.go); the whole evaluator is exhaustively minimax-
// equivalent on both seats (main.go verify).
package main

import (
	"math/rand"
	"sort"
	"strconv"

	"github.com/pflow-xyz/go-pflow/derive"
	"github.com/pflow-xyz/go-pflow/petri"
)

func sortedLineNames() []string {
	names := make([]string, 0, len(winLines))
	for name := range winLines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

const (
	championBlockBias = 2.724
	championLambda    = 1.872
)

// evalNet is an evaluation net plus the rates for its transitions.
type evalNet struct {
	net   *petri.PetriNet
	rates map[string]float64
}

// toPetriDeclared builds the declared net — the model, verbatim.
func (m *model) toPetriDeclared() *petri.PetriNet {
	net := petri.NewPetriNet()
	for _, p := range m.places {
		net.AddPlace(p, m.initial[p], nil, 0, 0, nil)
	}
	for _, t := range m.transitions {
		net.AddTransition(t, "", 0, 0, nil)
		for _, a := range m.inputs[t] {
			net.AddArc(a.from, t, a.weight, false)
		}
		for _, a := range m.outputs[t] {
			net.AddArc(t, a.to, a.weight, false)
		}
	}
	return net
}

// toPetriChampion derives the champion evaluation net from the declared
// net at the given block bias.
func (m *model) toPetriChampion(blockBias float64) evalNet {
	net := m.toPetriDeclared()
	derive.DropTransitions(net, "draw")
	derive.DropPlaces(net, "move_tokens", "game_active")

	rates := make(map[string]float64, len(m.transitions)+48)
	for _, t := range m.transitions {
		if t != "draw" {
			rates[t] = 1
		}
	}
	for _, name := range sortedLineNames() {
		line := winLines[name]
		for i, c := range line {
			for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
				side, opp := sides[0], sides[1]
				catalysts := map[string]float64{}
				for j, oc := range line {
					if j != i {
						catalysts[opp+oc] = 1
					}
				}
				blk := "blk_" + side + "_" + name + "_" + strconv.Itoa(i)
				if err := derive.AddCatalyzedCopy(net, side+"_play_"+c, blk, catalysts); err != nil {
					panic(err) // structural bug in this file, not a runtime condition
				}
				rates[blk] = blockBias
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
