package validator

import (
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

func mutexModel() *metamodel.Model {
	return &metamodel.Model{
		Places: []metamodel.Place{
			{ID: "idle1", Initial: 1}, {ID: "busy1"},
			{ID: "idle2", Initial: 1}, {ID: "busy2"},
			{ID: "sem", Initial: 1},
		},
		Transitions: []metamodel.Transition{
			{ID: "acquire1"}, {ID: "release1"},
			{ID: "acquire2"}, {ID: "release2"},
		},
		Arcs: []metamodel.Arc{
			{From: "idle1", To: "acquire1"}, {From: "sem", To: "acquire1"}, {From: "acquire1", To: "busy1"},
			{From: "busy1", To: "release1"}, {From: "release1", To: "idle1"}, {From: "release1", To: "sem"},
			{From: "idle2", To: "acquire2"}, {From: "sem", To: "acquire2"}, {From: "acquire2", To: "busy2"},
			{From: "busy2", To: "release2"}, {From: "release2", To: "idle2"}, {From: "release2", To: "sem"},
		},
	}
}

func TestInvariants(t *testing.T) {
	p, tInv := New(DefaultOptions()).Invariants(mutexModel())

	want := "busy1 + busy2 + sem == 1"
	found := false
	for _, inv := range p {
		if inv == want {
			found = true
		}
	}
	if !found {
		t.Errorf("P-invariants = %v, want to include %q", p, want)
	}

	// The mutex net cycles, so it has T-invariants.
	if len(tInv) == 0 {
		t.Error("expected T-invariants for a cyclic net")
	}
}

// TestInvariantsEmptyNotNil: callers serialise these directly, so an absence of
// invariants must be [] rather than null.
func TestInvariantsEmptyNotNil(t *testing.T) {
	model := &metamodel.Model{
		Places:      []metamodel.Place{{ID: "buffer"}},
		Transitions: []metamodel.Transition{{ID: "produce"}},
		Arcs:        []metamodel.Arc{{From: "produce", To: "buffer"}},
	}

	p, tInv := New(DefaultOptions()).Invariants(model)
	if p == nil || tInv == nil {
		t.Errorf("invariants must be empty slices, not nil: p=%v t=%v", p, tInv)
	}
	if len(p) != 0 {
		t.Errorf("unbounded producer should have no P-invariant, got %v", p)
	}
}

func TestInvariantsBadModelDoesNotPanic(t *testing.T) {
	// Arcs referencing undefined elements: must degrade, not crash.
	model := &metamodel.Model{
		Places: []metamodel.Place{{ID: "a", Initial: 1}},
		Arcs:   []metamodel.Arc{{From: "a", To: "ghost"}},
	}
	if p, tInv := New(DefaultOptions()).Invariants(model); p == nil || tInv == nil {
		t.Error("expected empty slices for a malformed model")
	}
}

// TestDeadlockStatesAreMarkings is the regression for deadlock rendering: the
// analysis used to fmt.Sprintf("%v") a *reachability.State, dumping the struct
// with its edge slices — so the output contained a pointer address and differed
// between runs on the same model.
func TestDeadlockStatesAreMarkings(t *testing.T) {
	model := &metamodel.Model{
		Places:      []metamodel.Place{{ID: "a", Initial: 1}, {ID: "b"}},
		Transitions: []metamodel.Transition{{ID: "t"}},
		Arcs:        []metamodel.Arc{{From: "a", To: "t"}, {From: "t", To: "b"}},
	}

	result, err := New(DefaultOptions()).Validate(model)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Analysis == nil || len(result.Analysis.Deadlocks) == 0 {
		t.Fatal("expected a deadlock state")
	}

	for _, dl := range result.Analysis.Deadlocks {
		if strings.Contains(dl, "0x") {
			t.Errorf("deadlock %q contains a pointer address", dl)
		}
		if strings.HasPrefix(dl, "&{") {
			t.Errorf("deadlock %q is a raw struct dump, want a marking", dl)
		}
		if !strings.Contains(dl, "b:1") {
			t.Errorf("deadlock %q should describe the marking (b:1)", dl)
		}
	}
}

// TestDeadlockStatesDeterministic: the list is built from a map, so without
// sorting the JSON differed run to run.
func TestDeadlockStatesDeterministic(t *testing.T) {
	model := &metamodel.Model{
		Places: []metamodel.Place{
			{ID: "a", Initial: 1}, {ID: "b"}, {ID: "c"}, {ID: "d"},
		},
		Transitions: []metamodel.Transition{{ID: "t1"}, {ID: "t2"}},
		Arcs: []metamodel.Arc{
			{From: "a", To: "t1"}, {From: "t1", To: "b"},
			{From: "b", To: "t2"}, {From: "t2", To: "c"},
		},
	}

	v := New(DefaultOptions())
	first, err := v.Validate(model)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	for i := 0; i < 15; i++ {
		got, err := v.Validate(model)
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if len(got.Analysis.Deadlocks) != len(first.Analysis.Deadlocks) {
			t.Fatalf("deadlock count varies")
		}
		for j := range got.Analysis.Deadlocks {
			if got.Analysis.Deadlocks[j] != first.Analysis.Deadlocks[j] {
				t.Fatalf("deadlocks not deterministic:\n got %v\nwant %v",
					got.Analysis.Deadlocks, first.Analysis.Deadlocks)
			}
		}
	}
}

// TestUnboundedDetectedViaWitness: a pump used to be reported as bounded,
// because exploration hit the state limit before the token limit.
func TestUnboundedDetectedViaWitness(t *testing.T) {
	model := &metamodel.Model{
		Places:      []metamodel.Place{{ID: "control", Initial: 1}, {ID: "overflow"}},
		Transitions: []metamodel.Transition{{ID: "spin"}},
		Arcs: []metamodel.Arc{
			{From: "control", To: "spin"}, {From: "spin", To: "control"},
			{From: "spin", To: "overflow"},
		},
	}

	result, err := New(DefaultOptions()).Validate(model)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Analysis == nil {
		t.Fatal("expected analysis")
	}
	if result.Analysis.Bounded {
		t.Error("Bounded = true for a net that pumps tokens without limit")
	}
}

func TestBoundedNetStillReportedBounded(t *testing.T) {
	result, err := New(DefaultOptions()).Validate(mutexModel())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Analysis.Bounded {
		t.Error("Bounded = false for the mutex net, which is structurally bounded")
	}
}
