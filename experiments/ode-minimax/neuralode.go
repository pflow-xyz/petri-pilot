// A REAL Neural ODE, reproducing ode-connect3's neuralode.go here for
// comparison. Every other evaluator in this experiment is classical system
// identification: a fixed mass-action RHS dictated by the declared Petri
// net's stoichiometry (derive.AddCatalyzedCopy's 48 forced-reply copies),
// with 1-2 physically-meaningful scalar rates fit to data. This file throws
// that structure away. dx/dt = MLP(x; theta) — an unconstrained function
// approximator with no wiring saying which cells threaten which win-line,
// nothing hand-derived.
//
// Scope, as in ode-connect3: the state actually integrated is the 4
// quantities every evaluator here reads out of a final state — win_x,
// win_o, x_turn, o_turn — conditioned on the 27 raw board features (p<c>,
// x<c>, o<c> per cell) held FIXED across the horizon. Training is
// discretize-then-optimize: forward with a fixed-step Euler scheme, exact
// backprop through the unrolled steps (not go-pflow's continuous adjoint,
// which is built for the RateFunc/mass-action framework). checkNeuralGrad
// verifies the hand-rolled backward pass against central finite differences
// before any of it is trusted.
//
// tic-tac-toe is a much smaller game than ode-connect3's gravity-constrained
// Connect-3 (9 cells vs 16, no gravity token layer, odeHorizon 3.0 vs 0.5),
// so this is also a test of whether ode-connect3's finding 11 (a real
// Neural ODE loses badly and more data/regularization doesn't close the
// gap) was specific to Connect-3's larger state space or holds here too.
package main

import (
	"math"
	"math/rand"

	"github.com/pflow-xyz/go-pflow/learn"
)

const (
	ndStateDim = 4 // [win_x, win_o, x_turn, o_turn], this fixed order throughout
	ndWinX     = 0
	ndWinO     = 1
	ndXTurn    = 2
	ndOTurn    = 3
)

// mlp is a 2-layer network: in -> tanh(H) -> linear(ndStateDim). in is
// boardDim (27, the raw cell features) + ndStateDim (the current dynamic
// state fed back in, mass-action's "state-dependent rate" analog).
type mlp struct {
	boardDim, h int
	w1          []float64 // h x in, row-major
	b1          []float64 // h
	w2          []float64 // ndStateDim x h, row-major
	b2          []float64 // ndStateDim
}

func (n *mlp) inDim() int { return n.boardDim + ndStateDim }

func newMLP(boardDim, h int, rng *rand.Rand) *mlp {
	in := boardDim + ndStateDim
	scale := func(fanIn int) float64 { return 1.0 / math.Sqrt(float64(fanIn)) }
	init := func(n int, s float64) []float64 {
		v := make([]float64, n)
		for i := range v {
			v[i] = (rng.Float64()*2 - 1) * s
		}
		return v
	}
	return &mlp{
		boardDim: boardDim, h: h,
		w1: init(h*in, scale(in)), b1: make([]float64, h),
		w2: init(ndStateDim*h, scale(h)), b2: make([]float64, ndStateDim),
	}
}

func (n *mlp) numParams() int { return len(n.w1) + len(n.b1) + len(n.w2) + len(n.b2) }

func (n *mlp) flatten() []float64 {
	out := make([]float64, 0, n.numParams())
	out = append(out, n.w1...)
	out = append(out, n.b1...)
	out = append(out, n.w2...)
	out = append(out, n.b2...)
	return out
}

func (n *mlp) load(theta []float64) {
	i := 0
	i += copy(n.w1, theta[i:])
	i += copy(n.b1, theta[i:])
	i += copy(n.w2, theta[i:])
	i += copy(n.b2, theta[i:])
}

// boardFeatures extracts the 27 raw board features (p<c>, x<c>, o<c> per
// cell, in `cells` order) as floats. Fixed across a trajectory — only the
// ndStateDim dynamic state evolves.
func boardFeatures(mk marking) []float64 {
	out := make([]float64, 0, 3*len(cells))
	for _, c := range cells {
		out = append(out, float64(mk["p"+c]), float64(mk["x"+c]), float64(mk["o"+c]))
	}
	return out
}

// forward evaluates f(x) = dx/dt at state x given fixed board features.
// Returns f, the pre-activation z1 and post-activation h1 (needed by the
// backward pass — recomputing them there would just duplicate this call).
func (n *mlp) forward(board, x []float64) (f, z1, h1 []float64) {
	in := make([]float64, 0, n.inDim())
	in = append(in, board...)
	in = append(in, x...)
	z1 = make([]float64, n.h)
	h1 = make([]float64, n.h)
	for i := 0; i < n.h; i++ {
		s := n.b1[i]
		row := i * len(in)
		for j, v := range in {
			s += n.w1[row+j] * v
		}
		z1[i] = s
		h1[i] = math.Tanh(s)
	}
	f = make([]float64, ndStateDim)
	for i := 0; i < ndStateDim; i++ {
		s := n.b2[i]
		row := i * n.h
		for j := 0; j < n.h; j++ {
			s += n.w2[row+j] * h1[j]
		}
		f[i] = s
	}
	return f, z1, h1
}

// ndTrajectory is one Euler-integrated forward pass: x[0]=x0 .. x[steps]
// alongside each step's h1 (post-tanh hidden activations), needed intact by
// the backward pass.
type ndTrajectory struct {
	x  [][]float64 // steps+1 entries, each ndStateDim
	h1 [][]float64 // steps entries, each h
	dt float64
}

func (n *mlp) integrate(board, x0 []float64, steps int, dt float64) *ndTrajectory {
	traj := &ndTrajectory{x: make([][]float64, steps+1), h1: make([][]float64, steps), dt: dt}
	traj.x[0] = append([]float64{}, x0...)
	for k := 0; k < steps; k++ {
		f, _, h1 := n.forward(board, traj.x[k])
		traj.h1[k] = h1
		next := make([]float64, ndStateDim)
		for i := range next {
			next[i] = traj.x[k][i] + dt*f[i]
		}
		traj.x[k+1] = next
	}
	return traj
}

// backward computes dL/dtheta given dL/dx at the final step, by reversing
// the Euler recursion x[k+1] = x[k] + dt*f(x[k]). Exact reverse-mode through
// the two dense layers per step (tanh derivative 1-h1^2 is standard).
// Returns the gradient in the same layout as mlp.flatten(), plus dL/dx0 (not
// used by the training loop here, but a natural byproduct and useful for
// checkNeuralGrad).
func (n *mlp) backward(board []float64, traj *ndTrajectory, dLdxFinal []float64) (grad []float64, dLdx0 []float64) {
	in := len(board) + ndStateDim
	gw1 := make([]float64, n.h*in)
	gb1 := make([]float64, n.h)
	gw2 := make([]float64, ndStateDim*n.h)
	gb2 := make([]float64, ndStateDim)

	g := append([]float64{}, dLdxFinal...) // dL/dx[k+1], starts at the final step
	steps := len(traj.h1)
	inbuf := make([]float64, in)
	copy(inbuf, board)
	for k := steps - 1; k >= 0; k-- {
		h1 := traj.h1[k]
		x := traj.x[k]
		copy(inbuf[len(board):], x)

		// dL/dout = dt*g  (x[k+1] = x[k] + dt*out)
		dOut := make([]float64, ndStateDim)
		for i := range dOut {
			dOut[i] = traj.dt * g[i]
		}
		// layer2: out = W2*h1 + b2
		dH1 := make([]float64, n.h)
		for i := 0; i < ndStateDim; i++ {
			row := i * n.h
			gb2[i] += dOut[i]
			for j := 0; j < n.h; j++ {
				gw2[row+j] += dOut[i] * h1[j]
				dH1[j] += n.w2[row+j] * dOut[i]
			}
		}
		// tanh
		dZ1 := make([]float64, n.h)
		for j := 0; j < n.h; j++ {
			dZ1[j] = dH1[j] * (1 - h1[j]*h1[j])
		}
		// layer1: z1 = W1*in + b1
		dIn := make([]float64, in)
		for i := 0; i < n.h; i++ {
			row := i * in
			gb1[i] += dZ1[i]
			for j := 0; j < in; j++ {
				gw1[row+j] += dZ1[i] * inbuf[j]
				dIn[j] += n.w1[row+j] * dZ1[i]
			}
		}
		// dL/dx[k] = g (direct, from x[k+1]=x[k]+...) + the dynamics part of dIn
		next := make([]float64, ndStateDim)
		for i := range next {
			next[i] = g[i] + dIn[len(board)+i]
		}
		g = next
	}
	grad = append(grad, gw1...)
	grad = append(grad, gb1...)
	grad = append(grad, gw2...)
	grad = append(grad, gb2...)
	return grad, g
}

// ndSteps/ndDt mirror odeHorizon at a fixed Euler step count. Cheap enough
// (two small dense layers, no adaptive solver) that a generous step count
// costs nothing. odeHorizon is 3.0 here (vs ode-connect3's 0.5), so more
// steps are used to hold the same per-step size (~0.0125) roughly constant.
const ndSteps = 240

func ndDt() float64 { return odeHorizon / ndSteps }

// ndInitialState builds x0 from the marking: win_x=win_o=0, x_turn/o_turn
// read off mk.
func ndInitialState(mk marking) []float64 {
	return []float64{0, 0, float64(mk["x_turn"]), float64(mk["o_turn"])}
}

func ndScore(f []float64, lam float64, maximizes bool) float64 {
	if maximizes {
		return f[ndWinX] - f[ndWinO]
	}
	return f[ndXTurn] + f[ndOTurn] + lam*f[ndWinO]
}

func ndScoreGrad(maximizes bool, lam float64) (dLdxFinal []float64) {
	dLdxFinal = make([]float64, ndStateDim)
	if maximizes {
		dLdxFinal[ndWinX], dLdxFinal[ndWinO] = 1, -1
		return dLdxFinal
	}
	dLdxFinal[ndXTurn], dLdxFinal[ndOTurn], dLdxFinal[ndWinO] = 1, 1, lam
	return dLdxFinal
}

// neuralODEPlayer: for each candidate, integrate the learned dynamics from
// the post-move marking and score exactly as championPlayer would from an
// ODE final read — the leaf logic is unchanged, only the dynamics engine is.
func neuralODEPlayer(n *mlp, lam float64) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", math.Inf(-1)
		for _, mv := range moves {
			mine := m.fire(mv, mk)
			board := boardFeatures(mine)
			traj := n.integrate(board, ndInitialState(mine), ndSteps, ndDt())
			s := ndScore(traj.x[ndSteps], lam, maximizes)
			if best == "" || s > bestScore {
				best, bestScore = mv, s
			}
		}
		return best
	}
}

// checkNeuralGrad verifies backward() against central finite differences on
// a small random net and trajectory. Returns the max absolute error over
// every parameter — call this before trusting any fit.
func checkNeuralGrad(rng *rand.Rand) float64 {
	n := newMLP(6, 5, rng)
	board := make([]float64, 6)
	for i := range board {
		board[i] = rng.Float64()
	}
	x0 := []float64{rng.Float64(), rng.Float64(), rng.Float64(), rng.Float64()}
	dLdxFinal := []float64{0.7, -1.3, 0.4, 2.1} // arbitrary fixed loss gradient
	loss := func() float64 {
		traj := n.integrate(board, x0, 6, 0.05)
		xf := traj.x[len(traj.x)-1]
		s := 0.0
		for i, g := range dLdxFinal {
			s += g * xf[i]
		}
		return s
	}
	traj := n.integrate(board, x0, 6, 0.05)
	analytic, _ := n.backward(board, traj, dLdxFinal)
	theta := n.flatten()
	maxErr := 0.0
	eps := 1e-5
	for i := range theta {
		orig := theta[i]
		theta[i] = orig + eps
		n.load(theta)
		lp := loss()
		theta[i] = orig - eps
		n.load(theta)
		lm := loss()
		theta[i] = orig
		n.load(theta)
		fd := (lp - lm) / (2 * eps)
		if err := math.Abs(fd - analytic[i]); err > maxErr {
			maxErr = err
		}
	}
	return maxErr
}

// evalNeuralDecisions / hingeSubgradFlat mirror ode-connect3's pattern:
// evaluate every candidate, find the best-preferred/best-non-preferred pair
// per decision, accumulate their score-gradient difference for any decision
// the hinge margin doesn't already satisfy — generalized to an arbitrary
// flat parameter vector rather than a handful of tied scalars.
func evalNeuralDecisions(m *model, n *mlp, positions []trainPos, lam float64) (decisions []learn.RankedDecision, grads [][][]float64, dlams [][]float64) {
	for _, p := range positions {
		d := learn.RankedDecision{Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves))}
		g := make([][]float64, len(p.moves))
		dl := make([]float64, len(p.moves))
		for i, mv := range p.moves {
			mine := m.fire(mv, p.mk)
			board := boardFeatures(mine)
			traj := n.integrate(board, ndInitialState(mine), ndSteps, ndDt())
			xf := traj.x[ndSteps]
			d.Scores[i] = ndScore(xf, lam, p.maximizes)
			d.Preferred[i] = p.optimal[mv]
			grad, _ := n.backward(board, traj, ndScoreGrad(p.maximizes, lam))
			g[i] = grad
			if !p.maximizes {
				dl[i] = xf[ndWinO]
			}
		}
		decisions = append(decisions, d)
		grads = append(grads, g)
		dlams = append(dlams, dl)
	}
	return decisions, grads, dlams
}

func hingeSubgradFlat(decisions []learn.RankedDecision, grads [][][]float64, dlams [][]float64, margin float64, numTheta int) (dLdTheta []float64, dLdlam float64) {
	dLdTheta = make([]float64, numTheta)
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
		if margin+d.Scores[iNon]-d.Scores[iPref] > 0 {
			for k := 0; k < numTheta; k++ {
				dLdTheta[k] += grads[di][iNon][k] - grads[di][iPref][k]
			}
			dLdlam += dlams[di][iNon] - dlams[di][iPref]
		}
	}
	return dLdTheta, dLdlam
}

// fitNeuralODE trains the MLP's weights plus lambda with Adam
// (learn.MinimizeGradient — generic over any flat parameter vector, so the
// mass-action-specific tooling elsewhere in this repo isn't needed here).
// h is the hidden width; lambda starts at 1 (log-space 0), weights at their
// random init (NOT log-space — unlike every scalar-bias fit elsewhere,
// network weights are signed).
func fitNeuralODE(m *model, positions []trainPos, h, iters int, l2 float64, verbose bool) (n *mlp, lam float64) {
	rng := rand.New(rand.NewSource(3))
	n = newMLP(3*len(cells), h, rng)
	numTheta := n.numParams()
	fg := func(u []float64) (float64, []float64) {
		n.load(u[:numTheta])
		l := math.Exp(u[numTheta])
		decisions, grads, dlams := evalNeuralDecisions(m, n, positions, l)
		loss := learn.HingeRankLoss(decisions, rankMargin)
		if math.IsNaN(loss) || math.IsInf(loss, 0) {
			return math.Inf(1), nil
		}
		dLdTheta, dLdlam := hingeSubgradFlat(decisions, grads, dlams, rankMargin, numTheta)
		g := make([]float64, numTheta+1)
		copy(g, dLdTheta)
		// L2 weight decay on the network weights only (never on lambda,
		// which lives in its own log-space coordinate).
		for i := 0; i < numTheta; i++ {
			loss += l2 * u[i] * u[i]
			g[i] += 2 * l2 * u[i]
		}
		g[numTheta] = l * dLdlam
		return loss, g
	}
	opts := learn.DefaultFitOptions()
	opts.Method = "" // adam
	opts.MaxIters = iters
	opts.Tolerance = 1e-9
	opts.GradTol = 1e-9
	opts.LearnRate = 0.02 // weights are signed and unnormalized, unlike log-space scalars
	opts.Verbose = verbose
	u0 := append(n.flatten(), 0)
	res, err := learn.MinimizeGradient(fg, u0, opts)
	if err != nil {
		panic(err)
	}
	n.load(res.Params[:numTheta])
	return n, math.Exp(res.Params[numTheta])
}
