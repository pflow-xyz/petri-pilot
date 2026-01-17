// Package bridge provides conversion between petri-pilot schema and go-pflow metamodel.
package bridge

import (
	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// ToMetamodel converts a petri-pilot Model to a go-pflow Schema.
func ToMetamodel(model *schema.Model) *metamodel.Schema {
	s := metamodel.NewSchema(model.Name)
	s.Version = model.Version
	if s.Version == "" {
		s.Version = "1.0.0"
	}

	// Convert places to states
	for _, place := range model.Places {
		state := metamodel.State{
			ID:       place.ID,
			Exported: place.Exported,
		}

		if place.IsData() {
			state.Kind = metamodel.DataState
			state.Type = place.Type
			state.Initial = nil // Data states start empty unless specified
		} else {
			state.Kind = metamodel.TokenState
			state.Type = "int"
			state.Initial = place.Initial
		}

		s.AddState(state)
	}

	// Convert transitions to actions
	for _, transition := range model.Transitions {
		action := metamodel.Action{
			ID:            transition.ID,
			Guard:         transition.Guard,
			EventID:       transition.EventType,
			EventBindings: transition.Bindings,
		}
		s.AddAction(action)
	}

	// Convert arcs
	for _, arc := range model.Arcs {
		metaArc := metamodel.Arc{
			Source: arc.From,
			Target: arc.To,
			Keys:   arc.Keys,
			Value:  arc.Value,
		}

		// Default value binding for data arcs
		if metaArc.Value == "" && len(metaArc.Keys) > 0 {
			metaArc.Value = "amount"
		}

		s.AddArc(metaArc)
	}

	// Convert constraints
	for _, constraint := range model.Constraints {
		s.AddConstraint(metamodel.Constraint{
			ID:   constraint.ID,
			Expr: constraint.Expr,
		})
	}

	return s
}

// FromMetamodel converts a go-pflow Schema to a petri-pilot Model.
func FromMetamodel(s *metamodel.Schema) *schema.Model {
	model := &schema.Model{
		Name:    s.Name,
		Version: s.Version,
	}

	// Convert states to places
	for _, state := range s.States {
		place := schema.Place{
			ID:       state.ID,
			Type:     state.Type,
			Exported: state.Exported,
		}

		if state.IsToken() {
			place.Kind = schema.TokenKind
			place.Initial = state.InitialTokens()
		} else {
			place.Kind = schema.DataKind
			place.Initial = 0
		}

		model.Places = append(model.Places, place)
	}

	// Convert actions to transitions
	for _, action := range s.Actions {
		transition := schema.Transition{
			ID:        action.ID,
			Guard:     action.Guard,
			EventType: action.EventID,
			Bindings:  action.EventBindings,
		}
		model.Transitions = append(model.Transitions, transition)
	}

	// Convert arcs
	for _, arc := range s.Arcs {
		schemaArc := schema.Arc{
			From:   arc.Source,
			To:     arc.Target,
			Weight: 1, // Default weight
			Keys:   arc.Keys,
			Value:  arc.Value,
		}
		model.Arcs = append(model.Arcs, schemaArc)
	}

	// Convert constraints
	for _, constraint := range s.Constraints {
		model.Constraints = append(model.Constraints, schema.Constraint{
			ID:   constraint.ID,
			Expr: constraint.Expr,
		})
	}

	return model
}

// EnrichModel adds default values and infers missing fields.
// This prepares a simple LLM-generated model for code generation.
func EnrichModel(model *schema.Model) *schema.Model {
	enriched := *model // shallow copy

	// Ensure places have default kind
	for i := range enriched.Places {
		if enriched.Places[i].Kind == "" {
			enriched.Places[i].Kind = schema.TokenKind
		}
	}

	// Infer event types from transition IDs
	for i := range enriched.Transitions {
		t := &enriched.Transitions[i]
		if t.EventType == "" {
			t.EventType = toEventType(t.ID)
		}
		if t.HTTPPath == "" {
			t.HTTPPath = "/api/" + t.ID
		}
		if t.HTTPMethod == "" {
			t.HTTPMethod = "POST"
		}
	}

	return &enriched
}

// toEventType converts a transition ID to an event type name.
// Examples: "validate_order" -> "OrderValidated", "ship" -> "Shipped"
func toEventType(id string) string {
	// Simple conversion: capitalize and add "ed" suffix
	// In practice, this should be smarter
	if len(id) == 0 {
		return "Event"
	}

	// Handle snake_case
	result := ""
	capitalizeNext := true
	for _, c := range id {
		if c == '_' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result += string(toUpper(c))
			capitalizeNext = false
		} else {
			result += string(c)
		}
	}

	// Add past tense suffix if not already present
	if len(result) > 2 && result[len(result)-2:] != "ed" {
		result += "ed"
	}

	return result
}

func toUpper(c rune) rune {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// ValidateForCodegen checks if a model is ready for code generation.
func ValidateForCodegen(model *schema.Model) []string {
	var issues []string

	if model.Name == "" {
		issues = append(issues, "model name is required")
	}

	if len(model.Places) == 0 {
		issues = append(issues, "model has no places (states)")
	}

	if len(model.Transitions) == 0 {
		issues = append(issues, "model has no transitions (actions)")
	}

	// Check for unconnected elements
	connected := make(map[string]bool)
	for _, arc := range model.Arcs {
		connected[arc.From] = true
		connected[arc.To] = true
	}

	for _, p := range model.Places {
		if !connected[p.ID] {
			issues = append(issues, "place '"+p.ID+"' has no connections")
		}
	}

	for _, t := range model.Transitions {
		if !connected[t.ID] {
			issues = append(issues, "transition '"+t.ID+"' has no connections")
		}
	}

	// Check data places have types
	for _, p := range model.Places {
		if p.IsData() && p.Type == "" {
			issues = append(issues, "data place '"+p.ID+"' needs a type")
		}
	}

	return issues
}

// InferAPIRoutes generates API route information from transitions.
func InferAPIRoutes(model *schema.Model) []APIRoute {
	var routes []APIRoute

	for _, t := range model.Transitions {
		route := APIRoute{
			TransitionID: t.ID,
			Method:       t.HTTPMethod,
			Path:         t.HTTPPath,
			Description:  t.Description,
			EventType:    t.EventType,
		}

		if route.Method == "" {
			route.Method = "POST"
		}
		if route.Path == "" {
			route.Path = "/api/" + t.ID
		}
		if route.EventType == "" {
			route.EventType = toEventType(t.ID)
		}

		routes = append(routes, route)
	}

	return routes
}

// APIRoute represents an inferred API endpoint.
type APIRoute struct {
	TransitionID string
	Method       string
	Path         string
	Description  string
	EventType    string
}

// InferEvents generates event definitions from transitions.
func InferEvents(model *schema.Model) []EventDef {
	var events []EventDef

	for _, t := range model.Transitions {
		eventType := t.EventType
		if eventType == "" {
			eventType = toEventType(t.ID)
		}

		event := EventDef{
			Type:         eventType,
			TransitionID: t.ID,
			Fields:       inferEventFields(model, t),
		}

		events = append(events, event)
	}

	return events
}

// EventDef represents an inferred event definition.
type EventDef struct {
	Type         string
	TransitionID string
	Fields       []EventField
}

// EventField represents a field in an event.
type EventField struct {
	Name string
	Type string
}

// inferEventFields infers event fields from arc bindings and guards.
func inferEventFields(model *schema.Model, t schema.Transition) []EventField {
	fields := []EventField{
		{Name: "aggregate_id", Type: "string"},
		{Name: "timestamp", Type: "time.Time"},
	}

	// Add fields from arc bindings
	seen := make(map[string]bool)
	seen["aggregate_id"] = true
	seen["timestamp"] = true

	for _, arc := range model.Arcs {
		if arc.From == t.ID || arc.To == t.ID {
			for _, key := range arc.Keys {
				if !seen[key] {
					fields = append(fields, EventField{Name: key, Type: "string"})
					seen[key] = true
				}
			}
			if arc.Value != "" && !seen[arc.Value] {
				fields = append(fields, EventField{Name: arc.Value, Type: "int"})
				seen[arc.Value] = true
			}
		}
	}

	return fields
}

// InferAggregateState generates aggregate state fields from places.
func InferAggregateState(model *schema.Model) []StateField {
	var fields []StateField

	for _, p := range model.Places {
		field := StateField{
			Name:      p.ID,
			Type:      "int",
			IsToken:   p.IsToken(),
			Persisted: p.Persisted,
		}

		if p.IsData() {
			field.Type = p.Type
			if field.Type == "" {
				field.Type = "any"
			}
		}

		fields = append(fields, field)
	}

	return fields
}

// StateField represents a field in aggregate state.
type StateField struct {
	Name      string
	Type      string
	IsToken   bool
	Persisted bool
}
