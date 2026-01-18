package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a SQLite-backed event store.
type SQLiteStore struct {
	db            *sql.DB
	mu            sync.RWMutex
	subscriptions map[string]*sqliteSubscription
	closed        bool
}

// NewSQLiteStore creates a new SQLite event store.
// dsn can be ":memory:" for in-memory or a file path.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL: %w", err)
	}

	store := &SQLiteStore{
		db:            db,
		subscriptions: make(map[string]*sqliteSubscription),
	}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			stream_id TEXT NOT NULL,
			type TEXT NOT NULL,
			version INTEGER NOT NULL,
			timestamp TEXT NOT NULL,
			data TEXT NOT NULL,
			metadata TEXT,
			UNIQUE(stream_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_events_stream ON events(stream_id, version);
		CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
		CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

		CREATE TABLE IF NOT EXISTS snapshots (
			stream_id TEXT PRIMARY KEY,
			version INTEGER NOT NULL,
			state BLOB NOT NULL,
			created_at TEXT NOT NULL
		);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Append adds events to a stream with optimistic concurrency control.
func (s *SQLiteStore) Append(ctx context.Context, streamID string, expectedVersion int, events []*runtime.Event) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStoreClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current version
	var currentVersion int
	err = tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), -1) FROM events WHERE stream_id = ?",
		streamID,
	).Scan(&currentVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to get stream version: %w", err)
	}

	// Check concurrency
	if expectedVersion >= 0 && currentVersion != expectedVersion {
		return 0, ErrConcurrencyConflict
	}

	// Insert events
	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO events (id, stream_id, type, version, timestamp, data, metadata) VALUES (?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, event := range events {
		event.ID = uuid.New().String()
		event.StreamID = streamID
		event.Version = currentVersion + i + 1

		var metadata []byte
		if event.Metadata != nil {
			metadata, _ = json.Marshal(event.Metadata)
		}

		_, err = stmt.ExecContext(ctx,
			event.ID,
			event.StreamID,
			event.Type,
			event.Version,
			event.Timestamp.Format(time.RFC3339Nano),
			string(event.Data),
			string(metadata),
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit: %w", err)
	}

	newVersion := currentVersion + len(events)

	// Notify subscribers
	go s.notifySubscribers(events)

	return newVersion, nil
}

func (s *SQLiteStore) notifySubscribers(events []*runtime.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.subscriptions {
		for _, event := range events {
			if sub.matches(event) {
				select {
				case sub.events <- event:
				default:
					// Drop if buffer full
				}
			}
		}
	}
}

// Read retrieves events from a stream starting at fromVersion.
func (s *SQLiteStore) Read(ctx context.Context, streamID string, fromVersion int) ([]*runtime.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, stream_id, type, version, timestamp, data, metadata FROM events WHERE stream_id = ? AND version >= ? ORDER BY version",
		streamID, fromVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// ReadAll retrieves all events matching the filter.
func (s *SQLiteStore) ReadAll(ctx context.Context, filter runtime.EventFilter) ([]*runtime.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	query := "SELECT id, stream_id, type, version, timestamp, data, metadata FROM events WHERE 1=1"
	var args []any

	if filter.StreamID != "" {
		query += " AND stream_id = ?"
		args = append(args, filter.StreamID)
	}

	if len(filter.Types) > 0 {
		query += " AND type IN ("
		for i, t := range filter.Types {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, t)
		}
		query += ")"
	}

	if filter.FromVersion > 0 {
		query += " AND version >= ?"
		args = append(args, filter.FromVersion)
	}

	if filter.ToVersion > 0 {
		query += " AND version <= ?"
		args = append(args, filter.ToVersion)
	}

	if filter.FromTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.FromTime.Format(time.RFC3339Nano))
	}

	if filter.ToTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.ToTime.Format(time.RFC3339Nano))
	}

	query += " ORDER BY timestamp, version"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

func (s *SQLiteStore) scanEvents(rows *sql.Rows) ([]*runtime.Event, error) {
	var events []*runtime.Event

	for rows.Next() {
		var (
			event         runtime.Event
			timestampStr  string
			dataStr       string
			metadataStr   sql.NullString
		)

		err := rows.Scan(
			&event.ID,
			&event.StreamID,
			&event.Type,
			&event.Version,
			&timestampStr,
			&dataStr,
			&metadataStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.Timestamp, _ = time.Parse(time.RFC3339Nano, timestampStr)
		event.Data = json.RawMessage(dataStr)

		if metadataStr.Valid && metadataStr.String != "" {
			json.Unmarshal([]byte(metadataStr.String), &event.Metadata)
		}

		events = append(events, &event)
	}

	return events, rows.Err()
}

// StreamVersion returns the current version of a stream.
func (s *SQLiteStore) StreamVersion(ctx context.Context, streamID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, ErrStoreClosed
	}

	var version int
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), -1) FROM events WHERE stream_id = ?",
		streamID,
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get stream version: %w", err)
	}

	return version, nil
}

// Subscribe creates a subscription for new events.
func (s *SQLiteStore) Subscribe(ctx context.Context, filter runtime.EventFilter) (runtime.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	sub := &sqliteSubscription{
		id:     uuid.New().String(),
		filter: filter,
		events: make(chan *runtime.Event, 100),
		errors: make(chan error, 1),
		done:   make(chan struct{}),
		store:  s,
	}

	s.subscriptions[sub.id] = sub

	go func() {
		<-ctx.Done()
		sub.Close()
	}()

	return sub, nil
}

// Close releases resources.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	for _, sub := range s.subscriptions {
		close(sub.events)
		close(sub.errors)
	}
	s.subscriptions = nil

	return s.db.Close()
}

// SaveSnapshot stores a snapshot.
func (s *SQLiteStore) SaveSnapshot(ctx context.Context, snapshot *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO snapshots (stream_id, version, state, created_at) VALUES (?, ?, ?, ?)`,
		snapshot.StreamID,
		snapshot.Version,
		snapshot.State,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// LoadSnapshot retrieves the latest snapshot for a stream.
func (s *SQLiteStore) LoadSnapshot(ctx context.Context, streamID string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	var snapshot Snapshot
	err := s.db.QueryRowContext(ctx,
		"SELECT stream_id, version, state FROM snapshots WHERE stream_id = ?",
		streamID,
	).Scan(&snapshot.StreamID, &snapshot.Version, &snapshot.State)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// DeleteSnapshot removes snapshots for a stream.
func (s *SQLiteStore) DeleteSnapshot(ctx context.Context, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM snapshots WHERE stream_id = ?", streamID)
	return err
}

// sqliteSubscription implements runtime.Subscription.
type sqliteSubscription struct {
	id     string
	filter runtime.EventFilter
	events chan *runtime.Event
	errors chan error
	done   chan struct{}
	store  *SQLiteStore
	closed bool
	mu     sync.Mutex
}

func (s *sqliteSubscription) Events() <-chan *runtime.Event {
	return s.events
}

func (s *sqliteSubscription) Errors() <-chan error {
	return s.errors
}

func (s *sqliteSubscription) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	s.store.mu.Lock()
	delete(s.store.subscriptions, s.id)
	s.store.mu.Unlock()

	close(s.done)
	return nil
}

// ListInstances returns a paginated list of aggregate instances with optional filters.
func (s *SQLiteStore) ListInstances(ctx context.Context, place, from, to string, page, perPage int) ([]Instance, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, 0, ErrStoreClosed
	}

	// Count total matching instances
	countQuery := `
		SELECT COUNT(DISTINCT stream_id)
		FROM events
		WHERE 1=1
	`
	var countArgs []interface{}

	if from != "" {
		countQuery += " AND timestamp >= ?"
		countArgs = append(countArgs, from)
	}
	if to != "" {
		countQuery += " AND timestamp <= ?"
		countArgs = append(countArgs, to)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count instances: %w", err)
	}

	// Build query for instances with filters
	query := `
		SELECT 
			stream_id,
			MAX(version) as version,
			MAX(timestamp) as updated_at
		FROM events
		WHERE 1=1
	`
	var args []interface{}

	if from != "" {
		query += " AND timestamp >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND timestamp <= ?"
		args = append(args, to)
	}

	query += " GROUP BY stream_id ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, perPage, (page-1)*perPage)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query instances: %w", err)
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var inst Instance
		var updatedAtStr string

		if err := rows.Scan(&inst.ID, &inst.Version, &updatedAtStr); err != nil {
			return nil, 0, fmt.Errorf("failed to scan instance: %w", err)
		}

		inst.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAtStr)

		// Load current state by replaying events
		events, err := s.Read(ctx, inst.ID, 0)
		if err != nil {
			continue // Skip instances we can't read
		}

		// Build state from events (simplified - assumes token places)
		inst.State = make(map[string]int)
		for _, evt := range events {
			// Extract state from event data if available
			var eventData map[string]interface{}
			if err := json.Unmarshal(evt.Data, &eventData); err == nil {
				if state, ok := eventData["state"].(map[string]interface{}); ok {
					for k, v := range state {
						if val, ok := v.(float64); ok {
							inst.State[k] = int(val)
						}
					}
				}
			}
		}

		instances = append(instances, inst)
	}

	return instances, total, rows.Err()
}

// GetStats returns statistics about stored aggregates.
func (s *SQLiteStore) GetStats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	// Count total unique streams
	var totalInstances int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT stream_id) FROM events",
	).Scan(&totalInstances)
	if err != nil {
		return nil, fmt.Errorf("failed to count instances: %w", err)
	}

	stats := &Stats{
		TotalInstances: totalInstances,
		ByPlace:        make(map[string]int),
	}

	// Get distinct stream IDs and compute their states
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT stream_id FROM events")
	if err != nil {
		return nil, fmt.Errorf("failed to query streams: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var streamID string
		if err := rows.Scan(&streamID); err != nil {
			continue
		}

		// Load events for this stream
		events, err := s.Read(ctx, streamID, 0)
		if err != nil {
			continue
		}

		// Find places with tokens in current state
		for _, evt := range events {
			var eventData map[string]interface{}
			if err := json.Unmarshal(evt.Data, &eventData); err == nil {
				if state, ok := eventData["state"].(map[string]interface{}); ok {
					for place, val := range state {
						if v, ok := val.(float64); ok && v > 0 {
							stats.ByPlace[place]++
						}
					}
				}
			}
		}
	}

	return stats, rows.Err()
}

func (s *sqliteSubscription) matches(event *runtime.Event) bool {
	if s.filter.StreamID != "" && s.filter.StreamID != event.StreamID {
		return false
	}
	if len(s.filter.Types) > 0 {
		found := false
		for _, t := range s.filter.Types {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Ensure SQLiteStore implements Store.
var _ Store = (*SQLiteStore)(nil)
