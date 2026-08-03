package mcp

import (
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// petri_simulate used to run on go-pflow's tokenmodel.Runtime, which ignores arc
// weights (enablement hardcoded to "< 1"), ignores inhibitor arcs entirely,
// moves one token per arc whatever the weight, and never evaluates a guard. It
// would therefore report firing sequences the model forbids — the one thing a
// simulator must not do. These pin each of the four.

func fire(t *testing.T, m *metamodel.Model, transitions ...string) SimulationResult {
	t.Helper()
	steps := make([]SimulationStep, 0, len(transitions))
	for _, id := range transitions {
		steps = append(steps, SimulationStep{Transition: id})
	}
	return simulate(m, steps)
}

// TestSimulateHonoursArcWeights: a weight-3 input arc must not fire on 2 tokens.
func TestSimulateHonoursArcWeights(t *testing.T) {
	m := &metamodel.Model{
		Name: "weights",
		Places: []metamodel.Place{
			{ID: "pool", Kind: metamodel.TokenKind, Initial: 2},
			{ID: "done", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "take"}},
		Arcs: []metamodel.Arc{
			{From: "pool", To: "take", Weight: 3},
			{From: "take", To: "done", Weight: 1},
		},
	}

	res := fire(t, m, "take")
	if res.Steps[0].Enabled {
		t.Error("a weight-3 arc must not be enabled by 2 tokens")
	}
	if !strings.Contains(res.Steps[0].Error, "need 3") {
		t.Errorf("reason %q should say how many tokens were needed", res.Steps[0].Error)
	}

	// With enough tokens it fires, and consumes the full weight.
	m.Places[0].Initial = 3
	res = fire(t, m, "take")
	if !res.Steps[0].Enabled {
		t.Fatalf("3 tokens should satisfy a weight-3 arc: %s", res.Steps[0].Error)
	}
	if got := res.Steps[0].StateAfter["pool"]; got != 0 {
		t.Errorf("pool = %d after firing, want 0 — the full weight must be consumed", got)
	}
}

// TestSimulateHonoursInhibitorArcs: an inhibitor blocks while the place is marked.
func TestSimulateHonoursInhibitorArcs(t *testing.T) {
	m := &metamodel.Model{
		Name: "inhibit",
		Places: []metamodel.Place{
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "lock", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "done", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "go"}, {ID: "unlock"}},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "go", Weight: 1},
			{From: "lock", To: "go", Weight: 1, Type: metamodel.InhibitorArc},
			{From: "go", To: "done", Weight: 1},
			{From: "lock", To: "unlock", Weight: 1},
		},
	}

	res := fire(t, m, "go")
	if res.Steps[0].Enabled {
		t.Error("an inhibitor arc must block while its place holds tokens")
	}
	if !strings.Contains(res.Steps[0].Error, "inhibited by") {
		t.Errorf("reason %q should name the inhibiting place", res.Steps[0].Error)
	}

	// Clear the lock, then it fires.
	res = fire(t, m, "unlock", "go")
	if !res.Steps[1].Enabled {
		t.Errorf("go should fire once the lock is cleared: %s", res.Steps[1].Error)
	}
}

// TestSimulateHonoursGuards covers both guard flavours: one over the marking
// (what a composed GuardLink lowers to) and one over step bindings.
func TestSimulateHonoursGuards(t *testing.T) {
	m := &metamodel.Model{
		Name: "guards",
		Places: []metamodel.Place{
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "stock", Kind: metamodel.TokenKind, Initial: 0},
			{ID: "done", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{
			{ID: "ship", Guard: `tokens("stock") > 0`},
			{ID: "restock"},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "ship", Weight: 1},
			{From: "ship", To: "done", Weight: 1},
			{From: "restock", To: "stock", Weight: 1},
		},
	}

	res := fire(t, m, "ship")
	if res.Steps[0].Error == "" && res.Steps[0].StateAfter["done"] == 1 {
		t.Error("a marking guard must refuse while stock is empty")
	}

	res = fire(t, m, "restock", "ship")
	if res.Steps[1].StateAfter["done"] != 1 {
		t.Errorf("ship should fire once stocked: %+v", res.Steps[1])
	}
}

// TestSimulateUsesStepBindings: bindings were parsed and then discarded, so a
// parameter guard could never refuse.
func TestSimulateUsesStepBindings(t *testing.T) {
	m := &metamodel.Model{
		Name: "params",
		Places: []metamodel.Place{
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 2},
			{ID: "done", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{
			{ID: "spend", Guard: "amount > 0",
				Bindings: []metamodel.Binding{{Name: "amount", Type: "int64"}}},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "spend", Weight: 1},
			{From: "spend", To: "done", Weight: 1},
		},
	}

	refused := simulate(m, []SimulationStep{
		{Transition: "spend", Bindings: map[string]any{"amount": 0}},
	})
	if refused.Steps[0].Error == "" {
		t.Error("amount > 0 must refuse amount=0; step bindings are not reaching the guard")
	}

	allowed := simulate(m, []SimulationStep{
		{Transition: "spend", Bindings: map[string]any{"amount": 5}},
	})
	if allowed.Steps[0].Error != "" {
		t.Errorf("amount=5 should satisfy the guard: %s", allowed.Steps[0].Error)
	}
}

// TestSimulateHonoursReadArcs is the fifth of the same family, and the one
// with a cumulative failure mode: a read arc gates firing but consumes
// nothing, so simulating N firings must leave the read place untouched. A
// single-step simulation passes even when the arc is treated as consuming —
// which is exactly how the gap survived.
func TestSimulateHonoursReadArcs(t *testing.T) {
	m := &metamodel.Model{
		Name: "readarc",
		Places: []metamodel.Place{
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "gate", Kind: metamodel.TokenKind, Initial: 2},
			{ID: "done", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "go"}, {ID: "reset"}},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "go", Weight: 1},
			{From: "gate", To: "go", Weight: 2, Type: metamodel.ReadArc},
			{From: "go", To: "done", Weight: 1},
			{From: "done", To: "reset", Weight: 1},
			{From: "reset", To: "ready", Weight: 1},
		},
	}

	res := fire(t, m, "go", "reset", "go", "reset", "go")
	for i, step := range res.Steps {
		if !step.Enabled {
			t.Fatalf("step %d (%s) disabled: %s", i, step.Transition, step.Error)
		}
		if got := step.StateAfter["gate"]; got != 2 {
			t.Fatalf("step %d: gate = %d, want 2 — a read arc consumes nothing", i, got)
		}
	}

	// And it is a real precondition: one token does not satisfy a weight-2 read.
	m.Places[1].Initial = 1
	res = fire(t, m, "go")
	if res.Steps[0].Enabled {
		t.Error("a weight-2 read arc must not be satisfied by 1 token")
	}
	if !strings.Contains(res.Steps[0].Error, "read condition not met") {
		t.Errorf("reason %q should distinguish an unmet read from consumed tokens", res.Steps[0].Error)
	}
}
