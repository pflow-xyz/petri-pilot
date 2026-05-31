package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// ticTacToeModel builds a 9-place model laid out like a 3x3 board with
// transitions for each cell. Useful as a heatmap exercise.
func ticTacToeModel() *goflowmetamodel.Model {
	cells := []string{"a1", "a2", "a3", "b1", "b2", "b3", "c1", "c2", "c3"}
	m := &goflowmetamodel.Model{}
	for _, id := range cells {
		m.Places = append(m.Places, goflowmetamodel.Place{ID: id})
	}
	m.Transitions = append(m.Transitions, goflowmetamodel.Transition{ID: "play"})
	return m
}

func TestHeatmap_TicTacToe(t *testing.T) {
	model := ticTacToeModel()
	// A mid-game-ish heatmap (move scores 0..10).
	marking := map[string]float64{
		"a1": 1, "a2": 3, "a3": 0,
		"b1": 5, "b2": 10, "b3": 4,
		"c1": 0, "c2": 2, "c3": 7,
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_heatmap"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, model),
		"marking": mustJSON(t, marking),
		"title":   "Tic-Tac-Toe move scores",
	}
	res, err := handleHeatmap(context.Background(), req)
	if err != nil {
		t.Fatalf("handleHeatmap: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 500 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("HEATMAP_TTT_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestHeatmap_AutoGrid(t *testing.T) {
	cases := map[int][2]int{
		1:  {1, 1},
		4:  {2, 2},
		7:  {3, 3},
		9:  {3, 3},
		33: {6, 6},
	}
	for n, want := range cases {
		r, c := autoGrid(n)
		if r != want[0] || c != want[1] {
			t.Errorf("autoGrid(%d) = (%d,%d) want (%d,%d)", n, r, c, want[0], want[1])
		}
	}
}

func TestHeatmap_CoffeeShop(t *testing.T) {
	// Heatmap of the coffee shop with no explicit marking — should use
	// Place.Initial values. Auto grid is 3x2 (5 places).
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_heatmap"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, coffeeShopModel()),
		"title": "Coffee shop — initial marking",
	}
	res, err := handleHeatmap(context.Background(), req)
	if err != nil {
		t.Fatalf("handleHeatmap: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 500 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("HEATMAP_COFFEE_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}
