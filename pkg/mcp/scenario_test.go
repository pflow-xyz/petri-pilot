package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

const staffedShopJSON = `{
  "name": "shop",
  "places": [
    {"id": "queue"}, {"id": "brewing"}, {"id": "served"},
    {"id": "available", "initial": 1}, {"id": "busy"}
  ],
  "transitions": [
    {"id": "arrive", "rate": 20}, {"id": "start", "rate": 60}, {"id": "finish", "rate": 15}
  ],
  "arcs": [
    {"from": "arrive", "to": "queue"},
    {"from": "queue", "to": "start"}, {"from": "available", "to": "start"},
    {"from": "start", "to": "brewing"}, {"from": "start", "to": "busy"},
    {"from": "brewing", "to": "finish"}, {"from": "busy", "to": "finish"},
    {"from": "finish", "to": "served"}, {"from": "finish", "to": "available"}
  ]
}`

func callScenario(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	args["model"] = staffedShopJSON
	res, err := handleScenario(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "petri_scenario", Arguments: args},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestScenarioToolComparesStaffing: the tool's reason for existing. Every other
// analytic tool varies a rate; this one varies the marking, which is where
// "how many people are on shift" actually lives.
func TestScenarioToolComparesStaffing(t *testing.T) {
	res := callScenario(t, map[string]any{
		"scenarios":    `[{"name":"one","marking":{"available":1}},{"name":"three","marking":{"available":3}}]`,
		"hours":        float64(8),
		"realizations": float64(10),
	})
	if res.IsError {
		t.Fatalf("tool errored: %s", resultText(res))
	}

	var summary comparisonSummary
	if err := json.Unmarshal([]byte(resultText(res)), &summary); err != nil {
		t.Fatalf("%v\n%s", err, resultText(res))
	}
	if len(summary.Scenarios) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(summary.Scenarios))
	}
	one, three := summary.Scenarios[0], summary.Scenarios[1]
	if three.Throughput["finish"] <= one.Throughput["finish"] {
		t.Errorf("three baristas served %.1f, one served %.1f", three.Throughput["finish"], one.Throughput["finish"])
	}
	if three.Utilization["pool"] >= one.Utilization["pool"] {
		t.Errorf("utilization did not fall with more staff: %v vs %v", three.Utilization, one.Utilization)
	}
	t.Logf("one: %v  three: %v", one.MeanPerc95, three.MeanPerc95)
}

// TestScenarioToolRejectsAnUnknownPlace: a silently ignored knob would produce
// a confident "no difference" to a question that was never asked.
func TestScenarioToolRejectsAnUnknownPlace(t *testing.T) {
	res := callScenario(t, map[string]any{"marking": `{"baristas": 3}`, "hours": float64(1)})
	if !res.IsError {
		t.Fatal("accepted a place the model does not have")
	}
	if !strings.Contains(resultText(res), "baristas") {
		t.Errorf("the error does not name the offending place: %s", resultText(res))
	}
}

// TestScenarioToolSchedulesARush: piecewise rates are the one thing the
// existing rate tools cannot express.
func TestScenarioToolSchedulesARush(t *testing.T) {
	res := callScenario(t, map[string]any{
		"schedule":     `{"arrive": [{"until": 1, "value": 300}, {"until": 8, "value": 1}]}`,
		"hours":        float64(8),
		"samples":      float64(80),
		"realizations": float64(5),
	})
	if res.IsError {
		t.Fatalf("tool errored: %s", resultText(res))
	}
	var out struct {
		Metrics struct {
			P95 map[string]float64 `json:"p95"`
		} `json:"metrics"`
		Times []float64 `json:"times"`
	}
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if n := len(out.Times); n == 0 || out.Times[n-1] < 7.5 {
		t.Fatalf("the run does not cover the horizon: %v", out.Times)
	}
	if out.Metrics.P95["queue"] <= 0 {
		t.Error("a 300/h rush against one barista produced no queue at all")
	}
}
