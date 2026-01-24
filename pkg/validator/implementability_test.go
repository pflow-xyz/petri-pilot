package validator_test

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
)

func TestValidateImplementability_ValidModel(t *testing.T) {
	model := &metamodel.Model{
		Name: "order-processing",
		Places: []metamodel.Place{
			{ID: "received", Initial: 1},
			{ID: "validated", Initial: 0},
			{ID: "shipped", Initial: 0},
		},
		Transitions: []metamodel.Transition{
			{ID: "validate"},
			{ID: "ship"},
		},
		Arcs: []metamodel.Arc{
			{From: "received", To: "validate"},
			{From: "validate", To: "validated"},
			{From: "validated", To: "ship"},
			{From: "ship", To: "shipped"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	if !result.Implementable {
		t.Errorf("Expected model to be implementable, got errors: %v", result.Errors)
	}

	// Should have event mappings for each transition
	if len(result.EventMappings) != 2 {
		t.Errorf("Expected 2 event mappings, got %d", len(result.EventMappings))
	}

	// Should have state mappings for each place
	if len(result.StateMappings) != 3 {
		t.Errorf("Expected 3 state mappings, got %d", len(result.StateMappings))
	}

	// Should detect workflow pattern
	if result.Pattern.Type != "workflow" {
		t.Errorf("Expected workflow pattern, got %s", result.Pattern.Type)
	}
}

func TestValidateImplementability_DataPlaceWithoutType(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "ready", Initial: 1},
			{ID: "data", Kind: metamodel.DataKind}, // No type specified
		},
		Transitions: []metamodel.Transition{
			{ID: "process"},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "process"},
			{From: "data", To: "process"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	// Should still be implementable but with warning
	if !result.Implementable {
		t.Error("Expected model to be implementable with warnings")
	}

	// Should have warning about untyped data place
	found := false
	for _, w := range result.Warnings {
		if w.Code == "UNTYPED_DATA_PLACE" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected UNTYPED_DATA_PLACE warning")
	}
}

func TestValidateImplementability_InvalidIdentifier(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "123-invalid", Initial: 1}, // Invalid identifier
		},
		Transitions: []metamodel.Transition{
			{ID: "process"},
		},
		Arcs: []metamodel.Arc{
			{From: "123-invalid", To: "process"},
			{From: "process", To: "123-invalid"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	// Should have error about invalid field name
	found := false
	for _, e := range result.Errors {
		if e.Code == "INVALID_FIELD_NAME" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected INVALID_FIELD_NAME error")
	}
}

func TestValidateImplementability_ComplexGuard(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "ready", Initial: 1},
		},
		Transitions: []metamodel.Transition{
			{ID: "process", Guard: "balance[from] >= amount && balance[to] + amount <= maxBalance"},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "process"},
			{From: "process", To: "ready"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	// Should have warning about complex guard
	found := false
	for _, w := range result.Warnings {
		if w.Code == "COMPLEX_GUARD" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected COMPLEX_GUARD warning")
	}
}

func TestValidateImplementability_SimpleGuard(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "tokens", Initial: 5},
		},
		Transitions: []metamodel.Transition{
			{ID: "consume", Guard: "tokens > 0"},
		},
		Arcs: []metamodel.Arc{
			{From: "tokens", To: "consume"},
			{From: "consume", To: "tokens"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	// Should NOT have warning about complex guard
	for _, w := range result.Warnings {
		if w.Code == "COMPLEX_GUARD" {
			t.Error("Unexpected COMPLEX_GUARD warning for simple guard")
		}
	}
}

func TestValidateImplementability_InvalidHTTPMethod(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "ready", Initial: 1},
		},
		Transitions: []metamodel.Transition{
			{ID: "process", HTTPMethod: "INVALID"},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "process"},
			{From: "process", To: "ready"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	// Should have warning about invalid HTTP method
	found := false
	for _, w := range result.Warnings {
		if w.Code == "INVALID_HTTP_METHOD" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected INVALID_HTTP_METHOD warning")
	}
}

func TestPatternDetection_Workflow(t *testing.T) {
	// Linear workflow: start -> a -> b -> c -> end
	model := &metamodel.Model{
		Name: "linear-workflow",
		Places: []metamodel.Place{
			{ID: "start", Initial: 1},
			{ID: "step1", Initial: 0},
			{ID: "step2", Initial: 0},
			{ID: "end", Initial: 0},
		},
		Transitions: []metamodel.Transition{
			{ID: "do_step1"},
			{ID: "do_step2"},
			{ID: "finish"},
		},
		Arcs: []metamodel.Arc{
			{From: "start", To: "do_step1"},
			{From: "do_step1", To: "step1"},
			{From: "step1", To: "do_step2"},
			{From: "do_step2", To: "step2"},
			{From: "step2", To: "finish"},
			{From: "finish", To: "end"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	if result.Pattern.Type != "workflow" {
		t.Errorf("Expected workflow pattern, got %s", result.Pattern.Type)
	}
}

func TestPatternDetection_StateMachine(t *testing.T) {
	// State machine with cycles: idle <-> active
	model := &metamodel.Model{
		Name: "state-machine",
		Places: []metamodel.Place{
			{ID: "idle", Initial: 1},
			{ID: "active", Initial: 0},
		},
		Transitions: []metamodel.Transition{
			{ID: "activate"},
			{ID: "deactivate"},
		},
		Arcs: []metamodel.Arc{
			{From: "idle", To: "activate"},
			{From: "activate", To: "active"},
			{From: "active", To: "deactivate"},
			{From: "deactivate", To: "idle"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	if result.Pattern.Type != "state_machine" {
		t.Errorf("Expected state_machine pattern, got %s", result.Pattern.Type)
	}
}

func TestPatternDetection_ResourcePool(t *testing.T) {
	// Resource pool: data state with cycles (like token balances)
	model := &metamodel.Model{
		Name: "token-transfer",
		Places: []metamodel.Place{
			{ID: "ready", Initial: 1},
			{ID: "balances", Kind: metamodel.DataKind, Type: "map[string]int"},
		},
		Transitions: []metamodel.Transition{
			{ID: "transfer"},
		},
		Arcs: []metamodel.Arc{
			{From: "ready", To: "transfer"},
			{From: "transfer", To: "ready"},
			{From: "balances", To: "transfer", Keys: []string{"from"}, Value: "amount"},
			{From: "transfer", To: "balances", Keys: []string{"to"}, Value: "amount"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	if result.Pattern.Type != "resource_pool" {
		t.Errorf("Expected resource_pool pattern, got %s", result.Pattern.Type)
	}
}

func TestEventMapping(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "a", Initial: 1},
			{ID: "b", Initial: 0},
		},
		Transitions: []metamodel.Transition{
			{ID: "transfer", EventType: "TransferCompleted"},
		},
		Arcs: []metamodel.Arc{
			{From: "a", To: "transfer", Keys: []string{"from"}, Value: "amount"},
			{From: "transfer", To: "b", Keys: []string{"to"}},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	if len(result.EventMappings) != 1 {
		t.Fatalf("Expected 1 event mapping, got %d", len(result.EventMappings))
	}

	mapping := result.EventMappings[0]
	if mapping.TransitionID != "transfer" {
		t.Errorf("Expected transition ID 'transfer', got '%s'", mapping.TransitionID)
	}
	if mapping.EventType != "TransferCompleted" {
		t.Errorf("Expected event type 'TransferCompleted', got '%s'", mapping.EventType)
	}

	// Should include fields from arc bindings
	hasFrom := false
	hasTo := false
	hasAmount := false
	for _, f := range mapping.Fields {
		switch f {
		case "from":
			hasFrom = true
		case "to":
			hasTo = true
		case "amount":
			hasAmount = true
		}
	}
	if !hasFrom || !hasTo || !hasAmount {
		t.Errorf("Missing expected fields. Has from=%v, to=%v, amount=%v", hasFrom, hasTo, hasAmount)
	}
}

func TestStateMapping(t *testing.T) {
	model := &metamodel.Model{
		Name: "test",
		Places: []metamodel.Place{
			{ID: "token_count", Initial: 5},
			{ID: "user_data", Kind: metamodel.DataKind, Type: "map[string]User"},
		},
		Transitions: []metamodel.Transition{
			{ID: "process"},
		},
		Arcs: []metamodel.Arc{
			{From: "token_count", To: "process"},
			{From: "user_data", To: "process"},
		},
	}

	v := validator.New(validator.DefaultOptions())
	result := v.ValidateImplementability(model)

	if len(result.StateMappings) != 2 {
		t.Fatalf("Expected 2 state mappings, got %d", len(result.StateMappings))
	}

	// Check token place mapping
	var tokenMapping, dataMapping *validator.StateMapping
	for i := range result.StateMappings {
		if result.StateMappings[i].PlaceID == "token_count" {
			tokenMapping = &result.StateMappings[i]
		}
		if result.StateMappings[i].PlaceID == "user_data" {
			dataMapping = &result.StateMappings[i]
		}
	}

	if tokenMapping == nil {
		t.Fatal("Missing token_count mapping")
	}
	if !tokenMapping.IsToken {
		t.Error("Expected token_count to be marked as token")
	}
	if tokenMapping.FieldType != "int" {
		t.Errorf("Expected token field type 'int', got '%s'", tokenMapping.FieldType)
	}

	if dataMapping == nil {
		t.Fatal("Missing user_data mapping")
	}
	if dataMapping.IsToken {
		t.Error("Expected user_data to NOT be marked as token")
	}
	if dataMapping.FieldType != "map[string]User" {
		t.Errorf("Expected data field type 'map[string]User', got '%s'", dataMapping.FieldType)
	}
}
