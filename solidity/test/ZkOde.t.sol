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

    function setUp() public {
        verifier = new ZkOdeVerifier();
        zkOde = new ZkOde(address(verifier), GENESIS_ROOT);
    }

    function testInitialState() public view {
        assertEq(zkOde.currentStateRoot(), GENESIS_ROOT);
        assertEq(zkOde.stepCount(), 0);
        assertEq(zkOde.prover(), proverAddr);
    }

    function testSubmitStep() public {
        uint256[8] memory proof;
        uint256[] memory publicInputs = new uint256[](5);
        publicInputs[0] = GENESIS_ROOT; // preRoot
        publicInputs[1] = 67890;        // postRoot
        publicInputs[2] = 1e16;         // stepSize (0.01 * 1e18)
        publicInputs[3] = 1e18;         // rate0
        publicInputs[4] = 1e18;         // rate1

        zkOde.submitStep(proof, publicInputs);

        assertEq(zkOde.currentStateRoot(), 67890);
        assertEq(zkOde.stepCount(), 1);

        ZkOde.Step memory step = zkOde.getStep(0);
        assertEq(step.preRoot, GENESIS_ROOT);
        assertEq(step.postRoot, 67890);
        assertEq(step.stepSize, 1e16);
    }

    function testChainedSteps() public {
        uint256 root = GENESIS_ROOT;
        for (uint256 i = 0; i < 5; i++) {
            uint256[8] memory proof;
            uint256 nextRoot = root + 1000 * (i + 1);

            uint256[] memory publicInputs = new uint256[](5);
            publicInputs[0] = root;
            publicInputs[1] = nextRoot;
            publicInputs[2] = 1e16;
            publicInputs[3] = 1e18;
            publicInputs[4] = 1e18;

            zkOde.submitStep(proof, publicInputs);
            root = nextRoot;
        }

        assertEq(zkOde.stepCount(), 5);
        assertEq(zkOde.currentStateRoot(), root);
    }

    function testBatchSteps() public {
        uint256[8][] memory proofs = new uint256[8][](3);
        uint256[][] memory publicInputsBatch = new uint256[][](3);

        uint256 root = GENESIS_ROOT;
        for (uint256 i = 0; i < 3; i++) {
            uint256 nextRoot = root + 100 * (i + 1);
            publicInputsBatch[i] = new uint256[](5);
            publicInputsBatch[i][0] = root;
            publicInputsBatch[i][1] = nextRoot;
            publicInputsBatch[i][2] = 1e16;
            publicInputsBatch[i][3] = 1e18;
            publicInputsBatch[i][4] = 1e18;
            root = nextRoot;
        }

        zkOde.submitBatchSteps(proofs, publicInputsBatch);

        assertEq(zkOde.stepCount(), 3);
        assertEq(zkOde.currentStateRoot(), root);
    }

    function testRevertOnBrokenChain() public {
        uint256[8] memory proof;
        uint256[] memory publicInputs = new uint256[](5);
        publicInputs[0] = 99999; // wrong preRoot
        publicInputs[1] = 67890;
        publicInputs[2] = 1e16;
        publicInputs[3] = 1e18;
        publicInputs[4] = 1e18;

        vm.expectRevert(
            abi.encodeWithSelector(
                ZkOde.InvalidStateChain.selector,
                GENESIS_ROOT,
                99999
            )
        );
        zkOde.submitStep(proof, publicInputs);
    }

    function testRevertOnNonProver() public {
        address other = address(0xBEEF);
        uint256[8] memory proof;
        uint256[] memory publicInputs = new uint256[](5);
        publicInputs[0] = GENESIS_ROOT;
        publicInputs[1] = 67890;
        publicInputs[2] = 1e16;
        publicInputs[3] = 1e18;
        publicInputs[4] = 1e18;

        vm.prank(other);
        vm.expectRevert(ZkOde.OnlyProver.selector);
        zkOde.submitStep(proof, publicInputs);
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
}
