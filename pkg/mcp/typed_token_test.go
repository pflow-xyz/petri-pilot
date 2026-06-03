package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

func TestTypedToken_VisualOutput(t *testing.T) {
	// Mixed model: a token place, a resource place (extra ring), and a
	// data place (rounded square + type sub-label).
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "pending", Initial: 3, X: 100, Y: 100},
			{ID: "worker", Initial: 2, Resource: true, X: 100, Y: 240},
			{ID: "result_count", Kind: goflowmetamodel.DataKind, Type: "int64", X: 360, Y: 170},
			{ID: "results", Kind: goflowmetamodel.DataKind, Type: "map[string]string", X: 560, Y: 170},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "process", X: 220, Y: 170},
			{ID: "store", X: 460, Y: 170},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "pending", To: "process"},
			{From: "worker", To: "process"},
			{From: "process", To: "result_count"},
			{From: "process", To: "worker"},
			{From: "result_count", To: "store"},
			{From: "store", To: "results"},
		},
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, model),
		"title": "Typed places (token / data) and resources",
	}
	res, err := handleVisualize(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVisualize: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d", len(img))
	}
	if path := os.Getenv("TYPED_TOKEN_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}
