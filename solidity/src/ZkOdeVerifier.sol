// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IVerifier} from "./IVerifier.sol";

/// @title ZkOdeVerifier
/// @author pflow-xyz (https://github.com/pflow-xyz/petri-pilot/tree/main/solidity)
/// @notice Stub Groth16 verifier for ZK-ODE Tsit5 step circuit.
/// @dev Replace with the gnark-generated verifier contract from:
///      prover.ExportVerifier("tsit5_step")
///
///      The generated verifier uses the BN254 ecPairing precompile (EIP-196/197)
///      which is available on Base L2.
contract ZkOdeVerifier is IVerifier {
    function verifyProof(
        uint256[2] calldata,
        uint256[2][2] calldata,
        uint256[2] calldata,
        uint256[] calldata
    ) external pure override returns (bool) {
        // Stub: always returns true.
        // Replace this contract with the gnark-generated Groth16 verifier.
        return true;
    }
}
