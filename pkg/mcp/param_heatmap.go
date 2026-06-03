package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_param_heatmap runs a cross-product sweep over two rate parameters
// and renders the resulting observable values as a 2D heatmap. Answers
// "how does Y depend on (k_a, k_b)?" in one image. The Petri net structure
// never appears — just the parameter landscape.

func paramHeatmapTool() mcp.Tool {
	return mcp.NewTool("petri_param_heatmap",
		mcp.WithDescription("2D parameter sweep: vary two rate constants over a grid, run each combo to equilibrium, render the observable as a viridis heatmap. Answers 'how does APY depend on fee_tier × liquidity?' or 'which parameter regime gives me the equilibrium I want?'."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("param_x",
			mcp.Required(),
			mcp.Description("First transition ID to sweep"),
		),
		mcp.WithString("param_y",
			mcp.Required(),
			mcp.Description("Second transition ID to sweep"),
		),
		mcp.WithString("observable",
			mcp.Required(),
			mcp.Description("Place ID whose equilibrium value is the heatmap color"),
		),
		mcp.WithString("range_x",
			mcp.Required(),
			mcp.Description("JSON array [start, stop, n] for x sweep (e.g. [0.1, 5.0, 20])"),
		),
		mcp.WithString("range_y",
			mcp.Required(),
			mcp.Description("JSON array [start, stop, n] for y sweep"),
		),
		mcp.WithString("fixed_rates",
			mcp.Description("JSON object of other transition rates held constant"),
		),
		mcp.WithString("tspan",
			mcp.Description("Per-run integration span (default [0, 50])"),
		),
		mcp.WithBoolean("log_scale",
			mcp.Description("Sweep parameters in log space (better for ranges spanning orders of magnitude). Default false"),
		),
	)
}

type paramHeatmapResponse struct {
	ParamX     string      `json:"paramX"`
	ParamY     string      `json:"paramY"`
	Observable string      `json:"observable"`
	XValues    []float64   `json:"xValues"`
	YValues    []float64   `json:"yValues"`
	Grid       [][]float64 `json:"grid"`
	Min        float64     `json:"min"`
	Max        float64     `json:"max"`
}

func handleParamHeatmap(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	paramX, err := request.RequireString("param_x")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing param_x: %v", err)), nil
	}
	paramY, err := request.RequireString("param_y")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing param_y: %v", err)), nil
	}
	observable, err := request.RequireString("observable")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing observable: %v", err)), nil
	}
	rangeXStr, err := request.RequireString("range_x")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing range_x: %v", err)), nil
	}
	rangeYStr, err := request.RequireString("range_y")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing range_y: %v", err)), nil
	}

	transSet := map[string]bool{}
	for _, t := range model.Transitions {
		transSet[t.ID] = true
	}
	if !transSet[paramX] {
		return mcp.NewToolResultError(fmt.Sprintf("param_x %q not a transition", paramX)), nil
	}
	if !transSet[paramY] {
		return mcp.NewToolResultError(fmt.Sprintf("param_y %q not a transition", paramY)), nil
	}
	placeSet := map[string]bool{}
	for _, p := range model.Places {
		placeSet[p.ID] = true
	}
	if !placeSet[observable] {
		return mcp.NewToolResultError(fmt.Sprintf("observable %q not a place", observable)), nil
	}

	var rangeX, rangeY [3]float64
	if err := json.Unmarshal([]byte(rangeXStr), &rangeX); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid range_x: %v", err)), nil
	}
	if err := json.Unmarshal([]byte(rangeYStr), &rangeY); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid range_y: %v", err)), nil
	}
	nx, ny := int(rangeX[2]), int(rangeY[2])
	if nx < 2 || ny < 2 {
		return mcp.NewToolResultError("ranges need at least 2 steps each"), nil
	}
	// Cap at 50×50 = 2500 ODE runs ≈ 20s.
	if nx > 50 {
		nx = 50
	}
	if ny > 50 {
		ny = 50
	}

	logScale := request.GetBool("log_scale", false)
	var xs, ys []float64
	if logScale {
		xs = logspace(rangeX[0], rangeX[1], nx)
		ys = logspace(rangeY[0], rangeY[1], ny)
	} else {
		xs = linspace(rangeX[0], rangeX[1], nx)
		ys = linspace(rangeY[0], rangeY[1], ny)
	}

	baseRates := map[string]float64{}
	for _, t := range model.Transitions {
		baseRates[t.ID] = 1.0
	}
	if s := request.GetString("fixed_rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid fixed_rates: %v", err)), nil
		}
		for k, v := range user {
			baseRates[k] = v
		}
	}

	tspan := [2]float64{0, 50}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan: %v", err)), nil
		}
		tspan = ts
	}

	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}

	grid := make([][]float64, ny)
	for i := range grid {
		grid[i] = make([]float64, nx)
	}
	minV, maxV := math.Inf(1), math.Inf(-1)
	for iy, yv := range ys {
		for ix, xv := range xs {
			rates := make(map[string]float64, len(baseRates))
			for k, v := range baseRates {
				rates[k] = v
			}
			rates[paramX] = xv
			rates[paramY] = yv
			prob := solver.NewProblem(net, initial, tspan, rates)
			sol, _ := solver.SolveUntilEquilibrium(prob, solver.Tsit5(), solver.JSParityOptions(), solver.FastEquilibriumOptions())
			val := math.NaN()
			if sol != nil {
				final := sol.GetFinalState()
				if final != nil {
					val = final[observable]
				}
			}
			grid[iy][ix] = val
			if !math.IsNaN(val) {
				if val < minV {
					minV = val
				}
				if val > maxV {
					maxV = val
				}
			}
		}
	}

	resp := paramHeatmapResponse{
		ParamX:     paramX,
		ParamY:     paramY,
		Observable: observable,
		XValues:    xs,
		YValues:    ys,
		Grid:       grid,
		Min:        minV,
		Max:        maxV,
	}
	text, _ := json.MarshalIndent(resp, "", "  ")
	if pngBytes, perr := renderParamHeatmapPNG(resp, logScale); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

func renderParamHeatmapPNG(resp paramHeatmapResponse, logScale bool) ([]byte, error) {
	const W, H = 760, 540
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	ny := len(resp.YValues)
	nx := len(resp.XValues)
	if ny == 0 || nx == 0 {
		return nil, fmt.Errorf("empty grid")
	}

	const (
		marginT = 50.0
		marginL = 70.0
		marginB = 60.0
		marginR = 120.0
	)
	plotW := float64(W) - marginL - marginR
	plotH := float64(H) - marginT - marginB
	cellW := plotW / float64(nx)
	cellH := plotH / float64(ny)

	title := fmt.Sprintf("%s = f(%s, %s)", resp.Observable, resp.ParamX, resp.ParamY)
	if f, err := pngFace(true, 16); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(title, float64(W)/2, 22, 0.5, 0.5)
	}

	span := resp.Max - resp.Min
	if span < 1e-12 {
		span = 1
	}
	for iy := 0; iy < ny; iy++ {
		for ix := 0; ix < nx; ix++ {
			v := resp.Grid[iy][ix]
			if math.IsNaN(v) {
				continue
			}
			t := (v - resp.Min) / span
			x := marginL + float64(ix)*cellW
			// Invert iy so smaller y values are at the bottom.
			y := marginT + float64(ny-1-iy)*cellH
			dc.SetHexColor(viridis(t))
			dc.DrawRectangle(x, y, cellW, cellH)
			dc.Fill()
		}
	}

	// Axis labels.
	if f, err := pngFace(false, 12); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(resp.ParamX, marginL+plotW/2, float64(H)-15, 0.5, 0.5)
		dc.Push()
		dc.RotateAbout(-math.Pi/2, 18, marginT+plotH/2)
		dc.DrawStringAnchored(resp.ParamY, 18, marginT+plotH/2, 0.5, 0.5)
		dc.Pop()
	}

	// Axis ticks (start, mid, end values).
	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		nTicks := 5
		for i := 0; i <= nTicks; i++ {
			frac := float64(i) / float64(nTicks)
			xv := resp.XValues[0] + frac*(resp.XValues[nx-1]-resp.XValues[0])
			if logScale {
				lx := math.Log(resp.XValues[0]) + frac*(math.Log(resp.XValues[nx-1])-math.Log(resp.XValues[0]))
				xv = math.Exp(lx)
			}
			px := marginL + frac*plotW
			dc.DrawStringAnchored(formatTick(xv), px, marginT+plotH+14, 0.5, 0.5)
		}
		for i := 0; i <= nTicks; i++ {
			frac := float64(i) / float64(nTicks)
			yv := resp.YValues[0] + frac*(resp.YValues[ny-1]-resp.YValues[0])
			if logScale {
				ly := math.Log(resp.YValues[0]) + frac*(math.Log(resp.YValues[ny-1])-math.Log(resp.YValues[0]))
				yv = math.Exp(ly)
			}
			py := marginT + plotH - frac*plotH
			dc.DrawStringAnchored(formatTick(yv), marginL-6, py, 1.0, 0.5)
		}
	}

	// Color bar.
	legendX := marginL + plotW + 16
	legendBarW := 18.0
	legendBarH := plotH
	steps := 64
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		dc.SetHexColor(viridis(t))
		y := marginT + legendBarH - float64(i+1)*legendBarH/float64(steps)
		dc.DrawRectangle(legendX, y, legendBarW, legendBarH/float64(steps)+0.5)
		dc.Fill()
	}
	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(formatTick(resp.Max), legendX+legendBarW+4, marginT, 0, 0.5)
		dc.DrawStringAnchored(formatTick((resp.Max+resp.Min)/2), legendX+legendBarW+4, marginT+legendBarH/2, 0, 0.5)
		dc.DrawStringAnchored(formatTick(resp.Min), legendX+legendBarW+4, marginT+legendBarH, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
