// parity.go: a structural "future support" / gravity-tempo predicate,
// answering ROADMAP.md Phase 1a on connect3 (finding 6's own diagnosis: "a
// useful next structure must represent ownership of future support, not
// merely current two-in-a-row geometry").
//
// Claimeven fact (board-size only, no game history, no fitting): this board
// has boardRows=4, an even column height. If a defender always replies
// directly above whatever the attacker just played in a column, the
// defender is guaranteed the "upper" cell of every vertical pair -- the
// 2nd and 4th cell filled from the floor in that column -- and the
// attacker is guaranteed the "lower" cell of every pair -- the 1st and 3rd.
// Distance from the floor for a cell at row r (0 = top, boardRows-1 =
// bottom) is boardRows-1-r; under this fixed row numbering that distance is
// even exactly when r is odd. So rows {1,3} are X's Claimeven rows, rows
// {0,2} are O's, uniformly across every column.
//
// This LOOKS like fitgrad.go's already-falsified groupByRowParity (finding
// 7: retuning the RATE of the same 288 force_/blk_ transitions by this same
// row-parity label made the referee worse, 34 errors) -- but the mechanism
// here is different in kind, not just in degree. groupByRowParity relabels
// an EXISTING transition's rate with a constant, fixed for the whole solve.
// Here the parity fact instead seeds a NEW persistent place per column and
// per side (claim_<side>_<col>), populated two ways:
//
//   - from cells the side has ALREADY played on-parity in the position
//     being scored (a derived readout of mk, exactly as m.position()
//     already derives landing tokens from occupancy -- "already owns", past
//     tense, needs the actual history, which a fresh rate constant cannot
//     see and a hypothetical future solve cannot recover once a cell's
//     landing token is consumed);
//   - and, during the relaxation itself, from the *same* existing
//     x_play_/o_play_ transitions continuing to fire fractionally within
//     the solve horizon (one extra output arc apiece, no new inputs) --
//     so claim mass can keep accruing from hypothetical future on-parity
//     plays inside the same horizon window the force_/blk_ copies are
//     integrated over.
//
// That claim place is then READ, never rewritten into a rate: a second,
// separately-rated catalyzed copy of every force_/blk_ transition
// (derive.AddCatalyzedCopy, the same tool blk_ itself is built from) is
// gated on the claim place for that cell's own column. Mass action then
// multiplies the *existing* forced-finish/block flow by however much
// on-parity investment has accumulated in that specific column, instead of
// relabeling the flow's speed by a constant that ignores which side is
// actually ahead there. Two new rate constants only (parityForceBias,
// parityBlockBias) -- finding 7 and finding 12 both show more freedom on
// top of this same structure makes things worse, not better.
package main

import (
	"math/rand"
	"strconv"

	"github.com/pflow-xyz/go-pflow/derive"
	"github.com/pflow-xyz/go-pflow/petri"
)

// Initial candidates only, fit like every other tier's biases.
const (
	candidateParityForceBias = 0.01
	candidateParityBlockBias = 0.0
	candidateTempoBias       = 0.0
)

// xParityRow reports whether row r is one of X's Claimeven rows.
func xParityRow(r int) bool { return (boardRows-1-r)%2 == 0 }

func claimPlace(side string, col int) string {
	return "claim_" + side + "_" + strconv.Itoa(col)
}

// deriveParityNet builds on the plain (ungrouped) force_/blk_ structure --
// identical net derivePolicyNet produces, no change to it -- and adds the
// claim places, their deposit arcs, a parity-gated catalyzed copy of every
// force_/blk_ transition, and a direct tempo tier (see toPetriParity).
// wins/blks are the original (plain-rated) families; pwins/pblks are the
// parity-gated copies; tempos is the direct claim -> win_<side> tier.
func (m *model) deriveParityNet() (net *petri.PetriNet, wins, blks, pwins, pblks, tempos []string) {
	net, wins, blks = m.derivePolicyNet()

	for col := 0; col < boardCols; col++ {
		net.AddPlace(claimPlace("x", col), 0, nil, 0, 0, nil)
		net.AddPlace(claimPlace("o", col), 0, nil, 0, 0, nil)
	}
	for _, c := range cells {
		r, col := int(c[0]-'0'), int(c[1]-'0')
		if xParityRow(r) {
			net.AddArc("x_play_"+c, claimPlace("x", col), 1, false)
		} else {
			net.AddArc("o_play_"+c, claimPlace("o", col), 1, false)
		}
	}

	for _, name := range sortedLineNames() {
		line := winLines[name]
		for i, c := range line {
			col := int(c[1] - '0')
			for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
				side := sides[0]
				idx := strconv.Itoa(i)

				finish := "force_" + side + "_" + name + "_" + idx
				pf := "p" + finish
				if err := derive.AddCatalyzedCopy(net, finish, pf, map[string]float64{claimPlace(side, col): 1}); err != nil {
					panic(err)
				}
				pwins = append(pwins, pf)

				blk := "blk_" + side + "_" + name + "_" + idx
				pb := "p" + blk
				if err := derive.AddCatalyzedCopy(net, blk, pb, map[string]float64{claimPlace(side, col): 1}); err != nil {
					panic(err)
				}
				pblks = append(pblks, pb)
			}
		}
	}

	// Direct tempo tier: a read-arc transition per (side, column) whose only
	// reactant is claim_<side>_<col> itself, depositing straight into
	// win_<side>. Under mass action this is d(win_side)/dt += k*claim, a
	// simple linear pumping proportional to on-parity investment -- the
	// literal "the flow integral can read [claim] directly" mechanism,
	// unlike pwins/pblks above which turned out inert: AddCatalyzedCopy is
	// an AND gate (rate multiplies every catalyst together), and the
	// existing force_/blk_ transitions' own two-mark catalysts are 0 at
	// every one of this referee's 8 residual failures (see README finding
	// 13) -- zero times any parityForceBias/parityBlockBias is still zero,
	// confirmed empirically (verify-parity leaves 6/2 unchanged from
	// parityForceBias/parityBlockBias in [0.02, 0.3]). This tier has no
	// such gate: it fires whenever claim is nonzero, which is exactly
	// "this side already owns an on-parity cell here", full stop.
	for _, side := range []string{"x", "o"} {
		win := "win_" + side
		for col := 0; col < boardCols; col++ {
			t := "tempo_" + side + "_" + strconv.Itoa(col)
			net.AddTransition(t, "", 0, 0, nil)
			net.AddArc(claimPlace(side, col), t, 1, false)
			net.AddArc(t, claimPlace(side, col), 1, false)
			net.AddArc(t, win, 1, false)
			tempos = append(tempos, t)
		}
	}
	return net, wins, blks, pwins, pblks, tempos
}

func (m *model) toPetriParity(winBias, blockBias, parityForceBias, parityBlockBias, tempoBias float64) evalNet {
	net, wins, blks, pwins, pblks, tempos := m.deriveParityNet()
	rates := make(map[string]float64, len(net.Transitions))
	for t := range net.Transitions {
		rates[t] = 1
	}
	for _, t := range blks {
		rates[t] = blockBias
	}
	for _, t := range wins {
		rates[t] = winBias
	}
	for _, t := range pblks {
		rates[t] = parityBlockBias
	}
	for _, t := range pwins {
		rates[t] = parityForceBias
	}
	for _, t := range tempos {
		rates[t] = tempoBias
	}
	return evalNet{net, rates}
}

// parityAugment derives the claim_<side>_<col> seed from mk's occupied
// cells -- the "already owns" half of the mechanism, computed once per
// evaluation call exactly as m.position() derives landing tokens from
// occupancy. mk is a post-move, not-yet-house-fired marking (the same
// convention odePlayer/rankLoss use throughout this package).
func parityAugment(mk marking) marking {
	aug := make(marking, len(mk)+2*boardCols)
	for k, v := range mk {
		aug[k] = v
	}
	for _, c := range cells {
		r, col := int(c[0]-'0'), int(c[1]-'0')
		if xParityRow(r) {
			if mk["x"+c] > 0 {
				aug[claimPlace("x", col)]++
			}
		} else if mk["o"+c] > 0 {
			aug[claimPlace("o", col)]++
		}
	}
	return aug
}

func parityOdePlayer(ev evalNet, lam float64) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			f := m.odeFinal(ev.net, parityAugment(m.fire(mv, mk)), ev.rates)
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
