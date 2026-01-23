package aggregate

import (
	"fmt"
	"sync"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime"
)

// Base provides common aggregate functionality that can be embedded.
type Base[S any] struct {
	id       string
	version  int
	state    S
	handlers map[string]func(*S, *runtime.Event) error
	mu       sync.RWMutex
}

// NewBase creates a new base aggregate with the given ID and initial state.
func NewBase[S any](id string, initialState S) *Base[S] {
	return &Base[S]{
		id:       id,
		version:  -1,
		state:    initialState,
		handlers: make(map[string]func(*S, *runtime.Event) error),
	}
}

// ID returns the aggregate identifier.
func (b *Base[S]) ID() string {
	return b.id
}

// Version returns the current event version.
func (b *Base[S]) Version() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.version
}

// State returns the current aggregate state.
func (b *Base[S]) State() any {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// TypedState returns the current state with proper type.
func (b *Base[S]) TypedState() S {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// RegisterHandler registers an event handler for a specific event type.
func (b *Base[S]) RegisterHandler(eventType string, handler func(*S, *runtime.Event) error) {
	b.handlers[eventType] = handler
}

// Apply applies an event to update the aggregate state.
func (b *Base[S]) Apply(event *runtime.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	handler, ok := b.handlers[event.Type]
	if !ok {
		return fmt.Errorf("no handler for event type: %s", event.Type)
	}

	if err := handler(&b.state, event); err != nil {
		return err
	}

	b.version = event.Version
	return nil
}

// StateMachine provides Petri net state machine semantics on top of an aggregate.
type StateMachine[S any] struct {
	*Base[S]
	places      map[string]int        // Current token counts
	transitions map[string]Transition // Transition definitions
}

// Transition defines a Petri net transition.
type Transition struct {
	// ID is the transition identifier.
	ID string

	// Inputs are the input places with required token counts.
	Inputs map[string]int

	// Outputs are the output places with produced token counts.
	Outputs map[string]int

	// Inhibitors are places that block firing if they have any tokens.
	// Unlike Inputs, inhibitor arcs don't consume tokens - they just check for absence.
	Inhibitors map[string]bool

	// Guard is an optional condition that must be true to fire.
	Guard func(state any) bool

	// EventType is the event type to emit when fired.
	EventType string
}

// NewStateMachine creates a new state machine aggregate.
func NewStateMachine[S any](id string, initialState S, initialPlaces map[string]int) *StateMachine[S] {
	places := make(map[string]int)
	for k, v := range initialPlaces {
		places[k] = v
	}

	return &StateMachine[S]{
		Base:        NewBase(id, initialState),
		places:      places,
		transitions: make(map[string]Transition),
	}
}

// AddTransition registers a transition.
func (sm *StateMachine[S]) AddTransition(t Transition) {
	sm.transitions[t.ID] = t
}

// Apply applies an event to update the state machine, including places.
// This overrides Base.Apply to also update token distribution.
func (sm *StateMachine[S]) Apply(event *runtime.Event) error {
	// First update places under our lock
	sm.mu.Lock()

	// Find the transition by event type
	var transition *Transition
	for _, t := range sm.transitions {
		if t.EventType == event.Type || t.ID == event.Type {
			transition = &t
			break
		}
	}

	// Update places if we found a matching transition
	if transition != nil {
		// Remove input tokens
		for place, count := range transition.Inputs {
			sm.places[place] -= count
		}
		// Add output tokens
		for place, count := range transition.Outputs {
			sm.places[place] += count
		}
	}

	sm.mu.Unlock()

	// Call base Apply for state handler and version update (it has its own lock)
	return sm.Base.Apply(event)
}

// CanFire checks if a transition is enabled.
func (sm *StateMachine[S]) CanFire(transitionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	t, ok := sm.transitions[transitionID]
	if !ok {
		return false
	}

	// Check input places have enough tokens
	for place, required := range t.Inputs {
		if sm.places[place] < required {
			return false
		}
	}

	// Check inhibitor arcs - blocked if any inhibitor place has tokens
	for place := range t.Inhibitors {
		if sm.places[place] > 0 {
			return false
		}
	}

	// Check guard
	if t.Guard != nil && !t.Guard(sm.state) {
		return false
	}

	return true
}

// EnabledTransitions returns all transitions that can currently fire.
func (sm *StateMachine[S]) EnabledTransitions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var enabled []string
	for id := range sm.transitions {
		if sm.canFireLocked(id) {
			enabled = append(enabled, id)
		}
	}
	return enabled
}

func (sm *StateMachine[S]) canFireLocked(transitionID string) bool {
	t, ok := sm.transitions[transitionID]
	if !ok {
		return false
	}

	for place, required := range t.Inputs {
		if sm.places[place] < required {
			return false
		}
	}

	// Check inhibitor arcs - blocked if any inhibitor place has tokens
	for place := range t.Inhibitors {
		if sm.places[place] > 0 {
			return false
		}
	}

	if t.Guard != nil && !t.Guard(sm.state) {
		return false
	}

	return true
}

// Fire checks if a transition can fire and creates an event.
// It does NOT update places - that's done by Apply when the event is applied.
// Returns an event representing the transition, or an error if the transition cannot fire.
func (sm *StateMachine[S]) Fire(transitionID string, data any) (*runtime.Event, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	t, ok := sm.transitions[transitionID]
	if !ok {
		return nil, fmt.Errorf("unknown transition: %s", transitionID)
	}

	// Check inputs
	for place, required := range t.Inputs {
		if sm.places[place] < required {
			return nil, fmt.Errorf("%w: insufficient tokens in %s", ErrInvalidTransition, place)
		}
	}

	// Check inhibitor arcs - blocked if any inhibitor place has tokens
	for place := range t.Inhibitors {
		if sm.places[place] > 0 {
			return nil, fmt.Errorf("%w: inhibited by tokens in %s", ErrInvalidTransition, place)
		}
	}

	// Check guard
	if t.Guard != nil && !t.Guard(sm.state) {
		return nil, ErrCommandRejected
	}

	// Create event (places are updated when Apply is called)
	eventType := t.EventType
	if eventType == "" {
		eventType = transitionID
	}

	return runtime.NewEvent(sm.id, eventType, data)
}

// Places returns a copy of the current place markings.
func (sm *StateMachine[S]) Places() map[string]int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]int)
	for k, v := range sm.places {
		result[k] = v
	}
	return result
}

// Tokens returns the token count for a specific place.
func (sm *StateMachine[S]) Tokens(place string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.places[place]
}
