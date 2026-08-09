package sim_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"

	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// TestSupplyClassificationOnTheCafe pins the three kinds to the three shapes the
// café actually contains, because the distinction is only useful if it is the
// structural one and not a naming convention that happens to fit this bundle.
//
//   - staff/available is conserved: available + busy is a P-invariant and the
//     headcount is written in `available`.
//   - pantry/* is bounded: refilled by a source transition, but the shelf has a
//     declared capacity.
//   - counter/pending_* is a queue: no invariant, no capacity, filled only by
//     the net's own flow.
//
// counter/brewing_* is the case that makes the rule non-obvious and it is
// checked too. Those places *do* sit in a P-invariant — available +
// sum(brewing_X) is constant at the headcount — so membership alone would call
// them resources, and a run then reports 58% of the day with no espresso
// brewing above 29% with no free barista. They start empty, so they hold no
// stock of their own: whatever is in them, the net put there.
func TestSupplyClassificationOnTheCafe(t *testing.T) {
	kinds := sim.ClassifySupply(cafe.FlatModel())

	want := map[string]sim.SupplyKind{
		"staff/available":            sim.SupplyConserved,
		"pantry/milk":                sim.SupplyBounded,
		"pantry/coffee_beans":        sim.SupplyBounded,
		"pantry/cups":                sim.SupplyBounded,
		"counter/pending_espresso":   sim.SupplyQueue,
		"counter/pending_latte":      sim.SupplyQueue,
		"counter/pending_cappuccino": sim.SupplyQueue,
		"counter/espresso_ready":     sim.SupplyQueue,
		"counter/brewing_espresso":   sim.SupplyQueue,
		"staff/busy":                 sim.SupplyQueue,
	}
	for place, kind := range want {
		got, classified := kinds[place]
		if !classified {
			t.Errorf("%s was not classified at all; a place with no kind is a place a caller has to "+
				"guess about from its name", place)
			continue
		}
		if got != kind {
			t.Errorf("%s classified %q, want %q", place, got, kind)
		}
	}

	// Nothing may be left unclassified: an absent kind reaches the wire as the
	// empty string and the console's filter would let it through as though it
	// were a capacity finding.
	for i := range cafe.FlatModel().Places {
		p := &cafe.FlatModel().Places[i]
		if p.IsToken() && kinds[p.ID] == "" {
			t.Errorf("token place %s has no supply kind", p.ID)
		}
	}
}

// TestQueueNeverOutranksACapacityConstraint is the ranking half, and it is
// written so that the ordering it asserts could not be produced by sorting on
// the fraction.
//
// At two baristas the café's emptiest order queue sits around 90% of the day
// while the staff pool that decides the day's throughput sits nearer 30%, so
// ranking on the raw fraction — which is what shipped — put four "the shop was
// quiet" rows above the one finding an operator could act on. Reported under a
// field documented as what the run spent its time waiting for, that is not a
// weak answer, it is the inverse of one.
func TestQueueNeverOutranksACapacityConstraint(t *testing.T) {
	res, err := sim.Simulate(cafe.FlatModel(), map[string]int{"staff/available": 2},
		sim.Options{Horizon: 8, Samples: 480, Realizations: 8, Seed: 20260807})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contended) == 0 {
		t.Fatal("a two-barista café waited for something; nothing was reported")
	}

	top := res.Contended[0]
	if top.Place != "staff/available" || top.Kind != sim.SupplyConserved {
		t.Errorf("the top finding is %s (%s) at %.0f%%; a shop this understaffed is short of baristas",
			top.Place, top.Kind, top.Fraction*100)
	}

	seenQueue := false
	for _, c := range res.Contended {
		t.Logf("%-32s %-10s %.0f%% blocking %v", c.Place, c.Kind, c.Fraction*100, c.Blocking)
		if c.Kind == sim.SupplyQueue {
			seenQueue = true
			continue
		}
		if seenQueue {
			t.Errorf("%s is a %s constraint but ranks below a queue — a caller reading the top of this "+
				"list would be told the shop's idleness is what to fix", c.Place, c.Kind)
		}
	}

	// And the ordering has to be doing work. If no queue waited longer than the
	// top capacity finding, sorting on the fraction alone would produce the same
	// list and this test would pass against the defect it exists to catch.
	inverted := false
	for _, c := range res.Contended {
		if c.Kind == sim.SupplyQueue && c.Fraction > top.Fraction {
			inverted = true
			t.Logf("fraction ordering would have put %s (%s, %.0f%%) above %s (%.0f%%)",
				c.Place, c.Kind, c.Fraction*100, top.Place, top.Fraction*100)
		}
	}
	if !inverted {
		t.Errorf("no queue out-waited the top capacity finding at %.0f%%, so this run cannot tell the "+
			"kind-aware ranking from the fraction ranking it replaced — pick a scenario where it can",
			top.Fraction*100)
	}
}

// serviceModel loads a single-net model from services/ the way coffeeshop does
// in sim_test.go, walking up because the test's working directory is the package.
func serviceModel(t *testing.T, name string) *metamodel.Model {
	t.Helper()
	root, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		if b, err := os.ReadFile(filepath.Join(root, "services", name)); err == nil {
			var m metamodel.Model
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			return &m
		}
		root = filepath.Dir(root)
	}
	t.Skipf("services/%s not found", name)
	return nil
}

// TestAStateVariableIsNotSomethingToBuy separates the two shapes conservation
// cannot tell apart on its own.
//
// A stoplight's `red + yellow + green == 1` is exactly as much a P-invariant as
// a one-barista shop's `available + busy == 1`, and both places are marked at
// time zero — which was the whole of the conserved test. So the run ranked
// "waiting for the light to be red" as a capacity finding, above every real
// queue, and an operator is invited to act on it. There is nothing to buy: the
// light is not short of red, it is amber.
//
// What separates them is whether the tokens are spent on anything. start_espresso
// takes a barista *and* an order, so the pool serves work arriving from outside
// it; `go` takes only the red light. Nothing about either net's naming is
// consulted, which is the point — the café classification must not shift.
//
// The rule catches a state machine that cycles on its own, and that is all it
// claims. Two state machines that rendezvous are indistinguishable from a pool
// serving outside work, because they are the same structure: tcp-handshake's
// send_syn consumes client_closed and server_listen, one from each machine's
// invariant, exactly as start_espresso consumes a barista and an order. Those
// still classify conserved. Telling them apart needs to know that a headcount is
// a number someone chose and a connection state is not, which is not in the net.
// TestTwoStateMachinesInRendezvousAreNotSeparable pins that limit so it stays a
// known one.
func TestAStateVariableIsNotSomethingToBuy(t *testing.T) {
	kinds := sim.ClassifySupply(serviceModel(t, "stoplight.json"))
	got, classified := kinds["red"]
	if !classified {
		t.Fatal("stoplight: red was not classified")
	}
	if got != sim.SupplyState {
		t.Errorf("stoplight: red classified %q, want %q", got, sim.SupplyState)
	}
	if got.IsCapacity() {
		t.Error("stoplight: red ranks as a capacity finding, so a contention report offers the colour " +
			"of a traffic light as something to acquire more of")
	}

	// The café must not move: its pool serves work from outside the invariant,
	// so it stays a capacity finding. A rule that quietened the stoplight by
	// demoting every conserved place would pass the loop above and lose the one
	// answer the café exists to give.
	cafeKinds := sim.ClassifySupply(cafe.FlatModel())
	if got := cafeKinds["staff/available"]; got != sim.SupplyConserved {
		t.Errorf("staff/available classified %q, want %q — the staffing answer is gone", got, sim.SupplyConserved)
	}
}

// TestTwoStateMachinesInRendezvousAreNotSeparable records a limit of the supply
// classification rather than a behaviour anyone wants.
//
// tcp-handshake has no capacity answer in it — nothing in a three-way handshake
// is a quantity an operator buys more of — yet client_closed and server_listen
// classify conserved, so a contention report on that model offers them as
// findings to act on. That is not a rule that can be tightened without breaking
// the café: `send_syn` consuming client_closed and server_listen is the same
// shape as `start_espresso` consuming a barista and an order, and no property of
// the net distinguishes a headcount someone chose from a protocol state that
// simply is. Separating them needs intent the model does not record.
//
// If a model ever gains a way to say "this total is a lever" — a scenario that
// overrides the place would be the obvious signal — this test is where to come.
func TestTwoStateMachinesInRendezvousAreNotSeparable(t *testing.T) {
	kinds := sim.ClassifySupply(serviceModel(t, "tcp-handshake.json"))
	for _, place := range []string{"client_closed", "server_listen"} {
		if got := kinds[place]; got != sim.SupplyConserved {
			t.Errorf("%s classified %q, want %q — if this now reports %q the limit above was closed, "+
				"so delete this test and widen TestAStateVariableIsNotSomethingToBuy",
				place, got, sim.SupplyConserved, sim.SupplyState)
		}
	}
}
