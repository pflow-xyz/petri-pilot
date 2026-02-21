package zkode

import (
	"fmt"
	"log/slog"
	"math/big"
	"os"

	"github.com/consensys/gnark/frontend"
	"github.com/pflow-xyz/go-pflow/prover"
)

// ZkODEWitnessFactory converts raw JSON witness maps into typed circuit assignments.
type ZkODEWitnessFactory struct{}

// CreateAssignment builds a circuit assignment from a witness map.
func (f *ZkODEWitnessFactory) CreateAssignment(circuitName string, witness map[string]string) (frontend.Circuit, error) {
	switch circuitName {
	case "tsit5_step":
		return createTsit5Assignment(witness)
	case "ttt_heatmap":
		return createTTTHeatmapAssignment(witness)
	default:
		return nil, fmt.Errorf("unknown circuit: %s", circuitName)
	}
}

func createTsit5Assignment(w map[string]string) (*Tsit5StepCircuit, error) {
	c := &Tsit5StepCircuit{}
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

	for t := 0; t < NumTransitions; t++ {
		c.Rates[t], err = parseWitnessField(w, fmt.Sprintf("rate_%d", t))
		if err != nil {
			return nil, err
		}
	}

	for p := 0; p < NumPlaces; p++ {
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

func createTTTHeatmapAssignment(w map[string]string) (*TTTHeatmapCircuit, error) {
	c := &TTTHeatmapCircuit{}
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

	for i := 0; i < 9; i++ {
		c.HeatmapScores[i], err = parseWitnessField(w, fmt.Sprintf("heatmap_score_%d", i))
		if err != nil {
			return nil, err
		}
	}

	for p := 0; p < TTTNumPlaces; p++ {
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
// If TTT_HEATMAP_ENABLED=1, also registers the "ttt_heatmap" circuit.
func NewZkODEService() (*prover.Service, error) {
	keyDir := os.Getenv("ZK_KEY_DIR")
	if keyDir == "" {
		keyDir = "keys/zk-ode"
	}
	p := prover.NewProverWithKeyDir(keyDir)

	slog.Info("Loading zk-ode circuits...", "key_dir", keyDir)

	cc, err := p.LoadOrCompile("tsit5_step", &Tsit5StepCircuit{})
	if err != nil {
		return nil, fmt.Errorf("failed to load/compile tsit5_step circuit: %w", err)
	}

	slog.Info("Circuit ready",
		"name", "tsit5_step",
		"constraints", cc.Constraints,
		"public", cc.PublicVars,
		"private", cc.PrivateVars,
	)

	// Optionally load the TTT heatmap circuit (tactical win/block detection)
	if os.Getenv("TTT_HEATMAP_ENABLED") == "1" {
		slog.Info("Loading TTT heatmap circuit...")
		hmCC, err := p.LoadOrCompile("ttt_heatmap", &TTTHeatmapCircuit{})
		if err != nil {
			return nil, fmt.Errorf("failed to load/compile ttt_heatmap circuit: %w", err)
		}
		slog.Info("Circuit ready",
			"name", "ttt_heatmap",
			"constraints", hmCC.Constraints,
			"public", hmCC.PublicVars,
			"private", hmCC.PrivateVars,
		)
	}

	factory := &ZkODEWitnessFactory{}
	return prover.NewService(p, factory), nil
}

// ProveStep generates a Groth16 proof for a single ODE step.
// This is a convenience wrapper for programmatic use (vs HTTP API).
func ProveStep(p *prover.Prover, state *ODEState, h *big.Int, rates [NumTransitions]*big.Int) (*prover.ProofResult, *ODEState, error) {
	w := ComputeStep(state, h, rates)
	assignment := w.ToCircuitAssignment()

	result, err := p.Prove("tsit5_step", assignment)
	if err != nil {
		return nil, nil, fmt.Errorf("proof generation failed: %w", err)
	}

	return result, w.PostState, nil
}

// ProveTTTHeatmapStep generates a Groth16 proof for a TTT heatmap step
// (ODE integration + tactical win/block evaluation).
func ProveTTTHeatmapStep(p *prover.Prover, state *TTTODEState, h *big.Int) (*prover.ProofResult, *TTTODEState, *TTTHeatmapWitness, error) {
	w := ComputeTTTHeatmapStep(state, h)
	assignment := w.ToHeatmapCircuitAssignment()

	result, err := p.Prove("ttt_heatmap", assignment)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("TTT heatmap proof generation failed: %w", err)
	}

	return result, w.PostState, w, nil
}
