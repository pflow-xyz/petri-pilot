package mcp

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestAmmQuote_BasicSwap(t *testing.T) {
	// Standard reference case: 100 ETH ↔ 200,000 USDC pool, swap 1 ETH in.
	// Spot price = 2000 USDC/ETH. With 0.3% fee:
	//   dxAfterFee = 0.997
	//   dy = 200000 * 0.997 / (100 + 0.997) ≈ 1974.69
	//   slippage ≈ (1 - 1974.69/2000) ≈ 1.27%
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_amm_quote"
	req.Params.Arguments = map[string]any{
		"reserve_x": 100.0,
		"reserve_y": 200000.0,
		"amount_in": 1.0,
	}
	res, err := handleAmmQuote(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAmmQuote: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp ammQuoteResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if math.Abs(resp.SpotPrice-2000) > 0.01 {
		t.Errorf("spot price = %v, want 2000", resp.SpotPrice)
	}
	if resp.AmountOut < 1970 || resp.AmountOut > 1980 {
		t.Errorf("amount out = %v, want ~1974", resp.AmountOut)
	}
	if resp.FeePaid != 0.003 {
		t.Errorf("fee paid = %v, want 0.003 (0.3%% of 1 ETH)", resp.FeePaid)
	}
}

func TestAmmIL_KnownValues(t *testing.T) {
	// For r=2 (price doubles): IL = 2·sqrt(2)/3 − 1 ≈ -0.0572 → -5.72%
	// For r=4: IL = 4/5 - 1 = -20%
	// For r=0.5: same as r=2 by symmetry
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_amm_il"
	req.Params.Arguments = map[string]any{
		"price_ratios": `[0.25, 0.5, 1, 2, 4]`,
	}
	res, err := handleAmmIL(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAmmIL: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp ammILResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.IL) != 5 {
		t.Fatalf("expected 5 IL values, got %d", len(resp.IL))
	}
	// Find r=1 → IL should be 0
	for i, r := range resp.PriceRatios {
		if math.Abs(r-1) < 1e-6 && math.Abs(resp.IL[i]) > 1e-6 {
			t.Errorf("IL at r=1 should be 0, got %v", resp.IL[i])
		}
	}
	// Find r=2 → IL ≈ -5.72%
	for i, r := range resp.PriceRatios {
		if math.Abs(r-2) < 1e-6 {
			if math.Abs(resp.IL[i]+5.72) > 0.01 {
				t.Errorf("IL at r=2 = %v, want ≈ -5.72", resp.IL[i])
			}
		}
	}
	// Find r=4 → IL = -20%
	for i, r := range resp.PriceRatios {
		if math.Abs(r-4) < 1e-6 {
			if math.Abs(resp.IL[i]+20) > 0.01 {
				t.Errorf("IL at r=4 = %v, want -20", resp.IL[i])
			}
		}
	}
}

func TestAmmIL_BreakevenPlot(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_amm_il"
	req.Params.Arguments = map[string]any{
		"range":               `[0.1, 10, 100]`,
		"fee_apy":             0.20,
		"holding_period_days": 365,
	}
	res, err := handleAmmIL(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAmmIL: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp ammILResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.FeeYieldPct != 20 {
		t.Errorf("FeeYieldPct = %v, want 20", resp.FeeYieldPct)
	}
	if len(resp.BreakevenRatios) != 2 {
		t.Errorf("expected 2 breakeven ratios (above/below r=1), got %d: %v", len(resp.BreakevenRatios), resp.BreakevenRatios)
	}
	if path := os.Getenv("AMM_IL_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestAmm_VerboseDerivations(t *testing.T) {
	// petri_amm_quote with verbose=true should include a derivation field
	// that walks through the formula and the substitution.
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_amm_quote"
	req.Params.Arguments = map[string]any{
		"reserve_x": 100.0,
		"reserve_y": 200000.0,
		"amount_in": 1.0,
		"verbose":   true,
	}
	res, err := handleAmmQuote(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAmmQuote: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp ammQuoteResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Derivation == "" {
		t.Fatalf("verbose=true should populate Derivation, got empty")
	}
	for _, want := range []string{"x · y", "Δx_after_fee", "Spot price", "Price impact"} {
		if !strings.Contains(resp.Derivation, want) {
			t.Errorf("derivation missing %q", want)
		}
	}

	// petri_amm_il with verbose=true should include the IL formula.
	req2 := mcp.CallToolRequest{}
	req2.Params.Name = "petri_amm_il"
	req2.Params.Arguments = map[string]any{
		"price_ratios": `[0.5, 1, 2]`,
		"verbose":      true,
	}
	res, err = handleAmmIL(context.Background(), req2)
	if err != nil {
		t.Fatalf("handleAmmIL: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var ilResp ammILResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &ilResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ilResp.Derivation == "" {
		t.Fatalf("verbose=true should populate Derivation, got empty")
	}
	if !strings.Contains(ilResp.Derivation, "2·√r") {
		t.Errorf("IL derivation missing the canonical 2·√r formula")
	}
}

func TestAmmDepth_RealisticPool(t *testing.T) {
	// ETH/USDC pool with ~1M USDC liquidity (~500 ETH).
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_amm_depth"
	req.Params.Arguments = map[string]any{
		"reserve_x":  500.0,
		"reserve_y":  1000000.0,
		"size_range": `[0.001, 0.5, 60]`,
	}
	res, err := handleAmmDepth(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAmmDepth: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp ammDepthResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Points) != 60 {
		t.Errorf("expected 60 points, got %d", len(resp.Points))
	}
	// Slippage must be monotonic increasing with trade size.
	for i := 1; i < len(resp.Points); i++ {
		if resp.Points[i].SlippageBps < resp.Points[i-1].SlippageBps {
			t.Errorf("slippage non-monotonic at %d", i)
			break
		}
	}
	if path := os.Getenv("AMM_DEPTH_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}
