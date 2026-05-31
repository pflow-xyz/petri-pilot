package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func callOde(t *testing.T, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode"
	req.Params.Arguments = args
	res, err := handleOde(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOde returned error: %v", err)
	}
	if res == nil {
		t.Fatalf("handleOde returned nil result")
	}
	return res
}

func textBlock(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := mcp.AsTextContent(c); ok {
			return tc.Text
		}
	}
	t.Fatalf("no text content in result")
	return ""
}

func TestHandleOde_Solve(t *testing.T) {
	modelJSON := mustJSON(t, coffeeShopModel())
	res := callOde(t, map[string]any{
		"model":   modelJSON,
		"tspan":   "[0, 5]",
		"samples": 50,
		"plot":    false,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textBlock(t, res))
	}
	var resp odeResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Method != "tsit5" {
		t.Errorf("method = %q, want tsit5", resp.Method)
	}
	if len(resp.Samples) < 2 {
		t.Errorf("expected ≥2 samples, got %d", len(resp.Samples))
	}
	if resp.Final["delivered"] <= 0 {
		t.Errorf("delivered should be > 0 at t=5, got %v", resp.Final["delivered"])
	}
	if resp.Final["order_pending"] >= 2 {
		t.Errorf("order_pending should have decreased from 2, got %v", resp.Final["order_pending"])
	}
}

func TestHandleOde_Equilibrium(t *testing.T) {
	modelJSON := mustJSON(t, coffeeShopModel())
	res := callOde(t, map[string]any{
		"model": modelJSON,
		"tspan": "[0, 50]",
		"mode":  "equilibrium",
		"plot":  false,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textBlock(t, res))
	}
	var resp odeResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Equilibrium == nil {
		t.Fatalf("expected equilibrium result")
	}
	if !resp.Equilibrium.Reached {
		t.Errorf("equilibrium should have been reached; reason=%q", resp.Equilibrium.Reason)
	}
	if resp.Final["delivered"] < 1.9 {
		t.Errorf("at equilibrium delivered should be ~2, got %v", resp.Final["delivered"])
	}
}

func TestHandleOde_RejectsBadMethod(t *testing.T) {
	modelJSON := mustJSON(t, coffeeShopModel())
	res := callOde(t, map[string]any{
		"model":  modelJSON,
		"method": "bogus",
	})
	if !res.IsError {
		t.Fatalf("expected error for bogus method")
	}
	if !strings.Contains(textBlock(t, res), "method") {
		t.Errorf("error should mention method, got %q", textBlock(t, res))
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
