// Package zktictactoe provides ZK proof integration for the tic-tac-toe service.
package zktictactoe

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"

	"github.com/pflow-xyz/go-pflow/prover"
)

// ZKIntegration provides ZK proof endpoints for tic-tac-toe.
type ZKIntegration struct {
	prover  *prover.Prover
	service *prover.Service
	games   map[string]*Game // in-memory game state for ZK proofs
	mu      sync.RWMutex
}

// NewZKIntegration creates a new ZK integration with compiled circuits.
func NewZKIntegration() (*ZKIntegration, error) {
	p := prover.NewProver()

	// Compile circuits
	moveCircuit, err := p.CompileCircuit("move", &MoveCircuit{})
	if err != nil {
		return nil, fmt.Errorf("failed to compile move circuit: %w", err)
	}
	p.StoreCircuit("move", moveCircuit)

	winCircuit, err := p.CompileCircuit("win", &WinCircuit{})
	if err != nil {
		return nil, fmt.Errorf("failed to compile win circuit: %w", err)
	}
	p.StoreCircuit("win", winCircuit)

	factory := &TicTacToeWitnessFactory{}

	return &ZKIntegration{
		prover:  p,
		service: prover.NewService(p, factory),
		games:   make(map[string]*Game),
	}, nil
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

	return mux
}

func (z *ZKIntegration) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"circuits": []string{"move", "win"},
	})
}

func (z *ZKIntegration) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	z.mu.Lock()
	defer z.mu.Unlock()

	game := NewGame()
	id := fmt.Sprintf("zk-%d", len(z.games)+1)
	z.games[id] = game

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"state_root": game.CurrentRoot().String(),
		"turn":       game.CurrentPlayer(),
		"board":      game.Board,
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

	var winner uint8
	if w := CheckWinner(game.Board); w != Empty {
		winner = w
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"state_root": game.CurrentRoot().String(),
		"turn":       game.CurrentPlayer(),
		"turn_count": game.TurnCount,
		"board":      game.Board,
		"is_over":    game.IsOver(),
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

	// Make the move
	witness, err := game.MakeMove(req.Position)
	z.mu.Unlock()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MoveResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Generate proof
	assignment := witness.ToMoveAssignment()
	proofResult, err := z.prover.Prove("move", assignment)

	var proof *Proof
	if err == nil {
		// Verify the proof
		verifyErr := z.prover.Verify("move", assignment)

		proof = &Proof{
			Circuit:      "move",
			ProofHex:     rawProofToHex(proofResult.RawProof),
			PublicInputs: proofResult.PublicInputs,
			Verified:     verifyErr == nil,
		}
	}

	// Check for winner
	var winner uint8
	if w := CheckWinner(game.Board); w != Empty {
		winner = w
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MoveResponse{
		Success:       true,
		Position:      req.Position,
		Player:        witness.Player,
		PreStateRoot:  witness.PreStateRoot.String(),
		PostStateRoot: witness.PostStateRoot.String(),
		Board:         game.Board,
		TurnCount:     game.TurnCount,
		IsOver:        game.IsOver(),
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

	witness := game.CheckWin()
	if witness == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(WinResponse{
			HasWinner: false,
			StateRoot: game.CurrentRoot().String(),
		})
		return
	}

	// Generate win proof
	assignment := witness.ToWinAssignment()
	proofResult, err := z.prover.Prove("win", assignment)

	var proof *Proof
	if err == nil {
		// Verify the proof
		verifyErr := z.prover.Verify("win", assignment)

		proof = &Proof{
			Circuit:      "win",
			ProofHex:     rawProofToHex(proofResult.RawProof),
			PublicInputs: proofResult.PublicInputs,
			Verified:     verifyErr == nil,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(WinResponse{
		HasWinner: true,
		Winner:    witness.Winner,
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
