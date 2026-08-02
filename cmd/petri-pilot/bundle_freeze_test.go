package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/bundle"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
)

// TestBundleCodegenMatchesCommittedShop regenerates the shop bundle and
// diffs every file against the committed generated/shop tree. Unlike the
// legacy freeze apps (whose committed trees predate template changes and get
// a hash-manifest baseline instead), the bundle pipeline was born
// reproducible — the committed tree IS the baseline, and any generator or
// template change that alters output fails here until generated/shop is
// deliberately regenerated in the same commit.
func TestBundleCodegenMatchesCommittedShop(t *testing.T) {
	b, err := bundle.LoadFile(filepath.Join("..", "..", "services", "bundles", "shop.bundle.json"))
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
