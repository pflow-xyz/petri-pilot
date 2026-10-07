package mcp

import (
	"fmt"
	"math"

	"github.com/pflow-xyz/go-pflow/learn"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// fitAdam is petri_fit's gradient path: the same squared-error loss over
// scattered per-place observations, with its exact gradient from one
// analytic forward-sensitivity solve per evaluation (go-pflow
// learn.SolveWithSensitivities), minimized by learn's bias-corrected
// Adam. Bounds are enforced the same way as the Nelder-Mead path — a
// hard clamp at evaluation time — with the gradient taken at the clamped
// point.
func fitAdam(
	net *petri.PetriNet,
	initial map[string]float64,
	tspan [2]float64,
	fixedRates map[string]float64,
	paramOrder []string,
	bounds map[string][2]float64,
	observations []fitObservation,
	x0 []float64,
	maxIter int,
	tol float64,
) (bestX []float64, bestLoss float64, iters int, converged bool, evals int, err error) {
	fitted := map[string]bool{}
	for _, k := range paramOrder {
		fitted[k] = true
	}
	rfs := make(map[string]learn.RateFunc, len(fixedRates))
	for tid, v := range fixedRates {
		if fitted[tid] {
			rfs[tid] = learn.NewScalarRateFunc(v)
		} else {
			rfs[tid] = learn.NewConstantRateFunc(v)
		}
	}
	lp := learn.NewLearnableProblem(net, initial, tspan, rfs)
	_, indices := lp.GetAllParams()
	thetaIdx := make([]int, len(paramOrder))
	for i, k := range paramOrder {
		rng, ok := indices[k]
		if !ok {
			return nil, 0, 0, false, 0, fmt.Errorf("parameter %q not in learnable problem", k)
		}
		thetaIdx[i] = rng[0]
	}

	clamp := func(k string, v float64) float64 {
		if v < bounds[k][0] {
			return bounds[k][0]
		}
		if v > bounds[k][1] {
			return bounds[k][1]
		}
		return v
	}

	fg := func(x []float64) (float64, []float64) {
		evals++
		for i, k := range paramOrder {
			rfs[k].SetParams([]float64{clamp(k, x[i])})
		}
		sens, serr := lp.SolveWithSensitivities(solver.Tsit5(), solver.JSParityOptions())
		if serr != nil || sens == nil || len(sens.T) == 0 {
			return math.Inf(1), make([]float64, len(x))
		}
		lossVal := 0.0
		grad := make([]float64, len(x))
		for _, obs := range observations {
			sim := interpolate(sens.T, sens.Sol.GetVariable(obs.Place), obs.T)
			d := sim - obs.V
			lossVal += d * d
			for i := range paramOrder {
				s := interpolateSensitivity(sens, obs.Place, thetaIdx[i], obs.T)
				grad[i] += 2 * d * s
			}
		}
		return lossVal, grad
	}

	opts := learn.DefaultFitOptions()
	opts.Method = "adam"
	opts.MaxIters = maxIter
	opts.GradTol = tol
	// DefaultFitOptions' loss-delta convergence check (Tolerance=1e-4) is a
	// second, unconfigurable stopping rule alongside GradTol: it reports
	// "converged" the moment consecutive-iteration loss stops moving by
	// more than 1e-4, which a coarser adaptive step (correct, but noisier
	// evaluation-to-evaluation than a needlessly over-refined solver) can
	// trip far from the true optimum — observed on coffeeShopModel after
	// go-pflow v0.26.0's Tsit5 fix: Adam plateaued at loss 0.0102 after 99
	// evals (rates off by up to 21%) where Nelder-Mead reached 4.6e-6.
	// GradTol is this tool's one exposed, meaningful stopping criterion
	// (via petri_fit's "tol" argument), so the loss-delta check is
	// tightened to the point it can no longer fire first.
	opts.Tolerance = 1e-10
	res, merr := learn.MinimizeGradient(fg, x0, opts)
	if merr != nil {
		return nil, 0, 0, false, evals, merr
	}
	out := make([]float64, len(paramOrder))
	for i, k := range paramOrder {
		out[i] = clamp(k, res.Params[i])
	}
	return out, res.FinalLoss, res.Iterations, res.Converged, evals, nil
}

// interpolateSensitivity linearly interpolates ∂x_place/∂θ_param at time t
// over the sensitivity solve's own grid — the same interpolation rule the
// loss uses for the state, applied to its derivative.
func interpolateSensitivity(sens *learn.Sensitivities, place string, param int, t float64) float64 {
	ts := sens.T
	if len(ts) == 0 {
		return 0
	}
	at := func(k int) float64 {
		v, _ := sens.At(k, place, param)
		return v
	}
	if t <= ts[0] {
		return at(0)
	}
	if t >= ts[len(ts)-1] {
		return at(len(ts) - 1)
	}
	lo, hi := 0, len(ts)-1
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if ts[mid] <= t {
			lo = mid
		} else {
			hi = mid
		}
	}
	dt := ts[hi] - ts[lo]
	if dt == 0 {
		return at(lo)
	}
	frac := (t - ts[lo]) / dt
	return at(lo) + frac*(at(hi)-at(lo))
}

// analyticElasticities computes d(observable at t_end)/d(rate_t) for every
// transition from one augmented forward-sensitivity solve, converted to the
// same dimensionless elasticity petri_ode_sensitivity's finite-difference
// path reports: (dE/E)/(dk/k), with the absolute form when the base value
// is ~0.
func analyticElasticities(
	net *petri.PetriNet,
	initial map[string]float64,
	tspan [2]float64,
	baseRates map[string]float64,
	observable string,
	baseEq float64,
) (map[string]float64, error) {
	rfs := make(map[string]learn.RateFunc, len(baseRates))
	for tid, v := range baseRates {
		rfs[tid] = learn.NewScalarRateFunc(v)
	}
	lp := learn.NewLearnableProblem(net, initial, tspan, rfs)
	_, indices := lp.GetAllParams()
	sens, err := lp.SolveWithSensitivities(solver.Tsit5(), solver.JSParityOptions())
	if err != nil {
		return nil, err
	}
	if len(sens.T) == 0 {
		return nil, fmt.Errorf("empty sensitivity solution")
	}
	last := len(sens.T) - 1
	out := make(map[string]float64, len(baseRates))
	for tid, rate := range baseRates {
		rng, ok := indices[tid]
		if !ok {
			return nil, fmt.Errorf("transition %q not in learnable problem", tid)
		}
		dEdk, ok := sens.At(last, observable, rng[0])
		if !ok {
			return nil, fmt.Errorf("no sensitivity row for observable %q", observable)
		}
		if math.Abs(baseEq) > 1e-9 {
			out[tid] = dEdk * rate / baseEq
		} else {
			out[tid] = dEdk * rate
		}
	}
	return out, nil
}
