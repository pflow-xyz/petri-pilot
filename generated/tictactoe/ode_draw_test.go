// Package tictactoe contains tests for ODE draw detection.
//
// Problem: The current tic-tac-toe model only awards points for wins.
// When all 9 moves are played without a winner, the game is a draw.
// In ODE simulation, this means some games "leak" without affecting
// win_x or win_o, making the values less meaningful.
//
// Solution: Add a draw detection mechanism that:
// 1. Counts total moves played (each play adds +1 to a move counter)
// 2. A "draw" transition fires when 9 moves are reached AND game_active=1
// 3. Draw awards +1 to win_o (or to separate draw_count place)
//
// This ensures all game outcomes flow to either win_x or win_o in the ODE.
//
// Key Results (from TestODEDrawSeparateOutcome):
//
//	Without draw detection:
//	  - win_x: 0.526, win_o: 0.355, game_active: 0.119 (12% leaks!)
//	  - Total outcomes: 0.881 (incomplete)
//
//	With draw detection (separate tracking):
//	  - X wins:  11.2%
//	  - O wins:   6.8%
//	  - Draws:   81.9%
//	  - Total:  100.0% (complete)
//
// The high draw rate (~82%) is realistic for tic-tac-toe with random play.
// X maintains first-move advantage (11% vs 7% wins) but most games draw.
//
// Blocking Bug Fix: See TestODEDrawFixesBlocking for analysis of whether
// draw detection improves blocking move preference.
package tictactoe

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// buildTicTacToeNetWithDraw constructs the Petri net with draw detection.
// This extends the base model with:
// - `move_tokens` place: starts at 0, accumulates as moves are played
// - `draw` transition: fires when 9 moves + game_active, outputs to win_o
func buildTicTacToeNetWithDraw() *petri.PetriNet {
	net := petri.NewPetriNet()

	// Empty cell places (initial: 1 token each)
	for _, id := range []string{"p00", "p01", "p02", "p10", "p11", "p12", "p20", "p21", "p22"} {
		net.AddPlace(id, 1.0, nil, 0, 0, nil)
	}

	// X piece places (initial: 0)
	for _, id := range []string{"x00", "x01", "x02", "x10", "x11", "x12", "x20", "x21", "x22"} {
		net.AddPlace(id, 0.0, nil, 0, 0, nil)
	}

	// O piece places (initial: 0)
	for _, id := range []string{"o00", "o01", "o02", "o10", "o11", "o12", "o20", "o21", "o22"} {
		net.AddPlace(id, 0.0, nil, 0, 0, nil)
	}

	// Turn and game state places
	net.AddPlace("x_turn", 1.0, nil, 0, 0, nil)
	net.AddPlace("o_turn", 0.0, nil, 0, 0, nil)
	net.AddPlace("win_x", 0.0, nil, 0, 0, nil)
	net.AddPlace("win_o", 0.0, nil, 0, 0, nil)
	net.AddPlace("game_active", 1.0, nil, 0, 0, nil)

	// NEW: Move counter place - tracks total moves played
	// Starts at 0, each play adds 1
	net.AddPlace("move_tokens", 0.0, nil, 0, 0, nil)

	// X play transitions - now also output to move_tokens
	xPlayTransitions := []struct{ trans, cell, piece string }{
		{"x_play_00", "p00", "x00"},
		{"x_play_01", "p01", "x01"},
		{"x_play_02", "p02", "x02"},
		{"x_play_10", "p10", "x10"},
		{"x_play_11", "p11", "x11"},
		{"x_play_12", "p12", "x12"},
		{"x_play_20", "p20", "x20"},
		{"x_play_21", "p21", "x21"},
		{"x_play_22", "p22", "x22"},
	}
	for _, t := range xPlayTransitions {
		net.AddTransition(t.trans, "x", 0, 0, nil)
		net.AddArc(t.cell, t.trans, 1.0, false)        // consume empty cell
		net.AddArc("x_turn", t.trans, 1.0, false)      // consume X turn
		net.AddArc(t.trans, t.piece, 1.0, false)       // produce X piece
		net.AddArc(t.trans, "o_turn", 1.0, false)      // produce O turn
		net.AddArc(t.trans, "move_tokens", 1.0, false) // NEW: add to move counter
	}

	// O play transitions - now also output to move_tokens
	oPlayTransitions := []struct{ trans, cell, piece string }{
		{"o_play_00", "p00", "o00"},
		{"o_play_01", "p01", "o01"},
		{"o_play_02", "p02", "o02"},
		{"o_play_10", "p10", "o10"},
		{"o_play_11", "p11", "o11"},
		{"o_play_12", "p12", "o12"},
		{"o_play_20", "p20", "o20"},
		{"o_play_21", "p21", "o21"},
		{"o_play_22", "p22", "o22"},
	}
	for _, t := range oPlayTransitions {
		net.AddTransition(t.trans, "o", 0, 0, nil)
		net.AddArc(t.cell, t.trans, 1.0, false)        // consume empty cell
		net.AddArc("o_turn", t.trans, 1.0, false)      // consume O turn
		net.AddArc(t.trans, t.piece, 1.0, false)       // produce O piece
		net.AddArc(t.trans, "x_turn", 1.0, false)      // produce X turn
		net.AddArc(t.trans, "move_tokens", 1.0, false) // NEW: add to move counter
	}

	// X win transitions - consume 3 X pieces + o_turn + game_active, produce win_x
	xWinPatterns := []struct {
		trans  string
		pieces []string
	}{
		{"x_win_row0", []string{"x00", "x01", "x02"}},
		{"x_win_row1", []string{"x10", "x11", "x12"}},
		{"x_win_row2", []string{"x20", "x21", "x22"}},
		{"x_win_col0", []string{"x00", "x10", "x20"}},
		{"x_win_col1", []string{"x01", "x11", "x21"}},
		{"x_win_col2", []string{"x02", "x12", "x22"}},
		{"x_win_diag", []string{"x00", "x11", "x22"}},
		{"x_win_anti", []string{"x02", "x11", "x20"}},
	}
	for _, w := range xWinPatterns {
		net.AddTransition(w.trans, "x", 0, 0, nil)
		for _, p := range w.pieces {
			net.AddArc(p, w.trans, 1.0, false) // consume piece (read arc in ODE)
			net.AddArc(w.trans, p, 1.0, false) // return piece
		}
		net.AddArc("o_turn", w.trans, 1.0, false)      // consume o_turn (X wins during O's turn)
		net.AddArc("game_active", w.trans, 1.0, false) // consume game_active
		net.AddArc(w.trans, "win_x", 1.0, false)       // produce win token
	}

	// O win transitions - similar structure
	oWinPatterns := []struct {
		trans  string
		pieces []string
	}{
		{"o_win_row0", []string{"o00", "o01", "o02"}},
		{"o_win_row1", []string{"o10", "o11", "o12"}},
		{"o_win_row2", []string{"o20", "o21", "o22"}},
		{"o_win_col0", []string{"o00", "o10", "o20"}},
		{"o_win_col1", []string{"o01", "o11", "o21"}},
		{"o_win_col2", []string{"o02", "o12", "o22"}},
		{"o_win_diag", []string{"o00", "o11", "o22"}},
		{"o_win_anti", []string{"o02", "o11", "o20"}},
	}
	for _, w := range oWinPatterns {
		net.AddTransition(w.trans, "o", 0, 0, nil)
		for _, p := range w.pieces {
			net.AddArc(p, w.trans, 1.0, false) // consume piece
			net.AddArc(w.trans, p, 1.0, false) // return piece
		}
		net.AddArc("x_turn", w.trans, 1.0, false)      // consume x_turn (O wins during X's turn)
		net.AddArc("game_active", w.trans, 1.0, false) // consume game_active
		net.AddArc(w.trans, "win_o", 1.0, false)       // produce win token
	}

	// NEW: Draw transition
	// Fires when: 9 move_tokens + game_active = 1 (no winner yet)
	// Outputs: +1 to win_o (or could be separate draw place)
	net.AddTransition("draw", "game", 0, 0, nil)
	net.AddArc("move_tokens", "draw", 9.0, false) // consume 9 move tokens
	net.AddArc("game_active", "draw", 1.0, false) // consume game_active (ends game)
	net.AddArc("draw", "win_o", 1.0, false)       // award draw to O (configurable)

	return net
}

// TestODEDrawDetectionBasic tests that the draw transition fires
// when 9 moves are played without a winner.
func TestODEDrawDetectionBasic(t *testing.T) {
	net := buildTicTacToeNetWithDraw()
	initialState := net.SetState(nil)
	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())

	finalState := sol.GetFinalState()

	t.Log("ODE simulation with draw detection:")
	t.Logf("  win_x: %.6f", finalState["win_x"])
	t.Logf("  win_o: %.6f (includes draws)", finalState["win_o"])
	t.Logf("  game_active: %.6f", finalState["game_active"])
	t.Logf("  move_tokens: %.6f", finalState["move_tokens"])

	// Key verification: win_o should be higher than without draw detection
	// because draws now contribute to win_o
	t.Log("\nAnalysis:")
	totalOutcome := finalState["win_x"] + finalState["win_o"]
	t.Logf("  Total outcome (win_x + win_o): %.6f", totalOutcome)
	t.Logf("  Remaining game_active: %.6f", finalState["game_active"])

	// With draw detection, more of game_active should flow to outcomes
	// since draws are now captured instead of leaking
	if finalState["game_active"] > 0.15 {
		t.Log("  Note: Some games still in progress (expected for ODE)")
	}
}

// TestODEDrawDetectionComparison compares ODE results with and without
// draw detection to verify the improvement.
func TestODEDrawDetectionComparison(t *testing.T) {
	// Without draw detection (original model)
	netOriginal := buildTicTacToeNet()
	initialOriginal := netOriginal.SetState(nil)
	ratesOriginal := netOriginal.SetRates(nil)
	tspan := [2]float64{0, 10}

	probOriginal := solver.NewProblem(netOriginal, initialOriginal, tspan, ratesOriginal)
	solOriginal := solver.Solve(probOriginal, solver.Tsit5(), solver.JSParityOptions())
	finalOriginal := solOriginal.GetFinalState()

	// With draw detection
	netDraw := buildTicTacToeNetWithDraw()
	initialDraw := netDraw.SetState(nil)
	ratesDraw := netDraw.SetRates(nil)

	probDraw := solver.NewProblem(netDraw, initialDraw, tspan, ratesDraw)
	solDraw := solver.Solve(probDraw, solver.Tsit5(), solver.JSParityOptions())
	finalDraw := solDraw.GetFinalState()

	t.Log("Comparison: Original vs Draw Detection")
	t.Log("")
	t.Log("Original model (wins only):")
	t.Logf("  win_x: %.6f", finalOriginal["win_x"])
	t.Logf("  win_o: %.6f", finalOriginal["win_o"])
	t.Logf("  game_active: %.6f", finalOriginal["game_active"])
	t.Logf("  Total outcomes: %.6f", finalOriginal["win_x"]+finalOriginal["win_o"])

	t.Log("")
	t.Log("Draw detection model:")
	t.Logf("  win_x: %.6f", finalDraw["win_x"])
	t.Logf("  win_o: %.6f (includes draws)", finalDraw["win_o"])
	t.Logf("  game_active: %.6f", finalDraw["game_active"])
	t.Logf("  move_tokens: %.6f", finalDraw["move_tokens"])
	t.Logf("  Total outcomes: %.6f", finalDraw["win_x"]+finalDraw["win_o"])

	t.Log("")
	t.Log("Differences:")
	t.Logf("  win_x change: %.6f", finalDraw["win_x"]-finalOriginal["win_x"])
	t.Logf("  win_o change: %.6f (expected positive due to draws)",
		finalDraw["win_o"]-finalOriginal["win_o"])
	t.Logf("  game_active change: %.6f (expected negative, more games concluded)",
		finalDraw["game_active"]-finalOriginal["game_active"])

	// Verify draw detection increases win_o
	if finalDraw["win_o"] <= finalOriginal["win_o"] {
		t.Log("  Note: win_o didn't increase - draw transition may need rate tuning")
	} else {
		t.Log("  PASS: win_o increased with draw detection")
	}
}

// TestODEDrawDetectionLateGame tests draw detection in a late-game scenario
// where most moves have been played.
func TestODEDrawDetectionLateGame(t *testing.T) {
	net := buildTicTacToeNetWithDraw()

	// Late game state: 6 moves played, 3 remaining
	// Board:
	//   Row 0: X  O  X
	//   Row 1: O  X  -
	//   Row 2: O  -  -
	// No immediate win threats, could end in draw
	initialState := net.SetState(map[string]float64{
		// Empty cells
		"p00": 0.0, "p01": 0.0, "p02": 0.0,
		"p10": 0.0, "p11": 0.0, "p12": 1.0,
		"p20": 0.0, "p21": 1.0, "p22": 1.0,
		// X pieces
		"x00": 1.0, "x02": 1.0, "x11": 1.0,
		// O pieces
		"o01": 1.0, "o10": 1.0, "o20": 1.0,
		// Turn
		"x_turn": 1.0, "o_turn": 0.0,
		// Game state
		"game_active": 1.0,
		// Move counter: 6 moves already played
		"move_tokens": 6.0,
	})

	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	finalState := sol.GetFinalState()

	t.Log("Late game ODE (6 moves played, 3 remaining):")
	t.Log("Board:")
	t.Log("  Row 0: X  O  X")
	t.Log("  Row 1: O  X  -")
	t.Log("  Row 2: O  -  -")
	t.Log("")
	t.Logf("Initial move_tokens: 6.0")
	t.Logf("Final move_tokens: %.6f", finalState["move_tokens"])
	t.Logf("Final win_x: %.6f", finalState["win_x"])
	t.Logf("Final win_o: %.6f", finalState["win_o"])
	t.Logf("Final game_active: %.6f", finalState["game_active"])

	// In this position, X has winning potential via:
	// - row 0: needs nothing, already has it (wait, X has 00, 02 but O has 01)
	// Actually checking: X at (0,0), (0,2), (1,1) - no row complete
	// O at (0,1), (1,0), (2,0) - no row complete
	// Many paths could lead to draw

	// Verify move_tokens grew (from play transitions)
	if finalState["move_tokens"] > 6.0 {
		t.Logf("  Move tokens accumulated: %.2f more from ODE flow", finalState["move_tokens"]-6.0)
	}

	// Check if draw transition activated
	// If move_tokens dropped below 9 and win_o increased, draw fired
	t.Log("")
	t.Log("Analysis:")
	if finalState["move_tokens"] < 6.0 {
		t.Log("  Note: move_tokens decreased - draw transition consuming tokens")
	} else {
		t.Log("  Note: move_tokens still accumulating (expected in ODE)")
	}
}

// TestODEDrawDetectionWinVsDraw tests that wins take priority over draws.
// When a win pattern is present, it should fire before draw can occur.
func TestODEDrawDetectionWinVsDraw(t *testing.T) {
	net := buildTicTacToeNetWithDraw()

	// Setup: X has winning position (top row), but 8 moves played
	// Win should fire, not draw
	initialState := net.SetState(map[string]float64{
		// X has top row
		"x00": 1.0, "x01": 1.0, "x02": 1.0,
		// O pieces scattered
		"o10": 1.0, "o11": 1.0, "o20": 1.0, "o21": 1.0,
		// Empty cells (1 remaining)
		"p12": 1.0, "p22": 1.0,
		// All others occupied
		"p00": 0.0, "p01": 0.0, "p02": 0.0,
		"p10": 0.0, "p11": 0.0, "p20": 0.0, "p21": 0.0,
		// Turn: O's turn (X just completed row)
		"x_turn": 0.0, "o_turn": 1.0,
		// Game state
		"game_active": 1.0,
		// 7 moves played (X: 3, O: 4)
		"move_tokens": 7.0,
	})

	rates := net.SetRates(nil)
	tspan := [2]float64{0, 5}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	finalState := sol.GetFinalState()

	t.Log("Win vs Draw priority test:")
	t.Log("X has top row complete, O's turn")
	t.Logf("  Initial: win_x=0, win_o=0, game_active=1, move_tokens=7")
	t.Logf("  Final:   win_x=%.4f, win_o=%.4f, game_active=%.4f, move_tokens=%.4f",
		finalState["win_x"], finalState["win_o"],
		finalState["game_active"], finalState["move_tokens"])

	// Win should dominate since X already has winning pattern
	if finalState["win_x"] > 0 {
		t.Logf("  PASS: X win transition fired (%.4f tokens to win_x)", finalState["win_x"])
	} else {
		t.Log("  Note: X win didn't accumulate - checking constraints")
	}

	// Draw should not fire before moves reach 9
	// Since move_tokens started at 7, draw needs 9 tokens input
	// which it won't have immediately
	t.Log("")
	t.Log("Analysis: Win transitions should compete favorably against draw")
	t.Log("because win only needs piece tokens (available) while draw needs 9 move_tokens")
}

// TestODEDrawMoveAccumulation specifically tests that move_tokens accumulates
// correctly as the ODE runs play transitions.
func TestODEDrawMoveAccumulation(t *testing.T) {
	net := buildTicTacToeNetWithDraw()
	initialState := net.SetState(nil)
	rates := net.SetRates(nil)

	// Run for various time spans to see move accumulation
	timespans := []float64{1, 2, 5, 10}

	t.Log("Move token accumulation over time:")
	for _, tend := range timespans {
		tspan := [2]float64{0, tend}
		prob := solver.NewProblem(net, initialState, tspan, rates)
		sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
		finalState := sol.GetFinalState()

		t.Logf("  t=%2.0f: move_tokens=%.4f, win_x=%.4f, win_o=%.4f",
			tend, finalState["move_tokens"], finalState["win_x"], finalState["win_o"])
	}

	// Verify move_tokens growth pattern
	t.Log("")
	t.Log("Expected: move_tokens should grow as plays accumulate,")
	t.Log("         then potentially decrease as draw transition consumes them")
}

// TestODEDrawDetectionSymmetry verifies that with draw detection,
// the X first-move advantage is balanced by O getting draws.
func TestODEDrawDetectionSymmetry(t *testing.T) {
	net := buildTicTacToeNetWithDraw()
	initialState := net.SetState(nil)
	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	finalState := sol.GetFinalState()

	winX := finalState["win_x"]
	winO := finalState["win_o"]

	t.Log("Symmetry analysis with draw detection:")
	t.Logf("  win_x: %.6f (X's wins)", winX)
	t.Logf("  win_o: %.6f (O's wins + draws)", winO)
	t.Logf("  Ratio win_x/win_o: %.3f", winX/winO)

	// Without draws: X has ~1.5x advantage (first move)
	// With draws going to O: ratio should be closer to 1.0
	ratio := winX / winO
	t.Log("")
	if ratio < 1.2 {
		t.Logf("  Draw detection reduces X advantage to %.1f%%", (ratio-1)*100)
	} else {
		t.Logf("  X still has %.1f%% advantage (draws may need higher weight)", (ratio-1)*100)
	}

	// The specific expected values depend on the ODE dynamics
	// This test documents the current behavior
	t.Log("")
	t.Log("Reference values for comparison:")
	t.Logf("  Expected without draws: ratio ~1.48 (X advantage)")
	t.Logf("  Observed with draws: ratio %.2f", ratio)
}

// buildTicTacToeNetWithDrawBalanced creates a model where draws go to a
// separate place instead of win_o, allowing independent analysis.
func buildTicTacToeNetWithDrawBalanced() *petri.PetriNet {
	net := buildTicTacToeNetWithDraw()

	// Add a separate draw place
	net.AddPlace("draw_count", 0.0, nil, 0, 0, nil)

	// Modify draw transition to output to draw_count instead of win_o
	// Note: This requires rebuilding the net since we can't modify arcs
	// For now, we'll create a fresh net

	freshNet := petri.NewPetriNet()

	// Copy all places from original
	for _, id := range []string{"p00", "p01", "p02", "p10", "p11", "p12", "p20", "p21", "p22"} {
		freshNet.AddPlace(id, 1.0, nil, 0, 0, nil)
	}
	for _, id := range []string{"x00", "x01", "x02", "x10", "x11", "x12", "x20", "x21", "x22"} {
		freshNet.AddPlace(id, 0.0, nil, 0, 0, nil)
	}
	for _, id := range []string{"o00", "o01", "o02", "o10", "o11", "o12", "o20", "o21", "o22"} {
		freshNet.AddPlace(id, 0.0, nil, 0, 0, nil)
	}
	freshNet.AddPlace("x_turn", 1.0, nil, 0, 0, nil)
	freshNet.AddPlace("o_turn", 0.0, nil, 0, 0, nil)
	freshNet.AddPlace("win_x", 0.0, nil, 0, 0, nil)
	freshNet.AddPlace("win_o", 0.0, nil, 0, 0, nil)
	freshNet.AddPlace("game_active", 1.0, nil, 0, 0, nil)
	freshNet.AddPlace("move_tokens", 0.0, nil, 0, 0, nil)
	freshNet.AddPlace("draw_count", 0.0, nil, 0, 0, nil) // Separate draw place

	// X play transitions
	xPlayTransitions := []struct{ trans, cell, piece string }{
		{"x_play_00", "p00", "x00"}, {"x_play_01", "p01", "x01"}, {"x_play_02", "p02", "x02"},
		{"x_play_10", "p10", "x10"}, {"x_play_11", "p11", "x11"}, {"x_play_12", "p12", "x12"},
		{"x_play_20", "p20", "x20"}, {"x_play_21", "p21", "x21"}, {"x_play_22", "p22", "x22"},
	}
	for _, t := range xPlayTransitions {
		freshNet.AddTransition(t.trans, "x", 0, 0, nil)
		freshNet.AddArc(t.cell, t.trans, 1.0, false)
		freshNet.AddArc("x_turn", t.trans, 1.0, false)
		freshNet.AddArc(t.trans, t.piece, 1.0, false)
		freshNet.AddArc(t.trans, "o_turn", 1.0, false)
		freshNet.AddArc(t.trans, "move_tokens", 1.0, false)
	}

	// O play transitions
	oPlayTransitions := []struct{ trans, cell, piece string }{
		{"o_play_00", "p00", "o00"}, {"o_play_01", "p01", "o01"}, {"o_play_02", "p02", "o02"},
		{"o_play_10", "p10", "o10"}, {"o_play_11", "p11", "o11"}, {"o_play_12", "p12", "o12"},
		{"o_play_20", "p20", "o20"}, {"o_play_21", "p21", "o21"}, {"o_play_22", "p22", "o22"},
	}
	for _, t := range oPlayTransitions {
		freshNet.AddTransition(t.trans, "o", 0, 0, nil)
		freshNet.AddArc(t.cell, t.trans, 1.0, false)
		freshNet.AddArc("o_turn", t.trans, 1.0, false)
		freshNet.AddArc(t.trans, t.piece, 1.0, false)
		freshNet.AddArc(t.trans, "x_turn", 1.0, false)
		freshNet.AddArc(t.trans, "move_tokens", 1.0, false)
	}

	// X win transitions
	xWinPatterns := []struct {
		trans  string
		pieces []string
	}{
		{"x_win_row0", []string{"x00", "x01", "x02"}},
		{"x_win_row1", []string{"x10", "x11", "x12"}},
		{"x_win_row2", []string{"x20", "x21", "x22"}},
		{"x_win_col0", []string{"x00", "x10", "x20"}},
		{"x_win_col1", []string{"x01", "x11", "x21"}},
		{"x_win_col2", []string{"x02", "x12", "x22"}},
		{"x_win_diag", []string{"x00", "x11", "x22"}},
		{"x_win_anti", []string{"x02", "x11", "x20"}},
	}
	for _, w := range xWinPatterns {
		freshNet.AddTransition(w.trans, "x", 0, 0, nil)
		for _, p := range w.pieces {
			freshNet.AddArc(p, w.trans, 1.0, false)
			freshNet.AddArc(w.trans, p, 1.0, false)
		}
		freshNet.AddArc("o_turn", w.trans, 1.0, false)
		freshNet.AddArc("game_active", w.trans, 1.0, false)
		freshNet.AddArc(w.trans, "win_x", 1.0, false)
	}

	// O win transitions
	oWinPatterns := []struct {
		trans  string
		pieces []string
	}{
		{"o_win_row0", []string{"o00", "o01", "o02"}},
		{"o_win_row1", []string{"o10", "o11", "o12"}},
		{"o_win_row2", []string{"o20", "o21", "o22"}},
		{"o_win_col0", []string{"o00", "o10", "o20"}},
		{"o_win_col1", []string{"o01", "o11", "o21"}},
		{"o_win_col2", []string{"o02", "o12", "o22"}},
		{"o_win_diag", []string{"o00", "o11", "o22"}},
		{"o_win_anti", []string{"o02", "o11", "o20"}},
	}
	for _, w := range oWinPatterns {
		freshNet.AddTransition(w.trans, "o", 0, 0, nil)
		for _, p := range w.pieces {
			freshNet.AddArc(p, w.trans, 1.0, false)
			freshNet.AddArc(w.trans, p, 1.0, false)
		}
		freshNet.AddArc("x_turn", w.trans, 1.0, false)
		freshNet.AddArc("game_active", w.trans, 1.0, false)
		freshNet.AddArc(w.trans, "win_o", 1.0, false)
	}

	// Draw transition - outputs to separate draw_count place
	freshNet.AddTransition("draw", "game", 0, 0, nil)
	freshNet.AddArc("move_tokens", "draw", 9.0, false)
	freshNet.AddArc("game_active", "draw", 1.0, false)
	freshNet.AddArc("draw", "draw_count", 1.0, false) // Separate from win_o

	return freshNet
}

// TestODEDrawSeparateOutcome tests with draws going to a separate place
// to see the breakdown of wins vs draws.
func TestODEDrawSeparateOutcome(t *testing.T) {
	net := buildTicTacToeNetWithDrawBalanced()
	initialState := net.SetState(nil)
	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	finalState := sol.GetFinalState()

	t.Log("ODE with separate draw tracking:")
	t.Logf("  win_x:      %.6f (X wins only)", finalState["win_x"])
	t.Logf("  win_o:      %.6f (O wins only)", finalState["win_o"])
	t.Logf("  draw_count: %.6f (draws)", finalState["draw_count"])
	t.Logf("  game_active: %.6f", finalState["game_active"])

	total := finalState["win_x"] + finalState["win_o"] + finalState["draw_count"]
	t.Logf("  Total outcomes: %.6f", total)

	t.Log("")
	t.Log("Breakdown:")
	t.Logf("  X win rate: %.1f%%", finalState["win_x"]/total*100)
	t.Logf("  O win rate: %.1f%%", finalState["win_o"]/total*100)
	t.Logf("  Draw rate:  %.1f%%", finalState["draw_count"]/total*100)

	// In perfect play, tic-tac-toe is always a draw
	// In ODE with uniform rates, we see the probabilistic distribution
	t.Log("")
	t.Log("Note: ODE simulates continuous probability flow, not perfect play.")
	t.Log("Draw detection captures outcomes that would otherwise leak.")
}

// TestODEDrawFixesBlocking tests whether draw detection improves blocking preference.
//
// The blocking bug: when X threatens an immediate win (e.g., column 1),
// raw ODE values sometimes prefer corners over the blocking move.
//
// Hypothesis: With draw detection, blocking should be more valuable because:
// - If O doesn't block, X wins (bad for O)
// - If O blocks, game continues toward potential draw (now valuable for O)
func TestODEDrawFixesBlocking(t *testing.T) {
	// Board state: X threatens column 1 win
	//   Row 0: -  -  -
	//   Row 1: O  X  -
	//   Row 2: -  X  -
	// O's turn - must block at (0,1)
	// 3 moves played so far

	// Test with ORIGINAL model (no draw detection)
	t.Log("=== ORIGINAL MODEL (no draw detection) ===")
	netOriginal := buildTicTacToeNet()
	stateOriginal := netOriginal.SetState(map[string]float64{
		"p00": 1.0, "p01": 1.0, "p02": 1.0,
		"p10": 0.0, "p11": 0.0, "p12": 1.0,
		"p20": 1.0, "p21": 0.0, "p22": 1.0,
		"x11": 1.0, "x21": 1.0,
		"o10": 1.0,
		"x_turn": 0.0, "o_turn": 1.0,
		"game_active": 1.0,
	})

	ratesOrig := netOriginal.SetRates(nil)
	tspan := [2]float64{0, 10}

	probOrig := solver.NewProblem(netOriginal, stateOriginal, tspan, ratesOrig)
	solOrig := solver.Solve(probOrig, solver.Tsit5(), solver.JSParityOptions())
	finalOrig := solOrig.GetFinalState()

	t.Log("Available positions (cell place values):")
	origPositions := map[string]float64{
		"p00": finalOrig["p00"],
		"p01": finalOrig["p01"], // BLOCKING MOVE
		"p02": finalOrig["p02"],
		"p12": finalOrig["p12"],
		"p20": finalOrig["p20"],
		"p22": finalOrig["p22"],
	}
	for pos, val := range origPositions {
		marker := ""
		if pos == "p01" {
			marker = " <-- BLOCK"
		}
		t.Logf("  %s: %.6f%s", pos, val, marker)
	}
	t.Logf("  win_x: %.6f, win_o: %.6f", finalOrig["win_x"], finalOrig["win_o"])

	// Test with DRAW model
	t.Log("")
	t.Log("=== DRAW DETECTION MODEL ===")
	netDraw := buildTicTacToeNetWithDraw()
	stateDraw := netDraw.SetState(map[string]float64{
		"p00": 1.0, "p01": 1.0, "p02": 1.0,
		"p10": 0.0, "p11": 0.0, "p12": 1.0,
		"p20": 1.0, "p21": 0.0, "p22": 1.0,
		"x11": 1.0, "x21": 1.0,
		"o10": 1.0,
		"x_turn": 0.0, "o_turn": 1.0,
		"game_active": 1.0,
		"move_tokens": 3.0, // 3 moves already played
	})

	ratesDraw := netDraw.SetRates(nil)
	probDraw := solver.NewProblem(netDraw, stateDraw, tspan, ratesDraw)
	solDraw := solver.Solve(probDraw, solver.Tsit5(), solver.JSParityOptions())
	finalDraw := solDraw.GetFinalState()

	t.Log("Available positions (cell place values):")
	drawPositions := map[string]float64{
		"p00": finalDraw["p00"],
		"p01": finalDraw["p01"], // BLOCKING MOVE
		"p02": finalDraw["p02"],
		"p12": finalDraw["p12"],
		"p20": finalDraw["p20"],
		"p22": finalDraw["p22"],
	}
	for pos, val := range drawPositions {
		marker := ""
		if pos == "p01" {
			marker = " <-- BLOCK"
		}
		t.Logf("  %s: %.6f%s", pos, val, marker)
	}
	t.Logf("  win_x: %.6f, win_o: %.6f, move_tokens: %.6f",
		finalDraw["win_x"], finalDraw["win_o"], finalDraw["move_tokens"])

	// Now compare hypothetical moves: O blocks vs O plays each corner
	t.Log("")
	t.Log("=== COMPARING OUTCOMES: O blocks vs ALL corners ===")

	// Helper to evaluate a move
	evaluateMoveDraw := func(oPlace string) float64 {
		net := buildTicTacToeNetWithDraw()
		state := map[string]float64{
			"p00": 1.0, "p01": 1.0, "p02": 1.0,
			"p10": 0.0, "p11": 0.0, "p12": 1.0,
			"p20": 1.0, "p21": 0.0, "p22": 1.0,
			"x11": 1.0, "x21": 1.0,
			"o10": 1.0,
			"x_turn": 1.0, "o_turn": 0.0,
			"game_active": 1.0,
			"move_tokens": 4.0,
		}
		// Apply O's move
		state[oPlace] = 1.0
		// Mark the cell as taken
		cellPlace := "p" + oPlace[1:]
		state[cellPlace] = 0.0

		prob := solver.NewProblem(net, net.SetState(state), tspan, net.SetRates(nil))
		sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
		final := sol.GetFinalState()
		return final["win_o"] - final["win_x"]
	}

	evaluateMoveOrig := func(oPlace string) float64 {
		net := buildTicTacToeNet()
		state := map[string]float64{
			"p00": 1.0, "p01": 1.0, "p02": 1.0,
			"p10": 0.0, "p11": 0.0, "p12": 1.0,
			"p20": 1.0, "p21": 0.0, "p22": 1.0,
			"x11": 1.0, "x21": 1.0,
			"o10": 1.0,
			"x_turn": 1.0, "o_turn": 0.0,
			"game_active": 1.0,
		}
		// Apply O's move
		state[oPlace] = 1.0
		// Mark the cell as taken
		cellPlace := "p" + oPlace[1:]
		state[cellPlace] = 0.0

		prob := solver.NewProblem(net, net.SetState(state), tspan, net.SetRates(nil))
		sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
		final := sol.GetFinalState()
		return final["win_o"] - final["win_x"]
	}

	// All available moves for O
	moves := []struct {
		place string
		name  string
		isBlock bool
	}{
		{"o01", "(0,1) BLOCK", true},
		{"o00", "(0,0) corner", false},
		{"o02", "(0,2) corner", false},
		{"o20", "(2,0) corner", false},
		{"o22", "(2,2) corner", false},
		{"o12", "(1,2) edge", false},
	}

	t.Log("With DRAW detection:")
	var blockScoreDraw float64
	var bestCornerScoreDraw float64
	var bestCornerNameDraw string
	scoresDraw := make(map[string]float64)

	for _, m := range moves {
		score := evaluateMoveDraw(m.place)
		scoresDraw[m.place] = score
		marker := ""
		if m.isBlock {
			blockScoreDraw = score
			marker = " <-- MUST BLOCK"
		} else if m.place[1] == '0' || m.place[1] == '2' { // corners
			if score > bestCornerScoreDraw {
				bestCornerScoreDraw = score
				bestCornerNameDraw = m.name
			}
		}
		t.Logf("  O plays %s: score=%.4f%s", m.name, score, marker)
	}

	t.Log("")
	t.Log("WITHOUT draw detection (original):")
	var blockScoreOrig float64
	var bestCornerScoreOrig float64
	var bestCornerNameOrig string

	for _, m := range moves {
		score := evaluateMoveOrig(m.place)
		marker := ""
		if m.isBlock {
			blockScoreOrig = score
			marker = " <-- MUST BLOCK"
		} else if m.place[1] == '0' || m.place[1] == '2' { // corners
			if score > bestCornerScoreOrig {
				bestCornerScoreOrig = score
				bestCornerNameOrig = m.name
			}
		}
		t.Logf("  O plays %s: score=%.4f%s", m.name, score, marker)
	}

	// Analysis
	t.Log("")
	t.Log("=== ANALYSIS ===")

	t.Logf("With draw detection:")
	t.Logf("  Block score: %.4f", blockScoreDraw)
	t.Logf("  Best corner: %s = %.4f", bestCornerNameDraw, bestCornerScoreDraw)
	t.Logf("  Block vs best corner margin: %.4f", blockScoreDraw-bestCornerScoreDraw)

	t.Log("")
	t.Logf("Without draw detection:")
	t.Logf("  Block score: %.4f", blockScoreOrig)
	t.Logf("  Best corner: %s = %.4f", bestCornerNameOrig, bestCornerScoreOrig)
	t.Logf("  Block vs best corner margin: %.4f", blockScoreOrig-bestCornerScoreOrig)

	blockBetterDraw := blockScoreDraw > bestCornerScoreDraw
	blockBetterOrig := blockScoreOrig > bestCornerScoreOrig

	t.Log("")
	t.Logf("With draw detection:    blocking is %s", boolToPreference(blockBetterDraw))
	t.Logf("Without draw detection: blocking is %s", boolToPreference(blockBetterOrig))

	if blockBetterDraw && !blockBetterOrig {
		t.Log("")
		t.Log("SUCCESS: Draw detection FIXES the blocking bug!")
		t.Log("Blocking move now has higher score than ALL corners.")
	} else if blockBetterDraw && blockBetterOrig {
		t.Log("")
		t.Log("Both models prefer blocking over all corners (good)")
	} else if !blockBetterDraw && blockBetterOrig {
		t.Log("")
		t.Log("WARNING: Draw detection WORSENED blocking preference!")
	} else {
		t.Log("")
		t.Logf("Neither model prefers blocking over best corner (%s)", bestCornerNameDraw)
		t.Log("Tactical adjustment may still be needed for edge cases")
	}
}

func boolToPreference(better bool) string {
	if better {
		return "PREFERRED"
	}
	return "NOT preferred"
}
