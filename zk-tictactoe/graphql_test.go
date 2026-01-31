package zktictactoe

import (
	"context"
	"testing"
)

func TestZKGraphQL_CreateAndMove(t *testing.T) {
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatal(err)
	}

	resolvers := zk.ZKGraphQLResolvers()
	ctx := context.Background()

	// Create game
	createResult, err := resolvers["zkCreateGame"](ctx, nil)
	if err != nil {
		t.Fatalf("zkCreateGame failed: %v", err)
	}

	game := createResult.(map[string]any)
	gameID := game["id"].(string)
	if gameID == "" {
		t.Fatal("expected game ID")
	}

	turn := game["turn"].(int)
	if turn != 1 {
		t.Fatalf("expected turn 1 (X), got %d", turn)
	}

	// Make move (X plays center)
	moveResult, err := resolvers["zkMove"](ctx, map[string]any{
		"gameId":   gameID,
		"position": float64(4), // GraphQL sends floats
	})
	if err != nil {
		t.Fatalf("zkMove failed: %v", err)
	}

	move := moveResult.(map[string]any)
	if !move["success"].(bool) {
		t.Fatalf("move failed: %v", move["error"])
	}

	if move["position"].(int) != 4 {
		t.Fatalf("expected position 4, got %v", move["position"])
	}

	if move["player"].(int) != 1 {
		t.Fatalf("expected player 1 (X), got %v", move["player"])
	}

	// Check proof was generated
	if move["proof"] == nil {
		t.Fatal("expected proof to be generated")
	}

	proof := move["proof"].(map[string]any)
	if proof["circuit"].(string) != "transition" {
		t.Fatalf("expected transition circuit, got %s", proof["circuit"])
	}

	if !proof["verified"].(bool) {
		t.Fatal("expected proof to be verified")
	}
}

func TestZKGraphQL_GetGame(t *testing.T) {
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatal(err)
	}

	resolvers := zk.ZKGraphQLResolvers()
	ctx := context.Background()

	// Create game
	createResult, err := resolvers["zkCreateGame"](ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	gameID := createResult.(map[string]any)["id"].(string)

	// Get game
	getResult, err := resolvers["zkGame"](ctx, map[string]any{
		"id": gameID,
	})
	if err != nil {
		t.Fatalf("zkGame failed: %v", err)
	}

	game := getResult.(map[string]any)
	if game["id"].(string) != gameID {
		t.Fatalf("expected id %s, got %s", gameID, game["id"])
	}

	board := game["board"].([]int)
	if len(board) != 9 {
		t.Fatalf("expected 9 cells, got %d", len(board))
	}

	// All cells should be empty (0) at start
	for i, cell := range board {
		if cell != 0 {
			t.Fatalf("expected empty cell at %d, got %d", i, cell)
		}
	}
}

func TestZKGraphQL_WinDetection(t *testing.T) {
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatal(err)
	}

	resolvers := zk.ZKGraphQLResolvers()
	ctx := context.Background()

	// Create game
	createResult, err := resolvers["zkCreateGame"](ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	gameID := createResult.(map[string]any)["id"].(string)

	// Play to X wins with left column: X plays 0, 3, 6; O plays 1, 4
	moves := []int{0, 1, 3, 4, 6}
	for _, pos := range moves {
		_, err := resolvers["zkMove"](ctx, map[string]any{
			"gameId":   gameID,
			"position": float64(pos),
		})
		if err != nil {
			t.Fatalf("move %d failed: %v", pos, err)
		}
	}

	// Fire win transition (x_win_col0 = transition 20)
	_, err = resolvers["zkFireTransition"](ctx, map[string]any{
		"gameId":     gameID,
		"transition": float64(TXWinCol0),
	})
	if err != nil {
		t.Fatalf("fire win transition failed: %v", err)
	}

	// Check win
	winResult, err := resolvers["zkCheckWin"](ctx, map[string]any{
		"gameId": gameID,
	})
	if err != nil {
		t.Fatalf("zkCheckWin failed: %v", err)
	}

	win := winResult.(map[string]any)
	if !win["hasWinner"].(bool) {
		t.Fatal("expected winner")
	}

	if win["winner"].(int) != 1 {
		t.Fatalf("expected X (1) to win, got %v", win["winner"])
	}

	// Check win proof
	if win["proof"] == nil {
		t.Fatal("expected win proof")
	}

	proof := win["proof"].(map[string]any)
	if proof["circuit"].(string) != "win" {
		t.Fatalf("expected win circuit, got %s", proof["circuit"])
	}

	if !proof["verified"].(bool) {
		t.Fatal("expected win proof to be verified")
	}
}

func TestZKGraphQL_ListCircuits(t *testing.T) {
	zk, err := NewZKIntegration()
	if err != nil {
		t.Fatal(err)
	}

	resolvers := zk.ZKGraphQLResolvers()
	ctx := context.Background()

	result, err := resolvers["zkCircuits"](ctx, nil)
	if err != nil {
		t.Fatalf("zkCircuits failed: %v", err)
	}

	circuits := result.([]string)
	if len(circuits) < 2 {
		t.Fatalf("expected at least 2 circuits, got %d", len(circuits))
	}

	// Should have transition and win circuits
	hasTransition := false
	hasWin := false
	for _, c := range circuits {
		if c == "transition" {
			hasTransition = true
		}
		if c == "win" {
			hasWin = true
		}
	}

	if !hasTransition {
		t.Fatal("expected transition circuit")
	}
	if !hasWin {
		t.Fatal("expected win circuit")
	}
}
