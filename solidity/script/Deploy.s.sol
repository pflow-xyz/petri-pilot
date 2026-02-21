// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Script.sol";
import {Verifier} from "../src/Groth16Verifier.sol";
import {Groth16VerifierAdapter} from "../src/Groth16VerifierAdapter.sol";
import {ZkOde} from "../src/ZkOde.sol";

/// @title Deploy
/// @notice Deploys Groth16Verifier, Groth16VerifierAdapter, and ZkOde to Base L2.
/// @dev Usage:
///   forge script script/Deploy.s.sol --rpc-url base_sepolia --broadcast --verify
///
///   Environment variables:
///     PRIVATE_KEY       - Deployer private key
///     GENESIS_ROOT      - MiMC hash of initial marking (default: 0)
///     NUM_TRANSITIONS   - Number of transitions in the net (default: 2)
///     ENFORCE_OPTIMAL   - Require optimal play (default: true)
///     NUM_PUBLIC_INPUTS - Number of public circuit inputs (default: 5)
contract Deploy is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        uint256 genesisRoot = vm.envOr("GENESIS_ROOT", uint256(0));
        uint256 numTransitions = vm.envOr("NUM_TRANSITIONS", uint256(2));
        bool enforceOptimal = vm.envOr("ENFORCE_OPTIMAL", true);
        uint256 numPublicInputs = vm.envOr("NUM_PUBLIC_INPUTS", uint256(5));

        // Selector for gnark's verifyProof(uint256[8],uint256[5])
        bytes4 verifySelector = bytes4(keccak256("verifyProof(uint256[8],uint256[5])"));

        vm.startBroadcast(deployerKey);

        // 1. Deploy gnark-generated Groth16 verifier (BN254 pairing)
        Verifier groth16 = new Verifier();
        console.log("Groth16Verifier deployed at:", address(groth16));

        // 2. Deploy adapter: wraps gnark's fixed-size interface for IVerifier
        Groth16VerifierAdapter adapter = new Groth16VerifierAdapter(
            address(groth16),
            verifySelector,
            numPublicInputs
        );
        console.log("Groth16VerifierAdapter deployed at:", address(adapter));

        // 3. Deploy ZkOde state manager with genesis root
        ZkOde zkOde = new ZkOde(address(adapter), genesisRoot, numTransitions, enforceOptimal);
        console.log("ZkOde deployed at:", address(zkOde));
        console.log("Genesis root:", genesisRoot);
        console.log("Transitions:", numTransitions);
        console.log("Enforce optimal:", enforceOptimal);
        console.log("Public inputs:", numPublicInputs);

        vm.stopBroadcast();
    }
}
