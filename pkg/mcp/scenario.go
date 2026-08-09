package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// petri_scenario answers a what-if.
//
// The other analytic tools take a model and vary a *rate*. That covers "what if
// demand doubled" and misses the question an operator usually has, which is
// about the marking: how many people are on shift, how much stock is on hand,
// how long the queue already is. Those live in the initial marking, and until
// now nothing exposed them.
//
// It runs the same code the generated applications serve, so an answer here and
// an answer from the deployed app are the same answer. That matters more than
// it sounds: a modelling tool that disagrees with the thing it generated is
// worse than having neither, because the disagreement is invisible.

func scenarioTool() mcp.Tool {
	return mcp.NewTool("petri_scenario",
		mcp.WithDescription("Answer a what-if about a Petri net: override the initial marking (staff on duty, stock on hand, queue depth), override rates, or vary a rate over time, then run it forward. Returns a trajectory plus operator metrics — throughput, mean and P95 per place, resource utilization, time-to-depletion. Use `scenarios` to compare several on one seed, which is the only way to tell a real difference from the dice. Pure: it never changes the model."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON, bundle document, or tokenmodel DSL"),
		),
		mcp.WithString("marking",
			mcp.Description(`JSON object overriding the initial marking, place by place — {"staff/available": 3}. Sparse: places you do not name keep what the model declares. Unknown place names are an error, not a silent no-op`),
		),
		mcp.WithString("rates",
			mcp.Description(`JSON object overriding transition rates for the whole horizon — {"order_latte": 30}`),
		),
		mcp.WithString("schedule",
			mcp.Description(`JSON object making a rate vary over time — {"order_latte": [{"until": 2, "value": 40}, {"until": 8, "value": 12}]}. This is how you express a morning rush; a constant rate cannot, and averaging one away hides whether the queue recovers`),
		),
		mcp.WithString("scenarios",
			mcp.Description(`JSON array of named scenarios to compare, each with its own marking/rates/schedule — [{"name":"today","marking":{"staff/available":2}},{"name":"one more","marking":{"staff/available":3}}]. All run on one seed. When set, the top-level marking/rates/schedule are ignored`),
		),
		mcp.WithNumber("hours",
			mcp.Description("How far forward to run, in the model's time unit (default 1)"),
		),
		mcp.WithNumber("samples",
			mcp.Description("Time points to report (default 60)"),
		),
		mcp.WithNumber("realizations",
			mcp.Description("Independent stochastic runs to average (default 1). A single run of a queue is an anecdote"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed (default 1, so an unconfigured call is still repeatable)"),
		),
		mcp.WithString("engine",
			mcp.Description("'ssa' (default, discrete) or 'ode' (continuous). SSA is right for staffing and queueing, where counts are small enough that noise decides the outcome. The ODE refuses models whose constraints it cannot represent rather than answering a less constrained question"),
		),
	)
}

func handleScenario(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model: %v", err)), nil
	}
	model := parsed.Model

	base := sim.Scenario{
		Horizon:      request.GetFloat("hours", 1),
		Samples:      request.GetInt("samples", 60),
		Realizations: request.GetInt("realizations", 1),
		Seed:         int64(request.GetInt("seed", 1)),
		Engine:       request.GetString("engine", ""),
	}

	if raw := request.GetString("scenarios", ""); raw != "" {
		var scenarios []sim.Scenario
		if err := json.Unmarshal([]byte(raw), &scenarios); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid scenarios JSON: %v", err)), nil
		}
		// Per-scenario knobs are for what differs between them; the run
		// settings are shared, so a caller does not have to repeat the horizon
		// on every entry and cannot accidentally compare different horizons.
		for i := range scenarios {
			if scenarios[i].Horizon <= 0 {
				scenarios[i].Horizon = base.Horizon
			}
			if scenarios[i].Samples <= 0 {
				scenarios[i].Samples = base.Samples
			}
			if scenarios[i].Realizations <= 0 {
				scenarios[i].Realizations = base.Realizations
			}
			if scenarios[i].Engine == "" {
				scenarios[i].Engine = base.Engine
			}
			scenarios[i].Seed = base.Seed
		}
		cmp, err := sim.Compare(model, scenarios)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		text, err := json.MarshalIndent(summariseComparison(cmp), "", "  ")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(text)), nil
	}

	if raw := request.GetString("marking", ""); raw != "" {
		if err := json.Unmarshal([]byte(raw), &base.Marking); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid marking JSON: %v", err)), nil
		}
	}
	if raw := request.GetString("rates", ""); raw != "" {
		if err := json.Unmarshal([]byte(raw), &base.Rates); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid rates JSON: %v", err)), nil
		}
	}
	if raw := request.GetString("schedule", ""); raw != "" {
		if err := json.Unmarshal([]byte(raw), &base.Schedule); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid schedule JSON: %v", err)), nil
		}
	}

	res, err := sim.Run(model, base)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// comparisonSummary is what a caller reading several scenarios actually wants:
// the numbers side by side, not three full trajectories to diff by eye.
type comparisonSummary struct {
	Scenarios []scenarioSummary `json:"scenarios"`
	Note      string            `json:"note"`
}

type scenarioSummary struct {
	Name        string             `json:"name"`
	Final       map[string]float64 `json:"final"`
	Throughput  map[string]float64 `json:"throughput,omitempty"`
	MeanPerc95  map[string]string  `json:"mean_and_p95,omitempty"`
	Utilization map[string]float64 `json:"utilization,omitempty"`
	Depleted    []sim.Depletion    `json:"depleted,omitempty"`
	// Caveats are constraints of *this model* the run could not enforce;
	// Assumptions are what the method assumes whatever the model. Kept apart
	// here for the same reason they are kept apart on sim.Result: an empty
	// caveat list is the claim that everything the net says was applied, and
	// folding a method assumption in makes that claim unfalsifiable.
	Caveats     []string `json:"caveats,omitempty"`
	Assumptions []string `json:"assumptions,omitempty"`
	Reason      string   `json:"refused,omitempty"`
}

func summariseComparison(cmp *sim.Comparison) comparisonSummary {
	out := comparisonSummary{
		Note: "All scenarios ran on one seed, so a difference between them is the change you made rather than the dice. " +
			"mean_and_p95 reads \"mean / P95\" per place: the average is reassuring and the 95th percentile is what the " +
			"person standing in the queue experiences. Both are weighted by time held, not by sample point, so they do " +
			"not move with the reporting grid. caveats are constraints of the model this run could not enforce — an " +
			"empty list means every one was applied; assumptions are what the method assumes whatever the model says.",
	}
	for _, s := range cmp.Scenarios {
		summary := scenarioSummary{
			Name:        s.Name,
			Final:       s.Result.Final,
			Depleted:    s.Result.Depleted,
			Caveats:     s.Result.Caveats,
			Assumptions: s.Result.Assumptions,
		}
		if s.Result.Diverged {
			summary.Reason = s.Result.Reason
		}
		if mt := s.Result.Metrics; mt != nil {
			summary.Throughput = mt.Throughput
			summary.Utilization = mt.Utilization
			summary.MeanPerc95 = map[string]string{}
			for _, p := range sortedPlaces(mt.Mean) {
				summary.MeanPerc95[p] = fmt.Sprintf("%.1f / %.0f", mt.Mean[p], mt.P95[p])
			}
		}
		out.Scenarios = append(out.Scenarios, summary)
	}
	return out
}

func sortedPlaces(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
