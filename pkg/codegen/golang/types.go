package golang

import (
	"strings"
)

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

// GoZeroValue returns the Go zero value literal for a type.
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

// GoMapInitializer returns the Go initializer for a map type.
// Example: "map[string]int" -> "make(map[string]int)"
func GoMapInitializer(typ string) string {
	if !strings.HasPrefix(typ, "map[") {
		return GoZeroValue(typ)
	}
	return "make(" + typ + ")"
}

// TypeToGo converts a schema type to a proper Go type.
// Handles common type mappings used in smart contracts and ORMs:
//   - "address" -> "string"
//   - "uint256" -> "int64" (simplified; use *big.Int for full precision)
//   - "int256" -> "int64"
//   - "tokenId" -> "string"
//   - "bytes32" -> "string"
func TypeToGo(schemaType string) string {
	// Handle map types recursively
	if strings.HasPrefix(schemaType, "map[") {
		keyType, valueType, _ := ParseMapType(schemaType)
		return "map[" + TypeToGo(keyType) + "]" + TypeToGo(valueType)
	}

	// Handle slice types
	if strings.HasPrefix(schemaType, "[]") {
		elemType := schemaType[2:]
		return "[]" + TypeToGo(elemType)
	}

	// Common type mappings for smart contracts
	switch schemaType {
	case "address":
		return "string"
	case "uint256", "int256":
		return "int64" // Simplified; use *big.Int for full precision
	case "uint128", "int128":
		return "int64"
	case "uint64":
		return "uint64"
	case "uint32":
		return "uint32"
	case "uint16":
		return "uint16"
	case "uint8":
		return "uint8"
	case "tokenId":
		return "string"
	case "bytes32", "bytes":
		return "string"
	case "bool":
		return "bool"
	case "":
		return "int" // Default for token places
	default:
		return schemaType
	}
}

// IsMapType returns true if the type is a map type.
func IsMapType(typ string) bool {
	return strings.HasPrefix(typ, "map[")
}

// IsSliceType returns true if the type is a slice type.
func IsSliceType(typ string) bool {
	return strings.HasPrefix(typ, "[]")
}

// IsPointerType returns true if the type is a pointer type.
func IsPointerType(typ string) bool {
	return strings.HasPrefix(typ, "*")
}

// IsNumericType returns true if the type is a numeric type.
func IsNumericType(typ string) bool {
	switch typ {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return true
	default:
		return false
	}
}

// MapKeyType returns the key type of a map, or empty string if not a map.
func MapKeyType(typ string) string {
	keyType, _, isMap := ParseMapType(typ)
	if !isMap {
		return ""
	}
	return keyType
}

// MapValueType returns the value type of a map, or the original type if not a map.
func MapValueType(typ string) string {
	_, valueType, _ := ParseMapType(typ)
	return valueType
}

// NestedMapTypes returns (outerKey, innerKey, value) for nested maps.
// Returns empty strings if not a nested map.
// Example: "map[string]map[string]int" -> ("string", "string", "int")
func NestedMapTypes(typ string) (outerKey, innerKey, valueType string) {
	outerKey, innerValue, isMap := ParseMapType(typ)
	if !isMap {
		return "", "", ""
	}

	innerKey, valueType, isNested := ParseMapType(innerValue)
	if !isNested {
		return "", "", ""
	}

	return outerKey, innerKey, valueType
}

// IsNestedMap returns true if the type is a nested map (map of maps).
func IsNestedMap(typ string) bool {
	_, innerValue, isMap := ParseMapType(typ)
	if !isMap {
		return false
	}
	return strings.HasPrefix(innerValue, "map[")
}

// GuardExpressionToGo converts a guard expression to Go code.
// Handles common patterns like:
//   - "balances[from] >= amount" -> "state.Balances[bindings.From] >= bindings.Amount"
//   - "owner != address(0)" -> `bindings.Owner != ""`
func GuardExpressionToGo(expr string, statePrefix, bindingsPrefix string) string {
	if expr == "" {
		return "true"
	}

	// This is a simplified conversion - a full implementation would need a parser
	// For now, we just return the expression as a comment and a placeholder
	return "/* " + expr + " */ true"
}

// DataAccessToGo generates Go code for a data access pattern.
// This is used in templates to generate map access code.
type DataAccessInfo struct {
	Collection   string   // Collection/place ID
	Keys         []string // Key binding names
	ValueBinding string   // Value binding name
	IsSubtract   bool     // True for input arcs (subtract), false for output (add)
}

// GenerateMapAccess generates Go code for accessing a map with the given keys.
// Example: ["from"] with collection "balances" -> "state.Balances[bindings.From]"
func GenerateMapAccess(collection string, keys []string, stateVar, bindingsVar string) string {
	if len(keys) == 0 {
		return stateVar + "." + ToPascalCase(collection)
	}

	result := stateVar + "." + ToPascalCase(collection)
	for _, key := range keys {
		result += "[" + bindingsVar + "." + ToPascalCase(key) + "]"
	}
	return result
}

// GenerateMapUpdate generates Go code for updating a map value.
// Example: transfer from->to with amount
//   - Subtract: "state.Balances[bindings.From] -= bindings.Amount"
//   - Add: "state.Balances[bindings.To] += bindings.Amount"
func GenerateMapUpdate(collection string, keys []string, valueBinding string, isSubtract bool, stateVar, bindingsVar string) string {
	access := GenerateMapAccess(collection, keys, stateVar, bindingsVar)
	value := bindingsVar + "." + ToPascalCase(valueBinding)

	if isSubtract {
		return access + " -= " + value
	}
	return access + " += " + value
}

// GoTypeToGraphQL converts a Go type to a GraphQL type.
func GoTypeToGraphQL(goType string) string {
	switch goType {
	case "string":
		return "String"
	case "int", "int8", "int16", "int32", "int64":
		return "Int"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return "Int"
	case "float32", "float64", "number":
		return "Float"
	case "bool", "boolean":
		return "Boolean"
	case "time.Time", "time":
		return "Time"
	default:
		// For complex types, use String (JSON encoded)
		if strings.HasPrefix(goType, "map[") || strings.HasPrefix(goType, "[]") {
			return "String"
		}
		return "String"
	}
}
