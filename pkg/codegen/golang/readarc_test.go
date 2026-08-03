package golang

import (
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// readArcModel: `ship` consumes `placed`, produces `shipped`, and READS
// `reserved`. This is the shape go-pflow's guard-link lowering produces for a
// cross-net "> 0" precondition, so it arrives at the generator whether or not
// anyone hand-authors a read arc.
func readArcModel() *metamodel.Model {
	return &metamodel.Model{
		Name: "readarc",
		Places: []metamodel.Place{
			{ID: "placed", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "shipped", Kind: metamodel.TokenKind},
			{ID: "reserved", Kind: metamodel.TokenKind, Initial: 2},
		},
		Transitions: []metamodel.Transition{{ID: "ship"}},
		Arcs: []metamodel.Arc{
			{From: "placed", To: "ship", Weight: 1},
			{From: "ship", To: "shipped", Weight: 1},
			{From: "reserved", To: "ship", Weight: 2, Type: metamodel.ReadArc},
		},
	}
}

// TestReadArcIsNotAConsumingInput: the context must classify a read arc as
// read-only. If it lands in Inputs unflagged, the generated aggregate
// decrements the read place on every firing AND on every replay.
func TestReadArcIsNotAConsumingInput(t *testing.T) {
	ctx, err := NewContext(readArcModel(), ContextOptions{PackageName: "readarc"})
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	var ship *TransitionContext
	for i := range ctx.Transitions {
		if ctx.Transitions[i].ID == "ship" {
			ship = &ctx.Transitions[i]
		}
	}
	if ship == nil {
		t.Fatal("transition ship not found")
	}

	var reserved *ArcContext
	for i := range ship.Inputs {
		if ship.Inputs[i].PlaceID == "reserved" {
			reserved = &ship.Inputs[i]
		}
	}
	if reserved == nil {
		t.Fatal("read arc missing from Inputs: it still gates enablement")
	}
	if !reserved.IsRead || !reserved.IsReadOnly() {
		t.Errorf("reserved arc: IsRead=%v IsReadOnly=%v, want both true", reserved.IsRead, reserved.IsReadOnly())
	}
	if reserved.Weight != 2 {
		t.Errorf("reserved weight = %d, want 2", reserved.Weight)
	}
	for _, o := range ship.Outputs {
		if o.PlaceID == "reserved" {
			t.Error("read arc produced an Output: firing would mint tokens in the place it only tests")
		}
	}
}

// TestGeneratedAggregateDeclaresReads is the one that matters, because it
// inspects what actually ships: the read place must appear under Reads and
// must NOT appear under Inputs. eventsource's Apply deliberately ignores
// Reads, so an entry in the wrong map is a token stolen per replayed event.
func TestGeneratedAggregateDeclaresReads(t *testing.T) {
	files := generateFiles(t, readArcModel())

	var agg string
	for _, f := range files {
		if f.Name == "aggregate.go" {
			agg = string(f.Content)
		}
	}
	if agg == "" {
		t.Fatal("aggregate.go not generated")
	}

	block := transitionBlock(t, agg, "TransitionShip")
	inputs := mapBlock(t, block, "Inputs:")
	reads := mapBlock(t, block, "Reads:")

	if strings.Contains(inputs, "PlaceReserved") {
		t.Errorf("read place is a consuming Input:\n%s", inputs)
	}
	if !strings.Contains(inputs, "PlacePlaced") {
		t.Errorf("normal input dropped:\n%s", inputs)
	}
	if !strings.Contains(reads, "PlaceReserved: 2") {
		t.Errorf("Reads = %q, want PlaceReserved: 2", reads)
	}
}

// TestGeneratedAggregateOmitsReadsWhenUnused keeps the frozen output of every
// read-free app byte-identical: the Reads field is emitted only when there is
// something to put in it.
func TestGeneratedAggregateOmitsReadsWhenUnused(t *testing.T) {
	m := readArcModel()
	// Make the read arc an ordinary consuming input; the net stays connected,
	// so only the arc's type differs from the case above.
	m.Arcs[2].Type = metamodel.NormalArc

	files := generateFiles(t, m)
	for _, f := range files {
		if f.Name == "aggregate.go" && strings.Contains(string(f.Content), "Reads:") {
			t.Error("Reads emitted for a model with no read arcs")
		}
	}
}

// TestGeneratedAggregateInhibitorsAreThresholds: eventsource.Transition's
// Inhibitors is map[string]int (a threshold), not map[string]bool. The
// template emitted the bool form, so any model with an inhibitor generated an
// app that did not compile — invisible because no committed app has one.
func TestGeneratedAggregateInhibitorsAreThresholds(t *testing.T) {
	m := readArcModel()
	m.Arcs[2] = metamodel.Arc{From: "reserved", To: "ship", Weight: 3, Type: metamodel.InhibitorArc}

	files := generateFiles(t, m)
	var agg string
	for _, f := range files {
		if f.Name == "aggregate.go" {
			agg = string(f.Content)
		}
	}
	block := transitionBlock(t, agg, "TransitionShip")
	inhib := mapBlock(t, block, "Inhibitors:")
	if !strings.Contains(inhib, "map[string]int{") {
		t.Errorf("Inhibitors is not map[string]int:\n%s", inhib)
	}
	if !strings.Contains(inhib, "PlaceReserved: 3") {
		t.Errorf("Inhibitors = %q, want the weight carried as a threshold", inhib)
	}
	if strings.Contains(mapBlock(t, block, "Inputs:"), "PlaceReserved") {
		t.Error("inhibitor place is a consuming Input")
	}
}

// generateFiles runs the Go application generator in memory.
func generateFiles(t *testing.T, m *metamodel.Model) []GeneratedFile {
	t.Helper()
	g, err := New(Options{PackageName: "readarc", AsSubmodule: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := g.GenerateFiles(m)
	if err != nil {
		t.Fatalf("GenerateFiles: %v", err)
	}
	return files
}

// transitionBlock returns the AddTransition call whose ID is constName.
func transitionBlock(t *testing.T, src, constName string) string {
	t.Helper()
	marker := "ID:        " + constName
	i := strings.Index(src, marker)
	if i < 0 {
		t.Fatalf("transition %s not found in generated aggregate", constName)
	}
	rest := src[i:]
	if end := strings.Index(rest, "\n\t})"); end >= 0 {
		return rest[:end]
	}
	return rest
}

// mapBlock returns the text of one `Field: map[...]{...}` literal.
func mapBlock(t *testing.T, block, field string) string {
	t.Helper()
	i := strings.Index(block, field)
	if i < 0 {
		return ""
	}
	rest := block[i:]
	end := strings.Index(rest, "\n\t\t},")
	if end < 0 {
		t.Fatalf("unterminated %s literal in:\n%s", field, block)
	}
	return rest[:end]
}
