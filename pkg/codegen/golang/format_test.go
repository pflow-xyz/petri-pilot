package golang

import (
	"go/format"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

func formatTestModel() *metamodel.Model {
	return &metamodel.Model{
		Name: "stoplight",
		Places: []metamodel.Place{
			{ID: "red", Initial: 1}, {ID: "green"}, {ID: "yellow"},
		},
		Transitions: []metamodel.Transition{
			{ID: "toGreen"}, {ID: "toYellow"}, {ID: "toRed"},
		},
		Arcs: []metamodel.Arc{
			{From: "red", To: "toGreen"}, {From: "toGreen", To: "green"},
			{From: "green", To: "toYellow"}, {From: "toYellow", To: "yellow"},
			{From: "yellow", To: "toRed"}, {From: "toRed", To: "red"},
		},
	}
}

// TestGeneratedGoIsFormatted is the guard that keeps generated/ gofmt-clean.
// Templates cannot align struct fields (alignment depends on the widest field,
// which the template cannot know), so without this the output is permanently
// gofmt-dirty and running gofmt on any package drags generated files into the
// diff.
func TestGeneratedGoIsFormatted(t *testing.T) {
	gen, err := New(Options{PackageName: "stoplight", AsSubmodule: true, IncludeTests: true})
	if err != nil {
		t.Fatalf("new generator: %v", err)
	}

	files, err := gen.GenerateFiles(formatTestModel())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no files generated")
	}

	checked := 0
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".go") {
			continue
		}
		checked++

		want, err := format.Source(f.Content)
		if err != nil {
			t.Errorf("%s is not valid Go: %v", f.Name, err)
			continue
		}
		if string(want) != string(f.Content) {
			t.Errorf("%s is not gofmt-clean; generator must format its output", f.Name)
		}
	}

	if checked == 0 {
		t.Fatal("no .go files were checked — the fixture generated nothing to verify")
	}
}

func TestFormatGoLeavesNonGoAlone(t *testing.T) {
	// A README or go.mod must pass through untouched — running it through
	// go/format would fail or corrupt it.
	content := []byte("# Title\n\n\n\nsome    text\n")
	got, err := formatGo("README.md", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("non-Go file was modified:\n got %q\nwant %q", got, content)
	}
}

func TestFormatGoFormats(t *testing.T) {
	got, err := formatGo("x.go", []byte("package x\n\n\n\nfunc  F( ) int  {return 1}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(got), "func  F") {
		t.Errorf("output was not formatted: %q", got)
	}
}

// TestFormatGoRejectsInvalid: a template emitting broken Go should fail at
// generation, naming the file, rather than writing something that fails to
// compile later with an error pointing at generated code.
func TestFormatGoRejectsInvalid(t *testing.T) {
	_, err := formatGo("broken.go", []byte("package x\nfunc F( {\n"))
	if err == nil {
		t.Fatal("expected an error for invalid Go")
	}
	if !strings.Contains(err.Error(), "broken.go") {
		t.Errorf("error %q should name the offending file", err)
	}
}
