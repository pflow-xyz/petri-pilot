// blk2_*: a third catalytic tier, sibling to force_*/blk_*, that represents
// one alternating ply of search (README findings 8/9) as declared structure
// instead of runtime tree expansion — still exactly one ODE solve per
// candidate move.
//
// blk_* boosts side_play_c on a single win line's "opponent has the other
// two marks" pattern. That is a one-line predicate: it cannot distinguish
// "c completes the opponent's only threat" from "c completes one of the
// opponent's TWO simultaneous threats" (a fork) — the second case is
// strictly worse to leave unblocked, because no single reply defuses both
// lines afterward. A 2-ply search discovers this by nesting the game: after
// my candidate move, the opponent's best reply is scored, and if that reply
// is itself a forced fork completion, the static leaf read after MY second
// move still shows an unresolved threat, which is exactly what makes
// finding 9's number drop. blk2_* tries to bake the same fact into topology:
// catalyze side_play_c on the AND of two *different* win lines through c
// both already sitting at "opponent's other two cells marked" — mass action
// multiplies every catalyst marking together, so the copy's rate turns on
// only when all four opponent marks (two per line) are present, which no
// single blk_* copy can express. That is "block-the-block": c is boosted
// not because it stops one threat, but because it is the one cell whose
// capture stops two at once.
package main

import (
	"sort"

	"github.com/pflow-xyz/go-pflow/derive"
	"github.com/pflow-xyz/go-pflow/petri"
)

// candidateForkBias is the initial guess for blk2_*'s shared rate constant.
// fit-fork calibrates it against oracle labels exactly as fit.go calibrates
// winBias/blockBias; this value is a starting point, not a result.
const candidateForkBias = 0.0

// cellLines maps every cell to the (sorted) win lines that pass through it.
// Computed once from winLines/sortedLineNames rather than hand-enumerated,
// so a board-size change can't silently desync it.
var cellLines = func() map[string][]string {
	out := map[string][]string{}
	for _, name := range sortedLineNames() {
		for _, c := range winLines[name] {
			out[c] = append(out[c], name)
		}
	}
	return out
}()

// addForkTier adds one blk2_side_c_L1_L2 catalyzed copy of side_play_c per
// side, per cell c, per unordered pair of distinct win lines through c —
// 144 (cell,line-pair) combinations (see experiment notes) x 2 sides = 288
// transitions, the same order of magnitude as force_*/blk_* (also 288).
// Each copy's catalysts are the opponent's marks on every cell of L1 and L2
// other than c; two lines can share a second cell on this board (crossing
// diagonals), and the catalyst map naturally dedups that place to one read
// arc rather than double-weighting it.
func (m *model) addForkTier(net *petri.PetriNet) []string {
	var forks []string
	for _, c := range cells {
		lines := cellLines[c]
		for i := 0; i < len(lines); i++ {
			for j := i + 1; j < len(lines); j++ {
				l1, l2 := lines[i], lines[j]
				for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
					side, opp := sides[0], sides[1]
					catalysts := map[string]float64{}
					for _, other := range winLines[l1] {
						if other != c {
							catalysts[opp+other] = 1
						}
					}
					for _, other := range winLines[l2] {
						if other != c {
							catalysts[opp+other] = 1
						}
					}
					name := "blk2_" + side + "_" + c + "_" + l1 + "_" + l2
					if err := derive.AddCatalyzedCopy(net, side+"_play_"+c, name, catalysts); err != nil {
						panic(err)
					}
					forks = append(forks, name)
				}
			}
		}
	}
	sort.Strings(forks)
	return forks
}

// derivePolicyNetFork is derivePolicyNet's net (identical force_*/blk_*
// transitions) plus the blk2_* tier layered on top of the same base play
// transitions.
func (m *model) derivePolicyNetFork() (net *petri.PetriNet, wins, blks, forks []string) {
	net, wins, blks = m.derivePolicyNet()
	forks = m.addForkTier(net)
	return net, wins, blks, forks
}

func (m *model) toPetriPolicyFork(winBias, blockBias, forkBias float64) evalNet {
	net, wins, blks, forks := m.derivePolicyNetFork()
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
	for _, f := range forks {
		rates[f] = forkBias
	}
	return evalNet{net, rates}
}
