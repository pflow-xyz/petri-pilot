package tictactoe

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// TicTacToeModel represents the Petri net for ODE simulation
type TicTacToeModel struct {
	Net   *petri.PetriNet
	Rates map[string]float64
}

// BuildTicTacToeNet creates a Petri net matching the go-pflow structure
// Places: P00-P22 (board), X00-X22 (X history), O00-O22 (O history), Next, WinX, WinO
// Transitions: PlayX00-PlayX22, PlayO00-PlayO22, XRow0-XDg1, ORow0-ODg1
func BuildTicTacToeNet() *TicTacToeModel {
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

	return &TicTacToeModel{Net: net, Rates: rates}
}

// TestODEStructure verifies the Petri net structure
func TestODEStructure(t *testing.T) {
	model := BuildTicTacToeNet()

	t.Logf("Places: %d", len(model.Net.Places))
	t.Logf("Transitions: %d", len(model.Net.Transitions))
	t.Logf("Arcs: %d", len(model.Net.Arcs))

	// Expected: 30 places (9 board + 9 X hist + 9 O hist + Next + WinX + WinO)
	if len(model.Net.Places) != 30 {
		t.Errorf("expected 30 places, got %d", len(model.Net.Places))
	}

	// Expected: 34 transitions (9 PlayX + 9 PlayO + 8 X win + 8 O win)
	if len(model.Net.Transitions) != 34 {
		t.Errorf("expected 34 transitions, got %d", len(model.Net.Transitions))
	}
}

// TestODESimulation runs the ODE and logs results
func TestODESimulation(t *testing.T) {
	model := BuildTicTacToeNet()
	state := model.Net.SetState(nil)

	// Run ODE
	prob := solver.NewProblem(model.Net, state, [2]float64{0, 3.0}, model.Rates)
	opts := solver.DefaultOptions()
	opts.Abstol = 1e-6
	opts.Reltol = 1e-4
	opts.Dt = 0.1

	sol := solver.Solve(prob, solver.Tsit5(), opts)
	finalState := sol.GetFinalState()

	t.Logf("WinX: %.6f", finalState["WinX"])
	t.Logf("WinO: %.6f", finalState["WinO"])

	// Log token flow to history places
	totalX, totalO := 0.0, 0.0
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			xID := fmt.Sprintf("X%d%d", row, col)
			oID := fmt.Sprintf("O%d%d", row, col)
			totalX += finalState[xID]
			totalO += finalState[oID]
		}
	}
	t.Logf("Total X history: %.6f", totalX)
	t.Logf("Total O history: %.6f", totalO)
}

// TestHeatmapValues computes strategic values for each position
// This is what the browser heatmap should show
func TestHeatmapValues(t *testing.T) {
	model := BuildTicTacToeNet()
	initialState := model.Net.SetState(nil)

	type posValue struct {
		Row, Col int
		Score    float64
		WinX     float64
		WinO     float64
	}
	var values []posValue

	// Evaluate each possible first move for X
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			// Create hypothetical state after X plays at (row, col)
			hypState := make(map[string]float64)
			for k, v := range initialState {
				hypState[k] = v
			}

			posID := fmt.Sprintf("P%d%d", row, col)
			xHistID := fmt.Sprintf("X%d%d", row, col)
			hypState[posID] = 0   // Position taken
			hypState[xHistID] = 1 // X played here
			hypState["Next"] = 1  // O's turn

			// Run ODE
			prob := solver.NewProblem(model.Net, hypState, [2]float64{0, 3.0}, model.Rates)
			opts := solver.FastOptions()
			sol := solver.Solve(prob, solver.Tsit5(), opts)
			finalState := sol.GetFinalState()

			score := finalState["WinX"] - finalState["WinO"]
			values = append(values, posValue{
				Row:   row,
				Col:   col,
				Score: score,
				WinX:  finalState["WinX"],
				WinO:  finalState["WinO"],
			})
		}
	}

	// Output as JSON for comparison with browser
	t.Log("\n=== HEATMAP VALUES (for browser comparison) ===")
	heatmap := make(map[string]float64)
	for _, v := range values {
		key := fmt.Sprintf("%d%d", v.Row, v.Col)
		heatmap[key] = v.Score
		t.Logf("Position (%d,%d): Score=%.6f (WinX=%.6f, WinO=%.6f)",
			v.Row, v.Col, v.Score, v.WinX, v.WinO)
	}

	// Find best and worst
	best, worst := values[0], values[0]
	for _, v := range values[1:] {
		if v.Score > best.Score {
			best = v
		}
		if v.Score < worst.Score {
			worst = v
		}
	}

	t.Logf("\nBest move: (%d,%d) with score %.6f", best.Row, best.Col, best.Score)
	t.Logf("Worst move: (%d,%d) with score %.6f", worst.Row, worst.Col, worst.Score)

	// Output JSON for browser comparison
	jsonBytes, _ := json.MarshalIndent(heatmap, "", "  ")
	t.Logf("\nJSON heatmap:\n%s", string(jsonBytes))
}

// TestHeatmapParity outputs values in a format that can be directly compared with browser
func TestHeatmapParity(t *testing.T) {
	model := BuildTicTacToeNet()
	initialState := model.Net.SetState(nil)

	fmt.Println("\n=== GO-PFLOW ODE HEATMAP VALUES ===")
	fmt.Println("Position | WinX   | WinO   | Score (WinX-WinO)")
	fmt.Println("---------|--------|--------|------------------")

	results := make(map[string]map[string]float64)

	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			hypState := make(map[string]float64)
			for k, v := range initialState {
				hypState[k] = v
			}

			posID := fmt.Sprintf("P%d%d", row, col)
			xHistID := fmt.Sprintf("X%d%d", row, col)
			hypState[posID] = 0
			hypState[xHistID] = 1
			hypState["Next"] = 1

			prob := solver.NewProblem(model.Net, hypState, [2]float64{0, 3.0}, model.Rates)
			opts := solver.FastOptions()
			sol := solver.Solve(prob, solver.Tsit5(), opts)
			finalState := sol.GetFinalState()

			key := fmt.Sprintf("%d%d", row, col)
			results[key] = map[string]float64{
				"WinX":  finalState["WinX"],
				"WinO":  finalState["WinO"],
				"Score": finalState["WinX"] - finalState["WinO"],
			}

			fmt.Printf("   %s     | %.4f | %.4f | %.4f\n",
				key, finalState["WinX"], finalState["WinO"],
				finalState["WinX"]-finalState["WinO"])
		}
	}

	// Write to file for browser comparison
	jsonBytes, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile("ode_heatmap_values.json", jsonBytes, 0644); err != nil {
		t.Logf("Warning: could not write JSON file: %v", err)
	} else {
		fmt.Println("\nWritten to ode_heatmap_values.json")
	}
}

// BenchmarkODEEvaluation benchmarks the ODE evaluation
func BenchmarkODEEvaluation(b *testing.B) {
	model := BuildTicTacToeNet()
	state := model.Net.SetState(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prob := solver.NewProblem(model.Net, state, [2]float64{0, 3.0}, model.Rates)
		opts := solver.FastOptions()
		_ = solver.Solve(prob, solver.Tsit5(), opts)
	}
}
