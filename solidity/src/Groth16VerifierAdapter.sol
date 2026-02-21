// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IVerifier} from "./IVerifier.sol";

/// @title Groth16VerifierAdapter
/// @notice Adapts a gnark-generated Groth16 verifier to the IVerifier interface.
/// @dev gnark's ExportSolidity produces a contract with fixed-size public input arrays:
///        verifyProof(uint256[8] calldata proof, uint256[N] calldata input)
///      This adapter accepts dynamic-length inputs and re-encodes the calldata
///      as fixed-size arrays via a low-level staticcall.
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
    /// @param proof Flat proof array [A.X, A.Y, B.X[0], B.X[1], B.Y[0], B.Y[1], C.X, C.Y].
    /// @param publicInputs Dynamic array of public circuit inputs.
    /// @return True if the proof is valid.
    function verifyProof(
        uint256[8] calldata proof,
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

            // Copy proof: 8 * 32 = 256 bytes from calldata
            // gnark expects uint256[8] calldata — same encoding as our uint256[8] calldata
            calldatacopy(add(ptr, 4), proof, 256)

            // Copy public inputs: n * 32 bytes from calldata
            // gnark expects uint256[N] calldata (fixed, packed without offset/length)
            // Our calldata has them as uint256[] (dynamic, with offset+length stripped by Solidity)
            // publicInputs.offset points to the first element's calldata position
            calldatacopy(add(ptr, 260), publicInputs.offset, mul(n, 32))

            // Call gnark verifier via staticcall
            // On success: returns empty (no return data)
            // On failure: reverts with ProofInvalid()
            success := staticcall(gas(), target, ptr, add(260, mul(n, 32)), 0, 0)
        }
        return success;
    }
}
