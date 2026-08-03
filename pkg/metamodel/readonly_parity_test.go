package metamodel

import "testing"

// inhibNet: `go` cycles a token through `ready` while `gate` inhibits it at
// the given weight.
func inhibNet(gateTokens, gateWeight int, typ ArcType, reversed bool) *Schema {
	s := NewSchema("readonly-parity")
	s.AddTokenState("gate", gateTokens)
	s.AddTokenState("ready", 1)
	s.AddAction(Action{ID: "go"})
	s.AddArc(Arc{Source: "ready", Target: "go", Weight: 1})
	s.AddArc(Arc{Source: "go", Target: "ready", Weight: 1})
	if reversed {
		s.AddArc(Arc{Source: "go", Target: "gate", Weight: gateWeight, Type: typ})
	} else {
		s.AddArc(Arc{Source: "gate", Target: "go", Weight: gateWeight, Type: typ})
	}
	return s
}

// TestInhibitorWeightIsAThreshold: an inhibitor arc's weight is the token count
// at which it starts blocking, exactly as in go-pflow's eventsource
// (inhibitThreshold) and therefore in every generated app. Treating it as
// "blocked if the place holds ANY token" made this runtime — the one
// petri_simulate reports from — call a transition dead that the generated app
// happily fires.
func TestInhibitorWeightIsAThreshold(t *testing.T) {
	cases := []struct {
		gate, weight int
		want         bool
	}{
		{0, 1, true},
		{1, 1, false},
		{2, 3, true},  // below the threshold: eventsource fires
		{3, 3, false}, // at the threshold: blocked
		{4, 3, false},
		{1, 0, false}, // unweighted means one token, never zero
	}
	for _, tc := range cases {
		r := NewRuntime(inhibNet(tc.gate, tc.weight, InhibitorArc, false))
		if got := r.Enabled("go"); got != tc.want {
			t.Errorf("gate=%d weight=%d: Enabled = %v, want %v", tc.gate, tc.weight, got, tc.want)
		}
	}
}

// TestReversedInhibitorGatesLikeARead: pflow-xyz spells a ">= n" precondition
// as an inhibitor pointing action -> state, and both pkg/codegen/core and the
// petri.PetriNet encoding in pkg/validator read it as a read arc. This runtime
// walked only InputArcs, so the condition was invisible here: petri_simulate
// fired a transition the generated app and every verification refuse.
func TestReversedInhibitorGatesLikeARead(t *testing.T) {
	cases := []struct {
		gate, weight int
		want         bool
	}{
		{0, 1, false},
		{1, 1, true},
		{1, 2, false},
		{2, 2, true},
	}
	for _, tc := range cases {
		r := NewRuntime(inhibNet(tc.gate, tc.weight, InhibitorArc, true))
		if got := r.Enabled("go"); got != tc.want {
			t.Errorf("gate=%d weight=%d: Enabled = %v, want %v", tc.gate, tc.weight, got, tc.want)
		}
	}

	// And it still consumes and produces nothing at `gate` when it does fire.
	r := NewRuntime(inhibNet(2, 2, InhibitorArc, true))
	for i := 0; i < 4; i++ {
		if err := r.Execute("go"); err != nil {
			t.Fatalf("firing %d: %v", i, err)
		}
		if got := r.Tokens("gate"); got != 2 {
			t.Fatalf("firing %d: gate = %d, want 2", i, got)
		}
	}
}
