package tictactoe

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/api"
)

// HeatmapRequest represents a board state for heatmap calculation
type HeatmapRequest struct {
	Board [3][3]string `json:"board"` // "X", "O", or ""
}

// HeatmapResponse contains ODE-computed strategic values
type HeatmapResponse struct {
	Values  map[string]float64            `json:"values"`
	Details map[string]map[string]float64 `json:"details,omitempty"`
	Player  string                        `json:"current_player"`
}

// HandleHeatmap computes ODE-based strategic values for a board state
func HandleHeatmap() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req HeatmapRequest

		// If POST with body, use provided board; otherwise compute for empty board
		if r.Method == http.MethodPost && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				api.Error(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
				return
			}
		}

		// Compute heatmap
		response := computeHeatmap(req.Board)
		api.JSON(w, http.StatusOK, response)
	}
}

// computeHeatmap runs ODE simulation for each empty position
func computeHeatmap(board [3][3]string) HeatmapResponse {
	model := buildODENet()
	values := make(map[string]float64)
	details := make(map[string]map[string]float64)

	// Determine current player
	xCount, oCount := 0, 0
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == "X" {
				xCount++
			} else if board[r][c] == "O" {
				oCount++
			}
		}
	}
	currentPlayer := "X"
	if xCount > oCount {
		currentPlayer = "O"
	}

	// For each empty position, compute strategic value
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			key := fmt.Sprintf("%d%d", row, col)

			if board[row][col] != "" {
				values[key] = 0
				continue
			}

			// Create hypothetical state
			hypState := buildInitialState(model, board)

			// Apply hypothetical move
			posID := fmt.Sprintf("P%d%d", row, col)
			histID := fmt.Sprintf("%s%d%d", currentPlayer, row, col)
			hypState[posID] = 0
			hypState[histID] = 1

			// Set turn: after current player moves, it's opponent's turn
			if currentPlayer == "X" {
				hypState["Next"] = 1 // O's turn
			} else {
				hypState["Next"] = 0 // X's turn
			}

			// Run ODE
			prob := solver.NewProblem(model.Net, hypState, [2]float64{0, 3.0}, model.Rates)
			opts := solver.FastOptions()
			sol := solver.Solve(prob, solver.Tsit5(), opts)
			finalState := sol.GetFinalState()

			winX := finalState["WinX"]
			winO := finalState["WinO"]

			// Score from current player's perspective
			var score float64
			if currentPlayer == "X" {
				score = winX - winO
			} else {
				score = winO - winX
			}

			values[key] = score
			details[key] = map[string]float64{
				"WinX":  winX,
				"WinO":  winO,
				"score": score,
			}
		}
	}

	return HeatmapResponse{
		Values:  values,
		Details: details,
		Player:  currentPlayer,
	}
}

// ODEModel holds the Petri net and rates for ODE simulation
type ODEModel struct {
	Net   *petri.PetriNet
	Rates map[string]float64
}

// buildODENet creates the Petri net for ODE simulation
func buildODENet() *ODEModel {
	net := &petri.PetriNet{
		Places:      make(map[string]*petri.Place),
		Transitions: make(map[string]*petri.Transition),
		Arcs:        []*petri.Arc{},
	}

	// Board position places (1 = empty)
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			id := fmt.Sprintf("P%d%d", row, col)
			net.Places[id] = petri.NewPlace(id, 1, 0, 0, 0, nil)
		}
	}

	// X history places (0 = not played)
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			id := fmt.Sprintf("X%d%d", row, col)
			net.Places[id] = petri.NewPlace(id, 0, 0, 0, 0, nil)
		}
	}

	// O history places
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			id := fmt.Sprintf("O%d%d", row, col)
			net.Places[id] = petri.NewPlace(id, 0, 0, 0, 0, nil)
		}
	}

	// Turn control and win places
	net.Places["Next"] = petri.NewPlace("Next", 0, 0, 0, 0, nil) // 0 = X's turn
	net.Places["WinX"] = petri.NewPlace("WinX", 0, 0, 0, 0, nil)
	net.Places["WinO"] = petri.NewPlace("WinO", 0, 0, 0, 0, nil)

	// X move transitions: P -> PlayX -> X + Next
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			tid := fmt.Sprintf("PlayX%d%d", row, col)
			net.Transitions[tid] = petri.NewTransition(tid, "", 0, 0, nil)

			posID := fmt.Sprintf("P%d%d", row, col)
			histID := fmt.Sprintf("X%d%d", row, col)

			net.Arcs = append(net.Arcs,
				petri.NewArc(posID, tid, 1, false),
				petri.NewArc(tid, histID, 1, false),
				petri.NewArc(tid, "Next", 1, false),
			)
		}
	}

	// O move transitions: Next + P -> PlayO -> O
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			tid := fmt.Sprintf("PlayO%d%d", row, col)
			net.Transitions[tid] = petri.NewTransition(tid, "", 0, 0, nil)

			posID := fmt.Sprintf("P%d%d", row, col)
			histID := fmt.Sprintf("O%d%d", row, col)

			net.Arcs = append(net.Arcs,
				petri.NewArc("Next", tid, 1, false),
				petri.NewArc(posID, tid, 1, false),
				petri.NewArc(tid, histID, 1, false),
			)
		}
	}

	// Win patterns
	winPatterns := [][]int{
		{0, 1, 2}, // Row 0
		{3, 4, 5}, // Row 1
		{6, 7, 8}, // Row 2
		{0, 3, 6}, // Col 0
		{1, 4, 7}, // Col 1
		{2, 5, 8}, // Col 2
		{0, 4, 8}, // Diag
		{2, 4, 6}, // Anti-diag
	}
	patternNames := []string{"Row0", "Row1", "Row2", "Col0", "Col1", "Col2", "Dg0", "Dg1"}

	// X win transitions
	for i, pattern := range winPatterns {
		tid := fmt.Sprintf("X%s", patternNames[i])
		net.Transitions[tid] = petri.NewTransition(tid, "", 0, 0, nil)

		for _, idx := range pattern {
			row, col := idx/3, idx%3
			histID := fmt.Sprintf("X%d%d", row, col)
			net.Arcs = append(net.Arcs, petri.NewArc(histID, tid, 1, false))
		}
		net.Arcs = append(net.Arcs, petri.NewArc(tid, "WinX", 1, false))
	}

	// O win transitions
	for i, pattern := range winPatterns {
		tid := fmt.Sprintf("O%s", patternNames[i])
		net.Transitions[tid] = petri.NewTransition(tid, "", 0, 0, nil)

		for _, idx := range pattern {
			row, col := idx/3, idx%3
			histID := fmt.Sprintf("O%d%d", row, col)
			net.Arcs = append(net.Arcs, petri.NewArc(histID, tid, 1, false))
		}
		net.Arcs = append(net.Arcs, petri.NewArc(tid, "WinO", 1, false))
	}

	// Default rates (all 1.0)
	rates := make(map[string]float64)
	for tid := range net.Transitions {
		rates[tid] = 1.0
	}

	return &ODEModel{Net: net, Rates: rates}
}

// buildInitialState creates initial state from board
func buildInitialState(model *ODEModel, board [3][3]string) map[string]float64 {
	state := model.Net.SetState(nil)

	// Set board positions and history based on current board
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			posID := fmt.Sprintf("P%d%d", row, col)
			xHistID := fmt.Sprintf("X%d%d", row, col)
			oHistID := fmt.Sprintf("O%d%d", row, col)

			switch board[row][col] {
			case "X":
				state[posID] = 0
				state[xHistID] = 1
				state[oHistID] = 0
			case "O":
				state[posID] = 0
				state[xHistID] = 0
				state[oHistID] = 1
			default:
				state[posID] = 1
				state[xHistID] = 0
				state[oHistID] = 0
			}
		}
	}

	// Set turn based on piece count
	xCount, oCount := 0, 0
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == "X" {
				xCount++
			} else if board[r][c] == "O" {
				oCount++
			}
		}
	}
	if xCount > oCount {
		state["Next"] = 1 // O's turn
	} else {
		state["Next"] = 0 // X's turn
	}

	state["WinX"] = 0
	state["WinO"] = 0

	return state
}
