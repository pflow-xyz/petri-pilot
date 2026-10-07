package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/learn"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/stochastic"
)

// petri_fit_discrete solves the inverse problem petri_fit cannot: given
// fully-observed discrete sample paths — which transition fired when, from
// which marking — recover the mass-action rate constants by maximising the
// exact continuous-time Markov chain (CTMC) likelihood.
//
//	log L(rates) = Σ_events log a_chosen(x_pre) − ∫_0^T a0(x(t)) dt
//
// petri_fit averages the noise away by fitting the ODE to (t, value) means;
// this tool fits the noise itself, which is the only honest option when the
// data are individual firings rather than a smoothed population curve. The
// propensity is linear in its own rate constant, so the gradient is
// closed-form and go-pflow's stochastic.FitDiscrete drives it through the
// same learn.MinimizeGradient entry point ODE fitting uses.
//
// Paths are accepted keyed by place id — the shape petri_stochastic emits
// with record_events=true — and converted to go-pflow's TokenPlaces order
// here, so a caller never has to know that order. A post-firing marking is
// optional per event: when omitted it is derived by applying the transition's
// stoichiometry, and when supplied it is checked against that derivation by
// go-pflow, which refuses inconsistent data rather than fitting to it.

func fitDiscreteTool() mcp.Tool {
	return mcp.NewTool("petri_fit_discrete",
		mcp.WithDescription("Fit transition rates to observed discrete event data — which transition fired at what time — by maximising the exact CTMC (Gillespie/SSA) log-likelihood. The stochastic counterpart of petri_fit: use it when the observations are individual firings or a recorded sample path rather than a smoothed (t, value) curve. Accepts the `paths` array petri_stochastic emits with record_events=true. Returns fitted rates, the per-transition firing counts, the closed-form MLE (count / exposure) alongside the optimiser's answer, and the negative log-likelihood before and after."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("paths",
			mcp.Required(),
			mcp.Description(`JSON array of observed sample paths: [{"initial": {place_id: count, ...}, "horizon": T, "events": [{"time": t, "transition": id, "marking": {place_id: count, ...}}, ...]}, ...]. "marking" is the post-firing marking and is optional (derived from stoichiometry when absent). Events must be time-ordered; horizon must be >= the last event time. Token places missing from "initial" default to the model's initial marking.`),
		),
		mcp.WithString("parameters",
			mcp.Description(`Which rates to fit: a JSON array of transition ids, or an object {transition_id: initial_guess}. Default: every transition, starting from its model-declared rate (1.0 where none)`),
		),
		mcp.WithString("fixed_rates",
			mcp.Description("JSON object of rates for transitions NOT being fit (default: the model's declared rate, or 1.0)"),
		),
		mcp.WithNumber("max_iter",
			mcp.Description("Max Adam iterations (default 500, max 5000)"),
		),
		mcp.WithNumber("learn_rate",
			mcp.Description("Adam step size (default 0.05)"),
		),
		mcp.WithNumber("grad_tol",
			mcp.Description("Converged when max |∂(−log L)/∂rate| < grad_tol (default 1e-6)"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include the CTMC likelihood derivation in the response. Default false"),
		),
	)
}

// discretePathJSON is the wire shape of one observed path — place-keyed
// so it reads as a model marking, not as a positional vector.
type discretePathJSON struct {
	Initial map[string]int      `json:"initial"`
	Horizon float64             `json:"horizon"`
	Events  []discreteEventJSON `json:"events"`
}

type discreteEventJSON struct {
	Time       float64        `json:"time"`
	Transition string         `json:"transition"`
	Marking    map[string]int `json:"marking,omitempty"`
}

type fitDiscreteResponse struct {
	FittedRates map[string]float64 `json:"fittedRates"`
	// MLE is the closed-form maximum-likelihood rate per fitted transition,
	// firings / exposure, where exposure is ∫ C_j(x(t)) dt over every path.
	// The optimiser's fittedRates should agree with it to convergence
	// tolerance; a gap means the fit stopped early.
	MLE         map[string]float64 `json:"mle"`
	Counts      map[string]int     `json:"counts"`
	Exposure    map[string]float64 `json:"exposure"`
	InitialLoss float64            `json:"initialNegLogLik"`
	FinalLoss   float64            `json:"finalNegLogLik"`
	Iterations  int                `json:"iterations"`
	Converged   bool               `json:"converged"`
	Evals       int                `json:"evals,omitempty"`
	ParamOrder  []string           `json:"paramOrder"`
	Paths       int                `json:"paths"`
	Events      int                `json:"events"`
	Explanation string             `json:"explanation,omitempty"`
}

func handleFitDiscrete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model
	if len(model.Transitions) == 0 {
		return mcp.NewToolResultError("model has no transitions"), nil
	}

	places, err := stochastic.TokenPlaces(model)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	placeIdx := make(map[string]int, len(places))
	for i, p := range places {
		placeIdx[p] = i
	}
	modelInitial := make([]int, len(places))
	for _, p := range model.Places {
		if i, ok := placeIdx[p.ID]; ok {
			modelInitial[i] = p.Initial
		}
	}
	transitions := map[string]bool{}
	for _, t := range model.Transitions {
		transitions[t.ID] = true
	}

	pathsStr, err := request.RequireString("paths")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing paths: %v", err)), nil
	}
	var raw []discretePathJSON
	if err := json.Unmarshal([]byte(pathsStr), &raw); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid paths JSON: %v", err)), nil
	}
	if len(raw) == 0 {
		return mcp.NewToolResultError("no paths supplied"), nil
	}
	paths, nEvents, err := discretePathsFromJSON(model, places, placeIdx, modelInitial, raw)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if nEvents == 0 {
		return mcp.NewToolResultError("paths contain no events; nothing to fit"), nil
	}

	base := stochastic.Rates(model)
	if s := request.GetString("fixed_rates", ""); s != "" {
		var fixed map[string]float64
		if err := json.Unmarshal([]byte(s), &fixed); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid fixed_rates JSON: %v", err)), nil
		}
		for id, r := range fixed {
			if !transitions[id] {
				return mcp.NewToolResultError(fmt.Sprintf("fixed_rates transition %q not found in model", id)), nil
			}
			base[id] = r
		}
	}

	var fit []string
	if s := request.GetString("parameters", ""); s != "" {
		var ids []string
		if err := json.Unmarshal([]byte(s), &ids); err == nil {
			fit = ids
		} else {
			var guesses map[string]float64
			if err := json.Unmarshal([]byte(s), &guesses); err != nil {
				return mcp.NewToolResultError("invalid parameters JSON: want an array of transition ids or an object {id: initial_guess}"), nil
			}
			for id, g := range guesses {
				fit = append(fit, id)
				base[id] = g
			}
			sort.Strings(fit)
		}
	}
	if len(fit) == 0 {
		for _, t := range model.Transitions {
			fit = append(fit, t.ID)
		}
	}
	for _, id := range fit {
		if !transitions[id] {
			return mcp.NewToolResultError(fmt.Sprintf("parameter transition %q not found in model", id)), nil
		}
		if !(base[id] > 0) {
			return mcp.NewToolResultError(fmt.Sprintf("parameter %q: initial guess must be positive (got %g)", id, base[id])), nil
		}
	}

	maxIter := request.GetInt("max_iter", 500)
	if maxIter < 1 {
		maxIter = 1
	}
	if maxIter > 5000 {
		maxIter = 5000
	}
	opts := learn.DefaultFitOptions()
	opts.Method = "adam"
	opts.MaxIters = maxIter
	// The loss-delta check is not the stopping rule here — GradTol is. Same
	// reasoning as fitAdam in fit_gradient.go: the default 1e-4 misreads a
	// slow, honest descent as convergence.
	opts.Tolerance = 1e-10
	opts.LearnRate = request.GetFloat("learn_rate", 0.05)
	opts.GradTol = request.GetFloat("grad_tol", 1e-6)

	initialLoss, _, err := stochastic.NegLogLikelihood(model, base, fit, paths)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("paths inconsistent with model: %v", err)), nil
	}

	res, fitted, err := stochastic.FitDiscrete(model, base, fit, paths, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("fit: %v", err)), nil
	}

	counts, exposure, err := discreteExposure(model, fitted, fit, paths)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("exposure: %v", err)), nil
	}
	mle := make(map[string]float64, len(fit))
	for _, id := range fit {
		if exposure[id] > 0 {
			mle[id] = float64(counts[id]) / exposure[id]
		} else {
			mle[id] = math.NaN()
		}
	}

	resp := fitDiscreteResponse{
		FittedRates: fitted,
		MLE:         mle,
		Counts:      counts,
		Exposure:    exposure,
		InitialLoss: initialLoss,
		FinalLoss:   res.FinalLoss,
		Iterations:  res.Iterations,
		Converged:   res.Converged,
		Evals:       res.Evals,
		ParamOrder:  fit,
		Paths:       len(paths),
		Events:      nEvents,
	}
	if request.GetBool("verbose", false) {
		resp.Explanation = verboseAnnotation("ctmc_likelihood",
			fmt.Sprintf("paths=%d, events=%d, fitting %d of %d transitions", len(paths), nEvents, len(fit), len(model.Transitions)))
	}

	text, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// discretePathsFromJSON converts place-keyed paths to go-pflow's positional
// DiscretePath, deriving each event's post-firing marking from the
// transition's stoichiometry when the caller left it out. Validation of a
// supplied marking is left to NegLogLikelihood, which already does it.
func discretePathsFromJSON(model *goflowmetamodel.Model, places []string, placeIdx map[string]int, modelInitial []int, raw []discretePathJSON) ([]stochastic.DiscretePath, int, error) {
	type arc struct {
		place, weight int
	}
	inputs := map[string][]arc{}
	outputs := map[string][]arc{}
	for _, a := range model.Arcs {
		if a.IsInhibitor() || a.IsRead() {
			continue
		}
		w := a.Weight
		if w == 0 {
			w = 1
		}
		if i, ok := placeIdx[a.From]; ok {
			inputs[a.To] = append(inputs[a.To], arc{i, w})
		} else if i, ok := placeIdx[a.To]; ok {
			outputs[a.From] = append(outputs[a.From], arc{i, w})
		}
	}

	var paths []stochastic.DiscretePath
	nEvents := 0
	for pi, rp := range raw {
		initial := make([]int, len(places))
		copy(initial, modelInitial)
		for id, n := range rp.Initial {
			i, ok := placeIdx[id]
			if !ok {
				return nil, 0, fmt.Errorf("path %d: initial names unknown token place %q", pi, id)
			}
			initial[i] = n
		}
		marking := make([]int, len(places))
		copy(marking, initial)

		events := make([]stochastic.FireEvent, 0, len(rp.Events))
		for ei, re := range rp.Events {
			if re.Transition == "" {
				return nil, 0, fmt.Errorf("path %d event %d: missing transition", pi, ei)
			}
			next := make([]int, len(places))
			if re.Marking != nil {
				copy(next, marking)
				for id, n := range re.Marking {
					i, ok := placeIdx[id]
					if !ok {
						return nil, 0, fmt.Errorf("path %d event %d: marking names unknown token place %q", pi, ei, id)
					}
					next[i] = n
				}
			} else {
				copy(next, marking)
				for _, in := range inputs[re.Transition] {
					next[in.place] -= in.weight
				}
				for _, out := range outputs[re.Transition] {
					next[out.place] += out.weight
				}
			}
			events = append(events, stochastic.FireEvent{Time: re.Time, Transition: re.Transition, Marking: next})
			marking = next
		}
		nEvents += len(events)
		paths = append(paths, stochastic.DiscretePath{Initial: initial, Horizon: rp.Horizon, Events: events})
	}
	return paths, nEvents, nil
}

// discreteExposure recovers, per fitted transition, how many times it fired
// and its integrated combinatorial factor ∫ C_j(x(t)) dt — the two numbers
// whose ratio is the closed-form CTMC MLE. NegLogLikelihood does not export
// them, but its gradient is exactly exposure_j − count_j / rate_j, so one
// gradient evaluation at the fitted rates plus a direct count gives both.
func discreteExposure(model *goflowmetamodel.Model, rates map[string]float64, fit []string, paths []stochastic.DiscretePath) (map[string]int, map[string]float64, error) {
	counts := make(map[string]int, len(fit))
	for _, p := range paths {
		for _, e := range p.Events {
			counts[e.Transition]++
		}
	}
	_, grad, err := stochastic.NegLogLikelihood(model, rates, fit, paths)
	if err != nil {
		return nil, nil, err
	}
	exposure := make(map[string]float64, len(fit))
	for k, id := range fit {
		exposure[id] = grad[k] + float64(counts[id])/rates[id]
	}
	fitCounts := make(map[string]int, len(fit))
	for _, id := range fit {
		fitCounts[id] = counts[id]
	}
	return fitCounts, exposure, nil
}
