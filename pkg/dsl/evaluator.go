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

// Ensure Evaluator implements metamodel.GuardEvaluator
var _ metamodel.GuardEvaluator = (*Evaluator)(nil)
