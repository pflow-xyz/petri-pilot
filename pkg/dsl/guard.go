package dsl

import (
	"fmt"
	"strings"
)

// Compiled represents a pre-compiled guard expression.
type Compiled struct {
	expr string
	ast  Node
}

// Compile parses a guard expression into a compiled form for repeated evaluation.
func Compile(expr string) (*Compiled, error) {
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}

	parser := NewParser(expr)
	ast, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return &Compiled{
		expr: expr,
		ast:  ast,
	}, nil
}

// String returns the original expression.
func (c *Compiled) String() string {
	return c.expr
}

// AST returns the parsed abstract syntax tree.
func (c *Compiled) AST() Node {
	return c.ast
}

// Evaluate parses and evaluates a guard expression.
// Returns true if guard passes, false if it fails, error if invalid.
func Evaluate(expr string, bindings map[string]any, funcs map[string]GuardFunc) (bool, error) {
	if expr == "" {
		return true, nil // Empty guard always passes
	}

	compiled, err := Compile(expr)
	if err != nil {
		return false, err
	}

	return EvalCompiled(compiled, bindings, funcs)
}

// EvalCompiled evaluates a pre-compiled guard expression.
func EvalCompiled(compiled *Compiled, bindings map[string]any, funcs map[string]GuardFunc) (bool, error) {
	if compiled == nil || compiled.ast == nil {
		return true, nil // Nil guard always passes
	}

	ctx := &Context{
		Bindings: bindings,
		Funcs:    funcs,
	}

	if ctx.Bindings == nil {
		ctx.Bindings = make(map[string]any)
	}
	if ctx.Funcs == nil {
		ctx.Funcs = make(map[string]GuardFunc)
	}

	// Add built-in functions
	addBuiltins(ctx)

	result, err := Eval(compiled.ast, ctx)
	if err != nil {
		return false, err
	}

	// Result must be boolean
	b, ok := toBool(result)
	if !ok {
		return false, fmt.Errorf("guard expression must evaluate to boolean, got %T", result)
	}

	return b, nil
}

// addBuiltins adds built-in functions to the context.
func addBuiltins(ctx *Context) {
	// len(collection) - returns the length of a collection
	if _, exists := ctx.Funcs["len"]; !exists {
		ctx.Funcs["len"] = func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("len() requires exactly 1 argument")
			}
			switch v := args[0].(type) {
			case string:
				return int64(len(v)), nil
			case map[string]any:
				return int64(len(v)), nil
			case map[string]int64:
				return int64(len(v)), nil
			case []any:
				return int64(len(v)), nil
			default:
				return nil, fmt.Errorf("len() cannot operate on %T", args[0])
			}
		}
	}

	// min(a, b) - returns the minimum of two numbers
	if _, exists := ctx.Funcs["min"]; !exists {
		ctx.Funcs["min"] = func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("min() requires exactly 2 arguments")
			}
			a, aok := toNumber(args[0])
			b, bok := toNumber(args[1])
			if !aok || !bok {
				return nil, fmt.Errorf("min() arguments must be numeric")
			}
			if a < b {
				return a, nil
			}
			return b, nil
		}
	}

	// max(a, b) - returns the maximum of two numbers
	if _, exists := ctx.Funcs["max"]; !exists {
		ctx.Funcs["max"] = func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("max() requires exactly 2 arguments")
			}
			a, aok := toNumber(args[0])
			b, bok := toNumber(args[1])
			if !aok || !bok {
				return nil, fmt.Errorf("max() arguments must be numeric")
			}
			if a > b {
				return a, nil
			}
			return b, nil
		}
	}

	// abs(n) - returns the absolute value
	if _, exists := ctx.Funcs["abs"]; !exists {
		ctx.Funcs["abs"] = func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("abs() requires exactly 1 argument")
			}
			n, ok := toNumber(args[0])
			if !ok {
				return nil, fmt.Errorf("abs() argument must be numeric")
			}
			if n < 0 {
				return -n, nil
			}
			return n, nil
		}
	}

	// contains(str, substr) - checks if string contains substring
	if _, exists := ctx.Funcs["contains"]; !exists {
		ctx.Funcs["contains"] = func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("contains() requires exactly 2 arguments")
			}
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("contains() first argument must be string")
			}
			substr, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("contains() second argument must be string")
			}
			return strings.Contains(str, substr), nil
		}
	}

	// startsWith(str, prefix) - checks if string starts with prefix
	if _, exists := ctx.Funcs["startsWith"]; !exists {
		ctx.Funcs["startsWith"] = func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("startsWith() requires exactly 2 arguments")
			}
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("startsWith() first argument must be string")
			}
			prefix, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("startsWith() second argument must be string")
			}
			return strings.HasPrefix(str, prefix), nil
		}
	}

	// endsWith(str, suffix) - checks if string ends with suffix
	if _, exists := ctx.Funcs["endsWith"]; !exists {
		ctx.Funcs["endsWith"] = func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("endsWith() requires exactly 2 arguments")
			}
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("endsWith() first argument must be string")
			}
			suffix, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("endsWith() second argument must be string")
			}
			return strings.HasSuffix(str, suffix), nil
		}
	}

	// includes(array, value) - checks if array contains value
	if _, exists := ctx.Funcs["includes"]; !exists {
		ctx.Funcs["includes"] = func(args ...any) (any, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("includes() requires exactly 2 arguments")
			}
			// Handle different array types
			switch arr := args[0].(type) {
			case []string:
				val, ok := args[1].(string)
				if !ok {
					return false, nil
				}
				for _, item := range arr {
					if item == val {
						return true, nil
					}
				}
				return false, nil
			case []any:
				for _, item := range arr {
					if item == args[1] {
						return true, nil
					}
				}
				return false, nil
			default:
				return nil, fmt.Errorf("includes() first argument must be an array, got %T", args[0])
			}
		}
	}

	// hasRole(roleName) - checks if current user has a specific role
	// Looks for user.roles in bindings (set by middleware)
	if _, exists := ctx.Funcs["hasRole"]; !exists {
		ctx.Funcs["hasRole"] = func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("hasRole() requires exactly 1 argument")
			}
			roleName, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("hasRole() argument must be a string")
			}

			// Get user from bindings
			userVal, ok := ctx.Bindings["user"]
			if !ok {
				return false, nil // No user in context
			}

			// User can be a map or a struct-like object
			var roles []string
			switch user := userVal.(type) {
			case map[string]any:
				if rolesVal, ok := user["roles"]; ok {
					switch r := rolesVal.(type) {
					case []string:
						roles = r
					case []any:
						for _, item := range r {
							if s, ok := item.(string); ok {
								roles = append(roles, s)
							}
						}
					}
				}
			}

			// Check if role is in the list
			for _, r := range roles {
				if r == roleName {
					return true, nil
				}
			}
			return false, nil
		}
	}
}

// EvaluateNumeric evaluates an expression and returns a numeric result.
// Unlike Evaluate (which returns bool), this is used for objective functions.
// Examples: "win_x - win_o", "tokens('goal') * 10", "sum('score')"
func EvaluateNumeric(expr string, bindings map[string]any, funcs map[string]GuardFunc) (float64, error) {
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	compiled, err := Compile(expr)
	if err != nil {
		return 0, err
	}

	return EvalNumericCompiled(compiled, bindings, funcs)
}

// EvalNumericCompiled evaluates a pre-compiled expression and returns a numeric result.
func EvalNumericCompiled(compiled *Compiled, bindings map[string]any, funcs map[string]GuardFunc) (float64, error) {
	if compiled == nil || compiled.ast == nil {
		return 0, fmt.Errorf("nil compiled expression")
	}

	ctx := &Context{
		Bindings: bindings,
		Funcs:    funcs,
	}

	if ctx.Bindings == nil {
		ctx.Bindings = make(map[string]any)
	}
	if ctx.Funcs == nil {
		ctx.Funcs = make(map[string]GuardFunc)
	}

	// Add built-in functions
	addBuiltins(ctx)

	result, err := Eval(compiled.ast, ctx)
	if err != nil {
		return 0, err
	}

	// Result must be numeric
	n, ok := toNumber(result)
	if !ok {
		return 0, fmt.Errorf("expression must evaluate to number, got %T: %v", result, result)
	}

	return n, nil
}

// EvaluateObjective evaluates an objective expression against a marking.
// This is the primary function for AI move evaluation.
// It provides aggregate functions (sum, count, tokens) and place values as bindings.
func EvaluateObjective(expr string, marking Marking) (float64, error) {
	if expr == "" {
		return 0, fmt.Errorf("empty objective expression")
	}

	// Create bindings from marking (place values accessible directly)
	bindings := make(map[string]any)
	for placeID, count := range marking {
		bindings[placeID] = int64(count)
	}

	// Create aggregate functions bound to this marking
	funcs := MakeAggregates(marking)

	return EvaluateNumeric(expr, bindings, funcs)
}

// Marking is a type alias for token state values.
type Marking map[string]int

// EvaluateInvariant checks if an invariant expression holds for a marking.
// It provides aggregate functions (sum, count, tokens) and the marking values as bindings.
func EvaluateInvariant(expr string, marking Marking) (bool, error) {
	if expr == "" {
		return true, nil // Empty invariant always holds
	}

	// Create bindings from marking (place values accessible directly)
	bindings := make(map[string]any)
	for placeID, count := range marking {
		bindings[placeID] = int64(count)
	}

	// Create aggregate functions bound to this marking
	funcs := MakeAggregates(marking)

	return Evaluate(expr, bindings, funcs)
}

// MakeAggregates creates aggregate functions bound to a specific marking.
// These are used for invariant evaluation.
func MakeAggregates(marking Marking) map[string]GuardFunc {
	return map[string]GuardFunc{
		"sum":    makeSumFunc(marking),
		"count":  makeCountFunc(marking),
		"tokens": makeTokensFunc(marking),
		"minOf":  makeMinOfFunc(marking),
		"maxOf":  makeMaxOfFunc(marking),
	}
}

// makeSumFunc returns a function that sums all values in places matching a prefix.
// Usage: sum("balances") - sums all places starting with "balances"
func makeSumFunc(marking Marking) GuardFunc {
	return func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("sum requires 1 argument (place prefix)")
		}

		prefix, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("sum argument must be a string, got %T", args[0])
		}

		var total int64
		for placeID, count := range marking {
			if strings.HasPrefix(placeID, prefix) || placeID == prefix {
				total += int64(count)
			}
		}
		return total, nil
	}
}

// makeCountFunc returns a function that counts non-zero places matching a prefix.
// Usage: count("balances") - counts places with non-zero tokens
func makeCountFunc(marking Marking) GuardFunc {
	return func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("count requires 1 argument (place prefix)")
		}

		prefix, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("count argument must be a string, got %T", args[0])
		}

		var count int64
		for placeID, tokens := range marking {
			if (strings.HasPrefix(placeID, prefix) || placeID == prefix) && tokens > 0 {
				count++
			}
		}
		return count, nil
	}
}

// makeTokensFunc returns a function that gets the token count at a specific place.
// Usage: tokens("totalSupply") - gets exact place value
func makeTokensFunc(marking Marking) GuardFunc {
	return func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("tokens requires 1 argument (place ID)")
		}

		placeID, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("tokens argument must be a string, got %T", args[0])
		}

		return int64(marking[placeID]), nil
	}
}

// makeMinOfFunc returns a function that finds the minimum value among places matching a prefix.
// Usage: minOf("balances") - minimum value of places starting with "balances"
func makeMinOfFunc(marking Marking) GuardFunc {
	return func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("minOf requires 1 argument (place prefix)")
		}

		prefix, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("minOf argument must be a string, got %T", args[0])
		}

		var minVal int64
		found := false
		for placeID, count := range marking {
			if strings.HasPrefix(placeID, prefix) || placeID == prefix {
				if !found || int64(count) < minVal {
					minVal = int64(count)
					found = true
				}
			}
		}
		return minVal, nil
	}
}

// makeMaxOfFunc returns a function that finds the maximum value among places matching a prefix.
// Usage: maxOf("balances") - maximum value of places starting with "balances"
func makeMaxOfFunc(marking Marking) GuardFunc {
	return func(args ...any) (any, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("maxOf requires 1 argument (place prefix)")
		}

		prefix, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("maxOf argument must be a string, got %T", args[0])
		}

		var maxVal int64
		for placeID, count := range marking {
			if strings.HasPrefix(placeID, prefix) || placeID == prefix {
				if int64(count) > maxVal {
					maxVal = int64(count)
				}
			}
		}
		return maxVal, nil
	}
}
