// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IVerifier} from "./IVerifier.sol";

/// @title ZkOde
/// @author pflow-xyz (https://github.com/pflow-xyz/petri-pilot/tree/main/solidity)
/// @notice State commitment manager for ZK-proven ODE steps on a Petri net.
/// @dev Accepts Groth16 proofs of Tsit5 ODE integration steps and maintains
///      a chain of state root commitments. Each step proves that the prover
///      correctly computed one integration step over the hidden marking.
///
///      Public inputs layout (3 + numTransitions values):
///        [0]          PreStateRoot   - MiMC hash of marking before step
///        [1]          PostStateRoot  - MiMC hash of marking after step
///        [2]          StepSize       - Fixed-point step size (h * 10^18)
///        [3..3+M-1]   Rates          - Rate constants for each transition
///
///      When enforceOptimal is enabled, the caller must specify which transition
///      they chose. The contract verifies it has the highest rate — proving
///      the player picked the optimal move according to the ODE dynamics.
contract ZkOde {
    // --- State ---
    IVerifier public verifier;
    address public prover;
    uint256 public currentStateRoot;
    uint256 public stepCount;
    uint256 public immutable numTransitions;
    bool public enforceOptimal;

    struct Step {
        uint256 preRoot;
        uint256 postRoot;
        uint256 stepSize;
        uint256 chosenTransition;
        uint256 timestamp;
    }

    mapping(uint256 => Step) public steps;

    // --- Events ---
    event StepVerified(
        uint256 indexed stepNumber,
        uint256 preRoot,
        uint256 postRoot,
        uint256 stepSize,
        uint256 chosenTransition,
        uint256 timestamp
    );

    event ProverUpdated(address indexed oldProver, address indexed newProver);
    event VerifierUpdated(address indexed oldVerifier, address indexed newVerifier);
    event EnforceOptimalUpdated(bool enforceOptimal);

    // --- Errors ---
    error InvalidProof();
    error InvalidStateChain(uint256 expected, uint256 got);
    error OnlyProver();
    error NotOptimalPlay(uint256 chosen, uint256 chosenRate, uint256 betterTransition, uint256 betterRate);
    error InvalidTransition(uint256 transition, uint256 max);

    // --- Modifiers ---
    modifier onlyProver() {
        if (msg.sender != prover) revert OnlyProver();
        _;
    }

    // --- Constructor ---
    constructor(address _verifier, uint256 _genesisRoot, uint256 _numTransitions, bool _enforceOptimal) {
        verifier = IVerifier(_verifier);
        prover = msg.sender;
        currentStateRoot = _genesisRoot;
        numTransitions = _numTransitions;
        enforceOptimal = _enforceOptimal;
    }

    // --- Core ---

    /// @notice Submit a single ODE step proof.
    /// @param proof Groth16 proof components [a, b, c].
    /// @param publicInputs Array of public circuit inputs.
    /// @param chosenTransition Index of the transition chosen by the player.
    function submitStep(
        uint256[8] calldata proof,
        uint256[] calldata publicInputs,
        uint256 chosenTransition
    ) external onlyProver {
        _verifyAndRecord(proof, publicInputs, chosenTransition);
    }

    /// @notice Submit a batch of sequential ODE step proofs in one transaction.
    /// @param proofs Array of Groth16 proofs.
    /// @param publicInputsBatch Array of public input arrays (one per proof).
    /// @param chosenTransitions Array of chosen transition indices (one per proof).
    function submitBatchSteps(
        uint256[8][] calldata proofs,
        uint256[][] calldata publicInputsBatch,
        uint256[] calldata chosenTransitions
    ) external onlyProver {
        require(proofs.length == publicInputsBatch.length, "length mismatch");
        require(proofs.length == chosenTransitions.length, "transitions length mismatch");
        for (uint256 i = 0; i < proofs.length; i++) {
            _verifyAndRecord(proofs[i], publicInputsBatch[i], chosenTransitions[i]);
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

    function setEnforceOptimal(bool _enforceOptimal) external onlyProver {
        enforceOptimal = _enforceOptimal;
        emit EnforceOptimalUpdated(_enforceOptimal);
    }

    // --- View ---

    function getStep(uint256 stepNumber) external view returns (Step memory) {
        return steps[stepNumber];
    }

    // --- Internal ---

    function _verifyAndRecord(
        uint256[8] calldata proof,
        uint256[] calldata publicInputs,
        uint256 chosenTransition
    ) internal {
        require(publicInputs.length >= 3 + numTransitions, "insufficient public inputs");

        uint256 preRoot = publicInputs[0];
        uint256 postRoot = publicInputs[1];
        uint256 stepSize = publicInputs[2];

        // Verify state chain: preRoot must match current state
        if (preRoot != currentStateRoot) {
            revert InvalidStateChain(currentStateRoot, preRoot);
        }

        // Validate chosen transition index
        if (chosenTransition >= numTransitions) {
            revert InvalidTransition(chosenTransition, numTransitions);
        }

        // Enforce optimal play: chosen transition must have the highest rate
        if (enforceOptimal) {
            uint256 chosenRate = publicInputs[3 + chosenTransition];
            for (uint256 t = 0; t < numTransitions; t++) {
                uint256 rate = publicInputs[3 + t];
                if (rate > chosenRate) {
                    revert NotOptimalPlay(chosenTransition, chosenRate, t, rate);
                }
            }
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
            chosenTransition: chosenTransition,
            timestamp: block.timestamp
        });

        currentStateRoot = postRoot;
        emit StepVerified(stepCount, preRoot, postRoot, stepSize, chosenTransition, block.timestamp);
        stepCount++;
    }
}
