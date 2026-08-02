package golang

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// shopBundle is a two-entity composition: an order workflow and an
// inventory workflow whose place_order / reserve_stock transitions fuse.
func shopBundle() *metamodel.Bundle {
	order := &metamodel.Model{
		Name: "order",
		Places: []metamodel.Place{
			{ID: "draft", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "placed", Kind: metamodel.TokenKind},
			{ID: "shipped", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{
			{ID: "place_order", Event: "order.place_order"},
			{ID: "ship", Event: "order.ship"},
		},
		Arcs: []metamodel.Arc{
			{From: "draft", To: "place_order"}, {From: "place_order", To: "placed"},
			{From: "placed", To: "ship"}, {From: "ship", To: "shipped"},
		},
	}
	inventory := &metamodel.Model{
		Name: "inventory",
		Places: []metamodel.Place{
			{ID: "available", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "reserved", Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{
			{ID: "reserve_stock", Event: "inventory.reserve_stock"},
			{ID: "release", Event: "inventory.release"},
		},
		Arcs: []metamodel.Arc{
			{From: "available", To: "reserve_stock"}, {From: "reserve_stock", To: "reserved"},
			{From: "reserved", To: "release"}, {From: "release", To: "available"},
		},
	}

	b := metamodel.NewBundle("shop")
	b.AddSubnet(metamodel.Subnet{ID: "order", NetType: metamodel.WorkflowNet, Model: order})
	b.AddSubnet(metamodel.Subnet{ID: "inventory", NetType: metamodel.WorkflowNet, Model: inventory})
	b.AddLink(metamodel.Link{
		ID:   "order_reserves_stock",
		Kind: metamodel.EventLink,
		From: metamodel.Endpoint{Subnet: "order", Transition: "place_order"},
		To:   metamodel.Endpoint{Subnet: "inventory", Transition: "reserve_stock"},
	})
	return b
}

func TestNewBundleContext(t *testing.T) {
	bc, err := NewBundleContext(shopBundle(), ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(bc.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(bc.Entities))
	}
	// Sorted by subnet ID.
	if bc.Entities[0].SubnetID != "inventory" || bc.Entities[1].SubnetID != "order" {
		t.Errorf("entity order: %s, %s", bc.Entities[0].SubnetID, bc.Entities[1].SubnetID)
	}
	for _, e := range bc.Entities {
		if e.Ctx == nil || e.Ctx.PackageName != e.PackageName {
			t.Errorf("entity %s: bad context", e.SubnetID)
		}
	}

	if len(bc.Coordinators) != 1 {
		t.Fatalf("coordinators = %+v, want 1", bc.Coordinators)
	}
	coord := bc.Coordinators[0]
	if coord.Initiator != "order" {
		t.Errorf("initiator = %q, want order (no incoming EventLink)", coord.Initiator)
	}
	if len(coord.Members) != 2 {
		t.Fatalf("members = %+v", coord.Members)
	}
	events := map[string]string{}
	for _, m := range coord.Members {
		events[m.SubnetID] = m.EventID
	}
	if events["order"] != "order.place_order" || events["inventory"] != "inventory.reserve_stock" {
		t.Errorf("member events = %v", events)
	}

	if !strings.Contains(bc.FlatModelJSON, "reserve_stock") {
		t.Error("flat model JSON missing fused transition content")
	}
}

// The identity short-circuit: one subnet, no links. Names stay unprefixed,
// there are no coordinators, and generation must still produce a working
// single-entity layout.
func TestBundleContextIdentity(t *testing.T) {
	gate := &metamodel.Model{
		Name:        "gate",
		Places:      []metamodel.Place{{ID: "open", Kind: metamodel.TokenKind, Initial: 1}, {ID: "closed", Kind: metamodel.TokenKind}},
		Transitions: []metamodel.Transition{{ID: "shut"}},
		Arcs:        []metamodel.Arc{{From: "open", To: "shut"}, {From: "shut", To: "closed"}},
	}
	b := metamodel.NewBundle("solo")
	b.AddSubnet(metamodel.Subnet{ID: "gate", Model: gate})

	bc, err := NewBundleContext(b, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Entities) != 1 || len(bc.Coordinators) != 0 {
		t.Fatalf("entities=%d coordinators=%d", len(bc.Entities), len(bc.Coordinators))
	}
	if !strings.Contains(bc.FlatModelJSON, `"shut"`) {
		t.Error("identity flatten lost the transition")
	}
}

// TestGenerateBundleFilesCompiles generates the shop bundle into a temp dir
// inside a scratch module and builds it — entity subpackages and root
// package together.
//
// Skipped under -test.short (shells out to the go toolchain).
func TestGenerateBundleFilesCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("needs go toolchain")
	}

	gen, err := New(Options{ModulePath: "example.com/shopapp"})
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.GenerateBundleFiles(shopBundle())
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, f := range files {
		names[f.Name] = true
	}
	for _, want := range []string{
		"bundle.go", "flatmodel.go",
		"order/aggregate.go", "order/service.go", "order/workflow.go",
		"inventory/aggregate.go", "inventory/events.go", "inventory/api.go",
	} {
		if !names[want] {
			t.Errorf("missing generated file %q (have %d files)", want, len(files))
		}
	}

	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A scratch module wrapping the generated tree, pointed at this repo
	// and the go-pflow sibling.
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module example.com/shopapp\n\ngo 1.25\n\n" +
		"require (\n\tgithub.com/pflow-xyz/go-pflow v0.0.0\n\tgithub.com/pflow-xyz/petri-pilot v0.0.0\n)\n\n" +
		"replace github.com/pflow-xyz/go-pflow => " + filepath.Join(repoRoot, "..", "go-pflow") + "\n" +
		"replace github.com/pflow-xyz/petri-pilot => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	build := exec.Command("go", "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("generated bundle app does not compile: %v\n%s", err, out)
	}
}
