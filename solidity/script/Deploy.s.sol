// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Script.sol";
import {ZkOdeVerifier} from "../src/ZkOdeVerifier.sol";
import {ZkOde} from "../src/ZkOde.sol";

/// @title Deploy
/// @notice Deploys ZkOdeVerifier and ZkOde to Base L2.
/// @dev Usage:
///   forge script script/Deploy.s.sol --rpc-url base_sepolia --broadcast --verify
///
///   Environment variables:
///     PRIVATE_KEY       - Deployer private key
///     GENESIS_ROOT      - MiMC hash of initial marking (default: 0)
///     NUM_TRANSITIONS   - Number of transitions in the net (default: 2)
///     ENFORCE_OPTIMAL   - Require optimal play (default: false)
contract Deploy is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        uint256 genesisRoot = vm.envOr("GENESIS_ROOT", uint256(0));
        uint256 numTransitions = vm.envOr("NUM_TRANSITIONS", uint256(2));
        bool enforceOptimal = vm.envOr("ENFORCE_OPTIMAL", false);

        vm.startBroadcast(deployerKey);

        // Deploy verifier (replace with gnark-generated verifier for production)
        ZkOdeVerifier verifier = new ZkOdeVerifier();
        console.log("ZkOdeVerifier deployed at:", address(verifier));

        // Deploy state manager with genesis root
        ZkOde zkOde = new ZkOde(address(verifier), genesisRoot, numTransitions, enforceOptimal);
        console.log("ZkOde deployed at:", address(zkOde));
        console.log("Genesis root:", genesisRoot);
        console.log("Transitions:", numTransitions);
        console.log("Enforce optimal:", enforceOptimal);

        vm.stopBroadcast();
    }
}
