package sim_test

import (
	"reflect"
	"testing"

	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// TestScheduleBoundaryThatChangesNoRateIsInvisible pins that a scenario's
// schedule keeps one random stream per realization across its segments
// (go-pflow Options.ContinueStreams). A boundary at which no rate changes must
// then leave the run exactly as it was: same Final as the unscheduled run on
// the same seed. Under the engine's default, which restarts each
// realization's stream at every segment, the hour-2 boundary replays the
// first draws and Final moves.
func TestScheduleBoundaryThatChangesNoRateIsInvisible(t *testing.T) {
	m := cafe.FlatModel()
	const tr = "counter/order_latte"
	rate, ok := sim.Rates(m)[tr]
	if !ok {
		t.Fatalf("no transition %q in the café", tr)
	}
	base := sim.Scenario{Horizon: 8, Samples: 60, Realizations: 5, Seed: 42}
	plain, err := sim.Run(m, base)
	if err != nil {
		t.Fatal(err)
	}
	split := base
	split.Schedule = map[string][]sim.Segment{tr: {{Until: 2, Value: rate}, {Until: 8, Value: rate}}}
	scheduled, err := sim.Run(m, split)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain.Final, scheduled.Final) {
		t.Fatalf("a schedule boundary that changes no rate moved the run:\nunscheduled %v\nscheduled   %v",
			plain.Final, scheduled.Final)
	}
	for _, s := range scheduled.Series {
		if len(s.StdDev) == 0 {
			t.Fatalf("scheduled run at %d realizations carries no Series.StdDev for %q", base.Realizations, s.Place)
		}
	}
}

// TestSimulateScheduleBoundaryIsInvisible is the same pin on sim.Simulate,
// the entry the MCP tools and generated apps call, which may carry a schedule
// in Options or one the model declares.
func TestSimulateScheduleBoundaryIsInvisible(t *testing.T) {
	m := cafe.FlatModel()
	const tr = "counter/order_latte"
	rate := sim.Rates(m)[tr]
	opts := sim.Options{Horizon: 8, Samples: 60, Realizations: 5, Seed: 42}
	plain, err := sim.Simulate(m, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Schedule = map[string][]sim.Segment{tr: {{Until: 2, Value: rate}, {Until: 8, Value: rate}}}
	scheduled, err := sim.Simulate(m, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain.Final, scheduled.Final) {
		t.Fatalf("a schedule boundary that changes no rate moved the run:\nunscheduled %v\nscheduled   %v",
			plain.Final, scheduled.Final)
	}
}
