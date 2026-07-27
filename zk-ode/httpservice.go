package zkode

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"

	"github.com/pflow-xyz/go-pflow/prover"
	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

const ServiceName = "zk-ode"

func init() {
	serve.Register(ServiceName, NewHTTPService)
}

// HTTPService wraps the ZK-ODE prover as a serve.Service for the multi-service server.
type HTTPService struct {
	svc    *prover.Service
	prover *prover.Prover
}

// NewHTTPService creates a new ZK-ODE HTTP service.
func NewHTTPService() (serve.Service, error) {
	log.Println("Initializing ZK-ODE prover service...")
	svc, err := NewZkODEService()
	if err != nil {
		return nil, fmt.Errorf("failed to create ZK-ODE service: %w", err)
	}

	return &HTTPService{
		svc:    svc,
		prover: svc.Prover(),
	}, nil
}

func (s *HTTPService) Name() string { return ServiceName }

func (s *HTTPService) Close() error { return nil }

// proveRequest is the JSON body for POST /api/prove.
type proveRequest struct {
	StepSize       float64   `json:"step_size"`
	Rates          []float64 `json:"rates"`
	InitialMarking []float64 `json:"initial_marking"`
	NumSteps       int       `json:"num_steps"`
}

// evaluateRequest is the JSON body for POST /api/evaluate.
type evaluateRequest struct {
	Board  [3][3]string `json:"board"`
	Player string       `json:"player"`
}

// proveOptimalRequest is the JSON body for POST /api/prove-optimal.
type proveOptimalRequest struct {
	Board  [3][3]string `json:"board"`
	Player string       `json:"player"`
	Move   [2]int       `json:"move"`
}

// BuildHandler returns the HTTP handler for the ZK-ODE service.
func (s *HTTPService) BuildHandler() http.Handler {
	mux := http.NewServeMux()

	// POST /api/prove — generate a Groth16 proof
	mux.HandleFunc("/api/prove", s.handleProve)

	// POST /api/evaluate — evaluate ODE heatmap for a board position
	mux.HandleFunc("/api/evaluate", s.handleEvaluate)

	// POST /api/prove-optimal — prove the optimal move with ZK
	mux.HandleFunc("/api/prove-optimal", s.handleProveOptimal)

	// GET /api/health
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"circuits": s.svc.ListCircuits(),
		})
	})

	// GET /api/circuits
	mux.HandleFunc("/api/circuits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.svc.ListCircuits())
	})

	return mux
}

func (s *HTTPService) handleProve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	var req proveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if len(req.Rates) != NumTransitions {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("expected %d rates, got %d", NumTransitions, len(req.Rates)))
		return
	}
	if len(req.InitialMarking) != NumPlaces {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("expected %d initial marking values, got %d", NumPlaces, len(req.InitialMarking)))
		return
	}
	if req.NumSteps < 1 {
		req.NumSteps = 1
	}

	// Convert float parameters to fixed-point big.Int for the circuit
	h := FixFromFloat(req.StepSize)
	var rates [NumTransitions]*big.Int
	for i := 0; i < NumTransitions; i++ {
		rates[i] = FixFromFloat(req.Rates[i])
	}
	var marking [NumPlaces]*big.Int
	for i := 0; i < NumPlaces; i++ {
		marking[i] = FixFromFloat(req.InitialMarking[i])
	}

	state := NewODEState(marking)

	type stepResult struct {
		Step  int                 `json:"step"`
		Proof *prover.ProofResult `json:"proof"`
	}
	results := make([]stepResult, 0, req.NumSteps)

	for i := 0; i < req.NumSteps; i++ {
		proof, postState, err := ProveStep(s.prover, state, h, rates)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("proof generation failed at step %d: %v", i, err))
			return
		}
		results = append(results, stepResult{Step: i, Proof: proof})
		state = postState
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"steps":  results,
		"status": "ok",
	})
}

func (s *HTTPService) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	var req evaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Player != "X" && req.Player != "O" {
		jsonError(w, http.StatusBadRequest, "player must be 'X' or 'O'")
		return
	}

	result := EvaluateAllMoves(Board(req.Board), req.Player)

	// Build values map keyed by "RC" (row+col)
	values := make(map[string]float64)
	for _, s := range result.Scores {
		key := fmt.Sprintf("%d%d", s.Row, s.Col)
		values[key] = s.Adjusted
	}

	resp := map[string]interface{}{
		"values": values,
		"scores": result.Scores,
		"player": result.Player,
		"status": "ok",
	}
	if result.Optimal != nil {
		resp["optimal"] = fmt.Sprintf("%d%d", result.Optimal.Row, result.Optimal.Col)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPService) handleProveOptimal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	var req proveOptimalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Player != "X" && req.Player != "O" {
		jsonError(w, http.StatusBadRequest, "player must be 'X' or 'O'")
		return
	}

	row, col := req.Move[0], req.Move[1]
	if row < 0 || row > 2 || col < 0 || col > 2 {
		jsonError(w, http.StatusBadRequest, "move must be [row, col] with 0 <= row,col <= 2")
		return
	}
	if req.Board[row][col] != "" {
		jsonError(w, http.StatusBadRequest, "cell is already occupied")
		return
	}

	// Evaluate all moves to determine optimality
	result := EvaluateAllMoves(Board(req.Board), req.Player)

	// Find the score for the requested move
	var moveScore *MoveScore
	for i := range result.Scores {
		if result.Scores[i].Row == row && result.Scores[i].Col == col {
			moveScore = &result.Scores[i]
			break
		}
	}
	if moveScore == nil {
		jsonError(w, http.StatusBadRequest, "move not found in available positions")
		return
	}

	isOptimal := result.Optimal != nil &&
		result.Optimal.Row == row && result.Optimal.Col == col

	// Build the hypothetical state for the chosen move
	hypotheticalBoard := Board(req.Board)
	hypotheticalBoard[row][col] = req.Player
	opponent := "O"
	if req.Player == "O" {
		opponent = "X"
	}

	h := FixFromFloat(0.01)

	// Use TTT heatmap circuit if enabled, otherwise fall back to cascade
	if _, hasTTT := s.prover.GetCircuit("ttt_heatmap"); hasTTT {
		state := BoardToTTTODEState(hypotheticalBoard, opponent)
		proof, _, witness, err := ProveTTTHeatmapStep(s.prover, state, h)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("TTT heatmap proof generation failed: %v", err))
			return
		}

		// Cell position for the chosen move
		cellPos := row*3 + col

		// Convert heatmap scores to float for response
		scoreValues := make(map[string]float64)
		for i := 0; i < 9; i++ {
			scoreValues[fmt.Sprintf("cell_%d%d", i/3, i%3)] = FixToFloat(witness.HeatmapScores[i])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"proof":           proof,
			"is_optimal":      isOptimal,
			"objective_value": moveScore.Adjusted,
			"scores":          result.Scores,
			"cell_position":   cellPos,
			"heatmap_scores":  scoreValues,
			"circuit":         "ttt_heatmap",
			"pre_state_root":  fmt.Sprintf("0x%s", state.Root.Text(16)),
			"status":          "ok",
		})
		return
	}

	// Fallback: cascade circuit (legacy)
	net := BuildTicTacToeNet()
	hypotheticalState := buildStateWithMove(Board(req.Board), req.Player, row, col)
	rates := net.SetRates(nil)

	var fixedRates [NumTransitions]*big.Int
	fixedRates[0] = FixFromFloat(rates["x_play_00"])
	fixedRates[1] = FixFromFloat(rates["x_play_01"])

	var marking [NumPlaces]*big.Int
	marking[0] = FixFromFloat(hypotheticalState["p00"])
	marking[1] = FixFromFloat(hypotheticalState["p01"])
	marking[2] = FixFromFloat(hypotheticalState["p02"])

	state := NewODEState(marking)

	proof, _, err := ProveStep(s.prover, state, h, fixedRates)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("proof generation failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proof":           proof,
		"is_optimal":      isOptimal,
		"objective_value": moveScore.Adjusted,
		"scores":          result.Scores,
		"circuit":         "tsit5_step",
		"status":          "ok",
	})
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
