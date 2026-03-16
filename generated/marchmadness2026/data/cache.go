package data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Cache provides SQLite-backed caching for fetched team data.
type Cache struct {
	db *sql.DB
}

// NewCache creates or opens a SQLite cache database.
func NewCache(path string) (*Cache, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS team_data (
			season INTEGER,
			name TEXT,
			data TEXT,
			fetched_at TEXT,
			PRIMARY KEY (season, name)
		);
		CREATE TABLE IF NOT EXISTS fetch_log (
			season INTEGER,
			source TEXT,
			fetched_at TEXT,
			team_count INTEGER,
			PRIMARY KEY (season, source)
		);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Cache{db: db}, nil
}

// Close closes the cache database.
func (c *Cache) Close() error {
	return c.db.Close()
}

// SaveTeams stores raw team data for a season.
func (c *Cache) SaveTeams(season int, teams []RawTeamData) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO team_data (season, name, data, fetched_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range teams {
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", t.Name, err)
		}
		if _, err := stmt.Exec(season, t.Name, string(data), now); err != nil {
			return err
		}
	}

	// Log the fetch
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO fetch_log (season, source, fetched_at, team_count)
		VALUES (?, ?, ?, ?)
	`, season, "all", now, len(teams))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// LoadTeams loads cached team data for a season.
// Returns nil if cache is stale (>24 hours old) or missing.
func (c *Cache) LoadTeams(season int) ([]RawTeamData, error) {
	// Check freshness
	var fetchedAt string
	err := c.db.QueryRow(`
		SELECT fetched_at FROM fetch_log WHERE season = ? AND source = 'all'
	`, season).Scan(&fetchedAt)
	if err != nil {
		return nil, fmt.Errorf("no cached data for season %d", season)
	}

	t, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return nil, err
	}
	if time.Since(t) > 24*time.Hour {
		return nil, fmt.Errorf("cache stale (fetched %s)", fetchedAt)
	}

	rows, err := c.db.Query(`
		SELECT data FROM team_data WHERE season = ?
	`, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []RawTeamData
	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			return nil, err
		}
		var t RawTeamData
		if err := json.Unmarshal([]byte(dataStr), &t); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}

	return teams, rows.Err()
}

// CacheStaleness returns how long ago the cache was last updated.
func (c *Cache) CacheStaleness(season int) (time.Duration, error) {
	var fetchedAt string
	err := c.db.QueryRow(`
		SELECT fetched_at FROM fetch_log WHERE season = ? AND source = 'all'
	`, season).Scan(&fetchedAt)
	if err != nil {
		return 0, err
	}

	t, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return 0, err
	}

	return time.Since(t), nil
}
