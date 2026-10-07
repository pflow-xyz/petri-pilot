// fitparity.go: calibrate parity.go's new rate constants against labeled
// positions, using fit.go's exact Nelder-Mead + hinge rank loss pattern
// (learn.Minimize / learn.HingeRankLoss), applied to toPetriParity instead
// of toPetriPolicy.
package main

import (
	"math"

	"github.com/pflow-xyz/go-pflow/learn"
)

func rankLossParity(m *model, positions []trainPos, winBias, blockBias, parityForceBias, parityBlockBias, tempoBias, lam float64) float64 {
	ev := m.toPetriParity(winBias, blockBias, parityForceBias, parityBlockBias, tempoBias)
	decisions := make([]learn.RankedDecision, 0, len(positions))
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves)),
		}
		for i, mv := range p.moves {
			f := m.odeFinal(ev.net, parityAugment(m.fire(mv, p.mk)), ev.rates)
			score := f["win_x"] - f["win_o"]
			if !p.maximizes {
				score = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
			}
			d.Scores[i], d.Preferred[i] = score, p.optimal[mv]
		}
		decisions = append(decisions, d)
	}
	return learn.HingeRankLoss(decisions, 0.0005)
}

// fitParityOnly holds winBias/blockBias/lam at the given (champion) values
// and fits only the three new rate constants -- the minimal-freedom
// experiment finding 7/12 both argue for: add one construction, calibrate
// only its own knobs, leave the already-calibrated tier alone.
func fitParityOnly(m *model, positions []trainPos, winBias, blockBias, lam float64, iters int, verbose bool) (parityForceBias, parityBlockBias, tempoBias float64) {
	f := func(logp []float64) float64 {
		return rankLossParity(m, positions, winBias, blockBias, math.Exp(logp[0]), math.Exp(logp[1]), math.Exp(logp[2]), lam)
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters, opts.Tolerance, opts.Verbose = iters, 1e-9, verbose
	result, err := learn.Minimize(f, []float64{0, 0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(result.Params[0]), math.Exp(result.Params[1]), math.Exp(result.Params[2])
}

// fitTempoOnly holds everything else at the champion/untuned values and
// fits only tempoBias -- the single scalar the manual scan (README finding
// 13) singled out as the only live knob in this tier.
func fitTempoOnly(m *model, positions []trainPos, winBias, blockBias, parityForceBias, parityBlockBias, lam float64, iters int, verbose bool) (tempoBias float64) {
	f := func(logp []float64) float64 {
		return rankLossParity(m, positions, winBias, blockBias, parityForceBias, parityBlockBias, math.Exp(logp[0]), lam)
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters, opts.Tolerance, opts.Verbose = iters, 1e-9, verbose
	result, err := learn.Minimize(f, []float64{0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(result.Params[0])
}

// fitPolicyParity is the joint 6-parameter fit (winBias, blockBias,
// parityForceBias, parityBlockBias, tempoBias, lam) -- kept as a comparison
// point, not the primary experiment: finding 7 showed more simultaneous
// freedom on this same 288-transition base already trades hinge loss for
// referee errors, so re-diffusing the champion's own already-calibrated
// winBias/blockBias at the same time carries that same risk.
func fitPolicyParity(m *model, positions []trainPos, iters int, verbose bool) (winBias, blockBias, parityForceBias, parityBlockBias, tempoBias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLossParity(m, positions, math.Exp(logp[0]), math.Exp(logp[1]), math.Exp(logp[2]), math.Exp(logp[3]), math.Exp(logp[4]), math.Exp(logp[5]))
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters, opts.Tolerance, opts.Verbose = iters, 1e-9, verbose
	result, err := learn.Minimize(f, []float64{0, 0, 0, 0, 0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(result.Params[0]), math.Exp(result.Params[1]), math.Exp(result.Params[2]), math.Exp(result.Params[3]), math.Exp(result.Params[4]), math.Exp(result.Params[5])
}
