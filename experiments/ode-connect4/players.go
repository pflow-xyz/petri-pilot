// The ODE bridge and the oracle-backed exact player.
//
// positionFromMarking translates the Petri marking into the oracle's
// bitboard Position (same (col,row) indexing as the net's gravity chain, so
// the two representations never have to be kept in sync by hand — they
// share cellKey/openPlace naming with model.go).
//
// odeFinal mirrors ode-minimax's: one mass-action solve of an evaluation net
// from a discrete marking, returning the final continuous state.
package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

type player func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string

// odeHorizon is the ODE solve span; the net is much larger than
// tic-tac-toe's (131 declared places, more with catalyzed copies), so the
// horizon and solver options below lean on solver.GameAIOptions()'s speed
// preset rather than accuracy.
var odeHorizon = 3.0

// positionFromMarking reads the mover's bitboard Position off a marking.
// moverIsX says whose stones become `current`.
func positionFromMarking(mk marking, moverIsX bool) Position {
	mine, other := "x", "o"
	if !moverIsX {
		mine, other = "o", "x"
	}
	var p Position
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			bit := uint64(1) << uint(c*h1+r)
			if mk[cellPlace(mine, c, r)] > 0 {
				p.current |= bit
				p.mask |= bit
				p.moves++
			} else if mk[cellPlace(other, c, r)] > 0 {
				p.mask |= bit
				p.moves++
			}
		}
	}
	return p
}

// ---- the ODE bridge ----

// odeFinal solves the net from mk and returns the final continuous state.
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
	opts := solver.GameAIOptions()
	return solver.Solve(prob, solver.Tsit5(), opts).GetFinalState()
}

// ---- oracle-backed exact player (the referee) ----

// oracleValueAfter returns the position value from the perspective of the
// player who is about to move in mk (i.e. before mv is played), evaluated
// by first firing mv then, if the game continues, negating the opponent's
// value in the resulting position. ok=false means the oracle's node budget
// was exhausted; the caller must not treat that as a value of 0.
func (m *model) oracleValueAfter(o *Oracle, mk marking, mv string, budget int) (val int8, ok bool) {
	next := m.fireHouse(m.fire(mv, mk))
	if v, done := m.terminal(next); done {
		return int8(v), true
	}
	_, nextMaximizes, _ := m.legalMoves(next)
	p := positionFromMarking(next, nextMaximizes)
	opp, solved := o.Solve(p, budget)
	if !solved {
		return 0, false
	}
	return -opp, true
}

// oracleOptimalSet returns every legal move at mk achieving the position's
// exact value, or ok=false if any candidate could not be solved within
// budget (a partial optimal set is worse than none — callers must know the
// position wasn't fully resolved).
func (m *model) oracleOptimalSet(o *Oracle, mk marking, moves []string, budget int) (best int8, optimal []string, ok bool) {
	type scored struct {
		mv string
		v  int8
	}
	all := make([]scored, 0, len(moves))
	best = -2
	for _, mv := range moves {
		v, solved := m.oracleValueAfter(o, mk, mv, budget)
		if !solved {
			return 0, nil, false
		}
		all = append(all, scored{mv, v})
		if v > best {
			best = v
		}
	}
	for _, s := range all {
		if s.v == best {
			optimal = append(optimal, s.mv)
		}
	}
	return best, optimal, true
}

// oraclePlayer picks uniformly among the exact-optimal set. Positions that
// exceed the budget fall back to the first legal move and are reported by
// the caller via the returned ok — used only in exploration (self-play
// sampling), never as ground truth for a labeled training/verification
// position.
func oraclePlayer(o *Oracle, budget int) (play player, unsolved *int) {
	miss := 0
	p := func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string {
		_, opt, ok := m.oracleOptimalSet(o, mk, moves, budget)
		if !ok {
			miss++
			return moves[rng.Intn(len(moves))]
		}
		return opt[rng.Intn(len(opt))]
	}
	return p, &miss
}

// ---- generic evaluation-net player ----

type evalNet struct {
	net   *petri.PetriNet
	rates map[string]float64
}

// scoreFn scores a final ODE state for the mover named by maximizes (true
// for X).
type scoreFn func(final map[string]float64, maximizes bool) float64

func evalPlayer(m *model, ev evalNet, score scoreFn) player {
	_ = m
	return func(mm *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			f := mm.odeFinal(ev.net, mm.fire(mv, mk), ev.rates)
			s := score(f, maximizes)
			if i == 0 || s > bestScore {
				best, bestScore = mv, s
			}
		}
		return best
	}
}

// naiveScore: the unmodified final-state coordinates, symmetric in the two
// seats — win_x - win_o for X, win_o - win_x for O.
func naiveScore(f map[string]float64, maximizes bool) float64 {
	if maximizes {
		return f["win_x"] - f["win_o"]
	}
	return f["win_o"] - f["win_x"]
}

// ---- misc: positions, keys, tournaments ----

// positionFromMoves builds a marking by firing a sequence of columns
// alternately (X first), house-firing (win detectors) after each move.
// Panics if a move ends the game before the sequence completes or if a
// column is full — these are constructed by this file's own code and by
// fit.go's sampler, not by untrusted input.
func (m *model) positionFromMoves(cols []int) marking {
	mk := m.start()
	for i, c := range cols {
		side := "x"
		if i%2 == 1 {
			side = "o"
		}
		moves, _, ok := m.legalMoves(mk)
		if !ok {
			panic(fmt.Sprintf("positionFromMoves: game already over before move %d", i))
		}
		var mv string
		for _, cand := range moves {
			if moveColumn(cand) == c && strings.HasPrefix(cand, side+"_play_") {
				mv = cand
				break
			}
		}
		if mv == "" {
			panic(fmt.Sprintf("positionFromMoves: column %d not legal for %s at move %d", c, side, i))
		}
		mk = m.fireHouse(m.fire(mv, mk))
	}
	return mk
}

// boardKey renders a marking as a compact string for dedup/caching: 42
// characters, column-major, '.' empty, 'X'/'O' owned.
func boardKey(mk marking) string {
	var b strings.Builder
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			switch {
			case mk[cellPlace("x", c, r)] > 0:
				b.WriteByte('X')
			case mk[cellPlace("o", c, r)] > 0:
				b.WriteByte('O')
			default:
				b.WriteByte('.')
			}
		}
	}
	return b.String()
}

func (m *model) playGame(xs, os player, rng *rand.Rand) string {
	mk := m.start()
	for {
		mk = m.fireHouse(mk)
		if v, done := m.terminal(mk); done {
			switch v {
			case 1:
				return "X"
			case -1:
				return "O"
			default:
				return "draw"
			}
		}
		moves, maximizes, _ := m.legalMoves(mk)
		p := os
		if maximizes {
			p = xs
		}
		mk = m.fire(p(m, mk, moves, maximizes, rng), mk)
	}
}

func tournament(m *model, name string, xs, os player, games int, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	wins := map[string]int{}
	for i := 0; i < games; i++ {
		wins[m.playGame(xs, os, rng)]++
	}
	fmt.Printf("%-32s games=%d  X: %d  O: %d  draws: %d\n",
		name, games, wins["X"], wins["O"], wins["draw"])
}
