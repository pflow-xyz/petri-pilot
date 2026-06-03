package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestExtend_VisualOutput(t *testing.T) {
	model := coffeeShopModel()
	ops := `[
		{"op":"add_place","id":"refunded","x":260,"y":50},
		{"op":"add_transition","id":"cancel","x":170,"y":50},
		{"op":"add_arc","from":"order_pending","to":"cancel"},
		{"op":"add_arc","from":"cancel","to":"refunded"}
	]`
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_extend"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, model),
		"operations": ops,
	}
	res, err := handleExtend(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExtend: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	img := extractImageBytes(t, res)
	if len(img) < 1000 {
		t.Fatalf("image too small: %d bytes", len(img))
	}
	if path := os.Getenv("EXTEND_VISUAL_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}
