package golang

import (
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// TestReversedInhibitorBecomesARead: pflow-xyz's spelling of a ">= n" guard is
// an inhibitor arc running transition -> place, and pkg/codegen/core lowers it
// to a Reads entry. The full-app generator matched neither of its arc branches
// — not an input (source is not a place), not an output (read-only) — so it
// DROPPED the precondition and emitted an aggregate that fires without it.
func TestReversedInhibitorBecomesARead(t *testing.T) {
	m := &metamodel.Model{
		Name: "revinh",
		Places: []metamodel.Place{
			{ID: "gate", Kind: metamodel.TokenKind, Initial: 0},
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "done", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "go"}},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "go", Weight: 1},
			{From: "go", To: "done", Weight: 1},
			{From: "go", To: "gate", Weight: 2, Type: metamodel.InhibitorArc},
		},
	}

	ctx, err := NewContext(m, ContextOptions{PackageName: "revinh"})
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	var goT *TransitionContext
	for i := range ctx.Transitions {
		if ctx.Transitions[i].ID == "go" {
			goT = &ctx.Transitions[i]
		}
	}
	if goT == nil {
		t.Fatal("transition go not found")
	}
	var gate *ArcContext
	for i := range goT.Inputs {
		if goT.Inputs[i].PlaceID == "gate" {
			gate = &goT.Inputs[i]
		}
	}
	if gate == nil {
		t.Fatal("reversed inhibitor dropped: the generated app fires with the precondition unchecked")
	}
	if !gate.IsRead || !gate.IsReadOnly() || gate.Weight != 2 {
		t.Errorf("gate arc = %+v, want a read of weight 2", *gate)
	}
	for _, o := range goT.Outputs {
		if o.PlaceID == "gate" {
			t.Error("reversed inhibitor produced an Output: firing would mint tokens")
		}
	}

	files := generateFiles(t, m)
	var agg string
	for _, f := range files {
		if f.Name == "aggregate.go" {
			agg = string(f.Content)
		}
	}
	block := transitionBlock(t, agg, "TransitionGo")
	if !strings.Contains(mapBlock(t, block, "Reads:"), "PlaceGate: 2") {
		t.Errorf("generated aggregate has no read on gate:\n%s", block)
	}
	if strings.Contains(mapBlock(t, block, "Inputs:"), "PlaceGate") {
		t.Error("reversed inhibitor became a consuming Input")
	}
}
