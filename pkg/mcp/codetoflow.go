package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/internal/llm"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
)

func codeToFlowTool() mcp.Tool {
	return mcp.NewTool("petri_code_to_flow",
		mcp.WithDescription("Convert source code into a formal Petri net model. Analyzes code structure (control flow, state machines, resource management, concurrency) and produces an executable, verifiable Petri net — not just a diagram."),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("Source code to analyze and convert into a Petri net model"),
		),
		mcp.WithString("language",
			mcp.Description("Programming language of the source code (e.g., go, python, javascript, java). Auto-detected if omitted."),
		),
		mcp.WithString("focus",
			mcp.Description("Analysis focus: control-flow (function call sequences), state-machine (state transitions), resources (resource allocation/consumption), concurrency (parallel processes, synchronization). Defaults to auto-detect."),
		),
		mcp.WithString("name",
			mcp.Description("Name for the generated model. Defaults to a name derived from the code."),
		),
	)
}

const codeToFlowSystemPrompt = `You are an expert at analyzing source code and extracting formal Petri net models.

Your task: analyze the provided source code and produce a valid Petri net model in JSON format.

## Petri Net Basics

A Petri net has:
- **Places**: States or conditions (circles). Each has an id, optional description, and initial token count.
- **Transitions**: Actions or events (rectangles). Each has an id and optional description.
- **Arcs**: Directed connections between places and transitions. Arcs go from place→transition (input) or transition→place (output). Optional weight (default 1).

## Output Format

Return ONLY a JSON object (no markdown fences, no explanation) with this structure:

{
  "name": "model-name",
  "description": "What this model represents",
  "places": [
    {"id": "place_id", "description": "What this state represents", "initial": 0}
  ],
  "transitions": [
    {"id": "transition_id", "description": "What this action does"}
  ],
  "arcs": [
    {"from": "place_id", "to": "transition_id"},
    {"from": "transition_id", "to": "another_place_id"}
  ]
}

## Rules

1. Place IDs use snake_case (e.g., order_pending, connection_established)
2. Transition IDs use snake_case verbs (e.g., submit_order, close_connection)
3. Every place must connect to at least one transition via an arc
4. Every transition must have at least one input arc (from a place) and one output arc (to a place)
5. Set initial: 1 on starting states, initial: 0 on others
6. Use arc weights > 1 only when the code explicitly consumes/produces multiple units
7. Keep the model minimal — capture the essential structure, not every code detail

## Analysis Focus Modes

- **control-flow**: Map function calls and branching into sequential/parallel place-transition flows
- **state-machine**: Identify state variables and their transitions (enum states, status fields, FSM patterns)
- **resources**: Model resource pools (connections, inventory, capacity) with consumption/production
- **concurrency**: Model goroutines/threads, channels, mutexes, producer-consumer patterns

## Example: Simple Order Workflow

Given code with states (pending, processing, shipped, delivered) and transitions between them:

{"name":"order-workflow","description":"Order processing lifecycle","places":[{"id":"pending","description":"Order placed, awaiting processing","initial":1},{"id":"processing","description":"Order being prepared","initial":0},{"id":"shipped","description":"Order shipped to customer","initial":0},{"id":"delivered","description":"Order delivered","initial":0}],"transitions":[{"id":"start_processing","description":"Begin order processing"},{"id":"ship_order","description":"Ship the order"},{"id":"confirm_delivery","description":"Confirm order delivered"}],"arcs":[{"from":"pending","to":"start_processing"},{"from":"start_processing","to":"processing"},{"from":"processing","to":"ship_order"},{"from":"ship_order","to":"shipped"},{"from":"shipped","to":"confirm_delivery"},{"from":"confirm_delivery","to":"delivered"}]}

## Example: Producer-Consumer with Buffer

Given code with producers adding to a bounded buffer and consumers taking from it:

{"name":"producer-consumer","description":"Bounded buffer producer-consumer pattern","places":[{"id":"empty_slots","description":"Available buffer slots","initial":5},{"id":"items_in_buffer","description":"Items waiting in buffer","initial":0},{"id":"producer_ready","description":"Producer ready to produce","initial":1},{"id":"consumer_ready","description":"Consumer ready to consume","initial":1}],"transitions":[{"id":"produce","description":"Producer adds item to buffer"},{"id":"consume","description":"Consumer takes item from buffer"}],"arcs":[{"from":"producer_ready","to":"produce"},{"from":"empty_slots","to":"produce"},{"from":"produce","to":"items_in_buffer"},{"from":"produce","to":"producer_ready"},{"from":"consumer_ready","to":"consume"},{"from":"items_in_buffer","to":"consume"},{"from":"consume","to":"empty_slots"},{"from":"consume","to":"consumer_ready"}]}

Return ONLY the JSON model. No explanation, no markdown fences.`

func handleCodeToFlow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError("missing required 'code' parameter"), nil
	}

	language := request.GetString("language", "")
	focus := request.GetString("focus", "")
	name := request.GetString("name", "")

	// Build the user prompt
	var prompt strings.Builder
	prompt.WriteString("Analyze this source code and produce a Petri net model.\n\n")

	if language != "" {
		prompt.WriteString(fmt.Sprintf("Language: %s\n", language))
	}
	if focus != "" {
		prompt.WriteString(fmt.Sprintf("Focus: %s\n", focus))
	}
	if name != "" {
		prompt.WriteString(fmt.Sprintf("Model name: %s\n", name))
	}

	prompt.WriteString("\n```\n")
	prompt.WriteString(code)
	prompt.WriteString("\n```\n")

	// Call Claude
	client := llm.NewClaudeClient(llm.DefaultClaudeOptions())
	resp, err := client.Complete(ctx, llm.Request{
		System:      codeToFlowSystemPrompt,
		Prompt:      prompt.String(),
		MaxTokens:   4096,
		Temperature: 0.2,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("LLM error: %v", err)), nil
	}

	// Extract JSON from response (handle possible markdown fences)
	modelJSON := extractJSON(resp.Content)
	if modelJSON == "" {
		return mcp.NewToolResultError(fmt.Sprintf("LLM did not return valid JSON. Raw response:\n%s", resp.Content)), nil
	}

	// Parse and validate the model
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		// Return the raw JSON with the parse error so the user can see what was generated
		return mcp.NewToolResultError(fmt.Sprintf("Generated model failed to parse: %v\n\nRaw model:\n%s", err, modelJSON)), nil
	}
	model := parsed.Model

	// Validate
	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false
	v := validator.New(opts)
	valResult, err := v.Validate(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}

	// Detect pattern via implementability analysis
	implResult := v.ValidateImplementability(model)

	// Build response
	output := codeToFlowResult{
		Model:      json.RawMessage(modelJSON),
		Valid:      valResult.Valid,
		Pattern:    implResult.Pattern.Type,
		Summary:    buildModelSummary(model),
		PreviewURL: fmt.Sprintf("https://pilot.pflow.xyz/pflow?model=%s", model.Name),
		Usage: codeToFlowUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}

	if !valResult.Valid {
		output.Errors = valResult.Errors
	}
	if len(valResult.Warnings) > 0 {
		output.Warnings = valResult.Warnings
	}

	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(outputJSON)), nil
}

type codeToFlowResult struct {
	Model      json.RawMessage                   `json:"model"`
	Valid      bool                              `json:"valid"`
	Pattern    string                            `json:"pattern,omitempty"`
	Summary    string                            `json:"summary"`
	PreviewURL string                            `json:"preview_url"`
	Errors     []goflowmetamodel.ValidationError `json:"errors,omitempty"`
	Warnings   []goflowmetamodel.ValidationError `json:"warnings,omitempty"`
	Usage      codeToFlowUsage                   `json:"usage"`
}

type codeToFlowUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// extractJSON finds a JSON object in LLM output, handling optional markdown fences.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Try direct parse first
	if strings.HasPrefix(s, "{") {
		if json.Valid([]byte(s)) {
			return s
		}
	}

	// Look for ```json ... ``` fences
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(s[start:], "```"); end >= 0 {
			candidate := strings.TrimSpace(s[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Look for ``` ... ``` fences (without json tag)
	if idx := strings.Index(s, "```"); idx >= 0 {
		start := idx + len("```")
		// Skip to end of first line (the language tag line)
		if nl := strings.Index(s[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(s[start:], "```"); end >= 0 {
			candidate := strings.TrimSpace(s[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Look for first { to last }
	first := strings.Index(s, "{")
	last := strings.LastIndex(s, "}")
	if first >= 0 && last > first {
		candidate := s[first : last+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	return ""
}

// buildModelSummary creates a human-readable summary of the model.
func buildModelSummary(model *goflowmetamodel.Model) string {
	places := len(model.Places)
	transitions := len(model.Transitions)
	arcs := len(model.Arcs)

	var initial []string
	for _, p := range model.Places {
		if p.Initial > 0 {
			initial = append(initial, p.ID)
		}
	}

	summary := fmt.Sprintf("%d places, %d transitions, %d arcs", places, transitions, arcs)
	if len(initial) > 0 {
		summary += fmt.Sprintf(". Initial tokens: %s", strings.Join(initial, ", "))
	}
	return summary
}
