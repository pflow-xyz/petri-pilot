// The structurally-biased evaluator: policy-in-structure, following
// ode-minimax finding 9 (forced-reply copies) and finding 15 (game_active is
// removable once win detectors already halt the game by consuming turn
// tokens).
//
// Connect Four's dominant single tactic mirrors tic-tac-toe's "block the
// open two": an opponent holding 3-of-4 on a line with the 4th cell
// reachable is a must-answer threat. Unlike ode-minimax — where a
// block-only tier plus a free lambda reached full minimax-equivalence, and
// finding 11 showed a symmetric "complete your own line" tier was
// unnecessary once lambda was free — a block-only tier here plateaus well
// short of the referee (README finding 5), and a second blind "complete
// your own line" tier makes it worse, not better (README finding 6). This
// champion net carries THREE catalyzed-copy tiers: `blk_*` (opponent's
// other 3 cells — "answer this threat"), `own_*` (mover's own other 3
// cells, every row — the tier finding 6 rejected, shipped at rate 0), and
// `prt_*` (mover's own other 3 cells, but ONLY on rows whose parity favors
// that mover — the theory-motivated tier from Allis's Connect Four thesis,
// tested in finding 7). Whether each tier earns its keep is a fitted-value
// question decided against the held-out referee, never an assumption.
package main

import (
	"strconv"

	"github.com/pflow-xyz/go-pflow/derive"
	"github.com/pflow-xyz/go-pflow/petri"
)

// toPetriDeclared builds the declared net, verbatim.
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

// favorableParity implements the core mechanism of Victor Allis's 1988
// Connect Four thesis, in the narrow form this experiment tests: a cell's
// row favors whichever side benefits from a threat sitting there once the
// board fills up around it (the "Claimeven" pairing argument — the second
// player can neutralize any single first-player threat strictly above an
// even row by always replying in the same column, forcing parity to hand
// them the cell below the threat first). Rows are 0-indexed from the
// bottom (0 = bottom row, 5 = top row, matching this file's `rc[1]`
// convention throughout). Under that indexing the theory's asymmetry is:
// X (first player) benefits from threats on ODD rows (1, 3, 5); O (second
// player) benefits from threats on EVEN rows (0, 2, 4).
func favorableParity(side string, row int) bool {
	if side == "x" {
		return row%2 == 1
	}
	return row%2 == 0
}

// deriveChampionNet applies the champion's derive transforms to the
// declared net: drop game_active, then add THREE catalyzed-copy tiers per
// (line, cell-in-line, side):
//
//   - blk_* (552 copies) — catalyzed by the OPPONENT's other 3 cells on the
//     line ("they have 3-of-4, answer it"): the block-bias tier described
//     in this file's header comment.
//   - own_* (552 copies) — catalyzed by the MOVER's own other 3 cells on
//     the line ("complete your own line"), with NO row-parity restriction.
//     README finding 6: this tier improves training loss but regresses the
//     held-out referee, so it ships at rate 0 by default.
//   - prt_* (fewer than 552 — only the (line, cell, side) triples where
//     favorableParity holds) — catalyzed the same way as own_*, but ONLY
//     added for cells whose row favors the mover under the theory above.
//     This is the tier README finding 7 tests: unlike own_*, it is not
//     blind to row — it specifically encodes the odd/even asymmetry rather
//     than rewarding "any 3-of-4" uniformly across all six rows.
func (m *model) deriveChampionNet() (net *petri.PetriNet, blks, owns, prts []string) {
	net = m.toPetriDeclared()
	derive.DropPlaces(net, "game_active")

	blks = make([]string, 0, len(winLines)*4*2)
	owns = make([]string, 0, len(winLines)*4*2)
	prts = make([]string, 0, len(winLines)*4) // at most half of 552, by construction
	for _, ln := range winLines {
		for i, rc := range ln.cells {
			c, r := rc[0], rc[1]
			for _, sides := range [][2]string{{"x", "o"}, {"o", "x"}} {
				side, opp := sides[0], sides[1]
				blockCat := map[string]float64{}
				ownCat := map[string]float64{}
				for j, orc := range ln.cells {
					if j != i {
						blockCat[cellPlace(opp, orc[0], orc[1])] = 1
						ownCat[cellPlace(side, orc[0], orc[1])] = 1
					}
				}
				blk := "blk_" + side + "_" + ln.id + "_" + strconv.Itoa(i)
				if err := derive.AddCatalyzedCopy(net, playTrans(side, c, r), blk, blockCat); err != nil {
					panic(err) // structural bug in this file, not a runtime condition
				}
				blks = append(blks, blk)

				own := "own_" + side + "_" + ln.id + "_" + strconv.Itoa(i)
				if err := derive.AddCatalyzedCopy(net, playTrans(side, c, r), own, ownCat); err != nil {
					panic(err)
				}
				owns = append(owns, own)

				if favorableParity(side, r) {
					prt := "prt_" + side + "_" + ln.id + "_" + strconv.Itoa(i)
					if err := derive.AddCatalyzedCopy(net, playTrans(side, c, r), prt, ownCat); err != nil {
						panic(err)
					}
					prts = append(prts, prt)
				}
			}
		}
	}
	return net, blks, owns, prts
}

// toPetriChampion derives the champion evaluation net at the given
// (blockBias, winBias, parityBias) triple; every declared transition keeps
// rate 1, every blk_* copy gets blockBias, every own_* copy gets winBias,
// every prt_* copy gets parityBias.
func (m *model) toPetriChampion(blockBias, winBias, parityBias float64) evalNet {
	net, blks, owns, prts := m.deriveChampionNet()
	rates := make(map[string]float64, len(m.transitions)+len(blks)+len(owns)+len(prts))
	for _, t := range m.transitions {
		rates[t] = 1
	}
	for _, blk := range blks {
		rates[blk] = blockBias
	}
	for _, own := range owns {
		rates[own] = winBias
	}
	for _, prt := range prts {
		rates[prt] = parityBias
	}
	return evalNet{net, rates}
}

// championScore: X maximizes win_x - win_o (same as naive); O maximizes the
// surviving turn-token mass (the undecided outcome, now that game_active is
// gone) plus lambda times win_o.
func championScore(lam float64) scoreFn {
	return func(f map[string]float64, maximizes bool) float64 {
		if maximizes {
			return f["win_x"] - f["win_o"]
		}
		return f["x_turn"] + f["o_turn"] + lam*f["win_o"]
	}
}

func championPlayer(m *model, blockBias, winBias, parityBias, lam float64) player {
	return evalPlayer(m, m.toPetriChampion(blockBias, winBias, parityBias), championScore(lam))
}
