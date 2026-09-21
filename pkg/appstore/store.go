// Package appstore is a persistent, content-addressed store for petri-pilot
// app specs (Petri net models, Application specs, and Bundle documents),
// with lineage tracking of the prompts and edits that produced each one.
//
// The unit of identity is a "spec": the canonical JSON encoding of a model,
// Application spec, or Bundle document, addressed by the sha256 hash of that
// canonical form. Two calls that produce byte-identical content (after
// canonicalization) collapse to the same id — nothing mutates, content is
// stored once.
//
// Four kinds of row, deliberately kept separate rather than folded into one
// table, because the distinction is what makes lineage queryable:
//
//   - specs   — the content itself, addressed by id.
//   - prompts — free-text instructions a caller attached to an edit.
//   - lineage — the edge from a spec to the (single) parent it was derived
//     from, naming the tool activity and optionally a prompt.
//   - apps    — a human-given name pointing at the current/latest spec id.
package appstore

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a SQLite-backed content-addressed store for app specs and their
// lineage.
type Store struct {
	db *sql.DB
}

// DefaultPath returns the default location of the appstore database,
// mirroring pkg/mcp/service.go's ~/.petri-pilot convention. Overridable via
// the PETRI_APPSTORE_DB environment variable, which tests must use (or an
// in-memory / temp-file path passed directly to Open) — never the real path.
func DefaultPath() string {
	if p := os.Getenv("PETRI_APPSTORE_DB"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".petri-pilot", "appstore.db")
}

// Open creates or opens a SQLite-backed Store at path. Pass ":memory:" for a
// throwaway in-process database (tests).
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("appstore: creating %s: %w", dir, err)
			}
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("appstore: opening %s: %w", path, err)
	}
	// SQLite handles one writer at a time; a busy connection pool just means
	// concurrent goroutines see SQLITE_BUSY instead of queuing.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("appstore: creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS specs (
	id         TEXT PRIMARY KEY,
	kind       TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS prompts (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	text           TEXT NOT NULL,
	parent_spec_id TEXT,
	result_spec_id TEXT NOT NULL,
	created_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS lineage (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	spec_id        TEXT NOT NULL,
	parent_spec_id TEXT,
	activity       TEXT NOT NULL,
	prompt_id      INTEGER,
	note           TEXT NOT NULL DEFAULT '',
	created_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_lineage_spec_id ON lineage(spec_id);

CREATE TABLE IF NOT EXISTS apps (
	name          TEXT PRIMARY KEY,
	head_spec_id  TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);
`

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// canonicalize re-encodes raw JSON with map keys sorted and no insignificant
// whitespace, so that two byte-different-but-semantically-identical
// documents hash to the same id. encoding/json already sorts map[string]any
// keys when marshaling (guaranteed by the stdlib, not merely observed), so
// round-tripping through interface{} is sufficient — this is explicit here,
// not assumed, and pinned by TestCanonicalizeSortsKeys.
func canonicalize(raw []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("appstore: content is not valid JSON: %w", err)
	}
	canon, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return canon, nil
}

func hashOf(canon []byte) string {
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:])
}

// ContentID returns the id a given piece of spec content would be stored
// under, without storing it. Useful for resolving "is this content already
// known" before deciding whether to treat it as a fresh root.
func ContentID(content []byte) (string, error) {
	canon, err := canonicalize(content)
	if err != nil {
		return "", err
	}
	return hashOf(canon), nil
}

// Put stores content under its content-addressed id and returns that id.
// Idempotent: storing the same content twice (even with a different `kind`
// tag — the first write wins) returns the same id without error and without
// a duplicate row.
func (s *Store) Put(kind string, content []byte) (string, error) {
	canon, err := canonicalize(content)
	if err != nil {
		return "", err
	}
	id := hashOf(canon)

	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO specs (id, kind, content, created_at) VALUES (?, ?, ?, ?)`,
		id, kind, string(canon), nowUTC(),
	)
	if err != nil {
		return "", fmt.Errorf("appstore: storing spec: %w", err)
	}
	return id, nil
}

// ErrNotFound is returned when a spec id or app name is unknown.
var ErrNotFound = errors.New("appstore: not found")

// Get returns the kind and content of the spec stored under id.
func (s *Store) Get(id string) (kind string, content []byte, err error) {
	var c string
	err = s.db.QueryRow(`SELECT kind, content FROM specs WHERE id = ?`, id).Scan(&kind, &c)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, fmt.Errorf("appstore: spec %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return "", nil, err
	}
	return kind, []byte(c), nil
}

// Exists reports whether a spec id is known to the store.
func (s *Store) Exists(id string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM specs WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RecordPrompt stores the free-text instruction behind an edit, along with
// the spec it started from (if any) and the spec it produced, and returns a
// prompt id suitable for RecordLineage.
func (s *Store) RecordPrompt(text string, parentSpecID, resultSpecID string) (string, error) {
	var parent sql.NullString
	if parentSpecID != "" {
		parent = sql.NullString{String: parentSpecID, Valid: true}
	}
	res, err := s.db.Exec(
		`INSERT INTO prompts (text, parent_spec_id, result_spec_id, created_at) VALUES (?, ?, ?, ?)`,
		text, parent, resultSpecID, nowUTC(),
	)
	if err != nil {
		return "", fmt.Errorf("appstore: recording prompt: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

// RecordLineage records the edge from specID to its (single) parent —
// parentSpecID may be nil for a root spec or for an activity (like
// petri_build) that annotates an existing spec rather than deriving a new
// one. promptID may be nil when no prompt was given.
//
// A parentSpecID equal to specID itself is stored as nil rather than as a
// self-edge. This is reachable, not hypothetical: ids are content-addressed,
// so an edit whose operations produce a byte-identical result (an empty
// operations list, or one that nets out to no change) collapses to the same
// id as its starting spec. A literal self-edge would make History's parent
// walk revisit specID immediately and report a cycle on every subsequent
// lookup. Treating it as parentless is correct, not just safe: if specID
// already has a real ancestor from an earlier, actually-different derivation,
// History still finds that ancestor from that earlier row at the same
// position; if it doesn't, specID genuinely behaves as a root here, which is
// what "this edit changed nothing" means.
func (s *Store) RecordLineage(specID string, parentSpecID *string, activity string, promptID *string, note string) error {
	var parent sql.NullString
	if parentSpecID != nil && *parentSpecID != "" && *parentSpecID != specID {
		parent = sql.NullString{String: *parentSpecID, Valid: true}
	}
	var prompt sql.NullInt64
	if promptID != nil && *promptID != "" {
		var n int64
		if _, err := fmt.Sscanf(*promptID, "%d", &n); err == nil {
			prompt = sql.NullInt64{Int64: n, Valid: true}
		}
	}
	_, err := s.db.Exec(
		`INSERT INTO lineage (spec_id, parent_spec_id, activity, prompt_id, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		specID, parent, activity, prompt, note, nowUTC(),
	)
	if err != nil {
		return fmt.Errorf("appstore: recording lineage: %w", err)
	}
	return nil
}

// LineageEntry is one step in a spec's history, root first.
type LineageEntry struct {
	ID        string  `json:"id"`
	ParentID  *string `json:"parentId,omitempty"`
	Activity  string  `json:"activity"`
	Prompt    *string `json:"prompt,omitempty"`
	Note      string  `json:"note,omitempty"`
	CreatedAt string  `json:"createdAt"`
}

type lineageRow struct {
	specID    string
	parentID  sql.NullString
	activity  string
	promptID  sql.NullInt64
	note      string
	createdAt string
}

// History walks the parent chain from specID back to its root (a spec with
// no recorded parent) and returns the chain root-first. A spec may carry
// more than one lineage row — e.g. the petri_extend edge that derived it, and
// a later petri_build annotation that references it without deriving a new
// spec — all such rows are included, in creation order, at that spec's
// position in the chain. specID itself must be a known spec; it is fine for
// it to carry zero lineage rows (a spec nobody has extended or built yet),
// in which case History returns an empty slice.
func (s *Store) History(specID string) ([]LineageEntry, error) {
	if ok, err := s.Exists(specID); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("appstore: spec %q: %w", specID, ErrNotFound)
	}

	var chain [][]lineageRow
	cur := specID
	visited := map[string]bool{}
	for cur != "" {
		if visited[cur] {
			// A cycle should be structurally impossible (parent_spec_id
			// always points at content stored strictly before it), but
			// refuse to loop forever if the data is ever corrupted by hand.
			return nil, fmt.Errorf("appstore: cycle detected in lineage at spec %q", cur)
		}
		visited[cur] = true

		rows, err := s.lineageRowsFor(cur)
		if err != nil {
			return nil, err
		}
		chain = append(chain, rows)

		next := ""
		for _, r := range rows {
			if r.parentID.Valid && r.parentID.String != "" {
				next = r.parentID.String
				break
			}
		}
		cur = next
	}

	// chain is head-first (specID's own rows first); reverse to root-first.
	var out []LineageEntry
	for i := len(chain) - 1; i >= 0; i-- {
		for _, r := range chain[i] {
			entry := LineageEntry{
				ID:        r.specID,
				Activity:  r.activity,
				Note:      r.note,
				CreatedAt: r.createdAt,
			}
			if r.parentID.Valid && r.parentID.String != "" {
				p := r.parentID.String
				entry.ParentID = &p
			}
			if r.promptID.Valid {
				text, err := s.promptText(r.promptID.Int64)
				if err == nil {
					entry.Prompt = &text
				}
			}
			out = append(out, entry)
		}
	}
	return out, nil
}

func (s *Store) lineageRowsFor(specID string) ([]lineageRow, error) {
	rows, err := s.db.Query(
		`SELECT spec_id, parent_spec_id, activity, prompt_id, note, created_at
		 FROM lineage WHERE spec_id = ? ORDER BY id ASC`,
		specID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []lineageRow
	for rows.Next() {
		var r lineageRow
		if err := rows.Scan(&r.specID, &r.parentID, &r.activity, &r.promptID, &r.note, &r.createdAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) promptText(id int64) (string, error) {
	var text string
	err := s.db.QueryRow(`SELECT text FROM prompts WHERE id = ?`, id).Scan(&text)
	return text, err
}

// AppEntry is a named app and the spec its name currently points at.
type AppEntry struct {
	Name       string `json:"name"`
	HeadSpecID string `json:"headSpecId"`
	UpdatedAt  string `json:"updatedAt"`
}

// SaveApp records (or updates) the name -> spec id mapping. specID must
// already exist in the store.
func (s *Store) SaveApp(name, specID string) error {
	if ok, err := s.Exists(specID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("appstore: spec %q: %w", specID, ErrNotFound)
	}
	_, err := s.db.Exec(
		`INSERT INTO apps (name, head_spec_id, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET head_spec_id = excluded.head_spec_id, updated_at = excluded.updated_at`,
		name, specID, nowUTC(),
	)
	if err != nil {
		return fmt.Errorf("appstore: saving app %q: %w", name, err)
	}
	return nil
}

// GetApp returns the spec id an app name currently points at.
func (s *Store) GetApp(name string) (specID string, err error) {
	err = s.db.QueryRow(`SELECT head_spec_id FROM apps WHERE name = ?`, name).Scan(&specID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("appstore: app %q: %w", name, ErrNotFound)
	}
	return specID, err
}

// ListApps returns every named app and the spec its name currently points
// at, ordered by name.
func (s *Store) ListApps() ([]AppEntry, error) {
	rows, err := s.db.Query(`SELECT name, head_spec_id, updated_at FROM apps ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AppEntry
	for rows.Next() {
		var a AppEntry
		if err := rows.Scan(&a.Name, &a.HeadSpecID, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
