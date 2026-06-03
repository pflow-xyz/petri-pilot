package mcp

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestStochastic_CoffeeShop_SingleRealization(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"tspan":   "[0, 10]",
		"samples": 100,
		"seed":    42,
	}
	res, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp stochasticResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Realizations != 1 {
		t.Errorf("realizations = %d, want 1", resp.Realizations)
	}
	if len(resp.Times) != 100 {
		t.Errorf("times count = %d, want 100", len(resp.Times))
	}
	// Mass conservation: total token count is preserved by transitions
	// (each fires 2 in / 1 out then 1 in / 1+1 out → net 0 over a cycle).
	// We have 2 orders + 1 barista = 3 tokens. With single realization,
	// final total should be near 3 (could be ±1 due to incomplete cycles).
	total := 0.0
	for _, v := range resp.FinalMean {
		total += v
	}
	if math.Abs(total-3) > 1 {
		t.Errorf("final total tokens = %v, want ~3", total)
	}
}

func TestStochastic_MultipleRealizations(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model":        mustJSON(t, coffeeShopModel()),
		"tspan":        "[0, 10]",
		"samples":      80,
		"realizations": 20,
		"seed":         42,
	}
	res, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp stochasticResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Realizations != 20 {
		t.Errorf("realizations = %d, want 20", resp.Realizations)
	}
	if resp.Stdev == nil || resp.Stdev["delivered"] == nil {
		t.Errorf("expected stdev for 'delivered' with multiple realizations")
	}
	// Mean of 'delivered' should approach 2 over time (mean of the
	// stochastic ensemble matches the deterministic equilibrium).
	finalDel := resp.FinalMean["delivered"]
	if finalDel < 1.5 || finalDel > 2.5 {
		t.Errorf("mean delivered at t=10 = %v, want ~2", finalDel)
	}
	if path := os.Getenv("STOCHASTIC_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestStochastic_Combinations(t *testing.T) {
	cases := []struct {
		m, w int
		want float64
	}{
		{0, 1, 0},
		{1, 1, 1},
		{5, 1, 5},
		{5, 2, 10}, // C(5, 2) = 10
		{5, 3, 10}, // C(5, 3) = 10
		{10, 0, 1},
	}
	for _, c := range cases {
		got := combinations(c.m, c.w)
		if got != c.want {
			t.Errorf("combinations(%d, %d) = %v, want %v", c.m, c.w, got, c.want)
		}
	}
}

func TestStochastic_Reproducible(t *testing.T) {
	// Same seed should produce identical realizations.
	args := map[string]any{
		"model":        mustJSON(t, coffeeShopModel()),
		"tspan":        "[0, 5]",
		"samples":      30,
		"realizations": 5,
		"seed":         123,
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = args
	res1, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic 1: %v", err)
	}
	res2, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic 2: %v", err)
	}
	if textBlock(t, res1) != textBlock(t, res2) {
		t.Errorf("same seed produced different output")
	}
}
