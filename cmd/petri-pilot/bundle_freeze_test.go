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
// diffs every file against the committed examples/shop tree. Unlike the
// legacy freeze apps (whose committed trees predate template changes and get
// a hash-manifest baseline instead), the bundle pipeline was born
// reproducible — the committed tree IS the baseline, and any generator or
// template change that alters output fails here until examples/shop is
// deliberately regenerated in the same commit.
func TestBundleCodegenMatchesCommittedShop(t *testing.T) {
	// examples/shop is frozen: it is the reference composed app.
	assertBundleMatchesTree(t, "shop.bundle.json", "examples", "shop")
}

// TestBundleCodegenMatchesGeneratedWarehouse covers generated/, which unlike
// examples/ is meant to be reproducible: it holds current generator output, so
// any drift here means the tree should be refreshed in the same commit.
//
// warehouse is the guard-link case — it exercises a cross-subnet precondition
// that no app in examples/ has.
func TestBundleCodegenMatchesGeneratedWarehouse(t *testing.T) {
	assertBundleMatchesTree(t, "warehouse.bundle.json", "generated", "warehouse")
}

func assertBundleMatchesTree(t *testing.T, bundleFile, tree, app string) {
	t.Helper()
	// The bundle document comes from the embedded services FS, so this test
	// is hermetic under Bazel; only the committed tree is a disk
	// read (supplied as runfiles data).
	doc, err := services.FS.ReadFile("bundles/" + bundleFile)
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
		ModulePath:   "github.com/pflow-xyz/petri-pilot/" + tree + "/" + app,
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

	root := filepath.Join("..", "..", tree, app)
	if _, err := os.Stat(root); err != nil {
		// Bazel sandbox: the committed tree is not in runfiles. The go test
		// CI job runs this comparison; under Bazel the generation half above
		// still ran.
		t.Skipf("committed tree not available: %v", err)
	}
	for _, f := range files {
		committed, err := os.ReadFile(filepath.Join(root, f.Name))
		if err != nil {
			t.Errorf("%s: %v (new output file? regenerate %s/%s)", f.Name, err, tree, app)
			continue
		}
		if !bytes.Equal(committed, f.Content) {
			t.Errorf("%s: generated output differs from the committed tree — regenerate with:\n  go run ./cmd/petri-pilot codegen services/bundles/%s -o %s/%s",
				f.Name, bundleFile, tree, app)
		}
	}
}
