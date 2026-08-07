package sim

import (
	"strings"
	"testing"
)

// TestScenarioMarkingIsSparse: naming one place must not reset every other one.
// If it did, "same shop but three baristas" would silently also mean "and no
// stock and an empty queue".
func TestScenarioMarkingIsSparse(t *testing.T) {
	m := staffedShop(2)
	m.Places[0].Initial = 7 // queue

	res, err := Run(m, Scenario{
		Marking: map[string]int{"available": 5},
		Horizon: 0.001, Samples: 2, Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Series[0].Values[0]; got != 7 {
		t.Errorf("queue started at %v; naming `available` should not have disturbed it", got)
	}
}

// TestScenarioRejectsUnknownNames is the guard against the worst failure mode:
// a scenario that runs, ignores the knob it did not recognise, and reports "no
// difference" to a question it never actually asked.
func TestScenarioRejectsUnknownNames(t *testing.T) {
	m := staffedShop(2)
	for _, tc := range []struct {
		name string
		s    Scenario
		want string
	}{
		{"place", Scenario{Marking: map[string]int{"baristas": 3}}, `no token place "baristas"`},
		{"rate", Scenario{Rates: map[string]float64{"arrivals": 3}}, `no transition "arrivals"`},
		{"schedule", Scenario{Schedule: map[string][]Segment{"rush": {{Until: 1, Value: 2}}}}, `no transition "rush"`},
		{"engine", Scenario{Engine: "montecarlo"}, `unknown engine "montecarlo"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Run(m, tc.s)
			if err == nil {
				t.Fatal("accepted a name the model does not have")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestScheduleMakesARush: a rate that varies over time is the one thing a
// constant rate cannot express, and averaging it away is exactly the smoothing
// that hides whether the queue recovers afterwards.
func TestScheduleMakesARush(t *testing.T) {
	m := staffedShop(2)

	// The same customers either way — about 254 over the day — but the rush
	// puts 240 of them in the first hour, well past what two baristas can
	// serve. Spread evenly they never queue at all.
	flat, err := Run(m, Scenario{
		Rates:   map[string]float64{"arrive": 254.0 / 8},
		Horizon: 8, Samples: 80, Realizations: 12, Seed: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	rush, err := Run(m, Scenario{
		Schedule: map[string][]Segment{
			"arrive": {{Until: 1, Value: 240}, {Until: 8, Value: 2}},
		},
		Horizon: 8, Samples: 80, Realizations: 12, Seed: 4,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Peak queue, not P95: a one-hour rush is an eighth of the horizon, so a
	// percentile over the whole day averages the very thing being asked about
	// back out again. What the owner wants to know is how bad it got.
	if peak(rush, "queue") <= 5*peak(flat, "queue") {
		t.Errorf("peak queue was %.0f under a rush and %.0f spread evenly; the schedule barely mattered",
			peak(rush, "queue"), peak(flat, "queue"))
	}
	if len(rush.Times) < 2 || rush.Times[len(rush.Times)-1] < 7.5 {
		t.Errorf("the scheduled run covers %v, not the full horizon", rush.Times[len(rush.Times)-1])
	}
	// Segments are stitched, not restarted: time must never go backwards.
	for i := 1; i < len(rush.Times); i++ {
		if rush.Times[i] < rush.Times[i-1] {
			t.Fatalf("time went backwards at sample %d: %v then %v", i, rush.Times[i-1], rush.Times[i])
		}
	}
	t.Logf("flat peak queue %.0f, rush peak queue %.0f", peak(flat, "queue"), peak(rush, "queue"))
}

func peak(res *Result, place string) float64 {
	var top float64
	for _, s := range res.Series {
		if s.Place != place {
			continue
		}
		for _, v := range s.Values {
			if v > top {
				top = v
			}
		}
	}
	return top
}

// TestScheduleHoldsItsLastRate: a schedule that stops short of the horizon must
// hold, not silently revert to the model's rate — which would look like the
// rush ending a second time.
func TestScheduleHoldsItsLastRate(t *testing.T) {
	m := staffedShop(2)
	s := Scenario{Schedule: map[string][]Segment{"arrive": {{Until: 1, Value: 100}}}, Horizon: 4}

	if got := ratesAt(m, s, 0.5)["arrive"]; got != 100 {
		t.Errorf("rate at t=0.5 is %v, want 100", got)
	}
	if got := ratesAt(m, s, 3)["arrive"]; got != 100 {
		t.Errorf("rate at t=3 is %v, want the last segment's 100 — not the model's %v",
			got, Rates(m)["arrive"])
	}
}

// TestCompareSharesOneSeed is the reason Compare exists at all. Two SSA runs of
// the same shop differ; unless the dice are held fixed, a caller cannot tell
// how much of the gap between two staffing levels is the staffing.
func TestCompareSharesOneSeed(t *testing.T) {
	m := staffedShop(1)

	cmp, err := Compare(m, []Scenario{
		{Name: "today", Marking: map[string]int{"available": 1}, Horizon: 8, Samples: 40, Realizations: 8, Seed: 9},
		{Name: "one more", Marking: map[string]int{"available": 2}, Horizon: 8, Samples: 40, Realizations: 8, Seed: 999},
		{Name: "unchanged", Marking: map[string]int{"available": 1}, Horizon: 8, Samples: 40, Realizations: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cmp.Scenarios) != 3 {
		t.Fatalf("got %d results, want 3", len(cmp.Scenarios))
	}

	// The first and third differ only in the seed they asked for, so forcing
	// one seed must make them identical.
	a, c := cmp.Scenarios[0].Result, cmp.Scenarios[2].Result
	for p, v := range a.Final {
		if c.Final[p] != v {
			t.Fatalf("two identical scenarios diverged at %s (%v vs %v): the seed was not shared", p, v, c.Final[p])
		}
	}

	if cmp.Scenarios[1].Result.Metrics.Throughput["finish"] <= a.Metrics.Throughput["finish"] {
		t.Errorf("the extra barista did not raise throughput: %.1f vs %.1f",
			cmp.Scenarios[1].Result.Metrics.Throughput["finish"], a.Metrics.Throughput["finish"])
	}
}

// TestCompareNamesTheFailure: one bad scenario must say which one it was.
func TestCompareNamesTheFailure(t *testing.T) {
	_, err := Compare(staffedShop(1), []Scenario{
		{Name: "good", Horizon: 1},
		{Name: "typo", Marking: map[string]int{"baristas": 3}, Horizon: 1},
	})
	if err == nil {
		t.Fatal("accepted a comparison containing an invalid scenario")
	}
	if !strings.Contains(err.Error(), "typo") {
		t.Errorf("error = %q, want it to name the failing scenario", err)
	}
}

// TestScenarioIsPure: asking a hypothetical must not change the model it is
// asked about — the property the whole package exists to keep.
func TestScenarioIsPure(t *testing.T) {
	m := staffedShop(2)
	before := m.InitialMarking()

	if _, err := Run(m, Scenario{
		Marking:  map[string]int{"available": 9, "queue": 4},
		Schedule: map[string][]Segment{"arrive": {{Until: 1, Value: 50}, {Until: 3, Value: 1}}},
		Horizon:  3, Samples: 20, Realizations: 3, Seed: 2,
	}); err != nil {
		t.Fatal(err)
	}

	for p, n := range before {
		if got := m.InitialMarking()[p]; got != n {
			t.Errorf("the scenario changed the model: %s went %d -> %d", p, n, got)
		}
	}
}

// TestSimulateHonoursRateOverrides was a silent hole: compile() read rates
// straight from the model and never saw Options.Rates, so the discrete engine
// ignored every override. `/api/simulate?rate.X=` appeared to work, returned a
// plausible trajectory, and answered the unmodified question.
func TestSimulateHonoursRateOverrides(t *testing.T) {
	m := staffedShop(3)

	slow, err := Simulate(m, nil, Options{
		Rates: map[string]float64{"arrive": 1}, Horizon: 8, Samples: 20, Realizations: 6, Seed: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	fast, err := Simulate(m, nil, Options{
		Rates: map[string]float64{"arrive": 60}, Horizon: 8, Samples: 20, Realizations: 6, Seed: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if fast.Metrics.Throughput["arrive"] <= 10*slow.Metrics.Throughput["arrive"] {
		t.Errorf("arrivals were %.1f at rate 60 and %.1f at rate 1: the override was ignored",
			fast.Metrics.Throughput["arrive"], slow.Metrics.Throughput["arrive"])
	}
	// And the model's own rate must still be the default for everything unset.
	if slow.Metrics.Throughput["finish"] == 0 {
		t.Error("nothing was served; an override of one rate should not silence the rest")
	}
}
