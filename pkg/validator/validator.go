// Package validator provides formal validation of Petri net models using go-pflow.
package validator

import (
	"fmt"

	mpetri "github.com/pflow-xyz/go-pflow/metamodel/petri"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/reachability"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Validator performs formal verification on Petri net models.
type Validator struct {
	opts Options
}

// Options configures validation behavior.
type Options struct {
	// MaxStates limits reachability analysis (default: 10000)
	MaxStates int

	// EnableSensitivity runs sensitivity analysis
	EnableSensitivity bool

	// Parallel enables parallel sensitivity analysis
	Parallel bool

	// MaxWorkers for parallel analysis (0 = auto)
	MaxWorkers int
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		MaxStates:         10000,
		EnableSensitivity: true,
		Parallel:          true,
		MaxWorkers:        0,
	}
}

// New creates a Validator with the given options.
func New(opts Options) *Validator {
	return &Validator{opts: opts}
}

// Validate performs full validation on a model.
func (v *Validator) Validate(model *schema.Model) (*schema.ValidationResult, error) {
	result := &schema.ValidationResult{Valid: true}

	// Structural validation
	if errs := v.validateStructure(model); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
		result.Valid = false
	}

	// Build go-pflow net
	net, err := v.buildNet(model)
	if err != nil {
		result.Errors = append(result.Errors, schema.ValidationError{
			Code:    "BUILD_FAILED",
			Message: err.Error(),
		})
		result.Valid = false
		return result, nil
	}

	// Reachability analysis
	analysis, err := v.analyzeReachability(net)
	if err != nil {
		result.Warnings = append(result.Warnings, schema.ValidationError{
			Code:    "REACHABILITY_FAILED",
			Message: err.Error(),
		})
	} else {
		result.Analysis = analysis
		if analysis.HasDeadlocks {
			result.Warnings = append(result.Warnings, schema.ValidationError{
				Code:    "DEADLOCK_DETECTED",
				Message: fmt.Sprintf("Model has %d deadlock state(s)", len(analysis.Deadlocks)),
				Fix:     "Add transitions to escape deadlock states or verify this is intended terminal behavior",
			})
		}
	}

	// Sensitivity analysis
	if v.opts.EnableSensitivity && result.Analysis != nil {
		if sens := v.analyzeSensitivity(net); sens != nil {
			result.Analysis.SymmetryGroups = sens.SymmetryGroups
			result.Analysis.Isolated = sens.Isolated
			result.Analysis.Importance = sens.Importance
		}
	}

	return result, nil
}

func (v *Validator) validateStructure(model *schema.Model) []schema.ValidationError {
	var errs []schema.ValidationError

	// Check for empty model
	if len(model.Places) == 0 {
		errs = append(errs, schema.ValidationError{
			Code:    "NO_PLACES",
			Message: "Model has no places",
			Fix:     "Add at least one place to represent state",
		})
	}
	if len(model.Transitions) == 0 {
		errs = append(errs, schema.ValidationError{
			Code:    "NO_TRANSITIONS",
			Message: "Model has no transitions",
			Fix:     "Add at least one transition to represent actions",
		})
	}

	// Check for unconnected elements
	connected := make(map[string]bool)
	for _, arc := range model.Arcs {
		connected[arc.From] = true
		connected[arc.To] = true
	}

	for _, p := range model.Places {
		if !connected[p.ID] {
			errs = append(errs, schema.ValidationError{
				Code:    "UNCONNECTED_PLACE",
				Message: fmt.Sprintf("Place '%s' has no arcs", p.ID),
				Element: p.ID,
				Fix:     "Connect this place to a transition or remove it",
			})
		}
	}

	for _, t := range model.Transitions {
		if !connected[t.ID] {
			errs = append(errs, schema.ValidationError{
				Code:    "UNCONNECTED_TRANSITION",
				Message: fmt.Sprintf("Transition '%s' has no arcs", t.ID),
				Element: t.ID,
				Fix:     "Connect this transition to places or remove it",
			})
		}
	}

	// Check for invalid arc references
	elements := make(map[string]bool)
	for _, p := range model.Places {
		elements[p.ID] = true
	}
	for _, t := range model.Transitions {
		elements[t.ID] = true
	}

	for _, arc := range model.Arcs {
		if !elements[arc.From] {
			errs = append(errs, schema.ValidationError{
				Code:    "INVALID_ARC_SOURCE",
				Message: fmt.Sprintf("Arc references unknown source '%s'", arc.From),
				Fix:     "Define the missing place or transition",
			})
		}
		if !elements[arc.To] {
			errs = append(errs, schema.ValidationError{
				Code:    "INVALID_ARC_TARGET",
				Message: fmt.Sprintf("Arc references unknown target '%s'", arc.To),
				Fix:     "Define the missing place or transition",
			})
		}
	}

	return errs
}

func (v *Validator) buildNet(model *schema.Model) (*petri.PetriNet, error) {
	builder := petri.Build()

	for _, p := range model.Places {
		builder = builder.Place(p.ID, float64(p.Initial))
	}

	for _, t := range model.Transitions {
		builder = builder.Transition(t.ID)
	}

	for _, arc := range model.Arcs {
		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}
		builder = builder.Arc(arc.From, arc.To, float64(weight))
	}

	return builder.Done(), nil
}

func (v *Validator) analyzeReachability(net *petri.PetriNet) (*schema.AnalysisResult, error) {
	analyzer := reachability.NewAnalyzer(net).WithMaxStates(v.opts.MaxStates)
	result := analyzer.Analyze()

	analysis := &schema.AnalysisResult{
		Bounded:      result.Bounded,
		Live:         result.Live,
		HasDeadlocks: result.HasDeadlock,
		StateCount:   result.StateCount,
	}

	// Convert deadlock states to string representation
	for _, dl := range result.Deadlocks {
		analysis.Deadlocks = append(analysis.Deadlocks, fmt.Sprintf("%v", dl))
	}

	return analysis, nil
}

func (v *Validator) analyzeSensitivity(net *petri.PetriNet) *schema.AnalysisResult {
	// Build metamodel for sensitivity analysis
	model := mpetri.FromPetriNet(net)

	opts := mpetri.DefaultSensitivityOptions()
	// Note: Parallel and MaxWorkers removed in go-pflow v0.6.0

	sensResult := model.AnalyzeSensitivity(opts)

	analysis := &schema.AnalysisResult{}

	// Group by impact for symmetry detection
	impactGroups := make(map[float64][]string)
	for _, elem := range sensResult.Elements {
		// Round to 3 decimal places for grouping
		roundedImpact := float64(int(elem.Impact*1000)) / 1000
		impactGroups[roundedImpact] = append(impactGroups[roundedImpact], elem.ID)

		analysis.Importance = append(analysis.Importance, schema.ElementAnalysis{
			ID:         elem.ID,
			Type:       elem.Type,
			Importance: elem.Impact,
			Category:   elem.Category,
		})

		if elem.Category == "redundant" || elem.Impact == 0 {
			analysis.Isolated = append(analysis.Isolated, elem.ID)
		}
	}

	// Build symmetry groups (elements with identical impact)
	for impact, elements := range impactGroups {
		if len(elements) > 1 {
			analysis.SymmetryGroups = append(analysis.SymmetryGroups, schema.SymmetryGroup{
				Elements: elements,
				Impact:   impact,
			})
		}
	}

	return analysis
}
