// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IVerifier} from "./IVerifier.sol";

/// @title ZkOdeVerifier
/// @notice Stub Groth16 verifier for ZK-ODE Tsit5 step circuit.
/// @dev Replace with Groth16VerifierAdapter wrapping the gnark-generated verifier.
contract ZkOdeVerifier is IVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[] calldata
    ) external pure override returns (bool) {
        // Stub: always returns true.
        return true;
    }
}
