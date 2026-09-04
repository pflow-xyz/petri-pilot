package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_ode runs an ODE (mass-action kinetics) simulation of the model and
// returns a downsampled time series + an inline PNG plot.
//
// The solver and equilibrium options match the pflow.xyz web simulator so
// trajectories computed here are bitwise-comparable to ones produced in the
// browser editor.

func odeTool() mcp.Tool {
	return mcp.NewTool("petri_ode",
		mcp.WithDescription("Run an ODE (mass-action kinetics) simulation of the Petri net using the Tsit5 solver (matches pflow.xyz). Returns a downsampled time series of place concentrations and an inline PNG plot. Use mode=equilibrium to integrate until the system stabilizes. Refuses (Diverged=true) a model with a read arc, inhibitor, reached capacity or guard rather than silently ignore it — use petri_stochastic for those. See the simulation_choice topic (petri_explain) for the full decision rule, including why weight>1 arcs get a genuinely different rate law here than in petri_stochastic."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("rates",
			mcp.Description("Optional JSON object mapping transition_id to rate (default 1.0 for each transition)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Optional JSON array [t0, tf] (default [0, 10])"),
		),
		mcp.WithString("mode",
			mcp.Description("'solve' (full trajectory, default) or 'equilibrium' (stop at steady state)"),
		),
		mcp.WithString("variables",
			mcp.Description("Optional JSON array of place IDs to plot (default: all places)"),
		),
		mcp.WithString("method",
			mcp.Description("'tsit5' (default), 'rk45', 'rk4', or 'euler'"),
		),
		mcp.WithNumber("samples",
			mcp.Description("Max trajectory samples returned (default 200, downsampled if needed)"),
		),
		mcp.WithBoolean("plot",
			mcp.Description("Include inline PNG plot (default true). Ignored when layout is set."),
		),
		mcp.WithString("layout",
			mcp.Description("Output layout: 'plot' (default, trajectory only), 'combined' (net snapshot at final marking + plot side-by-side), or 'net' (net snapshot only, no plot)"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include the algorithm description and formula in the response. Default false. Use to teach a user what the solver actually computed"),
		),
	)
}

type odeResponse struct {
	StateLabels []string           `json:"stateLabels"`
	Tspan       [2]float64         `json:"tspan"`
	Rates       map[string]float64 `json:"rates"`
	Method      string             `json:"method"`
	Samples     []odeSample        `json:"samples"`
	Final       map[string]float64 `json:"final"`
	Equilibrium *odeEquilibrium    `json:"equilibrium,omitempty"`
	Explanation string             `json:"explanation,omitempty"`
}

type odeSample struct {
	T float64            `json:"t"`
	U map[string]float64 `json:"u"`
}

type odeEquilibrium struct {
	Reached          bool    `json:"reached"`
	EffectiveReached bool    `json:"effectiveReached,omitempty"`
	Time             float64 `json:"time"`
	MaxChange        float64 `json:"maxChange"`
	Steps            int     `json:"steps"`
	Reason           string  `json:"reason"`
}

func handleOde(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return mcp.NewToolResultError(fmt.Sprintf("tspan: t1 (%v) must be greater than t0 (%v)", ts[1], ts[0])), nil
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

	methodName := strings.ToLower(request.GetString("method", "tsit5"))
	var solv *solver.Solver
	switch methodName {
	case "rk45":
		solv = solver.RK45()
	case "rk4":
		solv = solver.RK4()
	case "euler":
		solv = solver.Euler()
	case "", "tsit5":
		solv = solver.Tsit5()
		methodName = "tsit5"
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown method %q (use tsit5, rk45, rk4, or euler)", methodName)), nil
	}

	mode := strings.ToLower(request.GetString("mode", "solve"))

	maxSamples := request.GetInt("samples", 200)
	if maxSamples < 2 {
		maxSamples = 2
	}
	includePlot := request.GetBool("plot", true)

	net := buildOdeNet(model)
	initialState := map[string]float64{}
	for _, p := range model.Places {
		initialState[p.ID] = float64(p.Initial)
	}

	prob := solver.NewProblem(net, initialState, tspan, rates)
	opts := solver.JSParityOptions()

	var sol *solver.Solution
	var eqResult *solver.EquilibriumResult
	switch mode {
	case "equilibrium":
		// FastEquilibriumOptions matches what a UI user would consider
		// "stable" — tolerance 1e-4 instead of 1e-6 — and avoids
		// time_exhausted false negatives on asymptotic systems.
		sol, eqResult = solver.SolveUntilEquilibrium(prob, solv, opts, solver.FastEquilibriumOptions())
	case "", "solve":
		sol = solver.Solve(prob, solv, opts)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown mode %q (use solve or equilibrium)", mode)), nil
	}
	if sol == nil || len(sol.T) == 0 {
		return mcp.NewToolResultError("solver returned empty solution"), nil
	}

	resp := odeResponse{
		StateLabels: sol.StateLabels,
		Tspan:       tspan,
		Rates:       rates,
		Method:      methodName,
		Final:       sol.GetFinalState(),
	}
	if eqResult != nil {
		eq := &odeEquilibrium{
			Reached:   eqResult.Reached,
			Time:      eqResult.Time,
			MaxChange: eqResult.MaxChange,
			Steps:     eqResult.Steps,
			Reason:    eqResult.Reason,
		}
		// The detector only marks Reached=true after N consecutive steps
		// below tolerance. On asymptotic systems that condition can be slow
		// to fire even though the system is sitting still. If maxChange is
		// well below the tolerance, treat the result as effectively at
		// equilibrium so the response doesn't lie to the caller.
		const fastEqTolerance = 1e-4
		if !eq.Reached && eq.MaxChange < fastEqTolerance {
			eq.Reached = true
			eq.EffectiveReached = true
			eq.Reason = "effective_equilibrium_below_tolerance"
		}
		resp.Equilibrium = eq
	}

	// Downsample uniformly. Always keep the first and last samples so the
	// reported trajectory spans the full integrated range.
	n := len(sol.T)
	stride := 1
	if maxSamples > 0 && n > maxSamples {
		stride = (n + maxSamples - 1) / maxSamples
	}
	for i := 0; i < n; i += stride {
		resp.Samples = append(resp.Samples, odeSample{T: sol.T[i], U: sol.U[i]})
	}
	if n > 0 && (n-1)%stride != 0 {
		resp.Samples = append(resp.Samples, odeSample{T: sol.T[n-1], U: sol.U[n-1]})
	}

	if request.GetBool("verbose", false) {
		kind := "ode"
		summary := fmt.Sprintf("method=%s, tspan=[%v, %v], %d transitions, %d places", methodName, tspan[0], tspan[1], len(model.Transitions), len(model.Places))
		if mode == "equilibrium" {
			kind = "equilibrium"
			if resp.Equilibrium != nil {
				summary += fmt.Sprintf("\nreached=%v, effectiveReached=%v, time=%v, maxChange=%v",
					resp.Equilibrium.Reached, resp.Equilibrium.EffectiveReached, resp.Equilibrium.Time, resp.Equilibrium.MaxChange)
			}
		}
		resp.Explanation = verboseAnnotation(kind, summary)
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	layout := strings.ToLower(request.GetString("layout", ""))
	title := "ODE Simulation"
	if mode == "equilibrium" {
		title = "ODE → Equilibrium"
	}

	if layout == "" {
		if includePlot {
			layout = "plot"
		}
	}

	var pngBytes []byte
	switch layout {
	case "":
		// no image
	case "plot":
		pngBytes, _ = renderODEPlot(sol, variables, title)
	case "combined":
		pngBytes, _ = renderCombinedNetAndPlot(parsed.Model, sol, variables, sol.GetFinalState(), title)
	case "net":
		opts := &RenderOpts{
			Title:     title,
			Marking:   sol.GetFinalState(),
			ShadeKind: "marking",
		}
		opts.Shading = normalizeShading(opts.Marking)
		pngBytes, _ = renderPNGWithOpts(parsed.Model, opts)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown layout %q (use plot, combined, or net)", layout)), nil
	}

	if pngBytes != nil {
		return mcp.NewToolResultImage(string(withCaveats(text, model)), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(withCaveats(text, model))), nil
}

// buildOdeNet converts a model into the net the continuous solver consumes.
//
// Every arc used to become a consuming arc here, whatever its type. That turned
// a read arc — which tests a place without touching it — into a drain on that
// place, and an inhibitor arc, which *blocks* a firing, into a thing that feeds
// it. Twelve tools share this builder, so one wrong line mis-modelled every
// gated net across the whole analytic surface, quietly and in the same
// direction each time.
//
// Read arcs are dropped: they move no tokens, and the ODE has no firing instant
// at which to test them. Inhibitors are declared as inhibitors so the net at
// least carries the truth, but nothing downstream reads that flag — the solver
// integrates the arc as an ordinary input, consuming its place and multiplying
// it into the rate, so a marked inhibitor makes the transition it blocks run
// *faster*. Neither is honoured by a continuous solve; that is what odeCaveats
// exists to say, and petri-pilot's own Forecast refuses such a model outright
// rather than answering it.
//
// A non-kinetic arc is emitted as an ordinary arc, deliberately. It really does
// consume its tokens, so dropping it the way a read arc is dropped would break
// stoichiometry on top of the rate law and leave the net conserving nothing. The
// mass-action solver multiplies every input into the rate and has no way to be
// told "this one gates and is consumed but does not accelerate anything", so the
// curve overstates the rate whenever such an input is plentiful. That is a
// caveat, not a fix — see odeCaveats.
func buildOdeNet(model *goflowmetamodel.Model) *petri.PetriNet {
	b := petri.Build()
	for _, p := range model.Places {
		if p.Capacity > 0 {
			b = b.PlaceWithCapacity(p.ID, float64(p.Initial), float64(p.Capacity))
			continue
		}
		b = b.Place(p.ID, float64(p.Initial))
	}
	for _, t := range model.Transitions {
		b = b.Transition(t.ID)
	}
	for i := range model.Arcs {
		arc := &model.Arcs[i]
		if arc.IsRead() {
			continue
		}
		w := arc.Weight
		if w == 0 {
			w = 1
		}
		if arc.IsInhibitor() {
			b = b.InhibitorArc(arc.From, arc.To, float64(w))
			continue
		}
		b = b.Arc(arc.From, arc.To, float64(w))
	}
	return b.Done()
}

// odeCaveats is what a continuous answer cannot cover.
//
// The mass-action solver integrates a rate law over real-valued concentrations.
// There is no firing instant in that picture, so a read arc, an inhibitor, a
// capacity and a guard are all unrepresentable — the solver does not approximate
// them badly, it does not see them at all. A non-kinetic arc fails the other
// way: the solver sees it and cannot help but put it in the rate law, so a
// staffed shop is answered as one where every extra barista makes every drink in
// progress finish faster. Either way the model is being answered as though it
// were something looser than what was written, and the caller has to be told so,
// on every tool that shares this builder.
func odeCaveats(model *goflowmetamodel.Model) []string {
	return model.Gating()
}

// withCaveats adds a "caveats" field to a tool's JSON summary when the model
// leans on something the continuous solver cannot see.
//
// Returns text untouched when there is nothing to say, so every unconstrained
// model — which is all of the committed examples — produces byte-identical
// output. Only a gated model takes the re-marshalling path.
func withCaveats(text []byte, model *goflowmetamodel.Model) []byte {
	gating := odeCaveats(model)
	if len(gating) == 0 {
		return text
	}

	var summary map[string]any
	if err := json.Unmarshal(text, &summary); err != nil {
		return text // not a JSON object: leave it alone rather than corrupt it
	}
	summary["caveats"] = gating
	summary["caveats_note"] = "The continuous solver cannot enforce these, so the result above describes " +
		"a less constrained system than the one modelled. Use petri_stochastic for an answer that honours them."

	out, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return text
	}
	return out
}
