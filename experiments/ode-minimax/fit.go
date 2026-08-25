// Gradient-fitting the policy-bias constants instead of hand-fitting
// them — the follow-up finding 9 asks for. The parameter vector is the
// tied triple (winBias, blockBias, lam); the loss is a hinge ranking
// loss against minimax labels over a training set of positions sampled
// from random self-play; the optimizer is Nelder-Mead in log-space
// (rates are positive), matching go-pflow/learn's gradient-free
// approach — its own Fit entry point assumes a single-trajectory MSE
// loss, which a many-position ranking loss cannot be expressed as.
//
// The question this answers: do the hand-found constants fall out of
// optimization from a naive start, or were they luck?
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
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

	add(m.position([]string{"00"}, nil, "o"))                                     // corner reply
	add(m.position([]string{"11"}, nil, "o"))                                     // center reply
	add(m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o"))        // must-block
	add(m.position([]string{"00", "22"}, []string{"11"}, "o"))                    // double-corner fork
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

// rankLoss: per position, the worst hinge violation of "some optimal
// move must outscore every non-optimal move by margin".
func rankLoss(m *model, positions []trainPos, bias, lam float64) float64 {
	const margin = 0.0005
	ev := m.toPetriChampion(bias)
	loss := 0.0
	for _, p := range positions {
		bestOpt, bestNon := math.Inf(-1), math.Inf(-1)
		for _, mv := range p.moves {
			f := m.odeFinal(ev.net, m.fire(mv, p.mk), ev.rates)
			s := f["win_x"] - f["win_o"]
			if !p.maximizes {
				s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
			}
			if p.optimal[mv] {
				if s > bestOpt {
					bestOpt = s
				}
			} else if s > bestNon {
				bestNon = s
			}
		}
		if !math.IsInf(bestNon, -1) {
			if v := margin + bestNon - bestOpt; v > 0 {
				loss += v
			}
		}
	}
	return loss
}

// nelderMead minimizes f over log-space parameters (all params positive).
func nelderMead(f func([]float64) float64, x0 []float64, iters int, verbose bool) []float64 {
	n := len(x0)
	simplex := make([][]float64, n+1)
	vals := make([]float64, n+1)
	for i := range simplex {
		pt := append([]float64{}, x0...)
		if i > 0 {
			pt[i-1] += 0.7 // log-space step ~ factor 2
		}
		simplex[i] = pt
		vals[i] = f(pt)
	}
	order := func() {
		idx := make([]int, n+1)
		for i := range idx {
			idx[i] = i
		}
		sort.Slice(idx, func(a, b int) bool { return vals[idx[a]] < vals[idx[b]] })
		ns, nv := make([][]float64, n+1), make([]float64, n+1)
		for i, j := range idx {
			ns[i], nv[i] = simplex[j], vals[j]
		}
		copy(simplex, ns)
		copy(vals, nv)
	}
	for it := 0; it < iters; it++ {
		order()
		if verbose && it%5 == 0 {
			exp := make([]float64, n)
			for i, v := range simplex[0] {
				exp[i] = math.Exp(v)
			}
			fmt.Printf("  iter %3d  loss %.6f  params %.3v\n", it, vals[0], exp)
		}
		if vals[n]-vals[0] < 1e-9 {
			break
		}
		centroid := make([]float64, n)
		for i := 0; i < n; i++ {
			for d := 0; d < n; d++ {
				centroid[d] += simplex[i][d] / float64(n)
			}
		}
		lerp := func(a, b []float64, t float64) []float64 {
			out := make([]float64, n)
			for d := 0; d < n; d++ {
				out[d] = a[d] + t*(b[d]-a[d])
			}
			return out
		}
		refl := lerp(centroid, simplex[n], -1)
		fr := f(refl)
		switch {
		case fr < vals[0]:
			expd := lerp(centroid, simplex[n], -2)
			if fe := f(expd); fe < fr {
				simplex[n], vals[n] = expd, fe
			} else {
				simplex[n], vals[n] = refl, fr
			}
		case fr < vals[n-1]:
			simplex[n], vals[n] = refl, fr
		default:
			contr := lerp(centroid, simplex[n], 0.5)
			if fc := f(contr); fc < vals[n] {
				simplex[n], vals[n] = contr, fc
			} else {
				for i := 1; i <= n; i++ {
					simplex[i] = lerp(simplex[0], simplex[i], 0.5)
					vals[i] = f(simplex[i])
				}
			}
		}
	}
	order()
	return simplex[0]
}

// exhaustiveCheck walks every legal opponent line with the evaluator on
// one seat and reports (distinct decisions, game-losing moves, missed
// wins) — the same referee the verify mode prints.
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
