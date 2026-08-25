// The referee and the solver bridge.
//
// minimaxPlayer: exact alpha-beta over the net's discrete semantics.
// Leaves score win +1, loss -1, draw 0 — the three outcomes must rank for
// "perfect play" to mean anything (the declared objective folds draw into
// the defender's win, making X indifferent between drawing and losing).
//
// odeFinal: one mass-action solve of an evaluation net from a discrete
// marking, returning the final continuous state. Marking keys the net
// does not declare are inert state; net places the marking lacks are
// zero-seeded (dropping them silently was a real bug once).
package main

import (
	"math/rand"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

type player func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string

// odeHorizon is the ODE solve span.
var odeHorizon = 3.0

// ---- exact minimax ----

// minimax returns the game value of mk with alpha-beta pruning: +1 X
// wins, -1 O wins, 0 draw, from exact play by both sides.
func (m *model) minimax(mk marking, alpha, beta int) int {
	mk = m.fireHouse(mk)
	if mk["win_x"] > 0 {
		return 1
	}
	if mk["win_o"] > 0 {
		if m.hasLine(mk, "o") {
			return -1
		}
		return 0 // called draw
	}
	_, moves, maximizes, ok := m.legalMoves(mk)
	if !ok {
		return 0
	}
	if maximizes {
		best := -2
		for _, mv := range moves {
			v := m.minimax(m.fire(mv, mk), alpha, beta)
			if v > best {
				best = v
			}
			if best > alpha {
				alpha = best
			}
			if alpha >= beta {
				break
			}
		}
		return best
	}
	best := 2
	for _, mv := range moves {
		v := m.minimax(m.fire(mv, mk), alpha, beta)
		if v < best {
			best = v
		}
		if best < beta {
			beta = best
		}
		if alpha >= beta {
			break
		}
	}
	return best
}

// optimalSet returns every move achieving the position's minimax value.
func (m *model) optimalSet(mk marking, moves []string, maximizes bool) []string {
	type scored struct {
		mv string
		v  int
	}
	best := -2
	if !maximizes {
		best = 2
	}
	all := make([]scored, 0, len(moves))
	for _, mv := range moves {
		v := m.minimax(m.fire(mv, mk), -2, 2)
		all = append(all, scored{mv, v})
		if maximizes && v > best || !maximizes && v < best {
			best = v
		}
	}
	out := make([]string, 0, len(all))
	for _, s := range all {
		if s.v == best {
			out = append(out, s.mv)
		}
	}
	return out
}

// minimaxPlayer picks uniformly among the optimal set, so 100 games
// explore many optimal lines rather than one.
func minimaxPlayer(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string {
	opt := m.optimalSet(mk, moves, maximizes)
	return opt[rng.Intn(len(opt))]
}

// ---- the ODE bridge ----

// odeFinal solves the net from mk and returns the final continuous state.
func (m *model) odeFinal(net *petri.PetriNet, mk marking, rates map[string]float64) map[string]float64 {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
	// Places the net declares beyond the model's marking must still enter
	// the solve, or their mass is silently dropped.
	for p := range net.Places {
		if _, ok := state[p]; !ok {
			state[p] = 0
		}
	}
	prob := solver.NewProblem(net, state, [2]float64{0, odeHorizon}, rates)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 1000, Adaptive: true,
	}
	return solver.Solve(prob, solver.Tsit5(), opts).GetFinalState()
}
