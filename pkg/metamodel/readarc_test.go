package metamodel

import (
	"errors"
	"testing"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// readNet builds "gate tests, cycle fires": `go` consumes from `ready` and
// returns the token to `ready`, while READING `gate`. Firing it repeatedly is
// the whole point — a read arc that consumes looks correct exactly once.
func readNet(gateTokens, gateWeight int) *Schema {
	s := NewSchema("readarc")
	s.AddTokenState("gate", gateTokens)
	s.AddTokenState("ready", 1)
	s.AddAction(Action{ID: "go"})
	s.AddArc(Arc{Source: "ready", Target: "go", Weight: 1})
	s.AddArc(Arc{Source: "go", Target: "ready", Weight: 1})
	s.AddArc(Arc{Source: "gate", Target: "go", Weight: gateWeight, Type: ReadArc})
	return s
}

// TestReadArcIsNotConsumedOverManyFirings is the test this fork did not have.
//
// A single firing hides the bug: with 3 tokens in `gate` a consuming read
// still leaves 2, and the transition still fires, so the net looks right.
// Only repeated firing shows the drift — and drift is the failure mode that
// matters, because replay applies every event ever recorded.
func TestReadArcIsNotConsumedOverManyFirings(t *testing.T) {
	const firings = 5

	for _, tc := range []struct {
		name string
		exec func(r *Runtime) error
	}{
		{"Execute", func(r *Runtime) error { return r.Execute("go") }},
		{"ExecuteWithBindings", func(r *Runtime) error { return r.ExecuteWithBindings("go", Bindings{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRuntime(readNet(3, 2))

			for i := 0; i < firings; i++ {
				if !r.Enabled("go") {
					t.Fatalf("firing %d: transition disabled; gate=%d ready=%d",
						i, r.Tokens("gate"), r.Tokens("ready"))
				}
				if err := tc.exec(r); err != nil {
					t.Fatalf("firing %d: %v", i, err)
				}
				if got := r.Tokens("gate"); got != 3 {
					t.Fatalf("firing %d: read place gate = %d, want 3 (a read arc consumes nothing)", i, got)
				}
			}
			if got := r.Tokens("ready"); got != 1 {
				t.Errorf("ready = %d, want 1", got)
			}
		})
	}
}

// TestReadArcGatesEnablement: the other half of the contract. A read arc is
// not merely ignored — it is a lower bound, so too few tokens must disable.
func TestReadArcGatesEnablement(t *testing.T) {
	tests := []struct {
		gate, weight int
		want         bool
	}{
		{0, 1, false},
		{1, 1, true},
		{1, 2, false},
		{2, 2, true},
		{5, 2, true},
	}
	for _, tc := range tests {
		r := NewRuntime(readNet(tc.gate, tc.weight))
		if got := r.Enabled("go"); got != tc.want {
			t.Errorf("gate=%d weight=%d: Enabled = %v, want %v", tc.gate, tc.weight, got, tc.want)
		}
		if !tc.want {
			if err := r.Execute("go"); !errors.Is(err, ErrActionNotEnabled) {
				t.Errorf("gate=%d weight=%d: Execute err = %v, want ErrActionNotEnabled", tc.gate, tc.weight, err)
			}
			if r.Tokens("ready") != 1 {
				t.Errorf("gate=%d weight=%d: a refused firing consumed from ready", tc.gate, tc.weight)
			}
		}
	}
}

// TestArcTypePredicates pins the three-way split every "may this arc move
// tokens?" decision is made with.
func TestArcTypePredicates(t *testing.T) {
	cases := []struct {
		typ                     ArcType
		read, inhibit, readOnly bool
		known                   bool
	}{
		{NormalArc, false, false, false, true},
		{InhibitorArc, false, true, true, true},
		{ReadArc, true, false, true, true},
		{ArcType("reset"), false, false, false, false},
	}
	for _, tc := range cases {
		a := Arc{Type: tc.typ}
		if a.IsRead() != tc.read || a.IsInhibitor() != tc.inhibit || a.IsReadOnly() != tc.readOnly {
			t.Errorf("%q: read=%v inhibit=%v readOnly=%v, want %v %v %v",
				tc.typ, a.IsRead(), a.IsInhibitor(), a.IsReadOnly(), tc.read, tc.inhibit, tc.readOnly)
		}
		if IsKnownArcType(tc.typ) != tc.known {
			t.Errorf("%q: IsKnownArcType = %v, want %v", tc.typ, IsKnownArcType(tc.typ), tc.known)
		}
	}
}

// TestUnknownArcTypeIsRefused: the backwards-compatibility rule. An arc type
// from a newer writer must stop this build, not be executed as a normal
// consuming arc.
func TestUnknownArcTypeIsRefused(t *testing.T) {
	s := readNet(3, 1)
	s.Arcs[2].Type = ArcType("threshold")

	r := NewRuntime(s)
	if r.Enabled("go") {
		t.Error("Enabled = true for an action with an unknown arc type")
	}
	if err := r.Execute("go"); !errors.Is(err, ErrUnknownArcType) {
		t.Errorf("Execute err = %v, want ErrUnknownArcType", err)
	}
	if err := r.ExecuteWithBindings("go", Bindings{}); !errors.Is(err, ErrUnknownArcType) {
		t.Errorf("ExecuteWithBindings err = %v, want ErrUnknownArcType", err)
	}
	if r.Tokens("ready") != 1 || r.Tokens("gate") != 3 {
		t.Errorf("a refused firing moved tokens: gate=%d ready=%d", r.Tokens("gate"), r.Tokens("ready"))
	}

	errs := s.ValidateArcs()
	if len(errs) != 1 || !errors.Is(errs[0], ErrUnknownArcType) {
		t.Fatalf("ValidateArcs = %v, want one ErrUnknownArcType", errs)
	}
}

// TestValidateArcsRejectsReversedReadArc: a read arc tests a marking, and an
// action holds no tokens, so action -> state has no meaning.
func TestValidateArcsRejectsReversedReadArc(t *testing.T) {
	s := readNet(1, 1)
	s.Arcs[2] = Arc{Source: "go", Target: "gate", Weight: 1, Type: ReadArc}

	errs := s.ValidateArcs()
	if len(errs) != 1 || !errors.Is(errs[0], ErrReadArcDirection) {
		t.Fatalf("ValidateArcs = %v, want one ErrReadArcDirection", errs)
	}

	// However malformed, it must not mint tokens on the way out.
	r := NewRuntime(s)
	if err := r.Execute("go"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := r.Tokens("gate"); got != 1 {
		t.Errorf("gate = %d, want 1 (a read arc produces nothing)", got)
	}

	// A well-formed net produces no complaints.
	if errs := readNet(1, 1).ValidateArcs(); len(errs) != 0 {
		t.Errorf("ValidateArcs on a valid net = %v, want none", errs)
	}
}

// TestSchemaFromModelCarriesReadArc: read arcs reach this package by
// conversion from a go-pflow Model — that is how a flattened bundle's lowered
// guard link arrives — so the type must survive the round trip in both
// directions, not merely be copied.
func TestSchemaFromModelCarriesReadArc(t *testing.T) {
	m := &goflowmodel.Model{
		Name:        "flat",
		Places:      []goflowmodel.Place{{ID: "gate", Kind: goflowmodel.TokenKind, Initial: 2}},
		Transitions: []goflowmodel.Transition{{ID: "go"}},
		Arcs:        []goflowmodel.Arc{{From: "gate", To: "go", Weight: 2, Type: goflowmodel.ReadArc}},
	}

	s := SchemaFromModel(m)
	if len(s.Arcs) != 1 || !s.Arcs[0].IsRead() {
		t.Fatalf("SchemaFromModel dropped the read type: %+v", s.Arcs)
	}

	back := s.ToModel()
	if len(back.Arcs) != 1 || !back.Arcs[0].IsRead() {
		t.Fatalf("ToModel dropped the read type: %+v", back.Arcs)
	}

	// The two packages must agree on the wire value, or a model written by one
	// is mis-executed by the other.
	if string(ReadArc) != string(goflowmodel.ReadArc) {
		t.Errorf("ReadArc = %q, go-pflow = %q", ReadArc, goflowmodel.ReadArc)
	}
	if string(InhibitorArc) != string(goflowmodel.InhibitorArc) {
		t.Errorf("InhibitorArc = %q, go-pflow = %q", InhibitorArc, goflowmodel.InhibitorArc)
	}
}
