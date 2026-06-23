// Command zk-verifier-provenance binds each deployed Groth16 verifier to the
// circuit it was generated from, and surfaces when the circuit source has
// drifted away from what's actually on-chain (F4 Tier 3).
//
// The breach this closes: the on-chain verifiers (CLAUDE.md "ZkOde Contracts")
// are exported from a gnark circuit + a one-time trusted setup. Nothing checked
// that the committed circuit still matches the *deployed* verifier — a circuit
// edit that wasn't re-exported/redeployed silently leaves the chain verifying an
// older relation than the source describes.
//
// Model. Each verifier carries an immutable `deployed` baseline — the circuit
// (commit, public inputs, constraint count, R1CS digest) the on-chain verifier
// actually attests, captured once from the deploy commit. The tool compiles the
// *current* circuit hermetically (reproducible since F4 Tier 2) and records it
// as `current`. `inSyncWithDeployment` is true iff the current R1CS digest equals
// the deployed baseline's.
//
// The check (no -write):
//   - recomputes current + the committed verifier's source hash and asserts they
//     match the manifest — so a circuit edit or a verifier-source edit that
//     wasn't reconciled fails (run -write and review);
//   - asserts the committed verifier's hardcoded input arity matches the circuit
//     public-input count (the verifier's input[] excludes gnark's ONE wire);
//   - reports drift loudly but does NOT fail on it: a stale deployment is a real,
//     acknowledged state (inSyncWithDeployment:false), not a manifest error. Flip
//     `failOnDrift` to gate CI on it once the redeploy/revert decision is made.
//
// What this does NOT do: prove the *deployed bytecode* equals the committed
// Solidity — that needs forge+RPC (scripts/zk-onchain-bytecode-check.sh). The
// verifying key is baked into the committed .sol as constants, so the committed
// verifier is the vk of record.
//
// Usage:
//
//	go run ./cmd/zk-verifier-provenance            # check against provenance.json
//	go run ./cmd/zk-verifier-provenance -write     # recompute current/ + rewrite it
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/logger"

	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

const manifestPath = "zk-ode/provenance.json"

// failOnDrift gates CI on a stale deployment. Left false for now: the heatmap
// verifier is knowingly drifted (see provenance.json) pending a redeploy/revert
// decision, and we don't want that to red CI. Flip to true once reconciled so a
// future drift can't slip in.
const failOnDrift = false

// circuitState is a circuit's compiled shape — the falsifiable fingerprint.
type circuitState struct {
	Commit       string `json:"commit,omitempty"` // deploy commit (baseline only)
	PublicInputs int    `json:"publicInputs"`
	Constraints  int    `json:"constraints"`
	R1CSSHA256   string `json:"r1csSHA256"`
}

// verifier pairs a deployed verifier with its circuit and its deploy baseline.
type verifier struct {
	Name           string           `json:"name"`
	Circuit        string           `json:"circuit"`
	VerifierSource string           `json:"verifierSource"`
	SourceSHA256   string           `json:"verifierSourceSHA256"`
	Deployment     map[string]any   `json:"deployment"`
	Deployed       circuitState     `json:"deployed"` // immutable historical fact
	Current        circuitState     `json:"current"`  // recomputed from source
	InSync         bool             `json:"inSyncWithDeployment"`
	circuit        frontend.Circuit `json:"-"`
}

// pairs is the source of truth. The Deployed baseline is captured once from each
// verifier's deploy commit (compile the circuit at that commit) and never
// recomputed — it records what the on-chain verifier attests.
var pairs = []verifier{
	{
		Name:           "ttt_heatmap",
		Circuit:        "zk-ode.TTTHeatmapCircuit",
		VerifierSource: "solidity/src/TTTHeatmapVerifier.sol",
		circuit:        &zkode.TTTHeatmapCircuit{},
		Deployment: map[string]any{
			"address": "0x97a6Bb8FBBbBb81BF36456829A6a41e29030f351",
			"chainId": 84532,
			"network": "base-sepolia",
		},
		Deployed: circuitState{
			Commit:       "a0d6eb5",
			PublicInputs: 12,
			Constraints:  176891,
			R1CSSHA256:   "2be6c8562926862686bfb1219e7aef02cc45d0682f0a615e0a318fc28148570a",
		},
	},
	{
		Name:           "cascade",
		Circuit:        "zk-ode.Tsit5StepCircuit",
		VerifierSource: "solidity/src/Groth16Verifier.sol",
		circuit:        &zkode.Tsit5StepCircuit{},
		Deployment: map[string]any{
			"address": "0xA675a162C5097e5eBa2968C918D4D0530b7005Ae",
			"chainId": 84532,
			"network": "base-sepolia",
		},
		Deployed: circuitState{
			Commit:       "d84f9fc",
			PublicInputs: 5,
			Constraints:  9644,
			R1CSSHA256:   "ce2710080f475980c3249de54366140d3c87711308228c84cbcf7f37e75b2c84",
		},
	},
}

var inputArityRe = regexp.MustCompile(`uint256\[(\d+)\] calldata input`)

func main() {
	write := flag.Bool("write", false, "recompute current state and rewrite the manifest")
	flag.Parse()
	logger.Disable()

	computed := make([]verifier, 0, len(pairs))
	drift := false
	for i := range pairs {
		v, err := compute(pairs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", pairs[i].Name, err)
			os.Exit(2)
		}
		status := "ALIGNED"
		if !v.InSync {
			status = "DRIFT"
			drift = true
		}
		fmt.Printf("%-12s %-7s deployed=%d@%s current=%d  verifier=%s[%d]\n",
			v.Name, status, v.Deployed.Constraints, v.Deployed.Commit,
			v.Current.Constraints, filepath.Base(v.VerifierSource), v.Current.PublicInputs)
		if !v.InSync {
			fmt.Printf("             ⚠ on-chain %v attests the %d-constraint circuit @%s; "+
				"source now compiles to %d. Redeploy or reconcile.\n",
				v.Deployment["address"], v.Deployed.Constraints, v.Deployed.Commit, v.Current.Constraints)
		}
		computed = append(computed, v)
	}

	if *write {
		if err := writeManifest(computed); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("wrote %s\n", manifestPath)
		return
	}
	if err := checkManifest(computed); err != nil {
		fmt.Fprintf(os.Stderr, "\nMANIFEST STALE: %v\n", err)
		fmt.Fprintln(os.Stderr, "the circuit or verifier source changed without updating the manifest; "+
			"run `go run ./cmd/zk-verifier-provenance -write` and review the diff.")
		os.Exit(1)
	}
	if drift {
		fmt.Println("\nWARN: a deployed verifier is stale relative to its circuit (see ⚠ above). " +
			"Manifest is consistent; reconcile via redeploy/revert.")
		if failOnDrift {
			os.Exit(1)
		}
		return
	}
	fmt.Println("\nPASS: every committed verifier matches its circuit and its deployment")
}

// compute compiles the current circuit and parses the verifier, filling the
// derived fields (Current, SourceSHA256, InSync) of v.
func compute(v verifier) (verifier, error) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, v.circuit)
	if err != nil {
		return v, fmt.Errorf("compile: %w", err)
	}
	// gnark counts the implicit ONE wire among public variables; the Solidity
	// verifier's input[] excludes it.
	pub := ccs.GetNbPublicVariables() - 1

	h := sha256.New()
	if _, err := ccs.WriteTo(h); err != nil {
		return v, fmt.Errorf("r1cs digest: %w", err)
	}
	v.Current = circuitState{
		PublicInputs: pub,
		Constraints:  ccs.GetNbConstraints(),
		R1CSSHA256:   hex.EncodeToString(h.Sum(nil)),
	}
	v.InSync = v.Current.R1CSSHA256 == v.Deployed.R1CSSHA256

	src, err := os.ReadFile(v.VerifierSource)
	if err != nil {
		return v, fmt.Errorf("read verifier: %w", err)
	}
	sum := sha256.Sum256(src)
	v.SourceSHA256 = hex.EncodeToString(sum[:])

	arity, err := verifierArity(src)
	if err != nil {
		return v, err
	}
	if arity != pub {
		return v, fmt.Errorf("verifier hardcodes %d public inputs but circuit has %d", arity, pub)
	}
	return v, nil
}

// verifierArity extracts the single public-input array length the verifier
// hardcodes, requiring every occurrence to agree.
func verifierArity(src []byte) (int, error) {
	ms := inputArityRe.FindAllSubmatch(src, -1)
	if len(ms) == 0 {
		return 0, fmt.Errorf("no `uint256[N] calldata input` found in verifier")
	}
	first := string(ms[0][1])
	for _, m := range ms[1:] {
		if string(m[1]) != first {
			return 0, fmt.Errorf("verifier has inconsistent input arities (%s vs %s)", first, m[1])
		}
	}
	var n int
	fmt.Sscanf(first, "%d", &n)
	return n, nil
}

func writeManifest(vs []verifier) error {
	out := map[string]any{
		"_comment": "F4 Tier 3 verifier provenance. `deployed` is the circuit each on-chain " +
			"verifier attests (captured from its deploy commit, immutable); `current` is what the " +
			"source compiles to now. inSyncWithDeployment=false means the verifier is stale and needs " +
			"a redeploy/revert. Recompute current with `go run ./cmd/zk-verifier-provenance -write`.",
		"verifiers": vs,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, append(b, '\n'), 0644)
}

func checkManifest(computed []verifier) error {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest (run with -write to create it): %w", err)
	}
	var m struct {
		Verifiers []verifier `json:"verifiers"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if len(m.Verifiers) != len(computed) {
		return fmt.Errorf("manifest lists %d verifiers, code tracks %d", len(m.Verifiers), len(computed))
	}
	byName := map[string]verifier{}
	for _, v := range m.Verifiers {
		byName[v.Name] = v
	}
	for _, c := range computed {
		want, ok := byName[c.Name]
		if !ok {
			return fmt.Errorf("%s missing from manifest", c.Name)
		}
		switch {
		case want.Deployed != c.Deployed:
			return fmt.Errorf("%s deployed baseline in manifest != code (the baseline is immutable; "+
				"do not edit it by hand)", c.Name)
		case want.Current != c.Current:
			return fmt.Errorf("%s current circuit changed: manifest %+v, computed %+v", c.Name, want.Current, c.Current)
		case want.SourceSHA256 != c.SourceSHA256:
			return fmt.Errorf("%s verifier source changed: manifest %s, computed %s", c.Name, want.SourceSHA256, c.SourceSHA256)
		case want.InSync != c.InSync:
			return fmt.Errorf("%s inSyncWithDeployment stale: manifest %v, computed %v", c.Name, want.InSync, c.InSync)
		}
	}
	return nil
}
