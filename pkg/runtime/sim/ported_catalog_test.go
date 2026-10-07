package sim_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Catalog loader for the ported structural-reading tests (from sim's
// diagnose_test.go): every services/*.json that parses as a model.

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
			// Bazel runs tests in a hermetic sandbox with no source tree, so
			// this is "cannot run here", not "broken". Skipping keeps the
			// Bazel graph honest while `go test ./...` still exercises it —
			// the same accommodation //pkg/mcp:mcp_test makes for the tests
			// that shell out to the Go toolchain.
			t.Skip("no go.mod above the working directory: needs the source tree")
		}
		dir = parent
	}
}

func loadCatalogModels(t *testing.T) map[string]*metamodel.Model {
	t.Helper()
	root := repoRoot(t)
	paths, _ := filepath.Glob(filepath.Join(root, "services", "*.json"))
	out := map[string]*metamodel.Model{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var m metamodel.Model
		if json.Unmarshal(data, &m) != nil || len(m.Transitions) == 0 || len(m.Places) == 0 {
			continue
		}
		out[m.Name] = &m
	}
	return out
}

func loadModel(t *testing.T, name string) *metamodel.Model {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "services", name))
	if err != nil {
		t.Skipf("%s not present", name)
	}
	var m metamodel.Model
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return &m
}
