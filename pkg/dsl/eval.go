package dsl

import (
	"fmt"
)

// GuardFunc is a function that can be called from guard expressions.
type GuardFunc func(args ...any) (any, error)

// Context holds bindings and functions for guard evaluation.
type Context struct {
	Bindings map[string]any
	Funcs    map[string]GuardFunc
}

// NewContext creates a new evaluation context.
func NewContext() *Context {
	return &Context{
		Bindings: make(map[string]any),
		Funcs:    make(map[string]GuardFunc),
	}
}

// Eval evaluates an AST node in the given context.
func Eval(node Node, ctx *Context) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("nil node")
	}

	switch n := node.(type) {
	case *BoolLit:
		return n.Value, nil

	case *NumberLit:
		return n.Value, nil

	case *FloatLit:
		return n.Value, nil

	case *StringLit:
		return n.Value, nil

	case *Identifier:
		val, ok := ctx.Bindings[n.Name]
		if !ok {
			return nil, fmt.Errorf("unknown identifier: %s", n.Name)
		}
		return val, nil

	case *UnaryOp:
		operand, err := Eval(n.Operand, ctx)
		if err != nil {
			return nil, err
		}
		return evalUnary(n.Op, operand)

	case *BinaryOp:
		// Short-circuit evaluation for && and ||
		if n.Op == "&&" {
			left, err := Eval(n.Left, ctx)
			if err != nil {
				return nil, err
			}
			leftBool, ok := toBool(left)
			if !ok {
				return nil, fmt.Errorf("left operand of && must be boolean")
			}
			if !leftBool {
				return false, nil
			}
			right, err := Eval(n.Right, ctx)
			if err != nil {
				return nil, err
			}
			rightBool, ok := toBool(right)
			if !ok {
				return nil, fmt.Errorf("right operand of && must be boolean")
			}
			return rightBool, nil
		}

		if n.Op == "||" {
			left, err := Eval(n.Left, ctx)
			if err != nil {
				return nil, err
			}
			leftBool, ok := toBool(left)
			if !ok {
				return nil, fmt.Errorf("left operand of || must be boolean")
			}
			if leftBool {
				return true, nil
			}
			right, err := Eval(n.Right, ctx)
			if err != nil {
				return nil, err
			}
			rightBool, ok := toBool(right)
			if !ok {
				return nil, fmt.Errorf("right operand of || must be boolean")
			}
			return rightBool, nil
		}

		left, err := Eval(n.Left, ctx)
		if err != nil {
			return nil, err
		}
		right, err := Eval(n.Right, ctx)
		if err != nil {
			return nil, err
		}
		return evalBinary(n.Op, left, right)

	case *IndexExpr:
		obj, err := Eval(n.Object, ctx)
		if err != nil {
			return nil, err
		}
		index, err := Eval(n.Index, ctx)
		if err != nil {
			return nil, err
		}
		return evalIndex(obj, index)

	case *FieldExpr:
		obj, err := Eval(n.Object, ctx)
		if err != nil {
			return nil, err
		}
		return evalField(obj, n.Field)

	case *CallExpr:
		fn, ok := ctx.Funcs[n.Func]
		if !ok {
			return nil, fmt.Errorf("unknown function: %s", n.Func)
		}
		args := make([]any, len(n.Args))
		for i, arg := range n.Args {
			val, err := Eval(arg, ctx)
			if err != nil {
				return nil, err
			}
			args[i] = val
		}
		return fn(args...)

	default:
		return nil, fmt.Errorf("unknown node type: %T", node)
	}
}

func evalUnary(op string, operand any) (any, error) {
	switch op {
	case "!":
		b, ok := toBool(operand)
		if !ok {
			return nil, fmt.Errorf("operand of ! must be boolean")
		}
		return !b, nil
	case "-":
		n, ok := toNumber(operand)
		if !ok {
			return nil, fmt.Errorf("operand of unary - must be numeric")
		}
		return -n, nil
	default:
		return nil, fmt.Errorf("unknown unary operator: %s", op)
	}
}

func evalBinary(op string, left, right any) (any, error) {
	switch op {
	case "+", "-", "*", "/", "%":
		return evalArithmetic(op, left, right)
	case ">", "<", ">=", "<=":
		return evalRelational(op, left, right)
	case "==", "!=":
		return evalEquality(op, left, right)
	default:
		return nil, fmt.Errorf("unknown binary operator: %s", op)
	}
}

func evalArithmetic(op string, left, right any) (any, error) {
	l, lok := toNumber(left)
	r, rok := toNumber(right)
	if !lok || !rok {
		// Try string concatenation for +
		if op == "+" {
			ls, lsok := left.(string)
			rs, rsok := right.(string)
			if lsok && rsok {
				return ls + rs, nil
			}
		}
		return nil, fmt.Errorf("arithmetic operands must be numeric")
	}

	switch op {
	case "+":
		return l + r, nil
	case "-":
		return l - r, nil
	case "*":
		return l * r, nil
	case "/":
		if r == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return l / r, nil
	case "%":
		if r == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return int64(l) % int64(r), nil
	default:
		return nil, fmt.Errorf("unknown arithmetic operator: %s", op)
	}
}

func evalRelational(op string, left, right any) (any, error) {
	l, lok := toNumber(left)
	r, rok := toNumber(right)
	if !lok || !rok {
		return nil, fmt.Errorf("relational operands must be numeric")
	}

	switch op {
	case ">":
		return l > r, nil
	case "<":
		return l < r, nil
	case ">=":
		return l >= r, nil
	case "<=":
		return l <= r, nil
	default:
		return nil, fmt.Errorf("unknown relational operator: %s", op)
	}
}

func evalEquality(op string, left, right any) (any, error) {
	equal := compareValues(left, right)
	if op == "==" {
		return equal, nil
	}
	return !equal, nil
}

func compareValues(left, right any) bool {
	// Try numeric comparison
	l, lok := toNumber(left)
	r, rok := toNumber(right)
	if lok && rok {
		return l == r
	}

	// Try boolean comparison
	lb, lok := toBool(left)
	rb, rok := toBool(right)
	if lok && rok {
		return lb == rb
	}

	// Try string comparison
	ls, lok := left.(string)
	rs, rok := right.(string)
	if lok && rok {
		return ls == rs
	}

	// Fallback to interface comparison
	return left == right
}

func evalIndex(obj, index any) (any, error) {
	// Handle nil or zero value - return 0 (default value for missing keys)
	if obj == nil {
		return int64(0), nil
	}

	// Handle numeric types that might come from missing nested map access
	if _, ok := toNumber(obj); ok {
		return int64(0), nil
	}

	switch o := obj.(type) {
	case map[string]any:
		key, ok := toString(index)
		if !ok {
			return nil, fmt.Errorf("map index must be string")
		}
		val, exists := o[key]
		if !exists {
			return int64(0), nil // Default to 0 for missing keys
		}
		return val, nil

	case map[string]int64:
		key, ok := toString(index)
		if !ok {
			return nil, fmt.Errorf("map index must be string")
		}
		val, exists := o[key]
		if !exists {
			return int64(0), nil
		}
		return val, nil

	case map[string]int:
		key, ok := toString(index)
		if !ok {
			return nil, fmt.Errorf("map index must be string")
		}
		val, exists := o[key]
		if !exists {
			return int64(0), nil
		}
		return int64(val), nil

	case map[string]map[string]int64:
		key, ok := toString(index)
		if !ok {
			return nil, fmt.Errorf("map index must be string")
		}
		val, exists := o[key]
		if !exists {
			return make(map[string]int64), nil // Return empty map for missing nested keys
		}
		return val, nil

	case map[string]map[string]any:
		key, ok := toString(index)
		if !ok {
			return nil, fmt.Errorf("map index must be string")
		}
		val, exists := o[key]
		if !exists {
			return make(map[string]any), nil
		}
		return val, nil

	default:
		return nil, fmt.Errorf("cannot index type %T", obj)
	}
}

func evalField(obj any, field string) (any, error) {
	switch o := obj.(type) {
	case map[string]any:
		val, exists := o[field]
		if !exists {
			return nil, fmt.Errorf("field not found: %s", field)
		}
		return val, nil

	default:
		return nil, fmt.Errorf("cannot access field on type %T", obj)
	}
}

func toBool(v any) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case int64:
		return val != 0, true
	case int:
		return val != 0, true
	case float64:
		return val != 0, true
	default:
		return false, false
	}
}

func toNumber(v any) (float64, bool) {
	switch val := v.(type) {
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case float64:
		return val, true
	case float32:
		return float64(val), true
	default:
		return 0, false
	}
}

func toString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case int:
		return fmt.Sprintf("%d", val), true
	case int64:
		return fmt.Sprintf("%d", val), true
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val)), true
		}
		return fmt.Sprintf("%g", val), true
	default:
		return "", false
	}
}
