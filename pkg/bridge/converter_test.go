package bridge_test

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

func TestToMetamodel(t *testing.T) {
	model := &schema.Model{
		Name:    "test-model",
		Version: "1.0.0",
		Places: []schema.Place{
			{ID: "pending", Initial: 1, Kind: schema.TokenKind},
			{ID: "completed", Initial: 0, Kind: schema.TokenKind},
			{ID: "data_store", Kind: schema.DataKind, Type: "map[string]int", Exported: true},
		},
		Transitions: []schema.Transition{
			{ID: "process", Guard: "pending > 0", EventType: "Processed"},
		},
		Arcs: []schema.Arc{
			{From: "pending", To: "process"},
			{From: "process", To: "completed"},
			{From: "data_store", To: "process", Keys: []string{"key"}, Value: "value"},
		},
		Constraints: []schema.Constraint{
			{ID: "conservation", Expr: "pending + completed == 1"},
		},
	}

	meta := bridge.ToMetamodel(model)

	// Check basic fields
	if meta.Name != "test-model" {
		t.Errorf("expected name 'test-model', got '%s'", meta.Name)
	}
	if meta.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", meta.Version)
	}

	// Check states
	if len(meta.States) != 3 {
		t.Errorf("expected 3 states, got %d", len(meta.States))
	}

	pending := meta.StateByID("pending")
	if pending == nil {
		t.Fatal("pending state not found")
	}
	if pending.Kind != metamodel.TokenState {
		t.Errorf("expected pending to be token state")
	}
	if pending.InitialTokens() != 1 {
		t.Errorf("expected pending initial tokens 1, got %d", pending.InitialTokens())
	}

	dataStore := meta.StateByID("data_store")
	if dataStore == nil {
		t.Fatal("data_store state not found")
	}
	if dataStore.Kind != metamodel.DataState {
		t.Errorf("expected data_store to be data state")
	}
	if !dataStore.Exported {
		t.Error("expected data_store to be exported")
	}
	if dataStore.Type != "map[string]int" {
		t.Errorf("expected type 'map[string]int', got '%s'", dataStore.Type)
	}

	// Check actions
	if len(meta.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(meta.Actions))
	}

	process := meta.ActionByID("process")
	if process == nil {
		t.Fatal("process action not found")
	}
	if process.Guard != "pending > 0" {
		t.Errorf("expected guard 'pending > 0', got '%s'", process.Guard)
	}
	if process.EventID != "Processed" {
		t.Errorf("expected event ID 'Processed', got '%s'", process.EventID)
	}

	// Check arcs
	if len(meta.Arcs) != 3 {
		t.Errorf("expected 3 arcs, got %d", len(meta.Arcs))
	}

	// Check constraint
	if len(meta.Constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(meta.Constraints))
	}
}

func TestFromMetamodel(t *testing.T) {
	meta := metamodel.NewSchema("test-schema")
	meta.Version = "2.0.0"
	meta.AddTokenState("ready", 1)
	meta.AddDataState("balances", "map[address]uint256", nil, true)
	meta.AddAction(metamodel.Action{ID: "transfer", Guard: "balances[from] >= amount"})
	meta.AddArc(metamodel.Arc{Source: "ready", Target: "transfer"})
	meta.AddArc(metamodel.Arc{Source: "balances", Target: "transfer", Keys: []string{"from"}, Value: "amount"})
	meta.AddConstraint(metamodel.Constraint{ID: "total", Expr: "sum(balances) == totalSupply"})

	model := bridge.FromMetamodel(meta)

	if model.Name != "test-schema" {
		t.Errorf("expected name 'test-schema', got '%s'", model.Name)
	}
	if model.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", model.Version)
	}

	if len(model.Places) != 2 {
		t.Errorf("expected 2 places, got %d", len(model.Places))
	}

	if len(model.Transitions) != 1 {
		t.Errorf("expected 1 transition, got %d", len(model.Transitions))
	}

	if len(model.Arcs) != 2 {
		t.Errorf("expected 2 arcs, got %d", len(model.Arcs))
	}

	if len(model.Constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(model.Constraints))
	}
}

func TestRoundTrip(t *testing.T) {
	original := &schema.Model{
		Name:    "roundtrip-test",
		Version: "1.0.0",
		Places: []schema.Place{
			{ID: "start", Initial: 1, Kind: schema.TokenKind},
			{ID: "end", Initial: 0, Kind: schema.TokenKind},
		},
		Transitions: []schema.Transition{
			{ID: "go", Guard: "start > 0"},
		},
		Arcs: []schema.Arc{
			{From: "start", To: "go"},
			{From: "go", To: "end"},
		},
	}

	// Convert to metamodel and back
	meta := bridge.ToMetamodel(original)
	result := bridge.FromMetamodel(meta)

	if result.Name != original.Name {
		t.Errorf("name mismatch: %s vs %s", result.Name, original.Name)
	}
	if len(result.Places) != len(original.Places) {
		t.Errorf("place count mismatch: %d vs %d", len(result.Places), len(original.Places))
	}
	if len(result.Transitions) != len(original.Transitions) {
		t.Errorf("transition count mismatch: %d vs %d", len(result.Transitions), len(original.Transitions))
	}
	if len(result.Arcs) != len(original.Arcs) {
		t.Errorf("arc count mismatch: %d vs %d", len(result.Arcs), len(original.Arcs))
	}
}

func TestEnrichModel(t *testing.T) {
	model := &schema.Model{
		Name: "simple",
		Places: []schema.Place{
			{ID: "ready", Initial: 1},
		},
		Transitions: []schema.Transition{
			{ID: "validate_order"},
			{ID: "ship"},
		},
		Arcs: []schema.Arc{
			{From: "ready", To: "validate_order"},
			{From: "validate_order", To: "ready"},
		},
	}

	enriched := bridge.EnrichModel(model)

	// Check default kind
	if enriched.Places[0].Kind != schema.TokenKind {
		t.Error("expected default kind to be token")
	}

	// Check inferred event types
	if enriched.Transitions[0].EventType != "ValidateOrdered" {
		t.Errorf("expected event type 'ValidateOrdered', got '%s'", enriched.Transitions[0].EventType)
	}

	// Check inferred HTTP paths
	if enriched.Transitions[0].HTTPPath != "/api/validate_order" {
		t.Errorf("expected HTTP path '/api/validate_order', got '%s'", enriched.Transitions[0].HTTPPath)
	}
	if enriched.Transitions[0].HTTPMethod != "POST" {
		t.Errorf("expected HTTP method 'POST', got '%s'", enriched.Transitions[0].HTTPMethod)
	}
}

func TestValidateForCodegen(t *testing.T) {
	t.Run("ValidModel", func(t *testing.T) {
		model := &schema.Model{
			Name: "valid",
			Places: []schema.Place{
				{ID: "a", Initial: 1},
			},
			Transitions: []schema.Transition{
				{ID: "t"},
			},
			Arcs: []schema.Arc{
				{From: "a", To: "t"},
				{From: "t", To: "a"},
			},
		}

		issues := bridge.ValidateForCodegen(model)
		if len(issues) != 0 {
			t.Errorf("expected no issues, got: %v", issues)
		}
	})

	t.Run("EmptyModel", func(t *testing.T) {
		model := &schema.Model{}

		issues := bridge.ValidateForCodegen(model)
		if len(issues) < 3 {
			t.Errorf("expected at least 3 issues, got %d: %v", len(issues), issues)
		}
	})

	t.Run("UnconnectedPlace", func(t *testing.T) {
		model := &schema.Model{
			Name: "test",
			Places: []schema.Place{
				{ID: "connected", Initial: 1},
				{ID: "orphan", Initial: 0},
			},
			Transitions: []schema.Transition{
				{ID: "t"},
			},
			Arcs: []schema.Arc{
				{From: "connected", To: "t"},
			},
		}

		issues := bridge.ValidateForCodegen(model)
		found := false
		for _, issue := range issues {
			if issue == "place 'orphan' has no connections" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected orphan place issue, got: %v", issues)
		}
	})

	t.Run("DataPlaceWithoutType", func(t *testing.T) {
		model := &schema.Model{
			Name: "test",
			Places: []schema.Place{
				{ID: "data", Kind: schema.DataKind},
			},
			Transitions: []schema.Transition{
				{ID: "t"},
			},
			Arcs: []schema.Arc{
				{From: "data", To: "t"},
			},
		}

		issues := bridge.ValidateForCodegen(model)
		found := false
		for _, issue := range issues {
			if issue == "data place 'data' needs a type" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected data type issue, got: %v", issues)
		}
	})
}

func TestInferAPIRoutes(t *testing.T) {
	model := &schema.Model{
		Transitions: []schema.Transition{
			{ID: "create_order", Description: "Create a new order"},
			{ID: "confirm", HTTPMethod: "PUT", HTTPPath: "/orders/{id}/confirm"},
		},
	}

	routes := bridge.InferAPIRoutes(model)

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	// Check inferred route
	if routes[0].Method != "POST" {
		t.Errorf("expected POST, got %s", routes[0].Method)
	}
	if routes[0].Path != "/api/create_order" {
		t.Errorf("expected '/api/create_order', got '%s'", routes[0].Path)
	}

	// Check explicit route
	if routes[1].Method != "PUT" {
		t.Errorf("expected PUT, got %s", routes[1].Method)
	}
	if routes[1].Path != "/orders/{id}/confirm" {
		t.Errorf("expected '/orders/{id}/confirm', got '%s'", routes[1].Path)
	}
}

func TestInferEvents(t *testing.T) {
	model := &schema.Model{
		Places: []schema.Place{
			{ID: "balance", Kind: schema.DataKind},
		},
		Transitions: []schema.Transition{
			{ID: "transfer"},
		},
		Arcs: []schema.Arc{
			{From: "balance", To: "transfer", Keys: []string{"from"}, Value: "amount"},
			{From: "transfer", To: "balance", Keys: []string{"to"}, Value: "amount"},
		},
	}

	events := bridge.InferEvents(model)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Type != "Transfered" {
		t.Errorf("expected type 'Transfered', got '%s'", event.Type)
	}

	// Check fields were inferred
	fieldNames := make(map[string]bool)
	for _, f := range event.Fields {
		fieldNames[f.Name] = true
	}

	if !fieldNames["aggregate_id"] {
		t.Error("expected aggregate_id field")
	}
	if !fieldNames["from"] {
		t.Error("expected from field")
	}
	if !fieldNames["to"] {
		t.Error("expected to field")
	}
	if !fieldNames["amount"] {
		t.Error("expected amount field")
	}
}

func TestInferAggregateState(t *testing.T) {
	model := &schema.Model{
		Places: []schema.Place{
			{ID: "status", Initial: 1, Kind: schema.TokenKind},
			{ID: "items", Kind: schema.DataKind, Type: "[]string", Persisted: true},
		},
	}

	fields := bridge.InferAggregateState(model)

	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}

	// Check token field
	if fields[0].Name != "status" {
		t.Errorf("expected name 'status', got '%s'", fields[0].Name)
	}
	if fields[0].Type != "int" {
		t.Errorf("expected type 'int', got '%s'", fields[0].Type)
	}
	if !fields[0].IsToken {
		t.Error("expected status to be token")
	}

	// Check data field
	if fields[1].Name != "items" {
		t.Errorf("expected name 'items', got '%s'", fields[1].Name)
	}
	if fields[1].Type != "[]string" {
		t.Errorf("expected type '[]string', got '%s'", fields[1].Type)
	}
	if !fields[1].Persisted {
		t.Error("expected items to be persisted")
	}
}
