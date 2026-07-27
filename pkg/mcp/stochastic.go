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

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// petri_stochastic runs Gillespie's Stochastic Simulation Algorithm (SSA)
// on a Petri net under mass-action kinetics. Distinct from petri_ode in
// that it treats the marking as discrete integer counts and firings as
// random events — appropriate when token counts are small enough that
// noise matters (queueing, biology, scarce-resource problems).
//
// Algorithm (Gillespie 1977 / SSA):
//   1. Compute propensity a_i = k_i × C(m, w) over input arcs for each
//      transition i, where C is binomial selection coefficient.
//   2. Total rate A = Σ a_i.
//   3. Wait time τ ~ Exp(A) (i.e. −ln U / A).
//   4. Pick transition i with probability a_i / A.
//   5. Apply firing: −w on input arcs, +w on output arcs.
//   6. Repeat until t > t_end (or A = 0 → terminal state).
//
// With n_realizations > 1, runs are independent replicates whose mean and
// ±stdev band are plotted alongside the underlying trajectories.

func stochasticTool() mcp.Tool {
	return mcp.NewTool("petri_stochastic",
		mcp.WithDescription("Gillespie Stochastic Simulation Algorithm (SSA) over the Petri net's discrete marking. Distinct from petri_ode's continuous ODE — token counts stay integer, firings are random events, results have visible noise. Use when token counts are small enough that variance matters. Multiple realizations show mean ± stdev band."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of mass-action rate constants per transition (default 1.0)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span [t0, tf] (default [0, 10])"),
		),
		mcp.WithString("variables",
			mcp.Description("JSON array of place IDs to plot (default: all places)"),
		),
		mcp.WithNumber("realizations",
			mcp.Description("Number of independent SSA runs (default 1, max 50). With >1, mean and ±stdev band are plotted"),
		),
		mcp.WithNumber("samples",
			mcp.Description("Number of time points to record per realization (default 200)"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed for reproducibility (default 42)"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include the Gillespie SSA algorithm description in the response. Default false"),
		),
	)
}

type stochasticResponse struct {
	StateLabels  []string             `json:"stateLabels"`
	Tspan        [2]float64           `json:"tspan"`
	Rates        map[string]float64   `json:"rates"`
	Realizations int                  `json:"realizations"`
	Times        []float64            `json:"times"`
	Mean         map[string][]float64 `json:"mean"`
	Stdev        map[string][]float64 `json:"stdev,omitempty"`
	FinalMean    map[string]float64   `json:"finalMean"`
	Explanation  string               `json:"explanation,omitempty"`
}

// transitionEntry holds the pre-indexed input/output arcs for one
// transition, in terms of place indices in a stable order. Used by the SSA
// inner loop so we don't keep doing map lookups per step.
type transitionEntry struct {
	id      string
	rate    float64
	inputs  []arcEntry
	outputs []arcEntry
}

type arcEntry struct {
	placeIdx int
	weight   int
}

func handleStochastic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

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

	var variables []string
	if s := request.GetString("variables", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &variables); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid variables JSON: %v", err)), nil
		}
	}
	if len(variables) == 0 {
		for _, p := range model.Places {
			variables = append(variables, p.ID)
		}
	}

	realizations := request.GetInt("realizations", 1)
	if realizations < 1 {
		realizations = 1
	}
	if realizations > 50 {
		realizations = 50
	}
	samples := request.GetInt("samples", 200)
	if samples < 2 {
		samples = 2
	}
	seed := int64(request.GetInt("seed", 42))

	// Build stable place ordering and look-up table.
	placeIdx := map[string]int{}
	stateLabels := make([]string, len(model.Places))
	initialMarking := make([]int, len(model.Places))
	for i, p := range model.Places {
		placeIdx[p.ID] = i
		stateLabels[i] = p.ID
		initialMarking[i] = p.Initial
	}

	// Pre-index transitions with arc lookups in place-index form.
	transitions := buildTransitionEntries(model, placeIdx, rates)
	if len(transitions) == 0 {
		return mcp.NewToolResultError("model has no transitions"), nil
	}

	// Time grid for sampling — all realizations record values at the same
	// grid points so we can compute mean/stdev cleanly.
	times := make([]float64, samples)
	for i := 0; i < samples; i++ {
		times[i] = tspan[0] + (tspan[1]-tspan[0])*float64(i)/float64(samples-1)
	}

	// Run all realizations. trajectories[r][p][t] = marking of place p at
	// time t in realization r.
	trajectories := make([][][]float64, realizations)
	rng := rand.New(rand.NewSource(seed))
	for r := 0; r < realizations; r++ {
		// Each realization uses an independent stream derived from the
		// master seed so the result is reproducible across runs.
		subSeed := rng.Int63()
		trajectories[r] = runSSA(initialMarking, transitions, len(stateLabels), times, subSeed)
	}

	// Aggregate: mean and stdev per (place, time).
	mean := make(map[string][]float64, len(stateLabels))
	stdev := make(map[string][]float64, len(stateLabels))
	for p, label := range stateLabels {
		m := make([]float64, samples)
		s := make([]float64, samples)
		for t := 0; t < samples; t++ {
			sum := 0.0
			for r := 0; r < realizations; r++ {
				sum += trajectories[r][p][t]
			}
			m[t] = sum / float64(realizations)
			if realizations > 1 {
				ss := 0.0
				for r := 0; r < realizations; r++ {
					d := trajectories[r][p][t] - m[t]
					ss += d * d
				}
				s[t] = math.Sqrt(ss / float64(realizations-1))
			}
		}
		mean[label] = m
		if realizations > 1 {
			stdev[label] = s
		}
	}

	finalMean := map[string]float64{}
	for label, vals := range mean {
		finalMean[label] = vals[len(vals)-1]
	}

	resp := stochasticResponse{
		StateLabels:  stateLabels,
		Tspan:        tspan,
		Rates:        rates,
		Realizations: realizations,
		Times:        times,
		Mean:         mean,
		Stdev:        stdev,
		FinalMean:    finalMean,
	}

	if request.GetBool("verbose", false) {
		resp.Explanation = verboseAnnotation("ssa",
			fmt.Sprintf("realizations=%d, tspan=[%v, %v], %d transitions", realizations, tspan[0], tspan[1], len(model.Transitions)))
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	if pngBytes, perr := renderStochasticPNG(resp, variables, trajectories, placeIdx); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// buildTransitionEntries precomputes per-transition arc indices so the SSA
// inner loop can compute propensities without map lookups.
func buildTransitionEntries(model *goflowmetamodel.Model, placeIdx map[string]int, rates map[string]float64) []transitionEntry {
	out := make([]transitionEntry, 0, len(model.Transitions))
	for _, t := range model.Transitions {
		e := transitionEntry{id: t.ID, rate: rates[t.ID]}
		for _, arc := range model.Arcs {
			w := arc.Weight
			if w == 0 {
				w = 1
			}
			if arc.To == t.ID {
				if idx, ok := placeIdx[arc.From]; ok {
					e.inputs = append(e.inputs, arcEntry{placeIdx: idx, weight: w})
				}
			}
			if arc.From == t.ID {
				if idx, ok := placeIdx[arc.To]; ok {
					e.outputs = append(e.outputs, arcEntry{placeIdx: idx, weight: w})
				}
			}
		}
		out = append(out, e)
	}
	return out
}

// runSSA executes one Gillespie realization, recording markings at every
// time in samples. Returns trajectories[placeIdx][sampleIdx].
func runSSA(initial []int, transitions []transitionEntry, nPlaces int, samples []float64, seed int64) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	marking := make([]int, nPlaces)
	copy(marking, initial)

	trajectories := make([][]float64, nPlaces)
	for i := range trajectories {
		trajectories[i] = make([]float64, len(samples))
	}

	t := samples[0]
	nextSample := 0
	// Pre-fill samples at t=samples[0] with the initial state.
	for nextSample < len(samples) && samples[nextSample] <= t {
		for p := 0; p < nPlaces; p++ {
			trajectories[p][nextSample] = float64(marking[p])
		}
		nextSample++
	}

	tEnd := samples[len(samples)-1]
	const maxSteps = 1_000_000

	for step := 0; step < maxSteps && t < tEnd; step++ {
		// Compute propensities.
		totalRate := 0.0
		propensities := make([]float64, len(transitions))
		for i, tr := range transitions {
			a := tr.rate
			enabled := true
			for _, in := range tr.inputs {
				m := marking[in.placeIdx]
				if m < in.weight {
					enabled = false
					break
				}
				// Combinatorial selection: C(m, w).
				a *= combinations(m, in.weight)
			}
			if !enabled {
				a = 0
			}
			propensities[i] = a
			totalRate += a
		}
		if totalRate <= 0 {
			break // dead state — no transition can fire
		}

		// Time to next event ~ Exp(totalRate).
		u := rng.Float64()
		if u <= 0 {
			u = 1e-300
		}
		dt := -math.Log(u) / totalRate
		t += dt

		// Record any sample points we crossed at the pre-firing marking.
		for nextSample < len(samples) && samples[nextSample] <= t {
			for p := 0; p < nPlaces; p++ {
				trajectories[p][nextSample] = float64(marking[p])
			}
			nextSample++
		}
		if t > tEnd {
			break
		}

		// Choose which transition fires.
		r := rng.Float64() * totalRate
		chosen := -1
		acc := 0.0
		for i, a := range propensities {
			acc += a
			if r <= acc {
				chosen = i
				break
			}
		}
		if chosen < 0 {
			chosen = len(propensities) - 1
		}
		tr := transitions[chosen]
		for _, in := range tr.inputs {
			marking[in.placeIdx] -= in.weight
		}
		for _, out := range tr.outputs {
			marking[out.placeIdx] += out.weight
		}
	}

	// Carry final marking forward through any remaining sample points.
	for ; nextSample < len(samples); nextSample++ {
		for p := 0; p < nPlaces; p++ {
			trajectories[p][nextSample] = float64(marking[p])
		}
	}
	return trajectories
}

// combinations returns C(m, w) — the number of distinct multisets of size w
// drawn from m available tokens. C(m, 0) = 1, C(m, 1) = m. For arc weights
// >1 this enforces the requirement that enough indistinguishable tokens are
// present to support the firing.
func combinations(m, w int) float64 {
	if w == 0 {
		return 1
	}
	if w == 1 {
		return float64(m)
	}
	out := 1.0
	for i := 0; i < w; i++ {
		out *= float64(m - i)
	}
	for i := 2; i <= w; i++ {
		out /= float64(i)
	}
	return out
}

// renderStochasticPNG plots the stochastic trajectories. With one
// realization, draws the trajectory as a step function (color per place).
// With >1 realizations, shows mean + shaded ±stdev band per variable.
func renderStochasticPNG(resp stochasticResponse, variables []string, trajectories [][][]float64, placeIdx map[string]int) ([]byte, error) {
	const W, H = 760, 460
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	// Compute y range across selected variables and all realizations.
	xmin := resp.Times[0]
	xmax := resp.Times[len(resp.Times)-1]
	ymin := math.Inf(1)
	ymax := math.Inf(-1)
	for _, v := range variables {
		for _, y := range resp.Mean[v] {
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
		}
		if sd, ok := resp.Stdev[v]; ok && len(sd) == len(resp.Mean[v]) {
			for i, m := range resp.Mean[v] {
				s := sd[i]
				if m+s > ymax {
					ymax = m + s
				}
				if m-s < ymin {
					ymin = m - s
				}
			}
		}
	}
	if math.IsInf(ymin, 1) {
		ymin, ymax = 0, 1
	}
	yrange := ymax - ymin
	if yrange < 1e-9 {
		ymax = ymin + 1
		yrange = 1
	}
	ymin -= yrange * 0.1
	ymax += yrange * 0.1

	title := fmt.Sprintf("Stochastic SSA — %d realization", resp.Realizations)
	if resp.Realizations > 1 {
		title += "s"
	}
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, "Time", "Tokens", 0, 0, W, H)

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
	for i, v := range variables {
		color := plotColors[i%len(plotColors)]
		bandColor := lightenColor(color, 0.4)

		// Shaded ±stdev band when applicable.
		if sd, ok := resp.Stdev[v]; ok && len(sd) == len(resp.Times) {
			dc.SetHexColor(bandColor)
			dc.MoveTo(sx(resp.Times[0]), sy(resp.Mean[v][0]+sd[0]))
			for j := 1; j < len(resp.Times); j++ {
				dc.LineTo(sx(resp.Times[j]), sy(resp.Mean[v][j]+sd[j]))
			}
			for j := len(resp.Times) - 1; j >= 0; j-- {
				dc.LineTo(sx(resp.Times[j]), sy(resp.Mean[v][j]-sd[j]))
			}
			dc.ClosePath()
			dc.Fill()
		}

		// Mean (or single realization) curve on top.
		dc.SetHexColor(color)
		dc.SetLineWidth(2)
		dc.MoveTo(sx(resp.Times[0]), sy(resp.Mean[v][0]))
		for j := 1; j < len(resp.Times); j++ {
			dc.LineTo(sx(resp.Times[j]), sy(resp.Mean[v][j]))
		}
		dc.Stroke()
	}

	// Legend.
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		for i, v := range variables {
			dc.SetHexColor(plotColors[i%len(plotColors)])
			dc.SetLineWidth(2)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(v, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
		if resp.Realizations > 1 {
			legendY += 6
			dc.SetHexColor("#666666")
			dc.DrawStringAnchored("band: ±1 stdev", legendX, legendY+6, 0, 0.5)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// lightenColor blends a hex color with white by t in [0,1]. Used for the
// ±stdev band so it sits visibly beneath the mean curve.
func lightenColor(hex string, t float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	var r, g, b int
	fmt.Sscanf(hex[1:], "%02x%02x%02x", &r, &g, &b)
	r = int(float64(r) + (255-float64(r))*t)
	g = int(float64(g) + (255-float64(g))*t)
	b = int(float64(b) + (255-float64(b))*t)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
