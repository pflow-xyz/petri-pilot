// Package api provides HTTP API abstractions for generated applications.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/aggregate"
)

// TransitionRequest represents a request to fire a transition.
type TransitionRequest struct {
	// TransitionID is the transition to fire.
	TransitionID string `json:"transition_id"`

	// AggregateID is the target aggregate.
	AggregateID string `json:"aggregate_id"`

	// Data is the transition payload.
	Data json.RawMessage `json:"data,omitempty"`
}

// TransitionResult represents the result of firing a transition.
type TransitionResult struct {
	// Success indicates if the transition fired successfully.
	Success bool `json:"success"`

	// AggregateID is the affected aggregate.
	AggregateID string `json:"aggregate_id"`

	// Version is the new aggregate version.
	Version int `json:"version"`

	// State is the new aggregate state (optional).
	State any `json:"state,omitempty"`

	// EnabledTransitions lists transitions that can now fire.
	EnabledTransitions []string `json:"enabled,omitempty"`

	// Error contains error details if the transition failed.
	Error string `json:"error,omitempty"`
}

// StateResponse represents aggregate state.
type StateResponse struct {
	// AggregateID is the aggregate identifier.
	AggregateID string `json:"aggregate_id"`

	// Version is the current event version.
	Version int `json:"version"`

	// State is the current state.
	State any `json:"state"`

	// Places contains Petri net place markings (if applicable).
	Places map[string]int `json:"places,omitempty"`

	// EnabledTransitions lists transitions that can fire.
	EnabledTransitions []string `json:"enabled_transitions,omitempty"`
}

// TransitionHandler handles transition requests.
type TransitionHandler interface {
	// Handle processes a transition request.
	Handle(ctx context.Context, req TransitionRequest) (*TransitionResult, error)
}

// TransitionHandlerFunc is a function adapter for TransitionHandler.
type TransitionHandlerFunc func(ctx context.Context, req TransitionRequest) (*TransitionResult, error)

// Handle implements TransitionHandler.
func (f TransitionHandlerFunc) Handle(ctx context.Context, req TransitionRequest) (*TransitionResult, error) {
	return f(ctx, req)
}

// StateHandler handles state queries.
type StateHandler interface {
	// Get retrieves aggregate state.
	Get(ctx context.Context, aggregateID string) (*StateResponse, error)
}

// StateHandlerFunc is a function adapter for StateHandler.
type StateHandlerFunc func(ctx context.Context, aggregateID string) (*StateResponse, error)

// Get implements StateHandler.
func (f StateHandlerFunc) Get(ctx context.Context, aggregateID string) (*StateResponse, error) {
	return f(ctx, aggregateID)
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// HTTPHandler wraps handlers with HTTP semantics.
type HTTPHandler struct {
	repo aggregate.Repository
}

// NewHTTPHandler creates a new HTTP handler.
func NewHTTPHandler(repo aggregate.Repository) *HTTPHandler {
	return &HTTPHandler{repo: repo}
}

// JSON writes a JSON response.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Error writes an error response.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{
		Code:    code,
		Message: message,
	})
}

// DecodeJSON decodes a JSON request body.
func DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

// Middleware is an HTTP middleware function.
type Middleware func(http.Handler) http.Handler

// Chain combines multiple middleware into one.
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// RequestID middleware adds a request ID to the context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger middleware logs requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production, use a proper logger
		next.ServeHTTP(w, r)
	})
}

// Recovery middleware recovers from panics.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS middleware adds CORS headers.
func CORS(origins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range origins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

const requestIDKey contextKey = "request_id"

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

var requestCounter int64

func generateRequestID() string {
	requestCounter++
	return fmt.Sprintf("req-%d", requestCounter)
}
