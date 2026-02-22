package zkode

import (
	"fmt"
	"math"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// Board represents a tic-tac-toe board state.
// Empty cells are "", player cells are "X" or "O".
type Board [3][3]string

// MoveScore holds the ODE evaluation result for a single move.
type MoveScore struct {
	Row      int     `json:"row"`
	Col      int     `json:"col"`
	RawScore float64 `json:"raw_score"`
	Adjusted float64 `json:"adjusted"`
}

// EvaluationResult holds the full evaluation for a board position.
type EvaluationResult struct {
	Scores  []MoveScore `json:"scores"`
	Player  string      `json:"player"`
	Optimal *MoveScore  `json:"optimal"`
}

// BuildTicTacToeNet constructs the full 33-place Petri net for TTT ODE simulation.
// Places: 9 empty cells (p00-p22), 9 X pieces (x00-x22), 9 O pieces (o00-o22),
// plus x_turn, o_turn, win_x, win_o, game_active.
func BuildTicTacToeNet() *petri.PetriNet {
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
	net.AddPlace("move_tokens", 0.0, nil, 0, 0, nil)

	// X play transitions
	xPlayTransitions := []struct{ trans, cell, piece string }{
		{"x_play_00", "p00", "x00"}, {"x_play_01", "p01", "x01"}, {"x_play_02", "p02", "x02"},
		{"x_play_10", "p10", "x10"}, {"x_play_11", "p11", "x11"}, {"x_play_12", "p12", "x12"},
		{"x_play_20", "p20", "x20"}, {"x_play_21", "p21", "x21"}, {"x_play_22", "p22", "x22"},
	}
	for _, t := range xPlayTransitions {
		net.AddTransition(t.trans, "x", 0, 0, nil)
		net.AddArc(t.cell, t.trans, 1.0, false)
		net.AddArc("x_turn", t.trans, 1.0, false)
		net.AddArc(t.trans, t.piece, 1.0, false)
		net.AddArc(t.trans, "o_turn", 1.0, false)
		net.AddArc(t.trans, "move_tokens", 1.0, false)
	}

	// O play transitions
	oPlayTransitions := []struct{ trans, cell, piece string }{
		{"o_play_00", "p00", "o00"}, {"o_play_01", "p01", "o01"}, {"o_play_02", "p02", "o02"},
		{"o_play_10", "p10", "o10"}, {"o_play_11", "p11", "o11"}, {"o_play_12", "p12", "o12"},
		{"o_play_20", "p20", "o20"}, {"o_play_21", "p21", "o21"}, {"o_play_22", "p22", "o22"},
	}
	for _, t := range oPlayTransitions {
		net.AddTransition(t.trans, "o", 0, 0, nil)
		net.AddArc(t.cell, t.trans, 1.0, false)
		net.AddArc("o_turn", t.trans, 1.0, false)
		net.AddArc(t.trans, t.piece, 1.0, false)
		net.AddArc(t.trans, "x_turn", 1.0, false)
		net.AddArc(t.trans, "move_tokens", 1.0, false)
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
		net.AddTransition(w.trans, "x", 0, 0, nil)
		for _, p := range w.pieces {
			net.AddArc(p, w.trans, 1.0, false)
			net.AddArc(w.trans, p, 1.0, false)
		}
		net.AddArc("o_turn", w.trans, 1.0, false)
		net.AddArc("game_active", w.trans, 1.0, false)
		net.AddArc(w.trans, "win_x", 1.0, false)
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
		net.AddTransition(w.trans, "o", 0, 0, nil)
		for _, p := range w.pieces {
			net.AddArc(p, w.trans, 1.0, false)
			net.AddArc(w.trans, p, 1.0, false)
		}
		net.AddArc("x_turn", w.trans, 1.0, false)
		net.AddArc("game_active", w.trans, 1.0, false)
		net.AddArc(w.trans, "win_o", 1.0, false)
	}

	// Draw transition: consumes 9 move_tokens + game_active, produces win_o
	net.AddTransition("draw", "", 0, 0, nil)
	net.AddArc("move_tokens", "draw", 9.0, false)
	net.AddArc("game_active", "draw", 1.0, false)
	net.AddArc("draw", "win_o", 1.0, false)

	return net
}

// BoardToState converts a Board + player turn into an ODE initial state.
func BoardToState(board Board, currentPlayer string) map[string]float64 {
	state := make(map[string]float64)

	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			pKey := fmt.Sprintf("p%d%d", r, c)
			xKey := fmt.Sprintf("x%d%d", r, c)
			oKey := fmt.Sprintf("o%d%d", r, c)

			state[pKey] = 1.0
			state[xKey] = 0.0
			state[oKey] = 0.0

			if board[r][c] == "X" {
				state[pKey] = 0.0
				state[xKey] = 1.0
			} else if board[r][c] == "O" {
				state[pKey] = 0.0
				state[oKey] = 1.0
			}
		}
	}

	if currentPlayer == "X" {
		state["x_turn"] = 1.0
		state["o_turn"] = 0.0
	} else {
		state["x_turn"] = 0.0
		state["o_turn"] = 1.0
	}

	state["game_active"] = 1.0
	state["win_x"] = 0.0
	state["win_o"] = 0.0

	// Count pieces placed to set move_tokens
	moveCount := 0.0
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == "X" || board[r][c] == "O" {
				moveCount++
			}
		}
	}
	state["move_tokens"] = moveCount

	return state
}

// buildStateWithMove creates an ODE initial state with a hypothetical move applied.
func buildStateWithMove(board Board, player string, row, col int) map[string]float64 {
	hypothetical := board
	hypothetical[row][col] = player
	// After move, it's the opponent's turn
	opponent := "O"
	if player == "O" {
		opponent = "X"
	}
	return BoardToState(hypothetical, opponent)
}

// EvaluatePosition runs ODE simulation for a board state and returns the
// objective value (win_current - win_opponent).
func EvaluatePosition(board Board, player string) float64 {
	net := BuildTicTacToeNet()
	state := BoardToState(board, player)
	rates := net.SetRates(nil)
	tspan := [2]float64{0, 10}

	prob := solver.NewProblem(net, state, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.GameAIOptions())
	finalState := sol.GetFinalState()

	winCurrent := "win_x"
	winOpponent := "win_o"
	if player == "O" {
		winCurrent = "win_o"
		winOpponent = "win_x"
	}

	return finalState[winCurrent] - finalState[winOpponent]
}

// EvaluateAllMoves computes ODE scores for all empty positions on the board.
// Returns raw scores plus tactical adjustments for immediate wins/blocks.
func EvaluateAllMoves(board Board, player string) *EvaluationResult {
	opponent := "O"
	if player == "O" {
		opponent = "X"
	}

	// Find empty positions
	var positions [][2]int
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == "" {
				positions = append(positions, [2]int{r, c})
			}
		}
	}

	if len(positions) == 0 {
		return &EvaluationResult{Player: player}
	}

	// Step 1: Compute raw ODE values for each position
	scores := make([]MoveScore, len(positions))
	for i, pos := range positions {
		r, c := pos[0], pos[1]
		net := BuildTicTacToeNet()
		hypotheticalState := buildStateWithMove(board, player, r, c)
		rates := net.SetRates(nil)
		tspan := [2]float64{0, 10}

		prob := solver.NewProblem(net, hypotheticalState, tspan, rates)
		sol := solver.Solve(prob, solver.Tsit5(), solver.GameAIOptions())
		finalState := sol.GetFinalState()

		winCurrent := "win_x"
		winOpponent := "win_o"
		if player == "O" {
			winCurrent = "win_o"
			winOpponent = "win_x"
		}

		score := finalState[winCurrent] - finalState[winOpponent]
		scores[i] = MoveScore{
			Row:      r,
			Col:      c,
			RawScore: score,
			Adjusted: score,
		}
	}

	// Step 2: Calculate tactical adjustment factors
	maxAbs := 0.0
	for _, s := range scores {
		if abs := math.Abs(s.RawScore); abs > maxAbs {
			maxAbs = abs
		}
	}
	winBonus := maxAbs * 3
	if winBonus < 1 {
		winBonus = 1
	}
	lossPenalty := winBonus

	// Step 3: Apply tactical adjustments
	for i, s := range scores {
		testBoard := board
		testBoard[s.Row][s.Col] = player

		if IsWinningBoard(testBoard, player) {
			scores[i].Adjusted += winBonus
		} else {
			opponentWins := GetImmediateWinningMoves(testBoard, opponent)
			if len(opponentWins) > 0 {
				scores[i].Adjusted -= lossPenalty
			}
		}
	}

	// Find optimal move (highest adjusted score)
	var optimal *MoveScore
	for i := range scores {
		if optimal == nil || scores[i].Adjusted > optimal.Adjusted {
			optimal = &scores[i]
		}
	}

	return &EvaluationResult{
		Scores:  scores,
		Player:  player,
		Optimal: optimal,
	}
}

// FindOptimalMove returns the best move for the current player.
func FindOptimalMove(board Board, player string) (row, col int, score float64) {
	result := EvaluateAllMoves(board, player)
	if result.Optimal == nil {
		return -1, -1, 0
	}
	return result.Optimal.Row, result.Optimal.Col, result.Optimal.Adjusted
}

// IsWinningBoard checks if a player has won.
func IsWinningBoard(board Board, player string) bool {
	for r := 0; r < 3; r++ {
		if board[r][0] == player && board[r][1] == player && board[r][2] == player {
			return true
		}
	}
	for c := 0; c < 3; c++ {
		if board[0][c] == player && board[1][c] == player && board[2][c] == player {
			return true
		}
	}
	if board[0][0] == player && board[1][1] == player && board[2][2] == player {
		return true
	}
	if board[0][2] == player && board[1][1] == player && board[2][0] == player {
		return true
	}
	return false
}

// GetImmediateWinningMoves returns positions where a player can win immediately.
func GetImmediateWinningMoves(board Board, player string) [][2]int {
	var wins [][2]int
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] != "" {
				continue
			}
			board[r][c] = player
			if IsWinningBoard(board, player) {
				wins = append(wins, [2]int{r, c})
			}
			board[r][c] = ""
		}
	}
	return wins
}
