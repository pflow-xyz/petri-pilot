package dsl

import (
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

// Evaluator implements metamodel.GuardEvaluator using the DSL package.
type Evaluator struct{}

// NewEvaluator creates a new DSL-based guard evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate evaluates a guard expression with bindings.
func (e *Evaluator) Evaluate(expr string, bindings metamodel.Bindings, funcs map[string]metamodel.GuardFunc) (bool, error) {
	// Convert metamodel.Bindings to map[string]any
	bindingsMap := make(map[string]any, len(bindings))
	for k, v := range bindings {
		bindingsMap[k] = v
	}

	// Convert metamodel.GuardFunc to dsl.GuardFunc
	var dslFuncs map[string]GuardFunc
	if funcs != nil {
		dslFuncs = make(map[string]GuardFunc, len(funcs))
		for k, f := range funcs {
			f := f // capture loop variable
			dslFuncs[k] = func(args ...any) (any, error) {
				return f(args...)
			}
		}
	}

	return Evaluate(expr, bindingsMap, dslFuncs)
}

// EvaluateConstraint evaluates a constraint expression against token counts.
func (e *Evaluator) EvaluateConstraint(expr string, tokens map[string]int) (bool, error) {
	return EvaluateInvariant(expr, Marking(tokens))
}

// EvaluateConstraintWithData evaluates a constraint against token counts and
// data-state values together, implementing metamodel.DataAwareEvaluator.
func (e *Evaluator) EvaluateConstraintWithData(expr string, tokens map[string]int, data map[string]any) (bool, error) {
	return EvaluateInvariantWithData(expr, Marking(tokens), data)
}

// DataAggregates is Aggregates with the data states in reach: sum("points")
// or sum(points) totals a ledger map, count counts its keys, minOf/maxOf
// range over its values.
func (e *Evaluator) DataAggregates(tokens map[string]int, data map[string]any) map[string]metamodel.GuardFunc {
	aggregates := MakeAggregatesWithData(Marking(tokens), data)

	out := make(map[string]metamodel.GuardFunc, len(aggregates))
	for name, fn := range aggregates {
		fn := fn // capture loop variable
		out[name] = func(args ...any) (any, error) { return fn(args...) }
	}
	return out
}

// Ensure Evaluator implements metamodel.GuardEvaluator and the optional
// data-aware extension.
var _ metamodel.GuardEvaluator = (*Evaluator)(nil)
var _ metamodel.DataAwareEvaluator = (*Evaluator)(nil)

// Aggregates exposes the marking-aware guard functions — tokens, sum, count,
// minOf, maxOf — to transition guards, implementing metamodel.MarkingAggregator.
//
// These were previously reachable only from constraint/invariant evaluation, so
// a transition guard that read the marking silently failed to resolve.
func (e *Evaluator) Aggregates(tokens map[string]int) map[string]metamodel.GuardFunc {
	aggregates := MakeAggregates(Marking(tokens))

	out := make(map[string]metamodel.GuardFunc, len(aggregates))
	for name, fn := range aggregates {
		fn := fn // capture loop variable
		out[name] = func(args ...any) (any, error) { return fn(args...) }
	}
	return out
}

// Ensure Evaluator also provides marking-aware guard functions.
var _ metamodel.MarkingAggregator = (*Evaluator)(nil)
