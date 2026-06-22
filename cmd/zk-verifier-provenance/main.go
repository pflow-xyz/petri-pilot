// Command zk-verifier-provenance binds each deployed Groth16 verifier to the
// circuit it was generated from (F4 Tier 3).
//
// The breach this closes: the on-chain verifiers (CLAUDE.md "ZkOde Contracts")
// are exported from a gnark circuit + a one-time trusted setup. Nothing checked
// that the committed Solidity verifier still corresponds to the committed
// circuit — a circuit edit that wasn't re-exported/redeployed would silently
// leave the chain verifying the wrong relation.
//
// This makes that falsifiable. For each (circuit, verifier) pair it:
//   - compiles the circuit hermetically (R1CS — reproducible since F4 Tier 2),
//     recording its public-input count, constraint count, and R1CS digest;
//   - parses the committed Solidity verifier's hardcoded public-input arity;
//   - asserts the verifier arity == circuit public inputs (the verifier's
//     `input[]` excludes gnark's implicit ONE wire, so arity == nbPublic - 1);
//   - hashes the verifier source.
//
// The results are checked against (or written to) provenance.json. Any circuit
// change flips the digest/counts and fails the check until the manifest — and,
// by implication, the deployed verifier — is reconciled.
//
// What this does NOT do: prove the *deployed bytecode* equals the committed
// Solidity. That leg needs `forge` + an RPC and is recorded per-verifier as
// (address, chainId); verify it with scripts/zk-onchain-bytecode-check.sh in an
// environment that has both. The verifying key itself is baked into the
// committed verifier as constants, so the committed .sol is the vk of record.
//
// Usage:
//
//	go run ./cmd/zk-verifier-provenance            # check against provenance.json
//	go run ./cmd/zk-verifier-provenance -write     # regenerate provenance.json
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

// manifestPath is relative to the repo root (the command is run from there).
const manifestPath = "zk-ode/provenance.json"

// verifier pairs a deployed verifier with the circuit it must correspond to.
type verifier struct {
	Name           string           `json:"name"`
	Circuit        string           `json:"circuit"`
	VerifierSource string           `json:"verifierSource"`
	PublicInputs   int              `json:"publicInputs"`
	Constraints    int              `json:"constraints"`
	R1CSSHA256     string           `json:"r1csSHA256"`
	SourceSHA256   string           `json:"sourceSHA256"`
	Deployment     map[string]any   `json:"deployment"`
	circuit        frontend.Circuit `json:"-"`
}

// pairs is the source of truth for what's deployed. Add an entry to track a new
// verifier. Deployment metadata mirrors CLAUDE.md "ZkOde Contracts".
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
	},
}

// inputArityRe matches gnark's exported verifier public-input array, e.g.
// `uint256[12] calldata input`.
var inputArityRe = regexp.MustCompile(`uint256\[(\d+)\] calldata input`)

func main() {
	write := flag.Bool("write", false, "regenerate the provenance manifest instead of checking it")
	flag.Parse()
	logger.Disable()

	computed := make([]verifier, 0, len(pairs))
	for i := range pairs {
		v, err := compute(pairs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", pairs[i].Name, err)
			os.Exit(2)
		}
		fmt.Printf("%-12s circuit=%-26s public=%2d constraints=%-7d r1cs=%s… verifier=%s[%d]\n",
			v.Name, v.Circuit, v.PublicInputs, v.Constraints, v.R1CSSHA256[:12],
			filepath.Base(v.VerifierSource), v.PublicInputs)
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
		fmt.Fprintf(os.Stderr, "\nPROVENANCE MISMATCH: %v\n", err)
		fmt.Fprintln(os.Stderr, "the committed verifier no longer matches its circuit; "+
			"re-export/redeploy the verifier, then `go run ./cmd/zk-verifier-provenance -write`.")
		os.Exit(1)
	}
	fmt.Println("\nPASS: every committed verifier matches its circuit's hermetic R1CS")
}

// compute compiles the circuit and parses the verifier, filling the derived
// fields of v.
func compute(v verifier) (verifier, error) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, v.circuit)
	if err != nil {
		return v, fmt.Errorf("compile: %w", err)
	}
	// gnark counts the implicit ONE wire among public variables; the Solidity
	// verifier's input[] excludes it.
	v.PublicInputs = ccs.GetNbPublicVariables() - 1
	v.Constraints = ccs.GetNbConstraints()

	h := sha256.New()
	if _, err := ccs.WriteTo(h); err != nil {
		return v, fmt.Errorf("r1cs digest: %w", err)
	}
	v.R1CSSHA256 = hex.EncodeToString(h.Sum(nil))

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
	if arity != v.PublicInputs {
		return v, fmt.Errorf("verifier hardcodes %d public inputs but circuit has %d "+
			"(verifier is stale relative to the circuit)", arity, v.PublicInputs)
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
		"_comment": "F4 Tier 3 verifier provenance — binds each deployed Groth16 verifier to " +
			"its circuit's hermetic R1CS. Regenerate with `go run ./cmd/zk-verifier-provenance -write` " +
			"after re-exporting a verifier. CI runs the check (no -write).",
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
	byName := map[string]verifier{}
	for _, v := range m.Verifiers {
		byName[v.Name] = v
	}
	if len(m.Verifiers) != len(computed) {
		return fmt.Errorf("manifest lists %d verifiers, code tracks %d", len(m.Verifiers), len(computed))
	}
	for _, c := range computed {
		want, ok := byName[c.Name]
		if !ok {
			return fmt.Errorf("%s missing from manifest", c.Name)
		}
		switch {
		case want.PublicInputs != c.PublicInputs:
			return fmt.Errorf("%s public inputs: manifest %d, computed %d", c.Name, want.PublicInputs, c.PublicInputs)
		case want.Constraints != c.Constraints:
			return fmt.Errorf("%s constraints: manifest %d, computed %d", c.Name, want.Constraints, c.Constraints)
		case want.R1CSSHA256 != c.R1CSSHA256:
			return fmt.Errorf("%s R1CS digest changed (circuit edited): manifest %s, computed %s", c.Name, want.R1CSSHA256, c.R1CSSHA256)
		case want.SourceSHA256 != c.SourceSHA256:
			return fmt.Errorf("%s verifier source changed: manifest %s, computed %s", c.Name, want.SourceSHA256, c.SourceSHA256)
		}
	}
	return nil
}
