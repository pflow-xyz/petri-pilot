// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IVerifier} from "./IVerifier.sol";

/// @title Groth16VerifierAdapter
/// @notice Adapts a gnark-generated Groth16 verifier to the IVerifier interface.
/// @dev gnark's ExportSolidity produces a contract with fixed-size public input arrays:
///        verifyProof(uint256[8] calldata proof, uint256[N] calldata input)
///      This adapter accepts the structured IVerifier interface and re-encodes the
///      calldata as flat fixed-size arrays via a low-level staticcall.
///
///      The gnark verifier reverts on invalid proofs; this adapter catches the
///      revert and returns false for IVerifier compatibility.
contract Groth16VerifierAdapter is IVerifier {
    address public immutable groth16Verifier;
    bytes4 public immutable groth16Selector;
    uint256 public immutable numPublicInputs;

    error InputLengthMismatch(uint256 expected, uint256 got);

    constructor(address _verifier, bytes4 _selector, uint256 _numPublicInputs) {
        require(_verifier != address(0), "zero verifier");
        require(_numPublicInputs > 0, "zero inputs");
        groth16Verifier = _verifier;
        groth16Selector = _selector;
        numPublicInputs = _numPublicInputs;
    }

    /// @notice Verify a Groth16 proof by forwarding to the gnark verifier.
    /// @param a G1 point A [A.X, A.Y].
    /// @param b G2 point B [[B.X[0], B.X[1]], [B.Y[0], B.Y[1]]].
    /// @param c G1 point C [C.X, C.Y].
    /// @param publicInputs Dynamic array of public circuit inputs.
    /// @return True if the proof is valid.
    function verifyProof(
        uint256[2] calldata a,
        uint256[2][2] calldata b,
        uint256[2] calldata c,
        uint256[] calldata publicInputs
    ) external view override returns (bool) {
        if (publicInputs.length != numPublicInputs) {
            revert InputLengthMismatch(numPublicInputs, publicInputs.length);
        }

        address target = groth16Verifier;
        bytes4 sel = groth16Selector;
        uint256 n = numPublicInputs;

        bool success;
        assembly ("memory-safe") {
            let ptr := mload(0x40)

            // Write gnark function selector (4 bytes, left-aligned in 32-byte word)
            mstore(ptr, sel)

            // Build flat uint256[8] proof from structured (a, b, c) components.
            // In calldata, fixed-size arrays are stored contiguously:
            //   a: a[0], a[1]                         (2 words, 64 bytes)
            //   b: b[0][0], b[0][1], b[1][0], b[1][1] (4 words, 128 bytes)
            //   c: c[0], c[1]                         (2 words, 64 bytes)
            // gnark expects uint256[8] = proof[0..7] packed sequentially.
            calldatacopy(add(ptr, 4), a, 64)       // a → proof[0..1]
            calldatacopy(add(ptr, 68), b, 128)     // b → proof[2..5]
            calldatacopy(add(ptr, 196), c, 64)     // c → proof[6..7]

            // Copy public inputs: n * 32 bytes
            // publicInputs.offset points to the first element in calldata
            calldatacopy(add(ptr, 260), publicInputs.offset, mul(n, 32))

            // Call gnark verifier via staticcall
            // On success: returns empty (gnark verifier doesn't return data, just doesn't revert)
            // On failure: reverts with ProofInvalid()
            success := staticcall(gas(), target, ptr, add(260, mul(n, 32)), 0, 0)
        }
        return success;
    }
}
