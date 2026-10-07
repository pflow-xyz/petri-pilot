package sim_test

import (
	"testing"

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
