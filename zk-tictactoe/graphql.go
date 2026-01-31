// Package zktictactoe provides GraphQL support for ZK proof operations.
package zktictactoe

import (
	"context"
	"fmt"

	"github.com/pflow-xyz/go-pflow/prover"
	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

// ZKGraphQLSchema extends the base tic-tac-toe schema with ZK operations.
var ZKGraphQLSchema = `
# ZK Proof Operations for Tic-Tac-Toe

type Query {
  # Get ZK game state
  zkGame(id: ID!): ZKGameState

  # List available ZK circuits
  zkCircuits: [String!]!
}

type Mutation {
  # Create a new ZK-enabled game
  zkCreateGame: ZKGameState!

  # Make a move with ZK proof generation
  zkMove(gameId: ID!, position: Int!): ZKMoveResult!

  # Check for win and generate ZK proof
  zkCheckWin(gameId: ID!): ZKWinResult!

  # Fire a specific Petri net transition with ZK proof
  zkFireTransition(gameId: ID!, transition: Int!): ZKMoveResult!
}

# ZK game state
type ZKGameState {
  id: ID!
  stateRoot: String!
  board: [Int!]!
  turn: Int!
  turnCount: Int!
  isOver: Boolean!
  winner: Int
  enabledMoves: [Int!]!
  roots: [String!]!
}

# Result of a ZK move
type ZKMoveResult {
  success: Boolean!
  position: Int
  player: Int
  preStateRoot: String
  postStateRoot: String
  board: [Int!]
  turnCount: Int
  isOver: Boolean
  winner: Int
  proof: ZKProof
  error: String
}

# Result of checking for a win
type ZKWinResult {
  hasWinner: Boolean!
  winner: Int
  stateRoot: String
  proof: ZKProof
}

# ZK proof data (Groth16)
type ZKProof {
  circuit: String!
  proofHex: String!
  publicInputs: [String!]!
  verified: Boolean!
  # Solidity-compatible proof points
  a: [String!]!
  b: [[String!]!]!
  c: [String!]!
  rawProof: [String!]!
}
`

// ZKGraphQLResolvers returns GraphQL resolvers for ZK operations.
func (z *ZKIntegration) ZKGraphQLResolvers() map[string]serve.GraphQLResolver {
	resolvers := make(map[string]serve.GraphQLResolver)

	// Query: zkGame
	resolvers["zkGame"] = func(ctx context.Context, vars map[string]any) (any, error) {
		id, ok := vars["id"].(string)
		if !ok {
			return nil, fmt.Errorf("missing id parameter")
		}

		z.mu.RLock()
		game, ok := z.games[id]
		z.mu.RUnlock()

		if !ok {
			return nil, fmt.Errorf("game not found: %s", id)
		}

		return z.gameToGraphQL(id, game), nil
	}

	// Query: zkCircuits
	resolvers["zkCircuits"] = func(ctx context.Context, vars map[string]any) (any, error) {
		return z.service.ListCircuits(), nil
	}

	// Mutation: zkCreateGame
	resolvers["zkCreateGame"] = func(ctx context.Context, vars map[string]any) (any, error) {
		z.mu.Lock()
		defer z.mu.Unlock()

		game := NewPetriGame()
		id := fmt.Sprintf("zk-%d", len(z.games)+1)
		z.games[id] = game

		return z.gameToGraphQL(id, game), nil
	}

	// Mutation: zkMove
	resolvers["zkMove"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, ok := vars["gameId"].(string)
		if !ok {
			return nil, fmt.Errorf("missing gameId parameter")
		}

		position, ok := vars["position"].(float64)
		if !ok {
			return nil, fmt.Errorf("missing position parameter")
		}

		return z.handleMoveGraphQL(gameID, int(position))
	}

	// Mutation: zkCheckWin
	resolvers["zkCheckWin"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, ok := vars["gameId"].(string)
		if !ok {
			return nil, fmt.Errorf("missing gameId parameter")
		}

		return z.handleCheckWinGraphQL(gameID)
	}

	// Mutation: zkFireTransition
	resolvers["zkFireTransition"] = func(ctx context.Context, vars map[string]any) (any, error) {
		gameID, ok := vars["gameId"].(string)
		if !ok {
			return nil, fmt.Errorf("missing gameId parameter")
		}

		transition, ok := vars["transition"].(float64)
		if !ok {
			return nil, fmt.Errorf("missing transition parameter")
		}

		return z.handleFireTransitionGraphQL(gameID, int(transition))
	}

	return resolvers
}

// gameToGraphQL converts a PetriGame to GraphQL response format.
func (z *ZKIntegration) gameToGraphQL(id string, game *PetriGame) map[string]any {
	board := extractBoard(game.Marking)
	boardInts := make([]int, 9)
	for i, v := range board {
		boardInts[i] = int(v)
	}

	return map[string]any{
		"id":           id,
		"stateRoot":    game.CurrentRoot().String(),
		"board":        boardInts,
		"turn":         int(currentPlayer(game.Marking)),
		"turnCount":    turnCount(game.Marking),
		"isOver":       isGameOver(game.Marking),
		"winner":       int(getWinner(game.Marking)),
		"enabledMoves": game.EnabledMoves(),
		"roots":        rootsToStrings(game.Roots),
	}
}

// handleMoveGraphQL processes a move via GraphQL.
func (z *ZKIntegration) handleMoveGraphQL(gameID string, position int) (map[string]any, error) {
	z.mu.Lock()
	game, ok := z.games[gameID]
	if !ok {
		z.mu.Unlock()
		return map[string]any{
			"success": false,
			"error":   "game not found",
		}, nil
	}

	// Determine which player's turn and map position to transition
	player := currentPlayer(game.Marking)
	transition := positionToTransition(position, player)
	if transition < 0 {
		z.mu.Unlock()
		return map[string]any{
			"success": false,
			"error":   "invalid position",
		}, nil
	}

	// Fire the Petri net transition
	witness, err := game.FireTransition(transition)
	z.mu.Unlock()

	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	// Generate proof
	assignment := witness.ToPetriTransitionAssignment()
	proofResult, err := z.prover.Prove("transition", assignment)

	var proof map[string]any
	if err == nil {
		verifyErr := z.prover.Verify("transition", assignment)
		proof = proofToGraphQL("transition", proofResult, verifyErr == nil)
	}

	// Get current state
	board := extractBoard(game.Marking)
	boardInts := make([]int, 9)
	for i, v := range board {
		boardInts[i] = int(v)
	}

	return map[string]any{
		"success":       true,
		"position":      position,
		"player":        int(player),
		"preStateRoot":  witness.PreStateRoot.String(),
		"postStateRoot": witness.PostStateRoot.String(),
		"board":         boardInts,
		"turnCount":     turnCount(game.Marking),
		"isOver":        isGameOver(game.Marking),
		"winner":        int(getWinner(game.Marking)),
		"proof":         proof,
	}, nil
}

// handleCheckWinGraphQL checks for a win via GraphQL.
func (z *ZKIntegration) handleCheckWinGraphQL(gameID string) (map[string]any, error) {
	z.mu.RLock()
	game, ok := z.games[gameID]
	z.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("game not found: %s", gameID)
	}

	witness := game.GetWinWitness()
	if witness == nil {
		return map[string]any{
			"hasWinner": false,
			"stateRoot": game.CurrentRoot().String(),
		}, nil
	}

	// Generate win proof
	assignment := witness.ToPetriWinAssignment()
	proofResult, err := z.prover.Prove("win", assignment)

	var proof map[string]any
	if err == nil {
		verifyErr := z.prover.Verify("win", assignment)
		proof = proofToGraphQL("win", proofResult, verifyErr == nil)
	}

	// Determine winner
	var winner int
	if witness.Winner == PlaceWinX {
		winner = int(X)
	} else {
		winner = int(O)
	}

	return map[string]any{
		"hasWinner": true,
		"winner":    winner,
		"stateRoot": witness.StateRoot.String(),
		"proof":     proof,
	}, nil
}

// handleFireTransitionGraphQL fires an arbitrary transition via GraphQL.
func (z *ZKIntegration) handleFireTransitionGraphQL(gameID string, transition int) (map[string]any, error) {
	z.mu.Lock()
	game, ok := z.games[gameID]
	if !ok {
		z.mu.Unlock()
		return map[string]any{
			"success": false,
			"error":   "game not found",
		}, nil
	}

	// Fire the transition
	witness, err := game.FireTransition(transition)
	z.mu.Unlock()

	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	// Generate proof
	assignment := witness.ToPetriTransitionAssignment()
	proofResult, err := z.prover.Prove("transition", assignment)

	var proof map[string]any
	if err == nil {
		verifyErr := z.prover.Verify("transition", assignment)
		proof = proofToGraphQL("transition", proofResult, verifyErr == nil)
	}

	// Get current state
	board := extractBoard(game.Marking)
	boardInts := make([]int, 9)
	for i, v := range board {
		boardInts[i] = int(v)
	}

	return map[string]any{
		"success":       true,
		"preStateRoot":  witness.PreStateRoot.String(),
		"postStateRoot": witness.PostStateRoot.String(),
		"board":         boardInts,
		"turnCount":     turnCount(game.Marking),
		"isOver":        isGameOver(game.Marking),
		"winner":        int(getWinner(game.Marking)),
		"proof":         proof,
	}, nil
}

// proofToGraphQL converts a prover.ProofResult to GraphQL format.
func proofToGraphQL(circuit string, pr *prover.ProofResult, verified bool) map[string]any {
	a := []string{
		fmt.Sprintf("0x%064x", pr.A[0]),
		fmt.Sprintf("0x%064x", pr.A[1]),
	}

	b := [][]string{
		{fmt.Sprintf("0x%064x", pr.B[0][0]), fmt.Sprintf("0x%064x", pr.B[0][1])},
		{fmt.Sprintf("0x%064x", pr.B[1][0]), fmt.Sprintf("0x%064x", pr.B[1][1])},
	}

	c := []string{
		fmt.Sprintf("0x%064x", pr.C[0]),
		fmt.Sprintf("0x%064x", pr.C[1]),
	}

	rawProof := make([]string, len(pr.RawProof))
	for i, p := range pr.RawProof {
		rawProof[i] = fmt.Sprintf("0x%064x", p)
	}

	return map[string]any{
		"circuit":      circuit,
		"proofHex":     rawProofToHex(pr.RawProof),
		"publicInputs": pr.PublicInputs,
		"verified":     verified,
		"a":            a,
		"b":            b,
		"c":            c,
		"rawProof":     rawProof,
	}
}
