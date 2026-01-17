package mcp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

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
