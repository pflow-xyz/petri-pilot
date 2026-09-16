// The exact referee and ODE bridge.
package main

import (
	"math/rand"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

type player func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string

var odeHorizon = 0.5

// minimax returns the exact game value: +1 X win, -1 O win, 0 draw.
// Exact memoization is simpler here than caching alpha-beta bounds.
func (m *model) minimax(mk marking, _, _ int) int {
	mk = m.fireHouse(mk)
	if mk["win_x"] > 0 {
		return 1
	}
	if mk["win_o"] > 0 {
		if m.hasLine(mk, "o") {
			return -1
		}
		return 0
	}
	_, moves, maximizes, ok := m.legalMoves(mk)
	if !ok {
		return 0
	}
	key := boardKey(mk)
	if maximizes {
		key += "X"
	} else {
		key += "O"
	}
	if v, ok := m.minimaxMemo[key]; ok {
		return v
	}
	best := -2
	if !maximizes {
		best = 2
	}
	for _, mv := range moves {
		v := m.minimax(m.fire(mv, mk), -2, 2)
		if maximizes && v > best || !maximizes && v < best {
			best = v
		}
		if maximizes && best == 1 || !maximizes && best == -1 {
			break
		}
	}
	m.minimaxMemo[key] = best
	return best
}

func (m *model) optimalSet(mk marking, moves []string, maximizes bool) []string {
	best := -2
	if !maximizes {
		best = 2
	}
	values := make(map[string]int, len(moves))
	for _, mv := range moves {
		v := m.minimax(m.fire(mv, mk), -2, 2)
		values[mv] = v
		if maximizes && v > best || !maximizes && v < best {
			best = v
		}
	}
	out := make([]string, 0, len(moves))
	for _, mv := range moves {
		if values[mv] == best {
			out = append(out, mv)
		}
	}
	return out
}

func minimaxPlayer(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string {
	optimal := m.optimalSet(mk, moves, maximizes)
	if rng == nil {
		return optimal[0]
	}
	return optimal[rng.Intn(len(optimal))]
}

func (m *model) odeFinal(net *petri.PetriNet, mk marking, rates map[string]float64) map[string]float64 {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
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
