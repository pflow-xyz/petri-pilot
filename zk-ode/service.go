package zkode

import (
	"fmt"
	"log/slog"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/pflow-xyz/go-pflow/prover"
)

// ZkODEWitnessFactory converts raw JSON witness maps into typed circuit assignments.
type ZkODEWitnessFactory struct {
	Net NetConfig
}

// CreateAssignment builds a circuit assignment from a witness map.
//
// For "tsit5_step" circuit, expected witness keys:
//
//	pre_state_root, post_state_root, step_size,
//	rate_0..rate_{M-1}, pre_marking_0..pre_marking_{N-1}, post_marking_0..post_marking_{N-1}
func (f *ZkODEWitnessFactory) CreateAssignment(circuitName string, witness map[string]string) (frontend.Circuit, error) {
	switch circuitName {
	case "tsit5_step":
		return f.createTsit5Assignment(witness)
	default:
		return nil, fmt.Errorf("unknown circuit: %s", circuitName)
	}
}

func (f *ZkODEWitnessFactory) createTsit5Assignment(w map[string]string) (*Tsit5StepCircuit, error) {
	c := NewTsit5StepCircuit(f.Net)
	var err error

	c.PreStateRoot, err = parseWitnessField(w, "pre_state_root")
	if err != nil {
		return nil, err
	}
	c.PostStateRoot, err = parseWitnessField(w, "post_state_root")
	if err != nil {
		return nil, err
	}
	c.StepSize, err = parseWitnessField(w, "step_size")
	if err != nil {
		return nil, err
	}

	for t := 0; t < f.Net.NumTransitions; t++ {
		c.Rates[t], err = parseWitnessField(w, fmt.Sprintf("rate_%d", t))
		if err != nil {
			return nil, err
		}
	}

	for p := 0; p < f.Net.NumPlaces; p++ {
		c.PreMarking[p], err = parseWitnessField(w, fmt.Sprintf("pre_marking_%d", p))
		if err != nil {
			return nil, err
		}
		c.PostMarking[p], err = parseWitnessField(w, fmt.Sprintf("post_marking_%d", p))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func parseWitnessField(w map[string]string, key string) (interface{}, error) {
	val, ok := w[key]
	if !ok {
		return nil, fmt.Errorf("missing witness field: %s", key)
	}
	return prover.ParseBigInt(val)
}

// NewZkODEService creates a prover.Service with the "tsit5_step" circuit registered.
func NewZkODEService(net NetConfig) (*prover.Service, error) {
	p := prover.NewProver()

	slog.Info("Compiling zk-ode circuits...")

	template := NewTsit5StepCircuit(net)
	cc, err := p.CompileCircuit("tsit5_step", template)
	if err != nil {
		return nil, fmt.Errorf("failed to compile tsit5_step circuit: %w", err)
	}
	p.StoreCircuit("tsit5_step", cc)

	slog.Info("Circuit compiled",
		"name", "tsit5_step",
		"constraints", cc.Constraints,
		"public", cc.PublicVars,
		"private", cc.PrivateVars,
	)

	factory := &ZkODEWitnessFactory{Net: net}
	return prover.NewService(p, factory), nil
}

// ExportVerifier compiles the Tsit5 circuit for the given net, runs trusted setup,
// and exports the gnark-generated Groth16 Solidity verifier.
func ExportVerifier(net NetConfig) (string, error) {
	p := prover.NewProver()
	template := NewTsit5StepCircuit(net)
	cc, err := p.CompileCircuit("tsit5_step", template)
	if err != nil {
		return "", fmt.Errorf("compile: %w", err)
	}
	p.StoreCircuit("tsit5_step", cc)
	return p.ExportVerifier("tsit5_step")
}

// ProveStep generates a Groth16 proof for a single ODE step.
// This is a convenience wrapper for programmatic use (vs HTTP API).
func ProveStep(net NetConfig, p *prover.Prover, state *ODEState, h *big.Int, rates []*big.Int) (*prover.ProofResult, *ODEState, error) {
	w := ComputeStep(net, state, h, rates)
	assignment := w.ToCircuitAssignment()

	result, err := p.Prove("tsit5_step", assignment)
	if err != nil {
		return nil, nil, fmt.Errorf("proof generation failed: %w", err)
	}

	return result, w.PostState, nil
}
