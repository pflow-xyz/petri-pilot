// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Script.sol";
import {HeatmapVerifier} from "../src/TTTHeatmapVerifier.sol";
import {Groth16VerifierAdapter} from "../src/Groth16VerifierAdapter.sol";
import {ZkOde} from "../src/ZkOde.sol";

/// @title DeployHeatmap
/// @notice Deploys TTT HeatmapVerifier, Adapter (12 inputs), and ZkOde (9 transitions).
/// @dev The heatmap circuit uses 12 public inputs: PreStateRoot, PostStateRoot, StepSize, HeatmapScores[9].
///      numTransitions=9 means the contract enforces optimal play over cell positions (0-8).
///   Usage:
///   PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY \
///   forge script script/DeployHeatmap.s.sol --rpc-url $BASE_SEPOLIA_RPC_URL --broadcast --verify
contract DeployHeatmap is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");

        // Genesis root = MiMC(empty board, X turn, game_active) — same as before
        uint256 genesisRoot = vm.envOr(
            "GENESIS_ROOT",
            uint256(0x133e015bd26233707d7a1778a30a0f8de5e0b684c8e88705d770f1ba5cb3d27c)
        );
        uint256 numTransitions = vm.envOr("NUM_TRANSITIONS", uint256(9));
        bool enforceOptimal = vm.envOr("ENFORCE_OPTIMAL", true);
        uint256 numPublicInputs = vm.envOr("NUM_PUBLIC_INPUTS", uint256(12));

        // Selector for gnark's verifyProof(uint256[8],uint256[12])
        bytes4 verifySelector = bytes4(keccak256("verifyProof(uint256[8],uint256[12])"));

        vm.startBroadcast(deployerKey);

        // 1. Deploy gnark-generated Heatmap Groth16 verifier (BN254, 12 inputs)
        HeatmapVerifier groth16 = new HeatmapVerifier();
        console.log("HeatmapVerifier deployed at:", address(groth16));

        // 2. Deploy adapter: wraps gnark's fixed-size interface for IVerifier
        Groth16VerifierAdapter adapter = new Groth16VerifierAdapter(
            address(groth16),
            verifySelector,
            numPublicInputs
        );
        console.log("Groth16VerifierAdapter deployed at:", address(adapter));

        // 3. Deploy ZkOde state manager
        ZkOde zkOde = new ZkOde(address(adapter), genesisRoot, numTransitions, enforceOptimal);
        console.log("ZkOde (Heatmap) deployed at:", address(zkOde));
        console.log("Genesis root:", genesisRoot);
        console.log("Transitions:", numTransitions);
        console.log("Enforce optimal:", enforceOptimal);
        console.log("Public inputs:", numPublicInputs);

        vm.stopBroadcast();
    }
}
