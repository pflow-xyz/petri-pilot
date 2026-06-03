package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

func coffeeShopWithRefund() *goflowmetamodel.Model {
	// Coffee shop with a refund path added: a new "refunded" place plus a
	// "cancel" transition that consumes order_pending and produces a refund.
	m := coffeeShopModel()
	m.Places = append(m.Places,
		goflowmetamodel.Place{ID: "refunded", X: 260, Y: 50},
	)
	m.Transitions = append(m.Transitions,
		goflowmetamodel.Transition{ID: "cancel", X: 170, Y: 50},
	)
	m.Arcs = append(m.Arcs,
		goflowmetamodel.Arc{From: "order_pending", To: "cancel", Weight: 1},
		goflowmetamodel.Arc{From: "cancel", To: "refunded", Weight: 1},
	)
	return m
}

func TestDiff_VisualOutput(t *testing.T) {
	a := coffeeShopModel()
	b := coffeeShopWithRefund()

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_diff"
	req.Params.Arguments = map[string]any{
		"model_a": mustJSON(t, a),
		"model_b": mustJSON(t, b),
	}
	res, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d bytes", len(img))
	}
	if path := os.Getenv("DIFF_VISUAL_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestDiff_VisualNoChanges(t *testing.T) {
	// Identical models — image should still render but with "no changes"
	// in the legend.
	a := coffeeShopModel()
	b := coffeeShopModel()
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_diff"
	req.Params.Arguments = map[string]any{
		"model_a": mustJSON(t, a),
		"model_b": mustJSON(t, b),
	}
	res, err := handleDiff(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDiff: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	if img := extractImageBytes(t, res); len(img) == 0 {
		t.Fatalf("expected image even for no-changes diff")
	}
}
