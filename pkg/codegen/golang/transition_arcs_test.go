package golang

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Generated code embeds Inputs/Outputs positionally — the aggregate's guard
// checks and the OpenAPI request schema both iterate them in slice order — so
// the order is part of the frozen output, not an implementation detail. Nothing
// else pins it: the arcs are deduped through a map, and only a parallel
// order slice keeps the result independent of Go's map iteration. This test is
// that pin.
//
// The contract: arc declaration order, first appearance wins, duplicates to the
// same place merged into one entry at the position of the first.
func multiArcModel() *metamodel.Model {
	return &metamodel.Model{
		Name: "multiarc",
		Places: []metamodel.Place{
			// Declared in an order deliberately unlike the arc order below, so a
			// regression that sorted by place declaration (or by ID) would fail.
			{ID: "zulu", Initial: 1},
			{ID: "alpha", Initial: 1},
			{ID: "mike", Initial: 1},
			{ID: "sink_b"},
			{ID: "sink_a"},
			{ID: "blocked"},
			{ID: "data_gate", Kind: metamodel.DataKind, Type: "int64"},
		},
		Transitions: []metamodel.Transition{
			{ID: "fire"},
		},
		Arcs: []metamodel.Arc{
			{From: "mike", To: "fire", Weight: 2},
			{From: "zulu", To: "fire"},
			{From: "alpha", To: "fire", Weight: 3},
			// Second arc from an already-seen place: merges into mike's entry
			// rather than appending a new one.
			{From: "mike", To: "fire", Weight: 5},
			// Inhibitor: still an input, still ordered by first appearance.
			{From: "blocked", To: "fire", Type: metamodel.InhibitorArc, Weight: 1},
			// Data places carry values, not tokens — excluded from Inputs.
			{From: "data_gate", To: "fire"},
			{From: "fire", To: "sink_b", Weight: 4},
			{From: "fire", To: "sink_a"},
			{From: "fire", To: "sink_b", Weight: 6},
			{From: "fire", To: "data_gate"},
		},
	}
}

func TestTransitionArcSelectionAndOrder(t *testing.T) {
	ctx, err := NewContext(multiArcModel(), ContextOptions{PackageName: "multiarc"})
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	var fire *TransitionContext
	for i := range ctx.Transitions {
		if ctx.Transitions[i].ID == "fire" {
			fire = &ctx.Transitions[i]
		}
	}
	if fire == nil {
		t.Fatal("transition fire not found")
	}

	wantInputs := []ArcContext{
		{PlaceID: "mike", ConstName: "PlaceMike", Weight: 7},
		{PlaceID: "zulu", ConstName: "PlaceZulu", Weight: 1},
		{PlaceID: "alpha", ConstName: "PlaceAlpha", Weight: 3},
		{PlaceID: "blocked", ConstName: "PlaceBlocked", Weight: 1, IsInhibitor: true},
	}
	assertArcs(t, "Inputs", fire.Inputs, wantInputs)

	// sink_b keeps the position of its first arc even though its weight is the
	// sum of two; data_gate is absent because outputs to data places are not
	// token flow. Inhibitors never produce outputs.
	wantOutputs := []ArcContext{
		{PlaceID: "sink_b", ConstName: "PlaceSinkB", Weight: 10},
		{PlaceID: "sink_a", ConstName: "PlaceSinkA", Weight: 1},
	}
	assertArcs(t, "Outputs", fire.Outputs, wantOutputs)
}

// TestTransitionArcOrderIsStableAcrossBuilds guards specifically against map
// iteration leaking into the result: the same model built repeatedly must give
// the same order every time.
func TestTransitionArcOrderIsStableAcrossBuilds(t *testing.T) {
	first, err := NewContext(multiArcModel(), ContextOptions{PackageName: "multiarc"})
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := NewContext(multiArcModel(), ContextOptions{PackageName: "multiarc"})
		if err != nil {
			t.Fatalf("NewContext (run %d): %v", i, err)
		}
		for j := range first.Transitions {
			assertArcs(t, "Inputs", next.Transitions[j].Inputs, first.Transitions[j].Inputs)
			assertArcs(t, "Outputs", next.Transitions[j].Outputs, first.Transitions[j].Outputs)
		}
	}
}

func assertArcs(t *testing.T, label string, got, want []ArcContext) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d arcs %v, want %d %v", label, len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %+v, want %+v", label, i, got[i], want[i])
		}
	}
}
