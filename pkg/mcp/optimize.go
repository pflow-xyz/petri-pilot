package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_optimize runs multi-objective optimization over transition rates.
// Monte Carlo samples the parameter space, runs each combination to
// equilibrium, identifies the Pareto frontier (points not dominated by any
// other), and visualizes the result.
//
// For 2 objectives → scatter plot with Pareto frontier highlighted.
// For 3+ objectives → parallel-coordinates chart.
//
// Why Monte Carlo and not NSGA-II / Bayesian: this is the smallest sound
// MOO algorithm — no convexity assumptions, no dependencies, deterministic
// under a fixed seed. The scatter plot doubles as a visualization of the
// reachable objective space, not just the frontier, which is usually the
// more informative artifact.

func optimizeTool() mcp.Tool {
	return mcp.NewTool("petri_optimize",
		mcp.WithDescription("Multi-objective optimization over transition rates. Monte Carlo samples the parameter space, runs each combo to equilibrium, identifies the Pareto frontier (non-dominated points), and visualizes it. Returns JSON of every sample (with is_pareto flag) plus a scatter plot (2 objectives) or parallel-coordinates chart (3+)."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("parameters",
			mcp.Required(),
			mcp.Description(`JSON object mapping transition_id → [min, max] rate range. e.g. {"start_brew": [0.1, 5.0], "deliver": [0.5, 2.0]}`),
		),
		mcp.WithString("objectives",
			mcp.Required(),
			mcp.Description(`JSON array of objectives. Each entry: {"place": "place_id", "direction": "max"|"min"}. e.g. [{"place":"delivered","direction":"max"}, {"place":"refunded","direction":"min"}]`),
		),
		mcp.WithNumber("samples",
			mcp.Description("Number of Monte Carlo samples (default 200, max 2000)"),
		),
		mcp.WithString("fixed_rates",
			mcp.Description("JSON object of transition rates held constant during the sweep (default 1.0 for unspecified)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Per-run integration span (default [0, 50])"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed for reproducibility (default 42)"),
		),
	)
}

type objectiveSpec struct {
	Place     string `json:"place"`
	Direction string `json:"direction"` // "max" or "min"
}

type optimizeSample struct {
	Rates    map[string]float64 `json:"rates"`
	Values   map[string]float64 `json:"values"`
	IsPareto bool               `json:"isPareto"`
}

type optimizeResponse struct {
	Parameters  map[string][2]float64 `json:"parameters"`
	Objectives  []objectiveSpec       `json:"objectives"`
	Seed        int64                 `json:"seed"`
	Samples     []optimizeSample      `json:"samples"`
	ParetoCount int                   `json:"paretoCount"`
}

func handleOptimize(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	paramsStr, err := request.RequireString("parameters")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing parameters: %v", err)), nil
	}
	var params map[string][2]float64
	if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid parameters JSON: %v", err)), nil
	}
	if len(params) == 0 {
		return mcp.NewToolResultError("at least one parameter range required"), nil
	}
	// Validate transitions exist; ensure min < max.
	transitionSet := map[string]bool{}
	for _, t := range model.Transitions {
		transitionSet[t.ID] = true
	}
	for tid, rng := range params {
		if !transitionSet[tid] {
			return mcp.NewToolResultError(fmt.Sprintf("transition %q not found in model", tid)), nil
		}
		if rng[1] <= rng[0] {
			return mcp.NewToolResultError(fmt.Sprintf("parameter %q: max (%v) must exceed min (%v)", tid, rng[1], rng[0])), nil
		}
	}

	objsStr, err := request.RequireString("objectives")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing objectives: %v", err)), nil
	}
	var objectives []objectiveSpec
	if err := json.Unmarshal([]byte(objsStr), &objectives); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid objectives JSON: %v", err)), nil
	}
	if len(objectives) < 2 {
		return mcp.NewToolResultError("at least 2 objectives required for multi-objective optimization"), nil
	}
	placeSet := map[string]bool{}
	for _, p := range model.Places {
		placeSet[p.ID] = true
	}
	for _, obj := range objectives {
		if !placeSet[obj.Place] {
			return mcp.NewToolResultError(fmt.Sprintf("objective place %q not found in model", obj.Place)), nil
		}
		if obj.Direction != "max" && obj.Direction != "min" {
			return mcp.NewToolResultError(fmt.Sprintf("objective direction must be \"max\" or \"min\", got %q", obj.Direction)), nil
		}
	}

	n := request.GetInt("samples", 200)
	if n < 2 {
		n = 2
	}
	if n > 2000 {
		// Cap to keep per-call wall clock reasonable. Each sample is an
		// ODE-to-equilibrium run; 2000 × ~10ms = ~20s is already at the
		// upper bound of "interactive."
		n = 2000
	}

	seed := int64(request.GetInt("seed", 42))

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

	// Stable parameter ordering for reproducible sampling.
	paramKeys := make([]string, 0, len(params))
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)

	rng := rand.New(rand.NewSource(seed))
	samples := make([]optimizeSample, 0, n)
	for i := 0; i < n; i++ {
		rates := make(map[string]float64, len(baseRates))
		for k, v := range baseRates {
			rates[k] = v
		}
		sampleRates := make(map[string]float64, len(paramKeys))
		for _, k := range paramKeys {
			lo, hi := params[k][0], params[k][1]
			r := lo + rng.Float64()*(hi-lo)
			rates[k] = r
			sampleRates[k] = r
		}
		prob := solver.NewProblem(net, initial, tspan, rates)
		sol, _ := solver.SolveUntilEquilibrium(prob, solver.Tsit5(), solver.JSParityOptions(), solver.FastEquilibriumOptions())
		if sol == nil {
			continue
		}
		final := sol.GetFinalState()
		values := make(map[string]float64, len(objectives))
		for _, obj := range objectives {
			values[obj.Place] = final[obj.Place]
		}
		samples = append(samples, optimizeSample{
			Rates:  sampleRates,
			Values: values,
		})
	}

	// Mark Pareto-optimal samples.
	paretoCount := identifyPareto(samples, objectives)

	resp := optimizeResponse{
		Parameters:  params,
		Objectives:  objectives,
		Seed:        seed,
		Samples:     samples,
		ParetoCount: paretoCount,
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	if pngBytes, perr := renderOptimizePNG(resp); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// identifyPareto marks each sample's IsPareto flag in-place. A sample A is
// Pareto-optimal iff no other sample B dominates it: B dominates A if B is
// at least as good as A on every objective and strictly better on at least
// one. Returns the count of Pareto-optimal samples.
func identifyPareto(samples []optimizeSample, objectives []objectiveSpec) int {
	for i := range samples {
		samples[i].IsPareto = true
	}
	for i := range samples {
		for j := range samples {
			if i == j {
				continue
			}
			if dominates(samples[j], samples[i], objectives) {
				samples[i].IsPareto = false
				break
			}
		}
	}
	count := 0
	for _, s := range samples {
		if s.IsPareto {
			count++
		}
	}
	return count
}

// dominates returns true if a dominates b on the given objectives (≥ on all,
// > on at least one).
func dominates(a, b optimizeSample, objectives []objectiveSpec) bool {
	atLeastOneStrict := false
	for _, obj := range objectives {
		av, bv := a.Values[obj.Place], b.Values[obj.Place]
		switch obj.Direction {
		case "max":
			if av < bv {
				return false
			}
			if av > bv {
				atLeastOneStrict = true
			}
		case "min":
			if av > bv {
				return false
			}
			if av < bv {
				atLeastOneStrict = true
			}
		}
	}
	return atLeastOneStrict
}
