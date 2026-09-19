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
// champion net carries FOUR catalyzed-copy tiers: `blk_*` (opponent's
// other 3 cells — "answer this threat"), `own_*` (mover's own other 3
// cells, every row — the tier finding 6 rejected, shipped at rate 0),
// `prt_*` (mover's own other 3 cells, but ONLY on rows whose parity favors
// that mover — the theory-motivated RATE tier from Allis's Connect Four
// thesis, tested and rejected in finding 7), and `clm_*` (finding 9 /
// ROADMAP 1a: a STRUCTURAL, place-based encoding of the same Claimeven
// asymmetry — see this file's "column parity-claim account" section below
// for why it is a different mechanism from prt_*, not a retuning of it).
// Whether each tier earns its keep is a fitted-value question decided
// against the held-out referee, never an assumption.
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

// claimPlace names the per-(side,column) parity-claim account place — see
// "column parity-claim account" below.
func claimPlace(side string, col int) string { return "claim_" + side + "_" + strconv.Itoa(col) }

// deriveChampionNet applies the champion's derive transforms to the
// declared net: drop game_active, then add FOUR catalyzed-copy tiers per
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
//     This is the RATE tier README finding 7 tests and rejects: unlike
//     own_*, it is not blind to row, but it can only reward a play that is
//     ALREADY an immediate winning completion on a favorable row, right
//     now — it has no memory of anything that happened earlier in the
//     column.
//   - clm_* (same triples as prt_*) — the STRUCTURAL fix finding 7 leaves
//     open and ROADMAP 1a names: a persistent per-(side,column) "parity
//     claim account" place (`claim_<side>_<col>`, added below), fed by a
//     plain fixed-rate output arc on the BASE declared play transition
//     whenever a play lands on a row that favors that side — a deposit,
//     not a rate — and READ (as an extra catalyst, alongside the same
//     own-3-cells pattern prt_* uses) only by clm_*'s own copies. Firing a
//     favorable-parity move earlier in a column literally leaves more
//     token mass sitting in that column's claim place for every later
//     completion attempt to read, for as long as the flow integral runs —
//     information carried by topology across the continuation, which is
//     exactly what a rate multiplier computed from the instantaneous
//     marking (prt_*) cannot represent. See the deposit-wiring comment
//     below for why the deposit is added only to the BASE transition,
//     never to any catalyzed copy (self-catalysis / runaway feedback).
func (m *model) deriveChampionNet() (net *petri.PetriNet, blks, owns, prts, clms []string) {
	net = m.toPetriDeclared()
	derive.DropPlaces(net, "game_active")

	// Column parity-claim account places: one per (side, column), starting
	// empty. Nothing pre-seeds them from real game history (odeFinal always
	// starts every evaluation-net place not in the discrete marking at 0,
	// same as every other derived place here) — the account accrues purely
	// from flow through the deposit arcs added below, over the SAME
	// continuous relaxation the candidate move is already being scored
	// under. That is a real, stated scope limitation of this construction,
	// not an oversight — see the README finding this tier is written up as.
	for c := 0; c < Width; c++ {
		for _, side := range []string{"x", "o"} {
			net.AddPlace(claimPlace(side, c), 0, nil, 0, 0, nil)
		}
	}

	blks = make([]string, 0, len(winLines)*4*2)
	owns = make([]string, 0, len(winLines)*4*2)
	prts = make([]string, 0, len(winLines)*4) // at most half of 552, by construction
	clms = make([]string, 0, len(winLines)*4)
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

					// clm_*: identical catalyst pattern to prt_* (mover's
					// own other 3 cells on the line) PLUS a read of this
					// cell's column's claim account. Built here, BEFORE the
					// deposit arcs below exist, so AddCatalyzedCopy (which
					// copies src's CURRENT output arcs) cannot let clm_*
					// inherit a deposit arc into the very place it reads —
					// no self-catalysis, no runaway feedback.
					clmCat := make(map[string]float64, len(ownCat)+1)
					for p, w := range ownCat {
						clmCat[p] = w
					}
					clmCat[claimPlace(side, c)] = 1
					clm := "clm_" + side + "_" + ln.id + "_" + strconv.Itoa(i)
					if err := derive.AddCatalyzedCopy(net, playTrans(side, c, r), clm, clmCat); err != nil {
						panic(err)
					}
					clms = append(clms, clm)
				}
			}
		}
	}

	// Deposit wiring, added LAST and only to the BASE declared play
	// transitions (never to blk_/own_/prt_/clm_ — all already built above,
	// so they cannot retroactively inherit this arc): whenever playing at
	// (c, r) lands on a row that favors that side, the move also produces
	// one token into that column's claim account for that side. The base
	// transition's own rate is fixed at 1 (never calibrated), so this
	// deposit cannot itself be amplified by a fitted constant — only clm_*'s
	// READ of the resulting mass is calibrated.
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			for _, side := range []string{"x", "o"} {
				if favorableParity(side, r) {
					net.AddArc(playTrans(side, c, r), claimPlace(side, c), 1, false)
				}
			}
		}
	}

	return net, blks, owns, prts, clms
}

// toPetriChampion derives the champion evaluation net at the given
// (blockBias, winBias, parityBias, claimBias) quadruple; every declared
// transition keeps rate 1, every blk_* copy gets blockBias, every own_*
// copy gets winBias, every prt_* copy gets parityBias, every clm_* copy
// gets claimBias.
func (m *model) toPetriChampion(blockBias, winBias, parityBias, claimBias float64) evalNet {
	net, blks, owns, prts, clms := m.deriveChampionNet()
	rates := make(map[string]float64, len(m.transitions)+len(blks)+len(owns)+len(prts)+len(clms))
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
	for _, clm := range clms {
		rates[clm] = claimBias
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

func championPlayer(m *model, blockBias, winBias, parityBias, claimBias, lam float64) player {
	return evalPlayer(m, m.toPetriChampion(blockBias, winBias, parityBias, claimBias), championScore(lam))
}
