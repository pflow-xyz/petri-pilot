// Gradient calibration of the derived champion. The policy's strength is
// one number appearing 48 times — every blk_* copy AddCatalyzedCopy
// derived carries the same block bias — and the gradient knows that: the
// 48 transitions share one learn.SharedScalar, so a sensitivity solve
// carries a single parameter column that accumulates every copy's
// contribution. One augmented solve per candidate move replaces the
// 2P+1 plain solves a finite difference would need, and lambda's
// derivative is analytic (it multiplies a final-state read, not a rate),
// so it costs nothing at all.
//
// Training solves the augmented system adaptively while play solves the
// plain net; the two scores agree within solver tolerance, not exactly —
// the exhaustive referee (main.go fitgrad) is the acceptance gate.
package main

import (
	"math"

	"github.com/pflow-xyz/go-pflow/learn"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// rankMargin mirrors the hinge margin fit.go passes to HingeRankLoss.
const rankMargin = 0.0005

// champGrad is the champion evaluation net with a learnable tied bias:
// rate-1 constants at every kept declared transition and win detector,
// and the ONE *SharedScalar installed at all 48 blk_* copies.
type champGrad struct {
	net  *petri.PetriNet
	rfs  map[string]learn.RateFunc
	bias *learn.SharedScalar
}

// toChampionGrad derives the same net as toPetriChampion and wires its
// rates as learnable functions — the structure is identical (fitgrad_test
// pins it); only the bias is a parameter, so NumParams is exactly 1.
func (m *model) toChampionGrad(bias float64) *champGrad {
	net, blks := m.deriveChampionNet()
	rfs := make(map[string]learn.RateFunc, len(m.transitions)+len(blks))
	for _, t := range m.transitions {
		if t != "draw" {
			rfs[t] = learn.NewConstantRateFunc(1)
		}
	}
	b := learn.NewSharedScalar(bias)
	for _, blk := range blks {
		rfs[blk] = b
	}
	return &champGrad{net: net, rfs: rfs, bias: b}
}

// scoreGrad scores one candidate position with one sensitivity solve and
// returns the seat objective s together with ds/dbias and ds/dlambda.
// The state is built odeFinal-style: marking keys copied (extra keys like
// move_tokens ride as inert state — parity with play, do not "optimize"
// it away), net places zero-seeded. Options are odeFinal's exactly, with
// Maxiters raised to 2000 for the augmented system. ok is false when the
// solve truncates or errors.
func (cg *champGrad) scoreGrad(m *model, mk marking, maximizes bool, lam float64) (s, dsdb, dsdlam float64, ok bool) {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
	for p := range cg.net.Places {
		if _, has := state[p]; !has {
			state[p] = 0
		}
	}
	prob := learn.NewLearnableProblem(cg.net, state, [2]float64{0, odeHorizon}, cg.rfs)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 2000, Adaptive: true,
	}
	sens, err := prob.SolveWithSensitivities(nil, opts)
	if err != nil || sens.Truncated {
		return 0, 0, 0, false
	}
	K := len(sens.T) - 1
	f := sens.Sol.GetFinalState()
	sv := func(pl string) float64 {
		d, _ := sens.At(K, pl, 0)
		return d
	}
	if maximizes {
		// X: win_x - win_o. Lambda does not enter.
		s = f["win_x"] - f["win_o"]
		dsdb = sv("win_x") - sv("win_o")
		dsdlam = 0
	} else {
		// O: x_turn + o_turn + lam*win_o. Lambda multiplies a final-state
		// read, so its derivative is the read itself — analytic, no solve.
		s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
		dsdb = sv("x_turn") + sv("o_turn") + lam*sv("win_o")
		dsdlam = f["win_o"]
	}
	return s, dsdb, dsdlam, true
}

// evalDecisions scores every candidate at every position (m.fire without
// fireHouse — rankLoss's convention; the win detectors run continuously
// inside the ODE) and returns the ranked decisions with per-option
// (ds/dbias, ds/dlambda). solves counts sensitivity solves; ok is false
// when any solve fails.
func evalDecisions(m *model, cg *champGrad, positions []trainPos, lam float64) (decisions []learn.RankedDecision, dbs, dlams [][]float64, solves int, ok bool) {
	decisions = make([]learn.RankedDecision, 0, len(positions))
	dbs = make([][]float64, 0, len(positions))
	dlams = make([][]float64, 0, len(positions))
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores:    make([]float64, len(p.moves)),
			Preferred: make([]bool, len(p.moves)),
		}
		db := make([]float64, len(p.moves))
		dl := make([]float64, len(p.moves))
		for i, mv := range p.moves {
			s, gb, gl, sok := cg.scoreGrad(m, m.fire(mv, p.mk), p.maximizes, lam)
			solves++
			if !sok {
				return nil, nil, nil, solves, false
			}
			d.Scores[i], db[i], dl[i] = s, gb, gl
			d.Preferred[i] = p.optimal[mv]
		}
		decisions = append(decisions, d)
		dbs = append(dbs, db)
		dlams = append(dlams, dl)
	}
	return decisions, dbs, dlams, solves, true
}

// hingeSubgrad assembles the subgradient of HingeRankLoss over the given
// decisions from per-option score derivatives. Definitions, pinned here
// and in fitgrad_test:
//   - active decision (violation v_d > 0): dL_d/dθ = ds_{j*}/dθ - ds_{i*}/dθ,
//     with i* the best preferred and j* the best non-preferred option;
//   - at the kink (v_d == 0): 0, the inactive branch — lets adam settle at
//     exactly-satisfied margins;
//   - argmax ties: the smallest index among maximizers (matching
//     HingeRankLoss's strict-> scan), so the closure is a pure function of θ.
func hingeSubgrad(decisions []learn.RankedDecision, dbs, dlams [][]float64, margin float64) (dLdb, dLdlam float64) {
	for di, d := range decisions {
		iPref, iNon := -1, -1
		// Mirror HingeRankLoss's ragged-slice bound so a decision the loss
		// tolerates cannot panic the subgradient.
		n := min(len(d.Scores), len(d.Preferred))
		for i := 0; i < n; i++ {
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
		if margin+d.Scores[iNon]-d.Scores[iPref] > 0 {
			dLdb += dbs[di][iNon] - dbs[di][iPref]
			dLdlam += dlams[di][iNon] - dlams[di][iPref]
		}
	}
	return dLdb, dLdlam
}

// rankLossGrad is rankLoss with derivatives: the loss VALUE goes through
// learn.HingeRankLoss on the sensitivity-solve scores (so the two
// objectives cannot diverge in form), the gradient through hingeSubgrad.
// A failed solve returns NaN loss.
func rankLossGrad(m *model, cg *champGrad, positions []trainPos, lam float64) (loss, dLdb, dLdlam float64, solves int) {
	decisions, dbs, dlams, solves, ok := evalDecisions(m, cg, positions, lam)
	if !ok {
		return math.NaN(), 0, 0, solves
	}
	loss = learn.HingeRankLoss(decisions, rankMargin)
	dLdb, dLdlam = hingeSubgrad(decisions, dbs, dlams, rankMargin)
	return loss, dLdb, dLdlam, solves
}

// fitChampionGrad optimizes u = (log bias, log lambda) from (0, 0) with
// learn.MinimizeGradient (adam). The log-space chain rule is
// grad_u L = (b*dL/db, lam*dL/dlam). Non-finite loss or gradient rejects
// the point. Returns the fitted pair and the total sensitivity-solve
// count (each worth 2 plain-solve equivalents at P = 1).
func fitChampionGrad(m *model, positions []trainPos, iters int, verbose bool) (bias, lam float64, sensSolves int) {
	fg := func(u []float64) (float64, []float64) {
		b, l := math.Exp(u[0]), math.Exp(u[1])
		cg := m.toChampionGrad(b)
		loss, dLdb, dLdlam, solves := rankLossGrad(m, cg, positions, l)
		sensSolves += solves
		if math.IsNaN(loss) || math.IsInf(loss, 0) {
			return math.Inf(1), nil
		}
		g := []float64{b * dLdb, l * dLdlam}
		for _, gv := range g {
			if math.IsNaN(gv) || math.IsInf(gv, 0) {
				return math.Inf(1), nil
			}
		}
		return loss, g
	}
	opts := learn.DefaultFitOptions()
	opts.Method = "" // adam
	opts.MaxIters = iters
	opts.Tolerance = 1e-9
	opts.GradTol = 1e-9
	opts.LearnRate = 0.1
	opts.Verbose = verbose
	res, err := learn.MinimizeGradient(fg, []float64{0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(res.Params[0]), math.Exp(res.Params[1]), sensSolves
}

// fitChampionCounted is fitChampion's Nelder-Mead baseline with an
// objective-call counter, kept here so fit.go stays untouched. Each f
// call costs one plain solve per (position, candidate) pair.
func fitChampionCounted(m *model, positions []trainPos, iters int, verbose bool, fCalls *int) (bias, lam float64) {
	f := func(logp []float64) float64 {
		*fCalls++
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
