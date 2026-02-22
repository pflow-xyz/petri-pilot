// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Script.sol";
import {TTTVerifier} from "../src/TTTGroth16Verifier.sol";
import {Groth16VerifierAdapter} from "../src/Groth16VerifierAdapter.sol";
import {ZkOde} from "../src/ZkOde.sol";

/// @title DeployTTT
/// @notice Deploys TTT Groth16Verifier, Adapter (37 inputs), and ZkOde (35 transitions).
/// @dev Usage:
///   GENESIS_ROOT=0x133e015bd26233707d7a1778a30a0f8de5e0b684c8e88705d770f1ba5cb3d27c \
///   NUM_TRANSITIONS=34 NUM_PUBLIC_INPUTS=37 \
///   forge script script/DeployTTT.s.sol --rpc-url $BASE_SEPOLIA_RPC_URL --broadcast --verify
contract DeployTTT is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");

        // TTT genesis root = MiMC(empty board, X turn, game_active)
        uint256 genesisRoot = vm.envOr(
            "GENESIS_ROOT",
            uint256(0x133e015bd26233707d7a1778a30a0f8de5e0b684c8e88705d770f1ba5cb3d27c)
        );
        uint256 numTransitions = vm.envOr("NUM_TRANSITIONS", uint256(34));
        bool enforceOptimal = vm.envOr("ENFORCE_OPTIMAL", true);
        uint256 numPublicInputs = vm.envOr("NUM_PUBLIC_INPUTS", uint256(37));

        // Selector for gnark's verifyProof(uint256[8],uint256[37])
        bytes4 verifySelector = bytes4(keccak256("verifyProof(uint256[8],uint256[37])"));

        vm.startBroadcast(deployerKey);

        // 1. Deploy gnark-generated TTT Groth16 verifier (BN254 pairing, 37 inputs)
        TTTVerifier groth16 = new TTTVerifier();
        console.log("TTTGroth16Verifier deployed at:", address(groth16));

        // 2. Deploy adapter: wraps gnark's fixed-size interface for IVerifier
        Groth16VerifierAdapter adapter = new Groth16VerifierAdapter(
            address(groth16),
            verifySelector,
            numPublicInputs
        );
        console.log("Groth16VerifierAdapter deployed at:", address(adapter));

        // 3. Deploy ZkOde state manager with TTT genesis root
        ZkOde zkOde = new ZkOde(address(adapter), genesisRoot, numTransitions, enforceOptimal);
        console.log("ZkOde (TTT) deployed at:", address(zkOde));
        console.log("Genesis root:", genesisRoot);
        console.log("Transitions:", numTransitions);
        console.log("Enforce optimal:", enforceOptimal);
        console.log("Public inputs:", numPublicInputs);

        vm.stopBroadcast();
    }
}
