// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IVerifier} from "./IVerifier.sol";

/// @title ZkOde
/// @notice State commitment manager for ZK-proven ODE steps on a Petri net.
/// @dev Accepts Groth16 proofs of Tsit5 ODE integration steps and maintains
///      a chain of state root commitments. Each step proves that the prover
///      correctly computed one integration step over the hidden marking.
///
///      Public inputs layout (6 values):
///        [0] PreStateRoot   - MiMC hash of marking before step
///        [1] PostStateRoot  - MiMC hash of marking after step
///        [2] StepSize       - Fixed-point step size (h * 10^18)
///        [3] Rate0          - Rate constant for transition 0
///        [4] Rate1          - Rate constant for transition 1
///        [5] (reserved)     - Unused, set to 0
contract ZkOde {
    // --- State ---
    IVerifier public verifier;
    address public prover;
    uint256 public currentStateRoot;
    uint256 public stepCount;

    struct Step {
        uint256 preRoot;
        uint256 postRoot;
        uint256 stepSize;
        uint256 timestamp;
    }

    mapping(uint256 => Step) public steps;

    // --- Events ---
    event StepVerified(
        uint256 indexed stepNumber,
        uint256 preRoot,
        uint256 postRoot,
        uint256 stepSize,
        uint256 timestamp
    );

    event ProverUpdated(address indexed oldProver, address indexed newProver);
    event VerifierUpdated(address indexed oldVerifier, address indexed newVerifier);

    // --- Errors ---
    error InvalidProof();
    error InvalidStateChain(uint256 expected, uint256 got);
    error OnlyProver();

    // --- Modifiers ---
    modifier onlyProver() {
        if (msg.sender != prover) revert OnlyProver();
        _;
    }

    // --- Constructor ---
    constructor(address _verifier, uint256 _genesisRoot) {
        verifier = IVerifier(_verifier);
        prover = msg.sender;
        currentStateRoot = _genesisRoot;
    }

    // --- Core ---

    /// @notice Submit a single ODE step proof.
    /// @param proof Groth16 proof components [a, b, c].
    /// @param publicInputs Array of public circuit inputs.
    function submitStep(
        uint256[8] calldata proof,
        uint256[] calldata publicInputs
    ) external onlyProver {
        _verifyAndRecord(proof, publicInputs);
    }

    /// @notice Submit a batch of sequential ODE step proofs in one transaction.
    /// @param proofs Array of Groth16 proofs.
    /// @param publicInputsBatch Array of public input arrays (one per proof).
    function submitBatchSteps(
        uint256[8][] calldata proofs,
        uint256[][] calldata publicInputsBatch
    ) external onlyProver {
        require(proofs.length == publicInputsBatch.length, "length mismatch");
        for (uint256 i = 0; i < proofs.length; i++) {
            _verifyAndRecord(proofs[i], publicInputsBatch[i]);
        }
    }

    // --- Admin ---

    function setProver(address _prover) external onlyProver {
        emit ProverUpdated(prover, _prover);
        prover = _prover;
    }

    function setVerifier(address _verifier) external onlyProver {
        emit VerifierUpdated(address(verifier), _verifier);
        verifier = IVerifier(_verifier);
    }

    // --- View ---

    function getStep(uint256 stepNumber) external view returns (Step memory) {
        return steps[stepNumber];
    }

    // --- Internal ---

    function _verifyAndRecord(
        uint256[8] calldata proof,
        uint256[] calldata publicInputs
    ) internal {
        require(publicInputs.length >= 5, "insufficient public inputs");

        uint256 preRoot = publicInputs[0];
        uint256 postRoot = publicInputs[1];
        uint256 stepSize = publicInputs[2];

        // Verify state chain: preRoot must match current state
        if (preRoot != currentStateRoot) {
            revert InvalidStateChain(currentStateRoot, preRoot);
        }

        // Verify the ZK proof
        uint256[2] memory a = [proof[0], proof[1]];
        uint256[2][2] memory b = [[proof[2], proof[3]], [proof[4], proof[5]]];
        uint256[2] memory c = [proof[6], proof[7]];

        bool valid = verifier.verifyProof(a, b, c, publicInputs);
        if (!valid) revert InvalidProof();

        // Record step
        steps[stepCount] = Step({
            preRoot: preRoot,
            postRoot: postRoot,
            stepSize: stepSize,
            timestamp: block.timestamp
        });

        currentStateRoot = postRoot;
        emit StepVerified(stepCount, preRoot, postRoot, stepSize, block.timestamp);
        stepCount++;
    }
}
