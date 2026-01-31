// Package feedback generates structured prompts for LLM refinement.
package feedback

import (
	"fmt"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Builder creates structured feedback for LLM refinement.
type Builder struct {
	requirements string
	model        *metamodel.Model
	result       *metamodel.ValidationResult
}

// New creates a FeedbackBuilder.
func New(requirements string, model *metamodel.Model, result *metamodel.ValidationResult) *Builder {
	return &Builder{
		requirements: requirements,
		model:        model,
		result:       result,
	}
}

// Build generates a FeedbackPrompt for LLM refinement.
func (b *Builder) Build() *metamodel.FeedbackPrompt {
	return &metamodel.FeedbackPrompt{
		OriginalRequirements: b.requirements,
		CurrentModel:         b.model,
		ValidationResult:     b.result,
		Instructions:         b.generateInstructions(),
	}
}

func (b *Builder) generateInstructions() string {
	var instructions []string

	// Priority 1: Fix errors
	for _, err := range b.result.Errors {
		instruction := fmt.Sprintf("FIX: %s", err.Message)
		if err.Fix != "" {
			instruction += fmt.Sprintf(" - Suggestion: %s", err.Fix)
		}
		if err.Element != "" {
			instruction += fmt.Sprintf(" (element: %s)", err.Element)
		}
		instructions = append(instructions, instruction)
	}

	// Priority 2: Address warnings
	for _, warn := range b.result.Warnings {
		instruction := fmt.Sprintf("CONSIDER: %s", warn.Message)
		if warn.Fix != "" {
			instruction += fmt.Sprintf(" - Suggestion: %s", warn.Fix)
		}
		instructions = append(instructions, instruction)
	}

	// Priority 3: Improve based on analysis
	if b.result.Analysis != nil {
		analysis := b.result.Analysis

		// Isolated elements
		if len(analysis.Isolated) > 0 {
			instructions = append(instructions,
				fmt.Sprintf("REVIEW: Isolated elements detected: %v - consider removing or connecting them",
					analysis.Isolated))
		}

		// Symmetry insights
		if len(analysis.SymmetryGroups) > 0 {
			for _, group := range analysis.SymmetryGroups {
				instructions = append(instructions,
					fmt.Sprintf("INFO: Symmetry group detected: %v (impact: %.3f) - these elements behave identically",
						group.Elements, group.Impact))
			}
		}

		// Critical elements
		var critical []string
		for _, elem := range analysis.Importance {
			if elem.Category == "critical" {
				critical = append(critical, fmt.Sprintf("%s (%.2f)", elem.ID, elem.Importance))
			}
		}
		if len(critical) > 0 {
			instructions = append(instructions,
				fmt.Sprintf("INFO: Critical elements identified: %v - ensure these are well-tested",
					critical))
		}
	}

	if len(instructions) == 0 {
		return "The model validates successfully. No changes required."
	}

	return strings.Join(instructions, "\n")
}

// NeedsRefinement returns true if the validation result indicates the model needs work.
func NeedsRefinement(result *metamodel.ValidationResult) bool {
	return len(result.Errors) > 0 || len(result.Warnings) > 0
}

// Severity returns a priority level for the validation result.
func Severity(result *metamodel.ValidationResult) string {
	if len(result.Errors) > 0 {
		return "error"
	}
	if len(result.Warnings) > 0 {
		return "warning"
	}
	return "ok"
}
