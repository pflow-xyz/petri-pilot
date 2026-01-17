// Package generator provides LLM-based Petri net model generation.
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pflow-xyz/petri-pilot/internal/llm"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Generator creates Petri net models from natural language requirements.
type Generator struct {
	client llm.Client
	opts   Options
}

// Options configures the generator behavior.
type Options struct {
	// MaxIterations limits refinement loops (default: 5)
	MaxIterations int

	// Temperature for LLM generation (default: 0.7)
	Temperature float64

	// Verbose enables detailed output
	Verbose bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		MaxIterations: 5,
		Temperature:   0.7,
		Verbose:       false,
	}
}

// New creates a Generator with the given LLM client.
func New(client llm.Client, opts Options) *Generator {
	return &Generator{
		client: client,
		opts:   opts,
	}
}

// Generate creates a Petri net model from natural language requirements.
func (g *Generator) Generate(ctx context.Context, requirements string) (*schema.Model, error) {
	prompt := buildGenerationPrompt(requirements)

	response, err := g.client.Complete(ctx, llm.Request{
		Prompt:      prompt,
		Temperature: g.opts.Temperature,
		MaxTokens:   4096,
		System:      systemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("llm generation failed: %w", err)
	}

	model, err := parseModelResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse model: %w", err)
	}

	return model, nil
}

// Refine improves a model based on validation feedback.
func (g *Generator) Refine(ctx context.Context, feedback *schema.FeedbackPrompt) (*schema.Model, error) {
	prompt := buildRefinementPrompt(feedback)

	response, err := g.client.Complete(ctx, llm.Request{
		Prompt:      prompt,
		Temperature: g.opts.Temperature * 0.8, // slightly lower for refinement
		MaxTokens:   4096,
		System:      systemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("llm refinement failed: %w", err)
	}

	model, err := parseModelResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse refined model: %w", err)
	}

	return model, nil
}

const systemPrompt = `You are a Petri net modeling expert. You create formal Petri net models from natural language requirements.

A Petri net consists of:
- Places: States or resources that hold tokens
- Transitions: Actions or events that move tokens
- Arcs: Connections from places to transitions (inputs) or transitions to places (outputs)

Guidelines:
1. Use snake_case for all IDs
2. Places represent states, resources, or conditions
3. Transitions represent actions, events, or state changes
4. Every transition must have at least one input arc and one output arc
5. Consider error states and alternative paths
6. Include conservation constraints where tokens are preserved
7. Initial markings should reflect the starting state

Always output valid JSON matching the schema exactly.`

func buildGenerationPrompt(requirements string) string {
	return fmt.Sprintf(`Create a Petri net model for the following requirements:

REQUIREMENTS:
%s

OUTPUT FORMAT (strict JSON):
{
  "name": "model-name",
  "description": "brief description",
  "places": [
    {"id": "place_name", "description": "what this place represents", "initial": 1}
  ],
  "transitions": [
    {"id": "transition_name", "description": "what this action does"}
  ],
  "arcs": [
    {"from": "place_name", "to": "transition_name", "weight": 1}
  ],
  "constraints": [
    {"id": "constraint_name", "expr": "place1 + place2 == constant"}
  ]
}

Generate only the JSON model, no additional text:`, requirements)
}

func buildRefinementPrompt(feedback *schema.FeedbackPrompt) string {
	currentModelJSON, _ := json.MarshalIndent(feedback.CurrentModel, "", "  ")

	var issues strings.Builder
	for _, err := range feedback.ValidationResult.Errors {
		issues.WriteString(fmt.Sprintf("ERROR: %s", err.Message))
		if err.Fix != "" {
			issues.WriteString(fmt.Sprintf(" (Fix: %s)", err.Fix))
		}
		issues.WriteString("\n")
	}
	for _, warn := range feedback.ValidationResult.Warnings {
		issues.WriteString(fmt.Sprintf("WARNING: %s", warn.Message))
		if warn.Fix != "" {
			issues.WriteString(fmt.Sprintf(" (Fix: %s)", warn.Fix))
		}
		issues.WriteString("\n")
	}

	return fmt.Sprintf(`Refine the following Petri net model based on validation feedback.

ORIGINAL REQUIREMENTS:
%s

CURRENT MODEL:
%s

VALIDATION ISSUES:
%s

INSTRUCTIONS:
%s

Generate an improved JSON model that fixes all errors and addresses warnings.
Output only the JSON model, no additional text:`,
		feedback.OriginalRequirements,
		string(currentModelJSON),
		issues.String(),
		feedback.Instructions)
}

// parseModelResponse extracts and parses JSON from the LLM response.
func parseModelResponse(content string) (*schema.Model, error) {
	// Try to find JSON in the response
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var model schema.Model
	if err := json.Unmarshal([]byte(jsonStr), &model); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	// Validate basic structure
	if model.Name == "" {
		model.Name = "generated-model"
	}

	return &model, nil
}

// extractJSON finds and extracts JSON from text that may contain markdown or other content.
func extractJSON(content string) string {
	// Try to find JSON in code blocks first
	codeBlockRe := regexp.MustCompile("```(?:json)?\\s*\\n?([\\s\\S]*?)```")
	if matches := codeBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find raw JSON object
	start := strings.Index(content, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}

	return ""
}
