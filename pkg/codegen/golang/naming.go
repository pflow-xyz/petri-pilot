// Package golang generates Go code from Petri net models.
package golang

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// Match non-alphanumeric characters
	nonAlphaNumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	// Match leading digits
	leadingDigits = regexp.MustCompile(`^[0-9]+`)
)

// ToPascalCase converts a snake_case or kebab-case string to PascalCase.
// e.g., "validate_order" -> "ValidateOrder", "process-payment" -> "ProcessPayment"
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}

	// Replace separators with spaces for splitting
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	words := strings.Fields(s)
	var result strings.Builder

	for _, word := range words {
		if word == "" {
			continue
		}
		// Capitalize first letter, lowercase rest
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		result.WriteString(string(runes))
	}

	return result.String()
}

// ToCamelCase converts a snake_case or kebab-case string to camelCase.
// e.g., "validate_order" -> "validateOrder", "process-payment" -> "processPayment"
func ToCamelCase(s string) string {
	pascal := ToPascalCase(s)
	if pascal == "" {
		return ""
	}
	runes := []rune(pascal)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// ToConstName creates a constant name from a prefix and ID.
// e.g., "Transition", "validate" -> "TransitionValidate"
// e.g., "Place", "order_received" -> "PlaceOrderReceived"
func ToConstName(prefix, id string) string {
	return prefix + ToPascalCase(id)
}

// ToHandlerName creates a handler function name from a transition ID.
// e.g., "validate" -> "HandleValidate"
// e.g., "process_payment" -> "HandleProcessPayment"
func ToHandlerName(id string) string {
	return "Handle" + ToPascalCase(id)
}

// ToEventTypeName creates an event type name from a transition ID.
// e.g., "validate" -> "Validated"
// e.g., "process_payment" -> "PaymentProcessed"
func ToEventTypeName(id string) string {
	pascal := ToPascalCase(id)
	// Add "ed" suffix for past tense (simplified)
	if strings.HasSuffix(pascal, "e") {
		return pascal + "d"
	}
	return pascal + "ed"
}

// ToEventStructName creates an event struct name from an event type.
// e.g., "OrderValidated" -> "OrderValidatedEvent"
func ToEventStructName(eventType string) string {
	if strings.HasSuffix(eventType, "Event") {
		return eventType
	}
	return eventType + "Event"
}

// SanitizePackageName converts a model name to a valid Go package name.
// Package names must be lowercase, start with a letter, and contain only letters and digits.
// e.g., "Order-Processing" -> "orderprocessing"
// e.g., "123workflow" -> "workflow"
func SanitizePackageName(name string) string {
	if name == "" {
		return "workflow"
	}

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace common separators with nothing
	name = nonAlphaNumeric.ReplaceAllString(name, "")

	// Remove leading digits
	name = leadingDigits.ReplaceAllString(name, "")

	if name == "" {
		return "workflow"
	}

	return name
}

// ToFieldName creates a Go struct field name from a place ID.
// e.g., "order_received" -> "OrderReceived"
func ToFieldName(id string) string {
	return ToPascalCase(id)
}

// ToVarName creates a Go variable name from an ID.
// e.g., "order_received" -> "orderReceived"
func ToVarName(id string) string {
	return ToCamelCase(id)
}

// ToTypeName converts a schema type to a proper Go type.
// e.g., "map[string]int" -> "map[string]int" (unchanged)
// e.g., "string" -> "string"
func ToTypeName(schemaType string) string {
	if schemaType == "" {
		return "int" // Default for token places
	}
	return schemaType
}

// SanitizeModulePath creates a valid Go module path.
// e.g., "order-processing" -> "github.com/example/order-processing"
func SanitizeModulePath(name, defaultBase string) string {
	if defaultBase == "" {
		defaultBase = "github.com/example"
	}
	// Sanitize the name for use in module path
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = nonAlphaNumeric.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "workflow"
	}
	return defaultBase + "/" + name
}
