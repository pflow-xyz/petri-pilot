// Package engine provides a runtime engine that wraps the metamodel.Runtime
// with event sourcing capabilities for petri-pilot generated applications.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sync"

	"github.com/pflow-xyz/petri-pilot/pkg/dsl"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/eventstore"
)

// Engine wraps metamodel.Runtime with event sourcing.
// It provides the execution layer for generated applications.
type Engine struct {
	schema *metamodel.Schema
	store  eventstore.Store

	// runtimes holds per-aggregate runtime instances
	runtimes map[string]*metamodel.Runtime
	mu       sync.RWMutex
}

// NewEngine creates a new engine from a metamodel schema and event store.
func NewEngine(schema *metamodel.Schema, store eventstore.Store) *Engine {
	return &Engine{
		schema:   schema,
		store:    store,
		runtimes: make(map[string]*metamodel.Runtime),
	}
}

// Schema returns the underlying metamodel schema.
func (e *Engine) Schema() *metamodel.Schema {
	return e.schema
}

// getOrCreateRuntime gets an existing runtime for an aggregate or creates a new one.
func (e *Engine) getOrCreateRuntime(aggregateID string) *metamodel.Runtime {
	e.mu.RLock()
	rt, exists := e.runtimes[aggregateID]
	e.mu.RUnlock()

	if exists {
		return rt
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if rt, exists = e.runtimes[aggregateID]; exists {
		return rt
	}

	// Create new runtime with guard evaluator
	rt = metamodel.NewRuntime(e.schema)
	rt.GuardEvaluator = dsl.NewEvaluator()
	e.runtimes[aggregateID] = rt
	return rt
}

// LoadState loads the runtime state for an aggregate by replaying events.
func (e *Engine) LoadState(ctx context.Context, aggregateID string) (*metamodel.Runtime, error) {
	// Get or create runtime
	rt := e.getOrCreateRuntime(aggregateID)

	// Read events from store
	events, err := e.store.Read(ctx, aggregateID, 0)
	if err != nil && err != eventstore.ErrStreamNotFound {
		return nil, fmt.Errorf("reading events: %w", err)
	}

	// Replay events to reconstruct state
	for _, event := range events {
		bindings, err := eventToBindings(event)
		if err != nil {
			return nil, fmt.Errorf("converting event %s to bindings: %w", event.ID, err)
		}

		// Extract action ID from event type
		actionID := eventTypeToActionID(event.Type)
		if actionID == "" {
			continue // Skip events that don't map to actions
		}

		// Apply the action (without re-checking guards for replay)
		rt.CheckConstraints = false
		if err := rt.ExecuteWithBindings(actionID, bindings); err != nil {
			// Log but don't fail on replay errors - event already occurred
			rt.CheckConstraints = true
			continue
		}
		rt.CheckConstraints = true
	}

	return rt, nil
}

// Execute fires an action on an aggregate and persists the resulting event.
func (e *Engine) Execute(ctx context.Context, aggregateID, actionID string, bindings metamodel.Bindings) error {
	// Load current state
	rt, err := e.LoadState(ctx, aggregateID)
	if err != nil {
		return err
	}

	// Check if action is enabled
	if !rt.Enabled(actionID) {
		return fmt.Errorf("action %s is not enabled", actionID)
	}

	// Execute with bindings (this evaluates guards and applies transformations)
	if err := rt.ExecuteWithBindings(actionID, bindings); err != nil {
		return fmt.Errorf("executing action: %w", err)
	}

	// Create and persist event
	eventType := actionIDToEventType(actionID)
	event, err := runtime.NewEvent(aggregateID, eventType, bindings)
	if err != nil {
		return fmt.Errorf("creating event: %w", err)
	}

	// Get current version for optimistic concurrency
	version, err := e.store.StreamVersion(ctx, aggregateID)
	if err != nil && err != eventstore.ErrStreamNotFound {
		return fmt.Errorf("getting stream version: %w", err)
	}

	// Append event
	_, err = e.store.Append(ctx, aggregateID, version, []*runtime.Event{event})
	if err != nil {
		return fmt.Errorf("appending event: %w", err)
	}

	return nil
}

// Enabled returns all enabled actions for an aggregate.
func (e *Engine) Enabled(ctx context.Context, aggregateID string) ([]string, error) {
	rt, err := e.LoadState(ctx, aggregateID)
	if err != nil {
		return nil, err
	}
	return rt.EnabledActions(), nil
}

// State returns the current snapshot state for an aggregate.
func (e *Engine) State(ctx context.Context, aggregateID string) (*metamodel.Snapshot, error) {
	rt, err := e.LoadState(ctx, aggregateID)
	if err != nil {
		return nil, err
	}
	return rt.Snapshot, nil
}

// Tokens returns the current token counts for an aggregate.
func (e *Engine) Tokens(ctx context.Context, aggregateID string) (map[string]int, error) {
	rt, err := e.LoadState(ctx, aggregateID)
	if err != nil {
		return nil, err
	}
	return rt.Snapshot.Tokens, nil
}

// Data returns the current data state for an aggregate.
func (e *Engine) Data(ctx context.Context, aggregateID string) (map[string]any, error) {
	rt, err := e.LoadState(ctx, aggregateID)
	if err != nil {
		return nil, err
	}
	return rt.Snapshot.Data, nil
}

// CheckGuard evaluates a guard expression with the given bindings.
func (e *Engine) CheckGuard(ctx context.Context, aggregateID, guardExpr string, bindings metamodel.Bindings) (bool, error) {
	rt, err := e.LoadState(ctx, aggregateID)
	if err != nil {
		return false, err
	}

	// Merge snapshot data into bindings for guard evaluation
	mergedBindings := make(metamodel.Bindings)
	maps.Copy(mergedBindings, bindings)
	for k, v := range rt.Snapshot.Data {
		mergedBindings[k] = v
	}
	for k, v := range rt.Snapshot.Tokens {
		mergedBindings[k] = v
	}

	// Convert bindings to map[string]any for DSL evaluation
	bindingsMap := make(map[string]any, len(mergedBindings))
	for k, v := range mergedBindings {
		bindingsMap[k] = v
	}

	return dsl.Evaluate(guardExpr, bindingsMap, nil)
}

// eventToBindings converts an event's data to metamodel.Bindings.
func eventToBindings(event *runtime.Event) (metamodel.Bindings, error) {
	bindings := make(metamodel.Bindings)
	if len(event.Data) == 0 {
		return bindings, nil
	}

	if err := json.Unmarshal(event.Data, &bindings); err != nil {
		return nil, err
	}
	return bindings, nil
}

// eventTypeToActionID converts an event type to an action ID.
// This is the inverse of actionIDToEventType.
// e.g., "Transferred" -> "transfer", "Minted" -> "mint"
func eventTypeToActionID(eventType string) string {
	// Simple heuristic: lowercase and remove common suffixes
	id := eventType

	// Remove common past tense suffixes
	if len(id) > 2 && id[len(id)-2:] == "ed" {
		id = id[:len(id)-2]
		// Handle doubled consonants (e.g., "Transferred" -> "Transfer" -> "transfer")
		if len(id) > 1 && id[len(id)-1] == id[len(id)-2] {
			id = id[:len(id)-1]
		}
		// Handle -e ending (e.g., "Validated" -> "Validat" -> "validate")
		if len(id) > 0 && isConsonant(rune(id[len(id)-1])) {
			// Check if original likely ended in -e
			id = id + "e"
		}
	}

	// Convert to lowercase
	result := ""
	for i, r := range id {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result += "_"
			}
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}

	return result
}

// actionIDToEventType converts an action ID to an event type.
// e.g., "transfer" -> "Transferred", "mint" -> "Minted"
func actionIDToEventType(actionID string) string {
	// Convert to PascalCase
	result := ""
	capitalizeNext := true
	for _, r := range actionID {
		if r == '_' || r == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext && r >= 'a' && r <= 'z' {
			result += string(r - 32)
			capitalizeNext = false
		} else {
			result += string(r)
		}
	}

	// Add past tense suffix
	if len(result) > 0 {
		lastChar := result[len(result)-1]
		if lastChar == 'e' {
			result += "d"
		} else {
			result += "ed"
		}
	}

	return result
}

func isConsonant(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return false
	default:
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}
}
