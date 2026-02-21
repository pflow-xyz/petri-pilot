// Command submit-ttt-proof generates TTT ZK proofs using the heatmap circuit
// (tactical win/block detection) and prints cast send commands for on-chain submission.
//
// Set TTT_STEPS=N to generate N chained proofs (default: 1).
// Each proof starts from the post-state of the previous one.
package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/pflow-xyz/go-pflow/prover"
	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

func main() {
	keyDir := "keys/zk-ode"
	if d := os.Getenv("ZK_KEY_DIR"); d != "" {
		keyDir = d
	}

	contractAddr := os.Getenv("TTT_ZKODE_ADDRESS")
	if contractAddr == "" {
		contractAddr = "0xe260BA6e13a393018F394B9d847aEd4809f8d9Fa"
	}

	rpcURL := os.Getenv("BASE_SEPOLIA_RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://sepolia.base.org"
	}

	numSteps := 1
	if s := os.Getenv("TTT_STEPS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			log.Fatalf("Invalid TTT_STEPS: %v", err)
		}
		numSteps = n
	}

	log.Printf("Key directory: %s", keyDir)
	log.Printf("Contract: %s", contractAddr)
	log.Printf("Steps to generate: %d", numSteps)

	p := prover.NewProverWithKeyDir(keyDir)

	log.Println("Loading TTT heatmap circuit...")
	cc, err := p.LoadOrCompile("ttt_heatmap", &zkode.TTTHeatmapCircuit{})
	if err != nil {
		log.Fatalf("Failed to load circuit: %v", err)
	}
	log.Printf("Circuit ready: %d constraints", cc.Constraints)

	// Build genesis state (empty board, X's turn)
	state := zkode.NewTTTODEState(zkode.TTTDefaultInitialMarking())
	log.Printf("Genesis root: 0x%s", state.Root.Text(16))

	h := zkode.FixFromFloat(0.01)

	var scriptParts []string

	for step := 0; step < numSteps; step++ {
		log.Printf("=== Step %d (from root 0x%s) ===", step+1, state.Root.Text(16))

		w := zkode.ComputeTTTHeatmapStep(state, h)

		log.Printf("Heatmap scores:")
		for i := 0; i < 9; i++ {
			score := zkode.FixToFloat(w.HeatmapScores[i])
			if score > 0.001 {
				log.Printf("  cell %d (%d,%d): %.4f", i, i/3, i%3, score)
			}
		}

		// Find position with highest heatmap score
		bestPos := 0
		bestScore := w.HeatmapScores[0]
		for i := 1; i < 9; i++ {
			if w.HeatmapScores[i].Cmp(bestScore) > 0 {
				bestPos = i
				bestScore = w.HeatmapScores[i]
			}
		}
		chosenScore := zkode.FixToFloat(bestScore)

		// Map position to transition for discrete move application
		isXTurn := state.Marking[zkode.XTurn].Cmp(zkode.FixFromFloat(0.0)) > 0
		var discreteTransition int
		if isXTurn {
			discreteTransition = zkode.TXPlay00 + bestPos
		} else {
			discreteTransition = zkode.TOPlay00 + bestPos
		}
		chosenLabel := zkode.TTTTransitionNames[discreteTransition]

		log.Printf("Optimal: cell %d (%d,%d) -> %s (score: %.4f)",
			bestPos, bestPos/3, bestPos%3, chosenLabel, chosenScore)

		log.Printf("Generating heatmap proof...")
		result, _, _, err := zkode.ProveTTTHeatmapStep(p, state, h)
		if err != nil {
			log.Fatalf("Proof generation failed at step %d: %v", step+1, err)
		}
		log.Printf("Proof generated: %d public inputs", len(result.PublicInputs))

		// Compute discrete post-move board and its root
		discretePost := zkode.ApplyDiscreteMove(state.Marking, discreteTransition)
		nextRoot := zkode.ComputeRoot(discretePost[:])
		log.Printf("Discrete next root: 0x%s", nextRoot.Text(16))

		// Format proof as [p0,p1,p2,p3,p4,p5,p6,p7]
		var proofParts []string
		for _, v := range result.RawProof {
			proofParts = append(proofParts, v.Text(10))
		}
		proofStr := "[" + strings.Join(proofParts, ",") + "]"

		// Format public inputs
		var inputParts []string
		for _, hex := range result.PublicInputs {
			v := new(big.Int)
			clean := strings.TrimPrefix(hex, "0x")
			v.SetString(clean, 16)
			inputParts = append(inputParts, v.Text(10))
		}
		inputsStr := "[" + strings.Join(inputParts, ",") + "]"

		nextRootStr := nextRoot.Text(10)

		fmt.Println()
		fmt.Printf("=== Step %d: Cast Send Command ===\n", step+1)
		fmt.Println()
		fmt.Printf("cast send %s \\\n", contractAddr)
		fmt.Printf("  \"submitStep(uint256[8],uint256[],uint256,uint256)\" \\\n")
		fmt.Printf("  \"%s\" \\\n", proofStr)
		fmt.Printf("  \"%s\" \\\n", inputsStr)
		fmt.Printf("  %d \\\n", bestPos)
		fmt.Printf("  %s \\\n", nextRootStr)
		fmt.Printf("  --rpc-url %s \\\n", rpcURL)
		fmt.Println("  --private-key $DEPLOYER_PRIVATE_KEY")
		fmt.Println()

		scriptParts = append(scriptParts, fmt.Sprintf(
			`echo "Submitting step %d: %s (cell %d, score %.4f)"
cast send %s \
  "submitStep(uint256[8],uint256[],uint256,uint256)" \
  "%s" \
  "%s" \
  %d \
  %s \
  --rpc-url %s \
  --private-key $DEPLOYER_PRIVATE_KEY
`,
			step+1, chosenLabel, bestPos, chosenScore,
			contractAddr, proofStr, inputsStr, bestPos, nextRootStr, rpcURL))

		// Advance to the discrete post-move board for next step
		state = zkode.NewTTTODEState(discretePost)
	}

	// Write combined script
	scriptFile := "submit-ttt-proof.sh"
	script := "#!/bin/bash\n# Submit TTT proofs on-chain\nset -e\n\n" +
		strings.Join(scriptParts, "\n")

	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		log.Printf("Warning: could not write script file: %v", err)
	} else {
		fmt.Printf("Script saved to %s\n", scriptFile)
	}
}
