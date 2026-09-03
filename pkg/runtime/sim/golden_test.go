package sim_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// The goldens under testdata/ were generated on the PRE-MOVE engine — the
// SSA as it stood in this package before it was promoted into
// go-pflow/stochastic — at petri-pilot commit
//
//	67538d70c977fa7d6de9e0ac1f247da900b64600
//
// After the switch, the same seed must reproduce the same bytes. Anything
// that changes them is either a real change to the engine (bump the goldens
// with -update, and say why in the commit) or a bug in the move.
var updateGolden = flag.Bool("update", false, "regenerate the goldens under testdata/")

func goldenCoffeeshop(t *testing.T) *metamodel.Model {
	t.Helper()
	root, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		p := filepath.Join(root, "services", "coffeeshop.json")
		if b, err := os.ReadFile(p); err == nil {
			var m metamodel.Model
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			return &m
		}
		root = filepath.Dir(root)
	}
	t.Skip("services/coffeeshop.json not found")
	return nil
}

func checkGolden(t *testing.T, name string, res *sim.Result) {
	t.Helper()
	got, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to generate)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the pre-move golden (%d bytes got, %d want); "+
			"if the engine really changed, regenerate with -update and explain why", name, len(got), len(want))
	}
}

func TestGoldenSimulateCoffeeshop(t *testing.T) {
	m := goldenCoffeeshop(t)
	res, err := sim.Simulate(m, nil, sim.Options{Horizon: 8, Samples: 60, Realizations: 5, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "simulate_coffeeshop_seed42.golden", res)
}

func TestGoldenRunCafeRush(t *testing.T) {
	res, err := sim.Run(cafe.FlatModel(), sim.Scenario{
		Name: "rush",
		Schedule: map[string][]sim.Segment{
			"counter/order_latte": {{Until: 2, Value: 3}, {Until: 8, Value: 1}},
		},
		Horizon: 8, Samples: 60, Realizations: 5, Seed: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "run_cafe_rush_seed42.golden", res)
}
