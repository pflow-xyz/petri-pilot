package engine

import (
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/dsl"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

// A composed GuardLink lowers to a guard expression reading another subnet's
// place — tokens("inventory/available") > 0 — for any condition an inhibitor arc
// cannot express. Those functions used to be reachable only from constraint
// evaluation, so the guard resolved to nothing and the gate was a no-op: the
// composed app was less restrictive than the model said.

func gatedSchema() *metamodel.Schema {
	s := metamodel.NewSchema("gated")
	s.AddState(metamodel.State{ID: "ready", Kind: metamodel.TokenState, Initial: 1})
	s.AddState(metamodel.State{ID: "done", Kind: metamodel.TokenState})
	s.AddState(metamodel.State{ID: "available", Kind: metamodel.TokenState})
	s.AddAction(metamodel.Action{ID: "go", Guard: `tokens("available") > 0`})
	s.AddArc(metamodel.Arc{Source: "ready", Target: "go"})
	s.AddArc(metamodel.Arc{Source: "go", Target: "done"})
	return s
}

func gatedRuntime(t *testing.T, available int) *metamodel.Runtime {
	t.Helper()
	rt := metamodel.NewRuntime(gatedSchema())
	rt.GuardEvaluator = dsl.NewEvaluator()
	rt.Snapshot.Tokens["available"] = available
	return rt
}

func TestMarkingGuardBlocksWhenEmpty(t *testing.T) {
	rt := gatedRuntime(t, 0)
	err := rt.ExecuteWithBindings("go", metamodel.Bindings{})
	if err == nil {
		t.Fatal("guard tokens(\"available\") > 0 must refuse when available is 0")
	}
	if rt.Snapshot.Tokens["done"] != 0 {
		t.Error("a refused firing must not move tokens")
	}
}

func TestMarkingGuardPassesWhenStocked(t *testing.T) {
	rt := gatedRuntime(t, 1)
	if err := rt.ExecuteWithBindings("go", metamodel.Bindings{}); err != nil {
		t.Fatalf("guard should pass with available=1: %v", err)
	}
	if rt.Snapshot.Tokens["done"] != 1 {
		t.Error("the transition should have fired")
	}
}

// TestExecuteEnforcesMarkingGuard covers the bindings-free path, which
// previously skipped guards entirely — the bug that made petri_simulate report
// firings the model forbids.
func TestExecuteEnforcesMarkingGuard(t *testing.T) {
	rt := gatedRuntime(t, 0)
	if err := rt.Execute("go"); err == nil {
		t.Fatal("Execute must enforce a marking-decidable guard")
	}

	stocked := gatedRuntime(t, 1)
	if err := stocked.Execute("go"); err != nil {
		t.Fatalf("Execute should proceed when the guard holds: %v", err)
	}
}

// TestExecuteIgnoresParameterGuard pins the documented limit: Execute supplies
// no bindings, so a parameter guard is undecidable there and must not turn a
// working call into an error.
func TestExecuteIgnoresParameterGuard(t *testing.T) {
	s := metamodel.NewSchema("param")
	s.AddState(metamodel.State{ID: "ready", Kind: metamodel.TokenState, Initial: 1})
	s.AddState(metamodel.State{ID: "done", Kind: metamodel.TokenState})
	s.AddAction(metamodel.Action{ID: "go", Guard: "amount > 0"})
	s.AddArc(metamodel.Arc{Source: "ready", Target: "go"})
	s.AddArc(metamodel.Arc{Source: "go", Target: "done"})

	rt := metamodel.NewRuntime(s)
	rt.GuardEvaluator = dsl.NewEvaluator()

	if err := rt.Execute("go"); err != nil {
		t.Fatalf("a parameter guard is undecidable without bindings and must not fail Execute: %v", err)
	}
}

// TestParameterGuardStillEnforcedWithBindings: adding marking functions must not
// weaken ordinary parameter guards.
func TestParameterGuardStillEnforcedWithBindings(t *testing.T) {
	s := metamodel.NewSchema("param")
	s.AddState(metamodel.State{ID: "ready", Kind: metamodel.TokenState, Initial: 1})
	s.AddState(metamodel.State{ID: "done", Kind: metamodel.TokenState})
	s.AddAction(metamodel.Action{ID: "go", Guard: "amount > 0"})
	s.AddArc(metamodel.Arc{Source: "ready", Target: "go"})
	s.AddArc(metamodel.Arc{Source: "go", Target: "done"})

	rt := metamodel.NewRuntime(s)
	rt.GuardEvaluator = dsl.NewEvaluator()

	if err := rt.ExecuteWithBindings("go", metamodel.Bindings{"amount": 0}); err == nil {
		t.Error("amount > 0 must refuse amount=0")
	}
	if err := rt.ExecuteWithBindings("go", metamodel.Bindings{"amount": 5}); err != nil {
		t.Errorf("amount > 0 should pass amount=5: %v", err)
	}
}

// TestMixedGuardUsesBothSources: a guard may read the marking and its parameters
// in one expression.
func TestMixedGuardUsesBothSources(t *testing.T) {
	s := metamodel.NewSchema("mixed")
	s.AddState(metamodel.State{ID: "ready", Kind: metamodel.TokenState, Initial: 1})
	s.AddState(metamodel.State{ID: "done", Kind: metamodel.TokenState})
	s.AddState(metamodel.State{ID: "stock", Kind: metamodel.TokenState})
	s.AddAction(metamodel.Action{ID: "go", Guard: `tokens("stock") >= amount`})
	s.AddArc(metamodel.Arc{Source: "ready", Target: "go"})
	s.AddArc(metamodel.Arc{Source: "go", Target: "done"})

	rt := metamodel.NewRuntime(s)
	rt.GuardEvaluator = dsl.NewEvaluator()
	rt.Snapshot.Tokens["stock"] = 3

	if err := rt.ExecuteWithBindings("go", metamodel.Bindings{"amount": 5}); err == nil {
		t.Error("stock 3 should not satisfy a request for 5")
	}
	if err := rt.ExecuteWithBindings("go", metamodel.Bindings{"amount": 2}); err != nil {
		t.Errorf("stock 3 should satisfy a request for 2: %v", err)
	}
}

// TestEnabledAgreesWithExecute closes an Enabled/Execute divergence: Enabled
// checked only tokens and inhibitors, so EnabledTransitions offered a
// transition that Execute then refused with ErrGuardNotSatisfied. Anything
// driven off enablement — a UI, a planner, a reachability walk — was acting on
// a wrong answer.
func TestEnabledAgreesWithExecute(t *testing.T) {
	rt := gatedRuntime(t, 0) // guard: tokens("available") > 0, and it is empty

	if rt.Enabled("go") {
		t.Error("Enabled must be false when the guard is false")
	}
	for _, id := range rt.EnabledActions() {
		if id == "go" {
			t.Error("EnabledActions must not offer a guard-blocked transition")
		}
	}
	if err := rt.ExecuteWithBindings("go", metamodel.Bindings{}); err == nil {
		t.Error("Execute should refuse — this is the behaviour Enabled must agree with")
	}

	stocked := gatedRuntime(t, 1)
	if !stocked.Enabled("go") {
		t.Error("Enabled should be true once the guard holds")
	}
	if err := stocked.ExecuteWithBindings("go", metamodel.Bindings{}); err != nil {
		t.Errorf("Execute should proceed: %v", err)
	}
}

// TestEnabledIgnoresUndecidableParameterGuards: Enabled supplies no bindings, so
// a parameter guard must not be reported as disabled — that would hide a
// transition the caller can legitimately fire.
func TestEnabledIgnoresUndecidableParameterGuards(t *testing.T) {
	s := metamodel.NewSchema("param")
	s.AddState(metamodel.State{ID: "ready", Kind: metamodel.TokenState, Initial: 1})
	s.AddState(metamodel.State{ID: "done", Kind: metamodel.TokenState})
	s.AddAction(metamodel.Action{ID: "go", Guard: "amount > 0"})
	s.AddArc(metamodel.Arc{Source: "ready", Target: "go"})
	s.AddArc(metamodel.Arc{Source: "go", Target: "done"})

	rt := metamodel.NewRuntime(s)
	rt.GuardEvaluator = dsl.NewEvaluator()

	if !rt.Enabled("go") {
		t.Error("a parameter guard is undecidable without bindings; Enabled must not hide the transition")
	}
	if rt.EnabledWithBindings("go", metamodel.Bindings{"amount": 0}) {
		t.Error("EnabledWithBindings should decide it: amount=0 fails amount > 0")
	}
	if !rt.EnabledWithBindings("go", metamodel.Bindings{"amount": 5}) {
		t.Error("EnabledWithBindings should allow amount=5")
	}
}
