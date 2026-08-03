package golang

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// simpleModel is a minimal valid model for codegen tests.
func simpleModel(t *testing.T) *metamodel.Model {
	t.Helper()
	var m metamodel.Model
	if err := json.Unmarshal([]byte(`{
		"name": "order",
		"places": [{"id": "pending"}, {"id": "shipped"}],
		"transitions": [{"id": "ship"}],
		"arcs": [
			{"from": "pending", "to": "ship"},
			{"from": "ship", "to": "shipped"}
		]
	}`), &m); err != nil {
		t.Fatalf("parse model: %v", err)
	}
	return &m
}

// genFile returns the content of the named generated file, or fails.
func genFile(t *testing.T, files []GeneratedFile, name string) string {
	t.Helper()
	for _, f := range files {
		if f.Name == name {
			return string(f.Content)
		}
	}
	t.Fatalf("expected generated file %q; got %v", name, fileNames(files))
	return ""
}

func fileNames(files []GeneratedFile) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

func hasFile(files []GeneratedFile, name string) bool {
	for _, f := range files {
		if f.Name == name {
			return true
		}
	}
	return false
}

func TestBazel_SubmoduleEmitsBuildFile(t *testing.T) {
	gen, err := New(Options{
		ModulePath:   "github.com/pflow-xyz/petri-pilot/examples/order",
		PackageName:  "order",
		AsSubmodule:  true,
		IncludeBazel: true,
		IncludeTests: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.GenerateFiles(simpleModel(t))
	if err != nil {
		t.Fatal(err)
	}

	build := genFile(t, files, "BUILD.bazel")

	// Submodule mode: a go_library named after the package, the correct
	// importpath, and a glob-based srcs marked `# keep` so gazelle leaves it be.
	for _, want := range []string{
		`go_library(`,
		`name = "order"`,
		`importpath = "github.com/pflow-xyz/petri-pilot/examples/order"`,
		`srcs = glob(["*.go"], exclude = ["*_test.go"]),  # keep`,
		`"//pkg/runtime/api"`,
		`"@com_github_pflow_xyz_go_pflow//eventsource"`,
		`go_test(`,
	} {
		if !strings.Contains(build, want) {
			t.Errorf("BUILD.bazel missing %q\n---\n%s", want, build)
		}
	}

	// Submodule mode must NOT emit module-level Bazel files (the parent owns them).
	for _, unwanted := range []string{"MODULE.bazel", ".bazelrc", ".bazelversion"} {
		if hasFile(files, unwanted) {
			t.Errorf("submodule mode should not emit %s", unwanted)
		}
	}
	// And no gazelle/nogo wiring in a submodule BUILD.bazel.
	if strings.Contains(build, "gazelle(") || strings.Contains(build, "nogo(") {
		t.Errorf("submodule BUILD.bazel should not declare gazelle/nogo:\n%s", build)
	}
}

func TestBazel_StandaloneEmitsModuleFiles(t *testing.T) {
	gen, err := New(Options{
		ModulePath:   "example.com/order",
		PackageName:  "order",
		AsSubmodule:  false,
		IncludeBazel: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.GenerateFiles(simpleModel(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"BUILD.bazel", "MODULE.bazel", ".bazelrc", ".bazelversion"} {
		if !hasFile(files, name) {
			t.Errorf("standalone mode should emit %s; got %v", name, fileNames(files))
		}
	}

	build := genFile(t, files, "BUILD.bazel")
	// Standalone: a binary, plus gazelle + nogo wiring, and external-repo labels.
	for _, want := range []string{
		`gazelle(name = "gazelle")`,
		`nogo(`,
		`go_binary(`,
		`@com_github_pflow_xyz_petri_pilot//pkg/runtime/api`,
	} {
		if !strings.Contains(build, want) {
			t.Errorf("standalone BUILD.bazel missing %q\n---\n%s", want, build)
		}
	}

	mod := genFile(t, files, "MODULE.bazel")
	for _, want := range []string{
		`module(`,
		`rules_go`,
		`gazelle`,
		`com_github_pflow_xyz_petri_pilot`,
	} {
		if !strings.Contains(mod, want) {
			t.Errorf("MODULE.bazel missing %q\n---\n%s", want, mod)
		}
	}
}

func TestBazel_GraphSubpackageGatedOnGraphQL(t *testing.T) {
	// Without GraphQL: no graph/BUILD.bazel and no graph dep.
	gen, _ := New(Options{
		ModulePath:   "github.com/pflow-xyz/petri-pilot/examples/order",
		PackageName:  "order",
		AsSubmodule:  true,
		IncludeBazel: true,
	})
	files, err := gen.GenerateFiles(simpleModel(t))
	if err != nil {
		t.Fatal(err)
	}
	if hasFile(files, "graph/BUILD.bazel") {
		t.Error("graph/BUILD.bazel should not be emitted without GraphQL")
	}
	if strings.Contains(genFile(t, files, "BUILD.bazel"), "/graph\"") {
		t.Error("graph dep should not appear without GraphQL")
	}
}

func TestBazel_NotEmittedWhenDisabled(t *testing.T) {
	gen, _ := New(Options{
		ModulePath:   "github.com/pflow-xyz/petri-pilot/examples/order",
		PackageName:  "order",
		AsSubmodule:  true,
		IncludeBazel: false,
	})
	files, err := gen.GenerateFiles(simpleModel(t))
	if err != nil {
		t.Fatal(err)
	}
	if hasFile(files, "BUILD.bazel") {
		t.Error("BUILD.bazel should not be emitted when IncludeBazel is false")
	}
}
