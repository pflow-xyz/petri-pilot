package core

import (
	"reflect"
	"strings"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// TestReadArcSpellingsNormalizeIdentically: go-pflow lowers ">= n" guards to
// ArcType "read" (place -> transition), while pflow-xyz spells the same thing
// as a reversed inhibitor (transition -> place). Both mean "must hold w
// tokens, consumes nothing", so both must land in Reads — and produce the
// identical normalized net, hence identical generated code in every language.
//
// Before read arcs were handled here, the first spelling fell through to
// Inputs: the generated program consumed Payment on every PourCoffee.
func TestReadArcSpellingsNormalizeIdentically(t *testing.T) {
	reversedInhibitor, err := Normalize(coffeeMachine(), "coffee")
	if err != nil {
		t.Fatalf("Normalize (inhibitor spelling): %v", err)
	}

	m := coffeeMachine()
	last := len(m.Arcs) - 1
	if m.Arcs[last].Type != metamodel.InhibitorArc {
		t.Fatalf("fixture changed: last arc is %+v", m.Arcs[last])
	}
	m.Arcs[last] = metamodel.Arc{From: "Payment", To: "PourCoffee", Type: metamodel.ReadArc}

	readSpelling, err := Normalize(m, "coffee")
	if err != nil {
		t.Fatalf("Normalize (read spelling): %v", err)
	}

	if !reflect.DeepEqual(reversedInhibitor, readSpelling) {
		t.Fatalf("the two spellings normalize differently\ninhibitor: %+v\nread:      %+v",
			transitionByName(t, reversedInhibitor, "PourCoffee"),
			transitionByName(t, readSpelling, "PourCoffee"))
	}

	pour := transitionByName(t, readSpelling, "PourCoffee")
	if len(pour.Reads) != 1 || pour.Reads[0].Place != "Payment" {
		t.Fatalf("PourCoffee.Reads = %+v, want one read of Payment", pour.Reads)
	}
	for _, in := range pour.Inputs {
		if in.Place == "Payment" {
			t.Error("read place became a consuming Input: PourCoffee would spend the payment")
		}
	}
	for _, out := range pour.Outputs {
		if out.Place == "Payment" {
			t.Error("read place became an Output: PourCoffee would mint a payment")
		}
	}
}

// TestReadArcGeneratesTheSameProgram closes the loop: identical normalized
// nets must yield byte-identical source for every language and form.
func TestReadArcGeneratesTheSameProgram(t *testing.T) {
	m := coffeeMachine()
	m.Arcs[len(m.Arcs)-1] = metamodel.Arc{From: "Payment", To: "PourCoffee", Type: metamodel.ReadArc}

	for lang, spec := range Languages {
		if lang == "lean" {
			continue // proof form bakes in a model check; covered elsewhere
		}
		for _, form := range spec.Forms {
			want, err := Generate(coffeeMachine(), Options{Language: lang, Form: form, PackageName: "coffee"})
			if err != nil {
				t.Fatalf("%s/%s inhibitor spelling: %v", lang, form, err)
			}
			got, err := Generate(m, Options{Language: lang, Form: form, PackageName: "coffee"})
			if err != nil {
				t.Fatalf("%s/%s read spelling: %v", lang, form, err)
			}
			if len(want) != len(got) {
				t.Fatalf("%s/%s: file count %d != %d", lang, form, len(got), len(want))
			}
			for i := range want {
				if string(want[i].Content) != string(got[i].Content) {
					t.Errorf("%s/%s: %s differs between arc spellings", lang, form, want[i].Name)
				}
			}
		}
	}
}

// TestUnknownArcTypeRefusesGeneration: an arc type this build never heard of
// must stop generation. Every branch of the classifier ends in Inputs or
// Outputs, so guessing means emitting a program that consumes from a place the
// model only meant to test — and the program looks perfectly correct.
func TestUnknownArcTypeRefusesGeneration(t *testing.T) {
	m := coffeeMachine()
	m.Arcs[len(m.Arcs)-1] = metamodel.Arc{From: "Payment", To: "PourCoffee", Type: metamodel.ArcType("reset")}

	_, err := Normalize(m, "coffee")
	if err == nil {
		t.Fatal("Normalize accepted an unknown arc type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("error = %v, want it to name the unknown type", err)
	}
}

// TestReversedReadArcIsRefused: a read arc tests a marking and an action holds
// no tokens, so transition -> place has no reading. Emitting it as an output
// would mint tokens in the place being tested.
func TestReversedReadArcIsRefused(t *testing.T) {
	m := coffeeMachine()
	m.Arcs[len(m.Arcs)-1] = metamodel.Arc{From: "PourCoffee", To: "Payment", Type: metamodel.ReadArc}

	_, err := Normalize(m, "coffee")
	if err == nil {
		t.Fatal("Normalize accepted a reversed read arc")
	}
	if !strings.Contains(err.Error(), "place -> transition") {
		t.Errorf("error = %v, want it to name the required direction", err)
	}
}

func transitionByName(t *testing.T, m Model, name string) Transition {
	t.Helper()
	for _, tr := range m.Transitions {
		if tr.Name == name {
			return tr
		}
	}
	t.Fatalf("transition %q not found", name)
	return Transition{}
}
