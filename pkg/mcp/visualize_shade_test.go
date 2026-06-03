package mcp

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func extractImageBytes(t *testing.T, res *mcp.CallToolResult) []byte {
	t.Helper()
	for _, c := range res.Content {
		if ic, ok := mcp.AsImageContent(c); ok {
			b, err := base64.StdEncoding.DecodeString(ic.Data)
			if err != nil {
				t.Fatalf("decode image: %v", err)
			}
			return b
		}
	}
	return nil
}

func TestVisualize_SensitivityShade(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, coffeeShopModel()),
		"shade": "sensitivity",
		"title": "Coffee shop — sensitivity",
	}
	res, err := handleVisualize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVisualize: %v", err)
	}
	if res.IsError {
		t.Fatalf("result is error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 200 {
		t.Fatalf("image too small: %d bytes", len(img))
	}
	if path := os.Getenv("VISUALIZE_SENSITIVITY_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestVisualize_MarkingShade(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"shade":   "marking",
		"marking": `{"order_pending": 0, "barista_idle": 1, "brewing": 0, "ready": 0, "delivered": 2}`,
		"title":   "Coffee shop — final state",
	}
	res, err := handleVisualize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVisualize: %v", err)
	}
	if res.IsError {
		t.Fatalf("result is error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 200 {
		t.Fatalf("image too small: %d bytes", len(img))
	}
	if path := os.Getenv("VISUALIZE_MARKING_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestVisualize_SVGContainsShading(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"shade":   "marking",
		"marking": `{"order_pending": 0, "barista_idle": 1, "brewing": 0, "ready": 0, "delivered": 2}`,
	}
	res, err := handleVisualize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVisualize: %v", err)
	}
	svg := textBlock(t, res)
	// SVG should contain inline styles, not the default .place CSS class
	// reliance — the per-element fills should be present.
	if !strings.Contains(svg, "fill:#1976d2") {
		t.Errorf("expected delivered place to render as saturated blue (fill:#1976d2) in SVG; got:\n%s", svg[:min(500, len(svg))])
	}
	if !strings.Contains(svg, "fill:#ffffff") {
		t.Errorf("expected zero-valued places to render as white (fill:#ffffff) in SVG")
	}
}

func TestVisualize_SVGUnstyledByDefault(t *testing.T) {
	// Without shade, the SVG should still render. We don't require any
	// specific style structure here — just that the diagram is non-empty
	// and includes the place IDs.
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, coffeeShopModel()),
	}
	res, err := handleVisualize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVisualize: %v", err)
	}
	svg := textBlock(t, res)
	for _, id := range []string{"order_pending", "barista_idle", "brewing", "ready", "delivered"} {
		if !strings.Contains(svg, id) {
			t.Errorf("SVG missing place id %q", id)
		}
	}
}

func TestVisualize_MarkingShadeRequiresMarking(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, coffeeShopModel()),
		"shade": "marking",
	}
	res, err := handleVisualize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVisualize: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error when shade=marking without marking param")
	}
}
