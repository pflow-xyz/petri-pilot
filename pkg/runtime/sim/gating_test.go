package sim

import (
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// staffedShop is the smallest model in which "add a barista" is a question with
// an answer: orders arrive, a barista is held for the whole brew, and only then
// is a drink served.
//
// The staffing split is the point. An atomic make_drink that seized and
// released a barista in one firing would leave the pool untouched at every
// observable instant, so headcount could not possibly change the outcome.
func staffedShop(baristas int) *metamodel.Model {
	return &metamodel.Model{
		Name: "staffed",
		Places: []metamodel.Place{
			{ID: "queue"},
			{ID: "brewing"},
			{ID: "served"},
			{ID: "available", Initial: baristas},
			{ID: "busy"},
		},
		Transitions: []metamodel.Transition{
			{ID: "arrive", Rate: 12},
			{ID: "start", Rate: 60},
			{ID: "finish", Rate: 20},
		},
		Arcs: []metamodel.Arc{
			{From: "arrive", To: "queue"},
			{From: "queue", To: "start"},
			{From: "available", To: "start"},
			{From: "start", To: "brewing"},
			{From: "start", To: "busy"},
			{From: "brewing", To: "finish"},
			{From: "busy", To: "finish"},
			{From: "finish", To: "served"},
			{From: "finish", To: "available"},
		},
	}
}

// TestMarkingGuardIsEnforced is the go-pflow test of the same name run through
// this package's Simulate, so what is checked is the injection: the engine now
// lives in go-pflow/stochastic and evaluates guards only through the GuardFunc
// this wrapper supplies. If guardFunc stopped being wired in, every guard
// would be caveated instead of enforced and this test would fail on Caveats.
//
// TestMarkingGuardIsEnforced: a guard decidable from token counts alone is part
// of the firing rule, not decoration.
func TestMarkingGuardIsEnforced(t *testing.T) {
	m := &metamodel.Model{
		Name: "guarded",
		Places: []metamodel.Place{
			{ID: "orders", Initial: 10},
			{ID: "done"},
			{ID: "reserve", Initial: 3},
		},
		Transitions: []metamodel.Transition{
			{ID: "serve", Rate: 40, Guard: `tokens("reserve") >= 5`},
		},
		Arcs: []metamodel.Arc{
			{From: "orders", To: "serve"},
			{From: "serve", To: "done"},
		},
	}
	res, err := Simulate(m, map[string]int{}, Options{Horizon: 5, Samples: 20, Realizations: 2, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Metrics.Throughput["serve"]; got != 0 {
		t.Errorf("serve fired %.1f times with its guard false", got)
	}
	if len(res.Caveats) != 0 {
		t.Errorf("a marking-decidable guard should be enforced, not caveated: %v", res.Caveats)
	}
}

// TestMarkingGuardIsEnforcedUnderSchedule: the same guard through Run with a
// Schedule, which dispatches to stochastic.SimulateSchedule. It pins that the
// wrapper's guard evaluator reaches every segment of a scheduled run.
func TestMarkingGuardIsEnforcedUnderSchedule(t *testing.T) {
	m := &metamodel.Model{
		Name: "guarded",
		Places: []metamodel.Place{
			{ID: "orders", Initial: 10},
			{ID: "done"},
			{ID: "reserve", Initial: 3},
		},
		Transitions: []metamodel.Transition{
			{ID: "serve", Rate: 40, Guard: `tokens("reserve") >= 5`},
		},
		Arcs: []metamodel.Arc{
			{From: "orders", To: "serve"},
			{From: "serve", To: "done"},
		},
	}
	res, err := Run(m, Scenario{
		Horizon: 5, Samples: 20, Realizations: 2, Seed: 1,
		Schedule: map[string][]Segment{"serve": {{Until: 2, Value: 80}, {Until: 5, Value: 40}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Metrics.Throughput["serve"]; got != 0 {
		t.Errorf("serve fired %.1f times with its guard false", got)
	}
	for _, c := range res.Caveats {
		if strings.Contains(c, "not enforced") {
			t.Errorf("a marking-decidable guard should be enforced, not caveated: %q", c)
		}
	}
}
