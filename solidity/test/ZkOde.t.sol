// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import {ZkOdeVerifier} from "../src/ZkOdeVerifier.sol";
import {ZkOde} from "../src/ZkOde.sol";

contract ZkOdeTest is Test {
    ZkOdeVerifier verifier;
    ZkOde zkOde;
    address proverAddr = address(this);

    uint256 constant GENESIS_ROOT = 12345;
    uint256 constant NUM_TRANSITIONS = 2;

    function setUp() public {
        verifier = new ZkOdeVerifier();
        zkOde = new ZkOde(address(verifier), GENESIS_ROOT, NUM_TRANSITIONS, false);
    }

    function _makePublicInputs(uint256 preRoot, uint256 postRoot) internal pure returns (uint256[] memory) {
        uint256[] memory publicInputs = new uint256[](5);
        publicInputs[0] = preRoot;
        publicInputs[1] = postRoot;
        publicInputs[2] = 1e16;  // stepSize (0.01 * 1e18)
        publicInputs[3] = 1e18;  // rate0
        publicInputs[4] = 1e18;  // rate1
        return publicInputs;
    }

    function testInitialState() public view {
        assertEq(zkOde.currentStateRoot(), GENESIS_ROOT);
        assertEq(zkOde.stepCount(), 0);
        assertEq(zkOde.prover(), proverAddr);
        assertEq(zkOde.numTransitions(), NUM_TRANSITIONS);
        assertEq(zkOde.enforceOptimal(), false);
    }

    function testSubmitStep() public {
        uint256[8] memory proof;
        uint256[] memory publicInputs = _makePublicInputs(GENESIS_ROOT, 67890);

        zkOde.submitStep(proof, publicInputs, 0);

        assertEq(zkOde.currentStateRoot(), 67890);
        assertEq(zkOde.stepCount(), 1);

        ZkOde.Step memory step = zkOde.getStep(0);
        assertEq(step.preRoot, GENESIS_ROOT);
        assertEq(step.postRoot, 67890);
        assertEq(step.stepSize, 1e16);
        assertEq(step.chosenTransition, 0);
    }

    function testChainedSteps() public {
        uint256 root = GENESIS_ROOT;
        for (uint256 i = 0; i < 5; i++) {
            uint256[8] memory proof;
            uint256 nextRoot = root + 1000 * (i + 1);
            uint256[] memory publicInputs = _makePublicInputs(root, nextRoot);

            zkOde.submitStep(proof, publicInputs, i % NUM_TRANSITIONS);
            root = nextRoot;
        }

        assertEq(zkOde.stepCount(), 5);
        assertEq(zkOde.currentStateRoot(), root);
    }

    function testBatchSteps() public {
        uint256[8][] memory proofs = new uint256[8][](3);
        uint256[][] memory publicInputsBatch = new uint256[][](3);
        uint256[] memory chosenTransitions = new uint256[](3);

        uint256 root = GENESIS_ROOT;
        for (uint256 i = 0; i < 3; i++) {
            uint256 nextRoot = root + 100 * (i + 1);
            publicInputsBatch[i] = _makePublicInputs(root, nextRoot);
            chosenTransitions[i] = 0;
            root = nextRoot;
        }

        zkOde.submitBatchSteps(proofs, publicInputsBatch, chosenTransitions);

        assertEq(zkOde.stepCount(), 3);
        assertEq(zkOde.currentStateRoot(), root);
    }

    function testRevertOnBrokenChain() public {
        uint256[8] memory proof;
        uint256[] memory publicInputs = _makePublicInputs(99999, 67890);

        vm.expectRevert(
            abi.encodeWithSelector(
                ZkOde.InvalidStateChain.selector,
                GENESIS_ROOT,
                99999
            )
        );
        zkOde.submitStep(proof, publicInputs, 0);
    }

    function testRevertOnNonProver() public {
        address other = address(0xBEEF);
        uint256[8] memory proof;
        uint256[] memory publicInputs = _makePublicInputs(GENESIS_ROOT, 67890);

        vm.prank(other);
        vm.expectRevert(ZkOde.OnlyProver.selector);
        zkOde.submitStep(proof, publicInputs, 0);
    }

    function testRevertOnInvalidTransition() public {
        uint256[8] memory proof;
        uint256[] memory publicInputs = _makePublicInputs(GENESIS_ROOT, 67890);

        vm.expectRevert(
            abi.encodeWithSelector(ZkOde.InvalidTransition.selector, 5, NUM_TRANSITIONS)
        );
        zkOde.submitStep(proof, publicInputs, 5);
    }

    function testSetProver() public {
        address newProver = address(0xCAFE);
        zkOde.setProver(newProver);
        assertEq(zkOde.prover(), newProver);
    }

    function testSetVerifier() public {
        ZkOdeVerifier newVerifier = new ZkOdeVerifier();
        zkOde.setVerifier(address(newVerifier));
        assertEq(address(zkOde.verifier()), address(newVerifier));
    }

    function testSetEnforceOptimal() public {
        assertEq(zkOde.enforceOptimal(), false);
        zkOde.setEnforceOptimal(true);
        assertEq(zkOde.enforceOptimal(), true);
    }
}

/// @dev Separate test contract with enforceOptimal=true.
contract ZkOdeOptimalTest is Test {
    ZkOdeVerifier verifier;
    ZkOde zkOde;

    uint256 constant GENESIS_ROOT = 12345;
    uint256 constant NUM_TRANSITIONS = 35; // tic-tac-toe

    function setUp() public {
        verifier = new ZkOdeVerifier();
        zkOde = new ZkOde(address(verifier), GENESIS_ROOT, NUM_TRANSITIONS, true);
    }

    function _makeTTTPublicInputs(
        uint256 preRoot,
        uint256 postRoot,
        uint256[] memory rates
    ) internal pure returns (uint256[] memory) {
        uint256[] memory publicInputs = new uint256[](3 + rates.length);
        publicInputs[0] = preRoot;
        publicInputs[1] = postRoot;
        publicInputs[2] = 1e16; // stepSize
        for (uint256 i = 0; i < rates.length; i++) {
            publicInputs[3 + i] = rates[i];
        }
        return publicInputs;
    }

    function testOptimalPlayAccepted() public {
        uint256[8] memory proof;

        // Transition 4 (x_play_11, center) has the highest rate
        uint256[] memory rates = new uint256[](NUM_TRANSITIONS);
        for (uint256 i = 0; i < NUM_TRANSITIONS; i++) {
            rates[i] = 1e15; // low base rate
        }
        rates[4] = 5e18; // center move has highest rate

        uint256[] memory publicInputs = _makeTTTPublicInputs(GENESIS_ROOT, 67890, rates);

        // Choosing transition 4 (highest rate) should succeed
        zkOde.submitStep(proof, publicInputs, 4);

        assertEq(zkOde.stepCount(), 1);
        ZkOde.Step memory step = zkOde.getStep(0);
        assertEq(step.chosenTransition, 4);
    }

    function testSuboptimalPlayReverted() public {
        uint256[8] memory proof;

        // Transition 4 has the highest rate
        uint256[] memory rates = new uint256[](NUM_TRANSITIONS);
        for (uint256 i = 0; i < NUM_TRANSITIONS; i++) {
            rates[i] = 1e15;
        }
        rates[4] = 5e18; // center move is optimal

        uint256[] memory publicInputs = _makeTTTPublicInputs(GENESIS_ROOT, 67890, rates);

        // Choosing transition 0 (suboptimal) should revert
        vm.expectRevert(
            abi.encodeWithSelector(
                ZkOde.NotOptimalPlay.selector,
                0,       // chosen
                1e15,    // chosenRate
                4,       // betterTransition
                5e18     // betterRate
            )
        );
        zkOde.submitStep(proof, publicInputs, 0);
    }

    function testTiedRatesAccepted() public {
        uint256[8] memory proof;

        // Multiple transitions tied at the highest rate
        uint256[] memory rates = new uint256[](NUM_TRANSITIONS);
        for (uint256 i = 0; i < NUM_TRANSITIONS; i++) {
            rates[i] = 1e15;
        }
        rates[0] = 3e18;
        rates[4] = 3e18; // tied with transition 0

        uint256[] memory publicInputs = _makeTTTPublicInputs(GENESIS_ROOT, 67890, rates);

        // Either tied transition should be accepted
        zkOde.submitStep(proof, publicInputs, 0);
        assertEq(zkOde.stepCount(), 1);
    }

    function testOptimalCanBeDisabledAtRuntime() public {
        // Disable enforcement
        zkOde.setEnforceOptimal(false);

        uint256[8] memory proof;
        uint256[] memory rates = new uint256[](NUM_TRANSITIONS);
        rates[4] = 5e18; // transition 4 is optimal

        uint256[] memory publicInputs = _makeTTTPublicInputs(GENESIS_ROOT, 67890, rates);

        // Suboptimal choice accepted when enforcement is off
        zkOde.submitStep(proof, publicInputs, 0);
        assertEq(zkOde.stepCount(), 1);
    }
}
