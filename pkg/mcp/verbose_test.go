package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestVerbose_OdePopulatesExplanation(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"tspan":   "[0, 5]",
		"samples": 30,
		"plot":    false,
		"verbose": true,
	}
	res, err := handleOde(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOde: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp odeResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Explanation == "" {
		t.Fatalf("verbose=true should populate Explanation")
	}
	for _, want := range []string{"Tsit5", "mass-action", "petri_explain"} {
		if !strings.Contains(resp.Explanation, want) {
			t.Errorf("ODE explanation missing %q:\n%s", want, resp.Explanation)
		}
	}
}

func TestVerbose_OdeEquilibriumMentionsDetection(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"tspan":   "[0, 20]",
		"mode":    "equilibrium",
		"plot":    false,
		"verbose": true,
	}
	res, _ := handleOde(context.Background(), req)
	var resp odeResponse
	json.Unmarshal([]byte(textBlock(t, res)), &resp)
	if !strings.Contains(resp.Explanation, "Equilibrium detection") {
		t.Errorf("equilibrium mode should switch annotation to equilibrium detection variant:\n%s", resp.Explanation)
	}
	if !strings.Contains(resp.Explanation, "effectiveReached") {
		t.Errorf("equilibrium annotation should mention effectiveReached")
	}
}

func TestVerbose_StochasticPopulatesExplanation(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"tspan":   "[0, 5]",
		"samples": 30,
		"seed":    1,
		"verbose": true,
	}
	res, _ := handleStochastic(context.Background(), req)
	var resp stochasticResponse
	json.Unmarshal([]byte(textBlock(t, res)), &resp)
	if resp.Explanation == "" {
		t.Fatalf("verbose=true should populate Explanation")
	}
	for _, want := range []string{"Gillespie", "propensities", "petri_explain"} {
		if !strings.Contains(resp.Explanation, want) {
			t.Errorf("stochastic explanation missing %q", want)
		}
	}
}

func TestVerbose_SdePopulatesExplanation(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_sde"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, pureGBMModel()),
		"volatility": `{"price": 0.3}`,
		"tspan":      "[0, 1]",
		"paths":      10,
		"steps":      100,
		"samples":    20,
		"verbose":    true,
	}
	res, _ := handleSde(context.Background(), req)
	var resp sdeResponse
	json.Unmarshal([]byte(textBlock(t, res)), &resp)
	if resp.Explanation == "" {
		t.Fatalf("verbose=true should populate Explanation")
	}
	for _, want := range []string{"Euler-Maruyama", "drift", "dW"} {
		if !strings.Contains(resp.Explanation, want) {
			t.Errorf("SDE explanation missing %q", want)
		}
	}
}

func TestVerbose_OptimizePopulatesExplanation(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_optimize"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, competingFatesModel()),
		"parameters": `{"deliver":[0.1,5.0], "refund":[0.1,5.0]}`,
		"objectives": `[{"place":"delivered","direction":"max"},{"place":"refunded","direction":"max"}]`,
		"samples":    30,
		"verbose":    true,
	}
	res, _ := handleOptimize(context.Background(), req)
	var resp optimizeResponse
	json.Unmarshal([]byte(textBlock(t, res)), &resp)
	if resp.Explanation == "" {
		t.Fatalf("verbose=true should populate Explanation")
	}
	for _, want := range []string{"Monte Carlo", "Pareto", "dominates"} {
		if !strings.Contains(resp.Explanation, want) {
			t.Errorf("optimize explanation missing %q", want)
		}
	}
}
