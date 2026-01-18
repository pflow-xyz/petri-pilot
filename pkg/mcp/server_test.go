package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

func TestSimulate(t *testing.T) {
	tests := []struct {
		name              string
		modelJSON         string
		transitions       []string
		expectSuccess     bool
		expectFiredCount  int
		expectFailedCount int
		expectDeadlock    bool
		checkFinalMarking func(t *testing.T, marking map[string]int)
	}{
		{
			name: "successful transition sequence",
			modelJSON: `{
				"name": "simple-workflow",
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
			transitions:       []string{"step1", "step2"},
			expectSuccess:     true,
			expectFiredCount:  2,
			expectFailedCount: 0,
			expectDeadlock:    true,
			checkFinalMarking: func(t *testing.T, marking map[string]int) {
				if marking["end"] != 1 {
					t.Errorf("Expected end=1, got %d", marking["end"])
				}
				if marking["start"] != 0 {
					t.Errorf("Expected start=0, got %d", marking["start"])
				}
			},
		},
		{
			name: "transition not enabled",
			modelJSON: `{
				"name": "blocked-workflow",
				"places": [
					{"id": "p1", "initial": 0},
					{"id": "p2", "initial": 0}
				],
				"transitions": [
					{"id": "t1"}
				],
				"arcs": [
					{"from": "p1", "to": "t1"},
					{"from": "t1", "to": "p2"}
				]
			}`,
			transitions:       []string{"t1"},
			expectSuccess:     false,
			expectFiredCount:  0,
			expectFailedCount: 1,
			expectDeadlock:    true,
			checkFinalMarking: func(t *testing.T, marking map[string]int) {
				if marking["p1"] != 0 {
					t.Errorf("Expected p1=0, got %d", marking["p1"])
				}
			},
		},
		{
			name: "partial success",
			modelJSON: `{
				"name": "partial-workflow",
				"places": [
					{"id": "start", "initial": 1},
					{"id": "middle", "initial": 0},
					{"id": "blocked", "initial": 0}
				],
				"transitions": [
					{"id": "valid"},
					{"id": "invalid"}
				],
				"arcs": [
					{"from": "start", "to": "valid"},
					{"from": "valid", "to": "middle"},
					{"from": "blocked", "to": "invalid"},
					{"from": "invalid", "to": "middle"}
				]
			}`,
			transitions:       []string{"valid", "invalid"},
			expectSuccess:     false,
			expectFiredCount:  1,
			expectFailedCount: 1,
			expectDeadlock:    true,
			checkFinalMarking: func(t *testing.T, marking map[string]int) {
				if marking["middle"] != 1 {
					t.Errorf("Expected middle=1, got %d", marking["middle"])
				}
			},
		},
		{
			name: "transition not found",
			modelJSON: `{
				"name": "missing-transition",
				"places": [
					{"id": "p1", "initial": 1}
				],
				"transitions": [],
				"arcs": []
			}`,
			transitions:       []string{"nonexistent"},
			expectSuccess:     false,
			expectFiredCount:  0,
			expectFailedCount: 1,
			expectDeadlock:    true,
			checkFinalMarking: func(t *testing.T, marking map[string]int) {
				if marking["p1"] != 1 {
					t.Errorf("Expected p1=1, got %d", marking["p1"])
				}
			},
		},
		{
			name: "no deadlock - enabled transitions remain",
			modelJSON: `{
				"name": "cyclic-workflow",
				"places": [
					{"id": "p1", "initial": 1},
					{"id": "p2", "initial": 0}
				],
				"transitions": [
					{"id": "forward"},
					{"id": "backward"}
				],
				"arcs": [
					{"from": "p1", "to": "forward"},
					{"from": "forward", "to": "p2"},
					{"from": "p2", "to": "backward"},
					{"from": "backward", "to": "p1"}
				]
			}`,
			transitions:       []string{"forward"},
			expectSuccess:     true,
			expectFiredCount:  1,
			expectFailedCount: 0,
			expectDeadlock:    false,
			checkFinalMarking: func(t *testing.T, marking map[string]int) {
				if marking["p2"] != 1 {
					t.Errorf("Expected p2=1, got %d", marking["p2"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			transitionsJSON, _ := json.Marshal(tt.transitions)
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]interface{}{
						"model":       tt.modelJSON,
						"transitions": string(transitionsJSON),
					},
				},
			}

			// Call handler
			result, err := handleSimulate(context.Background(), request)
			if err != nil {
				t.Fatalf("handleSimulate returned error: %v", err)
			}

			// Parse result
			if len(result.Content) == 0 {
				t.Fatal("No content in result")
			}

			if result.IsError {
				t.Fatalf("Expected success but got error: %v", result.Content[0])
			}

			var simResult struct {
				Success        bool              `json:"success"`
				InitialMarking map[string]int    `json:"initial_marking"`
				FinalMarking   map[string]int    `json:"final_marking"`
				Fired          []string          `json:"fired"`
				Failed         []json.RawMessage `json:"failed"`
				IsDeadlock     bool              `json:"is_deadlock"`
				Enabled        []string          `json:"enabled"`
			}

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

			if len(simResult.Fired) != tt.expectFiredCount {
				t.Errorf("Expected %d fired transitions, got %d: %v", tt.expectFiredCount, len(simResult.Fired), simResult.Fired)
			}

			if len(simResult.Failed) != tt.expectFailedCount {
				t.Errorf("Expected %d failed transitions, got %d", tt.expectFailedCount, len(simResult.Failed))
			}

			if simResult.IsDeadlock != tt.expectDeadlock {
				t.Errorf("Expected is_deadlock=%v, got %v (enabled: %v)", tt.expectDeadlock, simResult.IsDeadlock, simResult.Enabled)
			}

			// Check final marking
			if tt.checkFinalMarking != nil {
				tt.checkFinalMarking(t, simResult.FinalMarking)
			}

			t.Logf("Simulation result: fired=%v, failed=%d, deadlock=%v, enabled=%v",
				simResult.Fired, len(simResult.Failed), simResult.IsDeadlock, simResult.Enabled)
		})
	}
}

func TestSimulateWithBlogPostExample(t *testing.T) {
	// Load the blog-post example
	data, err := os.ReadFile("../../examples/blog-post.json")
	if err != nil {
		t.Fatalf("Failed to read blog-post example: %v", err)
	}

	// Test a valid workflow path
	transitions := []string{"submit", "approve"}
	transitionsJSON, _ := json.Marshal(transitions)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"model":       string(data),
				"transitions": string(transitionsJSON),
			},
		},
	}

	result, err := handleSimulate(context.Background(), request)
	if err != nil {
		t.Fatalf("handleSimulate returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Expected success but got error: %v", result.Content[0])
	}

	var simResult struct {
		Success        bool           `json:"success"`
		InitialMarking map[string]int `json:"initial_marking"`
		FinalMarking   map[string]int `json:"final_marking"`
		Fired          []string       `json:"fired"`
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", result.Content[0])
	}
	
	if err := json.Unmarshal([]byte(textContent.Text), &simResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Verify the workflow executed correctly
	if !simResult.Success {
		t.Error("Expected successful simulation")
	}

	if len(simResult.Fired) != 2 {
		t.Errorf("Expected 2 transitions fired, got %d", len(simResult.Fired))
	}

	// Check that we ended up in the published state
	if simResult.FinalMarking["published"] != 1 {
		t.Errorf("Expected published=1, got %d", simResult.FinalMarking["published"])
	}

	t.Logf("Blog post workflow simulation successful: %v -> published", simResult.Fired)
}

func TestApplicationSpec(t *testing.T) {
	// Load example Application spec
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	// Verify it's valid JSON and parses as Application
	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}

	// Verify basic structure
	if app.Name != "task-manager" {
		t.Errorf("Expected name 'task-manager', got '%s'", app.Name)
	}

	if len(app.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(app.Entities))
	}

	if len(app.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(app.Roles))
	}

	if len(app.Pages) != 3 {
		t.Errorf("Expected 3 pages, got %d", len(app.Pages))
	}

	if len(app.Workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(app.Workflows))
	}

	// Verify entity has access rules
	task := app.Entities[0]
	if task.ID != "task" {
		t.Errorf("Expected entity ID 'task', got '%s'", task.ID)
	}

	if len(task.Access) == 0 {
		t.Error("Expected access rules on task entity")
	}

	// Verify access rules
	foundUserCreate := false
	foundAdminAssign := false
	for _, rule := range task.Access {
		if rule.Action == "create" {
			for _, role := range rule.Roles {
				if role == "user" {
					foundUserCreate = true
				}
			}
		}
		if rule.Action == "assign" {
			for _, role := range rule.Roles {
				if role == "admin" {
					foundAdminAssign = true
				}
			}
		}
	}

	if !foundUserCreate {
		t.Error("Expected 'user' role to have 'create' permission")
	}

	if !foundAdminAssign {
		t.Error("Expected 'admin' role to have 'assign' permission")
	}

	// Verify Entity.ToSchema() works
	entitySchema := task.ToSchema()
	if entitySchema == nil {
		t.Fatal("ToSchema() returned nil")
	}

	if entitySchema.Name != "task" {
		t.Errorf("Expected schema name 'task', got '%s'", entitySchema.Name)
	}

	t.Logf("Successfully parsed Application with %d entities, %d roles, %d pages, %d workflows",
		len(app.Entities), len(app.Roles), len(app.Pages), len(app.Workflows))
}
