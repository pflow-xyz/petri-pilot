package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// repoRoot walks up from the current test file's directory to find this
// repo's own go.mod, so a generated app's go.mod can replace
// github.com/pflow-xyz/petri-pilot with the local checkout instead of
// needing network access to a proxy.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod) above " + dir)
		}
		dir = parent
	}
}

func callBuild(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	t.Setenv("PETRI_PILOT_LOCAL_PATH", repoRoot(t))

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_build"
	req.Params.Arguments = args

	result, err := handleBuild(context.Background(), req)
	if err != nil {
		t.Fatalf("handleBuild error: %v", err)
	}
	return result
}

func decodeBuildReport(t *testing.T, result *mcp.CallToolResult) buildReport {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool returned error: %s", resultText(result))
	}
	var report buildReport
	if err := json.Unmarshal([]byte(resultText(result)), &report); err != nil {
		t.Fatalf("decoding report: %v\n%s", err, resultText(result))
	}
	return report
}

// TestBuildRequiresExactlyOneInput checks the mutual-exclusion / required
// gating before anything expensive (codegen, a Go build) runs.
func TestBuildRequiresExactlyOneInput(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_build"
	req.Params.Arguments = map[string]any{"output_dir": t.TempDir()}
	result, err := handleBuild(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected an error when none of model/spec/bundle is given")
	}

	req.Params.Arguments = map[string]any{
		"model":      coreModelJSON,
		"spec":       `{"name":"x","entities":[]}`,
		"output_dir": t.TempDir(),
	}
	result, err = handleBuild(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected an error when both model and spec are given")
	}
}

func TestBuildRequiresOutputDir(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_build"
	req.Params.Arguments = map[string]any{"model": coreModelJSON}
	result, err := handleBuild(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected an error when output_dir is missing")
	}
}

// TestBuildFromModel is the single-net path (formerly petri_codegen
// language='go'): it writes a standalone app with its own go.mod and main.go
// and, with verify disabled, only checks that it compiles.
func TestBuildFromModel(t *testing.T) {
	dir := t.TempDir()
	result := callBuild(t, map[string]any{
		"model":      coreModelJSON,
		"output_dir": dir,
		"verify":     false,
	})
	report := decodeBuildReport(t, result)

	if !report.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", report.Build.Errors)
	}
	if report.Verify.Ran {
		t.Error("expected verify to be skipped")
	}
	wantFiles := []string{"go.mod", "main.go", "workflow.go", "aggregate.go", "api.go"}
	for _, want := range wantFiles {
		found := false
		for _, f := range report.FilesWritten {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q among files_written: %v", want, report.FilesWritten)
		}
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s on disk: %v", want, err)
		}
	}
}

// TestBuildFromSpec composes entities into one bundle; a fused pair of
// actions must surface as a coordinator-backed composed app, and the
// generated tree must include the new run harness (cmd/server/main.go) and
// go.mod that make it independently runnable.
func TestBuildFromSpec(t *testing.T) {
	spec := `{
	  "name": "shopdemo",
	  "entities": [
	    {"id": "order",
	     "fields": [{"id": "item_id", "type": "reference", "reference": {"entity": "inventory", "on_delete": "restrict"}}],
	     "states": [{"id": "draft", "initial": true}, {"id": "placed"}],
	     "actions": [{"id": "place_order", "from_states": ["draft"], "to_state": "placed",
	                  "effects": [{"field": "item_id", "value": "item_id"}]}]},
	    {"id": "inventory",
	     "fields": [{"id": "stock", "type": "int64"}],
	     "states": [{"id": "available", "initial": true}, {"id": "reserved"}],
	     "actions": [{"id": "reserve_stock", "from_states": ["available"], "to_state": "reserved",
	                  "effects": [{"field": "stock", "value": "amount", "op": "subtract"}]}]}
	  ]
	}`
	fusions := `[{"id": "order_reserves_stock", "members": [
	  {"entity": "order", "action": "place_order"},
	  {"entity": "inventory", "action": "reserve_stock"}]}]`

	dir := t.TempDir()
	result := callBuild(t, map[string]any{
		"spec":       spec,
		"fusions":    fusions,
		"output_dir": dir,
		"verify":     false,
	})
	report := decodeBuildReport(t, result)

	if !report.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", report.Build.Errors)
	}
	wantFiles := []string{
		"go.mod", filepath.Join("cmd", "server", "main.go"),
		"app.go", "flatmodel.go",
		filepath.Join("order", "aggregate.go"), filepath.Join("inventory", "aggregate.go"),
	}
	for _, want := range wantFiles {
		found := false
		for _, f := range report.FilesWritten {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q among files_written: %v", want, report.FilesWritten)
		}
	}
}

// TestBuildFromBundleDoc takes a raw bundle document with inline models.
func TestBuildFromBundleDoc(t *testing.T) {
	doc := `{
	  "name": "duo",
	  "subnets": [
	    {"id": "alpha", "model": {"name": "alpha",
	      "places": [{"id": "p", "kind": "token", "initial": 1}, {"id": "q", "kind": "token"}],
	      "transitions": [{"id": "go"}],
	      "arcs": [{"from": "p", "to": "go"}, {"from": "go", "to": "q"}]}},
	    {"id": "beta", "model": {"name": "beta",
	      "places": [{"id": "x", "kind": "token", "initial": 1}, {"id": "y", "kind": "token"}],
	      "transitions": [{"id": "sync"}],
	      "arcs": [{"from": "x", "to": "sync"}, {"from": "sync", "to": "y"}]}}
	  ],
	  "links": [{"id": "handshake", "kind": "event",
	    "from": {"subnet": "alpha", "transition": "go"},
	    "to": {"subnet": "beta", "transition": "sync"}}]
	}`

	dir := t.TempDir()
	result := callBuild(t, map[string]any{
		"bundle":     doc,
		"output_dir": dir,
		"verify":     false,
	})
	report := decodeBuildReport(t, result)

	if !report.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", report.Build.Errors)
	}
	for _, want := range []string{"app.go", filepath.Join("cmd", "server", "main.go"), "go.mod"} {
		found := false
		for _, f := range report.FilesWritten {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q among files_written: %v", want, report.FilesWritten)
		}
	}
}

// TestBuildVerifiesRunningApp is the test that proves the feature: it
// generates a small app, actually builds and runs the real binary, and
// checks the report shows verify.passed == true.
func TestBuildVerifiesRunningApp(t *testing.T) {
	model := `{
	  "name": "verifydemo2",
	  "places": [{"id": "pending", "initial": 1}, {"id": "done"}],
	  "transitions": [{"id": "finish"}],
	  "arcs": [{"from": "pending", "to": "finish"}, {"from": "finish", "to": "done"}]
	}`

	dir := t.TempDir()
	result := callBuild(t, map[string]any{
		"model":      model,
		"output_dir": dir,
		"verify":     true,
	})
	report := decodeBuildReport(t, result)

	if !report.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", report.Build.Errors)
	}
	if !report.Verify.Ran {
		t.Fatal("expected verify to have run")
	}
	if !report.Verify.Passed {
		t.Fatalf("expected verification to pass, got errors: %v\nmismatches: %+v", report.Verify.Errors, report.Verify.Mismatches)
	}
	found := false
	for _, tr := range report.Verify.TransitionsFired {
		if tr == "finish" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected \"finish\" to have fired, got %v", report.Verify.TransitionsFired)
	}
}

// TestBuildVerifiesRunningBundleApp exercises the harder path end to end:
// two fused subnets, verified against the real running composed app —
// create an instance per subnet, fire the fused transition via
// POST /fire/<transition>, and confirm the marking on both subnets matches
// the oracle.
func TestBuildVerifiesRunningBundleApp(t *testing.T) {
	doc := `{
	  "name": "duoverify",
	  "subnets": [
	    {"id": "alpha", "model": {"name": "alpha",
	      "places": [{"id": "p", "kind": "token", "initial": 1}, {"id": "q", "kind": "token"}],
	      "transitions": [{"id": "go"}],
	      "arcs": [{"from": "p", "to": "go"}, {"from": "go", "to": "q"}]}},
	    {"id": "beta", "model": {"name": "beta",
	      "places": [{"id": "x", "kind": "token", "initial": 1}, {"id": "y", "kind": "token"}],
	      "transitions": [{"id": "sync"}],
	      "arcs": [{"from": "x", "to": "sync"}, {"from": "sync", "to": "y"}]}}
	  ],
	  "links": [{"id": "handshake", "kind": "event",
	    "from": {"subnet": "alpha", "transition": "go"},
	    "to": {"subnet": "beta", "transition": "sync"}}]
	}`

	dir := t.TempDir()
	result := callBuild(t, map[string]any{
		"bundle":     doc,
		"output_dir": dir,
		"verify":     true,
	})
	report := decodeBuildReport(t, result)

	if !report.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", report.Build.Errors)
	}
	if !report.Verify.Ran {
		t.Fatal("expected verify to have run")
	}
	if !report.Verify.Passed {
		t.Fatalf("expected verification to pass, got errors: %v\nmismatches: %+v", report.Verify.Errors, report.Verify.Mismatches)
	}
	found := false
	for _, tr := range report.Verify.TransitionsFired {
		if tr == "handshake" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the fused transition \"handshake\" to have fired, got %v", report.Verify.TransitionsFired)
	}
}

func TestCodegenRejectsGoLanguage(t *testing.T) {
	result := callCodegen(t, "go")
	if !result.IsError {
		t.Fatal("expected an error for language='go'")
	}
	if !strings.Contains(resultText(result), "petri_build") {
		t.Errorf("error should point at petri_build: %s", resultText(result))
	}
}
