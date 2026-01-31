package zktictactoe

import (
	"testing"
)

func TestE2E_FullGameWithProofs(t *testing.T) {
	// Create ZK integration
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatalf("failed to create ZK integration: %v", err)
	}

	// Create a game
	game := NewPetriGame()
	zk.games["test-1"] = game

	t.Log("Initial state:")
	t.Log(game.Marking.String())

	// Play moves to X wins main diagonal (positions 0, 4, 8)
	// Diagonal: (0,0)-(1,1)-(2,2)
	moves := []struct {
		pos    int
		player string
	}{
		{0, "X"}, // X top-left
		{1, "O"}, // O top-center
		{4, "X"}, // X center
		{2, "O"}, // O top-right
		{8, "X"}, // X bottom-right - completes main diagonal
	}

	for i, m := range moves {
		witness, err := game.MakeMove(m.pos)
		if err != nil {
			t.Fatalf("move %d (%s at %d) failed: %v", i+1, m.player, m.pos, err)
		}

		// Generate and verify proof
		assignment := witness.ToPetriTransitionAssignment()
		proofResult, err := zk.prover.Prove("transition", assignment)
		if err != nil {
			t.Fatalf("proof generation failed for move %d: %v", i+1, err)
		}

		err = zk.prover.Verify("transition", assignment)
		if err != nil {
			t.Fatalf("proof verification failed for move %d: %v", i+1, err)
		}

		t.Logf("Move %d: %s plays %d - proof verified (%d public inputs)",
			i+1, m.player, m.pos, len(proofResult.PublicInputs))
	}

	t.Log("\nAfter moves:")
	t.Log(game.Marking.String())

	// Check that X has main diagonal (0, 4, 8)
	if game.Marking[X00] != 1 || game.Marking[X11] != 1 || game.Marking[X22] != 1 {
		t.Fatal("expected X to have main diagonal pieces")
	}

	// Fire win transition (x_win_diag = 25)
	t.Log("Firing win transition (x_win_diag)...")
	winWitness, err := game.FireTransition(TXWinDiag)
	if err != nil {
		t.Fatalf("win transition failed: %v", err)
	}

	// Generate and verify win transition proof
	winAssignment := winWitness.ToPetriTransitionAssignment()
	winProofResult, err := zk.prover.Prove("transition", winAssignment)
	if err != nil {
		t.Fatalf("win proof generation failed: %v", err)
	}

	err = zk.prover.Verify("transition", winAssignment)
	if err != nil {
		t.Fatalf("win proof verification failed: %v", err)
	}

	t.Logf("Win transition proof verified (%d public inputs)", len(winProofResult.PublicInputs))

	// Verify X won
	if game.Winner() != 1 {
		t.Fatalf("expected X to win, got winner=%d", game.Winner())
	}

	t.Log("\nFinal state:")
	t.Log(game.Marking.String())

	// Get win witness and generate win proof
	t.Log("Generating win state proof...")
	stateWitness := game.GetWinWitness()
	if stateWitness == nil {
		t.Fatal("expected win witness")
	}

	stateAssignment := stateWitness.ToPetriWinAssignment()
	stateProofResult, err := zk.prover.Prove("win", stateAssignment)
	if err != nil {
		t.Fatalf("win state proof generation failed: %v", err)
	}

	err = zk.prover.Verify("win", stateAssignment)
	if err != nil {
		t.Fatalf("win state proof verification failed: %v", err)
	}

	t.Logf("Win state proof verified (%d public inputs)", len(stateProofResult.PublicInputs))

	// Log proof data
	t.Log("\n=== PROOF SUMMARY ===")
	t.Logf("Pre-state root:  %s", winWitness.PreStateRoot.String()[:40]+"...")
	t.Logf("Post-state root: %s", winWitness.PostStateRoot.String()[:40]+"...")
	t.Logf("Winner: X")
	t.Logf("Proofs generated: 6 (5 moves + 1 win transition)")
	t.Logf("All proofs verified: true")
}

func TestE2E_InvalidMoveRejected(t *testing.T) {
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatal(err)
	}

	game := NewPetriGame()
	zk.games["test-2"] = game

	// X plays center
	_, err = game.MakeMove(4)
	if err != nil {
		t.Fatal(err)
	}

	// O tries to play center (already occupied)
	_, err = game.MakeMove(4)
	if err == nil {
		t.Fatal("expected error for occupied cell")
	}
	t.Logf("Correctly rejected invalid move: %v", err)
}

func TestE2E_WrongTurnRejected(t *testing.T) {
	game := NewPetriGame()

	// X plays center
	_, err := game.MakeMove(4)
	if err != nil {
		t.Fatal(err)
	}

	// Now it's O's turn - try to fire an X transition directly
	_, err = game.FireTransition(TXPlay00) // X play at 0
	if err == nil {
		t.Fatal("expected error for wrong turn")
	}
	t.Logf("Correctly rejected wrong turn: %v", err)
}

func TestE2E_StateRootChain(t *testing.T) {
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatal(err)
	}

	game := NewPetriGame()
	zk.games["test-3"] = game

	// Track state roots
	initialRoot := game.CurrentRoot()
	t.Logf("Initial root: %s", initialRoot.String()[:30]+"...")

	moves := []int{0, 1, 4, 2, 8} // X wins diagonal: 0, 4, 8
	var lastRoot = initialRoot

	for i, pos := range moves {
		witness, err := game.MakeMove(pos)
		if err != nil {
			t.Fatal(err)
		}

		// Verify chain continuity
		if witness.PreStateRoot.Cmp(lastRoot) != 0 {
			t.Fatalf("move %d: pre-state root mismatch", i+1)
		}

		lastRoot = witness.PostStateRoot
		t.Logf("Move %d: root %s -> %s",
			i+1,
			witness.PreStateRoot.String()[:20]+"...",
			witness.PostStateRoot.String()[:20]+"...")
	}

	// Verify we have the right number of roots
	if len(game.Roots) != 6 { // initial + 5 moves
		t.Fatalf("expected 6 roots, got %d", len(game.Roots))
	}

	t.Log("State root chain verified - all transitions link correctly")
}
