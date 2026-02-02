package zkpoker

import (
	"context"
	"testing"
)

func TestGraphQLCreateGame(t *testing.T) {
	svc := NewZKPokerService()
	resolvers := svc.ZKPokerResolvers()

	// Create a game
	result, err := resolvers["createPokerGame"](context.Background(), map[string]any{
		"startingStack": float64(1000),
	})
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	game := result.(map[string]any)
	if game["id"] == nil {
		t.Error("Game should have an ID")
	}
	if game["phase"] != "waiting" {
		t.Errorf("Game should be in waiting phase, got %v", game["phase"])
	}
	if game["pot"] != int64(0) {
		t.Errorf("Pot should be 0, got %v", game["pot"])
	}

	t.Logf("Created game: %v", game["id"])
}

func TestGraphQLCommitHoleCards(t *testing.T) {
	svc := NewZKPokerService()
	resolvers := svc.ZKPokerResolvers()

	// Create a game
	result, _ := resolvers["createPokerGame"](context.Background(), map[string]any{
		"startingStack": float64(1000),
	})
	game := result.(map[string]any)
	gameID := game["id"].(string)

	// Commit hole cards for all players
	for p := 0; p < NumPlayers; p++ {
		// Each player gets different cards
		card1 := p * 2       // 0, 2, 4, 6, 8
		card2 := p*2 + 1 + 10 // 11, 13, 15, 17, 19

		result, err := resolvers["commitHoleCards"](context.Background(), map[string]any{
			"gameId": gameID,
			"player": float64(p),
			"card1":  float64(card1),
			"card2":  float64(card2),
		})
		if err != nil {
			t.Fatalf("Failed to commit for player %d: %v", p, err)
		}

		commit := result.(map[string]any)
		if !commit["success"].(bool) {
			t.Errorf("Player %d commit should succeed", p)
		}
		if commit["commitment"] == nil {
			t.Errorf("Player %d should have commitment", p)
		}
		t.Logf("Player %d commitment: %v", p, commit["commitment"].(string)[:20]+"...")
	}

	// Check game is now in preflop
	result, _ = resolvers["pokerGame"](context.Background(), map[string]any{
		"id": gameID,
	})
	game = result.(map[string]any)
	if game["phase"] != "preflop" {
		t.Errorf("Game should be in preflop after all commits, got %v", game["phase"])
	}
}

func TestGraphQLFullHand(t *testing.T) {
	svc := NewZKPokerService()
	resolvers := svc.ZKPokerResolvers()

	// Create game
	result, _ := resolvers["createPokerGame"](context.Background(), map[string]any{
		"startingStack": float64(1000),
	})
	gameID := result.(map[string]any)["id"].(string)

	// Player 0: Ah Kh (cards 50, 46) - will make flush
	// Player 1: 2c 3c (cards 0, 4) - weak
	// Player 2: 7d 8d (cards 21, 25) - weak
	// Players 3,4 fold

	holeCards := [][2]int{
		{50, 46}, // Ah, Kh
		{0, 4},   // 2c, 3c
		{21, 25}, // 7d, 8d
		{1, 5},   // 2d, 3d (will fold)
		{2, 6},   // 2h, 3h (will fold)
	}

	// Commit all hole cards
	for p := 0; p < NumPlayers; p++ {
		resolvers["commitHoleCards"](context.Background(), map[string]any{
			"gameId": gameID,
			"player": float64(p),
			"card1":  float64(holeCards[p][0]),
			"card2":  float64(holeCards[p][1]),
		})
	}

	// Players 3 and 4 fold
	for _, p := range []int{3, 4} {
		resolvers["playerAction"](context.Background(), map[string]any{
			"gameId": gameID,
			"player": float64(p),
			"action": "fold",
		})
	}

	// Deal flop: Qh Jh Th (cards 42, 38, 34) - gives player 0 a straight flush!
	resolvers["dealCommunity"](context.Background(), map[string]any{
		"gameId": gameID,
		"cards":  []any{float64(42), float64(38), float64(34)},
	})

	// Deal turn: 9d (card 29)
	resolvers["dealCommunity"](context.Background(), map[string]any{
		"gameId": gameID,
		"cards":  []any{float64(29)},
	})

	// Deal river: 8s (card 27)
	resolvers["dealCommunity"](context.Background(), map[string]any{
		"gameId": gameID,
		"cards":  []any{float64(27)},
	})

	// Run showdown
	result, err := resolvers["showdown"](context.Background(), map[string]any{
		"gameId": gameID,
	})
	if err != nil {
		t.Fatalf("Showdown failed: %v", err)
	}

	showdown := result.(map[string]any)
	if !showdown["success"].(bool) {
		t.Fatalf("Showdown should succeed: %v", showdown["error"])
	}

	t.Logf("Winner: Player %v", showdown["winner"])
	t.Logf("Winning hand: %v", showdown["winningHand"])

	if showdown["winner"].(int) != 0 {
		t.Errorf("Player 0 should win with straight flush, got player %v", showdown["winner"])
	}
	if showdown["winningHand"].(string) != "Straight Flush" {
		t.Errorf("Should be Straight Flush, got %v", showdown["winningHand"])
	}

	// Check proof data
	proof := showdown["proof"].(map[string]any)
	t.Logf("Constraint count: %v", proof["constraintCount"])
	t.Logf("Verified: %v", proof["verified"])

	if proof["constraintCount"].(int) != 8686 {
		t.Errorf("Expected 8686 constraints, got %v", proof["constraintCount"])
	}
}

func TestGraphQLEvaluateHand(t *testing.T) {
	svc := NewZKPokerService()
	resolvers := svc.ZKPokerResolvers()

	// Royal flush: Ah Kh Qh Jh Th + 2 random
	// Card encoding: rank*4 + suit, hearts=2
	// A=12, K=11, Q=10, J=9, T=8
	cards := []any{
		float64(12*4 + 2), // Ah = 50
		float64(11*4 + 2), // Kh = 46
		float64(10*4 + 2), // Qh = 42
		float64(9*4 + 2),  // Jh = 38
		float64(8*4 + 2),  // Th = 34
		float64(0),        // 2c
		float64(1),        // 2d
	}

	result, err := resolvers["evaluateHand"](context.Background(), map[string]any{
		"cards": cards,
	})
	if err != nil {
		t.Fatalf("Failed to evaluate: %v", err)
	}

	eval := result.(map[string]any)
	t.Logf("Rank: %v (%v)", eval["rank"], eval["rankName"])
	t.Logf("Description: %v", eval["description"])

	if eval["rank"].(int) != RankStraightFlush {
		t.Errorf("Should be straight flush (8), got %v", eval["rank"])
	}
}

func TestGraphQLPlayerActions(t *testing.T) {
	svc := NewZKPokerService()
	resolvers := svc.ZKPokerResolvers()

	// Create game
	result, _ := resolvers["createPokerGame"](context.Background(), map[string]any{
		"startingStack": float64(1000),
	})
	gameID := result.(map[string]any)["id"].(string)

	// Commit hole cards
	for p := 0; p < NumPlayers; p++ {
		resolvers["commitHoleCards"](context.Background(), map[string]any{
			"gameId": gameID,
			"player": float64(p),
			"card1":  float64(p * 2),
			"card2":  float64(p*2 + 1),
		})
	}

	// Player 0 raises 50
	result, _ = resolvers["playerAction"](context.Background(), map[string]any{
		"gameId": gameID,
		"player": float64(0),
		"action": "raise",
		"amount": float64(50),
	})
	action := result.(map[string]any)
	if !action["success"].(bool) {
		t.Errorf("Raise should succeed")
	}
	if action["pot"].(int64) != 50 {
		t.Errorf("Pot should be 50, got %v", action["pot"])
	}

	// Player 1 calls
	result, _ = resolvers["playerAction"](context.Background(), map[string]any{
		"gameId": gameID,
		"player": float64(1),
		"action": "call",
	})
	action = result.(map[string]any)
	if action["pot"].(int64) != 100 {
		t.Errorf("Pot should be 100, got %v", action["pot"])
	}

	// Player 2 folds
	result, _ = resolvers["playerAction"](context.Background(), map[string]any{
		"gameId": gameID,
		"player": float64(2),
		"action": "fold",
	})

	// Check game state
	result, _ = resolvers["pokerGame"](context.Background(), map[string]any{
		"id": gameID,
	})
	game := result.(map[string]any)
	players := game["players"].([]map[string]any)

	if !players[2]["folded"].(bool) {
		t.Error("Player 2 should be folded")
	}
	if players[0]["stack"].(int64) != 950 {
		t.Errorf("Player 0 stack should be 950, got %v", players[0]["stack"])
	}
	if players[1]["stack"].(int64) != 950 {
		t.Errorf("Player 1 stack should be 950, got %v", players[1]["stack"])
	}

	t.Logf("Pot: %v", game["pot"])
	t.Logf("Active players: %v", game["activeCount"])
}
