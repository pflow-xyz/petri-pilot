// Command paritygolden writes the byte-exact parity goldens that go-pflow's
// stochastic package checks itself against forever.
//
// The committed goldens were produced by the PRE-MOVE petri-pilot engine
// (sim.Simulate as it stood before the SSA was promoted into
// go-pflow/stochastic) on two models, recording every number the run
// produced. go-pflow's stochastic/parity_test.go then asserts == on every
// float, which is what makes "moved verbatim" a checked claim rather than a
// code-review impression.
//
// Generating commit (petri-pilot, branch sim-import-go-pflow, pre-move):
//
//	67538d70c977fa7d6de9e0ac1f247da900b64600
//
// Since that commit pkg/runtime/sim delegates to go-pflow/stochastic, so
// running this program today reproduces the goldens THROUGH the moved engine
// — it is a convenient reproduction, not independent evidence. Do not
// regenerate the goldens with it to "fix" a parity failure; a mismatch means
// the engine changed (see go-pflow/stochastic/testdata/README.md).
//
// Usage:
//
//	go run ./pkg/runtime/sim/internal/paritygolden \
//	    ../go-pflow/stochastic/testdata/parity_coffeeshop_seed42.json \
//	    ../go-pflow/stochastic/testdata/parity_sir_seed11.json
//
// The coffeeshop fixture is services/coffeeshop.json, found by walking up from
// the working directory (override with -coffeeshop). Floats are emitted by
// encoding/json, whose shortest-round-trip formatting is exact.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

func main() {
	fixture := flag.String("coffeeshop", "", "path to services/coffeeshop.json (default: walk up from cwd)")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: paritygolden [-coffeeshop path] <coffeeshop_out.json> <sir_out.json>")
		os.Exit(2)
	}

	shop, err := loadCoffeeshop(*fixture)
	if err != nil {
		fail(err)
	}
	if err := write(flag.Arg(0), shop, sim.Options{Horizon: 8, Samples: 60, Realizations: 5, Seed: 42}); err != nil {
		fail(err)
	}
	if err := write(flag.Arg(1), sir(), sim.Options{Horizon: 40, Samples: 81, Realizations: 8, Seed: 11}); err != nil {
		fail(err)
	}
}

func write(path string, m *metamodel.Model, opts sim.Options) error {
	res, err := sim.Simulate(m, nil, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// sir is the finite-N SIR net of go-pflow's consistency gate (Case 2):
// R0 = 5, infect is second order in S and I, recover is first order in I.
func sir() *metamodel.Model {
	return &metamodel.Model{
		Name: "sir",
		Places: []metamodel.Place{
			{ID: "S", Initial: 990},
			{ID: "I", Initial: 10},
			{ID: "R", Initial: 0},
		},
		Transitions: []metamodel.Transition{
			{ID: "infect", Rate: 0.0005},
			{ID: "recover", Rate: 0.1},
		},
		Arcs: []metamodel.Arc{
			{From: "S", To: "infect", Weight: 1},
			{From: "I", To: "infect", Weight: 1},
			{From: "infect", To: "I", Weight: 2},
			{From: "I", To: "recover", Weight: 1},
			{From: "recover", To: "R", Weight: 1},
		},
	}
}

func loadCoffeeshop(path string) (*metamodel.Model, error) {
	if path == "" {
		root, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		for i := 0; i < 6; i++ {
			p := filepath.Join(root, "services", "coffeeshop.json")
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
			root = filepath.Dir(root)
		}
		if path == "" {
			return nil, fmt.Errorf("services/coffeeshop.json not found above the working directory; pass -coffeeshop")
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m metamodel.Model
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "paritygolden:", err)
	os.Exit(1)
}
