package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestRateScan_CoffeeShop_ValuesArray(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_rate_scan"
	req.Params.Arguments = map[string]any{
		"model":       mustJSON(t, coffeeShopModel()),
		"transition":  "start_brew",
		"values":      "[0.1, 0.3, 0.5, 1.0, 2.0, 5.0]",
		"observables": `["delivered","order_pending"]`,
	}
	res, err := handleRateScan(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRateScan: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp rateScanResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Transition != "start_brew" {
		t.Errorf("transition = %q, want start_brew", resp.Transition)
	}
	if len(resp.Results) != 6 {
		t.Errorf("results count = %d, want 6", len(resp.Results))
	}
	// At higher rates, equilibrium should still have delivered ~2 (mass
	// conservation at steady state — start_brew rate just changes how fast
	// we get there).
	last := resp.Results[len(resp.Results)-1]
	if last.Final["delivered"] < 1.5 {
		t.Errorf("at rate=5 expected delivered ~2, got %v", last.Final["delivered"])
	}
	if path := os.Getenv("RATE_SCAN_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestRateScan_CoffeeShop_RangeForm(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_rate_scan"
	req.Params.Arguments = map[string]any{
		"model":       mustJSON(t, coffeeShopModel()),
		"transition":  "deliver",
		"range":       "[0.1, 2.0, 10]",
		"observables": `["delivered","ready"]`,
		"plot":        false,
	}
	res, err := handleRateScan(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRateScan: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp rateScanResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Results) != 10 {
		t.Errorf("results count = %d, want 10", len(resp.Results))
	}
	if len(resp.Values) != 10 {
		t.Errorf("values count = %d, want 10", len(resp.Values))
	}
	if resp.Values[0] != 0.1 || resp.Values[9] != 2.0 {
		t.Errorf("range endpoints wrong: first=%v last=%v", resp.Values[0], resp.Values[9])
	}
}

func TestRateScan_RejectsMissingTransition(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_rate_scan"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, coffeeShopModel()),
		"transition": "nonexistent",
		"values":     "[1.0]",
	}
	res, err := handleRateScan(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRateScan: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for nonexistent transition")
	}
}

func TestRateScan_Linspace(t *testing.T) {
	got := linspace(0, 10, 5)
	want := []float64{0, 2.5, 5, 7.5, 10}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
