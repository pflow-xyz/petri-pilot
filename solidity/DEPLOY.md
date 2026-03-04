# ZkOde Deployment Guide

## Architecture

```
ZkOde ──IVerifier──> Groth16VerifierAdapter ──staticcall──> Groth16Verifier (gnark BN254)
```

Three contracts deploy in sequence:

1. **Groth16Verifier** — gnark-generated BN254 pairing verifier with embedded verification keys. Function: `verifyProof(uint256[8] proof, uint256[5] input)`.
2. **Groth16VerifierAdapter** — Translates IVerifier's structured format `(uint256[2] a, uint256[2][2] b, uint256[2] c, uint256[] inputs)` to gnark's flat format `(uint256[8] proof, uint256[N] input)` via low-level `staticcall`.
3. **ZkOde** — State commitment manager. Chains state roots, enforces optimal play, forwards proofs through IVerifier.

## Prerequisites

```bash
# Install Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Install dependencies (from solidity/ directory)
forge install
```

### Environment Variables

Add to `~/.zshrc`:

```bash
export BASESCAN_API_KEY="<your-basescan-api-key>"
export BASE_SEPOLIA_RPC_URL="https://sepolia.base.org"
export DEPLOYER_PRIVATE_KEY="<your-deployer-private-key>"
```

### Generate a Deployer Wallet

```bash
cast wallet new
```

Fund the resulting address with Base Sepolia ETH from:
- https://portal.cdp.coinbase.com/products/faucet (select "Base Sepolia")
- https://www.alchemy.com/faucets/base-sepolia

Deployment costs ~0.00006 ETH on Base Sepolia.

## Genesis Root

The genesis root must match the MiMC hash of the initial marking the prover will use. For the A-B-C cascade with initial marking `[1, 0, 0]`:

```bash
# Get the genesis root from the prover
curl -s -X POST https://pilot.pflow.xyz/zk-ode/api/prove \
  -H "Content-Type: application/json" \
  -d '{"step_size":0.01,"rates":[1.0,1.0],"initial_marking":[1,0,0],"num_steps":1}' \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['steps'][0]['proof']['public_inputs'][0])"
```

Convert the hex to decimal for the deploy script:

```bash
cast to-dec 0x2cc32c87522be4b588f26301aef43e600ea46d912b6d781416c83074185892aa
# → 20246607840381229495038349234010146306150575613821200620564900551378416997034
```

**If genesis root is wrong**, `submitStep` will revert with `InvalidStateChain(expected, got)`.

## Deploy

```bash
cd solidity

PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY \
GENESIS_ROOT=20246607840381229495038349234010146306150575613821200620564900551378416997034 \
NUM_TRANSITIONS=2 \
ENFORCE_OPTIMAL=true \
BASESCAN_API_KEY=$BASESCAN_API_KEY \
forge script script/Deploy.s.sol \
  --rpc-url $BASE_SEPOLIA_RPC_URL \
  --broadcast \
  --verify
```

### Deploy Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `GENESIS_ROOT` | 0 | MiMC hash of initial marking |
| `NUM_TRANSITIONS` | 2 | Number of transitions in the Petri net |
| `ENFORCE_OPTIMAL` | true | Require highest-rate transition selection |
| `NUM_PUBLIC_INPUTS` | 5 | Circuit public inputs (preRoot, postRoot, stepSize, rate0, rate1) |

## Verify Contracts Manually

If verification fails during deploy (e.g., missing API key in env), verify afterward:

```bash
# Verifier (no constructor args)
BASESCAN_API_KEY=$BASESCAN_API_KEY \
forge verify-contract <VERIFIER_ADDR> src/Groth16Verifier.sol:Verifier \
  --chain base-sepolia --watch

# Adapter
BASESCAN_API_KEY=$BASESCAN_API_KEY \
forge verify-contract <ADAPTER_ADDR> src/Groth16VerifierAdapter.sol:Groth16VerifierAdapter \
  --chain base-sepolia --watch \
  --constructor-args $(cast abi-encode "constructor(address,bytes4,uint256)" \
    <VERIFIER_ADDR> \
    $(cast keccak "verifyProof(uint256[8],uint256[5])" | cut -c1-10) \
    5)

# ZkOde
BASESCAN_API_KEY=$BASESCAN_API_KEY \
forge verify-contract <ZKODE_ADDR> src/ZkOde.sol:ZkOde \
  --chain base-sepolia --watch \
  --constructor-args $(cast abi-encode "constructor(address,uint256,uint256,bool)" \
    <ADAPTER_ADDR> <GENESIS_ROOT_DECIMAL> 2 true)
```

## Query Deployed State

```bash
RPC=https://sepolia.base.org
ZKODE=<zkode-address>

cast call $ZKODE "currentStateRoot()" --rpc-url $RPC
cast call $ZKODE "enforceOptimal()" --rpc-url $RPC
cast call $ZKODE "numTransitions()" --rpc-url $RPC
cast call $ZKODE "stepCount()" --rpc-url $RPC
cast call $ZKODE "prover()" --rpc-url $RPC
```

## Submit a Proof

```bash
# Get proof from prover service
PROOF_JSON=$(curl -s -X POST https://pilot.pflow.xyz/zk-ode/api/prove \
  -H "Content-Type: application/json" \
  -d '{"step_size":0.01,"rates":[1.0,1.0],"initial_marking":[1,0,0],"num_steps":1}')

# Extract raw_proof and public_inputs, then:
cast send $ZKODE \
  "submitStep(uint256[8],uint256[],uint256)" \
  "[<8 proof values>]" \
  "[<5 public input values>]" \
  0 \
  --rpc-url $RPC \
  --private-key $DEPLOYER_PRIVATE_KEY
```

## Current Deployments

### Base Sepolia (2026-02-21)

| Contract | Address |
|----------|---------|
| Groth16Verifier | `0x97e0449de6b142f5bfafedd7ab3673b13efafd54` |
| Groth16VerifierAdapter | `0x0cbcdc06fda126d6968d7d6adb6a1bfd393113c6` |
| ZkOde | `0xdf7f1559061297f65b491126103d56fd4019c3cb` |

- **Network:** Base Sepolia (chain 84532)
- **Deployer/Prover:** `0x762593292f543948CA9A9a290adC1770746d059a`
- **Genesis root:** MiMC([1, 0, 0]) = `0x2cc32c...92aa`
- **Config:** 2 transitions, enforceOptimal=true, 5 public inputs

## Known Issues

### Prover proof serialization bug (FIXED)

The gnark prover in `go-pflow/prover/prover.go` was using `proof.WriteTo()` (compressed, 128 bytes) which drops Y-coordinates. Fixed to use `proof.WriteRawTo()` (uncompressed, 256 bytes). Awaiting go-pflow release + petri-pilot dependency update.

### Key persistence (OPEN — [#4](https://github.com/pflow-xyz/petri-pilot/issues/4))

The prover runs `groth16.Setup(cs)` on every startup, generating fresh proving/verifying keys. The deployed `Groth16Verifier.sol` has keys embedded from the original setup. Proofs generated with new keys won't verify on-chain. Fix: persist (pk, vk) to disk after first setup and reuse on subsequent starts, then redeploy the verifier with matching keys.

### Circuit constraint

The gnark circuit (`zk-ode/circuits.go`) is hard-coded for `NumPlaces=3, NumTransitions=2` (A-B-C linear cascade). A tic-tac-toe circuit with 35 transitions would require new topology constants, a circuit recompile, and a new Groth16Verifier with updated verification keys.

## Tests

```bash
cd solidity
forge test -vvv
```

17 tests across 3 suites:
- **ZkOdeTest** (10) — Basic functionality with stub verifier
- **ZkOdeOptimalTest** (4) — Optimal play enforcement
- **ZkOdeAdapterTest** (3) — Real Groth16 verifier + adapter integration
