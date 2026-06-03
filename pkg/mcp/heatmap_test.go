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

func TestHeatmap_PositionBinning(t *testing.T) {
	// 3x3 TTT laid out with explicit X/Y. Define places out of row-major
	// order to prove the binning ignores definition order and uses X/Y.
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			// Bottom-right first, then random-ish order.
			{ID: "c3", X: 300, Y: 300, Initial: 9},
			{ID: "a1", X: 100, Y: 100, Initial: 1},
			{ID: "b2", X: 200, Y: 200, Initial: 5},
			{ID: "a3", X: 300, Y: 100, Initial: 3},
			{ID: "c1", X: 100, Y: 300, Initial: 7},
			{ID: "a2", X: 200, Y: 100, Initial: 2},
			{ID: "b1", X: 100, Y: 200, Initial: 4},
			{ID: "b3", X: 300, Y: 200, Initial: 6},
			{ID: "c2", X: 200, Y: 300, Initial: 8},
		},
	}
	placement := positionBinPlacement(model, 3, 3)
	if placement == nil {
		t.Fatalf("expected non-nil placement when all places have positions")
	}
	// Build a map from place ID to expected (row, col).
	want := map[string][2]int{
		"a1": {0, 0}, "a2": {0, 1}, "a3": {0, 2},
		"b1": {1, 0}, "b2": {1, 1}, "b3": {1, 2},
		"c1": {2, 0}, "c2": {2, 1}, "c3": {2, 2},
	}
	for i, p := range model.Places {
		got := placement[i]
		if got != want[p.ID] {
			t.Errorf("%s placed at %v, want %v", p.ID, got, want[p.ID])
		}
	}
}

func TestHeatmap_PositionBinning_FallsBack(t *testing.T) {
	// One place missing position → must fall back to flat order (nil).
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "a", X: 100, Y: 100},
			{ID: "b"},
		},
	}
	if got := positionBinPlacement(model, 1, 2); got != nil {
		t.Errorf("expected nil placement when a place lacks position, got %v", got)
	}
}

func TestHeatmap_PositionBinning_Collision(t *testing.T) {
	// Two places at the same X,Y bin → second drifts to a free cell.
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "a", X: 100, Y: 100},
			{ID: "b", X: 100, Y: 100}, // same bin as a
			{ID: "c", X: 300, Y: 100},
			{ID: "d", X: 100, Y: 300},
		},
	}
	placement := positionBinPlacement(model, 2, 2)
	if placement == nil {
		t.Fatalf("expected non-nil placement")
	}
	seen := map[[2]int]string{}
	for i, p := range model.Places {
		if other, ok := seen[placement[i]]; ok {
			t.Errorf("collision: %s and %s both at %v", p.ID, other, placement[i])
		}
		seen[placement[i]] = p.ID
	}
}

func TestHeatmap_TTTWithPositions(t *testing.T) {
	// End-to-end: a model with X/Y positions should render via positional
	// binning even when places are defined out of order.
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "c3", X: 300, Y: 300, Initial: 9},
			{ID: "a1", X: 100, Y: 100, Initial: 1},
			{ID: "b2", X: 200, Y: 200, Initial: 5},
			{ID: "a3", X: 300, Y: 100, Initial: 3},
			{ID: "c1", X: 100, Y: 300, Initial: 7},
			{ID: "a2", X: 200, Y: 100, Initial: 2},
			{ID: "b1", X: 100, Y: 200, Initial: 4},
			{ID: "b3", X: 300, Y: 200, Initial: 6},
			{ID: "c2", X: 200, Y: 300, Initial: 8},
		},
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_heatmap"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, model),
		"title": "TTT — positional binning",
	}
	res, err := handleHeatmap(context.Background(), req)
	if err != nil {
		t.Fatalf("handleHeatmap: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if path := os.Getenv("HEATMAP_TTT_XY_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
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
