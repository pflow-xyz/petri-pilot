// Position sampling, oracle labeling, hinge-loss calibration, and the
// sampled (not exhaustive) referee.
//
// The single biggest structural difference from ode-minimax: tic-tac-toe's
// referee walked the ENTIRE reachable game tree (a few thousand states) and
// so "0 game-losing moves" was a claim about every position the game can
// ever reach. Connect Four's reachable set is on the order of 10^12
// positions; nothing here walks all of it, and nothing below should be read
// as if it did. Every count this file produces is over an explicit, stated
// sample size, plus the oracle's own honesty check (Solve's ok=false), so a
// position that could not be resolved in budget is never silently scored.
package main

import (
	"math"
	"math/rand"

	"github.com/pflow-xyz/go-pflow/learn"
)

type trainPos struct {
	mk        marking
	moves     []string
	maximizes bool
	best      int8
	optimal   map[string]bool
	tactical  bool // true if a strictly worse alternative existed (a "must-play" decision)
}

// collectPositions samples distinct decision points, biased toward
// mid-to-late game so the oracle mostly stays inside its node budget (see
// oracle.go's honesty note). It plays `games` random self-play games from
// the empty board, and for every decision point with at least minDiscs
// stones already on the board, asks the oracle for the exact value of every
// legal move. Positions the oracle cannot resolve within budget are
// dropped and counted in `skipped` rather than guessed at.
func collectPositions(m *model, o *Oracle, games, minDiscs, budget int, seed int64) (positions []trainPos, skipped int) {
	rng := rand.New(rand.NewSource(seed))
	seen := map[string]bool{}

	add := func(mk marking) {
		key := boardKey(mk)
		if seen[key] {
			return
		}
		moves, maximizes, ok := m.legalMoves(mk)
		if !ok {
			return
		}
		if countDiscs(mk) < minDiscs {
			return
		}
		best, opt, solved := m.oracleOptimalSet(o, mk, moves, budget)
		if !solved {
			skipped++
			return
		}
		seen[key] = true
		optSet := map[string]bool{}
		for _, mv := range opt {
			optSet[mv] = true
		}
		positions = append(positions, trainPos{mk, moves, maximizes, best, optSet, len(opt) < len(moves)})
	}

	for g := 0; g < games; g++ {
		mk := m.start()
		for {
			mk = m.fireHouse(mk)
			if _, done := m.terminal(mk); done {
				break
			}
			moves, _, ok := m.legalMoves(mk)
			if !ok {
				break
			}
			add(mk)
			mk = m.fire(moves[rng.Intn(len(moves))], mk)
		}
	}
	return positions, skipped
}

func countDiscs(mk marking) int {
	n := 0
	for c := 0; c < Width; c++ {
		for r := 0; r < Height; r++ {
			if mk[cellPlace("x", c, r)] > 0 || mk[cellPlace("o", c, r)] > 0 {
				n++
			}
		}
	}
	return n
}

// rankLoss scores every candidate move at every training position on the
// champion net at (blockBias, winBias, parityBias, lam) and returns the
// hinge ranking loss against the oracle's optimal-move labels.
func rankLoss(m *model, positions []trainPos, blockBias, winBias, parityBias, lam float64) float64 {
	ev := m.toPetriChampion(blockBias, winBias, parityBias)
	score := championScore(lam)
	decisions := make([]learn.RankedDecision, 0, len(positions))
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores:    make([]float64, len(p.moves)),
			Preferred: make([]bool, len(p.moves)),
		}
		for i, mv := range p.moves {
			f := m.odeFinal(ev.net, m.fire(mv, p.mk), ev.rates)
			d.Scores[i] = score(f, p.maximizes)
			d.Preferred[i] = p.optimal[mv]
		}
		decisions = append(decisions, d)
	}
	return learn.HingeRankLoss(decisions, 0.0005)
}

// fitChampion optimizes (blockBias, winBias, lambda) in log space from
// (1, 1, 1) with learn.Minimize (Nelder-Mead, gradient-free) against the
// oracle labels; parityBias is held at 0 (the prt_* tier is silent).
func fitChampion(m *model, positions []trainPos, iters int, verbose bool) (blockBias, winBias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLoss(m, positions, math.Exp(logp[0]), math.Exp(logp[1]), 0, math.Exp(logp[2]))
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters = iters
	opts.Tolerance = 1e-9
	opts.Verbose = verbose
	res, err := learn.Minimize(f, []float64{0, 0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(res.Params[0]), math.Exp(res.Params[1]), math.Exp(res.Params[2])
}

// fitChampionBlockParity optimizes (blockBias, parityBias, lambda) in log
// space from (1, 1, 1), holding winBias at 0 — README finding 7's "alongside
// blockBias" configuration: the theory-motivated parity tier ADDED to the
// existing block-bias tier rather than replacing it.
func fitChampionBlockParity(m *model, positions []trainPos, iters int, verbose bool) (blockBias, parityBias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLoss(m, positions, math.Exp(logp[0]), 0, math.Exp(logp[1]), math.Exp(logp[2]))
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters = iters
	opts.Tolerance = 1e-9
	opts.Verbose = verbose
	res, err := learn.Minimize(f, []float64{0, 0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(res.Params[0]), math.Exp(res.Params[1]), math.Exp(res.Params[2])
}

// fitChampionParityOnly optimizes (parityBias, lambda) in log space from
// (1, 1), holding blockBias AND winBias at 0 — README finding 7's "full
// replacement for blockBias" configuration: the theory-motivated parity
// tier standing in for the untargeted block-bias tier entirely.
func fitChampionParityOnly(m *model, positions []trainPos, iters int, verbose bool) (parityBias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLoss(m, positions, 0, 0, math.Exp(logp[0]), math.Exp(logp[1]))
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters = iters
	opts.Tolerance = 1e-9
	opts.Verbose = verbose
	res, err := learn.Minimize(f, []float64{0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(res.Params[0]), math.Exp(res.Params[1])
}

// sampledCheck runs player p on every position in `positions` (NOT the
// whole game tree — see the file-level note) and reports, over the
// positions p actually had to decide (its own seat only):
//
//	decisions: how many
//	blown:     the chosen move's oracle value is a loss when a non-losing
//	           move (best > -1) existed
//	missed:    the position was a forced win (best == 1) and the chosen
//	           move only draws or worse (value 0, not a loss)
//
// Positions whose oracle re-check (of the chosen move) can't be resolved in
// budget are counted in `unresolved` and excluded from the other three.
func sampledCheck(m *model, o *Oracle, p player, positions []trainPos, budget int, variantIsX bool) (decisions, blown, missed, unresolved int) {
	for _, pos := range positions {
		if pos.maximizes != variantIsX {
			continue
		}
		decisions++
		choice := p(m, pos.mk, pos.moves, pos.maximizes, nil)
		val, ok := m.oracleValueAfter(o, pos.mk, choice, budget)
		if !ok {
			unresolved++
			decisions--
			continue
		}
		if val < pos.best {
			if val == -1 {
				blown++
			} else if pos.best == 1 {
				missed++
			}
		}
	}
	return
}
