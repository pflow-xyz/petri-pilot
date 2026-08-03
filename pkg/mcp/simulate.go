package mcp

// This file implements the petri_simulate MCP tool, which allows firing transitions
// and observing state changes without generating code. It's useful for:
// - Verifying workflow reaches terminal state
// - Testing guard conditions
// - Exploring branching paths
// - Validating model before codegen
//
// The tool provides detailed step-by-step state traces showing the state before
// and after each transition, making it easy to understand the simulation execution.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/dsl"
	pilotmeta "github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

// SimulationStep represents a single step in a simulation.
// It specifies which transition to fire and optional bindings for the transition.
type SimulationStep struct {
	Transition string         `json:"transition"`
	Bindings   map[string]any `json:"bindings,omitempty"`
}

// SimulationResult represents the result of a simulation.
// It includes the overall success status, final state, and detailed step-by-step trace.
type SimulationResult struct {
	Success    bool           `json:"success"`
	FinalState map[string]int `json:"final_state"`
	Steps      []StepResult   `json:"steps"`
	Error      string         `json:"error,omitempty"`

	// Legacy fields for backwards compatibility with existing tests
	InitialMarking map[string]int `json:"initial_marking,omitempty"`
	FinalMarking   map[string]int `json:"final_marking,omitempty"`
	Fired          []string       `json:"fired,omitempty"`
	Failed         []FailedStep   `json:"failed,omitempty"`
	IsDeadlock     bool           `json:"is_deadlock,omitempty"`
	Enabled        []string       `json:"enabled,omitempty"`
}

// FailedStep represents a failed transition for backwards compatibility.
type FailedStep struct {
	TransitionID string `json:"transition_id"`
	Reason       string `json:"reason"`
}

// StepResult represents the result of executing a single step.
// It captures the state before and after the transition, enabling detailed analysis.
type StepResult struct {
	Transition  string         `json:"transition"`
	Enabled     bool           `json:"enabled"`
	StateBefore map[string]int `json:"state_before"`
	StateAfter  map[string]int `json:"state_after"`
	Error       string         `json:"error,omitempty"`
}

// simulate executes a simulation given a model and a list of steps.
//
// It runs on petri-pilot's own Runtime rather than go-pflow's tokenmodel one.
// That is not a preference: tokenmodel.Runtime ignores arc weights (its
// enablement check is hardcoded to "< 1"), ignores inhibitor arcs entirely,
// moves exactly one token per arc whatever the weight says, and never evaluates
// a guard — so it would report firing sequences the model forbids, which is the
// one thing a simulator must not do.
func simulate(model *metamodel.Model, steps []SimulationStep) SimulationResult {
	metaSchema := pilotmeta.SchemaFromModel(model)

	runtime := pilotmeta.NewRuntime(metaSchema)
	// Guards are part of the firing rule, so the simulator needs an evaluator.
	runtime.GuardEvaluator = dsl.NewEvaluator()

	result := SimulationResult{
		Success: true,
		Steps:   make([]StepResult, 0, len(steps)),
		Fired:   make([]string, 0, len(steps)),
		Failed:  make([]FailedStep, 0),
	}

	// Capture initial marking for backwards compatibility
	result.InitialMarking = captureMarking(runtime, metaSchema)

	// Execute each step
	for _, step := range steps {
		stepResult := executeStep(runtime, metaSchema, step)
		result.Steps = append(result.Steps, stepResult)

		// Track fired and failed for backwards compatibility
		if stepResult.Enabled && stepResult.Error == "" {
			result.Fired = append(result.Fired, stepResult.Transition)
		} else {
			result.Success = false
			errorMsg := stepResult.Error
			if errorMsg == "" {
				errorMsg = "transition not enabled"
			}
			result.Failed = append(result.Failed, FailedStep{
				TransitionID: stepResult.Transition,
				Reason:       errorMsg,
			})
		}
	}

	// Capture final state
	result.FinalState = captureMarking(runtime, metaSchema)
	result.FinalMarking = result.FinalState // Backwards compatibility

	// Check for deadlock (no enabled transitions)
	enabledTransitions := runtime.EnabledActions()
	result.IsDeadlock = len(enabledTransitions) == 0
	result.Enabled = enabledTransitions

	return result
}

// executeStep executes a single simulation step and returns its result.
func executeStep(runtime *pilotmeta.Runtime, metaSchema *pilotmeta.Schema, step SimulationStep) StepResult {
	stepResult := StepResult{
		Transition: step.Transition,
	}

	// Capture state before execution
	stepResult.StateBefore = captureMarking(runtime, metaSchema)

	// Check if action exists
	action := metaSchema.ActionByID(step.Transition)
	if action == nil {
		stepResult.Enabled = false
		stepResult.Error = "transition not found in model"
		stepResult.StateAfter = stepResult.StateBefore
		return stepResult
	}

	// Check if enabled
	if !runtime.Enabled(step.Transition) {
		stepResult.Enabled = false
		stepResult.Error = determineDisabledReason(runtime, metaSchema, step.Transition)
		stepResult.StateAfter = stepResult.StateBefore
		return stepResult
	}

	stepResult.Enabled = true

	// Execute the transition, passing the step's bindings so parameter guards
	// and data arcs see them. (They used to be parsed and then discarded.)
	bindings := pilotmeta.Bindings{}
	for k, v := range step.Bindings {
		bindings[k] = v
	}
	if err := runtime.ExecuteWithBindings(step.Transition, bindings); err != nil {
		stepResult.Error = fmt.Sprintf("execution error: %v", err)
		stepResult.StateAfter = captureMarking(runtime, metaSchema)
		return stepResult
	}

	// Capture state after execution
	stepResult.StateAfter = captureMarking(runtime, metaSchema)

	return stepResult
}

// captureMarking captures the current marking (token counts) of all places.
func captureMarking(runtime *pilotmeta.Runtime, metaSchema *pilotmeta.Schema) map[string]int {
	marking := make(map[string]int)
	for _, state := range metaSchema.States {
		if state.IsToken() {
			marking[state.ID] = runtime.Tokens(state.ID)
		}
	}
	return marking
}

// determineDisabledReason determines why a transition is disabled.
func determineDisabledReason(runtime *pilotmeta.Runtime, metaSchema *pilotmeta.Schema, transitionID string) string {
	inputArcs := metaSchema.InputArcs(transitionID)
	if len(inputArcs) == 0 {
		return "transition has no input arcs"
	}

	// Output-side read-only arcs gate too (pflow-xyz spells a guard as an
	// inhibitor pointing action -> state), so they must be walked here or a
	// transition disabled solely by one reports the wrong reason.
	arcs := append(append([]pilotmeta.Arc{}, inputArcs...), metaSchema.OutputArcs(transitionID)...)

	var missingTokens []string
	var blockedBy []string
	var unmetReads []string
	for _, arc := range arcs {
		reversed := arc.Source == transitionID
		if reversed && !arc.IsReadOnly() {
			continue
		}
		if !pilotmeta.IsKnownArcType(arc.Type) {
			// Enabled refuses the whole action for this, so say so instead of
			// reporting a marking problem the caller cannot fix.
			return fmt.Sprintf("arc %s -> %s has unknown type %q; this build cannot execute it",
				arc.Source, arc.Target, arc.Type)
		}
		place := arc.Source
		if reversed {
			place = arc.Target
		}
		st := metaSchema.StateByID(place)
		if st == nil || !st.IsToken() {
			continue
		}
		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}
		have := runtime.Tokens(place)
		if arc.IsInhibitor() && !reversed {
			// An inhibitor blocks while the place holds at least its weight.
			if have >= weight {
				blockedBy = append(blockedBy, fmt.Sprintf("%s (%d tokens)", place, have))
			}
			continue
		}
		if arc.IsReadOnly() {
			// A read arc is a lower bound like a normal arc, but naming it as
			// "insufficient tokens" would send the reader looking for the
			// firing that took them: nothing consumes a read place. A REVERSED
			// inhibitor is pflow-xyz's spelling of exactly this arc.
			if have < weight {
				unmetReads = append(unmetReads, fmt.Sprintf("%s (have %d, need %d)", place, have, weight))
			}
			continue
		}
		if have < weight {
			missingTokens = append(missingTokens, fmt.Sprintf("%s (have %d, need %d)", place, have, weight))
		}
	}

	// Assembled as clauses so a transition blocked several ways reports all of
	// them: fixing only the one that happened to be named leaves it disabled.
	var clauses []string
	if len(blockedBy) > 0 {
		clauses = append(clauses, "inhibited by: "+strings.Join(blockedBy, ", "))
	}
	if len(missingTokens) > 0 {
		clauses = append(clauses, "insufficient tokens in: "+strings.Join(missingTokens, ", "))
	}
	if len(unmetReads) > 0 {
		clauses = append(clauses, "read condition not met (nothing is consumed from these): "+
			strings.Join(unmetReads, ", "))
	}
	if len(clauses) > 0 {
		return strings.Join(clauses, "; ")
	}

	return "insufficient tokens in input places"
}

// handleSimulateWithSteps handles the petri_simulate tool request with detailed step-by-step results.
// This supports both the old "transitions" parameter (array of strings) and new "steps" parameter
// (array of SimulationStep objects with optional bindings).
func handleSimulateWithSteps(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	// Parse model (supports both v1 and v2 schemas)
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	// Try new "steps" parameter first, then fall back to "transitions" for backwards compatibility
	var steps []SimulationStep

	if stepsJSON := request.GetString("steps", ""); stepsJSON != "" {
		// New API with SimulationStep objects
		if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid steps JSON: %v", err)), nil
		}
	} else if transitionsJSON := request.GetString("transitions", ""); transitionsJSON != "" {
		// Old API with string array - convert to SimulationStep objects
		var transitions []string
		if err := json.Unmarshal([]byte(transitionsJSON), &transitions); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid transitions JSON: %v", err)), nil
		}

		// Convert string array to SimulationStep array
		for _, t := range transitions {
			steps = append(steps, SimulationStep{Transition: t})
		}
	} else {
		return mcp.NewToolResultError("missing 'steps' or 'transitions' parameter"), nil
	}

	// Run simulation
	result := simulate(model, steps)

	// Marshal result
	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(outputJSON)), nil
}
