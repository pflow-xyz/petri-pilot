// oraclefuture.go: the oracle-ceiling experiment. Two prior attempts (not in
// this file's history — see README) tried to give the search-free ODE
// evaluator a "future support" predicate by deriving it heuristically from
// the CURRENT marking (a parity/tempo place; a richer now-only AND-gate
// "block-the-block" tier). Both failed to reduce the exhaustive referee's 8
// naive errors, and diagnose-fork proved the second one's catalysts never
// fire on any of the 8 failures: those are about cells that have not opened
// under gravity yet, which no predicate over current marks alone can see
// coming.
//
// This file stops approximating and hands the evaluator the EXACT fact,
// computed offline by the game's own exact minimax oracle (players.go's
// m.minimax/m.optimalSet, already memoized and exhaustive over this game's
// ~5,000 reachable states). It is deliberately "cheating" relative to a real
// player — the point is to establish an upper bound on what the flow-integral
// representation could achieve if the future-support predicate findings 6/7
// named were solved exactly, decoupled from whether a cheap heuristic can
// find it.
package main

import "math/rand"

// oracleSeedPlies is the default depth of the forced-reply projection below.
// It is a ply COUNT, not a ply INDEX: 8 plies is 4 full turns per side, deep
// enough to reach the "several drops later" cells finding 6 names while
// still usually stopping short of a terminal state (see oracleForcedSeed's
// doc for why reaching terminal would make the seed degenerate).
const oracleSeedPlies = 8

// oracleForcedSeed walks forward from mk using the exact oracle, but ONLY
// through plies where the mover's optimalSet has exactly one member — a
// genuine forced reply (any deviation is provably worse), not an arbitrary
// tie-break among several equally-good moves. This is the literal ground
// truth "forced-reply/parity logic" the two failed heuristic attempts tried
// to approximate from current marks alone: a cell three drops from opening
// that only ever gets filled by one player, no matter which of several
// tied-optimal lines is actually played, is "already decided" in exactly
// this sense.
//
// It returns a COPY of mk with x<cell>/o<cell> set for every cell filled
// along that forced prefix — including cells still buried under gravity in
// mk (p<cell> for them is untouched, and does not need to change: the
// evaluation net's win-line detectors read x<cell>/o<cell> directly, not
// p<cell>, so a seeded mark on a not-yet-open cell is legible to them without
// the drop transition that would normally place it ever needing to fire).
// x_turn, o_turn, win_x, win_o and every place not named above are left
// exactly as in mk. This is the "seed a place before the solve, bypassing
// derivation from the instantaneous marking" the experiment plan asks for:
// the walk that computes the seed is discrete and exact (m.fire/m.fireHouse
// against the declared model, not the ODE relaxation), and happens entirely
// before odeFinal is ever called on the result.
//
// canonical, when true, does not stop at a tie: it takes optimalSet[0] (the
// row-major-first optimal move, since legalMoves/optimalSet always iterate
// `cells` in that fixed order — deterministic, not a random tie-break) and
// keeps projecting. That is a strictly stronger cheat than the forced-only
// walk above: it resolves genuine in-game choices, not just forced replies,
// so it is reported separately (see README) as the absolute upper bound
// rather than the literal "forced-reply/parity" fact the plan asked for —
// useful for telling apart "the representation has a ceiling" from "this
// experiment's predicate was too conservative to reach it".
func (m *model) oracleForcedSeed(mk marking, maxPlies int, canonical bool) marking {
	seeded := make(marking, len(mk))
	for k, v := range mk {
		seeded[k] = v
	}
	cur := mk
	for ply := 0; ply < maxPlies; ply++ {
		cur = m.fireHouse(cur)
		if cur["win_x"] > 0 || cur["win_o"] > 0 {
			break
		}
		_, moves, maximizes, ok := m.legalMoves(cur)
		if !ok {
			break
		}
		optimal := m.optimalSet(cur, moves, maximizes)
		if len(optimal) != 1 && !canonical {
			break // real choice, not a forced reply: stop, don't guess
		}
		mv := optimal[0]
		cell := mv[len(mv)-2:]
		side := "o"
		if maximizes {
			side = "x"
		}
		if seeded[side+cell] == 0 {
			seeded[side+cell] = 1
		}
		cur = m.fire(mv, cur)
	}
	return seeded
}

// odeOraclePlayer is odePlayer (policy.go) with one change: before the
// single ODE solve that scores each candidate move, the resulting marking is
// passed through oracleForcedSeed. Net structure, rates, horizon and score
// readout (win_x - win_o for X; x_turn + o_turn + lam*win_o for O) are
// unchanged from every other evaluator in this file — the only thing this
// variant adds is what the flow integral is allowed to start from.
func odeOraclePlayer(ev evalNet, lam float64, maxPlies int, canonical bool) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			seeded := m.oracleForcedSeed(m.fire(mv, mk), maxPlies, canonical)
			f := m.odeFinal(ev.net, seeded, ev.rates)
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
