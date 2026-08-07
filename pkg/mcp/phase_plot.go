package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_phase_plot runs an ODE and projects the trajectory into the
// (place_x, place_y) plane. The time axis disappears; what's visible is
// the geometry of the dynamics — attractors, limit cycles, manifolds.
// Useful for understanding two-state systems without thinking about
// where in the underlying Petri net they live.

func phasePlotTool() mcp.Tool {
	return mcp.NewTool("petri_phase_plot",
		mcp.WithDescription("Phase-space portrait: run an ODE, project trajectory into the (place_x, place_y) plane (no time axis). Reveals attractors, limit cycles, and geometric structure of two-state dynamics. Optionally overlays multiple trajectories from different initial conditions to map the basin structure."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("place_x",
			mcp.Required(),
			mcp.Description("Place ID for the x-axis"),
		),
		mcp.WithString("place_y",
			mcp.Required(),
			mcp.Description("Place ID for the y-axis"),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of rate constants (default 1.0 per transition)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span (default [0, 20])"),
		),
		mcp.WithString("initial_conditions",
			mcp.Description(`Optional JSON array of starting marking overrides, one per trajectory to draw. e.g. [{"place_x": 0.5}, {"place_x": 1.5}, {"place_x": 2.5}]. Default: a single trajectory from the model's initial marking`),
		),
		mcp.WithString("title",
			mcp.Description("Optional title shown above the plot"),
		),
	)
}

func handlePhasePlot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	placeX, err := request.RequireString("place_x")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing place_x: %v", err)), nil
	}
	placeY, err := request.RequireString("place_y")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing place_y: %v", err)), nil
	}
	placeSet := map[string]bool{}
	for _, p := range model.Places {
		placeSet[p.ID] = true
	}
	if !placeSet[placeX] {
		return mcp.NewToolResultError(fmt.Sprintf("place_x %q not found in model", placeX)), nil
	}
	if !placeSet[placeY] {
		return mcp.NewToolResultError(fmt.Sprintf("place_y %q not found in model", placeY)), nil
	}

	rates := map[string]float64{}
	for _, t := range model.Transitions {
		rates[t.ID] = 1.0
	}
	if s := request.GetString("rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid rates JSON: %v", err)), nil
		}
		for k, v := range user {
			rates[k] = v
		}
	}

	tspan := [2]float64{0, 20}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan JSON: %v", err)), nil
		}
		tspan = ts
	}

	baseInitial := map[string]float64{}
	for _, p := range model.Places {
		baseInitial[p.ID] = float64(p.Initial)
	}

	// Initial conditions: default single trajectory, or user array.
	var icList []map[string]float64
	if s := request.GetString("initial_conditions", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &icList); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid initial_conditions JSON: %v", err)), nil
		}
	}
	if len(icList) == 0 {
		icList = []map[string]float64{{}}
	}

	net := buildOdeNet(model)
	type traj struct {
		Xs, Ys []float64
		StartX float64
		StartY float64
	}
	trajectories := make([]traj, 0, len(icList))
	for _, ic := range icList {
		initial := map[string]float64{}
		for k, v := range baseInitial {
			initial[k] = v
		}
		for k, v := range ic {
			initial[k] = v
		}
		prob := solver.NewProblem(net, initial, tspan, rates)
		sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
		if sol == nil || len(sol.T) == 0 {
			continue
		}
		xs := sol.GetVariable(placeX)
		ys := sol.GetVariable(placeY)
		trajectories = append(trajectories, traj{
			Xs: xs, Ys: ys, StartX: xs[0], StartY: ys[0],
		})
	}
	if len(trajectories) == 0 {
		return mcp.NewToolResultError("solver returned no trajectories"), nil
	}

	title := request.GetString("title", "")
	if title == "" {
		title = fmt.Sprintf("Phase plot — %s vs %s", placeX, placeY)
	}

	const W, H = 720, 580
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	// Compute combined ranges across all trajectories.
	xmin, xmax := trajectories[0].Xs[0], trajectories[0].Xs[0]
	ymin, ymax := trajectories[0].Ys[0], trajectories[0].Ys[0]
	for _, t := range trajectories {
		for _, x := range t.Xs {
			if x < xmin {
				xmin = x
			}
			if x > xmax {
				xmax = x
			}
		}
		for _, y := range t.Ys {
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
		}
	}
	if xmax-xmin < 1e-9 {
		xmax = xmin + 1
	}
	if ymax-ymin < 1e-9 {
		ymax = ymin + 1
	}
	pad := 0.1
	xr, yr := xmax-xmin, ymax-ymin
	xmin -= xr * pad
	xmax += xr * pad
	ymin -= yr * pad
	ymax += yr * pad

	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, placeX, placeY, 0, 0, W, H)

	const (
		marginT = 40.0
		marginR = 140.0
		marginB = 50.0
		marginL = 70.0
	)
	plotW := float64(W) - marginL - marginR
	plotH := float64(H) - marginT - marginB
	sx := func(x float64) float64 {
		return marginL + (x-xmin)/(xmax-xmin)*plotW
	}
	sy := func(y float64) float64 {
		return marginT + plotH - (y-ymin)/(ymax-ymin)*plotH
	}

	for i, tr := range trajectories {
		var t01 float64
		if len(trajectories) > 1 {
			t01 = float64(i) / float64(len(trajectories)-1)
		}
		color := viridis(t01)
		dc.SetHexColor(color)
		dc.SetLineWidth(2)
		dc.MoveTo(sx(tr.Xs[0]), sy(tr.Ys[0]))
		for j := 1; j < len(tr.Xs); j++ {
			dc.LineTo(sx(tr.Xs[j]), sy(tr.Ys[j]))
		}
		dc.Stroke()
		// Start point: open circle.
		dc.SetHexColor(color)
		dc.DrawCircle(sx(tr.StartX), sy(tr.StartY), 5)
		dc.SetLineWidth(2)
		dc.Stroke()
		// End point: filled circle.
		dc.SetHexColor(color)
		dc.DrawCircle(sx(tr.Xs[len(tr.Xs)-1]), sy(tr.Ys[len(tr.Ys)-1]), 4)
		dc.Fill()
	}

	// Legend.
	if f, err := pngFace(false, 11); err == nil {
		legendX := marginL + plotW + 14
		legendY := marginT + 6
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("○ start", legendX, legendY+6, 0, 0.5)
		legendY += 16
		dc.DrawStringAnchored("● end", legendX, legendY+6, 0, 0.5)
		legendY += 22
		if len(trajectories) > 1 {
			dc.DrawStringAnchored(fmt.Sprintf("%d trajectories", len(trajectories)), legendX, legendY+6, 0, 0.5)
		} else {
			dc.DrawStringAnchored("1 trajectory", legendX, legendY+6, 0, 0.5)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	pngBytes := buf.Bytes()

	summary := map[string]any{
		"placeX":       placeX,
		"placeY":       placeY,
		"trajectories": len(trajectories),
		"xRange":       [2]float64{xmin, xmax},
		"yRange":       [2]float64{ymin, ymax},
	}
	text, _ := json.MarshalIndent(summary, "", "  ")
	return mcp.NewToolResultImage(string(withCaveats(text, model)), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
}
