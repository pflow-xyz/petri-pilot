package golang

import (
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// A guard that reads the marking needs the current token counts handed to the
// evaluator. This is the shape a composed GuardLink lowers to — go-pflow's
// metamodel.Bundle emits tokens("<place>") <cond> for any condition an inhibitor
// arc cannot express — so without it, cross-subnet gating fails at runtime with
// "unknown function: tokens" and the transition can never fire.
//
// No model under services/ uses a marking guard, so the codegen baseline cannot
// cover this branch. These tests are what exercise it.

func markingGuardModel() *metamodel.Model {
	return &metamodel.Model{
		Name: "gated",
		Places: []metamodel.Place{
			{ID: "ready", Kind: metamodel.TokenKind, Initial: 1},
			{ID: "done", Kind: metamodel.TokenKind},
			{ID: "stock", Kind: metamodel.TokenKind, Initial: 2},
		},
		Transitions: []metamodel.Transition{
			{ID: "ship", Guard: `tokens("stock") > 0`},
			{ID: "restock"},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "ship", Weight: 1},
			{From: "ship", To: "done", Weight: 1},
			{From: "restock", To: "stock", Weight: 1},
			{From: "stock", To: "ship", Weight: 1},
		},
	}
}

func generateMarkingGuardApp(t *testing.T) []GeneratedFile {
	t.Helper()
	gen, err := New(Options{PackageName: "gated", AsSubmodule: true, IncludeTests: true})
	if err != nil {
		t.Fatalf("new generator: %v", err)
	}
	files, err := gen.GenerateFiles(markingGuardModel())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return files
}

func TestMarkingGuardPassesAggregates(t *testing.T) {
	var aggregate string
	for _, f := range generateMarkingGuardApp(t) {
		if f.Name == "aggregate.go" {
			aggregate = string(f.Content)
		}
	}
	if aggregate == "" {
		t.Fatal("no aggregate.go generated")
	}

	if !strings.Contains(aggregate, "dsl.MakeAggregates(a.Places())") {
		t.Error("a marking-reading guard must be evaluated with the current token counts")
	}
	if !strings.Contains(aggregate, `dsl.Evaluate("tokens(\"stock\") > 0"`) {
		t.Errorf("the guard expression is missing from the generated evaluator:\n%s", aggregate)
	}
}

// TestNonMarkingGuardIsUnaffected pins the other branch: an ordinary parameter
// guard must not start paying for marking lookups, and — more importantly — its
// generated form must not change, which is what keeps the committed apps stable.
func TestNonMarkingGuardIsUnaffected(t *testing.T) {
	m := markingGuardModel()
	m.Transitions[0].Guard = "amount > 0"

	gen, err := New(Options{PackageName: "gated", AsSubmodule: true, IncludeTests: true})
	if err != nil {
		t.Fatalf("new generator: %v", err)
	}
	files, err := gen.GenerateFiles(m)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, f := range files {
		if f.Name != "aggregate.go" {
			continue
		}
		body := string(f.Content)
		if strings.Contains(body, "MakeAggregates") {
			t.Error("a parameter guard should not request the marking")
		}
		if !strings.Contains(body, `dsl.Evaluate("amount > 0", mb, nil)`) {
			t.Error("a parameter guard should keep the plain evaluation form")
		}
	}
}

func TestMarkingGuardOutputIsFormatted(t *testing.T) {
	for _, f := range generateMarkingGuardApp(t) {
		if !strings.HasSuffix(f.Name, ".go") {
			continue
		}
		want, err := format.Source(f.Content)
		if err != nil {
			t.Errorf("%s is not valid Go: %v", f.Name, err)
			continue
		}
		if string(want) != string(f.Content) {
			t.Errorf("%s is not gofmt-clean", f.Name)
		}
	}
}

func TestGuardUsesMarkingDetection(t *testing.T) {
	cases := map[string]bool{
		`tokens("p") > 0`:          true,
		`sum("balances") == 10`:    true,
		`count("q") > 1`:           true,
		`minOf("a") < maxOf("b")`:  true,
		`tokens ("p") > 0`:         true, // space before the paren
		"amount > 0":               false,
		"accounts(x) > 0":          false, // not count(
		"discount(x) > 0":          false, // not count(
		"tokensLeft > 0":           false, // identifier, not a call
		`balances[from] >= amount`: false,
		"":                         false,
	}
	for expr, want := range cases {
		if got := guardUsesMarking(expr); got != want {
			t.Errorf("guardUsesMarking(%q) = %v, want %v", expr, got, want)
		}
	}
}

// TestMarkingGuardAppCompiles is the end-to-end claim: the generated package
// builds, so the new template branch is not merely well-formatted but correct.
// Skipped in short mode — it shells out to `go build`, which needs a Go
// toolchain and a module cache (the same reason //pkg/mcp:mcp_test runs short
// under Bazel).
func TestMarkingGuardAppCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to go build")
	}

	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "gated")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range generateMarkingGuardApp(t) {
		p := filepath.Join(pkgDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Build inside this repo so the generated imports resolve against the
	// module already on disk, rather than needing a network fetch.
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(root))) // pkg/codegen/golang -> repo
	target := filepath.Join(repoRoot, "generated", "zz_markingguard_test_pkg")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Skipf("cannot stage generated package: %v", err)
	}
	defer os.RemoveAll(target)

	for _, f := range generateMarkingGuardApp(t) {
		p := filepath.Join(target, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("go", "build", "./generated/zz_markingguard_test_pkg/")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("generated app with a marking guard does not compile: %v\n%s", err, out)
	}
}
