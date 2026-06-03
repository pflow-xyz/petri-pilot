package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestCorrMatrix_FromPairs(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_corr_matrix"
	req.Params.Arguments = map[string]any{
		"correlations": `{"btc-eth": 0.85, "btc-sol": 0.7, "eth-sol": 0.75}`,
		"title":        "Crypto correlations",
	}
	res, err := handleCorrMatrix(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCorrMatrix: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("CORR_MATRIX_OUT"); path != "" {
		_ = os.WriteFile(path, img, 0644)
	}
}

func TestPhasePlot_CompetingFates(t *testing.T) {
	model := competingFatesModel()
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_phase_plot"
	req.Params.Arguments = map[string]any{
		"model":              mustJSON(t, model),
		"place_x":            "delivered",
		"place_y":            "refunded",
		"tspan":              "[0, 20]",
		"initial_conditions": `[{"pending":10},{"pending":5},{"pending":2}]`,
	}
	res, err := handlePhasePlot(context.Background(), req)
	if err != nil {
		t.Fatalf("handlePhasePlot: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("PHASE_PLOT_OUT"); path != "" {
		_ = os.WriteFile(path, img, 0644)
	}
}

func TestDistribution_SDE(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_distribution"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, pureGBMModel()),
		"observable": "price",
		"mode":       "sde",
		"volatility": `{"price": 0.4}`,
		"tspan":      "[0, 1]",
		"paths":      300,
		"bins":       25,
	}
	res, err := handleDistribution(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDistribution: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("DISTRIBUTION_OUT"); path != "" {
		_ = os.WriteFile(path, img, 0644)
	}
}

func TestParamHeatmap_CompetingFates(t *testing.T) {
	model := competingFatesModel()
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_param_heatmap"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, model),
		"param_x":    "deliver",
		"param_y":    "refund",
		"observable": "delivered",
		"range_x":    `[0.1, 5.0, 15]`,
		"range_y":    `[0.1, 5.0, 15]`,
	}
	res, err := handleParamHeatmap(context.Background(), req)
	if err != nil {
		t.Fatalf("handleParamHeatmap: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("PARAM_HEATMAP_OUT"); path != "" {
		_ = os.WriteFile(path, img, 0644)
	}
}

func TestRisk_Dashboard(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_risk"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, pureGBMModel()),
		"observable": "price",
		"volatility": `{"price": 0.5}`,
		"tspan":      "[0, 1]",
		"paths":      300,
		"steps":      150,
	}
	res, err := handleRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("handleRisk: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("RISK_OUT"); path != "" {
		_ = os.WriteFile(path, img, 0644)
	}
}

func TestSankey_CoffeeShop(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_sankey"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, coffeeShopModel()),
		"tspan": "[0, 15]",
		"title": "Coffee shop flow",
	}
	res, err := handleSankey(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSankey: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("SANKEY_OUT"); path != "" {
		_ = os.WriteFile(path, img, 0644)
	}
}
