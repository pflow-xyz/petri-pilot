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

// petri_rate_scan: vary a transition rate across a range of values, run each
// rate to equilibrium, and report the steady-state value of one or more
// observables. Returns a JSON summary plus an inline PNG showing each
// observable as a function of the swept rate.

func rateScanTool() mcp.Tool {
	return mcp.NewTool("petri_rate_scan",
		mcp.WithDescription("Parameter sweep: vary one transition's mass-action rate over a list of values, run each to equilibrium, plot observables (steady-state place concentrations) vs the swept rate. Returns JSON of all results plus an inline PNG."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("transition",
			mcp.Required(),
			mcp.Description("Transition ID whose rate is being swept"),
		),
		mcp.WithString("values",
			mcp.Description("JSON array of rate values to test (e.g. [0.1, 0.5, 1.0, 2.0, 5.0]). Either this or 'range' is required"),
		),
		mcp.WithString("range",
			mcp.Description("JSON array [start, stop, n] generating n equally-spaced rate values from start to stop. Alternative to 'values'"),
		),
		mcp.WithString("observables",
			mcp.Description("JSON array of place IDs to track at equilibrium (default: all places)"),
		),
		mcp.WithString("fixed_rates",
			mcp.Description("JSON object of other transition rates held constant during the sweep (default 1.0 for unspecified)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Per-run integration span (default [0, 50]). Must be long enough for the system to settle at each rate"),
		),
		mcp.WithBoolean("plot",
			mcp.Description("Include inline PNG plot (default true)"),
		),
	)
}

type rateScanResponse struct {
	Transition  string          `json:"transition"`
	Values      []float64       `json:"values"`
	Observables []string        `json:"observables"`
	Results     []rateScanPoint `json:"results"`
}

type rateScanPoint struct {
	Rate             float64            `json:"rate"`
	Final            map[string]float64 `json:"final"`
	EquilibriumTime  float64            `json:"equilibriumTime"`
	MaxChange        float64            `json:"maxChange"`
	Reached          bool               `json:"reached"`
	EffectiveReached bool               `json:"effectiveReached,omitempty"`
}

func handleRateScan(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	transitionID, err := request.RequireString("transition")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing transition parameter: %v", err)), nil
	}

	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	// Verify the swept transition exists.
	transitionFound := false
	for _, t := range model.Transitions {
		if t.ID == transitionID {
			transitionFound = true
			break
		}
	}
	if !transitionFound {
		return mcp.NewToolResultError(fmt.Sprintf("transition %q not found in model", transitionID)), nil
	}

	// Build the list of rates to sweep.
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
	if len(values) == 0 {
		return mcp.NewToolResultError("no rate values to scan"), nil
	}
	sort.Float64s(values)

	// Fixed rates for other transitions (default 1.0).
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

	// Observables (default: all places).
	var observables []string
	if s := request.GetString("observables", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &observables); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid observables JSON: %v", err)), nil
		}
	}
	if len(observables) == 0 {
		for _, p := range model.Places {
			observables = append(observables, p.ID)
		}
	}

	tspan := [2]float64{0, 50}
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

	includePlot := request.GetBool("plot", true)

	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}

	resp := rateScanResponse{
		Transition:  transitionID,
		Values:      values,
		Observables: observables,
	}

	const fastEqTolerance = 1e-4

	for _, rate := range values {
		// Per-run rate map — copy base, override swept transition.
		rates := make(map[string]float64, len(baseRates))
		for k, v := range baseRates {
			rates[k] = v
		}
		rates[transitionID] = rate

		prob := solver.NewProblem(net, initial, tspan, rates)
		sol, eq := solver.SolveUntilEquilibrium(prob, solver.Tsit5(), solver.JSParityOptions(), solver.FastEquilibriumOptions())
		if sol == nil {
			continue
		}
		point := rateScanPoint{
			Rate:  rate,
			Final: sol.GetFinalState(),
		}
		if eq != nil {
			point.EquilibriumTime = eq.Time
			point.MaxChange = eq.MaxChange
			point.Reached = eq.Reached
			if !point.Reached && eq.MaxChange < fastEqTolerance {
				point.Reached = true
				point.EffectiveReached = true
			}
		}
		resp.Results = append(resp.Results, point)
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	if includePlot {
		if pngBytes, perr := renderRateScanPNG(transitionID, resp.Results, observables); perr == nil {
			return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
		}
	}
	return mcp.NewToolResultText(string(text)), nil
}

// renderRateScanPNG draws equilibrium observable values as a function of the
// swept rate.
func renderRateScanPNG(transitionID string, results []rateScanPoint, observables []string) ([]byte, error) {
	const W, H = 720, 420
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	xs := make([]float64, len(results))
	ys := make([][]float64, len(observables))
	for i := range observables {
		ys[i] = make([]float64, len(results))
	}
	for i, r := range results {
		xs[i] = r.Rate
		for j, obs := range observables {
			ys[j][i] = r.Final[obs]
		}
	}
	title := fmt.Sprintf("Rate scan over %q", transitionID)
	drawXYPlot(dc, xs, ys, observables, title, "Rate", "Equilibrium value", 0, 0, float64(W), float64(H))

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// linspace returns n equally-spaced values from start to stop inclusive.
func linspace(start, stop float64, n int) []float64 {
	if n <= 1 {
		return []float64{start}
	}
	out := make([]float64, n)
	step := (stop - start) / float64(n-1)
	for i := 0; i < n; i++ {
		out[i] = start + float64(i)*step
	}
	return out
}
