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
)

// petri_risk runs N SDE paths and computes the summary statistics a
// risk-minded user wants in one place: mean, stdev, P5/P50/P95, max
// drawdown (worst peak-to-trough across all paths), CVaR (mean of bottom
// 5%), and a "downside histogram" of the drawdowns. Composite layout —
// stats card on the left, drawdown histogram on the right.
//
// No Petri net topology appears. This is the "what's my downside?" view.

func riskTool() mcp.Tool {
	return mcp.NewTool("petri_risk",
		mcp.WithDescription("Risk dashboard for an observable under SDE simulation. Runs N paths, computes mean / stdev / P5 / P50 / P95 of final values, plus max drawdown and CVaR (expected shortfall in the worst 5% of paths). Output: composite card with stats panel and drawdown distribution histogram. Answers 'what's the worst-case loss?' and 'how bad does it usually get?'."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("observable",
			mcp.Required(),
			mcp.Description("Place ID to monitor as the asset/portfolio value"),
		),
		mcp.WithString("volatility",
			mcp.Required(),
			mcp.Description("JSON object {place_id: sigma} for SDE noise"),
		),
		mcp.WithString("correlations",
			mcp.Description("Optional pairwise correlation dict (same format as petri_sde)"),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of rate constants (default 1.0)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span (default [0, 1])"),
		),
		mcp.WithNumber("paths",
			mcp.Description("Number of SDE paths (default 500, max 5000)"),
		),
		mcp.WithNumber("steps",
			mcp.Description("Euler-Maruyama step count per path (default 200)"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed (default 42)"),
		),
	)
}

type riskResponse struct {
	Observable    string             `json:"observable"`
	Paths         int                `json:"paths"`
	Tspan         [2]float64         `json:"tspan"`
	InitialValue  float64            `json:"initialValue"`
	Final         map[string]float64 `json:"final"`       // mean/stdev/percentiles of final value
	Returns       map[string]float64 `json:"returns"`     // same for log returns (final / initial)
	MaxDrawdown   map[string]float64 `json:"maxDrawdown"` // mean / median / worst peak-to-trough per-path
	CVaR95        float64            `json:"cvar95"`      // mean of worst 5% final values
	WorstFinal    float64            `json:"worstFinal"`
	BestFinal     float64            `json:"bestFinal"`
	DrawdownBins  []float64          `json:"drawdownBins"`
	DrawdownCount []int              `json:"drawdownCount"`
}

func handleRisk(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model: %v", err)), nil
	}
	model := parsed.Model

	observable, err := request.RequireString("observable")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing observable: %v", err)), nil
	}
	placeSet := map[string]bool{}
	initial := map[string]float64{}
	for _, p := range model.Places {
		placeSet[p.ID] = true
		initial[p.ID] = float64(p.Initial)
	}
	if !placeSet[observable] {
		return mcp.NewToolResultError(fmt.Sprintf("observable %q not a place", observable)), nil
	}

	volStr, err := request.RequireString("volatility")
	if err != nil {
		return mcp.NewToolResultError("volatility required"), nil
	}
	var volatility map[string]float64
	if err := json.Unmarshal([]byte(volStr), &volatility); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid volatility: %v", err)), nil
	}
	for k := range volatility {
		if !placeSet[k] {
			return mcp.NewToolResultError(fmt.Sprintf("volatility place %q not in model", k)), nil
		}
	}

	rates := map[string]float64{}
	for _, t := range model.Transitions {
		rates[t.ID] = 1.0
	}
	if s := request.GetString("rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid rates: %v", err)), nil
		}
		for k, v := range user {
			rates[k] = v
		}
	}

	tspan := [2]float64{0, 1}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan: %v", err)), nil
		}
		tspan = ts
	}

	paths := request.GetInt("paths", 500)
	if paths < 50 {
		paths = 50
	}
	if paths > 5000 {
		paths = 5000
	}
	steps := request.GetInt("steps", 200)
	if steps < 20 {
		steps = 20
	}
	seed := int64(request.GetInt("seed", 42))

	// Build Cholesky factor for optional correlations.
	volatilePlaces := make([]string, 0, len(volatility))
	for k := range volatility {
		volatilePlaces = append(volatilePlaces, k)
	}
	sort.Strings(volatilePlaces)
	nv := len(volatilePlaces)
	volIdx := map[string]int{}
	for i, k := range volatilePlaces {
		volIdx[k] = i
	}
	corr := make([][]float64, nv)
	for i := range corr {
		corr[i] = make([]float64, nv)
		corr[i][i] = 1
	}
	if s := request.GetString("correlations", ""); s != "" {
		var pairs map[string]float64
		if err := json.Unmarshal([]byte(s), &pairs); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid correlations: %v", err)), nil
		}
		for key, rho := range pairs {
			a, b, ok := splitCorrKey(key)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("bad correlation key %q", key)), nil
			}
			ia, ok1 := volIdx[a]
			ib, ok2 := volIdx[b]
			if !ok1 || !ok2 {
				return mcp.NewToolResultError(fmt.Sprintf("correlation %q references place not in volatility map", key)), nil
			}
			if rho < -1 || rho > 1 {
				return mcp.NewToolResultError(fmt.Sprintf("rho %v out of [-1, 1]", rho)), nil
			}
			corr[ia][ib] = rho
			corr[ib][ia] = rho
		}
	}
	chol, err := cholesky(corr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("correlation matrix not PSD: %v", err)), nil
	}

	net := buildOdeNet(model)
	prob := solver.NewProblem(net, initial, tspan, rates)
	initialVal := initial[observable]

	// Sample every step so we can compute per-path drawdown.
	sampleStep := make([]int, steps+1)
	for i := range sampleStep {
		sampleStep[i] = i
	}

	rng := rand.New(rand.NewSource(seed))
	finals := make([]float64, paths)
	maxDDs := make([]float64, paths)
	for p := 0; p < paths; p++ {
		sub := rng.Int63()
		out := runSDEPath(prob, initial, volatility, volatilePlaces, chol, tspan, steps, sampleStep, sub)
		traj := out[observable]
		finals[p] = traj[len(traj)-1]
		// Max drawdown for this path: 1 − (lowest after peak) / (peak so far).
		peak := traj[0]
		dd := 0.0
		for _, v := range traj {
			if v > peak {
				peak = v
			}
			if peak > 0 {
				cur := 1 - v/peak
				if cur > dd {
					dd = cur
				}
			}
		}
		maxDDs[p] = dd
	}

	sortedFinals := append([]float64(nil), finals...)
	sort.Float64s(sortedFinals)
	pct := func(arr []float64, q float64) float64 {
		idx := q * float64(len(arr)-1)
		lo, hi := int(math.Floor(idx)), int(math.Ceil(idx))
		if lo == hi {
			return arr[lo]
		}
		frac := idx - float64(lo)
		return arr[lo] + frac*(arr[hi]-arr[lo])
	}
	stats := func(arr []float64) map[string]float64 {
		sorted := append([]float64(nil), arr...)
		sort.Float64s(sorted)
		mean := 0.0
		for _, v := range arr {
			mean += v
		}
		mean /= float64(len(arr))
		var2 := 0.0
		for _, v := range arr {
			d := v - mean
			var2 += d * d
		}
		stdev := math.Sqrt(var2 / float64(len(arr)-1))
		return map[string]float64{
			"mean":  mean,
			"stdev": stdev,
			"P5":    pct(sorted, 0.05),
			"P50":   pct(sorted, 0.50),
			"P95":   pct(sorted, 0.95),
		}
	}

	final := stats(finals)
	maxDD := stats(maxDDs)
	maxDD["worst"] = func() float64 {
		w := 0.0
		for _, v := range maxDDs {
			if v > w {
				w = v
			}
		}
		return w
	}()

	// Log returns: log(final / initial).
	rets := make([]float64, len(finals))
	for i, v := range finals {
		if initialVal > 0 && v > 0 {
			rets[i] = math.Log(v / initialVal)
		}
	}
	returns := stats(rets)

	// CVaR_95: mean of worst 5%.
	nBottom := int(math.Max(1, math.Round(0.05*float64(len(sortedFinals)))))
	cvar := 0.0
	for i := 0; i < nBottom; i++ {
		cvar += sortedFinals[i]
	}
	cvar /= float64(nBottom)

	// Drawdown histogram (% units).
	const bins = 25
	ddBins := make([]float64, bins+1)
	ddCounts := make([]int, bins)
	ddMax := 0.0
	for _, d := range maxDDs {
		if d > ddMax {
			ddMax = d
		}
	}
	if ddMax < 0.01 {
		ddMax = 0.01
	}
	for i := 0; i <= bins; i++ {
		ddBins[i] = ddMax * float64(i) / float64(bins)
	}
	for _, d := range maxDDs {
		idx := int(d / ddMax * float64(bins))
		if idx >= bins {
			idx = bins - 1
		}
		ddCounts[idx]++
	}

	resp := riskResponse{
		Observable:    observable,
		Paths:         paths,
		Tspan:         tspan,
		InitialValue:  initialVal,
		Final:         final,
		Returns:       returns,
		MaxDrawdown:   maxDD,
		CVaR95:        cvar,
		WorstFinal:    sortedFinals[0],
		BestFinal:     sortedFinals[len(sortedFinals)-1],
		DrawdownBins:  ddBins,
		DrawdownCount: ddCounts,
	}
	text, _ := json.MarshalIndent(resp, "", "  ")
	if pngBytes, perr := renderRiskPNG(resp); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

func renderRiskPNG(resp riskResponse) ([]byte, error) {
	const (
		W      = 920
		H      = 460
		gap    = 16.0
		titleH = 36.0
		statsW = 320.0
	)
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	title := fmt.Sprintf("Risk dashboard — %s (n=%d paths, T=%.3g)", resp.Observable, resp.Paths, resp.Tspan[1])
	drawTitle(dc, title, float64(W)/2, titleH/2+4)

	// Stats panel (left).
	dc.SetHexColor("#f7f7f9")
	dc.DrawRoundedRectangle(20, titleH+10, statsW, float64(H)-titleH-30, 8)
	dc.Fill()
	dc.SetHexColor("#dddddd")
	dc.SetLineWidth(1)
	dc.DrawRoundedRectangle(20, titleH+10, statsW, float64(H)-titleH-30, 8)
	dc.Stroke()

	if f, err := pngFace(true, 13); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Final value", 36, titleH+34, 0, 0.5)
		dc.DrawStringAnchored("Log return", 36, titleH+118, 0, 0.5)
		dc.DrawStringAnchored("Max drawdown", 36, titleH+202, 0, 0.5)
		dc.DrawStringAnchored("Tail risk", 36, titleH+286, 0, 0.5)
	}
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#333333")
		// Final value block.
		y := titleH + 54.0
		dc.DrawStringAnchored(fmt.Sprintf("initial   %.4g", resp.InitialValue), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("mean      %.4g", resp.Final["mean"]), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("stdev     %.4g", resp.Final["stdev"]), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("P5 / P50 / P95   %.4g / %.4g / %.4g", resp.Final["P5"], resp.Final["P50"], resp.Final["P95"]), 36, y, 0, 0.5)

		// Returns.
		y = titleH + 138.0
		dc.DrawStringAnchored(fmt.Sprintf("mean   %+.4f", resp.Returns["mean"]), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("stdev   %.4f", resp.Returns["stdev"]), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("P5 / P50 / P95   %+.3f / %+.3f / %+.3f", resp.Returns["P5"], resp.Returns["P50"], resp.Returns["P95"]), 36, y, 0, 0.5)

		// Max drawdown.
		y = titleH + 222.0
		dc.DrawStringAnchored(fmt.Sprintf("mean    %.2f%%", resp.MaxDrawdown["mean"]*100), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("median  %.2f%%", resp.MaxDrawdown["P50"]*100), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("worst   %.2f%%", resp.MaxDrawdown["worst"]*100), 36, y, 0, 0.5)

		// Tail.
		y = titleH + 306.0
		dc.DrawStringAnchored(fmt.Sprintf("VaR_95    %.4g", resp.Final["P5"]), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("CVaR_95   %.4g  (mean of worst 5%%)", resp.CVaR95), 36, y, 0, 0.5)
		y += 14
		dc.DrawStringAnchored(fmt.Sprintf("worst     %.4g", resp.WorstFinal), 36, y, 0, 0.5)
	}

	// Drawdown histogram (right).
	chartX := 20 + statsW + gap
	chartW := float64(W) - chartX - 30
	chartTop := titleH + 10
	chartBot := float64(H) - 30

	// Title for the histogram subpanel.
	if f, err := pngFace(true, 13); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Max drawdown distribution", chartX+chartW/2, chartTop+8, 0.5, 0.5)
	}

	// Axes.
	innerTop := chartTop + 26
	axL := chartX + 50
	axR := chartX + chartW - 12
	axT := innerTop + 10
	axB := chartBot - 32

	dc.SetHexColor("#333333")
	dc.SetLineWidth(1.5)
	dc.DrawLine(axL, axT, axL, axB)
	dc.Stroke()
	dc.DrawLine(axL, axB, axR, axB)
	dc.Stroke()

	if len(resp.DrawdownCount) == 0 {
		return nil, fmt.Errorf("empty histogram")
	}
	maxCount := 0
	for _, c := range resp.DrawdownCount {
		if c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}
	binW := (axR - axL) / float64(len(resp.DrawdownCount))
	for i, c := range resp.DrawdownCount {
		h := (float64(c) / float64(maxCount)) * (axB - axT)
		x := axL + float64(i)*binW
		dc.SetHexColor("#1976d2")
		dc.DrawRectangle(x, axB-h, binW, h)
		dc.Fill()
		dc.SetHexColor("#ffffff")
		dc.SetLineWidth(0.5)
		dc.DrawRectangle(x, axB-h, binW, h)
		dc.Stroke()
	}

	// X-axis ticks.
	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("0%", axL, axB+14, 0.5, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%.1f%%", resp.DrawdownBins[len(resp.DrawdownBins)-1]*50), (axL+axR)/2, axB+14, 0.5, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%.1f%%", resp.DrawdownBins[len(resp.DrawdownBins)-1]*100), axR, axB+14, 0.5, 0.5)
		dc.DrawStringAnchored("Drawdown", (axL+axR)/2, axB+28, 0.5, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%d", maxCount), axL-8, axT, 1, 0.5)
		dc.DrawStringAnchored("0", axL-8, axB, 1, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
