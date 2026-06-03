package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// competingFatesModel: a single source `pending` (10 tokens) drains into
// two competing sinks via parallel transitions. The rate ratio between the
// two transitions determines how the 10 tokens split — a clean Pareto
// trade-off between the two sinks.
func competingFatesModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "pending", Initial: 10, X: 80, Y: 150},
			{ID: "delivered", X: 360, Y: 80},
			{ID: "refunded", X: 360, Y: 220},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "deliver", X: 220, Y: 80},
			{ID: "refund", X: 220, Y: 220},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "pending", To: "deliver"},
			{From: "deliver", To: "delivered"},
			{From: "pending", To: "refund"},
			{From: "refund", To: "refunded"},
		},
	}
}

func TestOptimize_TwoObjectives(t *testing.T) {
	// Both sinks are MAX objectives — they trade off because they share
	// the pending pool. Pareto frontier is the line delivered + refunded = 10.
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_optimize"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, competingFatesModel()),
		"parameters": `{"deliver":[0.1, 5.0], "refund":[0.1, 5.0]}`,
		"objectives": `[{"place":"delivered","direction":"max"},{"place":"refunded","direction":"max"}]`,
		"samples":    100,
	}
	res, err := handleOptimize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOptimize: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp optimizeResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Samples) != 100 {
		t.Errorf("samples = %d, want 100", len(resp.Samples))
	}
	// Both sinks share the pending pool, so samples cluster along
	// delivered + refunded = 10. Every sample on that line is non-dominated,
	// so we expect many Pareto points — not just one.
	if resp.ParetoCount < 10 {
		t.Errorf("expected many Pareto samples on the trade-off line, got %d", resp.ParetoCount)
	}
	// The sweep should find a sample close to (delivered=10, refunded=0) —
	// that's what happens when deliver >> refund.
	maxDelivered := 0.0
	for _, s := range resp.Samples {
		if s.Values["delivered"] > maxDelivered {
			maxDelivered = s.Values["delivered"]
		}
	}
	if maxDelivered < 9.0 {
		t.Errorf("max delivered = %v, expected close to 10 in the sample cloud", maxDelivered)
	}
	if path := os.Getenv("OPTIMIZE_2OBJ_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

// threeWayModel: source feeding three sinks (delivered / refunded / expired).
// 3 objectives — exercises the parallel-coordinates renderer.
func threeWayModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "pending", Initial: 10, X: 80, Y: 150},
			{ID: "delivered", X: 360, Y: 50},
			{ID: "refunded", X: 360, Y: 150},
			{ID: "expired", X: 360, Y: 250},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "deliver", X: 220, Y: 50},
			{ID: "refund", X: 220, Y: 150},
			{ID: "expire", X: 220, Y: 250},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "pending", To: "deliver"}, {From: "deliver", To: "delivered"},
			{From: "pending", To: "refund"}, {From: "refund", To: "refunded"},
			{From: "pending", To: "expire"}, {From: "expire", To: "expired"},
		},
	}
}

func TestOptimize_ThreeObjectives(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_optimize"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, threeWayModel()),
		"parameters": `{"deliver":[0.1, 5.0], "refund":[0.1, 5.0], "expire":[0.1, 5.0]}`,
		"objectives": `[{"place":"delivered","direction":"max"},{"place":"refunded","direction":"max"},{"place":"expired","direction":"max"}]`,
		"samples":    150,
	}
	res, err := handleOptimize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOptimize: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("OPTIMIZE_3OBJ_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestOptimize_Pareto_Dominance(t *testing.T) {
	// Hand-built samples to exercise the dominance logic.
	samples := []optimizeSample{
		{Values: map[string]float64{"a": 5, "b": 1}},
		{Values: map[string]float64{"a": 4, "b": 2}},
		{Values: map[string]float64{"a": 3, "b": 3}},
		{Values: map[string]float64{"a": 1, "b": 1}}, // dominated by sample[0]
		{Values: map[string]float64{"a": 5, "b": 0}}, // dominated by sample[0]? a same, b worse → dominated
	}
	objectives := []objectiveSpec{
		{Place: "a", Direction: "max"},
		{Place: "b", Direction: "max"},
	}
	count := identifyPareto(samples, objectives)
	// Samples 0, 1, 2 are non-dominated (frontier). Sample 3 is dominated by all
	// three. Sample 4 has a=5 (same as 0) but b=0 (worse than 0) → dominated.
	if count != 3 {
		t.Errorf("Pareto count = %d, want 3", count)
	}
	if !samples[0].IsPareto || !samples[1].IsPareto || !samples[2].IsPareto {
		t.Errorf("samples 0-2 should be Pareto-optimal")
	}
	if samples[3].IsPareto {
		t.Errorf("sample 3 should be dominated")
	}
	if samples[4].IsPareto {
		t.Errorf("sample 4 should be dominated")
	}
}

func TestOptimize_RejectsSingleObjective(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_optimize"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, competingFatesModel()),
		"parameters": `{"deliver":[0.1,5.0]}`,
		"objectives": `[{"place":"delivered","direction":"max"}]`,
	}
	res, err := handleOptimize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOptimize: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for single-objective input")
	}
}
