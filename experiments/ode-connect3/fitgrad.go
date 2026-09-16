// Gradient calibration with FINER, untied groups of the same 288 structural
// transitions fit.go already tunes as 3 global scalars. This is the "achieve
// 100% via tuning" experiment: re-tie force_*/blk_* into per-row,
// per-row-parity, per-line-type or per-cell-index groups (no new Petri-net
// structure — same derivePolicyNetGrouped as the ungrouped fit) and drive
// each group's rate with go-pflow's tied-scalar sensitivity tooling, the way
// ode-minimax/fitgrad.go drives its single shared bias.
//
// go/no-go, stated in the plan and worth repeating here: the README's own
// diagnosis (finding 6) attributes the 8 residual referee errors to
// "future support" — how many drops until a threatened cell opens, and
// whose turn it is then. That's a property of a multi-ply sub-game, not a
// static per-transition rate. Finer tying can at best give the ODE a proxy
// correlated with depth/parity; it cannot represent "N drops from now"
// exactly. This is a bounded experiment, not a guaranteed fix — the
// exhaustive referee (not training loss) is the only acceptance gate, and a
// scheme that plateaus above 0 errors is still a reportable, useful result.
package main

import (
	"math"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/learn"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// rankMargin mirrors the hinge margin fit.go's rankLoss passes to
// HingeRankLoss (0.0005), kept local here as ode-minimax/fitgrad.go does.
const rankMargin = 0.0005

// policyGrad is a policy evaluation net with N independently-tied group
// scalars — one *learn.SharedScalar per distinct winGroup/blkGroup label,
// prefixed "win:"/"blk:" so the two families never share a parameter even
// when a scheme produces the same label text for both. order is the sorted
// key list that fixes the flat parameter vector's layout; repTrans names one
// transition per group, used only to look up that group's block in a
// solved Sensitivities.ParamIndex.
type policyGrad struct {
	net      *petri.PetriNet
	rfs      map[string]learn.RateFunc
	scalars  map[string]*learn.SharedScalar
	order    []string
	repTrans map[string]string
}

func (m *model) toPolicyGradGrouped(winGroup, blkGroup groupKeyFn, init float64) *policyGrad {
	net, winGroups, blkGroups := m.derivePolicyNetGrouped(winGroup, blkGroup)
	rfs := make(map[string]learn.RateFunc, len(net.Transitions))
	for t := range net.Transitions {
		rfs[t] = learn.NewConstantRateFunc(1)
	}
	scalars := map[string]*learn.SharedScalar{}
	repTrans := map[string]string{}
	assign := func(prefix string, groups map[string]string) {
		for t, g := range groups {
			key := prefix + g
			s, ok := scalars[key]
			if !ok {
				s = learn.NewSharedScalar(init)
				scalars[key] = s
				repTrans[key] = t
			}
			rfs[t] = s
		}
	}
	assign("win:", winGroups)
	assign("blk:", blkGroups)
	order := make([]string, 0, len(scalars))
	for k := range scalars {
		order = append(order, k)
	}
	sort.Strings(order)
	return &policyGrad{net: net, rfs: rfs, scalars: scalars, order: order, repTrans: repTrans}
}

func (pg *policyGrad) numParams() int { return len(pg.order) }

func (pg *policyGrad) setParams(theta []float64) {
	for i, k := range pg.order {
		pg.scalars[k].Set(theta[i])
	}
}

// scoreGrad scores one candidate position with one sensitivity solve and
// returns the seat objective s together with ds/dtheta (one entry per group,
// in pg.order) and ds/dlambda. Mirrors ode-minimax/fitgrad.go's scoreGrad,
// generalized from one shared bias to N independent group scalars.
func (pg *policyGrad) scoreGrad(m *model, mk marking, maximizes bool, lam float64) (s float64, dsdTheta []float64, dsdlam float64, ok bool) {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
	for p := range pg.net.Places {
		if _, has := state[p]; !has {
			state[p] = 0
		}
	}
	prob := learn.NewLearnableProblem(pg.net, state, [2]float64{0, odeHorizon}, pg.rfs)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 2000, Adaptive: true,
	}
	sens, err := prob.SolveWithSensitivities(nil, opts)
	if err != nil || sens.Truncated {
		return 0, nil, 0, false
	}
	K := len(sens.T) - 1
	f := sens.Sol.GetFinalState()
	paramIdx := func(key string) int {
		blk, ok := sens.ParamIndex[pg.repTrans[key]]
		if !ok {
			return -1
		}
		return blk[0]
	}
	sv := func(place string, p int) float64 {
		d, _ := sens.At(K, place, p)
		return d
	}
	dsdTheta = make([]float64, len(pg.order))
	if maximizes {
		s = f["win_x"] - f["win_o"]
		for i, key := range pg.order {
			if p := paramIdx(key); p >= 0 {
				dsdTheta[i] = sv("win_x", p) - sv("win_o", p)
			}
		}
	} else {
		s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
		for i, key := range pg.order {
			if p := paramIdx(key); p >= 0 {
				dsdTheta[i] = sv("x_turn", p) + sv("o_turn", p) + lam*sv("win_o", p)
			}
		}
		dsdlam = f["win_o"]
	}
	return s, dsdTheta, dsdlam, true
}

func evalDecisionsGrouped(m *model, pg *policyGrad, positions []trainPos, lam float64) (decisions []learn.RankedDecision, dThetas [][][]float64, dLams [][]float64, solves int, ok bool) {
	numTheta := pg.numParams()
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves)),
		}
		dTheta := make([][]float64, len(p.moves))
		dl := make([]float64, len(p.moves))
		for i, mv := range p.moves {
			s, gTheta, gl, sok := pg.scoreGrad(m, m.fire(mv, p.mk), p.maximizes, lam)
			solves++
			if !sok {
				return nil, nil, nil, solves, false
			}
			d.Scores[i], d.Preferred[i] = s, p.optimal[mv]
			if gTheta == nil {
				gTheta = make([]float64, numTheta)
			}
			dTheta[i] = gTheta
			dl[i] = gl
		}
		decisions = append(decisions, d)
		dThetas = append(dThetas, dTheta)
		dLams = append(dLams, dl)
	}
	return decisions, dThetas, dLams, solves, true
}

// hingeSubgradGrouped generalizes ode-minimax/fitgrad.go's hingeSubgrad from
// one scalar to numTheta independent groups. Same definitions: active
// decision -> ds_{j*}/dtheta - ds_{i*}/dtheta; at the kink, 0; argmax ties
// break to the smallest index, matching HingeRankLoss's strict-> scan.
func hingeSubgradGrouped(decisions []learn.RankedDecision, dThetas [][][]float64, dLams [][]float64, margin float64, numTheta int) (dLdTheta []float64, dLdlam float64) {
	dLdTheta = make([]float64, numTheta)
	for di, d := range decisions {
		iPref, iNon := -1, -1
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
			for k := 0; k < numTheta; k++ {
				dLdTheta[k] += dThetas[di][iNon][k] - dThetas[di][iPref][k]
			}
			dLdlam += dLams[di][iNon] - dLams[di][iPref]
		}
	}
	return dLdTheta, dLdlam
}

// fitPolicyGrad fits one rate per (win/blk, group) label under winGroup/
// blkGroup plus lambda, from log-space zero (rate 1, lambda 1), with
// learn.MinimizeGradient (adam). Returns the fitted rates split back into
// win/blk maps keyed by group label, lambda, and the total sensitivity-solve
// count. The exhaustive referee (main.go), not the returned loss, decides
// whether a scheme is an improvement — same discipline fit.go's Nelder-Mead
// baseline is held to, and more necessary here: an N-parameter fit has more
// freedom to game a hinge loss than a 3-parameter one did (README finding 4).
func fitPolicyGrad(m *model, positions []trainPos, winGroup, blkGroup groupKeyFn, iters int, verbose bool) (winRates, blkRates map[string]float64, lam float64, sensSolves int) {
	pg := m.toPolicyGradGrouped(winGroup, blkGroup, 1)
	numTheta := pg.numParams()
	fg := func(u []float64) (float64, []float64) {
		theta := make([]float64, numTheta)
		for i := 0; i < numTheta; i++ {
			theta[i] = math.Exp(u[i])
		}
		l := math.Exp(u[numTheta])
		pg.setParams(theta)
		decisions, dThetas, dLams, solves, ok := evalDecisionsGrouped(m, pg, positions, l)
		sensSolves += solves
		if !ok {
			return math.Inf(1), nil
		}
		loss := learn.HingeRankLoss(decisions, rankMargin)
		if math.IsNaN(loss) || math.IsInf(loss, 0) {
			return math.Inf(1), nil
		}
		dLdTheta, dLdlam := hingeSubgradGrouped(decisions, dThetas, dLams, rankMargin, numTheta)
		g := make([]float64, numTheta+1)
		for i := 0; i < numTheta; i++ {
			g[i] = theta[i] * dLdTheta[i]
		}
		g[numTheta] = l * dLdlam
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
	res, err := learn.MinimizeGradient(fg, make([]float64, numTheta+1), opts)
	if err != nil {
		panic(err)
	}
	winRates, blkRates = map[string]float64{}, map[string]float64{}
	for i, key := range pg.order {
		v := math.Exp(res.Params[i])
		if g, ok := strings.CutPrefix(key, "win:"); ok {
			winRates[g] = v
		} else {
			blkRates[strings.TrimPrefix(key, "blk:")] = v
		}
	}
	return winRates, blkRates, math.Exp(res.Params[numTheta]), sensSolves
}
