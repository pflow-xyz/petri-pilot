// Package validator provides formal validation of Petri net models using go-pflow.
package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// ImplementabilityResult contains the results of implementability analysis.
type ImplementabilityResult struct {
	// Implementable is true if the model can be code-generated
	Implementable bool `json:"implementable"`

	// Errors are issues that prevent code generation
	Errors []ImplementabilityIssue `json:"errors,omitempty"`

	// Warnings are issues that may affect generated code quality
	Warnings []ImplementabilityIssue `json:"warnings,omitempty"`

	// Pattern describes the detected model pattern
	Pattern ModelPattern `json:"pattern"`

	// EventMappings shows how transitions map to events
	EventMappings []EventMapping `json:"event_mappings,omitempty"`

	// StateMappings shows how places map to state fields
	StateMappings []StateMapping `json:"state_mappings,omitempty"`
}

// ImplementabilityIssue describes a specific implementability problem.
type ImplementabilityIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Element string `json:"element,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

// ModelPattern describes the detected pattern type.
type ModelPattern struct {
	Type        string `json:"type"` // "workflow", "state_machine", "resource_pool", "mixed"
	Description string `json:"description"`
	Confidence  float64 `json:"confidence"` // 0.0 to 1.0
}

// EventMapping shows how a transition maps to an event.
type EventMapping struct {
	TransitionID string   `json:"transition_id"`
	EventType    string   `json:"event_type"`
	Fields       []string `json:"fields,omitempty"`
	Derivable    bool     `json:"derivable"`
}

// StateMapping shows how a place maps to aggregate state.
type StateMapping struct {
	PlaceID   string `json:"place_id"`
	FieldName string `json:"field_name"`
	FieldType string `json:"field_type"`
	IsToken   bool   `json:"is_token"`
}

// ValidateImplementability checks if a model can be successfully code-generated.
func (v *Validator) ValidateImplementability(model *metamodel.Model) *ImplementabilityResult {
	result := &ImplementabilityResult{
		Implementable: true,
	}

	// Run all checks
	v.checkEventReferences(model, result)
	v.checkEventDerivation(model, result)
	v.checkStateSchema(model, result)
	v.checkAPIMappings(model, result)
	v.checkTypeConsistency(model, result)
	v.checkGuardParsability(model, result)
	v.checkViewBindings(model, result)
	v.detectPattern(model, result)

	// Set implementable based on errors
	result.Implementable = len(result.Errors) == 0

	return result
}

// checkEventReferences validates that transition.Event references exist in model.Events.
func (v *Validator) checkEventReferences(model *metamodel.Model, result *ImplementabilityResult) {
	// Build event lookup
	eventIDs := make(map[string]bool)
	eventFields := make(map[string]map[string]bool) // event ID -> field names
	for _, e := range model.Events {
		eventIDs[e.ID] = true
		eventFields[e.ID] = make(map[string]bool)
		for _, f := range e.Fields {
			eventFields[e.ID][f.Name] = true
		}
	}

	// Build place lookup for binding validation
	placeIDs := make(map[string]bool)
	for _, p := range model.Places {
		placeIDs[p.ID] = true
	}

	// Check that all transition.Event references are valid
	for _, t := range model.Transitions {
		if t.Event != "" {
			if !eventIDs[t.Event] {
				result.Errors = append(result.Errors, ImplementabilityIssue{
					Code:    "INVALID_EVENT_REF",
					Message: fmt.Sprintf("Transition '%s' references undefined event '%s'", t.ID, t.Event),
					Element: t.ID,
					Fix:     fmt.Sprintf("Add event definition with id '%s' or remove event reference", t.Event),
				})
			}
		}

		// Validate bindings
		v.checkBindings(t, placeIDs, eventFields, result)
	}

	// Validate event field types
	validTypes := map[string]bool{
		"string": true, "number": true, "integer": true, "boolean": true,
		"array": true, "object": true, "time": true,
	}
	for _, e := range model.Events {
		for _, f := range e.Fields {
			if !validTypes[f.Type] && !isValidIdentifier(f.Type) {
				result.Warnings = append(result.Warnings, ImplementabilityIssue{
					Code:    "UNKNOWN_EVENT_FIELD_TYPE",
					Message: fmt.Sprintf("Event '%s' field '%s' has unknown type '%s'", e.ID, f.Name, f.Type),
					Element: e.ID,
					Fix:     "Use standard types: string, number, integer, boolean, array, object, time",
				})
			}
		}
	}
}

// checkBindings validates bindings on a transition.
func (v *Validator) checkBindings(t metamodel.Transition, placeIDs map[string]bool, eventFields map[string]map[string]bool, result *ImplementabilityResult) {
	validTypes := map[string]bool{
		"string": true, "number": true, "integer": true, "boolean": true,
		"time": true, "int": true, "int64": true, "float64": true, "bool": true,
	}

	for _, b := range t.Bindings {
		// Check binding name is valid identifier
		if !isValidIdentifier(b.Name) {
			result.Errors = append(result.Errors, ImplementabilityIssue{
				Code:    "INVALID_BINDING_NAME",
				Message: fmt.Sprintf("Transition '%s' binding '%s' is not a valid identifier", t.ID, b.Name),
				Element: t.ID,
				Fix:     "Use valid identifier characters for binding names",
			})
		}

		// Check binding type is valid (either standard type or starts with map[)
		if !validTypes[b.Type] && !isValidIdentifier(b.Type) && !isMapType(b.Type) {
			result.Warnings = append(result.Warnings, ImplementabilityIssue{
				Code:    "UNKNOWN_BINDING_TYPE",
				Message: fmt.Sprintf("Transition '%s' binding '%s' has unknown type '%s'", t.ID, b.Name, b.Type),
				Element: t.ID,
				Fix:     "Use standard types: string, number, integer, boolean, time, or map[K]V",
			})
		}

		// Check place reference if specified
		if b.Place != "" && !placeIDs[b.Place] {
			result.Errors = append(result.Errors, ImplementabilityIssue{
				Code:    "INVALID_BINDING_PLACE",
				Message: fmt.Sprintf("Transition '%s' binding '%s' references undefined place '%s'", t.ID, b.Name, b.Place),
				Element: t.ID,
				Fix:     fmt.Sprintf("Add place definition with id '%s' or remove place reference", b.Place),
			})
		}

		// Check binding matches event field if transition has event reference
		if t.Event != "" && len(eventFields[t.Event]) > 0 {
			if !eventFields[t.Event][b.Name] {
				result.Warnings = append(result.Warnings, ImplementabilityIssue{
					Code:    "BINDING_NOT_IN_EVENT",
					Message: fmt.Sprintf("Transition '%s' binding '%s' is not defined in event '%s'", t.ID, b.Name, t.Event),
					Element: t.ID,
					Fix:     fmt.Sprintf("Add field '%s' to event '%s' or remove binding", b.Name, t.Event),
				})
			}
		}
	}
}

// isMapType checks if a type string represents a map type.
func isMapType(typ string) bool {
	return len(typ) > 4 && typ[:4] == "map["
}

// checkEventDerivation verifies events can be inferred from transitions.
func (v *Validator) checkEventDerivation(model *metamodel.Model, result *ImplementabilityResult) {
	for _, t := range model.Transitions {
		mapping := EventMapping{
			TransitionID: t.ID,
			Derivable:    true,
		}

		// Determine event type
		if t.EventType != "" {
			mapping.EventType = t.EventType
		} else {
			// Infer from transition ID
			mapping.EventType = inferEventType(t.ID)
		}

		// Check for valid event type name
		if !isValidIdentifier(mapping.EventType) {
			result.Warnings = append(result.Warnings, ImplementabilityIssue{
				Code:    "INVALID_EVENT_NAME",
				Message: fmt.Sprintf("Inferred event type '%s' may not be a valid identifier", mapping.EventType),
				Element: t.ID,
				Fix:     "Set explicit eventType in transition definition",
			})
		}

		// Infer fields from arc bindings
		for _, arc := range model.Arcs {
			if arc.From == t.ID || arc.To == t.ID {
				mapping.Fields = append(mapping.Fields, arc.Keys...)
				if arc.Value != "" && !contains(mapping.Fields, arc.Value) {
					mapping.Fields = append(mapping.Fields, arc.Value)
				}
			}
		}

		// Add standard fields
		mapping.Fields = append([]string{"aggregate_id", "timestamp"}, mapping.Fields...)

		result.EventMappings = append(result.EventMappings, mapping)
	}
}

// checkStateSchema verifies aggregate state can be generated.
func (v *Validator) checkStateSchema(model *metamodel.Model, result *ImplementabilityResult) {
	for _, p := range model.Places {
		mapping := StateMapping{
			PlaceID:   p.ID,
			FieldName: toFieldName(p.ID),
			IsToken:   p.IsToken(),
		}

		if p.IsToken() {
			mapping.FieldType = "int"
		} else {
			if p.Type != "" {
				mapping.FieldType = p.Type
			} else {
				mapping.FieldType = "any"
				result.Warnings = append(result.Warnings, ImplementabilityIssue{
					Code:    "UNTYPED_DATA_PLACE",
					Message: fmt.Sprintf("Data place '%s' has no type, will use 'any'", p.ID),
					Element: p.ID,
					Fix:     "Add 'type' field to place definition",
				})
			}
		}

		// Check for valid field name
		if !isValidIdentifier(mapping.FieldName) {
			result.Errors = append(result.Errors, ImplementabilityIssue{
				Code:    "INVALID_FIELD_NAME",
				Message: fmt.Sprintf("Place '%s' produces invalid field name '%s'", p.ID, mapping.FieldName),
				Element: p.ID,
				Fix:     "Rename place to use valid Go identifier characters",
			})
		}

		result.StateMappings = append(result.StateMappings, mapping)
	}
}

// checkAPIMappings verifies transitions have valid HTTP semantics.
func (v *Validator) checkAPIMappings(model *metamodel.Model, result *ImplementabilityResult) {
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
	}

	pathPattern := regexp.MustCompile(`^/[a-zA-Z0-9_/{}\-]*$`)

	for _, t := range model.Transitions {
		// Check HTTP method if specified
		if t.HTTPMethod != "" && !validMethods[strings.ToUpper(t.HTTPMethod)] {
			result.Warnings = append(result.Warnings, ImplementabilityIssue{
				Code:    "INVALID_HTTP_METHOD",
				Message: fmt.Sprintf("Transition '%s' has invalid HTTP method '%s'", t.ID, t.HTTPMethod),
				Element: t.ID,
				Fix:     "Use GET, POST, PUT, DELETE, or PATCH",
			})
		}

		// Check HTTP path if specified
		if t.HTTPPath != "" && !pathPattern.MatchString(t.HTTPPath) {
			result.Warnings = append(result.Warnings, ImplementabilityIssue{
				Code:    "INVALID_HTTP_PATH",
				Message: fmt.Sprintf("Transition '%s' has potentially invalid HTTP path '%s'", t.ID, t.HTTPPath),
				Element: t.ID,
				Fix:     "Use valid URL path characters",
			})
		}
	}
}

// checkTypeConsistency verifies all types are valid Go types.
func (v *Validator) checkTypeConsistency(model *metamodel.Model, result *ImplementabilityResult) {
	// Basic Go types and common patterns
	validTypePattern := regexp.MustCompile(`^(\*?)(map\[.+\].+|\[\].+|[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?)$`)

	for _, p := range model.Places {
		if p.Type != "" {
			if !validTypePattern.MatchString(p.Type) && p.Type != "any" {
				result.Warnings = append(result.Warnings, ImplementabilityIssue{
					Code:    "SUSPICIOUS_TYPE",
					Message: fmt.Sprintf("Place '%s' has potentially invalid Go type '%s'", p.ID, p.Type),
					Element: p.ID,
					Fix:     "Use valid Go type syntax",
				})
			}
		}
	}
}

// checkGuardParsability verifies guard expressions can be translated.
func (v *Validator) checkGuardParsability(model *metamodel.Model, result *ImplementabilityResult) {
	// Simple patterns we can handle
	simpleGuardPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*\s*(>|<|>=|<=|==|!=)\s*\d+$`)
	compoundGuardPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\[\]"'\.]*\s*(>|<|>=|<=|==|!=)\s*[a-zA-Z0-9_\[\]"'\.]+$`)

	for _, t := range model.Transitions {
		if t.Guard == "" {
			continue
		}

		guard := strings.TrimSpace(t.Guard)

		// Check if guard matches known patterns
		if !simpleGuardPattern.MatchString(guard) && !compoundGuardPattern.MatchString(guard) {
			// More complex guard - may need manual implementation
			result.Warnings = append(result.Warnings, ImplementabilityIssue{
				Code:    "COMPLEX_GUARD",
				Message: fmt.Sprintf("Transition '%s' has complex guard '%s' that may need manual implementation", t.ID, t.Guard),
				Element: t.ID,
				Fix:     "Simplify guard or implement custom guard logic in generated code",
			})
		}
	}
}

// checkViewBindings validates that view field bindings reference defined event fields.
func (v *Validator) checkViewBindings(model *metamodel.Model, result *ImplementabilityResult) {
	// Skip if no explicit events are defined
	if len(model.Events) == 0 {
		return
	}

	// Build set of all event field names
	eventFields := make(map[string]bool)
	for _, e := range model.Events {
		for _, f := range e.Fields {
			eventFields[f.Name] = true
		}
	}

	// Also include standard fields that are always present
	eventFields["aggregate_id"] = true
	eventFields["timestamp"] = true

	// Check view field bindings against event fields
	for _, view := range model.Views {
		for _, group := range view.Groups {
			for _, field := range group.Fields {
				if field.Binding != "" && !eventFields[field.Binding] {
					result.Warnings = append(result.Warnings, ImplementabilityIssue{
						Code:    "UNBOUND_VIEW_FIELD",
						Message: fmt.Sprintf("View '%s' field binding '%s' does not match any event field", view.ID, field.Binding),
						Element: view.ID,
						Fix:     fmt.Sprintf("Add field '%s' to an event definition or update binding", field.Binding),
					})
				}
			}
		}
	}
}

// detectPattern identifies the model pattern type.
func (v *Validator) detectPattern(model *metamodel.Model, result *ImplementabilityResult) {
	// Count characteristics
	hasInitialTokens := 0
	hasDataPlaces := 0
	hasCycles := false
	hasParallelPaths := false

	for _, p := range model.Places {
		if p.Initial > 0 {
			hasInitialTokens++
		}
		if p.IsData() {
			hasDataPlaces++
		}
	}

	// Build adjacency for cycle/parallel detection
	outgoing := make(map[string][]string)
	incoming := make(map[string][]string)

	for _, arc := range model.Arcs {
		outgoing[arc.From] = append(outgoing[arc.From], arc.To)
		incoming[arc.To] = append(incoming[arc.To], arc.From)
	}

	// Check for parallel paths (transitions with multiple outputs)
	for _, t := range model.Transitions {
		if len(outgoing[t.ID]) > 1 {
			hasParallelPaths = true
			break
		}
	}

	// Simple cycle detection via DFS
	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var hasCycleDFS func(node string) bool
	hasCycleDFS = func(node string) bool {
		visited[node] = true
		inStack[node] = true

		for _, next := range outgoing[node] {
			if !visited[next] {
				if hasCycleDFS(next) {
					return true
				}
			} else if inStack[next] {
				return true
			}
		}

		inStack[node] = false
		return false
	}

	for _, p := range model.Places {
		if !visited[p.ID] {
			if hasCycleDFS(p.ID) {
				hasCycles = true
				break
			}
		}
	}

	// Determine pattern
	pattern := ModelPattern{Confidence: 0.7}

	if hasDataPlaces > 0 && hasCycles {
		pattern.Type = "resource_pool"
		pattern.Description = "Resource management with data state and cycles (e.g., token transfers, inventory)"
	} else if hasInitialTokens == 1 && !hasCycles && !hasParallelPaths {
		pattern.Type = "workflow"
		pattern.Description = "Linear workflow with sequential state transitions"
		pattern.Confidence = 0.9
	} else if hasInitialTokens == 1 && !hasCycles && hasParallelPaths {
		pattern.Type = "workflow"
		pattern.Description = "Branching workflow with alternative paths"
		pattern.Confidence = 0.85
	} else if hasCycles && hasInitialTokens > 0 {
		pattern.Type = "state_machine"
		pattern.Description = "State machine with cyclic transitions"
		pattern.Confidence = 0.8
	} else {
		pattern.Type = "mixed"
		pattern.Description = "Mixed pattern with multiple concerns"
		pattern.Confidence = 0.5
	}

	result.Pattern = pattern
}

// Helper functions

func inferEventType(transitionID string) string {
	// Convert snake_case to PascalCase and add past tense
	parts := strings.Split(transitionID, "_")
	var result strings.Builder

	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				result.WriteString(part[1:])
			}
		}
	}

	name := result.String()

	// Add past tense suffix
	if len(name) > 0 && !strings.HasSuffix(strings.ToLower(name), "ed") {
		if strings.HasSuffix(name, "e") {
			name += "d"
		} else {
			name += "ed"
		}
	}

	return name
}

func toFieldName(placeID string) string {
	// Convert to valid Go field name (PascalCase)
	parts := strings.FieldsFunc(placeID, func(c rune) bool {
		return c == '_' || c == '-' || c == '.'
	})

	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				result.WriteString(part[1:])
			}
		}
	}

	return result.String()
}

func isValidIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Must start with letter or underscore
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}

	// Rest must be alphanumeric or underscore
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return true
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
