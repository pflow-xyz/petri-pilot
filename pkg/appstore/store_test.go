package appstore

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	// A temp-file DB rather than ":memory:" for most tests: it exercises the
	// real Open() path (directory creation, schema) exactly like production,
	// while staying entirely inside t.TempDir() so it can never touch the
	// real ~/.petri-pilot/appstore.db.
	path := filepath.Join(t.TempDir(), "appstore.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCanonicalizeSortsKeys(t *testing.T) {
	a, err := canonicalize([]byte(`{"b":1,"a":2}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalize([]byte(`{"a":2,"b":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("canonicalization did not normalize key order: %q vs %q", a, b)
	}
	if string(a) != `{"a":2,"b":1}` {
		t.Fatalf("unexpected canonical form: %q", a)
	}
}

func TestPutIsIdempotent(t *testing.T) {
	s := openTestStore(t)

	id1, err := s.Put("model", []byte(`{"name":"gate","b":1,"a":2}`))
	if err != nil {
		t.Fatal(err)
	}
	// Same content, different key order and whitespace -> same id.
	id2, err := s.Put("model", []byte(`{"a": 2, "b": 1, "name": "gate"}`))
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("expected identical content to collapse to one id, got %q and %q", id1, id2)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM specs WHERE id = ?`, id1).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one row for id %q, got %d", id1, count)
	}
}

func TestGetRoundTrips(t *testing.T) {
	s := openTestStore(t)

	content := []byte(`{"name":"gate","places":[{"id":"open"}]}`)
	id, err := s.Put("model", content)
	if err != nil {
		t.Fatal(err)
	}

	kind, got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "model" {
		t.Errorf("kind = %q, want model", kind)
	}
	// Content round-trips byte-for-byte relative to its canonical form
	// (Get returns exactly what was stored, and Put stores the canonical
	// encoding, so re-canonicalizing the original input must match).
	wantCanon, err := canonicalize(content)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(wantCanon) {
		t.Errorf("content = %q, want %q", got, wantCanon)
	}
}

func TestGetUnknownID(t *testing.T) {
	s := openTestStore(t)
	if _, _, err := s.Get("deadbeef"); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}

// TestHistoryThreeStepChain covers root spec -> extend #1 (with a prompt) ->
// extend #2 (with a prompt), and checks History round-trips the chain in
// order with the right prompts attached, including that the root's own
// lineage entry carries a nil parent.
func TestHistoryThreeStepChain(t *testing.T) {
	s := openTestStore(t)

	rootID, err := s.Put("model", []byte(`{"name":"gate","v":0}`))
	if err != nil {
		t.Fatal(err)
	}
	// Root spec gets its own lineage row with a nil parent, established
	// directly at the Store level (the MCP wiring never needs to do this —
	// petri_extend only records an edge for the *derived* spec — but the
	// walk must still terminate correctly when a root row does exist).
	if err := s.RecordLineage(rootID, nil, "petri_build", nil, "initial spec"); err != nil {
		t.Fatal(err)
	}

	spec1ID, err := s.Put("model", []byte(`{"name":"gate","v":1}`))
	if err != nil {
		t.Fatal(err)
	}
	prompt1ID, err := s.RecordPrompt("add a place", rootID, spec1ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordLineage(spec1ID, &rootID, "petri_extend", &prompt1ID, "applied 1 operation"); err != nil {
		t.Fatal(err)
	}

	spec2ID, err := s.Put("model", []byte(`{"name":"gate","v":2}`))
	if err != nil {
		t.Fatal(err)
	}
	prompt2ID, err := s.RecordPrompt("add a transition", spec1ID, spec2ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordLineage(spec2ID, &spec1ID, "petri_extend", &prompt2ID, "applied 1 operation"); err != nil {
		t.Fatal(err)
	}

	history, err := s.History(spec2ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 lineage entries, got %d: %+v", len(history), history)
	}

	if history[0].ID != rootID {
		t.Errorf("entry 0 id = %q, want root %q", history[0].ID, rootID)
	}
	if history[0].ParentID != nil {
		t.Errorf("root entry should have a nil parent, got %v", *history[0].ParentID)
	}

	if history[1].ID != spec1ID || history[1].ParentID == nil || *history[1].ParentID != rootID {
		t.Errorf("entry 1 = %+v, want id=%s parent=%s", history[1], spec1ID, rootID)
	}
	if history[1].Prompt == nil || *history[1].Prompt != "add a place" {
		t.Errorf("entry 1 prompt = %v, want %q", history[1].Prompt, "add a place")
	}
	if history[1].Activity != "petri_extend" {
		t.Errorf("entry 1 activity = %q, want petri_extend", history[1].Activity)
	}

	if history[2].ID != spec2ID || history[2].ParentID == nil || *history[2].ParentID != spec1ID {
		t.Errorf("entry 2 = %+v, want id=%s parent=%s", history[2], spec2ID, spec1ID)
	}
	if history[2].Prompt == nil || *history[2].Prompt != "add a transition" {
		t.Errorf("entry 2 prompt = %v, want %q", history[2].Prompt, "add a transition")
	}
}

// TestHistoryNoParentIsNil covers a spec with no recorded parent at all
// (zero lineage rows): History must return an empty chain rather than
// erroring, since the spec itself is known.
func TestHistoryNoParentIsNil(t *testing.T) {
	s := openTestStore(t)

	id, err := s.Put("model", []byte(`{"name":"solo"}`))
	if err != nil {
		t.Fatal(err)
	}
	history, err := s.History(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("expected no lineage entries for a spec nobody has extended or built, got %+v", history)
	}
}

func TestHistoryUnknownID(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.History("deadbeef"); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}

// TestHistoryIncludesBuildAnnotation covers a spec that is both the target
// of a derivation edge (from petri_extend) and a later annotation edge (from
// petri_build) that does not derive a new spec — both rows must appear, at
// that spec's position, and the walk must still find the true parent.
func TestHistoryIncludesBuildAnnotation(t *testing.T) {
	s := openTestStore(t)

	rootID, err := s.Put("model", []byte(`{"name":"gate","v":0}`))
	if err != nil {
		t.Fatal(err)
	}
	spec1ID, err := s.Put("model", []byte(`{"name":"gate","v":1}`))
	if err != nil {
		t.Fatal(err)
	}
	promptID, err := s.RecordPrompt("add a place", rootID, spec1ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordLineage(spec1ID, &rootID, "petri_extend", &promptID, "applied 1 operation"); err != nil {
		t.Fatal(err)
	}
	// petri_build annotates spec1 without deriving a new spec: parent nil.
	if err := s.RecordLineage(spec1ID, nil, "petri_build", nil, "build ok, verify passed"); err != nil {
		t.Fatal(err)
	}

	history, err := s.History(spec1ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 entries (extend + build) for spec1, got %d: %+v", len(history), history)
	}
	if history[0].Activity != "petri_extend" || history[0].ParentID == nil || *history[0].ParentID != rootID {
		t.Errorf("entry 0 = %+v, want petri_extend with parent %s", history[0], rootID)
	}
	if history[1].Activity != "petri_build" || history[1].ParentID != nil {
		t.Errorf("entry 1 = %+v, want petri_build with nil parent", history[1])
	}
}

func TestSaveAndGetApp(t *testing.T) {
	s := openTestStore(t)

	id, err := s.Put("model", []byte(`{"name":"gate"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApp("my-app", id); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetApp("my-app")
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Errorf("GetApp = %q, want %q", got, id)
	}

	// Update to a new spec.
	id2, err := s.Put("model", []byte(`{"name":"gate","v":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveApp("my-app", id2); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetApp("my-app")
	if err != nil {
		t.Fatal(err)
	}
	if got != id2 {
		t.Errorf("GetApp after update = %q, want %q", got, id2)
	}

	apps, err := s.ListApps()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].Name != "my-app" || apps[0].HeadSpecID != id2 {
		t.Errorf("ListApps = %+v, want one entry my-app -> %s", apps, id2)
	}
}

func TestSaveAppRequiresKnownSpec(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveApp("ghost", "deadbeef"); err == nil {
		t.Fatal("expected an error saving an app against an unknown spec id")
	}
}

func TestGetAppUnknownName(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.GetApp("nope"); err == nil {
		t.Fatal("expected an error for an unknown app name")
	}
}

// TestRecordLineageSelfEdgeBecomesRoot pins a reachable, not hypothetical,
// case: content-addressing means an edit that changes nothing produces the
// same id as its own starting spec. A literal spec_id == parent_spec_id edge
// would make History revisit specID on its very first parent hop and report
// a false cycle on every later lookup. RecordLineage must store that edge as
// parentless instead, and History must then treat specID as a root.
func TestRecordLineageSelfEdgeBecomesRoot(t *testing.T) {
	s := openTestStore(t)

	id, err := s.Put("model", []byte(`{"name":"unchanged"}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RecordLineage(id, &id, "petri_extend", nil, "no-op edit"); err != nil {
		t.Fatal(err)
	}

	hist, err := s.History(id)
	if err != nil {
		t.Fatalf("History after a self-edge lineage row: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("History = %d entries, want 1: %+v", len(hist), hist)
	}
	if hist[0].ParentID != nil {
		t.Errorf("entry.ParentID = %v, want nil (self-edge normalized to root)", *hist[0].ParentID)
	}
	if hist[0].ID != id {
		t.Errorf("entry.ID = %q, want %q", hist[0].ID, id)
	}
}

// TestRecordLineageSelfEdgeKeepsRealParent confirms the self-edge guard does
// not hide a genuine ancestor: if specID already has a real (different)
// parent recorded from an earlier derivation, a later self-edge row against
// that same specID must not stop History's parent walk from reaching it.
// (root itself carries no lineage row here — root's own history, if any,
// lives on whatever produced root; History only ever reports recorded rows,
// never a synthetic entry for an unrecorded spec, exactly as production
// handleExtend records a row only for what it derives, never for the
// untouched starting spec — see TestIterativeRefinementRoundTrip.)
func TestRecordLineageSelfEdgeKeepsRealParent(t *testing.T) {
	s := openTestStore(t)

	root, err := s.Put("model", []byte(`{"name":"root"}`))
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.Put("model", []byte(`{"name":"child"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordLineage(child, &root, "petri_extend", nil, "real edit"); err != nil {
		t.Fatal(err)
	}
	// A later no-op edit against the same child collapses to child's own id.
	if err := s.RecordLineage(child, &child, "petri_extend", nil, "no-op edit"); err != nil {
		t.Fatal(err)
	}

	hist, err := s.History(child)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("History = %d entries, want 2 (child's real-edit row, then its no-op row): %+v", len(hist), hist)
	}
	if hist[0].ID != child || hist[0].ParentID == nil || *hist[0].ParentID != root || hist[0].Note != "real edit" {
		t.Errorf("entry 0 = %+v, want child %q with parent %q and note %q", hist[0], child, root, "real edit")
	}
	if hist[1].ID != child || hist[1].ParentID != nil || hist[1].Note != "no-op edit" {
		t.Errorf("entry 1 = %+v, want child %q with no parent and note %q", hist[1], child, "no-op edit")
	}
}
