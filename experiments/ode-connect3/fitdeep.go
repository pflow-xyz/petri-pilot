// Retrying finding 7's tuning idea, but scored through finding 8's lookahead
// evaluator instead of a single static solve. Deliberately uses Nelder-Mead
// (fit.go's cheap gradient-free optimizer) over the same 3 global scalars
// fit.go already tunes, NOT the tied-group sensitivity-gradient machinery
// fitgrad.go built for finding 7. Two reasons:
//
//   - fitgrad.go's cost was already the bottleneck at 1 static solve per
//     candidate (~0.5s/solve for a sensitivity solve). Differentiating
//     through odeSearchScore's nested min/max would need a sensitivity solve
//     per REPLY the worst-case scan considers (the envelope theorem lets you
//     propagate through only the achieving branch, but you still have to
//     solve every branch to find it), multiplying an already-expensive
//     operation by the branching factor per ply. Not worth it here.
//   - finding 7's result was that MORE tuning freedom on a fixed evaluator
//     made things worse (Goodhart). The open question this file answers is
//     narrower and cheaper to test: does retuning the SAME 3 scalars fit.go
//     always used, now against a strictly more informative evaluator, do
//     better than it did against the shallow one? That does not need finer
//     groups or gradients — Nelder-Mead over 3 scalars, scored via
//     odeSearchScore, answers it directly.
package main

import (
	"math"

	"github.com/pflow-xyz/go-pflow/learn"
)

// rankLossDeep is fit.go's rankLoss with each candidate's score computed by
// odeSearchScore at the given depth instead of one odeFinal solve.
func rankLossDeep(m *model, positions []trainPos, winBias, blockBias, lam float64, plies int) float64 {
	ev := m.toPetriPolicy(winBias, blockBias)
	decisions := make([]learn.RankedDecision, 0, len(positions))
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves)),
		}
		for i, mv := range p.moves {
			d.Scores[i] = m.odeSearchScore(ev, m.fire(mv, p.mk), lam, p.maximizes, plies)
			d.Preferred[i] = p.optimal[mv]
		}
		decisions = append(decisions, d)
	}
	return learn.HingeRankLoss(decisions, 0.0005)
}

// fitPolicyDeep is fit.go's fitPolicy, scored via rankLossDeep.
func fitPolicyDeep(m *model, positions []trainPos, plies, iters int, verbose bool) (winBias, blockBias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLossDeep(m, positions, math.Exp(logp[0]), math.Exp(logp[1]), math.Exp(logp[2]), plies)
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters, opts.Tolerance, opts.Verbose = iters, 1e-9, verbose
	result, err := learn.Minimize(f, []float64{0, 0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(result.Params[0]), math.Exp(result.Params[1]), math.Exp(result.Params[2])
}
