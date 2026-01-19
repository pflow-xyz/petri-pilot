package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestSimulateWithDetailedSteps(t *testing.T) {
	tests := []struct {
		name               string
		modelJSON          string
		steps              []SimulationStep
		expectSuccess      bool
		expectStepCount    int
		checkSteps         func(t *testing.T, steps []StepResult)
	}{
		{
			name: "detailed step-by-step trace",
			modelJSON: `{
				"name": "workflow",
				"places": [
					{"id": "start", "initial": 1},
					{"id": "middle", "initial": 0},
					{"id": "end", "initial": 0}
				],
				"transitions": [
					{"id": "step1"},
					{"id": "step2"}
				],
				"arcs": [
					{"from": "start", "to": "step1"},
					{"from": "step1", "to": "middle"},
					{"from": "middle", "to": "step2"},
					{"from": "step2", "to": "end"}
				]
			}`,
			steps: []SimulationStep{
				{Transition: "step1"},
				{Transition: "step2"},
			},
			expectSuccess:   true,
			expectStepCount: 2,
			checkSteps: func(t *testing.T, steps []StepResult) {
				// First step
				if !steps[0].Enabled {
					t.Error("Expected step1 to be enabled")
				}
				if steps[0].StateBefore["start"] != 1 {
					t.Errorf("Expected start=1 before step1, got %d", steps[0].StateBefore["start"])
				}
				if steps[0].StateAfter["middle"] != 1 {
					t.Errorf("Expected middle=1 after step1, got %d", steps[0].StateAfter["middle"])
				}

				// Second step
				if !steps[1].Enabled {
					t.Error("Expected step2 to be enabled")
				}
				if steps[1].StateBefore["middle"] != 1 {
					t.Errorf("Expected middle=1 before step2, got %d", steps[1].StateBefore["middle"])
				}
				if steps[1].StateAfter["end"] != 1 {
					t.Errorf("Expected end=1 after step2, got %d", steps[1].StateAfter["end"])
				}
			},
		},
		{
			name: "disabled transition with detailed error",
			modelJSON: `{
				"name": "blocked",
				"places": [
					{"id": "p1", "initial": 0},
					{"id": "p2", "initial": 0}
				],
				"transitions": [
					{"id": "blocked_transition"}
				],
				"arcs": [
					{"from": "p1", "to": "blocked_transition"},
					{"from": "blocked_transition", "to": "p2"}
				]
			}`,
			steps: []SimulationStep{
				{Transition: "blocked_transition"},
			},
			expectSuccess:   false,
			expectStepCount: 1,
			checkSteps: func(t *testing.T, steps []StepResult) {
				if steps[0].Enabled {
					t.Error("Expected transition to be disabled")
				}
				if steps[0].Error == "" {
					t.Error("Expected error message for disabled transition")
				}
				if !contains(steps[0].Error, "insufficient tokens") {
					t.Errorf("Expected 'insufficient tokens' in error, got: %s", steps[0].Error)
				}
				// State should be unchanged
				if steps[0].StateBefore["p1"] != steps[0].StateAfter["p1"] {
					t.Error("State should be unchanged for disabled transition")
				}
			},
		},
		{
			name: "mixed success and failure",
			modelJSON: `{
				"name": "partial",
				"places": [
					{"id": "p1", "initial": 1},
					{"id": "p2", "initial": 0},
					{"id": "p3", "initial": 0}
				],
				"transitions": [
					{"id": "enabled"},
					{"id": "disabled"}
				],
				"arcs": [
					{"from": "p1", "to": "enabled"},
					{"from": "enabled", "to": "p2"},
					{"from": "p3", "to": "disabled"},
					{"from": "disabled", "to": "p2"}
				]
			}`,
			steps: []SimulationStep{
				{Transition: "enabled"},
				{Transition: "disabled"},
			},
			expectSuccess:   false,
			expectStepCount: 2,
			checkSteps: func(t *testing.T, steps []StepResult) {
				// First step should succeed
				if !steps[0].Enabled {
					t.Error("Expected first transition to be enabled")
				}
				if steps[0].Error != "" {
					t.Errorf("Expected no error for first step, got: %s", steps[0].Error)
				}

				// Second step should fail
				if steps[1].Enabled {
					t.Error("Expected second transition to be disabled")
				}
				if steps[1].Error == "" {
					t.Error("Expected error for disabled second transition")
				}
			},
		},
		{
			name: "transition with bindings",
			modelJSON: `{
				"name": "with-bindings",
				"places": [
					{"id": "start", "initial": 1},
					{"id": "end", "initial": 0}
				],
				"transitions": [
					{"id": "transfer"}
				],
				"arcs": [
					{"from": "start", "to": "transfer"},
					{"from": "transfer", "to": "end"}
				]
			}`,
			steps: []SimulationStep{
				{
					Transition: "transfer",
					Bindings: map[string]any{
						"amount": 100,
						"from":   "alice",
						"to":     "bob",
					},
				},
			},
			expectSuccess:   true,
			expectStepCount: 1,
			checkSteps: func(t *testing.T, steps []StepResult) {
				if !steps[0].Enabled {
					t.Error("Expected transition to be enabled")
				}
				if steps[0].StateAfter["end"] != 1 {
					t.Errorf("Expected end=1, got %d", steps[0].StateAfter["end"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with new steps API
			stepsJSON, _ := json.Marshal(tt.steps)
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]interface{}{
						"model": tt.modelJSON,
						"steps": string(stepsJSON),
					},
				},
			}

			// Call handler
			result, err := handleSimulateWithSteps(context.Background(), request)
			if err != nil {
				t.Fatalf("handleSimulateWithSteps returned error: %v", err)
			}

			if result.IsError {
				t.Fatalf("Expected success but got error: %v", result.Content[0])
			}

			// Parse result
			var simResult SimulationResult
			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatalf("Expected TextContent, got %T", result.Content[0])
			}

			if err := json.Unmarshal([]byte(textContent.Text), &simResult); err != nil {
				t.Fatalf("Failed to parse result JSON: %v\nJSON: %s", err, textContent.Text)
			}

			// Verify expectations
			if simResult.Success != tt.expectSuccess {
				t.Errorf("Expected success=%v, got %v", tt.expectSuccess, simResult.Success)
			}

			if len(simResult.Steps) != tt.expectStepCount {
				t.Errorf("Expected %d steps, got %d", tt.expectStepCount, len(simResult.Steps))
			}

			// Check detailed step results
			if tt.checkSteps != nil {
				tt.checkSteps(t, simResult.Steps)
			}

			// Verify all steps have state_before and state_after
			for i, step := range simResult.Steps {
				if step.StateBefore == nil {
					t.Errorf("Step %d missing state_before", i)
				}
				if step.StateAfter == nil {
					t.Errorf("Step %d missing state_after", i)
				}
			}

			t.Logf("Simulation completed: success=%v, steps=%d, final_state=%v",
				simResult.Success, len(simResult.Steps), simResult.FinalState)
		})
	}
}

func TestSimulateBackwardsCompatibility(t *testing.T) {
	// Test that old "transitions" parameter still works
	modelJSON := `{
		"name": "simple",
		"places": [
			{"id": "start", "initial": 1},
			{"id": "end", "initial": 0}
		],
		"transitions": [
			{"id": "go"}
		],
		"arcs": [
			{"from": "start", "to": "go"},
			{"from": "go", "to": "end"}
		]
	}`

	transitions := []string{"go"}
	transitionsJSON, _ := json.Marshal(transitions)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"model":       modelJSON,
				"transitions": string(transitionsJSON),
			},
		},
	}

	result, err := handleSimulateWithSteps(context.Background(), request)
	if err != nil {
		t.Fatalf("handleSimulateWithSteps returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Expected success but got error: %v", result.Content[0])
	}

	// Parse result and verify it has both old and new fields
	var simResult SimulationResult
	textContent := result.Content[0].(mcp.TextContent)
	if err := json.Unmarshal([]byte(textContent.Text), &simResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Check new fields
	if len(simResult.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(simResult.Steps))
	}
	if simResult.FinalState["end"] != 1 {
		t.Errorf("Expected end=1 in final_state, got %d", simResult.FinalState["end"])
	}

	// Check old fields for backwards compatibility
	if len(simResult.Fired) != 1 {
		t.Errorf("Expected 1 fired transition, got %d", len(simResult.Fired))
	}
	if simResult.FinalMarking["end"] != 1 {
		t.Errorf("Expected end=1 in final_marking, got %d", simResult.FinalMarking["end"])
	}
	if simResult.InitialMarking["start"] != 1 {
		t.Errorf("Expected start=1 in initial_marking, got %d", simResult.InitialMarking["start"])
	}

	t.Log("Backwards compatibility test passed")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
