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

// BuildHandler returns the HTTP handler for the ZK-ODE service.
func (s *HTTPService) BuildHandler() http.Handler {
	mux := http.NewServeMux()

	// POST /api/prove — generate a Groth16 proof
	mux.HandleFunc("/api/prove", s.handleProve)

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

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
