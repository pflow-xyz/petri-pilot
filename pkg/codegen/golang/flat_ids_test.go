package golang

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// flatModel builds a single-package model whose element IDs are spelled the way
// metamodel.Flatten spells them: namespaced ("orders/ship"), fused
// ("fused:a/t+b/t") and wired ("wire:b/ready"). None of "/", ":" or "+" is legal
// in a Go identifier, so this is the shape that has to survive the generator.
func flatModel() *metamodel.Model {
	return &metamodel.Model{
		Name: "warehouse",
		Places: []metamodel.Place{
			{ID: "orders/draft", Initial: 1},
			{ID: "orders/shipped"},
			{ID: "wire:b/ready", Initial: 1},
		},
		Transitions: []metamodel.Transition{
			{ID: "orders/ship"},
			{ID: "fused:a/t+b/t"},
		},
		Arcs: []metamodel.Arc{
			{From: "orders/draft", To: "orders/ship"},
			{From: "orders/ship", To: "orders/shipped"},
			{From: "wire:b/ready", To: "fused:a/t+b/t"},
			{From: "fused:a/t+b/t", To: "orders/draft"},
		},
	}
}

func generate(t *testing.T, model *metamodel.Model) []GeneratedFile {
	t.Helper()
	gen, err := New(Options{PackageName: "app", AsSubmodule: true})
	if err != nil {
		t.Fatalf("new generator: %v", err)
	}
	files, err := gen.GenerateFiles(model)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return files
}

// TestNamespacedFlatIDsGenerateValidGo: a flattened bundle model must produce
// compilable Go. Before the sanitizer, "orders/ship" reached the templates
// verbatim and the const block read `TransitionOrders/ship = "orders/ship"`.
func TestNamespacedFlatIDsGenerateValidGo(t *testing.T) {
	files := generate(t, flatModel())

	fset := token.NewFileSet()
	checked := 0
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".go") {
			continue
		}
		checked++
		if _, err := parser.ParseFile(fset, f.Name, f.Content, parser.SkipObjectResolution); err != nil {
			t.Errorf("%s does not parse: %v", f.Name, err)
		}
	}
	if checked == 0 {
		t.Fatal("no .go files generated")
	}
	if err := checkGeneratedPackage(files); err != nil {
		t.Errorf("generated package does not type-check: %v", err)
	}
}

// TestFlatIDsProduceLegalIdentifiers pins the sanitizer's output rather than
// only its compilability, so a regression names the offending stem.
func TestFlatIDsProduceLegalIdentifiers(t *testing.T) {
	cases := map[string]string{
		"orders/ship":    "OrdersShip",
		"fused:a/t+b/t":  "FusedATBT",
		"wire:b/ready":   "WireBReady",
		"orders/ship_no": "OrdersShipNo",
	}
	for id, want := range cases {
		if got := ToPascalCase(id); got != want {
			t.Errorf("ToPascalCase(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestSanitizedIDCollisionsAreDisambiguated is the trap this change exists for.
// Sanitizing is many-to-one, so distinct IDs can land on the same stem; the
// generated code then declares one identifier twice, and go/format accepts it,
// so generation reports success and `go build` fails later.
func TestSanitizedIDCollisionsAreDisambiguated(t *testing.T) {
	model := &metamodel.Model{
		Name: "collide",
		Places: []metamodel.Place{
			{ID: "orders/ready", Initial: 1},
			{ID: "orders/Ready"},
			{ID: "wire:a/x"},
			{ID: "wire_a/x"},
		},
		Transitions: []metamodel.Transition{
			{ID: "orders/ship_now"},
			{ID: "orders_ship/now"},
		},
		Arcs: []metamodel.Arc{
			{From: "orders/ready", To: "orders/ship_now"},
			{From: "orders/ship_now", To: "orders/Ready"},
			{From: "wire:a/x", To: "orders_ship/now"},
			{From: "orders_ship/now", To: "wire_a/x"},
		},
	}

	files := generate(t, model)

	// The strong claim: the package type-checks, which is what catches a
	// duplicate declaration. Parsing alone does not.
	if err := checkGeneratedPackage(files); err != nil {
		t.Fatalf("generated package does not compile: %v", err)
	}

	// And the specific claim: colliding IDs got distinct stems.
	ctx, err := NewContext(model, ContextOptions{PackageName: "collide"})
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	seen := map[string]string{}
	for _, p := range ctx.Places {
		if prev, dup := seen[p.ConstName]; dup {
			t.Errorf("places %q and %q share ConstName %q", prev, p.ID, p.ConstName)
		}
		seen[p.ConstName] = p.ID
		if prev, dup := seen[p.FieldName]; dup && prev != p.ID {
			t.Errorf("places %q and %q share FieldName %q", prev, p.ID, p.FieldName)
		}
		seen[p.FieldName] = p.ID
	}
	seen = map[string]string{}
	for _, tr := range ctx.Transitions {
		for _, name := range []string{tr.ConstName, tr.HandlerName, tr.FuncName, tr.EventName} {
			if prev, dup := seen[name]; dup && prev != tr.ID {
				t.Errorf("transitions %q and %q share identifier %q", prev, tr.ID, name)
			}
			seen[name] = tr.ID
		}
	}
}

// TestArcConstNamesMatchPlaceConstNames: an arc names the place it points at,
// so it must resolve to the same disambiguated stem the place got. Allocating
// stems per builder instead of per model would compile — and reference the
// wrong place.
func TestArcConstNamesMatchPlaceConstNames(t *testing.T) {
	model := &metamodel.Model{
		Name: "collide",
		Places: []metamodel.Place{
			{ID: "orders/ready", Initial: 1},
			{ID: "orders_ready"},
		},
		Transitions: []metamodel.Transition{{ID: "move"}},
		Arcs: []metamodel.Arc{
			{From: "orders/ready", To: "move"},
			{From: "move", To: "orders_ready"},
		},
	}

	ctx, err := NewContext(model, ContextOptions{PackageName: "collide"})
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	byID := map[string]string{}
	for _, p := range ctx.Places {
		byID[p.ID] = p.ConstName
	}
	for _, tr := range ctx.Transitions {
		for _, arc := range append(append([]ArcContext{}, tr.Inputs...), tr.Outputs...) {
			if want := byID[arc.PlaceID]; arc.ConstName != want {
				t.Errorf("arc on place %q uses const %q, place declares %q", arc.PlaceID, arc.ConstName, want)
			}
		}
	}
}

// TestIdentScopeIsDeterministic: the disambiguating suffix must depend only on
// declaration order, or two runs over one model produce different code.
func TestIdentScopeIsDeterministic(t *testing.T) {
	ids := []string{"a/b", "a_b", "a-b", "a.b"}
	first := make([]string, len(ids))
	scope := newIdentScope()
	for i, id := range ids {
		first[i] = scope.Stem(id)
	}
	scope = newIdentScope()
	for i, id := range ids {
		if got := scope.Stem(id); got != first[i] {
			t.Errorf("Stem(%q) = %q on rerun, was %q", id, got, first[i])
		}
	}
	// Idempotent within a scope: asking twice must not allocate twice.
	if got := scope.Stem(ids[0]); got != first[0] {
		t.Errorf("second Stem(%q) = %q, want %q", ids[0], got, first[0])
	}
	for i := range ids {
		for j := range ids {
			if i != j && first[i] == first[j] {
				t.Errorf("ids %q and %q share stem %q", ids[i], ids[j], first[i])
			}
		}
	}
}

// TestCheckGeneratedPackageCatchesRedeclaration is the backstop's own test:
// go/format accepts this input, so parsing cannot catch it.
func TestCheckGeneratedPackageCatchesRedeclaration(t *testing.T) {
	src := "package app\n\nconst (\n\tPlaceX = \"a\"\n\tPlaceX = \"b\"\n)\n"
	if _, err := formatGo("dup.go", []byte(src)); err != nil {
		t.Fatalf("go/format was expected to accept the duplicate: %v", err)
	}
	err := checkGeneratedPackage([]GeneratedFile{{Name: "dup.go", Content: []byte(src)}})
	if err == nil {
		t.Fatal("expected a redeclaration error")
	}
	if !strings.Contains(err.Error(), "PlaceX") {
		t.Errorf("error %q should name the redeclared identifier", err)
	}
}

// TestCheckGeneratedPackageIgnoresUnresolvableImports: imports are stubbed, so
// references through them must not be reported as errors.
func TestCheckGeneratedPackageIgnoresUnresolvableImports(t *testing.T) {
	src := "package app\n\nimport \"example.com/nope/eventstore\"\n\nfunc F() *eventstore.Store { return nil }\n"
	if err := checkGeneratedPackage([]GeneratedFile{{Name: "a.go", Content: []byte(src)}}); err != nil {
		t.Errorf("unresolvable import was reported as an error: %v", err)
	}
}

// TestCheckGeneratedPackageGroupsByDirectory: files in graph/ are a different
// package, and checking them alongside the root would report the root's own
// declarations as undefined.
func TestCheckGeneratedPackageGroupsByDirectory(t *testing.T) {
	files := []GeneratedFile{
		{Name: "a.go", Content: []byte("package app\n\nconst X = 1\n")},
		{Name: "graph/b.go", Content: []byte("package graph\n\nconst X = 2\n")},
	}
	if err := checkGeneratedPackage(files); err != nil {
		t.Errorf("two packages were checked as one: %v", err)
	}
	// Sanity: the AST-level package names really do differ.
	fset := token.NewFileSet()
	var names []string
	for _, f := range files {
		file, err := parser.ParseFile(fset, f.Name, f.Content, parser.PackageClauseOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f.Name, err)
		}
		names = append(names, file.Name.Name)
	}
	if names[0] == names[1] {
		t.Fatalf("fixture is wrong: both files declare package %q", names[0])
	}
}
