package appverify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
)

// repoRoot walks up from the current package directory to find the module
// root (the directory carrying this repo's own go.mod), so the generated
// app's go.mod can replace github.com/pflow-xyz/petri-pilot with the local
// checkout instead of needing network access to a proxy.
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

// smallModel is a tiny single-net workflow: two places, one transition. It
// exists purely so this test can generate, build and run a real app without
// depending on any fixture in services/.
func smallModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Name: "verifydemo",
		Places: []goflowmetamodel.Place{
			{ID: "pending", Initial: 1},
			{ID: "done", Initial: 0},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "finish"},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "pending", To: "finish", Weight: 1},
			{From: "finish", To: "done", Weight: 1},
		},
	}
}

// TestVerifyAppSingleNet generates a tiny app, builds it as a standalone
// module, starts the real binary and runs VerifyApp against it end to end.
// This is the test that proves the feature: it must actually start a
// process, hit it over HTTP, and see Passed == true.
func TestVerifyAppSingleNet(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a real binary; skipped in -short")
	}

	model := smallModel()
	dir := t.TempDir()

	root := repoRoot(t)
	localReplaceEnv := "PETRI_PILOT_LOCAL_PATH=" + root

	// go.mod's replace directive is baked in at generation time from this
	// environment variable, so it must be set before GenerateFiles runs.
	os.Setenv("PETRI_PILOT_LOCAL_PATH", root)
	defer os.Unsetenv("PETRI_PILOT_LOCAL_PATH")

	gen, err := golang.New(golang.Options{
		ModulePath:   "verifydemo",
		IncludeTests: false,
	})
	if err != nil {
		t.Fatalf("creating generator: %v", err)
	}
	files, err := gen.GenerateFiles(model)
	if err != nil {
		t.Fatalf("generating files: %v", err)
	}
	for _, f := range files {
		p := filepath.Join(dir, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := Tidy(dir, localReplaceEnv); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}

	bin := filepath.Join(dir, "verifydemo-bin")
	if out, err := BuildBinary(dir, bin, ".", localReplaceEnv); err != nil {
		t.Fatalf("building generated app: %v\n%s", err, out)
	}

	proc, err := StartProcess(dir, bin, 20*time.Second, localReplaceEnv)
	if err != nil {
		stdout, stderr := proc.Output()
		t.Fatalf("starting generated app: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	defer proc.Stop()

	modelJSON, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("marshaling model: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	report, err := VerifyApp(ctx, proc.URL, modelJSON, false)
	if err != nil {
		t.Fatalf("VerifyApp returned an error: %v", err)
	}

	if !report.Passed {
		stdout, stderr := proc.Output()
		reportJSON, _ := json.MarshalIndent(report, "", "  ")
		t.Fatalf("verification did not pass:\n%s\nstdout:\n%s\nstderr:\n%s", reportJSON, stdout, stderr)
	}
	if !report.HealthOK {
		t.Error("expected HealthOK")
	}
	if !report.InitialMarkingOK {
		t.Error("expected InitialMarkingOK")
	}
	if report.Steps == 0 {
		t.Error("expected at least one transition to have fired")
	}
	found := false
	for _, tr := range report.TransitionsFired {
		if tr == "finish" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected \"finish\" to have fired, got %v", report.TransitionsFired)
	}
}
