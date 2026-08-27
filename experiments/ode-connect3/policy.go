// Search-free evaluation nets derived from the declared Connect-3 model.
package main

import (
	"math/rand"
	"strconv"

	"github.com/pflow-xyz/go-pflow/derive"
	"github.com/pflow-xyz/go-pflow/petri"
)

// Initial candidate only. `fit` and the exhaustive referee decide what
// survives; these values are not results merely by existing.
const (
	candidateForceBias = 0.01
	candidateBlockBias = 0.0
	scoreLambda        = 2.0
)

type evalNet struct {
	net   *petri.PetriNet
	rates map[string]float64
}

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

func (m *model) evaluationBase() *petri.PetriNet {
	net := m.toPetriDeclared()
	derive.DropTransitions(net, "draw")
	derive.DropPlaces(net, "move_tokens", "game_active")
	return net
}

func (m *model) toPetriBaseline() evalNet {
	net := m.evaluationBase()
	rates := make(map[string]float64, len(net.Transitions))
	for t := range net.Transitions {
		rates[t] = 1
	}
	return evalNet{net, rates}
}

func (m *model) derivePolicyNet() (*petri.PetriNet, []string, []string) {
	net := m.evaluationBase()
	wins := make([]string, 0, len(winLines)*connectN*2)
	blks := make([]string, 0, len(winLines)*connectN*2)
	for _, name := range sortedLineNames() {
		line := winLines[name]
		for i, c := range line {
			for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
				side, opp := sides[0], sides[1]
				catalysts := map[string]float64{}
				for j, other := range line {
					if i != j {
						catalysts[opp+other] = 1
					}
				}
				// A forced finish compresses the two-step relaxed path
				// (play the now-supported square, then fire its detector)
				// into one declared policy transition. The landing token p<c>
				// is the gravity precondition; the two friendly marks are
				// catalytic reads.
				finish := "force_" + side + "_" + name + "_" + strconv.Itoa(i)
				net.AddTransition(finish, "", 0, 0, nil)
				net.AddArc("p"+c, finish, 1, false)
				net.AddArc(side+"_turn", finish, 1, false)
				for j, other := range line {
					if i == j {
						continue
					}
					net.AddArc(side+other, finish, 1, false)
					net.AddArc(finish, side+other, 1, false)
				}
				net.AddArc(finish, side+c, 1, false)
				net.AddArc(finish, "win_"+side, 1, false)
				wins = append(wins, finish)
				blk := "blk_" + side + "_" + name + "_" + strconv.Itoa(i)
				if err := derive.AddCatalyzedCopy(net, side+"_play_"+c, blk, catalysts); err != nil {
					panic(err)
				}
				blks = append(blks, blk)
			}
		}
	}
	return net, wins, blks
}

func (m *model) toPetriPolicy(winBias, blockBias float64) evalNet {
	net, wins, blks := m.derivePolicyNet()
	rates := make(map[string]float64, len(net.Transitions))
	for t := range net.Transitions {
		rates[t] = 1
	}
	for _, blk := range blks {
		rates[blk] = blockBias
	}
	for _, finish := range wins {
		rates[finish] = winBias
	}
	return evalNet{net, rates}
}

func odePlayer(ev evalNet, lam float64) player {
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

// tacticalGuard is the separately reported shipping layer: take an immediate
// win, otherwise discard candidates that permit an immediate opponent win
// when at least one safe candidate exists. It is discrete one-ply logic and
// is never counted as evidence for the ODE or structural-policy result.
func tacticalGuard(base player) player {
	return func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string {
		var wins, safe []string
		for _, move := range moves {
			next := m.fireHouse(m.fire(move, mk))
			ownWin := next["win_x"] > 0
			if !maximizes {
				ownWin = m.hasLine(next, "o")
			}
			if ownWin {
				wins = append(wins, move)
				continue
			}
			_, replies, _, ok := m.legalMoves(next)
			unsafe := false
			if ok {
				for _, reply := range replies {
					after := m.fireHouse(m.fire(reply, next))
					opponentWin := m.hasLine(after, "o")
					if !maximizes {
						opponentWin = after["win_x"] > 0
					}
					if opponentWin {
						unsafe = true
						break
					}
				}
			}
			if !unsafe {
				safe = append(safe, move)
			}
		}
		if len(wins) > 0 {
			return base(m, mk, wins, maximizes, rng)
		}
		if len(safe) > 0 {
			return base(m, mk, safe, maximizes, rng)
		}
		return base(m, mk, moves, maximizes, rng)
	}
}
