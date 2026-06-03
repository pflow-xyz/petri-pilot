package mcp

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

func TestAutoLayout_ChainModel(t *testing.T) {
	// A 20-place chain with 19 transitions linking each pair. With no
	// explicit positions, the layout should be readable.
	model := &goflowmetamodel.Model{}
	for i := 0; i < 20; i++ {
		model.Places = append(model.Places, goflowmetamodel.Place{ID: fmt.Sprintf("p%02d", i)})
	}
	for i := 0; i < 19; i++ {
		tid := fmt.Sprintf("t%02d", i)
		model.Transitions = append(model.Transitions, goflowmetamodel.Transition{ID: tid})
		model.Arcs = append(model.Arcs,
			goflowmetamodel.Arc{From: fmt.Sprintf("p%02d", i), To: tid},
			goflowmetamodel.Arc{From: tid, To: fmt.Sprintf("p%02d", i+1)},
		)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_visualize"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, model),
		"title": "Auto-layout: 20-place chain",
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
	if path := os.Getenv("AUTO_LAYOUT_CHAIN_OUT"); path != "" {
		if err := os.WriteFile(path, img, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		t.Logf("wrote %d bytes to %s", len(img), path)
	}
}

func TestAutoLayout_Deterministic(t *testing.T) {
	// Same model → same positions twice.
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "a"}, {ID: "b"}, {ID: "c"},
		},
		Transitions: []goflowmetamodel.Transition{{ID: "t"}},
		Arcs: []goflowmetamodel.Arc{
			{From: "a", To: "t"}, {From: "t", To: "b"},
			{From: "b", To: "t"}, {From: "t", To: "c"},
		},
	}
	p1, t1 := computeAutoLayout(model)
	p2, t2 := computeAutoLayout(model)
	for k, v := range p1 {
		if p2[k] != v {
			t.Errorf("place %s differs: %v vs %v", k, v, p2[k])
		}
	}
	for k, v := range t1 {
		if t2[k] != v {
			t.Errorf("transition %s differs: %v vs %v", k, v, t2[k])
		}
	}
}

func TestAutoLayout_RespectsExplicitPositions(t *testing.T) {
	// If even one node has a position, the legacy row layout is used so
	// users who have hand-placed parts of their model see them at the
	// requested positions.
	model := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "a", X: 100, Y: 100},
			{ID: "b"},
		},
	}
	if shouldAutoLayout(model) {
		t.Errorf("shouldAutoLayout returned true with one positioned place")
	}
}
