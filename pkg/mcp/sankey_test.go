package mcp

import (
	"math"
	"testing"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/solver"
)

// odeFluxThrough is the flux the solver actually produced at the model's
// initial marking, read off a short step. The transition's only output place
// has no other source, so its derivative is the flux through it.
//
// Measuring rather than restating is the point: the claim under test is that
// sankey's widths come from the curve drawn above them, and the only witness
// to what that curve integrated is the solver.
func odeFluxThrough(t *testing.T, model *goflowmetamodel.Model, out string, rates map[string]float64) (float64, map[string]float64) {
	t.Helper()
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}
	const h = 1e-4
	prob := solver.NewProblem(buildOdeNet(model), initial, [2]float64{0, h}, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	if sol == nil || len(sol.T) < 2 {
		t.Fatal("solver returned insufficient samples")
	}
	final := sol.GetFinalState()
	return (final[out] - initial[out]) / (sol.T[len(sol.T)-1] - sol.T[0]), initial
}

// TestSankeyWidthsComeFromTheCurveAboveThem is the regression test for widths
// drawn from a rate law the plotted trajectory never had.
//
// transitionRate took every arc pointing at the transition, read arcs included,
// while buildOdeNet drops a read arc — it moves no tokens and a continuous
// solve has no firing instant to test it at. So a gated net was annotated with
// k·a·gate against the k·a the solver integrated: here 2·3·5 = 30 against 6, a
// 5x band on a flow that never moved that fast.
func TestSankeyWidthsComeFromTheCurveAboveThem(t *testing.T) {
	model := &goflowmetamodel.Model{
		Name: "gated",
		Places: []goflowmetamodel.Place{
			{ID: "a", Initial: 3},
			{ID: "gate", Initial: 5},
			{ID: "out"},
		},
		Transitions: []goflowmetamodel.Transition{{ID: "t"}},
		Arcs: []goflowmetamodel.Arc{
			{From: "a", To: "t"},
			{From: "gate", To: "t", Type: goflowmetamodel.ReadArc},
			{From: "t", To: "out"},
		},
	}
	rates := map[string]float64{"t": 2}

	integrated, marking := odeFluxThrough(t, model, "out", rates)
	got := transitionRate(model, "t", rates, marking)
	if math.Abs(got-integrated) > 0.01*math.Abs(integrated) {
		t.Errorf("sankey rate %.4g against the solver's %.4g at the same marking; the widths have to be "+
			"integrated from the law the plotted trajectory came from, not from a second one", got, integrated)
	}
	if want := 6.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("sankey rate %.4g, want %g = k·a — the read place is not a term in it", got, want)
	}
}

// TestSankeyMirrorsTheOdeEvenWhereTheOdeIsWrong pins the two cases where the
// honest answer is to agree with a curve that is itself an approximation, since
// the alternative is a diagram that disagrees with the plot it annotates.
//
//   - A non-kinetic arc really does consume its tokens, so buildOdeNet emits it
//     as an ordinary arc and the mass-action solver cannot help but scale by it.
//     The SSA leaves it out; matching the SSA here would draw a flux the curve
//     never had.
//   - An inhibitor is emitted via petri.InhibitorArc and then integrated as an
//     ordinary input, because nothing in the solver reads that flag — so the
//     place that *blocks* the transition speeds it up.
//
// Both are what odeCaveats exists to declare. The fix for the second is in
// buildOdeNet; if it lands, this expectation should move with it rather than
// this function being special-cased.
func TestSankeyMirrorsTheOdeEvenWhereTheOdeIsWrong(t *testing.T) {
	no := false
	model := &goflowmetamodel.Model{
		Name: "caveated",
		Places: []goflowmetamodel.Place{
			{ID: "a", Initial: 3},
			{ID: "nk", Initial: 7},
			{ID: "block", Initial: 2},
			{ID: "out"},
		},
		Transitions: []goflowmetamodel.Transition{{ID: "t"}},
		Arcs: []goflowmetamodel.Arc{
			{From: "a", To: "t"},
			{From: "nk", To: "t", Kinetic: &no},
			{From: "block", To: "t", Type: goflowmetamodel.InhibitorArc},
			{From: "t", To: "out"},
		},
	}
	rates := map[string]float64{"t": 2}

	integrated, marking := odeFluxThrough(t, model, "out", rates)
	got := transitionRate(model, "t", rates, marking)
	if math.Abs(got-integrated) > 0.01*math.Abs(integrated) {
		t.Errorf("sankey rate %.4g against the solver's %.4g; the widths and the curve have to be the "+
			"same law even where that law is caveated", got, integrated)
	}
	if want := 84.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("sankey rate %.4g, want %g = k·a·nk·block — what the continuous engine integrates here", got, want)
	}
}
