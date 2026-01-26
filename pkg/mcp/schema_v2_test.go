package mcp

import (
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

func TestParseModelV2(t *testing.T) {
	// v2 schema with nested net and extensions
	v2JSON := `{
		"version": "2.0",
		"net": {
			"name": "order-workflow",
			"places": [
				{"id": "pending", "initial": 1},
				{"id": "completed"}
			],
			"transitions": [
				{"id": "complete"}
			],
			"arcs": [
				{"from": "pending", "to": "complete"},
				{"from": "complete", "to": "completed"}
			]
		},
		"extensions": {
			"petri-pilot/roles": [
				{"id": "admin", "name": "Administrator"},
				{"id": "user", "name": "User"}
			],
			"petri-pilot/views": [
				{"id": "order-list", "name": "Orders", "kind": "table"}
			]
		}
	}`

	result, err := parseModelV2(v2JSON)
	if err != nil {
		t.Fatalf("Failed to parse v2 schema: %v", err)
	}

	if result.Version != "2.0" {
		t.Errorf("Expected version 2.0, got %s", result.Version)
	}

	if result.Model.Name != "order-workflow" {
		t.Errorf("Expected name 'order-workflow', got %s", result.Model.Name)
	}

	if len(result.Model.Places) != 2 {
		t.Errorf("Expected 2 places, got %d", len(result.Model.Places))
	}

	if len(result.Extensions) != 2 {
		t.Errorf("Expected 2 extensions, got %d", len(result.Extensions))
	}

	// Verify extension keys
	if _, ok := result.Extensions["petri-pilot/roles"]; !ok {
		t.Error("Expected petri-pilot/roles extension")
	}
	if _, ok := result.Extensions["petri-pilot/views"]; !ok {
		t.Error("Expected petri-pilot/views extension")
	}
}

func TestParseModelV2FallbackToV1(t *testing.T) {
	// v1 schema (flat format without version)
	v1JSON := `{
		"name": "simple-workflow",
		"places": [
			{"id": "start", "initial": 1},
			{"id": "end"}
		],
		"transitions": [
			{"id": "go"}
		],
		"arcs": [
			{"from": "start", "to": "go"},
			{"from": "go", "to": "end"}
		]
	}`

	result, err := parseModelV2(v1JSON)
	if err != nil {
		t.Fatalf("Failed to parse v1 schema: %v", err)
	}

	if result.Version != "1.0" {
		t.Errorf("Expected version 1.0 for v1 schema, got %s", result.Version)
	}

	if result.Model.Name != "simple-workflow" {
		t.Errorf("Expected name 'simple-workflow', got %s", result.Model.Name)
	}

	if result.Extensions != nil {
		t.Errorf("Expected no extensions for v1 schema, got %d", len(result.Extensions))
	}
}

func TestParseV2Extensions(t *testing.T) {
	v2JSON := `{
		"version": "2.0",
		"net": {
			"name": "test-workflow",
			"places": [{"id": "start", "initial": 1}],
			"transitions": [{"id": "go"}],
			"arcs": [{"from": "start", "to": "go"}]
		},
		"extensions": {
			"petri-pilot/roles": [
				{"id": "admin", "name": "Administrator"},
				{"id": "user", "name": "User", "inherits": ["admin"]}
			]
		}
	}`

	result, err := parseModelV2(v2JSON)
	if err != nil {
		t.Fatalf("Failed to parse v2 schema: %v", err)
	}

	app := extensions.NewApplicationSpec(result.Model)
	if err := parseV2Extensions(app, result.Extensions); err != nil {
		t.Fatalf("Failed to parse v2 extensions: %v", err)
	}

	roles := app.Roles()
	if roles == nil {
		t.Fatal("Expected roles extension")
	}

	if len(roles.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(roles.Roles))
	}

	admin := roles.RoleByID("admin")
	if admin == nil {
		t.Fatal("Expected admin role")
	}
	if admin.Name != "Administrator" {
		t.Errorf("Expected admin name 'Administrator', got %s", admin.Name)
	}

	user := roles.RoleByID("user")
	if user == nil {
		t.Fatal("Expected user role")
	}
	if len(user.Inherits) != 1 || user.Inherits[0] != "admin" {
		t.Errorf("Expected user to inherit from admin, got %v", user.Inherits)
	}
}
