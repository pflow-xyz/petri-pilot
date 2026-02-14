package zkerc20

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/pflow-xyz/go-pflow/prover"
)

// ERC20WitnessFactory converts raw JSON witness maps into typed circuit assignments.
type ERC20WitnessFactory struct{}

// CreateAssignment builds a circuit assignment from a witness map.
//
// For "transition" circuit, expected witness keys:
//
//	pre_state_root, post_state_root, transition, amount,
//	pre_marking_0..pre_marking_4, post_marking_0..post_marking_4
//
// For "invariant" circuit, expected witness keys:
//
//	state_root, marking_0..marking_4
func (f *ERC20WitnessFactory) CreateAssignment(circuitName string, witness map[string]string) (frontend.Circuit, error) {
	switch circuitName {
	case "transition":
		return createTransitionAssignment(witness)
	case "invariant":
		return createInvariantAssignment(witness)
	default:
		return nil, fmt.Errorf("unknown circuit: %s", circuitName)
	}
}

func createTransitionAssignment(w map[string]string) (*ERC20TransitionCircuit, error) {
	c := &ERC20TransitionCircuit{}
	var err error

	c.PreStateRoot, err = parseField(w, "pre_state_root")
	if err != nil {
		return nil, err
	}
	c.PostStateRoot, err = parseField(w, "post_state_root")
	if err != nil {
		return nil, err
	}
	c.Transition, err = parseField(w, "transition")
	if err != nil {
		return nil, err
	}
	c.Amount, err = parseField(w, "amount")
	if err != nil {
		return nil, err
	}

	for i := 0; i < NumPlaces; i++ {
		c.PreMarking[i], err = parseField(w, fmt.Sprintf("pre_marking_%d", i))
		if err != nil {
			return nil, err
		}
		c.PostMarking[i], err = parseField(w, fmt.Sprintf("post_marking_%d", i))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func createInvariantAssignment(w map[string]string) (*ERC20InvariantCircuit, error) {
	c := &ERC20InvariantCircuit{}
	var err error

	c.StateRoot, err = parseField(w, "state_root")
	if err != nil {
		return nil, err
	}

	for i := 0; i < NumPlaces; i++ {
		c.Marking[i], err = parseField(w, fmt.Sprintf("marking_%d", i))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// parseField extracts a field from the witness map, parsing hex or decimal.
func parseField(w map[string]string, key string) (interface{}, error) {
	val, ok := w[key]
	if !ok {
		return nil, fmt.Errorf("missing witness field: %s", key)
	}
	return prover.ParseBigInt(val)
}

// NewERC20Service creates a prover.Service with "transition" and "invariant" circuits registered.
func NewERC20Service() (*prover.Service, error) {
	p := prover.NewProver()

	slog.Info("Compiling ERC-20 circuits...")

	type result struct {
		name string
		cc   *prover.CompiledCircuit
		err  error
	}
	results := make(chan result, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		cc, err := p.CompileCircuit("transition", &ERC20TransitionCircuit{})
		results <- result{"transition", cc, err}
	}()

	go func() {
		defer wg.Done()
		cc, err := p.CompileCircuit("invariant", &ERC20InvariantCircuit{})
		results <- result{"invariant", cc, err}
	}()

	wg.Wait()
	close(results)

	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("failed to compile %s circuit: %w", r.name, r.err)
		}
		p.StoreCircuit(r.name, r.cc)
		slog.Info("Circuit compiled",
			"name", r.name,
			"constraints", r.cc.Constraints,
			"public", r.cc.PublicVars,
			"private", r.cc.PrivateVars,
		)
	}

	factory := &ERC20WitnessFactory{}
	return prover.NewService(p, factory), nil
}
