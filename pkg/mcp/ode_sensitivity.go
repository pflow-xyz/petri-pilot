package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_ode_sensitivity computes which transition rates most influence a
// chosen observable at equilibrium, using finite differences. Where
// petri_analyze's sensitivity is structural (importance from reachability
// graph), this one is behavioral — it answers "if I sped this transition
// up 5%, how much would the equilibrium value of place X move?"

func odeSensitivityTool() mcp.Tool {
	return mcp.NewTool("petri_ode_sensitivity",
		mcp.WithDescription("ODE sensitivity analysis: perturb each transition's rate by a small delta and measure how much an observable's equilibrium value moves. Returns dimensionless elasticities per transition plus an inline net diagram tinted by influence. Use when you want to know which knobs matter for dynamics, not just structure."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("observable",
			mcp.Required(),
			mcp.Description("Place ID whose equilibrium value is the target metric"),
		),
		mcp.WithString("base_rates",
			mcp.Description("JSON object of base rates per transition (default 1.0)"),
		),
		mcp.WithNumber("delta",
			mcp.Description("Perturbation fraction (default 0.05 = 5%)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Per-run integration span (default [0, 50])"),
		),
		mcp.WithString("title",
			mcp.Description("Title shown above the sensitivity diagram"),
		),
	)
}

type odeSensitivityResponse struct {
	Observable       string             `json:"observable"`
	BaseEquilibrium  float64            `json:"baseEquilibrium"`
	BaseRates        map[string]float64 `json:"baseRates"`
	Delta            float64            `json:"delta"`
	Elasticities     map[string]float64 `json:"elasticities"`
	Ranked           []elasticityEntry  `json:"ranked"`
}

type elasticityEntry struct {
	Transition string  `json:"transition"`
	Elasticity float64 `json:"elasticity"`
}

func handleOdeSensitivity(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
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

	observableFound := false
	for _, p := range model.Places {
		if p.ID == observable {
			observableFound = true
			break
		}
	}
	if !observableFound {
		return mcp.NewToolResultError(fmt.Sprintf("observable %q is not a place in the model", observable)), nil
	}

	baseRates := map[string]float64{}
	for _, t := range model.Transitions {
		baseRates[t.ID] = 1.0
	}
	if s := request.GetString("base_rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid base_rates JSON: %v", err)), nil
		}
		for k, v := range user {
			baseRates[k] = v
		}
	}

	delta := request.GetFloat("delta", 0.05)
	if delta <= 0 || delta >= 1 {
		return mcp.NewToolResultError("delta must be in (0, 1)"), nil
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

	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}

	// Base run.
	baseEq := runToEquilibrium(net, initial, tspan, baseRates, observable)

	// Per-transition perturbation runs.
	elasticities := map[string]float64{}
	for tid, base := range baseRates {
		perturbed := make(map[string]float64, len(baseRates))
		for k, v := range baseRates {
			perturbed[k] = v
		}
		perturbed[tid] = base * (1 + delta)
		eq := runToEquilibrium(net, initial, tspan, perturbed, observable)

		// Dimensionless elasticity: (dE/E) / (dk/k). Guard against base
		// equilibrium == 0 (use absolute change instead).
		if math.Abs(baseEq) > 1e-9 {
			elasticities[tid] = (eq - baseEq) / baseEq / delta
		} else {
			elasticities[tid] = (eq - baseEq) / delta
		}
	}

	// Rank by absolute elasticity for the response and for the visualization
	// shading.
	ranked := make([]elasticityEntry, 0, len(elasticities))
	for tid, e := range elasticities {
		ranked = append(ranked, elasticityEntry{Transition: tid, Elasticity: e})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return math.Abs(ranked[i].Elasticity) > math.Abs(ranked[j].Elasticity)
	})

	resp := odeSensitivityResponse{
		Observable:      observable,
		BaseEquilibrium: baseEq,
		BaseRates:       baseRates,
		Delta:           delta,
		Elasticities:    elasticities,
		Ranked:          ranked,
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	// Build a Shading map (absolute elasticities normalized to [0, 1]) and
	// render the net.
	shadingRaw := map[string]float64{}
	for tid, e := range elasticities {
		shadingRaw[tid] = math.Abs(e)
	}
	title := request.GetString("title", "")
	if title == "" {
		title = fmt.Sprintf("ODE sensitivity → %s", observable)
	}
	opts := &RenderOpts{
		Title:     title,
		Shading:   normalizeShading(shadingRaw),
		ShadeKind: "sensitivity",
	}

	if pngBytes, perr := renderPNGWithOpts(model, opts); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// runToEquilibrium runs a single ODE problem and returns the final value of
// the chosen observable. Equilibrium options are FastEquilibrium so the
// inner loop is cheap; sensitivity analysis runs O(transitions) integrations
// so per-call cost matters.
func runToEquilibrium(net *petri.PetriNet, initial map[string]float64, tspan [2]float64, rates map[string]float64, observable string) float64 {
	prob := solver.NewProblem(net, initial, tspan, rates)
	sol, _ := solver.SolveUntilEquilibrium(prob, solver.Tsit5(), solver.JSParityOptions(), solver.FastEquilibriumOptions())
	if sol == nil {
		return math.NaN()
	}
	final := sol.GetFinalState()
	if final == nil {
		return math.NaN()
	}
	return final[observable]
}
