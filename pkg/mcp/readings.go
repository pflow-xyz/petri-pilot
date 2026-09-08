package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/eventgen"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// The structural readings sim.pflow.xyz exposes as sim_invariants,
// sim_canonical and the lumping half of sim_classify, plus its dataset
// generator, offered here on an inline model. All four are pure functions of
// the model: no store, no auth, no simulation except the seeded playout that
// dataset asks for. They arrived in pilot through the gap audit that
// followed the pflow showcase; the engine-bound readings (diagnose, class
// verification by experiment, exact CTMC lumpability) stay on sim until
// go-pflow exposes the compiled net they need.

func invariantsTool() mcp.Tool {
	return mcp.NewTool("petri_invariants",
		mcp.WithDescription("Derive a model's full algebraic invariant structure: conservation laws (Farkas P-invariants — weighted place sums every run preserves), firing cycles (T-invariants, named per cycle, each tagged StructuralProof), and the siphon/trap report (every minimal siphon and trap, plus deadlock witnesses — minimal siphons empty at the initial marking, which proves every transition needing one permanently disabled). Pure structure, no simulation; every claim holds for every trajectory from this initial marking. Caveats name what the analysable net encoded lossily or losslessly (capacity as a post-firing bound, a read arc as a reversed inhibitor, a dropped guard). Strictly more than the P-invariants petri_validate and petri_analyze report."),
		mcp.WithString("model", mcp.Required(), mcp.Description("Petri net model JSON or tokenmodel DSL")),
	)
}

func handleInvariants(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	report, err := sim.Invariants(parsed.Model)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func canonicalTool() mcp.Tool {
	return mcp.NewTool("petri_canonical",
		mcp.WithDescription("Exact automorphism orbits and an isomorphism-invariant canonical id for a model: two nets that are the same up to renaming get the same id, and places or transitions in one orbit are provably interchangeable positions in the net (individualization-refinement, McKay & Piperno 2014). Refuses past a 200,000-leaf search budget rather than guess. Use it to detect a template you already have, to dedupe a catalog, or to settle 'are these two pools the same knob' outright."),
		mcp.WithString("model", mcp.Required(), mcp.Description("Petri net model JSON or tokenmodel DSL")),
	)
}

func handleCanonical(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	orbits, err := sim.ExactOrbits(parsed.Model)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := sim.CanonicalModelID(parsed.Model)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out, err := json.MarshalIndent(struct {
		CanonicalID string           `json:"canonical_id"`
		Orbits      *sim.OrbitReport `json:"orbits"`
	}{CanonicalID: id, Orbits: orbits}, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func lumpingTool() mcp.Tool {
	return mcp.NewTool("petri_lumping",
		mcp.WithDescription("Proved place-level reductions of a model's mass-action ODE, each with the question it answers and the refusals it hit. Backward differential equivalence (Cardelli, Tribastone, Tschaikowski & Vandin, POPL 2016): the coarsest partition of places whose members hold identical trajectories for every uniform initial condition, in exact rational arithmetic. Constrained lumping (CLUE, Ovchinnikov et al. 2021), when 'observable' names places: the coarsest partition from which that sum alone stays exactly reconstructible — weaker, so it can find reductions the first cannot. Both refuse schedules and gates rather than answer a different net."),
		mcp.WithString("model", mcp.Required(), mcp.Description("Petri net model JSON or tokenmodel DSL")),
		mcp.WithString("observable", mcp.Description("Optional JSON array of place ids whose SUM is the observable for constrained lumping, e.g. [\"served\",\"vip_served\"]")),
	)
}

func handleLumping(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	result := struct {
		Backward    *sim.DiffEqLumping `json:"backward_differential_equivalence"`
		Constrained *sim.ClueLumping   `json:"constrained_lumping,omitempty"`
	}{}
	result.Backward, err = sim.BackwardDifferentialEquivalence(parsed.Model)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if s := request.GetString("observable", ""); s != "" {
		var ids []string
		if err := json.Unmarshal([]byte(s), &ids); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid observable JSON: %v", err)), nil
		}
		known := map[string]bool{}
		for _, p := range parsed.Model.Places {
			known[p.ID] = true
		}
		for _, id := range ids {
			if !known[id] {
				return mcp.NewToolResultError(fmt.Sprintf("observable names unknown place %q", id)), nil
			}
		}
		result.Constrained, err = sim.ConstrainedLumping(parsed.Model, sim.ObservablePlaces(ids...))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func datasetTool() mcp.Tool {
	return mcp.NewTool("petri_dataset",
		mcp.WithDescription("Generate a synthetic event log from a model: a seeded SSA playout, one case per arrival, emitted as CSV (case_id, activity, timestamp) — exactly the shape petri_conformance replays and the shape a calibration expects. Deterministic: same model, same seed, same bytes. Closes the loop generate → fit → conform without leaving the session."),
		mcp.WithString("model", mcp.Required(), mcp.Description("Petri net model JSON or tokenmodel DSL")),
		mcp.WithNumber("cases", mcp.Description("cases to generate (default 200, max 2000)")),
		mcp.WithNumber("seed", mcp.Description("PRNG seed (default 1)")),
		mcp.WithString("format", mcp.Description("'csv' (default) or 'jsonl'")),
	)
}

func handleDataset(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	cases := int(request.GetFloat("cases", 200))
	if cases <= 0 {
		cases = 200
	}
	if cases > 2000 {
		cases = 2000
	}
	seed := int64(request.GetFloat("seed", 1))
	log, err := eventgen.Playout(parsed.Model, eventgen.Options{Cases: cases, Seed: seed})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var buf bytes.Buffer
	if request.GetString("format", "csv") == "jsonl" {
		err = eventgen.WriteJSONL(&buf, log)
	} else {
		err = eventgen.WriteCSV(&buf, log)
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(buf.String()), nil
}
