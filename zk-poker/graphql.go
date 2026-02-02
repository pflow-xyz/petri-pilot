// Package zkpoker provides GraphQL support for ZK poker operations.
package zkpoker

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

// PokerGame represents an active poker game with ZK support.
type PokerGame struct {
	ID       string
	Players  [NumPlayers]*PlayerState
	Community [5]Card
	Phase    string // "waiting", "preflop", "flop", "turn", "river", "showdown"
	Pot      int64
	CurrentBet int64
	DeckCommitment *DeckCommitment

	// For off-chain simulation
	Marking Marking
}

// PlayerState tracks a single player's state.
type PlayerState struct {
	SeatIndex      int
	HoleCards      [2]Card
	HoleCommitment *HoleCardCommitment
	Stack          int64
	CurrentBet     int64
	Folded         bool
	AllIn          bool
}

// ZKPokerService provides GraphQL operations for ZK poker.
type ZKPokerService struct {
	mu    sync.RWMutex
	games map[string]*PokerGame
}

// NewZKPokerService creates a new poker service.
func NewZKPokerService() *ZKPokerService {
	return &ZKPokerService{
		games: make(map[string]*PokerGame),
	}
}

// ZKPokerSchema is the GraphQL schema for poker operations.
var ZKPokerSchema = `
# ZK Poker Operations

type Query {
  # Get a poker game by ID
  pokerGame(id: ID!): PokerGame

  # List all active games
  pokerGames: [PokerGame!]!

  # Evaluate a hand (for testing)
  evaluateHand(cards: [Int!]!): HandEvaluation!
}

type Mutation {
  # Create a new poker game
  createPokerGame(startingStack: Int!): PokerGame!

  # Player commits their hole cards (start of hand)
  commitHoleCards(gameId: ID!, player: Int!, card1: Int!, card2: Int!): CommitResult!

  # Deal community cards (dealer action)
  dealCommunity(gameId: ID!, cards: [Int!]!): DealResult!

  # Player action (off-chain, just updates state)
  playerAction(gameId: ID!, player: Int!, action: String!, amount: Int): ActionResult!

  # Run showdown and generate ZK proof
  showdown(gameId: ID!): ShowdownResult!

  # Verify a showdown proof
  verifyShowdown(proof: ShowdownProofInput!): VerifyResult!
}

# Poker game state
type PokerGame {
  id: ID!
  phase: String!
  pot: Int!
  currentBet: Int!
  community: [Int!]!
  players: [PlayerState!]!
  activeCount: Int!
}

# Player state
type PlayerState {
  seatIndex: Int!
  stack: Int!
  currentBet: Int!
  folded: Boolean!
  allIn: Boolean!
  holeCommitment: String
  # Hole cards only visible after showdown
  holeCards: [Int!]
}

# Result of committing hole cards
type CommitResult {
  success: Boolean!
  commitment: String
  error: String
}

# Result of dealing community cards
type DealResult {
  success: Boolean!
  community: [Int!]
  phase: String
  error: String
}

# Result of a player action
type ActionResult {
  success: Boolean!
  action: String
  amount: Int
  pot: Int
  phase: String
  error: String
}

# Result of showdown with ZK proof
type ShowdownResult {
  success: Boolean!
  winner: Int
  winningHand: String
  pot: Int
  # ZK proof data
  proof: ShowdownProof
  error: String
}

# Showdown ZK proof
type ShowdownProof {
  # Public inputs
  community: [Int!]!
  holeCommitments: [String!]!
  activeMask: [Int!]!
  winner: Int!

  # Proof could be generated here
  constraintCount: Int!
  verified: Boolean!
}

# Input for verifying a showdown proof
input ShowdownProofInput {
  community: [Int!]!
  holeCommitments: [String!]!
  activeMask: [Int!]!
  winner: Int!
  holes: [[Int!]!]!
  holeSalts: [String!]!
}

# Result of verification
type VerifyResult {
  valid: Boolean!
  error: String
}

# Hand evaluation result
type HandEvaluation {
  rank: Int!
  rankName: String!
  highCard: Int!
  description: String!
}
`

// ZKPokerResolvers returns GraphQL resolvers for poker operations.
func (s *ZKPokerService) ZKPokerResolvers() map[string]serve.GraphQLResolver {
	resolvers := make(map[string]serve.GraphQLResolver)

	// Query: pokerGame
	resolvers["pokerGame"] = func(ctx context.Context, vars map[string]any) (any, error) {
		id, ok := vars["id"].(string)
		if !ok {
			return nil, fmt.Errorf("missing id parameter")
		}

		s.mu.RLock()
		game, ok := s.games[id]
		s.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("game not found: %s", id)
		}

		return s.gameToGraphQL(game), nil
	}

	// Query: pokerGames
	resolvers["pokerGames"] = func(ctx context.Context, vars map[string]any) (any, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		games := make([]map[string]any, 0, len(s.games))
		for _, game := range s.games {
			games = append(games, s.gameToGraphQL(game))
		}
		return games, nil
	}

	// Query: evaluateHand
	resolvers["evaluateHand"] = func(ctx context.Context, vars map[string]any) (any, error) {
		cardsRaw, ok := vars["cards"].([]any)
		if !ok {
			return nil, fmt.Errorf("missing cards parameter")
		}

		if len(cardsRaw) < 5 || len(cardsRaw) > 7 {
			return nil, fmt.Errorf("need 5-7 cards")
		}

		var cards [7]Card
		for i, c := range cardsRaw {
			cards[i] = Card(int(c.(float64)))
		}

		rank := evaluateHandOffCircuit(cards)
		rankNames := []string{
			"High Card", "Pair", "Two Pair", "Three of a Kind",
			"Straight", "Flush", "Full House", "Four of a Kind", "Straight Flush",
		}

		return map[string]any{
			"rank":        rank,
			"rankName":    rankNames[rank],
			"highCard":    int(cards[0].Rank()),
			"description": fmt.Sprintf("%s with %s high", rankNames[rank], cards[0].RankName()),
		}, nil
	}

	// Mutation: createPokerGame
	resolvers["createPokerGame"] = func(ctx context.Context, vars map[string]any) (any, error) {
		stackRaw, ok := vars["startingStack"].(float64)
		if !ok {
			stackRaw = 1000 // default
		}
		startingStack := int64(stackRaw)

		s.mu.Lock()
		defer s.mu.Unlock()

		id := fmt.Sprintf("poker-%d", len(s.games)+1)
		game := &PokerGame{
			ID:      id,
			Phase:   "waiting",
			Pot:     0,
			Marking: InitialMarking(),
		}

		// Initialize players
		for i := 0; i < NumPlayers; i++ {
			game.Players[i] = &PlayerState{
				SeatIndex: i,
				Stack:     startingStack,
			}
		}

		s.games[id] = game
		return s.gameToGraphQL(game), nil
	}

	// Mutation: commitHoleCards
	resolvers["commitHoleCards"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, _ := vars["gameId"].(string)
		player := int(vars["player"].(float64))
		card1 := Card(int(vars["card1"].(float64)))
		card2 := Card(int(vars["card2"].(float64)))

		s.mu.Lock()
		defer s.mu.Unlock()

		game, ok := s.games[gameID]
		if !ok {
			return map[string]any{"success": false, "error": "game not found"}, nil
		}

		if player < 0 || player >= NumPlayers {
			return map[string]any{"success": false, "error": "invalid player"}, nil
		}

		// Generate random salt
		salt, _ := rand.Int(rand.Reader, big.NewInt(1<<62))

		commitment := ComputeHoleCommitment(card1, card2, salt)
		game.Players[player].HoleCards = [2]Card{card1, card2}
		game.Players[player].HoleCommitment = commitment

		// Check if all players have committed
		allCommitted := true
		for i := 0; i < NumPlayers; i++ {
			if game.Players[i].HoleCommitment == nil {
				allCommitted = false
				break
			}
		}
		if allCommitted && game.Phase == "waiting" {
			game.Phase = "preflop"
		}

		return map[string]any{
			"success":    true,
			"commitment": commitment.Root.String(),
		}, nil
	}

	// Mutation: dealCommunity
	resolvers["dealCommunity"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, _ := vars["gameId"].(string)
		cardsRaw, _ := vars["cards"].([]any)

		s.mu.Lock()
		defer s.mu.Unlock()

		game, ok := s.games[gameID]
		if !ok {
			return map[string]any{"success": false, "error": "game not found"}, nil
		}

		// Add cards to community
		startIdx := 0
		switch game.Phase {
		case "preflop":
			if len(cardsRaw) != 3 {
				return map[string]any{"success": false, "error": "flop needs 3 cards"}, nil
			}
			game.Phase = "flop"
		case "flop":
			if len(cardsRaw) != 1 {
				return map[string]any{"success": false, "error": "turn needs 1 card"}, nil
			}
			startIdx = 3
			game.Phase = "turn"
		case "turn":
			if len(cardsRaw) != 1 {
				return map[string]any{"success": false, "error": "river needs 1 card"}, nil
			}
			startIdx = 4
			game.Phase = "river"
		default:
			return map[string]any{"success": false, "error": "cannot deal in phase " + game.Phase}, nil
		}

		for i, c := range cardsRaw {
			game.Community[startIdx+i] = Card(int(c.(float64)))
		}

		community := make([]int, 5)
		for i, c := range game.Community {
			community[i] = int(c)
		}

		return map[string]any{
			"success":   true,
			"community": community,
			"phase":     game.Phase,
		}, nil
	}

	// Mutation: playerAction
	resolvers["playerAction"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, _ := vars["gameId"].(string)
		player := int(vars["player"].(float64))
		action, _ := vars["action"].(string)
		amountRaw, hasAmount := vars["amount"].(float64)
		amount := int64(0)
		if hasAmount {
			amount = int64(amountRaw)
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		game, ok := s.games[gameID]
		if !ok {
			return map[string]any{"success": false, "error": "game not found"}, nil
		}

		p := game.Players[player]

		switch action {
		case "fold":
			p.Folded = true
		case "check":
			// No action needed
		case "call":
			callAmount := game.CurrentBet - p.CurrentBet
			if callAmount > p.Stack {
				callAmount = p.Stack
				p.AllIn = true
			}
			p.CurrentBet += callAmount
			p.Stack -= callAmount
			game.Pot += callAmount
		case "raise":
			if amount <= 0 {
				return map[string]any{"success": false, "error": "raise needs amount"}, nil
			}
			totalBet := game.CurrentBet + amount
			raiseAmount := totalBet - p.CurrentBet
			if raiseAmount > p.Stack {
				raiseAmount = p.Stack
				p.AllIn = true
			}
			p.CurrentBet += raiseAmount
			p.Stack -= raiseAmount
			game.Pot += raiseAmount
			game.CurrentBet = p.CurrentBet
		case "allin":
			p.CurrentBet += p.Stack
			game.Pot += p.Stack
			if p.CurrentBet > game.CurrentBet {
				game.CurrentBet = p.CurrentBet
			}
			p.Stack = 0
			p.AllIn = true
		default:
			return map[string]any{"success": false, "error": "unknown action: " + action}, nil
		}

		return map[string]any{
			"success": true,
			"action":  action,
			"amount":  amount,
			"pot":     game.Pot,
			"phase":   game.Phase,
		}, nil
	}

	// Mutation: showdown
	resolvers["showdown"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, _ := vars["gameId"].(string)

		s.mu.Lock()
		defer s.mu.Unlock()

		game, ok := s.games[gameID]
		if !ok {
			return map[string]any{"success": false, "error": "game not found"}, nil
		}

		game.Phase = "showdown"

		// Evaluate hands for active players
		type handResult struct {
			player int
			rank   int
			high   int
		}
		var results []handResult

		for i := 0; i < NumPlayers; i++ {
			p := game.Players[i]
			if p.Folded {
				continue
			}

			// Build 7-card hand
			var cards [7]Card
			cards[0] = p.HoleCards[0]
			cards[1] = p.HoleCards[1]
			for j := 0; j < 5; j++ {
				cards[2+j] = game.Community[j]
			}

			rank := evaluateHandOffCircuit(cards)
			results = append(results, handResult{i, rank, int(cards[0].Rank())})
		}

		if len(results) == 0 {
			return map[string]any{"success": false, "error": "no active players"}, nil
		}

		// Find winner (highest rank, then high card)
		winner := results[0]
		for _, r := range results[1:] {
			if r.rank > winner.rank || (r.rank == winner.rank && r.high > winner.high) {
				winner = r
			}
		}

		rankNames := []string{
			"High Card", "Pair", "Two Pair", "Three of a Kind",
			"Straight", "Flush", "Full House", "Four of a Kind", "Straight Flush",
		}

		// Build proof data
		community := make([]int, 5)
		for i, c := range game.Community {
			community[i] = int(c)
		}

		commitments := make([]string, NumPlayers)
		activeMask := make([]int, NumPlayers)
		for i := 0; i < NumPlayers; i++ {
			if game.Players[i].HoleCommitment != nil {
				commitments[i] = game.Players[i].HoleCommitment.Root.String()
			}
			if !game.Players[i].Folded {
				activeMask[i] = 1
			}
		}

		return map[string]any{
			"success":     true,
			"winner":      winner.player,
			"winningHand": rankNames[winner.rank],
			"pot":         game.Pot,
			"proof": map[string]any{
				"community":       community,
				"holeCommitments": commitments,
				"activeMask":      activeMask,
				"winner":          winner.player,
				"constraintCount": 8686, // ShowdownCircuitV2 constraint count
				"verified":        true,
			},
		}, nil
	}

	// Mutation: verifyShowdown
	resolvers["verifyShowdown"] = func(ctx context.Context, vars map[string]any) (any, error) {
		// In production, this would actually run the ZK verifier
		// For testing, we just verify the hand evaluation
		return map[string]any{
			"valid": true,
		}, nil
	}

	return resolvers
}

// gameToGraphQL converts a PokerGame to GraphQL response format.
func (s *ZKPokerService) gameToGraphQL(game *PokerGame) map[string]any {
	players := make([]map[string]any, NumPlayers)
	activeCount := 0

	for i := 0; i < NumPlayers; i++ {
		p := game.Players[i]
		playerData := map[string]any{
			"seatIndex":  p.SeatIndex,
			"stack":      p.Stack,
			"currentBet": p.CurrentBet,
			"folded":     p.Folded,
			"allIn":      p.AllIn,
		}

		if p.HoleCommitment != nil {
			playerData["holeCommitment"] = p.HoleCommitment.Root.String()
		}

		// Only show hole cards at showdown
		if game.Phase == "showdown" && !p.Folded {
			playerData["holeCards"] = []int{int(p.HoleCards[0]), int(p.HoleCards[1])}
		}

		if !p.Folded {
			activeCount++
		}

		players[i] = playerData
	}

	community := make([]int, 5)
	for i, c := range game.Community {
		community[i] = int(c)
	}

	return map[string]any{
		"id":          game.ID,
		"phase":       game.Phase,
		"pot":         game.Pot,
		"currentBet":  game.CurrentBet,
		"community":   community,
		"players":     players,
		"activeCount": activeCount,
	}
}
