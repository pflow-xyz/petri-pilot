#!/usr/bin/env bash
# zk-onchain-bytecode-check.sh — the on-chain leg of F4 Tier 3.
#
# zk-verifier-provenance binds the committed Solidity verifier to its circuit
# (hermetic, runs in CI). This binds the committed Solidity to what's actually
# DEPLOYED: it compiles each verifier locally with forge and compares the result
# to the runtime bytecode at the deployed address (eth_getCode). Together the two
# close the chain circuit -> verifier -> deployment.
#
# Requires `forge`/`cast` (foundry) and an RPC for the target chain. It is NOT a
# CI gate (CI has neither), so it SKIPs cleanly when they're absent — run it
# manually from a machine that has them:
#
#   RPC_URL=https://sepolia.base.org scripts/zk-onchain-bytecode-check.sh
#
# Bytecode equality holds only when the local compile matches the deployment's
# solc version + optimizer settings; the trailing CBOR metadata hash is stripped
# before comparing so a metadata-only difference doesn't read as a mismatch.
set -euo pipefail

cd "$(dirname "$0")/.."
manifest="zk-ode/provenance.json"

if ! command -v forge >/dev/null 2>&1 || ! command -v cast >/dev/null 2>&1; then
	echo "SKIP: foundry (forge/cast) not installed — on-chain check not run."
	exit 0
fi
if [ -z "${RPC_URL:-}" ]; then
	echo "SKIP: set RPC_URL to the target chain's endpoint to run the on-chain check."
	exit 0
fi

# Strip the Solidity CBOR metadata trailer (a2 64 'ipfs'… / a1 65 'bzzr0'…) so
# only executable bytecode is compared.
strip_metadata() {
	sed -E 's/(a264697066[0-9a-f]*|a165627a7a72[0-9a-f]*)$//I'
}

( cd solidity && forge build >/dev/null )

rc=0
# name, source, address parsed out of the manifest with jq.
while IFS=$'\t' read -r name source address; do
	[ -n "$address" ] || continue
	contract="$(basename "$source" .sol)"
	local_bc="$(cd solidity && forge inspect "$contract" deployedBytecode 2>/dev/null | tr -d '\n')"
	chain_bc="$(cast code "$address" --rpc-url "$RPC_URL" 2>/dev/null | tr -d '\n')"
	if [ -z "$local_bc" ] || [ -z "$chain_bc" ]; then
		echo "ERROR  $name: could not obtain bytecode (contract=$contract addr=$address)"
		rc=1; continue
	fi
	l="$(printf '%s' "${local_bc#0x}" | strip_metadata)"
	c="$(printf '%s' "${chain_bc#0x}" | strip_metadata)"
	if [ "$l" = "$c" ]; then
		echo "OK     $name: deployed bytecode matches committed $contract.sol ($address)"
	else
		echo "FAIL   $name: deployed bytecode != committed $contract.sol ($address)"
		rc=1
	fi
done < <(jq -r '.verifiers[] | select(.deployment.address) | [.name, .verifierSource, .deployment.address] | @tsv' "$manifest")

[ "$rc" -eq 0 ] && echo "PASS: all deployed verifiers match their committed Solidity"
exit "$rc"
