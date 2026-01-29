// Package zktictactoe provides ZK proof integration for the tic-tac-toe service.
// This version uses the full Petri net model for ZK proofs, not just the board state.
package zktictactoe

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"

	"github.com/pflow-xyz/go-pflow/prover"
)

// ZKIntegration provides ZK proof endpoints for tic-tac-toe using full Petri net semantics.
type ZKIntegration struct {
	prover  *prover.Prover
	service *prover.Service
	games   map[string]*PetriGame // in-memory game state for ZK proofs
	mu      sync.RWMutex
}

// NewZKIntegration creates a new ZK integration with compiled Petri net circuits.
func NewZKIntegration() (*ZKIntegration, error) {
	p := prover.NewProver()

	// Compile Petri net transition circuit (proves valid transition firing)
	transitionCircuit, err := p.CompileCircuit("transition", &PetriTransitionCircuit{})
	if err != nil {
		return nil, fmt.Errorf("failed to compile transition circuit: %w", err)
	}
	p.StoreCircuit("transition", transitionCircuit)

	// Keep "move" as alias for backwards compatibility
	p.StoreCircuit("move", transitionCircuit)

	// Compile Petri net win circuit (proves a place has tokens)
	winCircuit, err := p.CompileCircuit("win", &PetriWinCircuit{})
	if err != nil {
		return nil, fmt.Errorf("failed to compile win circuit: %w", err)
	}
	p.StoreCircuit("win", winCircuit)

	factory := &TicTacToeWitnessFactory{}

	return &ZKIntegration{
		prover:  p,
		service: prover.NewService(p, factory),
		games:   make(map[string]*PetriGame),
	}, nil
}

// extractBoard extracts the 9-cell board state from the full Petri net marking.
// Returns a board where 0=empty, 1=X, 2=O.
func extractBoard(m Marking) [9]uint8 {
	var board [9]uint8
	for i := 0; i < 9; i++ {
		if m[X00+i] > 0 {
			board[i] = X
		} else if m[O00+i] > 0 {
			board[i] = O
		} else {
			board[i] = Empty
		}
	}
	return board
}

// currentPlayer returns whose turn it is based on the marking.
func currentPlayer(m Marking) uint8 {
	if m[PlaceXTurn] > 0 {
		return X
	}
	return O
}

// positionToTransition maps a board position (0-8) to the appropriate Petri net transition
// based on whose turn it is.
func positionToTransition(pos int, player uint8) int {
	return TransitionForMove(player, pos)
}

// isGameOver checks if the game is over (win or draw) based on the marking.
func isGameOver(m Marking) bool {
	// Check for win
	if m[PlaceWinX] > 0 || m[PlaceWinO] > 0 {
		return true
	}
	// Check for draw (no empty cells)
	for i := 0; i < 9; i++ {
		if m[P00+i] > 0 { // Cell is still empty
			return false
		}
	}
	return true
}

// getWinner returns the winner (X, O, or Empty) based on the marking.
func getWinner(m Marking) uint8 {
	if m[PlaceWinX] > 0 {
		return X
	}
	if m[PlaceWinO] > 0 {
		return O
	}
	return Empty
}

// turnCount calculates the number of moves made based on the marking.
func turnCount(m Marking) int {
	count := 0
	for i := 0; i < 9; i++ {
		if m[X00+i] > 0 || m[O00+i] > 0 {
			count++
		}
	}
	return count
}

// Handler returns the HTTP handler for ZK endpoints.
// Mount this at /zk on your service.
func (z *ZKIntegration) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", z.handleHealth)

	// Get or create ZK game state
	mux.HandleFunc("POST /game", z.handleCreateGame)
	mux.HandleFunc("GET /game/{id}", z.handleGetGame)

	// Make a move and generate proof
	mux.HandleFunc("POST /game/{id}/move", z.handleMove)

	// Check for win and generate proof
	mux.HandleFunc("POST /game/{id}/check-win", z.handleCheckWin)

	// Verify a proof
	mux.HandleFunc("POST /verify", z.handleVerify)

	// Get circuit info
	mux.HandleFunc("GET /circuits", z.handleCircuits)

	// Export Solidity verifier contract
	mux.HandleFunc("GET /verifier/{circuit}", z.handleExportVerifier)

	// Replay verification - verify entire game history
	mux.HandleFunc("POST /replay", z.handleReplay)

	return mux
}

func (z *ZKIntegration) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"circuits": []string{"transition", "win", "move"},
		"model":    "petri-net",
		"places":   NumPlaces,
		"transitions": NumTransitions,
	})
}

func (z *ZKIntegration) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	z.mu.Lock()
	defer z.mu.Unlock()

	game := NewPetriGame()
	id := fmt.Sprintf("zk-%d", len(z.games)+1)
	z.games[id] = game

	board := extractBoard(game.Marking)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"state_root": game.CurrentRoot().String(),
		"turn":       currentPlayer(game.Marking),
		"board":      board,
	})
}

func (z *ZKIntegration) handleGetGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	z.mu.RLock()
	game, ok := z.games[id]
	z.mu.RUnlock()

	if !ok {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	board := extractBoard(game.Marking)
	winner := getWinner(game.Marking)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"state_root": game.CurrentRoot().String(),
		"turn":       currentPlayer(game.Marking),
		"turn_count": turnCount(game.Marking),
		"board":      board,
		"is_over":    isGameOver(game.Marking),
		"winner":     winner,
		"roots":      rootsToStrings(game.Roots),
	})
}

// MoveRequest is the request body for making a move.
type MoveRequest struct {
	Position int `json:"position"` // 0-8
}

// MoveResponse is the response for a move with ZK proof.
type MoveResponse struct {
	Success       bool     `json:"success"`
	Position      int      `json:"position"`
	Player        uint8    `json:"player"`
	PreStateRoot  string   `json:"pre_state_root"`
	PostStateRoot string   `json:"post_state_root"`
	Board         [9]uint8 `json:"board"`
	TurnCount     int      `json:"turn_count"`
	IsOver        bool     `json:"is_over"`
	Winner        uint8    `json:"winner,omitempty"`
	Proof         *Proof   `json:"proof,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// Proof contains the ZK proof data.
type Proof struct {
	Circuit      string   `json:"circuit"`
	ProofHex     string   `json:"proof_hex"`
	PublicInputs []string `json:"public_inputs"`
	Verified     bool     `json:"verified"`

	// Solidity-compatible proof points (for on-chain verification)
	A        [2]string    `json:"a,omitempty"`
	B        [2][2]string `json:"b,omitempty"`
	C        [2]string    `json:"c,omitempty"`
	RawProof []string     `json:"raw_proof,omitempty"` // Flat array for calldata
}

func (z *ZKIntegration) handleMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	z.mu.Lock()
	game, ok := z.games[id]
	if !ok {
		z.mu.Unlock()
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		z.mu.Unlock()
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Determine which player's turn and map position to transition
	player := currentPlayer(game.Marking)
	transition := positionToTransition(req.Position, player)
	if transition < 0 {
		z.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MoveResponse{
			Success: false,
			Error:   "invalid position",
		})
		return
	}

	// Fire the Petri net transition
	witness, err := game.FireTransition(transition)
	z.mu.Unlock()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MoveResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Generate proof using Petri net transition circuit
	assignment := witness.ToPetriTransitionAssignment()
	proofResult, err := z.prover.Prove("transition", assignment)

	var proof *Proof
	if err == nil {
		// Verify the proof
		verifyErr := z.prover.Verify("transition", assignment)
		proof = proofResultToProof("transition", proofResult, verifyErr == nil)
	}

	// Get current state from updated marking
	board := extractBoard(game.Marking)
	winner := getWinner(game.Marking)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MoveResponse{
		Success:       true,
		Position:      req.Position,
		Player:        player,
		PreStateRoot:  witness.PreStateRoot.String(),
		PostStateRoot: witness.PostStateRoot.String(),
		Board:         board,
		TurnCount:     turnCount(game.Marking),
		IsOver:        isGameOver(game.Marking),
		Winner:        winner,
		Proof:         proof,
	})
}

// WinResponse is the response for checking a win with ZK proof.
type WinResponse struct {
	HasWinner bool   `json:"has_winner"`
	Winner    uint8  `json:"winner,omitempty"`
	StateRoot string `json:"state_root"`
	Proof     *Proof `json:"proof,omitempty"`
}

func (z *ZKIntegration) handleCheckWin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	z.mu.RLock()
	game, ok := z.games[id]
	z.mu.RUnlock()

	if !ok {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	// Get win witness (returns nil if no winner)
	witness := game.GetWinWitness()
	if witness == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WinResponse{
			HasWinner: false,
			StateRoot: game.CurrentRoot().String(),
		})
		return
	}

	// Generate win proof using Petri net win circuit
	assignment := witness.ToPetriWinAssignment()
	proofResult, err := z.prover.Prove("win", assignment)

	var proof *Proof
	if err == nil {
		// Verify the proof
		verifyErr := z.prover.Verify("win", assignment)
		proof = proofResultToProof("win", proofResult, verifyErr == nil)
	}

	// Determine winner from which place has tokens
	var winner uint8
	if witness.Winner == PlaceWinX {
		winner = X
	} else {
		winner = O
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(WinResponse{
		HasWinner: true,
		Winner:    winner,
		StateRoot: witness.StateRoot.String(),
		Proof:     proof,
	})
}

func (z *ZKIntegration) handleVerify(w http.ResponseWriter, r *http.Request) {
	// For now, delegate to the prover service
	z.service.Handler().ServeHTTP(w, r)
}

func (z *ZKIntegration) handleCircuits(w http.ResponseWriter, r *http.Request) {
	circuits := z.service.ListCircuits()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"circuits": circuits,
	})
}

func (z *ZKIntegration) handleExportVerifier(w http.ResponseWriter, r *http.Request) {
	circuitName := r.PathValue("circuit")

	solidity, err := z.prover.ExportVerifier(circuitName)
	if err != nil {
		http.Error(w, fmt.Sprintf("circuit not found: %s", circuitName), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_verifier.sol", circuitName))
	w.Write([]byte(solidity))
}

func rootsToStrings(roots []*big.Int) []string {
	result := make([]string, len(roots))
	for i, r := range roots {
		result[i] = r.String()
	}
	return result
}

func rawProofToHex(rawProof []*big.Int) string {
	var result string
	for _, p := range rawProof {
		result += fmt.Sprintf("%064x", p)
	}
	return result
}

// proofResultToProof converts a prover.ProofResult to our Proof struct with Solidity-compatible fields.
func proofResultToProof(circuit string, pr *prover.ProofResult, verified bool) *Proof {
	// Convert A, B, C to hex strings for Solidity
	a := [2]string{
		fmt.Sprintf("0x%064x", pr.A[0]),
		fmt.Sprintf("0x%064x", pr.A[1]),
	}
	b := [2][2]string{
		{fmt.Sprintf("0x%064x", pr.B[0][0]), fmt.Sprintf("0x%064x", pr.B[0][1])},
		{fmt.Sprintf("0x%064x", pr.B[1][0]), fmt.Sprintf("0x%064x", pr.B[1][1])},
	}
	c := [2]string{
		fmt.Sprintf("0x%064x", pr.C[0]),
		fmt.Sprintf("0x%064x", pr.C[1]),
	}

	// Convert RawProof to hex strings
	rawProof := make([]string, len(pr.RawProof))
	for i, p := range pr.RawProof {
		rawProof[i] = fmt.Sprintf("0x%064x", p)
	}

	return &Proof{
		Circuit:      circuit,
		ProofHex:     rawProofToHex(pr.RawProof),
		PublicInputs: pr.PublicInputs,
		Verified:     verified,
		A:            a,
		B:            b,
		C:            c,
		RawProof:     rawProof,
	}
}

// ReplayRequest is the request for verifying an entire game history.
type ReplayRequest struct {
	InitialRoot string       `json:"initial_root"`
	Moves       []ReplayMove `json:"moves"`
	WinProof    *ReplayProof `json:"win_proof,omitempty"`
}

// ReplayMove represents a move with its proof for replay verification.
type ReplayMove struct {
	Position     int    `json:"position"`
	Player       uint8  `json:"player"`
	PreRoot      string `json:"pre_root"`
	PostRoot     string `json:"post_root"`
	ProofVerified bool  `json:"proof_verified"` // Was proof verified when move was made
}

// ReplayProof contains proof metadata for verification status.
type ReplayProof struct {
	Circuit  string `json:"circuit"`
	Verified bool   `json:"verified"`
}

// ReplayResponse is the response for replay verification.
type ReplayResponse struct {
	Valid       bool               `json:"valid"`
	MoveCount   int                `json:"move_count"`
	MoveResults []MoveVerifyResult `json:"move_results"`
	WinVerified bool               `json:"win_verified,omitempty"`
	ChainValid  bool               `json:"chain_valid"`
	FinalRoot   string             `json:"final_root"`
	Error       string             `json:"error,omitempty"`
}

// MoveVerifyResult is the verification result for a single move.
type MoveVerifyResult struct {
	Move          int    `json:"move"`
	Position      int    `json:"position"`
	Player        uint8  `json:"player"`
	ProofVerified bool   `json:"proof_verified"`
	ChainValid    bool   `json:"chain_valid"`
	PreRoot       string `json:"pre_root"`
	PostRoot      string `json:"post_root"`
	Error         string `json:"error,omitempty"`
}

func (z *ZKIntegration) handleReplay(w http.ResponseWriter, r *http.Request) {
	var req ReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	response := ReplayResponse{
		MoveCount:   len(req.Moves),
		MoveResults: make([]MoveVerifyResult, len(req.Moves)),
		ChainValid:  true,
	}

	// Verify the state root chain continuity
	// This proves that each move's pre_state_root matches the previous move's post_state_root
	// forming an unbroken chain from the initial empty board to the final state
	expectedPreRoot := req.InitialRoot
	allProofsVerified := true
	var finalRoot string

	for i, move := range req.Moves {
		result := MoveVerifyResult{
			Move:          i + 1,
			Position:      move.Position,
			Player:        move.Player,
			ProofVerified: move.ProofVerified,
			PreRoot:       truncateRoot(move.PreRoot),
			PostRoot:      truncateRoot(move.PostRoot),
		}

		// Check state root chain continuity
		if move.PreRoot != expectedPreRoot {
			result.ChainValid = false
			result.Error = fmt.Sprintf("chain broken: expected pre_root %s, got %s",
				truncateRoot(expectedPreRoot), truncateRoot(move.PreRoot))
			response.ChainValid = false
		} else {
			result.ChainValid = true
		}

		// Track proof verification status
		if !move.ProofVerified {
			allProofsVerified = false
		}

		response.MoveResults[i] = result
		expectedPreRoot = move.PostRoot
		finalRoot = move.PostRoot
	}

	response.FinalRoot = truncateRoot(finalRoot)

	// Check win proof if provided
	if req.WinProof != nil {
		response.WinVerified = req.WinProof.Verified
	}

	// Game history is valid if:
	// 1. All state roots form an unbroken chain
	// 2. All move proofs were verified when made
	response.Valid = response.ChainValid && allProofsVerified

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func truncateRoot(root string) string {
	if len(root) > 20 {
		return root[:10] + "..." + root[len(root)-8:]
	}
	return root
}
