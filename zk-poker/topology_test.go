package zkpoker

import (
	"testing"
)

func TestTopologyInit(t *testing.T) {
	// Verify all transitions have been initialized
	emptyCount := 0
	for i := 0; i < NumTransitions; i++ {
		if len(Topology[i].Inputs) == 0 && len(Topology[i].Outputs) == 0 {
			emptyCount++
			t.Errorf("Transition %d (%s) has no arcs", i, TransitionName(i))
		}
	}
	if emptyCount > 0 {
		t.Errorf("%d transitions have no arcs defined", emptyCount)
	}
}

func TestDealTransitions(t *testing.T) {
	for i := 0; i < 52; i++ {
		tr := DealTransitionStart + i
		arcs := Topology[tr]

		// Should consume from deck
		if len(arcs.Inputs) != 1 || arcs.Inputs[0] != DeckPlace(i) {
			t.Errorf("Deal %d should consume from deck place %d", i, DeckPlace(i))
		}

		// Should produce to dealt
		if len(arcs.Outputs) != 1 || arcs.Outputs[0] != DealtPlace(i) {
			t.Errorf("Deal %d should produce to dealt place %d", i, DealtPlace(i))
		}
	}
}

func TestActionTransitions(t *testing.T) {
	actions := []string{"fold", "check", "call", "raise"}

	for p := 0; p < NumPlayers; p++ {
		for a := 0; a < 4; a++ {
			tr := ActionTransition(p, a)
			arcs := Topology[tr]

			// All actions should consume turn token
			hasTurn := false
			for _, inp := range arcs.Inputs {
				if inp == TurnPlace(p) {
					hasTurn = true
					break
				}
			}
			if !hasTurn {
				t.Errorf("Player %d %s should consume turn token", p, actions[a])
			}

			// All actions should consume active token
			hasActive := false
			for _, inp := range arcs.Inputs {
				if inp == ActivePlace(p) {
					hasActive = true
					break
				}
			}
			if !hasActive {
				t.Errorf("Player %d %s should consume active token", p, actions[a])
			}

			// Fold should NOT produce active token (produces folded instead)
			if a == ActionFold {
				hasActiveOutput := false
				hasFoldedOutput := false
				for _, out := range arcs.Outputs {
					if out == ActivePlace(p) {
						hasActiveOutput = true
					}
					if out == FoldedPlace(p) {
						hasFoldedOutput = true
					}
				}
				if hasActiveOutput {
					t.Errorf("Player %d fold should NOT produce active token", p)
				}
				if !hasFoldedOutput {
					t.Errorf("Player %d fold should produce folded token", p)
				}
			} else {
				// Non-fold actions should produce active token back
				hasActiveOutput := false
				for _, out := range arcs.Outputs {
					if out == ActivePlace(p) {
						hasActiveOutput = true
						break
					}
				}
				if !hasActiveOutput {
					t.Errorf("Player %d %s should produce active token", p, actions[a])
				}
			}
		}
	}
}

func TestSkipTransitions(t *testing.T) {
	for p := 0; p < NumPlayers; p++ {
		tr := SkipTransition(p)
		arcs := Topology[tr]

		// Skip should only consume turn token (not active)
		if len(arcs.Inputs) != 1 || arcs.Inputs[0] != TurnPlace(p) {
			t.Errorf("Player %d skip should only consume turn token", p)
		}

		// Skip should not produce active token
		for _, out := range arcs.Outputs {
			if out == ActivePlace(p) {
				t.Errorf("Player %d skip should not produce active token", p)
			}
		}
	}
}

func TestPhaseTransitions(t *testing.T) {
	tests := []struct {
		transition int
		name       string
		wantInput  int
		wantOutput int
	}{
		{TransitionStartHand, "start_hand", PhaseWaiting, PhasePreflop},
		{TransitionDealFlop, "deal_flop", PhasePreflop, PhaseFlop},
		{TransitionDealTurn, "deal_turn", PhaseFlop, PhaseTurn},
		{TransitionDealRiver, "deal_river", PhaseTurn, PhaseRiver},
		{TransitionToShowdown, "to_showdown", PhaseRiver, PhaseShowdown},
		{TransitionDetermineWinner, "determine_winner", PhaseShowdown, PlaceHandComplete},
		{TransitionEndHand, "end_hand", PlaceHandComplete, PhaseWaiting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arcs := Topology[tt.transition]

			// Check input phase
			hasInputPhase := false
			for _, inp := range arcs.Inputs {
				if inp == tt.wantInput {
					hasInputPhase = true
					break
				}
			}
			if !hasInputPhase {
				t.Errorf("%s should consume %s", tt.name, PlaceName(tt.wantInput))
			}

			// Check output phase
			hasOutputPhase := false
			for _, out := range arcs.Outputs {
				if out == tt.wantOutput {
					hasOutputPhase = true
					break
				}
			}
			if !hasOutputPhase {
				t.Errorf("%s should produce %s", tt.name, PlaceName(tt.wantOutput))
			}
		})
	}
}

func TestSimpleGameFlow(t *testing.T) {
	m := InitialMarking()

	// Initial state: waiting phase, all cards in deck, all players active
	if m[PhaseWaiting] != 1 {
		t.Fatal("Should start in waiting phase")
	}

	// Start hand should be enabled
	if !IsEnabled(m, TransitionStartHand) {
		t.Fatal("StartHand should be enabled in waiting phase")
	}

	// Fire start_hand
	m, ok := Fire(m, TransitionStartHand)
	if !ok {
		t.Fatal("Failed to fire StartHand")
	}

	// Should now be in preflop, player 0's turn
	if m[PhasePreflop] != 1 {
		t.Error("Should be in preflop phase")
	}
	if m[TurnPlace(0)] != 1 {
		t.Error("Should be player 0's turn")
	}

	// Player 0 can fold, check, call, or raise
	for a := 0; a < 4; a++ {
		if !IsEnabled(m, ActionTransition(0, a)) {
			t.Errorf("Player 0 %s should be enabled", []string{"fold", "check", "call", "raise"}[a])
		}
	}

	// Player 1 cannot act (not their turn)
	for a := 0; a < 4; a++ {
		if IsEnabled(m, ActionTransition(1, a)) {
			t.Errorf("Player 1 %s should NOT be enabled", []string{"fold", "check", "call", "raise"}[a])
		}
	}

	// Player 0 checks
	m, ok = Fire(m, ActionTransition(0, ActionCheck))
	if !ok {
		t.Fatal("Player 0 check failed")
	}

	// Now player 1's turn
	if m[TurnPlace(1)] != 1 {
		t.Error("Should be player 1's turn after player 0 checks")
	}
	if m[ActivePlace(0)] != 1 {
		t.Error("Player 0 should still be active after checking")
	}
}

func TestFoldRemovesActive(t *testing.T) {
	m := InitialMarking()

	// Start hand
	m, _ = Fire(m, TransitionStartHand)

	// Player 0 folds
	m, ok := Fire(m, ActionTransition(0, ActionFold))
	if !ok {
		t.Fatal("Player 0 fold failed")
	}

	// Player 0 should no longer be active
	if m[ActivePlace(0)] != 0 {
		t.Error("Player 0 should not be active after folding")
	}
	if m[FoldedPlace(0)] != 1 {
		t.Error("Player 0 should be marked as folded")
	}

	// Turn should pass to player 1
	if m[TurnPlace(1)] != 1 {
		t.Error("Should be player 1's turn")
	}
}

func TestFullBettingRound(t *testing.T) {
	m := InitialMarking()

	// Start hand
	m, _ = Fire(m, TransitionStartHand)

	// All 5 players check
	for p := 0; p < NumPlayers; p++ {
		if !IsEnabled(m, ActionTransition(p, ActionCheck)) {
			t.Fatalf("Player %d check should be enabled", p)
		}
		var ok bool
		m, ok = Fire(m, ActionTransition(p, ActionCheck))
		if !ok {
			t.Fatalf("Player %d check failed", p)
		}
	}

	// After all players act, betting_complete should be set
	if m[PlaceBettingComplete] != 1 {
		t.Error("Betting should be complete after all players act")
	}

	// Deal flop should now be enabled
	if !IsEnabled(m, TransitionDealFlop) {
		t.Error("DealFlop should be enabled after betting complete")
	}
}

func TestDealCards(t *testing.T) {
	m := InitialMarking()

	// Deal first card (2c, index 0)
	if !IsEnabled(m, DealTransitionStart) {
		t.Error("Deal 2c should be enabled")
	}

	m, ok := Fire(m, DealTransitionStart)
	if !ok {
		t.Fatal("Deal 2c failed")
	}

	// Card should now be dealt, not in deck
	if m[DeckPlace(0)] != 0 {
		t.Error("2c should no longer be in deck")
	}
	if m[DealtPlace(0)] != 1 {
		t.Error("2c should be in dealt pile")
	}

	// Can't deal same card again
	if IsEnabled(m, DealTransitionStart) {
		t.Error("Should not be able to deal 2c twice")
	}
}

func TestTransitionNames(t *testing.T) {
	tests := []struct {
		transition int
		want       string
	}{
		{DealTransitionStart, "deal_2c"},
		{DealTransitionStart + 51, "deal_As"},
		{ActionTransition(0, ActionFold), "p0_fold"},
		{ActionTransition(2, ActionRaise), "p2_raise"},
		{SkipTransition(3), "p3_skip"},
		{TransitionStartHand, "start_hand"},
		{TransitionDealFlop, "deal_flop"},
	}

	for _, tt := range tests {
		got := TransitionName(tt.transition)
		if got != tt.want {
			t.Errorf("TransitionName(%d) = %q, want %q", tt.transition, got, tt.want)
		}
	}
}

func TestPlaceNames(t *testing.T) {
	tests := []struct {
		place int
		want  string
	}{
		{DeckPlace(0), "deck_2c"},
		{DealtPlace(51), "dealt_As"},
		{ActivePlace(2), "p2_active"},
		{TurnPlace(0), "p0_turn"},
		{PhaseFlop, "phase_flop"},
		{PlaceBettingComplete, "betting_complete"},
	}

	for _, tt := range tests {
		got := PlaceName(tt.place)
		if got != tt.want {
			t.Errorf("PlaceName(%d) = %q, want %q", tt.place, got, tt.want)
		}
	}
}
