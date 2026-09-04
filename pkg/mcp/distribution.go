package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"math/rand"
	"sort"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// petri_distribution runs N stochastic paths (SDE or SSA) and renders the
// distribution of an observable's value at the final time. Output is a
// histogram plus the percentiles a risk-minded user actually cares about
// (P5, P50, P95, mean, stdev). The Petri net is invisible — this is a
// projection into "what's my downside?" space.

func distributionTool() mcp.Tool {
	return mcp.NewTool("petri_distribution",
		mcp.WithDescription("Run N stochastic paths and visualize the distribution of an observable at the final time. Output: histogram + percentile band (P5 / P25 / P50 / P75 / P95) + mean and stdev. Answers questions like 'what's the probability the LP ends below $X' or 'what's the 5th-percentile worst case'. Mode selects SDE (continuous noise, for prices) or SSA (discrete events, for counts)."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("observable",
			mcp.Required(),
			mcp.Description("Place ID whose final-time distribution is plotted"),
		),
		mcp.WithString("mode",
			mcp.Description("'sde' (continuous-noise GBM, requires volatility) or 'ssa' (discrete Gillespie events). Default 'sde'"),
		),
		mcp.WithString("volatility",
			mcp.Description("Required for mode=sde. JSON object mapping place_id → sigma"),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of rate constants (default 1.0 per transition)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span (default [0, 1])"),
		),
		mcp.WithNumber("paths",
			mcp.Description("Number of paths (default 500, max 5000). Higher = tighter percentile estimates"),
		),
		mcp.WithNumber("bins",
			mcp.Description("Histogram bin count (default 30, max 100)"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed for reproducibility (default 42)"),
		),
	)
}

type distributionResponse struct {
	Observable  string             `json:"observable"`
	Mode        string             `json:"mode"`
	Paths       int                `json:"paths"`
	Tspan       [2]float64         `json:"tspan"`
	Mean        float64            `json:"mean"`
	Stdev       float64            `json:"stdev"`
	Percentiles map[string]float64 `json:"percentiles"`
	BinEdges    []float64          `json:"binEdges"`
	BinCounts   []int              `json:"binCounts"`
}

func handleDistribution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	observable, err := request.RequireString("observable")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing observable: %v", err)), nil
	}
	placeFound := false
	for _, p := range model.Places {
		if p.ID == observable {
			placeFound = true
			break
		}
	}
	if !placeFound {
		return mcp.NewToolResultError(fmt.Sprintf("observable %q not found in model", observable)), nil
	}

	mode := request.GetString("mode", "sde")
	if mode != "sde" && mode != "ssa" {
		return mcp.NewToolResultError(fmt.Sprintf("mode must be 'sde' or 'ssa', got %q", mode)), nil
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

	tspan := [2]float64{0, 1}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan JSON: %v", err)), nil
		}
		tspan = ts
	}

	paths := request.GetInt("paths", 500)
	if paths < 10 {
		paths = 10
	}
	if paths > 5000 {
		paths = 5000
	}
	bins := request.GetInt("bins", 30)
	if bins < 5 {
		bins = 5
	}
	if bins > 100 {
		bins = 100
	}
	seed := int64(request.GetInt("seed", 42))

	// Initial state from the model.
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}
	net := buildOdeNet(model)

	finals := make([]float64, 0, paths)
	rng := rand.New(rand.NewSource(seed))

	switch mode {
	case "sde":
		volStr, err := request.RequireString("volatility")
		if err != nil {
			return mcp.NewToolResultError("mode=sde requires 'volatility'"), nil
		}
		var volatility map[string]float64
		if err := json.Unmarshal([]byte(volStr), &volatility); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid volatility JSON: %v", err)), nil
		}
		// Ensure each volatility place exists.
		for k := range volatility {
			if _, ok := initial[k]; !ok {
				return mcp.NewToolResultError(fmt.Sprintf("volatility place %q not found in model", k)), nil
			}
		}

		// Reuse SDE path runner. Need volatilePlaces order + Cholesky factor
		// (identity for uncorrelated runs, which is the default here).
		volatilePlaces := make([]string, 0, len(volatility))
		for k := range volatility {
			volatilePlaces = append(volatilePlaces, k)
		}
		sort.Strings(volatilePlaces)
		nv := len(volatilePlaces)
		chol := make([][]float64, nv)
		for i := range chol {
			chol[i] = make([]float64, nv)
			chol[i][i] = 1
		}
		prob := solver.NewProblem(net, initial, tspan, rates)
		steps := 200
		sampleStep := []int{steps} // one sample, at the end
		for p := 0; p < paths; p++ {
			sub := rng.Int63()
			out := runSDEPath(prob, initial, volatility, volatilePlaces, chol, tspan, steps, sampleStep, sub)
			finals = append(finals, out[observable][0])
		}
	case "ssa":
		for p := 0; p < paths; p++ {
			sub := rng.Int63()
			out, serr := sim.Simulate(model, nil, sim.Options{
				Horizon: tspan[1] - tspan[0],
				Samples: 2, // only the final time matters here
				Rates:   rates,
				Seed:    sub,
			})
			if serr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("simulate: %v", serr)), nil
			}
			finals = append(finals, out.Final[observable])
		}
	}

	// Stats.
	mean := 0.0
	for _, v := range finals {
		mean += v
	}
	mean /= float64(len(finals))
	variance := 0.0
	for _, v := range finals {
		d := v - mean
		variance += d * d
	}
	stdev := math.Sqrt(variance / float64(len(finals)-1))
	sorted := append([]float64(nil), finals...)
	sort.Float64s(sorted)
	pct := func(q float64) float64 {
		idx := q * float64(len(sorted)-1)
		lo, hi := int(math.Floor(idx)), int(math.Ceil(idx))
		if lo == hi {
			return sorted[lo]
		}
		frac := idx - float64(lo)
		return sorted[lo] + frac*(sorted[hi]-sorted[lo])
	}
	percentiles := map[string]float64{
		"P5":  pct(0.05),
		"P25": pct(0.25),
		"P50": pct(0.50),
		"P75": pct(0.75),
		"P95": pct(0.95),
		"min": sorted[0],
		"max": sorted[len(sorted)-1],
	}

	// Histogram.
	binEdges := make([]float64, bins+1)
	binCounts := make([]int, bins)
	minV, maxV := sorted[0], sorted[len(sorted)-1]
	if maxV-minV < 1e-9 {
		maxV = minV + 1
	}
	for i := 0; i <= bins; i++ {
		binEdges[i] = minV + (maxV-minV)*float64(i)/float64(bins)
	}
	for _, v := range finals {
		idx := int((v - minV) / (maxV - minV) * float64(bins))
		if idx < 0 {
			idx = 0
		}
		if idx >= bins {
			idx = bins - 1
		}
		binCounts[idx]++
	}

	resp := distributionResponse{
		Observable:  observable,
		Mode:        mode,
		Paths:       paths,
		Tspan:       tspan,
		Mean:        mean,
		Stdev:       stdev,
		Percentiles: percentiles,
		BinEdges:    binEdges,
		BinCounts:   binCounts,
	}

	text, _ := json.MarshalIndent(resp, "", "  ")
	if pngBytes, perr := renderDistributionPNG(resp); perr == nil {
		return mcp.NewToolResultImage(string(withCaveats(text, model)), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(withCaveats(text, model))), nil
}

func renderDistributionPNG(resp distributionResponse) ([]byte, error) {
	const W, H = 760, 460
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if len(resp.BinCounts) == 0 {
		return nil, fmt.Errorf("empty histogram")
	}
	xmin := resp.BinEdges[0]
	xmax := resp.BinEdges[len(resp.BinEdges)-1]
	maxCount := 0
	for _, c := range resp.BinCounts {
		if c > maxCount {
			maxCount = c
		}
	}
	ymin, ymax := 0.0, float64(maxCount)*1.1
	if ymax < 1 {
		ymax = 1
	}
	title := fmt.Sprintf("Distribution of %s at t=%.3g (mode=%s, n=%d)", resp.Observable, resp.Tspan[1], resp.Mode, resp.Paths)
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, resp.Observable, "Count", 0, 0, W, H)

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

	// Bars.
	for i, c := range resp.BinCounts {
		lo := resp.BinEdges[i]
		hi := resp.BinEdges[i+1]
		x := sx(lo)
		w := sx(hi) - x
		y := sy(float64(c))
		h := sy(0) - y
		dc.SetHexColor("#1976d2")
		dc.DrawRectangle(x, y, w, h)
		dc.Fill()
		dc.SetHexColor("#ffffff")
		dc.SetLineWidth(0.5)
		dc.DrawRectangle(x, y, w, h)
		dc.Stroke()
	}

	// Percentile lines.
	pcts := []struct {
		name  string
		value float64
		color string
		dash  []float64
	}{
		{"P5", resp.Percentiles["P5"], "#d32f2f", []float64{6, 4}},
		{"P50", resp.Percentiles["P50"], "#000000", nil},
		{"P95", resp.Percentiles["P95"], "#43a047", []float64{6, 4}},
	}
	for _, p := range pcts {
		if p.value < xmin || p.value > xmax {
			continue
		}
		dc.SetHexColor(p.color)
		dc.SetLineWidth(1.5)
		if len(p.dash) > 0 {
			dc.SetDash(p.dash...)
		}
		dc.DrawLine(sx(p.value), sy(0), sx(p.value), sy(ymax))
		dc.Stroke()
		dc.SetDash()
	}

	// Legend with stats.
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(fmt.Sprintf("mean  %.4g", resp.Mean), legendX, legendY+6, 0, 0.5)
		legendY += 16
		dc.DrawStringAnchored(fmt.Sprintf("stdev %.4g", resp.Stdev), legendX, legendY+6, 0, 0.5)
		legendY += 16
		dc.DrawStringAnchored(fmt.Sprintf("min   %.4g", resp.Percentiles["min"]), legendX, legendY+6, 0, 0.5)
		legendY += 16
		dc.DrawStringAnchored(fmt.Sprintf("max   %.4g", resp.Percentiles["max"]), legendX, legendY+6, 0, 0.5)
		legendY += 22

		dc.SetHexColor("#d32f2f")
		dc.SetLineWidth(1.5)
		dc.SetDash(4, 3)
		dc.DrawLine(legendX, legendY+6, legendX+18, legendY+6)
		dc.Stroke()
		dc.SetDash()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(fmt.Sprintf("P5  %.4g", resp.Percentiles["P5"]), legendX+22, legendY+6, 0, 0.5)
		legendY += 16

		dc.SetHexColor("#000000")
		dc.SetLineWidth(1.5)
		dc.DrawLine(legendX, legendY+6, legendX+18, legendY+6)
		dc.Stroke()
		dc.DrawStringAnchored(fmt.Sprintf("P50 %.4g", resp.Percentiles["P50"]), legendX+22, legendY+6, 0, 0.5)
		legendY += 16

		dc.SetHexColor("#43a047")
		dc.SetLineWidth(1.5)
		dc.SetDash(4, 3)
		dc.DrawLine(legendX, legendY+6, legendX+18, legendY+6)
		dc.Stroke()
		dc.SetDash()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(fmt.Sprintf("P95 %.4g", resp.Percentiles["P95"]), legendX+22, legendY+6, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
