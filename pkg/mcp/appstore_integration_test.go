package mcp

// End-to-end tests for the app-store wiring: petri_extend and petri_build
// gain optional 'id'/'prompt' persistence, and petri_history/petri_app_save/
// petri_app_get/petri_app_list surface it. The legacy no-id/no-prompt call
// shape of both tools must remain byte-for-byte unchanged and must not touch
// the store at all.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/petri-pilot/pkg/appstore"
)

// withTestAppStore opens an isolated, temp-file-backed store and installs it
// as the process-wide store for the duration of the test, so persistence
// tests never risk the real ~/.petri-pilot/appstore.db.
func withTestAppStore(t *testing.T) *appstore.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "appstore.db")
	s, err := appstore.Open(path)
	if err != nil {
		t.Fatalf("appstore.Open: %v", err)
	}
	restore := setAppStoreForTest(s)
	t.Cleanup(func() {
		restore()
		s.Close()
	})
	return s
}

func callTool(t *testing.T, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	var handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	switch name {
	case "petri_extend":
		handler = handleExtend
	case "petri_build":
		handler = handleBuild
	case "petri_history":
		handler = handleHistory
	case "petri_app_save":
		handler = handleAppSave
	case "petri_app_get":
		handler = handleAppGet
	case "petri_app_list":
		handler = handleAppList
	default:
		t.Fatalf("callTool: unknown tool %q", name)
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return result
}

func decodeJSON[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool returned error: %s", resultText(result))
	}
	var v T
	if err := json.Unmarshal([]byte(resultText(result)), &v); err != nil {
		t.Fatalf("decoding result: %v\n%s", err, resultText(result))
	}
	return v
}

type extendResult struct {
	Success  bool     `json:"success"`
	Applied  []string `json:"applied"`
	Errors   []string `json:"errors,omitempty"`
	Valid    bool     `json:"valid"`
	Model    string   `json:"model"`
	ID       string   `json:"id,omitempty"`
	ParentID string   `json:"parentId,omitempty"`
}

const extendBaseModel = `{
  "name": "gate",
  "places": [{"id": "open", "initial": 1}, {"id": "closed"}],
  "transitions": [{"id": "shut"}],
  "arcs": [{"from": "open", "to": "shut"}, {"from": "shut", "to": "closed"}]
}`

// TestExtend_LegacyShapeUntouched confirms the exact legacy call (no id, no
// prompt) still behaves as before: no store interaction, no 'id'/'parentId'
// in the result.
func TestExtend_LegacyShapeUntouched(t *testing.T) {
	store := withTestAppStore(t)

	res := callTool(t, "petri_extend", map[string]any{
		"model":      extendBaseModel,
		"operations": `[{"op":"add_place","id":"jammed"}]`,
	})
	out := decodeJSON[extendResult](t, res)

	if out.ID != "" || out.ParentID != "" {
		t.Errorf("legacy call must not report an id/parentId, got id=%q parentId=%q", out.ID, out.ParentID)
	}
	if !out.Success || out.Applied[0] != "add_place" {
		t.Errorf("unexpected result: %+v", out)
	}

	apps, err := store.ListApps()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 0 {
		t.Errorf("expected no apps recorded, got %+v", apps)
	}
	// No direct "row count" accessor on Store for specs/lineage/prompts, so
	// assert indirectly: petri_history on any hash derived from the legacy
	// call's output must be unknown, since nothing was ever Put.
	id, err := appstore.ContentID([]byte(out.Model))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(id); err == nil {
		t.Error("expected the legacy call's resulting model to be absent from the store")
	}
}

// TestBuild_LegacyShapeUntouched confirms petri_build's exact legacy call
// (no id, no prompt) leaves the store untouched and the report shape is
// additive-only (an empty 'id').
func TestBuild_LegacyShapeUntouched(t *testing.T) {
	store := withTestAppStore(t)

	res := callTool(t, "petri_build", map[string]any{
		"model":      coreModelJSON,
		"output_dir": t.TempDir(),
		"verify":     false,
	})
	report := decodeBuildReport(t, res)

	if report.ID != "" {
		t.Errorf("legacy call must not report an id, got %q", report.ID)
	}
	if !report.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", report.Build.Errors)
	}

	apps, err := store.ListApps()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 0 {
		t.Errorf("expected no apps recorded, got %+v", apps)
	}
	id, err := appstore.ContentID([]byte(coreModelJSON))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(id); err == nil {
		t.Error("expected the legacy build's model to be absent from the store")
	}
}

// TestIterativeRefinementRoundTrip is the test that proves the actual
// feature: two rounds of petri_extend with prompts and no explicit starting
// id (first) then an explicit id (second), petri_history showing both
// prompts and activities in order, then petri_build(id=...) building and
// verifying that same spec and petri_history showing the build too.
func TestIterativeRefinementRoundTrip(t *testing.T) {
	withTestAppStore(t)

	// Round 1: fresh spec, no id, with a prompt.
	res1 := callTool(t, "petri_extend", map[string]any{
		"model":      extendBaseModel,
		"operations": `[{"op":"add_place","id":"jammed"}]`,
		"prompt":     "add a jammed state for the failure path",
	})
	out1 := decodeJSON[extendResult](t, res1)
	if out1.ID == "" {
		t.Fatalf("expected an id back from the first extend call: %+v", out1)
	}
	if out1.ParentID == "" {
		t.Fatalf("expected a parentId back from the first extend call: %+v", out1)
	}

	// Round 2: continue from that id, with a second prompt and a different
	// operation.
	res2 := callTool(t, "petri_extend", map[string]any{
		"id":         out1.ID,
		"operations": `[{"op":"add_transition","id":"clear_jam"},{"op":"add_arc","from":"jammed","to":"clear_jam"},{"op":"add_arc","from":"clear_jam","to":"open"}]`,
		"prompt":     "add a transition to clear the jam and reopen",
	})
	out2 := decodeJSON[extendResult](t, res2)
	if out2.ID == "" {
		t.Fatalf("expected an id back from the second extend call: %+v", out2)
	}
	if out2.ParentID != out1.ID {
		t.Fatalf("expected the second spec's parent to be the first spec's id: parentId=%q want %q", out2.ParentID, out1.ID)
	}

	// petri_history on the second spec must show both prompts and both
	// activities, in order.
	histRes := callTool(t, "petri_history", map[string]any{"id": out2.ID})
	hist := decodeJSON[struct {
		ID      string                  `json:"id"`
		History []appstore.LineageEntry `json:"history"`
	}](t, histRes)

	if len(hist.History) != 2 {
		t.Fatalf("expected 2 lineage entries before any build, got %d: %+v", len(hist.History), hist.History)
	}
	if hist.History[0].ID != out1.ID || hist.History[0].Prompt == nil || *hist.History[0].Prompt != "add a jammed state for the failure path" {
		t.Errorf("entry 0 = %+v, want id=%s with the first prompt", hist.History[0], out1.ID)
	}
	if hist.History[0].Activity != "petri_extend" {
		t.Errorf("entry 0 activity = %q, want petri_extend", hist.History[0].Activity)
	}
	if hist.History[1].ID != out2.ID || hist.History[1].Prompt == nil || *hist.History[1].Prompt != "add a transition to clear the jam and reopen" {
		t.Errorf("entry 1 = %+v, want id=%s with the second prompt", hist.History[1], out2.ID)
	}

	// petri_build against the same spec id: it must build and verify, and
	// the lineage must now show a petri_build entry too.
	buildDir := t.TempDir()
	buildRes := callTool(t, "petri_build", map[string]any{
		"id":         out2.ID,
		"output_dir": buildDir,
		"verify":     true,
	})
	buildReport := decodeBuildReport(t, buildRes)
	if !buildReport.Build.OK {
		t.Fatalf("expected build to succeed:\n%s", buildReport.Build.Errors)
	}
	if !buildReport.Verify.Ran || !buildReport.Verify.Passed {
		t.Fatalf("expected verify to run and pass: %+v", buildReport.Verify)
	}
	if buildReport.ID != out2.ID {
		t.Errorf("expected petri_build to report the same spec id it was given, got %q want %q", buildReport.ID, out2.ID)
	}

	histRes2 := callTool(t, "petri_history", map[string]any{"id": out2.ID})
	hist2 := decodeJSON[struct {
		ID      string                  `json:"id"`
		History []appstore.LineageEntry `json:"history"`
	}](t, histRes2)
	if len(hist2.History) != 3 {
		t.Fatalf("expected 3 lineage entries after the build, got %d: %+v", len(hist2.History), hist2.History)
	}
	last := hist2.History[2]
	if last.ID != out2.ID || last.Activity != "petri_build" {
		t.Errorf("last entry = %+v, want id=%s activity=petri_build", last, out2.ID)
	}

	// petri_app_save / petri_app_get / petri_app_list round-trip.
	saveRes := callTool(t, "petri_app_save", map[string]any{"name": "gate-app", "id": out2.ID})
	if saveRes.IsError {
		t.Fatalf("petri_app_save failed: %s", resultText(saveRes))
	}

	getRes := callTool(t, "petri_app_get", map[string]any{"name": "gate-app"})
	got := decodeJSON[struct {
		Name    string          `json:"name"`
		ID      string          `json:"id"`
		Kind    string          `json:"kind"`
		Content json.RawMessage `json:"content"`
	}](t, getRes)
	if got.ID != out2.ID {
		t.Errorf("petri_app_get id = %q, want %q", got.ID, out2.ID)
	}
	if got.Kind != "model" {
		t.Errorf("petri_app_get kind = %q, want model", got.Kind)
	}

	listRes := callTool(t, "petri_app_list", nil)
	list := decodeJSON[struct {
		Apps []appstore.AppEntry `json:"apps"`
	}](t, listRes)
	if len(list.Apps) != 1 || list.Apps[0].Name != "gate-app" || list.Apps[0].HeadSpecID != out2.ID {
		t.Errorf("petri_app_list = %+v, want one entry gate-app -> %s", list.Apps, out2.ID)
	}
}

// TestHistory_UnknownID confirms a clear error rather than an empty result.
func TestHistory_UnknownID(t *testing.T) {
	withTestAppStore(t)
	res := callTool(t, "petri_history", map[string]any{"id": "deadbeef"})
	if !res.IsError {
		t.Fatal("expected an error for an unknown spec id")
	}
}

// TestAppGet_UnknownName confirms a clear error rather than an empty result.
func TestAppGet_UnknownName(t *testing.T) {
	withTestAppStore(t)
	res := callTool(t, "petri_app_get", map[string]any{"name": "nope"})
	if !res.IsError {
		t.Fatal("expected an error for an unknown app name")
	}
}

// TestExtend_IDWinsOverModel confirms that when both 'id' and 'model' are
// given, 'id' wins and 'model' is ignored, per the documented precedence.
func TestExtend_IDWinsOverModel(t *testing.T) {
	withTestAppStore(t)

	res1 := callTool(t, "petri_extend", map[string]any{
		"model":      extendBaseModel,
		"operations": `[]`,
		"prompt":     "seed",
	})
	out1 := decodeJSON[extendResult](t, res1)

	// Pass a completely different, invalid-looking model string alongside a
	// valid id: if 'id' didn't win, this would fail to parse.
	res2 := callTool(t, "petri_extend", map[string]any{
		"id":         out1.ID,
		"model":      "not valid json at all",
		"operations": `[{"op":"add_place","id":"extra"}]`,
	})
	out2 := decodeJSON[extendResult](t, res2)
	if !out2.Success {
		t.Fatalf("expected success (id should have won over the bogus model), got %+v", out2)
	}
}

// TestExtend_NoOpEditThenHistory pins a reachable edge case. Ids are hashes
// of content, not edit counters, so any petri_extend call whose result is
// byte-identical (after the model parse/remarshal round trip) to its
// starting spec collapses to that same id. The *first* operations=[] edit on
// a hand-written model doesn't actually trigger this — the round trip fills
// in implicit zero-value defaults (e.g. an omitted "initial" becomes an
// explicit "initial":0), so the first pass still changes content. A
// *second* no-op edit against that already-normalized spec does trigger it,
// since the round trip is idempotent once nothing is left to normalize.
// Before appstore.Store.RecordLineage normalized a self-referencing parent
// to nil, this recorded a literal spec_id == parent_spec_id lineage row, and
// petri_history on that id then failed with "cycle detected in lineage" on
// its very first parent hop.
func TestExtend_NoOpEditThenHistory(t *testing.T) {
	withTestAppStore(t)

	res1 := callTool(t, "petri_extend", map[string]any{
		"model":      extendBaseModel,
		"operations": `[]`,
		"prompt":     "normalize",
	})
	out1 := decodeJSON[extendResult](t, res1)
	if out1.ID == "" {
		t.Fatalf("expected an id back: %+v", out1)
	}

	res2 := callTool(t, "petri_extend", map[string]any{
		"id":         out1.ID,
		"operations": `[]`,
		"prompt":     "no changes, just recording intent",
	})
	out2 := decodeJSON[extendResult](t, res2)
	if out2.ID != out1.ID {
		t.Fatalf("expected a second no-op edit against an already-normalized spec to collapse to its own starting id: id=%q parentId=%q startId=%q", out2.ID, out2.ParentID, out1.ID)
	}

	histRes := callTool(t, "petri_history", map[string]any{"id": out2.ID})
	if histRes.IsError {
		t.Fatalf("petri_history on a no-op-edit id must not error: %+v", histRes)
	}
	hist := decodeJSON[struct {
		ID      string                  `json:"id"`
		History []appstore.LineageEntry `json:"history"`
	}](t, histRes)
	if len(hist.History) != 2 {
		t.Fatalf("expected exactly 2 lineage entries (the normalizing edit, then the no-op edit), got %d: %+v", len(hist.History), hist.History)
	}
	last := hist.History[len(hist.History)-1]
	if last.ParentID != nil {
		t.Errorf("last entry.ParentID = %v, want nil (self-edge normalized to root)", *last.ParentID)
	}
	if last.Prompt == nil || *last.Prompt != "no changes, just recording intent" {
		t.Errorf("last entry.Prompt = %v, want the recorded prompt", last.Prompt)
	}
}
