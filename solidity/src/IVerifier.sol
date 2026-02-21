// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IVerifier
/// @notice Standard interface for Groth16 proof verification (BN254).
/// @dev Proof is passed as a flat uint256[8] array:
///      [A.X, A.Y, B.X[0], B.X[1], B.Y[0], B.Y[1], C.X, C.Y]
interface IVerifier {
    function verifyProof(
        uint256[8] calldata proof,
        uint256[] calldata publicInputs
    ) external view returns (bool);
}
