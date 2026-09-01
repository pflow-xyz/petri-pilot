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

// groupKeyFn labels a force_*/blk_* transition by (side, win-line name, the
// cell it acts on, its index within the line). Two transitions returning the
// same label share one fitted rate. The plain, ungrouped policy (below) uses
// one label for everything in each family — the untuned baseline that
// fitgrad.go's finer grouping schemes are compared against.
type groupKeyFn func(side, lineName, cell string, i int) string

func groupSingle(string, string, string, int) string { return "all" }

// groupByRow labels by the gravity depth of the cell the transition acts on
// — the strongest hypothesis for the "future support" failure mode (see
// README finding 6): the residual referee errors are late gravity-tempo
// positions, and row is the direct proxy for how many drops a cell is from
// the floor.
func groupByRow(side, _, cell string, _ int) string { return side + "_row" + cell[:1] }

// groupByRowParity is a coarser version of groupByRow: turn-parity-at-opening
// (who is due to move when a supporting cell opens) collapses to row%2.
func groupByRowParity(side, _, cell string, _ int) string {
	row := int(cell[0] - '0')
	return side + "_parity" + strconv.Itoa(row%2)
}

// groupByLineType labels by row/col/diag/anti. Weak hypothesis: nothing about
// gravity distinguishes one line orientation from another once dropped into.
func groupByLineType(side, lineName, _ string, _ int) string { return side + "_" + lineTypeOf(lineName) }

// groupByCellIndex labels by position within the line (0,1,2). Weak
// hypothesis: doesn't correlate with gravity depth.
func groupByCellIndex(side, _, _ string, i int) string { return side + "_idx" + strconv.Itoa(i) }

// groupByLineTypeRowParity is the finest scheme worth trying — line-type x
// row-parity, ~16 groups. Past this point, grouping starts approximating one
// rate per known counterexample rather than a generalizable rule.
func groupByLineTypeRowParity(side, lineName, cell string, _ int) string {
	row := int(cell[0] - '0')
	return side + "_" + lineTypeOf(lineName) + "_parity" + strconv.Itoa(row%2)
}

// lineTypeOf strips the trailing "%d_%d" winLines suffixes any of
// row/col/diag/anti carries, leaving the type prefix.
func lineTypeOf(name string) string {
	i := 0
	for i < len(name) && (name[i] < '0' || name[i] > '9') {
		i++
	}
	return name[:i]
}

// derivePolicyNetGrouped is derivePolicyNet's structure — identical net,
// identical 144 force_* and 144 blk_* transitions, no new derive calls — with
// each transition labeled by winGroup/blkGroup instead of returned as a flat
// list. derivePolicyNet is this with both families collapsed to one label.
func (m *model) derivePolicyNetGrouped(winGroup, blkGroup groupKeyFn) (net *petri.PetriNet, winGroups, blkGroups map[string]string) {
	net = m.evaluationBase()
	winGroups = make(map[string]string, len(winLines)*connectN*2)
	blkGroups = make(map[string]string, len(winLines)*connectN*2)
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
				winGroups[finish] = winGroup(side, name, c, i)
				blk := "blk_" + side + "_" + name + "_" + strconv.Itoa(i)
				if err := derive.AddCatalyzedCopy(net, side+"_play_"+c, blk, catalysts); err != nil {
					panic(err)
				}
				blkGroups[blk] = blkGroup(side, name, c, i)
			}
		}
	}
	return net, winGroups, blkGroups
}

func (m *model) derivePolicyNet() (*petri.PetriNet, []string, []string) {
	net, winGroups, blkGroups := m.derivePolicyNetGrouped(groupSingle, groupSingle)
	wins := make([]string, 0, len(winGroups))
	for t := range winGroups {
		wins = append(wins, t)
	}
	blks := make([]string, 0, len(blkGroups))
	for t := range blkGroups {
		blks = append(blks, t)
	}
	return net, wins, blks
}

// toPetriPolicyGrouped rebuilds a plain (fixed-rate) evaluation net from
// fitted per-group rates — the acceptance-gate form of a fitgrad.go fit,
// used by the exhaustive referee exactly as toPetriPolicy is.
func (m *model) toPetriPolicyGrouped(winGroup, blkGroup groupKeyFn, winRates, blkRates map[string]float64) evalNet {
	net, winGroups, blkGroups := m.derivePolicyNetGrouped(winGroup, blkGroup)
	rates := make(map[string]float64, len(net.Transitions))
	for t := range net.Transitions {
		rates[t] = 1
	}
	for t, g := range winGroups {
		rates[t] = winRates[g]
	}
	for t, g := range blkGroups {
		rates[t] = blkRates[g]
	}
	return evalNet{net, rates}
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
