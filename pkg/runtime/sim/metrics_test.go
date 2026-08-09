package sim_test

import (
	"math"
	"strings"
	"testing"

	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// What a summary number is allowed to depend on.
//
// Metrics.Mean and P95 are time-weighted integrals over the trajectory, not
// averages of the points Series happens to report. That is the textbook
// estimator for a continuous-time Markov chain, and these tests exist because
// the alternative was in place and was wrong in a way nothing could see: the
// sample grid starts at t=0, the empty shop, so every mean carried a transient
// the operator did not ask about, weighted by however coarse the grid was.

// gridScenario is GATE 2's scenario at three baristas — the shipped café with
// abandonment switched off, so the pool sees the whole offered load and its
// utilization is the number that gate compares against M/M/c arithmetic. Only
// the reporting grid varies.
func gridScenario(samples int) sim.Scenario {
	return sim.Scenario{
		Marking: map[string]int{"staff/available": 3},
		Rates: map[string]float64{
			"counter/abandon_espresso":   0,
			"counter/abandon_latte":      0,
			"counter/abandon_cappuccino": 0,
		},
		Horizon:      8,
		Samples:      samples,
		Realizations: 24,
		Seed:         20260807,
	}
}

// sampleMean is the estimator this package used to ship: the unweighted average
// of the points Series reports, which is exactly the population the old
// Metrics.Mean averaged over.
func sampleMean(res *sim.Result) map[string]float64 {
	out := map[string]float64{}
	for _, s := range res.Series {
		var total float64
		for _, v := range s.Values {
			total += v
		}
		out[s.Place] = total / float64(len(s.Values))
	}
	return out
}

// TestMetricsDoNotMoveWithTheReportingGrid is the property the fix buys, and
// the same run is used to show what the old estimator did with it.
//
// The SSA's draws do not depend on Samples — the grid decides only where the
// trajectory is recorded — so four runs of one seed at four resolutions are one
// run. A summary of that run must therefore be the same number four times.
// Averaging the sample points was not: those points are equally spaced and
// start at t=0, the empty shop, so the first one entered the average with the
// same weight as a state the run spent real time in, and the coarser the grid
// the heavier that weight.
//
// Both halves are asserted here on purpose. Grid-independence alone is
// satisfied by any estimator that ignores the run, so the second half measures
// what the sample-point average would have said about this identical
// trajectory — Series is exactly the population it averaged — and pins both the
// direction and the size. Utilization is where the direction is not an
// accident: staff/busy starts at zero and staff/available at n, so the pool
// always reads idler than it was, at every grid, and the error shrinks as the
// grid refines. Measured on this scenario: 74.1% at 8 samples, 81.7% at the
// Options default of 60, 82.8% converged. GATE 2 reads this exact number
// against a 10% band.
func TestMetricsDoNotMoveWithTheReportingGrid(t *testing.T) {
	m := cafe.FlatModel()
	grids := []int{8, 60, 480, 4000}

	var reference *sim.Result
	var lastBiased float64
	for _, samples := range grids {
		res, err := sim.Run(m, gridScenario(samples))
		if err != nil {
			t.Fatal(err)
		}
		old := sampleMean(res)
		biased := old["staff/busy"] / (old["staff/busy"] + old["staff/available"])
		t.Logf("samples=%4d  utilization: time-weighted %.4f, sample-point average %.4f (%.1f%% low)",
			samples, res.Metrics.Utilization["staff"], biased,
			(res.Metrics.Utilization["staff"]-biased)/res.Metrics.Utilization["staff"]*100)

		if reference == nil {
			reference = res
			lastBiased = biased
			continue
		}

		// A tolerance rather than exact equality only because the grid decides
		// the horizon's last float: Horizon/(Samples-1)*(Samples-1) is not
		// bit-identical for 8 and 4000. Nothing else in the estimator can differ.
		const tol = 1e-9
		for _, p := range []string{"staff/available", "staff/busy", "counter/pending_latte", "counter/brewing_latte"} {
			a, b := res.Metrics.Mean[p], reference.Metrics.Mean[p]
			if rel := math.Abs(a-b) / math.Max(math.Abs(b), 1e-12); rel > tol {
				t.Errorf("%s means %.6f on a %d-point grid and %.6f on an %d-point one — "+
					"the metric is reporting the grid, not the run", p, a, samples, b, grids[0])
			}
			if a, b := res.Metrics.P95[p], reference.Metrics.P95[p]; a != b {
				t.Errorf("%s P95 is %.0f on a %d-point grid and %.0f on an %d-point one",
					p, a, samples, b, grids[0])
			}
		}
		if a, b := res.Metrics.Utilization["staff"], reference.Metrics.Utilization["staff"]; math.Abs(a-b) > tol {
			t.Errorf("utilization is %.6f on a %d-point grid and %.6f on an %d-point one",
				a, samples, b, grids[0])
		}

		// And the biased estimator has to move, or the assertions above are
		// measuring nothing.
		if biased <= lastBiased {
			t.Errorf("the sample-point average read %.4f at %d samples and %.4f at the previous grid; "+
				"it should climb toward the truth as the grid refines", biased, samples, lastBiased)
		}
		if biased >= res.Metrics.Utilization["staff"] {
			t.Errorf("at %d samples the sample-point average put utilization at %.4f, not below the "+
				"time-weighted %.4f — but t=0 is an empty shop and the pool cannot read busier for it",
				samples, biased, res.Metrics.Utilization["staff"])
		}
		lastBiased = biased
	}

	// The size, at the coarsest grid, so the drift above is a gap a reader
	// would have acted on rather than a rounding difference.
	coarse, err := sim.Run(m, gridScenario(grids[0]))
	if err != nil {
		t.Fatal(err)
	}
	old := sampleMean(coarse)
	biased := old["staff/busy"] / (old["staff/busy"] + old["staff/available"])
	if rel := (coarse.Metrics.Utilization["staff"] - biased) / coarse.Metrics.Utilization["staff"]; rel < 0.05 {
		t.Errorf("at %d samples the two estimators differ by only %.1f%%, so this test is not "+
			"measuring the defect it exists for", grids[0], rel*100)
	}
}

// TestAMethodAssumptionIsNotAnUnenforcedConstraint pins the two lists apart.
//
// Result.Caveats is documented as constraints the model expresses that the run
// could not enforce, and its emptiness is a claim: everything the net says was
// applied. The SSA's exponential-service note used to be appended to it on
// every scenario, so a correct statement about the engine was rendered under
// "Not enforced in this run" — and the claim became unfalsifiable, because the
// list could no longer be empty for any scenario at all.
//
// The café is a model the engine fully honours, which is what makes the empty
// caveat list here a real assertion rather than an accident of this fixture.
func TestAMethodAssumptionIsNotAnUnenforcedConstraint(t *testing.T) {
	res, err := sim.Run(cafe.FlatModel(), gridScenario(60))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Caveats) != 0 {
		t.Errorf("the café is fully honoured by the discrete engine, so nothing should be filed as "+
			"unenforced: %v", res.Caveats)
	}
	if len(res.Assumptions) == 0 {
		t.Fatal("no assumptions on an SSA scenario; the exponential-service note has to be reported somewhere")
	}
	joined := strings.Join(res.Assumptions, " ")
	if !strings.Contains(joined, "exponentially distributed") {
		t.Errorf("the assumptions do not mention the service-time distribution: %v", res.Assumptions)
	}
}
