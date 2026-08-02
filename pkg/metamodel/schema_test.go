package metamodel

import (
	"reflect"
	"testing"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// TestMapToBindingsDeterministic guards code-generation reproducibility.
//
// Go randomizes map iteration order, so an unsorted walk over Action.Bindings
// produced a differently-ordered []Binding on every run — and that slice flows
// straight into the codegen context. With 8 keys the odds of this passing by
// luck across 50 rounds are nil.
func TestMapToBindingsDeterministic(t *testing.T) {
	in := map[string]string{
		"to": "string", "from": "string", "amount": "int64", "nonce": "int64",
		"memo": "string", "deadline": "int64", "spender": "string", "owner": "string",
	}

	want := mapToBindings(in)
	for i := 0; i < 50; i++ {
		if got := mapToBindings(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d differs:\n got %v\nwant %v", i, got, want)
		}
	}

	// Sorted by name, and every entry carries its type.
	wantOrder := []string{"amount", "deadline", "from", "memo", "nonce", "owner", "spender", "to"}
	if len(want) != len(wantOrder) {
		t.Fatalf("got %d bindings, want %d", len(want), len(wantOrder))
	}
	for i, name := range wantOrder {
		if want[i].Name != name {
			t.Errorf("bindings[%d].Name = %q, want %q", i, want[i].Name, name)
		}
		if want[i].Type != in[name] {
			t.Errorf("bindings[%q].Type = %q, want %q", name, want[i].Type, in[name])
		}
	}
}

func TestMapToBindingsEmpty(t *testing.T) {
	if got := mapToBindings(nil); got != nil {
		t.Errorf("mapToBindings(nil) = %v, want nil", got)
	}
	if got := mapToBindings(map[string]string{}); got != nil {
		t.Errorf("mapToBindings(empty) = %v, want nil", got)
	}
}

// TestToModelDeterministic covers the same property one level up, where it
// actually matters: the same Schema must convert to an identical Model.
func TestToModelDeterministic(t *testing.T) {
	s := NewSchema("token")
	s.AddState(State{ID: "balances", Kind: DataState, Type: "map[string]int64", Exported: true})
	s.AddState(State{ID: "ready", Kind: TokenState, Initial: 1})
	s.AddAction(Action{
		ID:    "transfer",
		Guard: "balances[from] >= amount",
		Bindings: map[string]string{
			"from": "string", "to": "string", "amount": "int64",
			"nonce": "int64", "memo": "string", "deadline": "int64",
		},
	})
	s.AddArc(Arc{Source: "balances", Target: "transfer", Keys: []string{"from"}, Value: "amount"})
	s.AddArc(Arc{Source: "transfer", Target: "balances", Keys: []string{"to"}, Value: "amount"})

	want := s.ToModel()
	for i := 0; i < 50; i++ {
		if got := s.ToModel(); !reflect.DeepEqual(got, want) {
			t.Fatalf("ToModel round %d differs from first conversion", i)
		}
	}
}

// TestToModelKindMapping pins the token/data split that ToModel performs.
//
// The two schema packages disagree on what an unset Kind means — go-pflow
// treats "" as token, this package treats it as data — so ToModel is the
// conversion boundary where that ambiguity has to be resolved explicitly.
// Every place it emits carries an explicit Kind.
func TestToModelKindMapping(t *testing.T) {
	s := NewSchema("mixed")
	s.AddState(State{ID: "counter", Kind: TokenState, Initial: 3})
	s.AddState(State{ID: "ledger", Kind: DataState, Type: "map[string]int64"})
	s.AddState(State{ID: "unset"}) // no Kind: this package reads that as data

	m := s.ToModel()
	want := map[string]struct {
		kind    goflowmodel.StateKind
		initial int
	}{
		"counter": {goflowmodel.TokenKind, 3},
		"ledger":  {goflowmodel.DataKind, 0},
		"unset":   {goflowmodel.DataKind, 0},
	}

	if len(m.Places) != len(want) {
		t.Fatalf("got %d places, want %d", len(m.Places), len(want))
	}
	for _, p := range m.Places {
		w, ok := want[p.ID]
		if !ok {
			t.Errorf("unexpected place %q", p.ID)
			continue
		}
		if p.Kind != w.kind {
			t.Errorf("place %q Kind = %q, want %q", p.ID, p.Kind, w.kind)
		}
		if p.Initial != w.initial {
			t.Errorf("place %q Initial = %d, want %d", p.ID, p.Initial, w.initial)
		}
	}
}
