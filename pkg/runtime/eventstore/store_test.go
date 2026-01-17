package eventstore_test

import (
	"context"
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/eventstore"
)

func TestMemoryStore(t *testing.T) {
	runStoreTests(t, func() eventstore.Store {
		return eventstore.NewMemoryStore()
	})
}

func TestSQLiteStore(t *testing.T) {
	runStoreTests(t, func() eventstore.Store {
		store, err := eventstore.NewSQLiteStore(":memory:")
		if err != nil {
			t.Fatalf("failed to create sqlite store: %v", err)
		}
		return store
	})
}

func runStoreTests(t *testing.T, newStore func() eventstore.Store) {
	t.Run("AppendAndRead", func(t *testing.T) {
		store := newStore()
		defer store.Close()
		ctx := context.Background()

		// Create events
		event1, _ := runtime.NewEvent("stream-1", "Created", map[string]string{"name": "test"})
		event2, _ := runtime.NewEvent("stream-1", "Updated", map[string]string{"name": "updated"})

		// Append to new stream
		version, err := store.Append(ctx, "stream-1", -1, []*runtime.Event{event1})
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}
		if version != 0 {
			t.Errorf("expected version 0, got %d", version)
		}

		// Append more events
		version, err = store.Append(ctx, "stream-1", 0, []*runtime.Event{event2})
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}
		if version != 1 {
			t.Errorf("expected version 1, got %d", version)
		}

		// Read all events
		events, err := store.Read(ctx, "stream-1", 0)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}

		// Check event data
		if events[0].Type != "Created" {
			t.Errorf("expected type Created, got %s", events[0].Type)
		}
		if events[1].Type != "Updated" {
			t.Errorf("expected type Updated, got %s", events[1].Type)
		}
	})

	t.Run("ConcurrencyConflict", func(t *testing.T) {
		store := newStore()
		defer store.Close()
		ctx := context.Background()

		event1, _ := runtime.NewEvent("stream-1", "Created", nil)
		event2, _ := runtime.NewEvent("stream-1", "Updated", nil)

		// Append first event
		_, err := store.Append(ctx, "stream-1", -1, []*runtime.Event{event1})
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}

		// Try to append with wrong expected version (5 instead of 0)
		_, err = store.Append(ctx, "stream-1", 5, []*runtime.Event{event2})
		if err != eventstore.ErrConcurrencyConflict {
			t.Errorf("expected concurrency conflict, got: %v", err)
		}

		// Append with correct version should succeed
		_, err = store.Append(ctx, "stream-1", 0, []*runtime.Event{event2})
		if err != nil {
			t.Errorf("append with correct version failed: %v", err)
		}
	})

	t.Run("StreamVersion", func(t *testing.T) {
		store := newStore()
		defer store.Close()
		ctx := context.Background()

		// Non-existent stream
		version, err := store.StreamVersion(ctx, "stream-1")
		if err != nil {
			t.Fatalf("stream version failed: %v", err)
		}
		if version != -1 {
			t.Errorf("expected version -1 for non-existent stream, got %d", version)
		}

		// Append event
		event, _ := runtime.NewEvent("stream-1", "Created", nil)
		_, err = store.Append(ctx, "stream-1", -1, []*runtime.Event{event})
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}

		// Check version
		version, err = store.StreamVersion(ctx, "stream-1")
		if err != nil {
			t.Fatalf("stream version failed: %v", err)
		}
		if version != 0 {
			t.Errorf("expected version 0, got %d", version)
		}
	})

	t.Run("ReadFromVersion", func(t *testing.T) {
		store := newStore()
		defer store.Close()
		ctx := context.Background()

		// Append 3 events
		for i := 0; i < 3; i++ {
			event, _ := runtime.NewEvent("stream-1", "Event", i)
			expectedVersion := i - 1
			_, err := store.Append(ctx, "stream-1", expectedVersion, []*runtime.Event{event})
			if err != nil {
				t.Fatalf("append failed: %v", err)
			}
		}

		// Read from version 1
		events, err := store.Read(ctx, "stream-1", 1)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
		if events[0].Version != 1 {
			t.Errorf("expected first event version 1, got %d", events[0].Version)
		}
	})

	t.Run("ReadAllWithFilter", func(t *testing.T) {
		store := newStore()
		defer store.Close()
		ctx := context.Background()

		// Append events to multiple streams
		event1, _ := runtime.NewEvent("stream-1", "TypeA", nil)
		event2, _ := runtime.NewEvent("stream-1", "TypeB", nil)
		event3, _ := runtime.NewEvent("stream-2", "TypeA", nil)

		store.Append(ctx, "stream-1", -1, []*runtime.Event{event1, event2})
		store.Append(ctx, "stream-2", -1, []*runtime.Event{event3})

		// Filter by type
		events, err := store.ReadAll(ctx, runtime.EventFilter{
			Types: []string{"TypeA"},
		})
		if err != nil {
			t.Fatalf("read all failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 TypeA events, got %d", len(events))
		}

		// Filter by stream
		events, err = store.ReadAll(ctx, runtime.EventFilter{
			StreamID: "stream-1",
		})
		if err != nil {
			t.Fatalf("read all failed: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 events in stream-1, got %d", len(events))
		}
	})
}
