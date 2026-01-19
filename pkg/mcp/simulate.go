package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	pflowMetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// SimulationStep represents a single step in a simulation.
type SimulationStep struct {
	Transition string         `json:"transition"`
	Bindings   map[string]any `json:"bindings,omitempty"`
}

// SimulationResult represents the result of a simulation.
type SimulationResult struct {
	Success    bool              `json:"success"`
	FinalState map[string]int    `json:"final_state"`
	Steps      []StepResult      `json:"steps"`
	Error      string            `json:"error,omitempty"`
	
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
type StepResult struct {
	Transition  string         `json:"transition"`
	Enabled     bool           `json:"enabled"`
	StateBefore map[string]int `json:"state_before"`
	StateAfter  map[string]int `json:"state_after"`
	Error       string         `json:"error,omitempty"`
}

// simulate executes a simulation given a model and a list of steps.
func simulate(model *schema.Model, steps []SimulationStep) SimulationResult {
	// Convert to go-pflow metamodel
	metaSchema := bridge.ToMetamodel(model)

	// Create runtime
	runtime := pflowMetamodel.NewRuntime(metaSchema)

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
func executeStep(runtime *pflowMetamodel.Runtime, metaSchema *pflowMetamodel.Schema, step SimulationStep) StepResult {
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

	// Execute the transition
	if err := runtime.Execute(step.Transition); err != nil {
		stepResult.Error = fmt.Sprintf("execution error: %v", err)
		stepResult.StateAfter = captureMarking(runtime, metaSchema)
		return stepResult
	}

	// Capture state after execution
	stepResult.StateAfter = captureMarking(runtime, metaSchema)

	return stepResult
}

// captureMarking captures the current marking (token counts) of all places.
func captureMarking(runtime *pflowMetamodel.Runtime, metaSchema *pflowMetamodel.Schema) map[string]int {
	marking := make(map[string]int)
	for _, state := range metaSchema.States {
		if state.IsToken() {
			marking[state.ID] = runtime.Tokens(state.ID)
		}
	}
	return marking
}

// determineDisabledReason determines why a transition is disabled.
func determineDisabledReason(runtime *pflowMetamodel.Runtime, metaSchema *pflowMetamodel.Schema, transitionID string) string {
	inputArcs := metaSchema.InputArcs(transitionID)
	if len(inputArcs) == 0 {
		return "transition has no input arcs"
	}

	var missingTokens []string
	for _, arc := range inputArcs {
		st := metaSchema.StateByID(arc.Source)
		if st != nil && st.IsToken() {
			if runtime.Tokens(arc.Source) < 1 {
				missingTokens = append(missingTokens, arc.Source)
			}
		}
	}

	if len(missingTokens) > 0 {
		return fmt.Sprintf("insufficient tokens in: %s", strings.Join(missingTokens, ", "))
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

	// Parse model
	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

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
