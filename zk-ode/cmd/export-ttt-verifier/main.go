// Command export-ttt-verifier compiles the TTT circuit, generates keys, and
// exports the Solidity verifier contract.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pflow-xyz/go-pflow/prover"
	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

func main() {
	keyDir := "keys/zk-ode"
	if d := os.Getenv("ZK_KEY_DIR"); d != "" {
		keyDir = d
	}

	outFile := "solidity/src/TTTGroth16Verifier.sol"
	if len(os.Args) > 1 {
		outFile = os.Args[1]
	}

	log.Printf("Key directory: %s", keyDir)
	log.Printf("Output file: %s", outFile)

	p := prover.NewProverWithKeyDir(keyDir)

	log.Println("Compiling TTT circuit (118k constraints)...")
	cc, err := p.LoadOrCompile("ttt_step", &zkode.TTTStepCircuit{})
	if err != nil {
		log.Fatalf("Failed to compile: %v", err)
	}
	log.Printf("Circuit ready: %d constraints, %d public, %d private",
		cc.Constraints, cc.PublicVars, cc.PrivateVars)

	log.Println("Exporting Solidity verifier...")
	solidity, err := p.ExportVerifier("ttt_step")
	if err != nil {
		log.Fatalf("Failed to export verifier: %v", err)
	}

	if err := os.WriteFile(outFile, []byte(solidity), 0644); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	fmt.Printf("Verifier exported to %s (%d bytes)\n", outFile, len(solidity))

	// Print genesis root
	state := zkode.NewTTTODEState(zkode.TTTDefaultInitialMarking())
	fmt.Printf("Genesis root: 0x%s\n", state.Root.Text(16))
}
