package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/bundle"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/services"
)

// TestBundleCodegenMatchesCommittedShop regenerates the shop bundle and
// diffs every file against the committed generated/shop tree. Unlike the
// legacy freeze apps (whose committed trees predate template changes and get
// a hash-manifest baseline instead), the bundle pipeline was born
// reproducible — the committed tree IS the baseline, and any generator or
// template change that alters output fails here until generated/shop is
// deliberately regenerated in the same commit.
func TestBundleCodegenMatchesCommittedShop(t *testing.T) {
	// The bundle document comes from the embedded services FS, so this test
	// is hermetic under Bazel; only the committed generated/ tree is a disk
	// read (supplied as runfiles data).
	doc, err := services.FS.ReadFile("bundles/shop.bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Load(doc, func(ref string) (*metamodel.Model, error) {
		raw, err := services.FS.ReadFile("bundles/" + ref)
		if err != nil {
			return nil, err
		}
		var m metamodel.Model
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		return &m, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	gen, err := golang.New(golang.Options{
		ModulePath:   "github.com/pflow-xyz/petri-pilot/generated/shop",
		IncludeTests: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.GenerateBundleFiles(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files generated")
	}

	root := filepath.Join("..", "..", "generated", "shop")
	if _, err := os.Stat(root); err != nil {
		// Bazel sandbox: the committed tree is not in runfiles. The go test
		// CI job runs this comparison; under Bazel the generation half above
		// still ran.
		t.Skipf("committed tree not available: %v", err)
	}
	for _, f := range files {
		committed, err := os.ReadFile(filepath.Join(root, f.Name))
		if err != nil {
			t.Errorf("%s: %v (new output file? regenerate generated/shop)", f.Name, err)
			continue
		}
		if !bytes.Equal(committed, f.Content) {
			t.Errorf("%s: generated output differs from committed tree — regenerate generated/shop with:\n  go run ./cmd/petri-pilot codegen services/bundles/shop.bundle.json -o generated/shop", f.Name)
		}
	}
}
