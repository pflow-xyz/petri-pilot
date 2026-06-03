package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestOdeSweep_CoffeeShop(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode_sweep"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, coffeeShopModel()),
		"transition": "start_brew",
		"observable": "delivered",
		"range":      "[0.1, 3.0, 6]",
		"tspan":      "[0, 8]",
	}
	res, err := handleOdeSweep(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOdeSweep: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp odeSweepResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Transition != "start_brew" {
		t.Errorf("transition = %q, want start_brew", resp.Transition)
	}
	if resp.Observable != "delivered" {
		t.Errorf("observable = %q, want delivered", resp.Observable)
	}
	if len(resp.Trajectories) != 6 {
		t.Errorf("trajectories count = %d, want 6", len(resp.Trajectories))
	}
	for _, traj := range resp.Trajectories {
		if len(traj.T) < 2 {
			t.Errorf("trajectory at rate %v has only %d samples", traj.Rate, len(traj.T))
		}
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d bytes", len(img))
	}
	if path := os.Getenv("ODE_SWEEP_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestOdeSweep_RejectsMissingObservable(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_ode_sweep"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, coffeeShopModel()),
		"transition": "start_brew",
		"observable": "nonexistent",
		"values":     "[1.0]",
	}
	res, err := handleOdeSweep(context.Background(), req)
	if err != nil {
		t.Fatalf("handleOdeSweep: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for nonexistent observable")
	}
}
