// Finding 12's diagnosed fix: hybridrate.go tied ONE LinearRateFunc across
// every transition in a family, reading all 16 board places globally — so
// a transition acting on column 2 and one acting on column 0 saw identical
// input and could only respond through the same shared weights. That can
// express "the board is generally fuller" but not "my own column is close
// to opening," which is exactly the local signal finding 6's future-support
// predicate needs.
//
// This file groups by (side, column) instead of globally: one
// LinearRateFunc per family per column (8 win-groups + 8 block-groups = 16
// tied RateFuncs), each reading ONLY that column's 4 landing-token places.
// Since exactly one of those 4 is ever 1 (whichever row is the current
// landing spot), a LINEAR function over them is a full lookup table over
// the 4 possible landing depths for that column — no MLP nonlinearity
// needed to express depth-dependent (or, in principle, parity-dependent)
// behavior; the earlier design just never gave the linear model the right
// 4 numbers to look up.
package main

import (
	"fmt"
	"math"
	"sort"

	"github.com/pflow-xyz/go-pflow/learn"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// groupByColumn ties by (side, column) — the played cell's own column,
// nothing else. Two win-lines whose cell shares a column and side land in
// the same group; different columns never do.
func groupByColumn(side, _, cell string, _ int) string { return side + "_col" + string(cell[1]) }

// columnPlaces returns the 4 landing-token places for one column digit, in
// row order.
func columnPlaces(col byte) []string {
	out := make([]string, boardRows)
	for r := 0; r < boardRows; r++ {
		out[r] = fmt.Sprintf("p%d%c", r, col)
	}
	return out
}

// hybridColGrad is the per-column analog of fitgrad.go's policyGrad: N tied
// learn.LinearRateFuncs instead of N tied SharedScalars, each with its own
// (column-specific) place set. order fixes the flat parameter vector's
// layout; repTrans names one transition per group for ParamIndex lookups.
type hybridColGrad struct {
	net      *petri.PetriNet
	rfs      map[string]learn.RateFunc
	groups   map[string]*learn.LinearRateFunc
	order    []string
	repTrans map[string]string
}

func (m *model) toHybridColumnNet() *hybridColGrad {
	net, winGroups, blkGroups := m.derivePolicyNetGrouped(groupByColumn, groupByColumn)
	rfs := make(map[string]learn.RateFunc, len(net.Transitions))
	for t := range net.Transitions {
		rfs[t] = learn.NewConstantRateFunc(1)
	}
	groups := map[string]*learn.LinearRateFunc{}
	repTrans := map[string]string{}
	assign := func(prefix string, groupMap map[string]string) {
		for t, g := range groupMap {
			key := prefix + g
			rf, ok := groups[key]
			if !ok {
				col := g[len(g)-1]
				places := columnPlaces(col)
				init := make([]float64, len(places)+1)
				init[0] = 1 // mild non-zero bias, all column weights start at 0
				rf = learn.NewLinearRateFunc(places, init, false, false)
				groups[key] = rf
				repTrans[key] = t
			}
			rfs[t] = rf
		}
	}
	assign("win:", winGroups)
	assign("blk:", blkGroups)
	order := make([]string, 0, len(groups))
	for k := range groups {
		order = append(order, k)
	}
	sort.Strings(order)
	return &hybridColGrad{net: net, rfs: rfs, groups: groups, order: order, repTrans: repTrans}
}

func (hg *hybridColGrad) numParams() int {
	n := 0
	for _, k := range hg.order {
		n += hg.groups[k].NumParams()
	}
	return n
}

func (hg *hybridColGrad) setParams(theta []float64) {
	i := 0
	for _, k := range hg.order {
		rf := hg.groups[k]
		np := rf.NumParams()
		rf.SetParams(theta[i : i+np])
		i += np
	}
}

func (hg *hybridColGrad) getParams() []float64 {
	out := make([]float64, 0, hg.numParams())
	for _, k := range hg.order {
		out = append(out, hg.groups[k].GetParams()...)
	}
	return out
}

// adjointScorePlaces/adjointCoefficients turn the package's fixed score
// convention (win_x - win_o for X, x_turn + o_turn + lam*win_o for O) into
// the (Dataset, PointLossGrad) shape SolveAdjoint wants: one observation
// "point" at the horizon per place the score reads, with dLossDSim set to
// that place's coefficient in the score — the observed value itself is
// never used (loss is always returned 0; only the gradient of a LINEAR
// score wrt the final state is being extracted, not fit against real data).
var adjointScorePlaces = []string{"win_x", "win_o", "x_turn", "o_turn"}

// scoreGrad mirrors hybridScoreGrad/fitgrad.go's scoreGrad, generalized to
// N groups of potentially-differing (here uniform, 5-wide) parameter count.
// Uses SolveAdjoint (D5), not SolveWithSensitivities: adjoint cost is
// independent of parameter count (one backward solve of width n+P), where
// forward-mode's augmented state is n·(P+1) — at this file's P=80, adjoint's
// n+80 against forward's 81n is the difference that made the first
// per-column attempt take ~2h40m in a container.
func (hg *hybridColGrad) scoreGrad(m *model, mk marking, lam float64, maximizes bool) (s float64, dsdTheta []float64, dsdlam float64, ok bool) {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
	for p := range hg.net.Places {
		if _, has := state[p]; !has {
			state[p] = 0
		}
	}
	prob := learn.NewLearnableProblem(hg.net, state, [2]float64{0, odeHorizon}, hg.rfs)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 2000, Adaptive: true,
	}

	obs := make(map[string][]float64, len(adjointScorePlaces))
	for _, p := range adjointScorePlaces {
		obs[p] = []float64{0}
	}
	data, derr := learn.NewDataset([]float64{odeHorizon}, obs)
	if derr != nil {
		return 0, nil, 0, false
	}
	coef := map[string]float64{}
	if maximizes {
		coef["win_x"], coef["win_o"] = 1, -1
	} else {
		coef["x_turn"], coef["o_turn"], coef["win_o"] = 1, 1, lam
	}
	pl := func(place string, _, _ float64) (loss, dLossDSim float64) {
		return 0, coef[place]
	}

	res, err := prob.SolveAdjoint(data, pl, nil, opts)
	if err != nil || res.Truncated {
		return 0, nil, 0, false
	}
	f := res.Sol.GetFinalState()
	if maximizes {
		s = f["win_x"] - f["win_o"]
	} else {
		s = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
		dsdlam = f["win_o"]
	}
	dsdTheta = make([]float64, hg.numParams())
	offset := 0
	for _, key := range hg.order {
		np := hg.groups[key].NumParams()
		if blk, okp := res.ParamIndex[hg.repTrans[key]]; okp {
			copy(dsdTheta[offset:offset+np], res.Grad[blk[0]:blk[1]])
		}
		offset += np
	}
	return s, dsdTheta, dsdlam, true
}

func evalHybridColDecisions(m *model, hg *hybridColGrad, positions []trainPos, lam float64) (decisions []learn.RankedDecision, dThetas [][][]float64, dLams [][]float64, ok bool) {
	for _, p := range positions {
		d := learn.RankedDecision{Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves))}
		dTheta := make([][]float64, len(p.moves))
		dl := make([]float64, len(p.moves))
		for i, mv := range p.moves {
			s, gTheta, gl, sok := hg.scoreGrad(m, m.fire(mv, p.mk), lam, p.maximizes)
			if !sok {
				return nil, nil, nil, false
			}
			d.Scores[i], d.Preferred[i] = s, p.optimal[mv]
			dTheta[i] = gTheta
			dl[i] = gl
		}
		decisions = append(decisions, d)
		dThetas = append(dThetas, dTheta)
		dLams = append(dLams, dl)
	}
	return decisions, dThetas, dLams, true
}

// fitHybridColumn trains all 16 groups' LinearRateFunc weights plus lambda
// with Adam. hingeSubgradGrouped is reused unchanged from fitgrad.go — it
// operates purely on the (decisions, per-option gradient) shapes, not on
// what kind of RateFunc produced them.
func fitHybridColumn(m *model, positions []trainPos, iters int, l2 float64, verbose bool) (hg *hybridColGrad, lam float64) {
	hg = m.toHybridColumnNet()
	numTheta := hg.numParams()
	fg := func(u []float64) (float64, []float64) {
		hg.setParams(u[:numTheta])
		l := math.Exp(u[numTheta])
		decisions, dThetas, dLams, ok := evalHybridColDecisions(m, hg, positions, l)
		if !ok {
			return math.Inf(1), nil
		}
		loss := learn.HingeRankLoss(decisions, rankMargin)
		if math.IsNaN(loss) || math.IsInf(loss, 0) {
			return math.Inf(1), nil
		}
		dLdTheta, dLdlam := hingeSubgradGrouped(decisions, dThetas, dLams, rankMargin, numTheta)
		g := make([]float64, numTheta+1)
		copy(g, dLdTheta)
		// L2 weight decay on the RateFunc weights only, never on lambda
		// (its own log-space coordinate) — matches neuralode.go's pattern,
		// missing from the first hybrid attempts (finding 12/13's likely
		// under-regularization at higher parameter counts).
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
	opts.LearnRate = 0.05
	opts.Verbose = verbose
	u0 := append(hg.getParams(), 0)
	res, err := learn.MinimizeGradient(fg, u0, opts)
	if err != nil {
		panic(err)
	}
	hg.setParams(res.Params[:numTheta])
	return hg, math.Exp(res.Params[numTheta])
}
