package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"sort"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_ode_sweep runs N ODE trajectories — one per rate value — and overlays
// them on a single plot. Where petri_rate_scan reports equilibrium values
// only, this one returns the full transient so regime changes (peaks,
// crossover points, time-to-equilibrium) are visible.

func odeSweepTool() mcp.Tool {
	return mcp.NewTool("petri_ode_sweep",
		mcp.WithDescription("Run multiple ODE trajectories at different rates and overlay them on one plot. Useful for seeing how dynamics change with a parameter — regime shifts, peak shifts, time-to-equilibrium. Each rate value gets its own viridis-colored line."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("transition",
			mcp.Required(),
			mcp.Description("Transition ID whose rate is being swept"),
		),
		mcp.WithString("observable",
			mcp.Required(),
			mcp.Description("Place ID whose trajectory is shown (one observable per call to keep the plot readable)"),
		),
		mcp.WithString("values",
			mcp.Description("JSON array of rate values to sweep. Alternative to range"),
		),
		mcp.WithString("range",
			mcp.Description("JSON array [start, stop, n] generating n equally-spaced rates. Alternative to values"),
		),
		mcp.WithString("fixed_rates",
			mcp.Description("JSON object of other transition rates (default 1.0)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span (default [0, 10])"),
		),
		mcp.WithNumber("samples",
			mcp.Description("Max samples per trajectory after downsampling (default 200)"),
		),
	)
}

type odeSweepResponse struct {
	Transition   string            `json:"transition"`
	Observable   string            `json:"observable"`
	Values       []float64         `json:"values"`
	Trajectories []sweepTrajectory `json:"trajectories"`
}

type sweepTrajectory struct {
	Rate float64   `json:"rate"`
	T    []float64 `json:"t"`
	V    []float64 `json:"v"`
}

func handleOdeSweep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	transitionID, err := request.RequireString("transition")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing transition parameter: %v", err)), nil
	}
	observable, err := request.RequireString("observable")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing observable parameter: %v", err)), nil
	}

	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	transitionFound, observableFound := false, false
	for _, t := range model.Transitions {
		if t.ID == transitionID {
			transitionFound = true
			break
		}
	}
	for _, p := range model.Places {
		if p.ID == observable {
			observableFound = true
			break
		}
	}
	if !transitionFound {
		return mcp.NewToolResultError(fmt.Sprintf("transition %q not found in model", transitionID)), nil
	}
	if !observableFound {
		return mcp.NewToolResultError(fmt.Sprintf("observable %q is not a place in the model", observable)), nil
	}

	var values []float64
	if s := request.GetString("values", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &values); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid values JSON: %v", err)), nil
		}
	} else if s := request.GetString("range", ""); s != "" {
		var rng [3]float64
		if err := json.Unmarshal([]byte(s), &rng); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid range JSON: %v", err)), nil
		}
		n := int(rng[2])
		if n < 2 {
			return mcp.NewToolResultError("range requires at least 2 steps"), nil
		}
		values = linspace(rng[0], rng[1], n)
	} else {
		return mcp.NewToolResultError("either 'values' or 'range' is required"), nil
	}
	sort.Float64s(values)

	baseRates := map[string]float64{}
	for _, t := range model.Transitions {
		baseRates[t.ID] = 1.0
	}
	if s := request.GetString("fixed_rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid fixed_rates JSON: %v", err)), nil
		}
		for k, v := range user {
			baseRates[k] = v
		}
	}

	tspan := [2]float64{0, 10}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan JSON: %v", err)), nil
		}
		if ts[1] <= ts[0] {
			return mcp.NewToolResultError("tspan: t1 must exceed t0"), nil
		}
		tspan = ts
	}

	maxSamples := request.GetInt("samples", 200)
	if maxSamples < 2 {
		maxSamples = 2
	}

	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}

	resp := odeSweepResponse{
		Transition: transitionID,
		Observable: observable,
		Values:     values,
	}

	for _, rate := range values {
		rates := make(map[string]float64, len(baseRates))
		for k, v := range baseRates {
			rates[k] = v
		}
		rates[transitionID] = rate

		prob := solver.NewProblem(net, initial, tspan, rates)
		sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
		if sol == nil || len(sol.T) == 0 {
			continue
		}
		ys := sol.GetVariable(observable)
		n := len(sol.T)
		stride := 1
		if maxSamples > 0 && n > maxSamples {
			stride = (n + maxSamples - 1) / maxSamples
		}
		traj := sweepTrajectory{Rate: rate}
		for i := 0; i < n; i += stride {
			traj.T = append(traj.T, sol.T[i])
			traj.V = append(traj.V, ys[i])
		}
		if n > 0 && (n-1)%stride != 0 {
			traj.T = append(traj.T, sol.T[n-1])
			traj.V = append(traj.V, ys[n-1])
		}
		resp.Trajectories = append(resp.Trajectories, traj)
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	if pngBytes, perr := renderOdeSweepPNG(resp, observable, transitionID); perr == nil {
		return mcp.NewToolResultImage(string(withCaveats(text, model)), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(withCaveats(text, model))), nil
}

// renderOdeSweepPNG draws all trajectories on one plot, colored by rate via
// the viridis colormap. Each trajectory keeps its own (t, v) array — the
// time grids may differ after downsampling — so we draw the axes/legend
// manually rather than packing per-series xs into drawXYPlot.
func renderOdeSweepPNG(resp odeSweepResponse, observable, transitionID string) ([]byte, error) {
	const W, H = 720, 420
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if len(resp.Trajectories) == 0 {
		return nil, fmt.Errorf("no trajectories to render")
	}

	xmin := resp.Trajectories[0].T[0]
	xmax := xmin
	ymin := resp.Trajectories[0].V[0]
	ymax := ymin
	for _, traj := range resp.Trajectories {
		for _, x := range traj.T {
			if x < xmin {
				xmin = x
			}
			if x > xmax {
				xmax = x
			}
		}
		for _, y := range traj.V {
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
		}
	}
	if xmax-xmin < 1e-12 {
		xmax = xmin + 1
	}
	yrange := ymax - ymin
	if yrange < 1e-9 {
		ymax = ymin + 1
		yrange = 1
	}
	ymin -= yrange * 0.1
	ymax += yrange * 0.1

	colors := make([]string, len(resp.Trajectories))
	labels := make([]string, len(resp.Trajectories))
	for i, traj := range resp.Trajectories {
		var t float64
		if len(resp.Values) > 1 {
			t = float64(i) / float64(len(resp.Values)-1)
		}
		colors[i] = viridis(t)
		labels[i] = formatRateLabel(traj.Rate)
	}

	title := fmt.Sprintf("ODE sweep over %q — %s", transitionID, observable)
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, "Time", observable, 0, 0, W, H)

	// Stroke trajectories using the frame's axis mapping.
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
	dc.SetLineWidth(2)
	for i, traj := range resp.Trajectories {
		if len(traj.T) == 0 {
			continue
		}
		dc.SetHexColor(colors[i])
		dc.MoveTo(sx(traj.T[0]), sy(traj.V[0]))
		for j := 1; j < len(traj.T); j++ {
			dc.LineTo(sx(traj.T[j]), sy(traj.V[j]))
		}
		dc.Stroke()
	}

	// Legend on the right.
	right := marginL + plotW
	top := marginT
	legendX := right + 14
	legendY := top + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		for i, label := range labels {
			dc.SetHexColor(colors[i])
			dc.SetLineWidth(2)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(label, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatRateLabel(rate float64) string {
	if rate == float64(int(rate)) && rate < 100 {
		return fmt.Sprintf("k=%.0f", rate)
	}
	return fmt.Sprintf("k=%.2g", rate)
}
