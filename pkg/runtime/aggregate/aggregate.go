// Package aggregate provides event-sourced aggregate abstractions.
package aggregate

import (
	"context"
	"errors"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/eventstore"
)

// Common errors.
var (
	ErrAggregateNotFound = errors.New("aggregate not found")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrCommandRejected   = errors.New("command rejected by guard")
)

// Aggregate is the interface for event-sourced aggregates.
type Aggregate interface {
	// ID returns the aggregate identifier.
	ID() string

	// Version returns the current event version.
	Version() int

	// Apply applies an event to update the aggregate state.
	// This should be a pure function with no side effects.
	Apply(event *runtime.Event) error

	// State returns the current aggregate state.
	State() any
}

// Command represents an intent to change aggregate state.
type Command struct {
	// Type is the command type name.
	Type string

	// AggregateID is the target aggregate.
	AggregateID string

	// Payload contains the command data.
	Payload any

	// Metadata contains optional context.
	Metadata map[string]string
}

// CommandHandler processes commands and produces events.
type CommandHandler func(ctx context.Context, agg Aggregate, cmd Command) ([]*runtime.Event, error)

// Repository provides aggregate persistence.
type Repository interface {
	// Load retrieves an aggregate by ID, replaying events to rebuild state.
	Load(ctx context.Context, id string) (Aggregate, error)

	// Save persists new events for an aggregate.
	Save(ctx context.Context, agg Aggregate, events []*runtime.Event) error

	// Execute loads an aggregate, applies a command, and saves the resulting events.
	Execute(ctx context.Context, id string, cmd Command, handler CommandHandler) error
}

// Factory creates new aggregate instances.
type Factory func(id string) Aggregate

// BaseRepository provides a standard Repository implementation.
type BaseRepository struct {
	store   eventstore.Store
	factory Factory
}

// NewRepository creates a new aggregate repository.
func NewRepository(store eventstore.Store, factory Factory) *BaseRepository {
	return &BaseRepository{
		store:   store,
		factory: factory,
	}
}

// Load retrieves an aggregate by ID.
func (r *BaseRepository) Load(ctx context.Context, id string) (Aggregate, error) {
	agg := r.factory(id)

	events, err := r.store.Read(ctx, id, 0)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if err := agg.Apply(event); err != nil {
			return nil, err
		}
	}

	return agg, nil
}

// Save persists new events for an aggregate.
func (r *BaseRepository) Save(ctx context.Context, agg Aggregate, events []*runtime.Event) error {
	if len(events) == 0 {
		return nil
	}

	_, err := r.store.Append(ctx, agg.ID(), agg.Version(), events)
	return err
}

// Execute loads an aggregate, applies a command, and saves the resulting events.
func (r *BaseRepository) Execute(ctx context.Context, id string, cmd Command, handler CommandHandler) error {
	agg, err := r.Load(ctx, id)
	if err != nil {
		return err
	}

	events, err := handler(ctx, agg, cmd)
	if err != nil {
		return err
	}

	return r.Save(ctx, agg, events)
}

// Ensure BaseRepository implements Repository.
var _ Repository = (*BaseRepository)(nil)
