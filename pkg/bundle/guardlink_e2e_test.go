package bundle_test

import (
	"strings"
	"testing"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
)

// A GuardLink gates a transition in one subnet on a place in another. go-pflow
// lowers it two different ways, and petri-pilot has to handle both:
//
//   - STRUCTURAL, when the condition is a threshold an arc can express. "> 0"
//     becomes a read arc on the flattened place and Transition.Guard is left
//     EMPTY. This is the common case and the one that broke the generator:
//     a cross-entity command table keyed on Transition.Guard saw nothing.
//   - EXPRESSION, when it is not. "!= n" stays as tokens("<flat place>") != n
//     on Transition.Guard.
//
// Either way the precondition reads a place the gated transition's own entity
// does not own, so the entity aggregate — which replays only its own log —
// cannot decide it. The composed generator must lift the transition into a
// cross-entity command on the composition root and refuse it on the entity.

func gatedBundle(t *testing.T, condition string) *goflowmodel.Bundle {
	t.Helper()

	orders := &goflowmodel.Model{
		Name: "orders",
		Places: []goflowmodel.Place{
			{ID: "pending", Kind: goflowmodel.TokenKind, Initial: 1},
			{ID: "shipped", Kind: goflowmodel.TokenKind, Exported: true},
		},
		Transitions: []goflowmodel.Transition{{ID: "ship"}},
		Arcs: []goflowmodel.Arc{
			{From: "pending", To: "ship", Weight: 1},
			{From: "ship", To: "shipped", Weight: 1},
		},
	}
	inventory := &goflowmodel.Model{
		Name: "inventory",
		Places: []goflowmodel.Place{
			{ID: "available", Kind: goflowmodel.TokenKind, Initial: 0, Exported: true},
		},
		Transitions: []goflowmodel.Transition{{ID: "restock"}},
		Arcs:        []goflowmodel.Arc{{From: "restock", To: "available", Weight: 1}},
	}

	b := goflowmodel.NewBundle("shop")
	b.AddSubnet(goflowmodel.Subnet{ID: "orders", NetType: goflowmodel.WorkflowNet, Model: orders})
	b.AddSubnet(goflowmodel.Subnet{ID: "inventory", NetType: goflowmodel.ResourceNet, Model: inventory})
	b.AddLink(goflowmodel.Link{
		Kind:      goflowmodel.GuardLink,
		From:      goflowmodel.Endpoint{Subnet: "orders", Transition: "ship"},
		To:        goflowmodel.Endpoint{Subnet: "inventory", Place: "available"},
		Condition: condition,
	})
	return b
}

// TestGuardLinkLowersStructurally pins which of the two lowerings "> 0" takes.
// Getting this wrong is not a cosmetic mistake: it decides whether the
// generator can see the precondition at all.
func TestGuardLinkLowersStructurally(t *testing.T) {
	flat, err := gatedBundle(t, "> 0").Flatten()
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}

	tr := flat.TransitionByID("orders/ship")
	if tr == nil {
		t.Fatal("orders/ship missing from the flattened model")
	}
	if tr.Guard != "" {
		t.Errorf("guard = %q; a structural lowering leaves the expression empty", tr.Guard)
	}

	var found bool
	for i := range flat.Arcs {
		a := &flat.Arcs[i]
		if a.From == "inventory/available" && a.To == "orders/ship" {
			found = true
			if !a.IsRead() {
				t.Errorf("inventory/available -> orders/ship is %q, want a read arc", a.Type)
			}
		}
	}
	if !found {
		t.Error("the guard link lowered to neither an expression nor an arc")
	}
}

// TestStructuralGuardLinkBecomesACommand: the read arc reaching into another
// entity must turn orders/ship into a cross-entity command with inventory as a
// read participant, and must take ship away from the orders entity.
//
// This is the regression the whole change exists for. Before it, the guard
// reached the embedded flat model and stopped there: the orders aggregate
// happily fired ship with no stock, because it had never heard of inventory.
func TestStructuralGuardLinkBecomesACommand(t *testing.T) {
	bc, err := golang.NewBundleContext(gatedBundle(t, "> 0"), golang.ContextOptions{})
	if err != nil {
		t.Fatalf("bundle context: %v", err)
	}

	cmd := findCommand(t, bc, "orders/ship")
	if cmd.Kind != "guarded" {
		t.Errorf("kind = %q, want guarded", cmd.Kind)
	}
	if want := `tokens("inventory/available") >= 1`; cmd.Condition != want {
		t.Errorf("condition = %q, want %q", cmd.Condition, want)
	}
	if len(cmd.Reads) != 1 || cmd.Reads[0].SubnetID != "inventory" || cmd.Reads[0].LocalPlace != "available" {
		t.Errorf("reads = %+v, want inventory's available place", cmd.Reads)
	}
	if len(cmd.Members) != 1 || cmd.Members[0].SubnetID != "orders" {
		t.Errorf("members = %+v, want orders alone (nothing was fused)", cmd.Members)
	}

	// inventory is read but appends nothing, so it participates without being
	// a member — that is what gets it fenced.
	var readOnly bool
	for _, p := range cmd.Participants {
		if p.SubnetID == "inventory" {
			readOnly = !p.IsMember && len(p.Reads) == 1
		}
	}
	if !readOnly {
		t.Errorf("inventory should participate read-only: %+v", cmd.Participants)
	}

	assertEntityRefuses(t, bc, "orders", "ship", "orders/ship")
}

// TestExpressionGuardLinkBecomesACommand covers the other lowering. "!= 1" has
// no arc form, so it survives as an expression — and must be picked up just the
// same, from Transition.Guard rather than from the arcs.
func TestExpressionGuardLinkBecomesACommand(t *testing.T) {
	flat, err := gatedBundle(t, "!= 1").Flatten()
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	if tr := flat.TransitionByID("orders/ship"); tr == nil || tr.Guard == "" {
		t.Skip("go-pflow no longer lowers != to an expression; nothing to cover here")
	}

	bc, err := golang.NewBundleContext(gatedBundle(t, "!= 1"), golang.ContextOptions{})
	if err != nil {
		t.Fatalf("bundle context: %v", err)
	}

	cmd := findCommand(t, bc, "orders/ship")
	if !strings.Contains(cmd.Condition, `tokens("inventory/available")`) {
		t.Errorf("condition = %q, want it to carry the lowered expression", cmd.Condition)
	}
	if len(cmd.Reads) != 1 || cmd.Reads[0].FlatPlaceID != "inventory/available" {
		t.Errorf("reads = %+v; the expression's place refs are its read set", cmd.Reads)
	}
	assertEntityRefuses(t, bc, "orders", "ship", "orders/ship")
}

// TestGuardLinkGeneratesCompilingCode: the generated composition root evaluates
// the condition, and the gated entity's aggregate refuses the transition. Both
// halves are checked in the emitted source because a command that is only half
// wired compiles fine and is wrong at runtime.
func TestGuardLinkGeneratesCompilingCode(t *testing.T) {
	gen, err := golang.New(golang.Options{ModulePath: "example.com/shopapp", IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.GenerateBundleFiles(gatedBundle(t, "> 0"))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	byName := map[string]string{}
	for _, f := range files {
		byName[f.Name] = string(f.Content)
	}

	app := byName["app.go"]
	if !strings.Contains(app, "func (a *App) FireOrdersShip(") {
		t.Error("no command method for the gated transition")
	}
	if !strings.Contains(app, "dsl.MakeAggregates(marking)") {
		t.Error("the command does not evaluate its condition against an assembled marking")
	}
	if !strings.Contains(app, `marking["inventory/available"] = places[inventory.PlaceAvailable]`) {
		t.Errorf("the command does not read the place its condition names:\n%s", app)
	}

	orders := byName["orders/aggregate.go"]
	if !strings.Contains(orders, `TransitionShip: "orders/ship"`) {
		t.Errorf("the orders entity does not know ship was taken over:\n%s", orders)
	}
	if !strings.Contains(orders, "crossEntityCommands[transitionID]; ok {\n\t\treturn false") {
		t.Error("the orders entity still reports the taken-over transition as enabled")
	}
}

func findCommand(t *testing.T, bc *golang.BundleContext, flatID string) golang.CommandContext {
	t.Helper()
	for _, c := range bc.Commands {
		if c.FlatID == flatID {
			return c
		}
	}
	t.Fatalf("%q is not a cross-entity command; commands = %+v", flatID, bc.Commands)
	return golang.CommandContext{}
}

func assertEntityRefuses(t *testing.T, bc *golang.BundleContext, subnet, transition, command string) {
	t.Helper()
	for _, e := range bc.Entities {
		if e.SubnetID != subnet {
			continue
		}
		for _, x := range e.CrossEntity {
			if x.TransitionID == transition && x.Command == command {
				return
			}
		}
		t.Fatalf("entity %q does not refuse %q: %+v", subnet, transition, e.CrossEntity)
	}
	t.Fatalf("no entity %q", subnet)
}
