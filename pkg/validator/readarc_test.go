package validator

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// readArcCycle: `go` consumes `ready` and gives it back, while READING `gate`.
// With one token in gate the net can fire forever and has exactly one
// reachable marking.
//
// If the read arc is analysed as a normal consuming arc, gate empties on the
// first firing: the analysis reports two states and a deadlock. That is a
// different net from the one the author wrote, and every claim made about it —
// bounded, live, deadlock-free — is a claim about the wrong thing.
func readArcCycle() *metamodel.Model {
	return &metamodel.Model{
		Name: "readcycle",
		Places: []metamodel.Place{
			{ID: "gate", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 1},
		},
		Transitions: []metamodel.Transition{{ID: "go"}},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "go", Weight: 1},
			{From: "go", To: "ready", Weight: 1},
			{From: "gate", To: "go", Weight: 1, Type: metamodel.ReadArc},
		},
	}
}

func TestReachabilityHonoursReadArcs(t *testing.T) {
	v := New(DefaultOptions())
	res, err := v.Validate(readArcCycle())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if res.Analysis == nil {
		t.Fatal("no analysis produced")
	}
	if res.Analysis.HasDeadlocks {
		t.Error("read arc analysed as consuming: the net deadlocks once gate is emptied")
	}
	if res.Analysis.StateCount != 1 {
		t.Errorf("StateCount = %d, want 1 (the marking never changes)", res.Analysis.StateCount)
	}
}

// TestReadArcStillGatesReachability: the read arc is a precondition, not a
// no-op. With gate empty, nothing can fire.
func TestReadArcStillGatesReachability(t *testing.T) {
	m := readArcCycle()
	m.Places[0].Initial = 0

	v := New(DefaultOptions())
	res, err := v.Validate(m)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if res.Analysis == nil {
		t.Fatal("no analysis produced")
	}
	if !res.Analysis.HasDeadlocks {
		t.Error("empty gate must block `go`: a read arc is a lower bound, not decoration")
	}
}
