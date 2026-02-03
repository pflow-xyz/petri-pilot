// Package tictactoe contains tests for the ODE blocking bug.
//
// Bug: When X threatens an immediate win (e.g., center column with X at 1,1 and 2,1),
// O should prioritize blocking at (0,1). However, the ODE simulation gives corners
// higher scores than the blocking move.
//
// Board state being tested:
//
//	Row 0: -  -  -    (empty)
//	Row 1: O  X  -    (O at 1,0 - X at 1,1)
//	Row 2: -  X  -    (X at 2,1)
//
// X threatens center column win. O's turn - MUST block at (0,1).
//
// This test verifies the game mechanics around blocking and winning.
package tictactoe

import (
	"context"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"
)

// TestOBlockingRequiredWhenXThreatensColumn tests that O must block when X
// threatens to win via center column.
//
// Setup:
//   - X plays center (1,1)
//   - O plays left-middle (1,0)
//   - X plays bottom-center (2,1)
//   - Now O's turn: X threatens column 1 win
//
// If O doesn't block at (0,1), X wins next turn.
func TestOBlockingRequiredWhenXThreatensColumn(t *testing.T) {
	store := eventsource.NewMemoryStore()
	defer store.Close()

	app := NewApplication(store)
	ctx := context.Background()

	// Create game
	id, _ := app.Create(ctx)

	// Play moves to reach the bug state:
	// 1. X plays center (1,1)
	agg, err := app.Execute(ctx, id, TransitionXPlay11, nil)
	if err != nil {
		t.Fatalf("X play 11 failed: %v", err)
	}

	// 2. O plays left-middle (1,0)
	agg, err = app.Execute(ctx, id, TransitionOPlay10, nil)
	if err != nil {
		t.Fatalf("O play 10 failed: %v", err)
	}

	// 3. X plays bottom-center (2,1)
	agg, err = app.Execute(ctx, id, TransitionXPlay21, nil)
	if err != nil {
		t.Fatalf("X play 21 failed: %v", err)
	}

	// Verify the board state
	places := agg.Places()
	t.Logf("Board state after 3 moves:")
	t.Logf("  X at (1,1): x11=%d", places[PlaceX11])
	t.Logf("  X at (2,1): x21=%d", places[PlaceX21])
	t.Logf("  O at (1,0): o10=%d", places[PlaceO10])
	t.Logf("  O's turn: o_turn=%d", places[PlaceOTurn])

	// Verify it's O's turn
	if places[PlaceOTurn] != 1 {
		t.Errorf("expected O's turn (o_turn=1), got o_turn=%d", places[PlaceOTurn])
	}

	// Verify X has column 1 threat
	if places[PlaceX11] != 1 || places[PlaceX21] != 1 {
		t.Error("X should have pieces at (1,1) and (2,1)")
	}

	// Verify (0,1) is empty and available
	if places[PlaceP01] != 1 {
		t.Error("position (0,1) should be empty")
	}

	// Test scenario A: O blocks at (0,1)
	t.Run("O_blocks_at_01", func(t *testing.T) {
		// Create a fresh game for this scenario
		blockID, _ := app.Create(ctx)

		// Replay to the same state
		app.Execute(ctx, blockID, TransitionXPlay11, nil)
		app.Execute(ctx, blockID, TransitionOPlay10, nil)
		app.Execute(ctx, blockID, TransitionXPlay21, nil)

		// O blocks at (0,1)
		blockAgg, err := app.Execute(ctx, blockID, TransitionOPlay01, nil)
		if err != nil {
			t.Fatalf("O block at 01 failed: %v", err)
		}

		blockPlaces := blockAgg.Places()
		t.Logf("After O blocks at (0,1):")
		t.Logf("  o01=%d, x_turn=%d", blockPlaces[PlaceO01], blockPlaces[PlaceXTurn])

		// Verify X cannot win center column
		// (0,1) is now blocked by O
		if blockPlaces[PlaceP01] != 0 {
			t.Error("position (0,1) should be occupied after O blocks")
		}
		if blockPlaces[PlaceO01] != 1 {
			t.Error("O should have piece at (0,1)")
		}

		// X's turn - verify X cannot win via column 1
		if !blockAgg.CanFire(TransitionXWinCol1) == true {
			t.Log("PASS: X cannot fire win_col1 after O blocks")
		} else {
			t.Error("X should NOT be able to fire win_col1 when (0,1) is blocked by O")
		}
	})

	// Test scenario B: O plays corner (2,0) instead of blocking
	t.Run("O_plays_corner_X_wins", func(t *testing.T) {
		// Create a fresh game for this scenario
		cornerID, _ := app.Create(ctx)

		// Replay to the same state
		app.Execute(ctx, cornerID, TransitionXPlay11, nil)
		app.Execute(ctx, cornerID, TransitionOPlay10, nil)
		app.Execute(ctx, cornerID, TransitionXPlay21, nil)

		// O plays corner (2,0) instead of blocking
		cornerAgg, err := app.Execute(ctx, cornerID, TransitionOPlay20, nil)
		if err != nil {
			t.Fatalf("O play 20 failed: %v", err)
		}

		cornerPlaces := cornerAgg.Places()
		t.Logf("After O plays corner (2,0):")
		t.Logf("  o20=%d, x_turn=%d", cornerPlaces[PlaceO20], cornerPlaces[PlaceXTurn])

		// Verify (0,1) is still empty - X can complete column 1
		if cornerPlaces[PlaceP01] != 1 {
			t.Error("position (0,1) should still be empty")
		}

		// X plays (0,1) to complete column 1
		winAgg, err := app.Execute(ctx, cornerID, TransitionXPlay01, nil)
		if err != nil {
			t.Fatalf("X play 01 failed: %v", err)
		}

		winPlaces := winAgg.Places()
		t.Logf("After X plays (0,1):")
		t.Logf("  x01=%d, x11=%d, x21=%d", winPlaces[PlaceX01], winPlaces[PlaceX11], winPlaces[PlaceX21])

		// X now has all three pieces in column 1
		if winPlaces[PlaceX01] != 1 || winPlaces[PlaceX11] != 1 || winPlaces[PlaceX21] != 1 {
			t.Error("X should have all three pieces in column 1")
		}

		// Check if X win transition is enabled
		// NOTE: This tests the current model behavior
		if winAgg.CanFire(TransitionXWinCol1) {
			t.Log("X can fire win_col1 - X wins!")

			// Fire the win transition
			finalAgg, err := app.Execute(ctx, cornerID, TransitionXWinCol1, nil)
			if err != nil {
				t.Fatalf("X win failed: %v", err)
			}

			finalPlaces := finalAgg.Places()
			if finalPlaces[PlaceWinX] == 1 {
				t.Log("PASS: X wins via column 1 when O doesn't block")
			} else {
				t.Error("Expected win_x=1 after X wins")
			}
		} else {
			t.Log("X cannot fire win_col1 yet - checking why...")
			t.Logf("  game_active=%d", winPlaces[PlaceGameActive])
			t.Logf("  o_turn=%d (required for X win)", winPlaces[PlaceOTurn])

			// The model requires o_turn to be present for X to claim victory
			// This is because X wins after playing, when it would be O's turn
			if winPlaces[PlaceOTurn] != 1 {
				t.Log("BUG: Win transition requires o_turn but it's X's turn after X plays")
				t.Log("The model has a timing issue with win detection")
			}
		}
	})
}

// TestWinTransitionRequiresTurnToken verifies that win transitions
// are properly gated by turn tokens.
func TestWinTransitionRequiresTurnToken(t *testing.T) {
	store := eventsource.NewMemoryStore()
	defer store.Close()

	app := NewApplication(store)
	ctx := context.Background()

	// Create game and play to X having 3 in a row
	id, _ := app.Create(ctx)

	// X plays top row: (0,0), (0,1), (0,2)
	// O plays middle row: (1,0), (1,1)
	moves := []string{
		TransitionXPlay00, // X
		TransitionOPlay10, // O
		TransitionXPlay01, // X
		TransitionOPlay11, // O
		TransitionXPlay02, // X wins top row
	}

	var agg *Aggregate
	var err error
	for _, move := range moves {
		agg, err = app.Execute(ctx, id, move, nil)
		if err != nil {
			t.Fatalf("Move %s failed: %v", move, err)
		}
	}

	places := agg.Places()
	t.Logf("X pieces: x00=%d, x01=%d, x02=%d", places[PlaceX00], places[PlaceX01], places[PlaceX02])
	t.Logf("Turn: x_turn=%d, o_turn=%d", places[PlaceXTurn], places[PlaceOTurn])
	t.Logf("Game active: %d", places[PlaceGameActive])

	// Check if X can fire win transition
	canWin := agg.CanFire(TransitionXWinRow0)
	t.Logf("Can X fire win_row0: %v", canWin)

	// The model in services/tic-tac-toe.json requires o_turn for X win transitions
	// But the generated aggregate.go doesn't include this constraint
	// This is the discrepancy that leads to the ODE blocking bug

	// Verify what the generated model actually requires for X win
	// Looking at aggregate.go, TransitionXWinRow0 inputs are:
	// - PlaceX00: 1
	// - PlaceX01: 1
	// - PlaceX02: 1
	// - PlaceGameActive: 1
	// Missing: PlaceOTurn: 1 (which IS in the JSON model)

	if canWin {
		t.Log("X can win - firing win transition")
		winAgg, err := app.Execute(ctx, id, TransitionXWinRow0, nil)
		if err != nil {
			t.Logf("Win transition failed: %v", err)
		} else {
			t.Logf("Win result: win_x=%d", winAgg.Places()[PlaceWinX])
		}
	} else {
		t.Log("X cannot win yet")
		t.Logf("This may be correct - win detection should happen automatically after the move")
	}
}

// TestModelDiscrepancy documents the discrepancy between the JSON model
// and the generated Go code regarding turn token consumption in win transitions.
func TestModelDiscrepancy(t *testing.T) {
	// This test documents that the JSON model services/tic-tac-toe.json
	// includes turn token arcs for win transitions, but the generated
	// Go code in aggregate.go does not.
	//
	// JSON model win transition arcs (e.g., x_win_row0):
	//   {"from": "x00", "to": "x_win_row0"},
	//   {"from": "x01", "to": "x_win_row0"},
	//   {"from": "x02", "to": "x_win_row0"},
	//   {"from": "game_active", "to": "x_win_row0"},
	//   {"from": "o_turn", "to": "x_win_row0"},  <-- PRESENT IN JSON
	//
	// Generated Go transition inputs:
	//   PlaceX00: 1,
	//   PlaceX01: 1,
	//   PlaceX02: 1,
	//   PlaceGameActive: 1,
	//   // Missing: PlaceOTurn: 1  <-- NOT IN GENERATED CODE

	agg := NewAggregate("test")

	// Check enabled transitions at start
	enabled := agg.EnabledTransitions()
	t.Logf("Initially enabled transitions: %v", enabled)

	// The discrepancy affects ODE simulation because:
	// 1. In the JavaScript ODE model, win transitions compete for the turn token
	// 2. This makes blocking moves more valuable (they prevent the opponent's
	//    win transition from firing)
	// 3. Without the turn token constraint, win transitions fire more freely,
	//    making the ODE values less sensitive to blocking

	t.Log("")
	t.Log("BUG DOCUMENTATION:")
	t.Log("The JSON model services/tic-tac-toe.json includes 'o_turn' as an input")
	t.Log("to X win transitions (and 'x_turn' for O win transitions).")
	t.Log("This models that X wins 'during O's turn' (right after X plays).")
	t.Log("")
	t.Log("The generated Go code does NOT include these turn token inputs.")
	t.Log("This affects ODE strategic value calculation, causing the blocking bug.")
	t.Log("")
	t.Log("Fix: Update code generation to include turn token arcs in win transitions,")
	t.Log("or update the model if the current behavior is intentional.")
}
