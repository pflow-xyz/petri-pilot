// Package eventstore provides event storage abstractions for event sourcing.
package eventstore

import (
	"context"
	"errors"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime"
)

// Common errors.
var (
	ErrStreamNotFound     = errors.New("stream not found")
	ErrConcurrencyConflict = errors.New("concurrency conflict: expected version mismatch")
	ErrEventNotFound      = errors.New("event not found")
	ErrStoreClosed        = errors.New("store is closed")
)

// Store is the interface for event storage backends.
type Store interface {
	// Append adds events to a stream with optimistic concurrency control.
	// expectedVersion should match the current stream version, or -1 for new streams.
	// Returns the new stream version after appending.
	Append(ctx context.Context, streamID string, expectedVersion int, events []*runtime.Event) (int, error)

	// Read retrieves events from a stream starting at fromVersion.
	// Returns events in order from fromVersion to the end of the stream.
	Read(ctx context.Context, streamID string, fromVersion int) ([]*runtime.Event, error)

	// ReadAll retrieves all events matching the filter.
	ReadAll(ctx context.Context, filter runtime.EventFilter) ([]*runtime.Event, error)

	// StreamVersion returns the current version of a stream.
	// Returns -1 if the stream doesn't exist.
	StreamVersion(ctx context.Context, streamID string) (int, error)

	// Subscribe creates a subscription for new events.
	// The subscription receives events matching the filter as they are appended.
	Subscribe(ctx context.Context, filter runtime.EventFilter) (runtime.Subscription, error)

	// Close releases any resources held by the store.
	Close() error
}

// AppendResult contains the result of an append operation.
type AppendResult struct {
	// StreamID is the stream that was appended to.
	StreamID string

	// FromVersion is the version before the append.
	FromVersion int

	// ToVersion is the version after the append.
	ToVersion int

	// Events contains the appended events with assigned versions.
	Events []*runtime.Event
}

// Snapshot represents a point-in-time state snapshot.
type Snapshot struct {
	// StreamID identifies the aggregate.
	StreamID string

	// Version is the event version this snapshot represents.
	Version int

	// State is the serialized aggregate state.
	State []byte
}

// SnapshotStore provides snapshot storage for aggregate state.
type SnapshotStore interface {
	// Save stores a snapshot.
	Save(ctx context.Context, snapshot *Snapshot) error

	// Load retrieves the latest snapshot for a stream.
	// Returns nil if no snapshot exists.
	Load(ctx context.Context, streamID string) (*Snapshot, error)

	// Delete removes snapshots for a stream.
	Delete(ctx context.Context, streamID string) error
}
