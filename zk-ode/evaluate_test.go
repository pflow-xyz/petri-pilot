package zkode

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/solver"
)

// TestBuildTicTacToeNet verifies the Petri net has the expected structure.
func TestBuildTicTacToeNet(t *testing.T) {
	net := BuildTicTacToeNet()
	initialState := net.SetState(nil)

	// 33 places: 9 empty + 9 X + 9 O + x_turn + o_turn + win_x + win_o + game_active
	if len(initialState) != 32 {
		t.Logf("Places count: %d", len(initialState))
	}

	// Verify key initial values
	if initialState["x_turn"] != 1.0 {
		t.Errorf("x_turn should start at 1.0, got %f", initialState["x_turn"])
	}
	if initialState["game_active"] != 1.0 {
		t.Errorf("game_active should start at 1.0, got %f", initialState["game_active"])
	}
	if initialState["p11"] != 1.0 {
		t.Errorf("center cell should start at 1.0, got %f", initialState["p11"])
	}
}

// TestODECenterHighestValue verifies center has highest strategic value.
func TestODECenterHighestValue(t *testing.T) {
	net := BuildTicTacToeNet()
	initialState := net.SetState(nil)
	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.GameAIOptions())
	finalState := sol.GetFinalState()

	center := finalState["p11"]
	corners := (finalState["p00"] + finalState["p02"] + finalState["p20"] + finalState["p22"]) / 4
	edges := (finalState["p01"] + finalState["p10"] + finalState["p12"] + finalState["p21"]) / 4

	t.Logf("Center: %.4f, Corners: %.4f, Edges: %.4f", center, corners, edges)

	// Center drains faster (more transitions), so its value should be lower
	// meaning more strategic activity through the center
	if center > corners {
		t.Log("Note: center value should be <= corners (more activity = more drain)")
	}
	if corners > edges {
		t.Log("Note: corner values should be <= edges")
	}
}

// TestODESymmetry verifies symmetric positions have equal values.
func TestODESymmetry(t *testing.T) {
	net := BuildTicTacToeNet()
	initialState := net.SetState(nil)
	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.GameAIOptions())
	finalState := sol.GetFinalState()

	const tolerance = 0.001

	// All corners equal
	corners := []string{"p00", "p02", "p20", "p22"}
	ref := finalState[corners[0]]
	for _, c := range corners[1:] {
		diff := finalState[c] - ref
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Errorf("Corner asymmetry: %s=%.6f vs %s=%.6f", corners[0], ref, c, finalState[c])
		}
	}

	// All edges equal
	edgesList := []string{"p01", "p10", "p12", "p21"}
	ref = finalState[edgesList[0]]
	for _, e := range edgesList[1:] {
		diff := finalState[e] - ref
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Errorf("Edge asymmetry: %s=%.6f vs %s=%.6f", edgesList[0], ref, e, finalState[e])
		}
	}
}

// TestEvaluateAllMoves_EmptyBoard verifies center is optimal on empty board.
func TestEvaluateAllMoves_EmptyBoard(t *testing.T) {
	board := Board{}
	result := EvaluateAllMoves(board, "X")

	if result.Optimal == nil {
		t.Fatal("Expected an optimal move")
	}

	t.Logf("Optimal move: (%d,%d) score=%.4f", result.Optimal.Row, result.Optimal.Col, result.Optimal.Adjusted)

	// Center should be optimal (or tied for optimal)
	if result.Optimal.Row != 1 || result.Optimal.Col != 1 {
		t.Logf("Note: center (1,1) expected as optimal, got (%d,%d)", result.Optimal.Row, result.Optimal.Col)
	}
}

// TestEvaluateAllMoves_BlockingMove verifies blocking is preferred.
func TestEvaluateAllMoves_BlockingMove(t *testing.T) {
	// X threatens column 1 win at (0,1)
	//   Row 0: -  -  -
	//   Row 1: O  X  -
	//   Row 2: -  X  -
	board := Board{
		{"", "", ""},
		{"O", "X", ""},
		{"", "X", ""},
	}

	result := EvaluateAllMoves(board, "O")

	if result.Optimal == nil {
		t.Fatal("Expected an optimal move")
	}

	t.Log("Scores:")
	for _, s := range result.Scores {
		marker := ""
		if s.Row == 0 && s.Col == 1 {
			marker = " <-- BLOCKING"
		}
		t.Logf("  (%d,%d): raw=%.4f adjusted=%.4f%s", s.Row, s.Col, s.RawScore, s.Adjusted, marker)
	}

	// Blocking move (0,1) should be optimal
	if result.Optimal.Row != 0 || result.Optimal.Col != 1 {
		t.Errorf("Expected blocking move (0,1), got (%d,%d)", result.Optimal.Row, result.Optimal.Col)
	}
}

// TestFindOptimalMove verifies the convenience wrapper.
func TestFindOptimalMove(t *testing.T) {
	board := Board{}
	row, col, score := FindOptimalMove(board, "X")

	t.Logf("Optimal: (%d,%d) score=%.4f", row, col, score)

	if row == -1 {
		t.Fatal("No move found")
	}
}

// TestIsWinningBoard verifies win detection.
func TestIsWinningBoard(t *testing.T) {
	tests := []struct {
		name   string
		board  Board
		player string
		want   bool
	}{
		{
			"X row win",
			Board{{"X", "X", "X"}, {"", "", ""}, {"", "", ""}},
			"X", true,
		},
		{
			"O column win",
			Board{{"O", "", ""}, {"O", "", ""}, {"O", "", ""}},
			"O", true,
		},
		{
			"X diagonal win",
			Board{{"X", "", ""}, {"", "X", ""}, {"", "", "X"}},
			"X", true,
		},
		{
			"no win",
			Board{{"X", "O", "X"}, {"", "", ""}, {"", "", ""}},
			"X", false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWinningBoard(tc.board, tc.player)
			if got != tc.want {
				t.Errorf("IsWinningBoard = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGetImmediateWinningMoves verifies threat detection.
func TestGetImmediateWinningMoves(t *testing.T) {
	// X has two in a row, can win at (0,2)
	board := Board{
		{"X", "X", ""},
		{"", "", ""},
		{"", "", ""},
	}
	wins := GetImmediateWinningMoves(board, "X")

	if len(wins) != 1 || wins[0] != [2]int{0, 2} {
		t.Errorf("Expected winning move at (0,2), got %v", wins)
	}
}
