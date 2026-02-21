// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Script.sol";
import {ZkOdeVerifier} from "../src/ZkOdeVerifier.sol";
import {Groth16VerifierAdapter} from "../src/Groth16VerifierAdapter.sol";
import {ZkOde} from "../src/ZkOde.sol";

/// @title Deploy
/// @notice Deploys ZkOde with either the stub or real gnark verifier.
/// @dev Usage (stub, for testing):
///   forge script script/Deploy.s.sol --rpc-url base_sepolia --broadcast --verify
///
///   Usage (real verifier):
///   Set GROTH16_VERIFIER to the address of a deployed Groth16Verifier contract.
///   The adapter will wrap it with the correct selector and input count.
///
///   Environment variables:
///     PRIVATE_KEY         - Deployer private key
///     GENESIS_ROOT        - MiMC hash of initial marking (default: 0)
///     NUM_TRANSITIONS     - Number of transitions in the net (default: 2)
///     ENFORCE_OPTIMAL     - Require optimal play (default: false)
///     GROTH16_VERIFIER    - Address of deployed gnark verifier (optional)
///     GROTH16_SELECTOR    - 4-byte selector for gnark verifyProof (optional)
contract Deploy is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        uint256 genesisRoot = vm.envOr("GENESIS_ROOT", uint256(0));
        uint256 numTransitions = vm.envOr("NUM_TRANSITIONS", uint256(2));
        bool enforceOptimal = vm.envOr("ENFORCE_OPTIMAL", false);

        vm.startBroadcast(deployerKey);

        address verifierAddr;

        // Check if a real gnark verifier address is provided
        address groth16Addr = vm.envOr("GROTH16_VERIFIER", address(0));
        if (groth16Addr != address(0)) {
            // Deploy adapter wrapping the real gnark verifier
            bytes4 selector = bytes4(vm.envOr("GROTH16_SELECTOR", bytes32(0)));
            require(selector != bytes4(0), "GROTH16_SELECTOR required with GROTH16_VERIFIER");

            uint256 numPublicInputs = 3 + numTransitions; // preRoot, postRoot, stepSize, rates...
            Groth16VerifierAdapter adapter = new Groth16VerifierAdapter(
                groth16Addr, selector, numPublicInputs
            );
            verifierAddr = address(adapter);
            console.log("Groth16VerifierAdapter deployed at:", verifierAddr);
            console.log("  wrapping gnark verifier at:", groth16Addr);
        } else {
            // Deploy stub verifier (for testing)
            ZkOdeVerifier stub = new ZkOdeVerifier();
            verifierAddr = address(stub);
            console.log("ZkOdeVerifier (stub) deployed at:", verifierAddr);
        }

        // Deploy state manager with genesis root
        ZkOde zkOde = new ZkOde(verifierAddr, genesisRoot, numTransitions, enforceOptimal);
        console.log("ZkOde deployed at:", address(zkOde));
        console.log("Genesis root:", genesisRoot);
        console.log("Transitions:", numTransitions);
        console.log("Enforce optimal:", enforceOptimal);

        vm.stopBroadcast();
    }
}
