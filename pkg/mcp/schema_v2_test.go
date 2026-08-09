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

func TestParseModelV2ComputationNetEnvelope(t *testing.T) {
	// The frontends ship ComputationNet documents at version 2.1. These used
	// to fail the exact-"2.0" check, fall through to v1 parsing, and come
	// back as a "valid" model with zero places.
	doc := `{
		"@context": "https://pflow.xyz/schema",
		"@type": "ComputationNet",
		"@id": "vet-clinic",
		"version": "2.1",
		"net": {
			"name": "vet-clinic",
			"places": [
				{"id": "wait_exam", "initial": 0},
				{"id": "dvm_avail", "initial": 2}
			],
			"transitions": [{"id": "start_wellness"}],
			"arcs": [
				{"from": "wait_exam", "to": "start_wellness"},
				{"from": "dvm_avail", "to": "start_wellness"}
			]
		}
	}`

	result, err := parseModelV2(doc)
	if err != nil {
		t.Fatalf("Failed to parse ComputationNet envelope: %v", err)
	}
	if result.Version != "2.1" {
		t.Errorf("Expected version 2.1, got %s", result.Version)
	}
	if result.Model.Name != "vet-clinic" {
		t.Errorf("Expected name 'vet-clinic', got %s", result.Model.Name)
	}
	if len(result.Model.Places) != 2 {
		t.Errorf("Expected 2 places, got %d", len(result.Model.Places))
	}
}

func TestParseModelV2EnvelopeWithBadNetErrors(t *testing.T) {
	// A present-but-unparseable net must be a loud error, never a
	// fallthrough to v1 that reports an empty model as valid.
	doc := `{"version": "2.1", "net": "not a model"}`
	if _, err := parseModelV2(doc); err == nil {
		t.Fatal("Expected error for envelope with unparseable net, got nil")
	}
}

func TestMigrateV1ToV2(t *testing.T) {
	// v1 schema with extensions in flat format
	v1JSON := `{
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
		],
		"roles": [
			{"id": "admin", "name": "Administrator"}
		],
		"views": [
			{"id": "order-list", "name": "Orders", "kind": "table"}
		]
	}`

	// Simulate what handleMigrate does
	parsed, err := parseModelV2(v1JSON)
	if err != nil {
		t.Fatalf("Failed to parse v1 schema: %v", err)
	}

	if parsed.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", parsed.Version)
	}

	// Verify model was extracted correctly
	if parsed.Model.Name != "order-workflow" {
		t.Errorf("Expected name 'order-workflow', got %s", parsed.Model.Name)
	}

	if len(parsed.Model.Places) != 2 {
		t.Errorf("Expected 2 places, got %d", len(parsed.Model.Places))
	}
}
