// odeLookaheadPlayer: one real ply of search with the existing static ODE
// solve as leaf evaluator, instead of ensembling several same-depth solves.
// A single odeFinal call cannot see "how many drops until a threatened cell
// opens, and whose turn it is then" (README finding 6/7) — it is a snapshot
// at one horizon. Nesting one more ply of the exact game lets the mover's
// score reflect what the opponent can actually do about it, which averaging
// several static evaluations (horizons, biases, random inits) cannot supply:
// those vary the read, not the information available to it.
package main

import (
	"math"
	"math/rand"
)

// odeLeafEval scores a (post-move) position exactly as odePlayer does: one
// ODE solve, no discrete fireHouse step first (the net's own catalytic win
// detectors evolve inside the relaxation) — same convention as toPetriPolicy/
// toPetriBaseline scoring in policy.go and rankLoss in fit.go.
func odeLeafEval(m *model, ev evalNet, mk marking, lam float64, maximizes bool) float64 {
	f := m.odeFinal(ev.net, mk, ev.rates)
	if maximizes {
		return f["win_x"] - f["win_o"]
	}
	return f["x_turn"] + f["o_turn"] + lam*f["win_o"]
}

// odeSearchScore is odeLookaheadPlayer's leaf logic generalized to `plies`
// alternating turns of real search before falling back to odeLeafEval — 0
// plies is a plain odePlayer-style solve, 1 ply is odeLookaheadPlayer's
// worst-reply scan, 2 plies lets the root mover answer that reply before the
// leaf read, and so on. mk is always a post-move, not-yet-house-fired state
// (fire's convention throughout this package); rootMaximizes is fixed to the
// player whose score is being computed, never the side currently to move.
func (m *model) odeSearchScore(ev evalNet, mk marking, lam float64, rootMaximizes bool, plies int) float64 {
	next := m.fireHouse(mk)
	if next["win_x"] > 0 || next["win_o"] > 0 || plies <= 0 {
		return odeLeafEval(m, ev, mk, lam, rootMaximizes)
	}
	_, moves, maximizes, ok := m.legalMoves(next)
	if !ok {
		return odeLeafEval(m, ev, mk, lam, rootMaximizes)
	}
	rootToMove := maximizes == rootMaximizes
	best := math.Inf(-1)
	if !rootToMove {
		best = math.Inf(1)
	}
	for _, mv := range moves {
		s := m.odeSearchScore(ev, m.fire(mv, next), lam, rootMaximizes, plies-1)
		if rootToMove {
			if s > best {
				best = s
			}
		} else if s < best {
			best = s
		}
	}
	return best
}

// odeSearchPlayer scores each candidate with odeSearchScore at the given
// depth. odeLookaheadPlayer(ev, lam) == odeSearchPlayer(ev, lam, 1).
func odeSearchPlayer(ev evalNet, lam float64, plies int) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", math.Inf(-1)
		for _, mv := range moves {
			s := m.odeSearchScore(ev, m.fire(mv, mk), lam, maximizes, plies)
			if best == "" || s > bestScore {
				best, bestScore = mv, s
			}
		}
		return best
	}
}

// odeLookaheadPlayer is the depth-1 case: fire the candidate, then take the
// worst leaf score across every legal opponent reply (a terminal position
// scores directly). Kept as its own name — the finding-8 result was
// reported against it — but it is exactly odeSearchPlayer(ev, lam, 1).
func odeLookaheadPlayer(ev evalNet, lam float64) player {
	return odeSearchPlayer(ev, lam, 1)
}
