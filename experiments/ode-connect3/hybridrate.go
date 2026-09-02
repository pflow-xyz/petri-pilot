// The hybrid experiment: declare the structure (the same derivePolicyNet
// used everywhere else — 144 force_* + 144 blk_* transitions, no new
// structure), but let each family's RATE be a genuine regression over board
// state instead of a hand-picked constant. This is what findings 7 and 11
// argue FOR by elimination: finding 7 showed re-tying the SAME constant
// into finer groups Goodharts; finding 11 showed throwing the structure
// away entirely for an unconstrained MLP loses badly and more data doesn't
// fix it. Neither tried what go-pflow already ships for exactly this: a
// GradRateFunc whose rate is a LEARNED FUNCTION of live board state,
// installed on the declared structure, trained through the SAME
// SolveWithSensitivities/MinimizeGradient machinery fitgrad.go already
// uses — no hand-rolled backprop this time.
//
// Feature choice matters here more than the RateFunc machinery. The
// catalyzed-copy transitions already read the opponent's other marks in the
// line (as catalysts) and the landing token for their own cell (as the
// gravity precondition) — both already enter the mass-action product, so a
// RateFunc reading them again is redundant. What ISN'T read anywhere is the
// rest of the board: which OTHER columns are filled how deep. That's the
// closest cheap proxy to the "future support" predicate findings 6/8
// diagnosed (a defense that depends on which upper cells become available
// several drops later, elsewhere on the board) without hand-deriving the
// exact predicate — so both families' rate reads all 16 landing-token
// places p<c>, tied across the whole family (2 RateFuncs total: one win,
// one block), not per-transition or per-group. This is deliberately the
// SMALLEST version of this experiment: 2*(16+1)=34 board-regression
// parameters plus lambda, fewer than finding 7's `row` scheme (16) and
// vastly fewer than finding 11's smallest Neural ODE (232). If a cheap
// linear regression over already-available structure can't move the
// referee number at all, a bigger one is unlikely to be the answer either.
package main

import (
	"math"
	"math/rand"

	"github.com/pflow-xyz/go-pflow/learn"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

func l2Norm(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

// boardPlaceNames is the fixed feature set every hybrid RateFunc reads: the
// 16 landing-token places, in `cells` order.
func boardPlaceNames() []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = "p" + c
	}
	return out
}

// toHybridRateNet derives the same structure toPetriPolicy/toPolicyGradGrouped
// use, but installs two tied learn.LinearRateFuncs (one per family) instead
// of a constant. Init: bias 1 (a mild, non-zero starting rate, matching
// every other experiment's convention of starting near "uniform"), all
// board-feature weights 0 — so the fit starts exactly at the constant-rate
// baseline and only departs from it if the referee-independent hinge loss
// finds a reason to.
func (m *model) toHybridRateNet() (net *petri.PetriNet, rfs map[string]learn.RateFunc, winRF, blkRF *learn.LinearRateFunc, repWin, repBlk string) {
	net, winGroups, blkGroups := m.derivePolicyNetGrouped(groupSingle, groupSingle)
	places := boardPlaceNames()
	winInit := make([]float64, len(places)+1)
	blkInit := make([]float64, len(places)+1)
	winInit[0], blkInit[0] = 1, 1
	winRF = learn.NewLinearRateFunc(places, winInit, false, false)
	blkRF = learn.NewLinearRateFunc(places, blkInit, false, false)

	rfs = make(map[string]learn.RateFunc, len(net.Transitions))
	for t := range net.Transitions {
		rfs[t] = learn.NewConstantRateFunc(1)
	}
	for t := range winGroups {
		rfs[t] = winRF
		if repWin == "" {
			repWin = t
		}
	}
	for t := range blkGroups {
		rfs[t] = blkRF
		if repBlk == "" {
			repBlk = t
		}
	}
	return net, rfs, winRF, blkRF, repWin, repBlk
}

// hybridPlayer scores each candidate by solving the LIVE (non-sensitivity)
// LearnableProblem — unlike odePlayer/odeLookaheadPlayer's plain
// solver.NewProblem with a static rates map, a LinearRateFunc's rate
// depends on the live state during integration, so it needs
// LearnableProblem.Solve, not odeFinal.
func hybridPlayer(net *petri.PetriNet, rfs map[string]learn.RateFunc, lam float64) player {
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 1000, Adaptive: true,
	}
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", math.Inf(-1)
		for _, mv := range moves {
			state := make(map[string]float64, len(mk)+1)
			for k, v := range m.fire(mv, mk) {
				state[k] = float64(v)
			}
			for p := range net.Places {
				if _, ok := state[p]; !ok {
					state[p] = 0
				}
			}
			prob := learn.NewLearnableProblem(net, state, [2]float64{0, odeHorizon}, rfs)
			f := prob.Solve(nil, opts).GetFinalState()
			s := f["win_x"] - f["win_o"]
			if !maximizes {
				s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
			}
			if best == "" || s > bestScore {
				best, bestScore = mv, s
			}
		}
		return best
	}
}

// hybridScoreGrad scores one candidate with one sensitivity solve and
// returns ds/d(winParams), ds/d(blkParams) alongside the score — mirrors
// fitgrad.go's scoreGrad/policyGrad.scoreGrad, generalized to 2 fixed-size
// tied RateFuncs read via go-pflow's own ParamIndex instead of a hand-built
// param map.
func hybridScoreGrad(m *model, net *petri.PetriNet, rfs map[string]learn.RateFunc, repWin, repBlk string, mk marking, lam float64, maximizes bool) (s float64, dWin, dBlk []float64, dsdlam float64, ok bool) {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
	for p := range net.Places {
		if _, has := state[p]; !has {
			state[p] = 0
		}
	}
	prob := learn.NewLearnableProblem(net, state, [2]float64{0, odeHorizon}, rfs)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 2000, Adaptive: true,
	}
	sens, err := prob.SolveWithSensitivities(nil, opts)
	if err != nil || sens.Truncated {
		return 0, nil, nil, 0, false
	}
	K := len(sens.T) - 1
	f := sens.Sol.GetFinalState()
	winBlk, wok := sens.ParamIndex[repWin]
	blkBlk, bok := sens.ParamIndex[repBlk]
	if !wok || !bok {
		return 0, nil, nil, 0, false
	}
	sv := func(place string, p int) float64 {
		d, _ := sens.At(K, place, p)
		return d
	}
	dWin = make([]float64, winBlk[1]-winBlk[0])
	dBlk = make([]float64, blkBlk[1]-blkBlk[0])
	if maximizes {
		s = f["win_x"] - f["win_o"]
		for i := range dWin {
			dWin[i] = sv("win_x", winBlk[0]+i) - sv("win_o", winBlk[0]+i)
		}
		for i := range dBlk {
			dBlk[i] = sv("win_x", blkBlk[0]+i) - sv("win_o", blkBlk[0]+i)
		}
	} else {
		s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
		for i := range dWin {
			dWin[i] = sv("x_turn", winBlk[0]+i) + sv("o_turn", winBlk[0]+i) + lam*sv("win_o", winBlk[0]+i)
		}
		for i := range dBlk {
			dBlk[i] = sv("x_turn", blkBlk[0]+i) + sv("o_turn", blkBlk[0]+i) + lam*sv("win_o", blkBlk[0]+i)
		}
		dsdlam = f["win_o"]
	}
	return s, dWin, dBlk, dsdlam, true
}

// fitHybridRate trains winRF/blkRF's parameters plus lambda with Adam.
// Weights are NOT log-space (they're signed, like neuralode.go's), lambda
// is (matching every other fit in this package).
func fitHybridRate(m *model, positions []trainPos, iters int, verbose bool) (net *petri.PetriNet, rfs map[string]learn.RateFunc, winRF, blkRF *learn.LinearRateFunc, lam float64) {
	net, rfs, winRF, blkRF, repWin, repBlk := m.toHybridRateNet()
	nWin, nBlk := winRF.NumParams(), blkRF.NumParams()

	fg := func(u []float64) (float64, []float64) {
		winRF.SetParams(u[:nWin])
		blkRF.SetParams(u[nWin : nWin+nBlk])
		l := math.Exp(u[nWin+nBlk])

		decisions := make([]learn.RankedDecision, 0, len(positions))
		dWins := make([][]float64, 0, len(positions))
		dBlks := make([][]float64, 0, len(positions))
		dLams := make([][]float64, 0, len(positions))
		for _, p := range positions {
			d := learn.RankedDecision{Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves))}
			dw := make([]float64, 0, len(p.moves)*nWin)
			db := make([]float64, 0, len(p.moves)*nBlk)
			dl := make([]float64, len(p.moves))
			for i, mv := range p.moves {
				s, gw, gb, gl, sok := hybridScoreGrad(m, net, rfs, repWin, repBlk, m.fire(mv, p.mk), l, p.maximizes)
				if !sok {
					return math.Inf(1), nil
				}
				d.Scores[i], d.Preferred[i] = s, p.optimal[mv]
				dw = append(dw, gw...)
				db = append(db, gb...)
				dl[i] = gl
			}
			decisions = append(decisions, d)
			dWins = append(dWins, dw)
			dBlks = append(dBlks, db)
			dLams = append(dLams, dl)
		}
		loss := learn.HingeRankLoss(decisions, rankMargin)
		if math.IsNaN(loss) || math.IsInf(loss, 0) {
			return math.Inf(1), nil
		}
		dLdWin := make([]float64, nWin)
		dLdBlk := make([]float64, nBlk)
		dLdlam := 0.0
		for di, d := range decisions {
			iPref, iNon := -1, -1
			nOpt := min(len(d.Scores), len(d.Preferred))
			for i := 0; i < nOpt; i++ {
				if d.Preferred[i] {
					if iPref < 0 || d.Scores[i] > d.Scores[iPref] {
						iPref = i
					}
				} else if iNon < 0 || d.Scores[i] > d.Scores[iNon] {
					iNon = i
				}
			}
			if iPref < 0 || iNon < 0 {
				continue
			}
			if rankMargin+d.Scores[iNon]-d.Scores[iPref] > 0 {
				for k := 0; k < nWin; k++ {
					dLdWin[k] += dWins[di][iNon*nWin+k] - dWins[di][iPref*nWin+k]
				}
				for k := 0; k < nBlk; k++ {
					dLdBlk[k] += dBlks[di][iNon*nBlk+k] - dBlks[di][iPref*nBlk+k]
				}
				dLdlam += dLams[di][iNon] - dLams[di][iPref]
			}
		}
		g := make([]float64, nWin+nBlk+1)
		copy(g[:nWin], dLdWin)
		copy(g[nWin:nWin+nBlk], dLdBlk)
		g[nWin+nBlk] = l * dLdlam
		return loss, g
	}

	opts := learn.DefaultFitOptions()
	opts.Method = "" // adam
	opts.MaxIters = iters
	opts.Tolerance = 1e-9
	opts.GradTol = 1e-9
	opts.LearnRate = 0.05
	opts.Verbose = verbose
	u0 := make([]float64, nWin+nBlk+1)
	copy(u0[:nWin], winRF.GetParams())
	copy(u0[nWin:nWin+nBlk], blkRF.GetParams())
	res, err := learn.MinimizeGradient(fg, u0, opts)
	if err != nil {
		panic(err)
	}
	winRF.SetParams(res.Params[:nWin])
	blkRF.SetParams(res.Params[nWin : nWin+nBlk])
	return net, rfs, winRF, blkRF, math.Exp(res.Params[nWin+nBlk])
}
