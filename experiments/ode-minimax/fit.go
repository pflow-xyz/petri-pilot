// Calibration and the exhaustive referee. The generic halves —
// gradient-free optimization and the hinge ranking loss — live in
// go-pflow/learn (Minimize, HingeRankLoss); this file keeps only what
// knows the game: sampling positions, labeling them with minimax, and
// walking every opponent line.
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
	optimal   map[string]bool
}

// collectPositions samples distinct decision points from random
// self-play and labels them with minimax-optimal move sets. The four
// tactical audit positions are always included.
func collectPositions(m *model, games int, seed int64) []trainPos {
	rng := rand.New(rand.NewSource(seed))
	seen := map[string]bool{}
	var out []trainPos

	add := func(mk marking) {
		mk = m.fireHouse(mk)
		key := boardKey(mk)
		if seen[key] {
			return
		}
		_, moves, maximizes, ok := m.legalMoves(mk)
		if !ok {
			return
		}
		seen[key] = true
		opt := map[string]bool{}
		for _, mv := range m.optimalSet(mk, moves, maximizes) {
			opt[mv] = true
		}
		out = append(out, trainPos{mk, moves, maximizes, opt})
	}

	add(m.position([]string{"00"}, nil, "o"))                              // corner reply
	add(m.position([]string{"11"}, nil, "o"))                              // center reply
	add(m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o")) // must-block
	add(m.position([]string{"00", "22"}, []string{"11"}, "o"))             // double-corner fork
	for g := 0; g < games; g++ {
		mk := m.start()
		for {
			mk = m.fireHouse(mk)
			if mk["win_x"] > 0 || mk["win_o"] > 0 {
				break
			}
			_, moves, _, ok := m.legalMoves(mk)
			if !ok {
				break
			}
			add(mk)
			mk = m.fire(moves[rng.Intn(len(moves))], mk)
		}
	}
	return out
}

// rankLoss scores every candidate at every training position on the
// champion net at (bias, lam) and returns the hinge ranking loss
// against the minimax labels.
func rankLoss(m *model, positions []trainPos, bias, lam float64) float64 {
	ev := m.toPetriChampion(bias)
	decisions := make([]learn.RankedDecision, 0, len(positions))
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores:    make([]float64, len(p.moves)),
			Preferred: make([]bool, len(p.moves)),
		}
		for i, mv := range p.moves {
			f := m.odeFinal(ev.net, m.fire(mv, p.mk), ev.rates)
			s := f["win_x"] - f["win_o"]
			if !p.maximizes {
				s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
			}
			d.Scores[i] = s
			d.Preferred[i] = p.optimal[mv]
		}
		decisions = append(decisions, d)
	}
	return learn.HingeRankLoss(decisions, 0.0005)
}

// fitChampion optimizes (blockBias, lambda) in log space from (1, 1)
// with learn.Minimize and returns the fitted pair.
func fitChampion(m *model, positions []trainPos, iters int, verbose bool) (bias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLoss(m, positions, math.Exp(logp[0]), math.Exp(logp[1]))
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

// exhaustiveCheck walks every legal opponent line with the evaluator on
// one seat and reports (distinct decisions, game-losing moves, missed
// wins) — the referee behind the verify and fit modes.
func exhaustiveCheck(m *model, p player, variantIsX bool) (decisions, blown, missed int) {
	cache := map[string]string{}
	seen := map[string]bool{}
	var walk func(mk marking)
	walk = func(mk marking) {
		mk = m.fireHouse(mk)
		if mk["win_x"] > 0 || mk["win_o"] > 0 {
			return
		}
		_, moves, maximizes, ok := m.legalMoves(mk)
		if !ok {
			return
		}
		if maximizes == variantIsX {
			key := boardKey(mk)
			mv, hit := cache[key]
			if !hit {
				mv = p(m, mk, moves, maximizes, nil)
				cache[key] = mv
			}
			if !seen[key] {
				seen[key] = true
				decisions++
				before := m.minimax(mk, -2, 2)
				after := m.minimax(m.fireHouse(m.fire(mv, mk)), -2, 2)
				worse, lost := after > before, after == 1
				if variantIsX {
					worse, lost = after < before, after == -1
				}
				if worse {
					if lost {
						blown++
					} else {
						missed++
					}
				}
			}
			walk(m.fire(mv, mk))
			return
		}
		for _, mv := range moves {
			walk(m.fire(mv, mk))
		}
	}
	walk(m.start())
	return
}
