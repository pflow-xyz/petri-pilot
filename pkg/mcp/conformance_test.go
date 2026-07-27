package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// orderModelJSON is a simple linear workflow: receive → validate → ship.
const orderModelJSON = `{
  "name":"order",
  "places":[
    {"id":"received","initial":1},
    {"id":"validated"},
    {"id":"shipped"}],
  "transitions":[{"id":"validate"},{"id":"ship"}],
  "arcs":[
    {"from":"received","to":"validate"},{"from":"validate","to":"validated"},
    {"from":"validated","to":"ship"},{"from":"ship","to":"shipped"}]
}`

type conformanceOutput struct {
	Fitness         float64 `json:"fitness"`
	Precision       float64 `json:"precision"`
	FScore          float64 `json:"f_score"`
	Cases           int     `json:"cases"`
	FittingTraces   int     `json:"fitting_traces"`
	AvgTraceFitness float64 `json:"avg_trace_fitness"`
	Traces          []struct {
		CaseID  string   `json:"case_id"`
		Fitness float64  `json:"fitness"`
		Fits    bool     `json:"fits"`
		Missing []string `json:"missing_activities"`
	} `json:"traces"`
	Warnings       []string `json:"warnings"`
	Interpretation string   `json:"interpretation"`
}

func callConformance(t *testing.T, model, log string) *conformanceOutput {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_conformance"
	req.Params.Arguments = map[string]any{"model": model, "log": log}

	result, err := handleConformance(context.Background(), req)
	if err != nil {
		t.Fatalf("handleConformance error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned an error: %s", resultText(result))
	}

	var out conformanceOutput
	if err := json.Unmarshal([]byte(resultText(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, resultText(result))
	}
	return &out
}

func TestConformanceFittingLog(t *testing.T) {
	log := `[
      {"case":"o1","activity":"validate","timestamp":"2026-01-01T10:00:00Z"},
      {"case":"o1","activity":"ship","timestamp":"2026-01-01T11:00:00Z"}
    ]`

	out := callConformance(t, orderModelJSON, log)

	if out.Cases != 1 {
		t.Errorf("cases = %d, want 1", out.Cases)
	}
	if out.Fitness < 0.99 {
		t.Errorf("fitness = %.3f, want ~1.0 for a log the model reproduces exactly", out.Fitness)
	}
	if out.FittingTraces != 1 {
		t.Errorf("fitting_traces = %d, want 1", out.FittingTraces)
	}
}

// TestConformanceUnknownActivityWarns is the diagnostic that saves an agent from
// chasing a low fitness score that is really a name mismatch.
func TestConformanceUnknownActivityWarns(t *testing.T) {
	log := `[
      {"case":"o1","activity":"validate"},
      {"case":"o1","activity":"dispatch"}
    ]`

	out := callConformance(t, orderModelJSON, log)

	found := false
	for _, w := range out.Warnings {
		if strings.Contains(w, "dispatch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning naming the unmatched activity, got %v", out.Warnings)
	}
}

func TestConformanceNonFittingLog(t *testing.T) {
	// "ship" before "validate" is not a path the model allows.
	log := `[
      {"case":"o1","activity":"ship"},
      {"case":"o1","activity":"validate"}
    ]`

	out := callConformance(t, orderModelJSON, log)

	if out.Fitness >= 1.0 {
		t.Errorf("fitness = %.3f, want < 1.0 for an out-of-order trace", out.Fitness)
	}
	if len(out.Traces) == 0 {
		t.Fatal("expected per-trace detail")
	}
	if out.Traces[0].Fits {
		t.Error("out-of-order trace should not be marked as fitting")
	}
}

// TestConformanceTracesSortedWorstFirst checks the ordering choice: the traces
// most worth reading come first.
func TestConformanceTracesSortedWorstFirst(t *testing.T) {
	log := `[
      {"case":"good","activity":"validate"},
      {"case":"good","activity":"ship"},
      {"case":"bad","activity":"ship"},
      {"case":"bad","activity":"validate"}
    ]`

	out := callConformance(t, orderModelJSON, log)

	if len(out.Traces) != 2 {
		t.Fatalf("got %d traces, want 2", len(out.Traces))
	}
	if out.Traces[0].Fitness > out.Traces[1].Fitness {
		t.Errorf("traces not sorted worst-first: %.3f then %.3f",
			out.Traces[0].Fitness, out.Traces[1].Fitness)
	}
}

func TestConformanceNoTimestampsWarns(t *testing.T) {
	log := `[{"case":"o1","activity":"validate"},{"case":"o1","activity":"ship"}]`

	out := callConformance(t, orderModelJSON, log)

	found := false
	for _, w := range out.Warnings {
		if strings.Contains(w, "timestamp") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning about missing timestamps, got %v", out.Warnings)
	}
	// Order given must still be respected, so this log fits.
	if out.Fitness < 0.99 {
		t.Errorf("fitness = %.3f — events without timestamps should keep input order", out.Fitness)
	}
}

func TestConformanceKeyAliases(t *testing.T) {
	// caseId + action instead of case + activity.
	log := `[{"caseId":"o1","action":"validate"},{"caseId":"o1","action":"ship"}]`

	out := callConformance(t, orderModelJSON, log)

	if out.Cases != 1 {
		t.Errorf("cases = %d, want 1 — key aliases should be accepted", out.Cases)
	}
	if out.Fitness < 0.99 {
		t.Errorf("fitness = %.3f, want ~1.0", out.Fitness)
	}
}

func TestConformanceSkipsIncompleteEvents(t *testing.T) {
	log := `[
      {"case":"o1","activity":"validate"},
      {"activity":"orphan"},
      {"case":"o1","activity":"ship"}
    ]`

	out := callConformance(t, orderModelJSON, log)

	found := false
	for _, w := range out.Warnings {
		if strings.Contains(w, "skipped") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning about skipped events, got %v", out.Warnings)
	}
}

func TestConformanceInterpretationPresent(t *testing.T) {
	log := `[{"case":"o1","activity":"validate"},{"case":"o1","activity":"ship"}]`
	out := callConformance(t, orderModelJSON, log)

	if out.Interpretation == "" {
		t.Error("interpretation should never be empty — it is the actionable part of the result")
	}
}

func TestConformanceErrors(t *testing.T) {
	tests := []struct {
		name  string
		args  map[string]any
		match string
	}{
		{"missing model", map[string]any{"log": `[{"case":"a","activity":"b"}]`}, "model"},
		{"missing log", map[string]any{"model": orderModelJSON}, "log"},
		{"bad model", map[string]any{"model": "nope", "log": `[{"case":"a","activity":"b"}]`}, "model"},
		{"log not an array", map[string]any{"model": orderModelJSON, "log": `{"case":"a"}`}, "array"},
		{"empty log", map[string]any{"model": orderModelJSON, "log": `[]`}, "empty"},
		{"no usable events", map[string]any{"model": orderModelJSON, "log": `[{"foo":"bar"}]`}, "case"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "petri_conformance"
			req.Params.Arguments = tt.args

			result, err := handleConformance(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result, got: %s", resultText(result))
			}
			if !strings.Contains(strings.ToLower(resultText(result)), strings.ToLower(tt.match)) {
				t.Errorf("error %q should mention %q", resultText(result), tt.match)
			}
		})
	}
}

func TestParseEventLogTimestampFormats(t *testing.T) {
	log := `[
      {"case":"a","activity":"x","timestamp":"2026-01-01T10:00:00Z"},
      {"case":"b","activity":"x","time":"2026-01-02 11:00:00"},
      {"case":"c","activity":"x","ts":"2026-01-03"}
    ]`

	parsed, _, err := parseEventLog(log)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(parsed.Cases) != 3 {
		t.Errorf("got %d cases, want 3", len(parsed.Cases))
	}
}
