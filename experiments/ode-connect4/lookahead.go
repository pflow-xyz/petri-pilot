// N-ply minimax with the champion ODE evaluator as the leaf/static
// evaluation function — README finding 8.
//
// FRAMING SHIFT, stated plainly because it matters: everything else in this
// experiment (findings 1-7) is a SEARCH-FREE evaluator — one ODE solve per
// candidate move, zero game-tree expansion beyond that. This file is a
// different kind of thing: ordinary game-tree search (minimax, full-width,
// no pruning) with the ODE relaxation standing in for the position-value
// function an ordinary game engine would hand-write. That is exactly the
// "shallow search + static eval" architecture chess/checkers engines have
// used for decades; the only novelty here is that the "static eval" is
// itself an ODE solve rather than a weighted material count. Do not read
// any number in this file as evidence about the search-free claim in
// findings 1-7 — it is a claim about a different, strictly more expensive
// approach, deliberately investigated to find out whether spending search
// budget closes the gap that adding more STRUCTURE (findings 6-7) could not.
//
// Ply-counting convention (stated explicitly because "N-ply" is ambiguous
// across engines): N counts FULL-WIDTH PLIES SEARCHED AFTER the candidate
// move already being evaluated. So:
//
//	N=0: evaluate each candidate move by firing it and leaf-scoring the
//	     resulting position directly — one ODE solve per candidate move,
//	     the same shape (not the same formula — see leafScore) as the
//	     existing 0-ply champion.
//	N=1: for each candidate move, expand ALL of the opponent's replies at
//	     the resulting position and take the min/max of their leaf scores
//	     — 1 ODE solve per (my move, opponent's reply) pair, so roughly
//	     branching-factor-times (~7x) the cost of N=0 per decision.
//	N=2: one more full-width ply (my own reply to the opponent's reply)
//	     before leaf-scoring — roughly 7x more than N=1, ~49x N=0.
//
// This matches an ordinary engine's "search depth" once you note the
// player's own candidate move is always ply zero of the search tree; N
// here is how many MORE plies are explored below it before falling back to
// the static evaluator.
package main

import (
	"math"
	"math/rand"
)

// leafScore is the static (no-lookahead) position value used at the bottom
// of the search: ONE ODE solve of the champion-structured net (fixed
// blockBias, matching the shipped 0-ply default) directly from mk, scored
// as f[win_x] - f[win_o]. This is deliberately a single SYMMETRIC scalar,
// positive favoring X and negative favoring O — unlike championScore
// (players.go/champion.go), which uses two DIFFERENT formulas for X and O
// (win_x-win_o for X; x_turn+o_turn+lambda*win_o for O) because the 0-ply
// champion only ever ranks candidate moves for ONE side at a time and never
// needs its two formulas to agree on a shared scale. A minimax search does
// need that: the maximizing and minimizing sides must be pushing the same
// number in opposite directions, or "best for me" and "worst for you" stop
// meaning the same thing. So this file drops lambda's O-specific
// undecided-mass weighting entirely rather than trying to force it onto a
// shared scale it wasn't built for — see README finding 8 for the
// consequences of that choice.
func leafScore(m *model, ev evalNet, mk marking) float64 {
	f := m.odeFinal(ev.net, mk, ev.rates)
	return f["win_x"] - f["win_o"]
}

// searchMinimax evaluates mk from the shared X-favoring scale, searching
// `depth` more full-width plies before falling back to leafScore. Terminal
// positions short-circuit to their exact discrete value (free, and exact —
// no reason to approximate a decided game with an ODE solve).
func searchMinimax(m *model, ev evalNet, mk marking, depth int) float64 {
	if v, done := m.terminal(mk); done {
		return float64(v)
	}
	if depth <= 0 {
		return leafScore(m, ev, mk)
	}
	moves, maximizes, ok := m.legalMoves(mk)
	if !ok {
		return 0 // terminal() already handles this; defensive only
	}
	best := math.Inf(-1)
	if !maximizes {
		best = math.Inf(1)
	}
	for _, mv := range moves {
		child := m.fireHouse(m.fire(mv, mk))
		v := searchMinimax(m, ev, child, depth-1)
		if maximizes {
			if v > best {
				best = v
			}
		} else {
			if v < best {
				best = v
			}
		}
	}
	return best
}

// lookaheadPlayer picks the candidate move whose resulting position scores
// best (for the mover) after searching `plies` additional full-width plies
// with searchMinimax, using ev as the leaf evaluator throughout (the SAME
// net/rates at every level, not re-derived per node).
func lookaheadPlayer(ev evalNet, plies int) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			child := m.fireHouse(m.fire(mv, mk))
			v := searchMinimax(m, ev, child, plies)
			if i == 0 || (maximizes && v > bestScore) || (!maximizes && v < bestScore) {
				best, bestScore = mv, v
			}
		}
		return best
	}
}
