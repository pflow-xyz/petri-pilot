package serve

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/internal/llm"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
)

// codeToFlowSystemPrompt is the same prompt used by the MCP tool.
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

// CodeToFlowRequest is the HTTP request body for the code-to-flow API.
type CodeToFlowRequest struct {
	Code     string `json:"code"`
	Language string `json:"language,omitempty"`
	Focus    string `json:"focus,omitempty"`
	Name     string `json:"name,omitempty"`
}

// CodeToFlowResponse is the HTTP response body for the code-to-flow API.
type CodeToFlowResponse struct {
	Model      json.RawMessage                   `json:"model"`
	Valid      bool                              `json:"valid"`
	Pattern    string                            `json:"pattern,omitempty"`
	Summary    string                            `json:"summary"`
	PreviewURL string                            `json:"preview_url"`
	Errors     []goflowmetamodel.ValidationError `json:"errors,omitempty"`
	Warnings   []goflowmetamodel.ValidationError `json:"warnings,omitempty"`
}

// RegisterCodeToFlowAPI registers the /api/code-to-flow endpoint if ANTHROPIC_API_KEY is set.
func RegisterCodeToFlowAPI(mux *http.ServeMux) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return
	}
	mux.HandleFunc("/api/code-to-flow", handleCodeToFlowHTTP)
	log.Printf("  Code-to-Flow API at /api/code-to-flow")
}

func handleCodeToFlowHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req CodeToFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		http.Error(w, `{"error":"code is required"}`, http.StatusBadRequest)
		return
	}

	// Build the LLM prompt
	var prompt strings.Builder
	prompt.WriteString("Analyze this source code and produce a Petri net model.\n\n")
	if req.Language != "" {
		prompt.WriteString(fmt.Sprintf("Language: %s\n", req.Language))
	}
	if req.Focus != "" {
		prompt.WriteString(fmt.Sprintf("Focus: %s\n", req.Focus))
	}
	if req.Name != "" {
		prompt.WriteString(fmt.Sprintf("Model name: %s\n", req.Name))
	}
	prompt.WriteString("\n```\n")
	prompt.WriteString(req.Code)
	prompt.WriteString("\n```\n")

	// Call Claude
	client := llm.NewClaudeClient(llm.DefaultClaudeOptions())
	resp, err := client.Complete(r.Context(), llm.Request{
		System:      codeToFlowSystemPrompt,
		Prompt:      prompt.String(),
		MaxTokens:   4096,
		Temperature: 0.2,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("LLM error: %v", err)})
		return
	}

	// Extract JSON from response
	modelJSON := extractJSONFromLLM(resp.Content)
	if modelJSON == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "LLM did not return valid JSON", "raw": resp.Content})
		return
	}

	// Parse and validate
	var model goflowmetamodel.Model
	if err := json.Unmarshal([]byte(modelJSON), &model); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("parse error: %v", err), "model": modelJSON})
		return
	}

	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false
	v := validator.New(opts)
	valResult, err := v.Validate(&model)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("validation error: %v", err)})
		return
	}

	implResult := v.ValidateImplementability(&model)

	result := CodeToFlowResponse{
		Model:      json.RawMessage(modelJSON),
		Valid:      valResult.Valid,
		Pattern:    implResult.Pattern.Type,
		Summary:    buildCodeToFlowSummary(&model),
		PreviewURL: fmt.Sprintf("/pflow?model=%s", model.Name),
	}
	if !valResult.Valid {
		result.Errors = valResult.Errors
	}
	if len(valResult.Warnings) > 0 {
		result.Warnings = valResult.Warnings
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// extractJSONFromLLM finds a JSON object in LLM output, handling optional markdown fences.
func extractJSONFromLLM(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "{") && json.Valid([]byte(s)) {
		return s
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

	// Look for ``` ... ``` fences
	if idx := strings.Index(s, "```"); idx >= 0 {
		start := idx + len("```")
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

func buildCodeToFlowSummary(model *goflowmetamodel.Model) string {
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
