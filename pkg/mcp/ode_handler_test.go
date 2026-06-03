package mcp

import (
	"context"
	"encoding/json"
	"os"
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

func TestHandleOde_Layouts(t *testing.T) {
	modelJSON := mustJSON(t, coffeeShopModel())
	cases := []struct {
		layout string
		envVar string
	}{
		{"plot", "ODE_LAYOUT_PLOT_OUT"},
		{"combined", "ODE_LAYOUT_COMBINED_OUT"},
		{"net", "ODE_LAYOUT_NET_OUT"},
	}
	for _, tc := range cases {
		t.Run(tc.layout, func(t *testing.T) {
			res := callOde(t, map[string]any{
				"model":   modelJSON,
				"tspan":   "[0, 20]",
				"samples": 40,
				"layout":  tc.layout,
			})
			if res.IsError {
				t.Fatalf("error: %s", textBlock(t, res))
			}
			img := extractImageBytes(t, res)
			if len(img) < 1000 {
				t.Fatalf("image too small for layout %s: %d", tc.layout, len(img))
			}
			if path := os.Getenv(tc.envVar); path != "" {
				if err := os.WriteFile(path, img, 0644); err != nil {
					t.Fatalf("write: %v", err)
				}
				t.Logf("wrote %d bytes to %s", len(img), path)
			}
		})
	}
}

func TestHandleOde_EquilibriumEffectiveReached(t *testing.T) {
	// Short tspan: system reaches steady-state values well before the
	// consecutive-steps gate fires. Expect Reached=true with the
	// EffectiveReached signal so the response doesn't lie.
	modelJSON := mustJSON(t, coffeeShopModel())
	res := callOde(t, map[string]any{
		"model": modelJSON,
		"tspan": "[0, 20]",
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
		t.Errorf("Reached should be true (either gate-fired or effective); reason=%q maxChange=%v", resp.Equilibrium.Reason, resp.Equilibrium.MaxChange)
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
