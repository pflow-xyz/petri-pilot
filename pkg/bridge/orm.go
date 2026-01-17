// Package bridge provides conversion between petri-pilot schema and go-pflow metamodel.
// This file adds ORM pattern extraction for data-centric models.
package bridge

import (
	"strings"

	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// ORMSpec describes the ORM-like patterns extracted from a model.
// It identifies collections (DataState places) and operations (transitions with data arcs).
type ORMSpec struct {
	// Collections are DataState places that act as ORM tables/maps.
	Collections []CollectionSpec

	// Operations are transitions that operate on collections.
	Operations []OperationSpec

	// Constraints are invariants over the data.
	Constraints []ConstraintSpec
}

// CollectionSpec describes a single collection extracted from a DataState place.
type CollectionSpec struct {
	// PlaceID is the original place ID.
	PlaceID string

	// Name is the display/code name (e.g., "Balances", "Allowances").
	Name string

	// Type is the full type string from the place definition.
	Type string

	// KeyType is the type of the map key (e.g., "string" for map[string]int).
	// Empty for simple types.
	KeyType string

	// ValueType is the type of the value.
	// For maps: the map value type (e.g., "int64" for map[string]int64)
	// For simple types: the type itself (e.g., "string", "int64")
	ValueType string

	// IsSimple indicates this is a simple type (string, int64, bool, etc.)
	// that is directly assigned from bindings, not accessed via keys.
	IsSimple bool

	// IsMap indicates if this is a map type (vs a single value).
	IsMap bool

	// IsNested indicates if this is a nested map (e.g., map[string]map[string]int).
	IsNested bool

	// NestedKeyType is the key type of the nested map (if IsNested).
	NestedKeyType string

	// Description from the original place.
	Description string

	// Exported indicates if this collection is externally visible.
	Exported bool
}

// OperationSpec describes an operation (transition) that accesses collections.
type OperationSpec struct {
	// TransitionID is the original transition ID.
	TransitionID string

	// Name is the display/code name (e.g., "Transfer", "Mint").
	Name string

	// Guard is the precondition expression.
	Guard string

	// Reads are read operations (input arcs from DataState places).
	Reads []DataAccessSpec

	// Writes are write operations (output arcs to DataState places).
	Writes []DataAccessSpec

	// Description from the original transition.
	Description string
}

// DataAccessSpec describes a single data access (read or write).
type DataAccessSpec struct {
	// Collection is the target collection (place ID).
	Collection string

	// CollectionType is the type of the collection (e.g., "string", "map[string]int64").
	CollectionType string

	// IsSimple indicates this accesses a simple type (direct assignment, no keys).
	IsSimple bool

	// Keys are the binding names for map access (e.g., ["from"] or ["owner", "spender"]).
	// Empty for simple types.
	Keys []string

	// ValueBinding is the binding name for the value (e.g., "amount" or "name").
	ValueBinding string

	// IsSubtract indicates if this is a subtract operation (input arc).
	// Only meaningful for numeric types (int64, maps with numeric values).
	IsSubtract bool
}

// ConstraintSpec describes an invariant constraint.
type ConstraintSpec struct {
	// ID is the constraint identifier.
	ID string

	// Expr is the constraint expression.
	Expr string

	// Collections are the collections referenced by this constraint.
	Collections []string
}

// ExtractORMSpec extracts ORM-like patterns from a model.
func ExtractORMSpec(model *schema.Model) *ORMSpec {
	spec := &ORMSpec{
		Collections: make([]CollectionSpec, 0),
		Operations:  make([]OperationSpec, 0),
		Constraints: make([]ConstraintSpec, 0),
	}

	// Build place lookup
	placeMap := make(map[string]*schema.Place)
	for i := range model.Places {
		placeMap[model.Places[i].ID] = &model.Places[i]
	}

	// Extract collections from DataState places
	for _, place := range model.Places {
		if place.IsData() {
			coll := extractCollection(place)
			spec.Collections = append(spec.Collections, coll)
		}
	}

	// Extract operations from transitions
	for _, transition := range model.Transitions {
		op := extractOperation(transition, model.Arcs, placeMap)
		if len(op.Reads) > 0 || len(op.Writes) > 0 {
			spec.Operations = append(spec.Operations, op)
		}
	}

	// Extract constraint specs
	for _, constraint := range model.Constraints {
		cs := extractConstraint(constraint, spec.Collections)
		spec.Constraints = append(spec.Constraints, cs)
	}

	return spec
}

// extractCollection extracts a CollectionSpec from a DataState place.
func extractCollection(place schema.Place) CollectionSpec {
	coll := CollectionSpec{
		PlaceID:     place.ID,
		Name:        toPascalCase(place.ID),
		Type:        place.Type,
		Description: place.Description,
		Exported:    place.Exported,
	}

	// Check if this is a simple type (string, int64, bool, etc.)
	if place.IsSimpleType() {
		coll.IsSimple = true
		coll.ValueType = place.Type
		return coll
	}

	// Parse the type to extract key/value types for maps
	keyType, valueType, isMap := ParseMapType(place.Type)
	coll.IsMap = isMap
	coll.KeyType = keyType
	coll.ValueType = valueType

	// Check for nested maps (e.g., map[string]map[string]int)
	if isMap && strings.HasPrefix(valueType, "map[") {
		coll.IsNested = true
		nestedKey, nestedValue, _ := ParseMapType(valueType)
		coll.NestedKeyType = nestedKey
		coll.ValueType = nestedValue
	}

	return coll
}

// extractOperation extracts an OperationSpec from a transition.
func extractOperation(transition schema.Transition, arcs []schema.Arc, placeMap map[string]*schema.Place) OperationSpec {
	op := OperationSpec{
		TransitionID: transition.ID,
		Name:         toPascalCase(transition.ID),
		Guard:        transition.Guard,
		Description:  transition.Description,
		Reads:        make([]DataAccessSpec, 0),
		Writes:       make([]DataAccessSpec, 0),
	}

	for _, arc := range arcs {
		// Input arc: place -> transition (read/subtract)
		if arc.To == transition.ID {
			place := placeMap[arc.From]
			if place != nil && place.IsData() {
				access := DataAccessSpec{
					Collection:     arc.From,
					CollectionType: place.Type,
					IsSimple:       place.IsSimpleType(),
					Keys:           arc.Keys,
					ValueBinding:   arc.Value,
					IsSubtract:     true,
				}
				// Default value binding based on type
				if access.ValueBinding == "" {
					if access.IsSimple {
						access.ValueBinding = place.ID // Use place ID as default binding
					} else {
						access.ValueBinding = "amount"
					}
				}
				op.Reads = append(op.Reads, access)
			}
		}

		// Output arc: transition -> place (write/add)
		if arc.From == transition.ID {
			place := placeMap[arc.To]
			if place != nil && place.IsData() {
				access := DataAccessSpec{
					Collection:     arc.To,
					CollectionType: place.Type,
					IsSimple:       place.IsSimpleType(),
					Keys:           arc.Keys,
					ValueBinding:   arc.Value,
					IsSubtract:     false,
				}
				// Default value binding based on type
				if access.ValueBinding == "" {
					if access.IsSimple {
						access.ValueBinding = place.ID // Use place ID as default binding
					} else {
						access.ValueBinding = "amount"
					}
				}
				op.Writes = append(op.Writes, access)
			}
		}
	}

	return op
}

// extractConstraint extracts a ConstraintSpec from a constraint.
func extractConstraint(constraint schema.Constraint, collections []CollectionSpec) ConstraintSpec {
	cs := ConstraintSpec{
		ID:          constraint.ID,
		Expr:        constraint.Expr,
		Collections: make([]string, 0),
	}

	// Find referenced collections in the expression
	for _, coll := range collections {
		if strings.Contains(constraint.Expr, coll.PlaceID) {
			cs.Collections = append(cs.Collections, coll.PlaceID)
		}
	}

	return cs
}

// ParseMapType parses a map type string and extracts key/value types.
// Examples:
//   - "map[string]int" -> ("string", "int", true)
//   - "map[address]uint256" -> ("address", "uint256", true)
//   - "int" -> ("", "int", false)
//   - "map[string]map[string]int" -> ("string", "map[string]int", true)
func ParseMapType(typ string) (keyType, valueType string, isMap bool) {
	if !strings.HasPrefix(typ, "map[") {
		return "", typ, false
	}

	// Find the closing bracket for the key type
	depth := 0
	keyEnd := -1
	for i, c := range typ[4:] {
		if c == '[' {
			depth++
		} else if c == ']' {
			if depth == 0 {
				keyEnd = i + 4
				break
			}
			depth--
		}
	}

	if keyEnd == -1 {
		return "", typ, false
	}

	keyType = typ[4:keyEnd]
	valueType = typ[keyEnd+1:]

	return keyType, valueType, true
}

// GoZeroValue returns the Go zero value for a type.
func GoZeroValue(typ string) string {
	switch typ {
	case "int", "int8", "int16", "int32", "int64":
		return "0"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "0"
	case "float32", "float64":
		return "0.0"
	case "string":
		return `""`
	case "bool":
		return "false"
	default:
		if strings.HasPrefix(typ, "map[") {
			return "nil"
		}
		if strings.HasPrefix(typ, "[]") {
			return "nil"
		}
		if strings.HasPrefix(typ, "*") {
			return "nil"
		}
		return "nil"
	}
}

// TypeToGo converts a schema type to a proper Go type.
// Handles common type mappings:
//   - "address" -> "string"
//   - "uint256" -> "int64" (or "*big.Int" for full precision)
//   - "tokenId" -> "string"
func TypeToGo(schemaType string) string {
	// Handle map types recursively
	if strings.HasPrefix(schemaType, "map[") {
		keyType, valueType, _ := ParseMapType(schemaType)
		return "map[" + TypeToGo(keyType) + "]" + TypeToGo(valueType)
	}

	// Common type mappings
	switch schemaType {
	case "address":
		return "string"
	case "uint256", "int256":
		return "int64" // Simplified; use *big.Int for full precision
	case "tokenId":
		return "string"
	case "":
		return "int" // Default for token places
	default:
		return schemaType
	}
}

// toPascalCase converts a snake_case or kebab-case string to PascalCase.
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}

	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	words := strings.Fields(s)
	var result strings.Builder

	for _, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		runes[0] = toUpperRune(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = toLowerRune(runes[i])
		}
		result.WriteString(string(runes))
	}

	return result.String()
}

func toUpperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

func toLowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// HasDataOperations returns true if the spec has any data operations.
func (s *ORMSpec) HasDataOperations() bool {
	return len(s.Operations) > 0
}

// HasNestedMaps returns true if any collection uses nested maps.
func (s *ORMSpec) HasNestedMaps() bool {
	for _, c := range s.Collections {
		if c.IsNested {
			return true
		}
	}
	return false
}

// CollectionByID returns a collection by its place ID.
func (s *ORMSpec) CollectionByID(placeID string) *CollectionSpec {
	for i := range s.Collections {
		if s.Collections[i].PlaceID == placeID {
			return &s.Collections[i]
		}
	}
	return nil
}
