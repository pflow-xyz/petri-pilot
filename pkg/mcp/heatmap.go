package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// petri_heatmap exposes the heatmap renderer. Useful for high-place models
// where the standard net diagram is too dense — e.g. TTT (33 places),
// zk-ode topologies, board states.

func heatmapTool() mcp.Tool {
	return mcp.NewTool("petri_heatmap",
		mcp.WithDescription("Render the model's marking as a 2D colored grid heatmap (viridis colormap). Useful for high-place models (TTT boards, zk-ode topologies, grid-structured nets). Each cell shows place ID + value. Returns inline PNG."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("marking",
			mcp.Description("Optional JSON object {place_id: value} overriding the initial marking"),
		),
		mcp.WithNumber("rows",
			mcp.Description("Grid rows (0/omit = auto-square)"),
		),
		mcp.WithNumber("cols",
			mcp.Description("Grid columns (0/omit = auto-square)"),
		),
		mcp.WithBoolean("labels",
			mcp.Description("Show place IDs and values in each cell (default true)"),
		),
		mcp.WithString("title",
			mcp.Description("Optional title shown above the heatmap"),
		),
	)
}

func handleHeatmap(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	opts := &HeatmapOpts{
		Title:  request.GetString("title", ""),
		Rows:   request.GetInt("rows", 0),
		Cols:   request.GetInt("cols", 0),
		Labels: request.GetBool("labels", true),
	}

	if s := request.GetString("marking", ""); s != "" {
		var m map[string]float64
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid marking JSON: %v", err)), nil
		}
		opts.Marking = m
	}

	pngBytes, err := renderHeatmapPNG(model, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("render failed: %v", err)), nil
	}

	summary := map[string]any{
		"places":  len(model.Places),
		"marking": resolvedMarking(model, opts.Marking),
	}
	text, _ := json.MarshalIndent(summary, "", "  ")

	return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
}

// resolvedMarking returns the marking actually used for rendering: the
// override values when present, falling back to Place.Initial.
func resolvedMarking(model *goflowmetamodel.Model, override map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(model.Places))
	for _, p := range model.Places {
		out[p.ID] = float64(p.Initial)
		if override != nil {
			if v, ok := override[p.ID]; ok {
				out[p.ID] = v
			}
		}
	}
	return out
}
