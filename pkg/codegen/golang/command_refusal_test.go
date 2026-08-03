package golang

import (
	"strings"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// A cross-entity command is only honest if the coordinator can actually
// evaluate the rule it was given. When it cannot, the generator refuses rather
// than emitting code that decides on a fact nobody stated — every case below
// would otherwise produce a running app whose answer is silently wrong.

// twoEntityBundle is orders (pending -> ship -> shipped) plus inventory
// (restock -> available), with no links. Callers add whatever the case needs.
func twoEntityBundle() *metamodel.Bundle {
	orders := &metamodel.Model{
		Name: "orders",
		Places: []metamodel.Place{
			{ID: "pending", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "shipped", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "ship"}},
		Arcs: []metamodel.Arc{
			{From: "pending", To: "ship", Weight: 1},
			{From: "ship", To: "shipped", Weight: 1},
		},
	}
	inventory := &metamodel.Model{
		Name: "inventory",
		Places: []metamodel.Place{
			{ID: "available", Kind: metamodel.TokenKind, Initial: 2},
			{ID: "reserved", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "restock"}},
		Arcs:        []metamodel.Arc{{From: "restock", To: "available", Weight: 1}},
	}
	b := metamodel.NewBundle("shop")
	b.AddSubnet(metamodel.Subnet{ID: "orders", NetType: metamodel.WorkflowNet, Model: orders})
	b.AddSubnet(metamodel.Subnet{ID: "inventory", NetType: metamodel.ResourceNet, Model: inventory})
	return b
}

// contextErr builds the bundle context and returns the error, failing if there
// was none.
func contextErr(t *testing.T, b *metamodel.Bundle) string {
	t.Helper()
	bc, err := NewBundleContext(b, ContextOptions{})
	if err == nil {
		t.Fatalf("generation succeeded; commands = %+v", bc.Commands)
	}
	return err.Error()
}

// A cross-entity guard naming a place that does not exist reads as zero
// tokens: tokens() returns 0 for an absent key, so the comparison still
// evaluates and the guard decides on nothing. That is a wrong answer rather
// than an error, so it must not generate.
//
// Namespacing is off because flattening otherwise rewrites a subnet-local
// guard into that subnet's own namespace, which is exactly the mechanism that
// makes a cross-entity reference unwritable by hand. With flat names the guard
// reaches across as authored.
func TestUnknownPlaceReferenceIsRefused(t *testing.T) {
	b := twoEntityBundle()
	off := false
	b.Namespace = &off
	b.Subnets[0].Model.Transitions[0].Guard = `tokens("available") > 0 && tokens("nonexistent") > 0`

	msg := contextErr(t, b)
	for _, want := range []string{"unknown place", "nonexistent", "zero tokens"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not mention %q", msg, want)
		}
	}
}

// sum/count match place IDs by PREFIX. A prefix names a SET of places, and the
// coordinator assembles its marking one named place at a time — across
// entities it also knows only one aggregate id each, so it would sum one
// aggregate and report it as the entity's total.
func TestPrefixReferenceIsRefused(t *testing.T) {
	b := twoEntityBundle()
	off := false
	b.Namespace = &off
	// "avail" is no place, but it prefixes inventory's "available".
	b.Subnets[0].Model.Transitions[0].Guard = `sum("avail") > 0`

	msg := contextErr(t, b)
	for _, want := range []string{"PREFIX", "one aggregate id per entity", "available"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not mention %q", msg, want)
		}
	}
}

// The mirror image: a guard that reads only its OWN entity's places is not a
// command. Its aggregate already evaluates it, and routing it through the
// composition root would make the entity API refuse something it can decide
// alone — a refusal check that fired on everything would look just as "safe"
// as one that fires on the right things.
func TestLocalGuardStaysOnTheEntity(t *testing.T) {
	b := twoEntityBundle()
	b.Subnets[0].Model.Transitions[0].Guard = `tokens("pending") > 0`

	bc, err := NewBundleContext(b, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Commands) != 0 {
		t.Errorf("commands = %+v; a guard over the entity's own places is local", bc.Commands)
	}
}

// An arc that MOVES tokens into an entity which appends no event would change
// that entity's state with no history behind it — the one thing an
// event-sourced aggregate can never recover from. No bundle link produces one
// today, so the check is stated directly against the flattened shape it
// guards, rather than left to be discovered the first time one does.
func TestCrossEntityConsumingArcIsRefused(t *testing.T) {
	flat := &metamodel.Model{
		Name: "shop",
		Places: []metamodel.Place{
			{ID: "orders/pending", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "inventory/available", Kind: metamodel.TokenKind, Initial: 1},
		},
		Transitions: []metamodel.Transition{{ID: "orders/ship"}},
		Arcs: []metamodel.Arc{
			{From: "orders/pending", To: "orders/ship", Weight: 1},
			// Consumes from a place only inventory owns, while only orders fires.
			{From: "inventory/available", To: "orders/ship", Weight: 1},
		},
	}
	owners := map[string][][2]string{
		"orders/pending":      {{"orders", "pending"}},
		"inventory/available": {{"inventory", "available"}},
	}

	_, err := foreignConditions(flat.Transitions[0], flat, owners, map[string]bool{"orders": true})
	if err == nil {
		t.Fatal("a token-moving cross-entity arc must not generate")
	}
	for _, want := range []string{"MOVES tokens", "event link"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not mention %q", err, want)
		}
	}

	// The control: the same arc made read-only is exactly what a guard link
	// lowers to, and must be accepted.
	flat.Arcs[1].Type = metamodel.ReadArc
	got, err := foreignConditions(flat.Transitions[0], flat, owners, map[string]bool{"orders": true})
	if err != nil {
		t.Fatalf("a read-only cross-entity arc is legal: %v", err)
	}
	if len(got) != 1 || got[0].expr != `tokens("inventory/available") >= 1` {
		t.Errorf("conditions = %+v", got)
	}
}

// The happy path, stated so the refusals above cannot pass by refusing
// everything: a structural cross-entity guard generates one command whose
// condition names the foreign place.
func TestCrossEntityGuardGeneratesACommand(t *testing.T) {
	b := twoEntityBundle()
	b.AddLink(metamodel.Link{
		ID:        "ship_needs_stock",
		Kind:      metamodel.GuardLink,
		From:      metamodel.Endpoint{Subnet: "orders", Transition: "ship"},
		To:        metamodel.Endpoint{Subnet: "inventory", Place: "available"},
		Condition: ">= 2",
	})

	bc, err := NewBundleContext(b, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Commands) != 1 {
		t.Fatalf("commands = %+v, want exactly one", bc.Commands)
	}
	cmd := bc.Commands[0]
	if want := `tokens("inventory/available") >= 2`; cmd.Condition != want {
		t.Errorf("condition = %q, want %q", cmd.Condition, want)
	}
	if !bc.HasConditions() {
		t.Error("HasConditions is false, so the generated app would not import an evaluator it needs")
	}
}

// An inhibitor lowering is the mirror image and must invert, not merely
// mention the place. "== 0" lowering to ">= 1" would enable exactly the
// firings it forbids.
func TestInhibitorLoweringInvertsTheCondition(t *testing.T) {
	b := twoEntityBundle()
	b.AddLink(metamodel.Link{
		ID:        "ship_needs_nothing_reserved",
		Kind:      metamodel.GuardLink,
		From:      metamodel.Endpoint{Subnet: "orders", Transition: "ship"},
		To:        metamodel.Endpoint{Subnet: "inventory", Place: "reserved"},
		Condition: "== 0",
	})

	bc, err := NewBundleContext(b, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Commands) != 1 {
		t.Fatalf("commands = %+v, want exactly one", bc.Commands)
	}
	if want := `tokens("inventory/reserved") < 1`; bc.Commands[0].Condition != want {
		t.Errorf("condition = %q, want %q", bc.Commands[0].Condition, want)
	}
}

// A transition whose every arc stays inside its own entity is not a command:
// its entity can decide it alone, and routing it through the composition root
// would cost a hop and, worse, make the entity API refuse something it is
// perfectly able to do.
func TestLocalTransitionIsNotACommand(t *testing.T) {
	bc, err := NewBundleContext(twoEntityBundle(), ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Commands) != 0 {
		t.Errorf("commands = %+v, want none — nothing links these subnets", bc.Commands)
	}
	for _, e := range bc.Entities {
		if len(e.CrossEntity) != 0 {
			t.Errorf("entity %q gave up %+v with no links in the bundle", e.SubnetID, e.CrossEntity)
		}
	}
}

// A transition can be fused AND guarded at once — an event link joins it to
// another entity while a guard link gates it on a third's marking. Neither
// committed bundle exercises that combination, so the branch where the two
// halves of the coordinator meet (a marking assembled for entities that are
// also members, plus a fence for the one that is not) is otherwise generated
// by nobody. It is the shape most likely to emit code that does not compile.
func TestFusedAndGuardedCommandGeneratesOnce(t *testing.T) {
	b := twoEntityBundle()
	// Full generation validates each subnet, and twoEntityBundle leaves
	// inventory's "reserved" dangling; wire it locally so the failure under
	// test can only be the fused+guarded path.
	inv := b.Subnets[1].Model
	inv.Transitions = append(inv.Transitions, metamodel.Transition{ID: "reserve"})
	inv.Arcs = append(inv.Arcs,
		metamodel.Arc{From: "available", To: "reserve", Weight: 1},
		metamodel.Arc{From: "reserve", To: "reserved", Weight: 1},
	)

	// A third entity so the guard reads someone who is not a member.
	audit := &metamodel.Model{
		Name: "audit",
		Places: []metamodel.Place{
			{ID: "open", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "closed", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{{ID: "close"}},
		Arcs: []metamodel.Arc{
			{From: "open", To: "close", Weight: 1},
			{From: "close", To: "closed", Weight: 1},
		},
	}
	b.AddSubnet(metamodel.Subnet{ID: "audit", NetType: metamodel.WorkflowNet, Model: audit})
	b.AddLink(metamodel.Link{
		ID:   "ship_restocks",
		Kind: metamodel.EventLink,
		From: metamodel.Endpoint{Subnet: "orders", Transition: "ship"},
		To:   metamodel.Endpoint{Subnet: "inventory", Transition: "restock"},
	})
	b.AddLink(metamodel.Link{
		ID:        "ship_needs_open_audit",
		Kind:      metamodel.GuardLink,
		From:      metamodel.Endpoint{Subnet: "orders", Transition: "ship"},
		To:        metamodel.Endpoint{Subnet: "audit", Place: "open"},
		Condition: "> 0",
	})

	bc, err := NewBundleContext(b, ContextOptions{ModulePath: "example.com/fusedguarded"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Commands) != 1 {
		t.Fatalf("commands = %+v, want exactly one", bc.Commands)
	}
	cmd := bc.Commands[0]
	if cmd.Kind != "fused+guarded" {
		t.Errorf("kind = %q, want fused+guarded", cmd.Kind)
	}
	if want := `tokens("audit/open") >= 1`; cmd.Condition != want {
		t.Errorf("condition = %q, want %q", cmd.Condition, want)
	}
	if len(cmd.Members) != 2 {
		t.Errorf("members = %+v, want the two fused transitions", cmd.Members)
	}

	// Every participant appears once, and the guard's entity is present as a
	// read-only one — a fence, not a second append on a stream that already
	// has one.
	seen := map[string]int{}
	var fenced int
	for _, p := range cmd.Participants {
		seen[p.SubnetID]++
		if !p.IsMember {
			fenced++
		}
	}
	for subnet, n := range seen {
		if n != 1 {
			t.Errorf("participant %q appears %d times; a stream appended twice in one MultiAppend is an error", subnet, n)
		}
	}
	if fenced != 1 {
		t.Errorf("%d read-only participants, want exactly audit", fenced)
	}

	// And it must actually compile: GenerateBundleFiles type-checks each
	// generated package, so this is the fused+guarded template branch's only
	// compile gate.
	gen, err := New(Options{ModulePath: "example.com/fusedguarded", IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.GenerateBundleFiles(b); err != nil {
		t.Fatalf("fused+guarded bundle does not generate: %v", err)
	}
}
