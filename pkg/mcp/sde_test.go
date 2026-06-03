package mcp

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// pureGBMModel: a single place "price" with no drift transitions — pure
// geometric Brownian motion when sigma > 0. Validates the SDE math against
// the analytic GBM second moment.
func pureGBMModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "price", Initial: 100, X: 100, Y: 100},
		},
		Transitions: []goflowmetamodel.Transition{}, // no drift
	}
}

func TestSDE_GBM_RoughlyMatches(t *testing.T) {
	// For S(0)=100, sigma=0.3, T=1 and no drift, E[S(T)]=100 and
	// Var(S(T)) ≈ S(0)² (exp(σ²T) - 1) ≈ 10000 × (exp(0.09) - 1) ≈ 940
	// → stdev ≈ 30. With 80 paths we expect noisy but not absurd estimates.
	model := pureGBMModel()

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_sde"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, model),
		"volatility": `{"price": 0.3}`,
		"tspan":      "[0, 1]",
		"paths":      80,
		"steps":      1000,
		"samples":    80,
		"seed":       42,
	}
	res, err := handleSde(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSde: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp sdeResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Paths != 80 {
		t.Errorf("paths = %d, want 80", resp.Paths)
	}
	finalMean := resp.FinalMean["price"]
	finalStdev := resp.FinalStdev["price"]
	t.Logf("final price: mean=%.2f, stdev=%.2f (analytic E≈100, stdev≈30)", finalMean, finalStdev)
	// Mean should be near 100 ± ~10% with 80 paths.
	if math.Abs(finalMean-100) > 20 {
		t.Errorf("E[S(T)] = %v, want ~100 ± 20", finalMean)
	}
	// Stdev should be roughly 20-40 (the analytic value is ~30).
	if finalStdev < 15 || finalStdev > 50 {
		t.Errorf("Std[S(T)] = %v, want roughly 30", finalStdev)
	}
	if path := os.Getenv("SDE_GBM_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

// twoTokenPriceModel: two price places with independent volatilities — the
// kind of setup you'd use to drive an AMM impermanent-loss simulation.
func twoTokenPriceModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "eth_price", Initial: 2000, X: 100, Y: 100},
			{ID: "btc_price", Initial: 50000, X: 300, Y: 100},
		},
	}
}

func TestSDE_TwoPriceProcesses(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_sde"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, twoTokenPriceModel()),
		"volatility": `{"eth_price": 0.6, "btc_price": 0.45}`,
		"tspan":      "[0, 1]",
		"paths":      100,
		"steps":      500,
		"samples":    60,
		"seed":       7,
	}
	res, err := handleSde(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSde: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp sdeResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// ETH should show more relative noise than BTC because sigma is higher.
	ethCoV := resp.FinalStdev["eth_price"] / resp.FinalMean["eth_price"]
	btcCoV := resp.FinalStdev["btc_price"] / resp.FinalMean["btc_price"]
	t.Logf("CoV: ETH=%.3f, BTC=%.3f", ethCoV, btcCoV)
	if ethCoV <= btcCoV {
		t.Errorf("expected ETH CoV (%v) > BTC CoV (%v) since sigma_eth > sigma_btc", ethCoV, btcCoV)
	}
	if path := os.Getenv("SDE_TWOTOKEN_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestSDE_RejectsUnknownPlace(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_sde"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, pureGBMModel()),
		"volatility": `{"nonexistent": 0.5}`,
	}
	res, err := handleSde(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSde: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for unknown place in volatility")
	}
}
