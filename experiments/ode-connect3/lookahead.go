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

// odeLookaheadPlayer scores each candidate by firing it, then — if the game
// is not over — enumerating every legal opponent reply and taking the WORST
// leaf score across those replies (the opponent plays to minimize the
// mover's score); a terminal position scores directly. One-ply minimax with
// odeLeafEval as the leaf, replacing odePlayer's single solve.
func odeLookaheadPlayer(ev evalNet, lam float64) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", math.Inf(-1)
		for _, mv := range moves {
			mine := m.fire(mv, mk)
			next := m.fireHouse(mine)
			score := math.Inf(1)
			if next["win_x"] > 0 || next["win_o"] > 0 {
				score = odeLeafEval(m, ev, mine, lam, maximizes)
			} else if _, replies, _, ok := m.legalMoves(next); !ok {
				score = odeLeafEval(m, ev, mine, lam, maximizes)
			} else {
				for _, r := range replies {
					s := odeLeafEval(m, ev, m.fire(r, mine), lam, maximizes)
					if s < score {
						score = s
					}
				}
			}
			if best == "" || score > bestScore {
				best, bestScore = mv, score
			}
		}
		return best
	}
}
