// Package validator provides formal validation of Petri net models using go-pflow.
package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pflow-xyz/petri-pilot/pkg/schema"
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
func (v *Validator) ValidateImplementability(model *schema.Model) *ImplementabilityResult {
	result := &ImplementabilityResult{
		Implementable: true,
	}

	// Run all checks
	v.checkEventDerivation(model, result)
	v.checkStateSchema(model, result)
	v.checkAPIMappings(model, result)
	v.checkTypeConsistency(model, result)
	v.checkGuardParsability(model, result)
	v.detectPattern(model, result)

	// Set implementable based on errors
	result.Implementable = len(result.Errors) == 0

	return result
}

// checkEventDerivation verifies events can be inferred from transitions.
func (v *Validator) checkEventDerivation(model *schema.Model, result *ImplementabilityResult) {
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
func (v *Validator) checkStateSchema(model *schema.Model, result *ImplementabilityResult) {
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
func (v *Validator) checkAPIMappings(model *schema.Model, result *ImplementabilityResult) {
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
func (v *Validator) checkTypeConsistency(model *schema.Model, result *ImplementabilityResult) {
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
func (v *Validator) checkGuardParsability(model *schema.Model, result *ImplementabilityResult) {
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

// detectPattern identifies the model pattern type.
func (v *Validator) detectPattern(model *schema.Model, result *ImplementabilityResult) {
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
