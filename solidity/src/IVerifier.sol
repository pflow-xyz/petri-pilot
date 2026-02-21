// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IVerifier
/// @author pflow-xyz (https://github.com/pflow-xyz/petri-pilot/tree/main/solidity)
/// @notice Standard interface for Groth16 proof verification (BN254).
/// @dev Generated verifiers from gnark's ExportVerifier implement this interface.
interface IVerifier {
    function verifyProof(
        uint256[2] calldata a,
        uint256[2][2] calldata b,
        uint256[2] calldata c,
        uint256[] calldata publicInputs
    ) external view returns (bool);
}
