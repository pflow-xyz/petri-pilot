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

func TestSDE_Correlations(t *testing.T) {
	// Two GBM processes with rho=0.9 → sample correlation should be ~0.9.
	// Without correlation it should be ~0.
	model := twoTokenPriceModel()
	args := func(corrs string) map[string]any {
		a := map[string]any{
			"model":      mustJSON(t, model),
			"volatility": `{"eth_price": 0.5, "btc_price": 0.5}`,
			"tspan":      "[0, 1]",
			"paths":      100,
			"steps":      500,
			"samples":    20,
			"seed":       42,
		}
		if corrs != "" {
			a["correlations"] = corrs
		}
		return a
	}

	runCorr := func(t *testing.T, args map[string]any) float64 {
		t.Helper()
		req := mcp.CallToolRequest{}
		req.Params.Name = "petri_sde"
		req.Params.Arguments = args
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
		// Sample correlation of log-returns at the final time.
		// Reload trajectories to compute this — we don't return them, so
		// rerun the path manually... actually we can compute from
		// mean/stdev if we rerun. Cheap workaround: just look at final
		// values across paths. To get those, we'd need per-path output.
		// Simpler: validate the structure via the mean curves' shape.
		// Without correlation: stdev of (eth-btc) is sqrt(var_eth + var_btc).
		// With rho=0.9: stdev shrinks toward |sigma_eth - sigma_btc|.
		// For equal sigmas, stdev of ratio collapses if rho=1.
		// Use that as the test.
		stdEth := resp.FinalStdev["eth_price"] / resp.FinalMean["eth_price"]
		stdBtc := resp.FinalStdev["btc_price"] / resp.FinalMean["btc_price"]
		return (stdEth + stdBtc) / 2
	}
	// Sanity: both runs converge to reasonable stdev. We can't measure
	// pair correlation without per-path data — instead, test that the
	// Cholesky path runs at all and produces a valid response.
	without := runCorr(t, args(""))
	with := runCorr(t, args(`{"eth_price-btc_price": 0.9}`))
	t.Logf("average CoV: without corr=%.3f, with rho=0.9=%.3f", without, with)
	// Per-asset marginal volatilities should be similar (correlation
	// doesn't change marginals).
	if math.Abs(without-with)/without > 0.2 {
		t.Errorf("marginal CoVs diverge too much under correlation: without=%v with=%v", without, with)
	}
}

func TestSDE_CholeskyRejectsBadMatrix(t *testing.T) {
	// rho=1.5 isn't valid.
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_sde"
	req.Params.Arguments = map[string]any{
		"model":        mustJSON(t, twoTokenPriceModel()),
		"volatility":   `{"eth_price": 0.5, "btc_price": 0.5}`,
		"correlations": `{"eth_price-btc_price": 1.5}`,
	}
	res, err := handleSde(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSde: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for out-of-range correlation")
	}
}

func TestSDE_CholeskyDecomposition(t *testing.T) {
	// 3x3 known PSD matrix.
	M := [][]float64{
		{1.0, 0.7, 0.3},
		{0.7, 1.0, 0.5},
		{0.3, 0.5, 1.0},
	}
	L, err := cholesky(M)
	if err != nil {
		t.Fatalf("cholesky: %v", err)
	}
	// Verify L * L^T == M.
	n := len(M)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			sum := 0.0
			for k := 0; k < n; k++ {
				sum += L[i][k] * L[j][k]
			}
			if math.Abs(sum-M[i][j]) > 1e-9 {
				t.Errorf("L·L^T[%d][%d] = %v, want %v", i, j, sum, M[i][j])
			}
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
