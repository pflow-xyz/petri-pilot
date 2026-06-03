package mcp

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestOdeSensitivity_CoffeeShop(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode_sensitivity"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, coffeeShopModel()),
		"observable": "delivered",
		"delta":      0.05,
		"tspan":      "[0, 20]",
	}
	res, err := handleOdeSensitivity(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOdeSensitivity: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp odeSensitivityResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Observable != "delivered" {
		t.Errorf("observable = %q, want delivered", resp.Observable)
	}
	if len(resp.Elasticities) != 3 {
		t.Errorf("expected 3 elasticities (one per transition), got %d", len(resp.Elasticities))
	}
	// All three transitions are in series toward delivered, so all three
	// should have non-zero elasticity. At equilibrium (delivered ~2),
	// rate changes that compress time-to-equilibrium shouldn't change the
	// final value much — elasticities can be small but non-NaN.
	for tid, e := range resp.Elasticities {
		if math.IsNaN(e) {
			t.Errorf("elasticity for %q is NaN", tid)
		}
	}
	if path := os.Getenv("ODE_SENSITIVITY_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestOdeSensitivity_RejectsMissingObservable(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode_sensitivity"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, coffeeShopModel()),
		"observable": "nonexistent",
	}
	res, err := handleOdeSensitivity(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOdeSensitivity: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for nonexistent observable")
	}
}
