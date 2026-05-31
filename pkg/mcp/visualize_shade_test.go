package mcp

import (
	"context"
	"encoding/base64"
	"os"
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
