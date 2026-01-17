# Petri Pilot

LLM-augmented Petri net model design and validation tool.

## Quick Start

```bash
# Set API key
export ANTHROPIC_API_KEY="your-key"

# Generate a model
petri-pilot generate "A checkout workflow with cart, payment, and shipping"

# Validate a model
petri-pilot validate model.json

# Generate with auto-refinement
petri-pilot generate -auto -v "User registration with email verification"
```

## Architecture

```
petri-pilot/
├── cmd/petri-pilot/main.go   # CLI entry point
├── pkg/
│   ├── schema/schema.go      # Model IR and validation types
│   ├── generator/generator.go # LLM-based model generation
│   ├── validator/validator.go # Formal validation (go-pflow)
│   └── feedback/feedback.go   # Structured refinement prompts
├── internal/
│   └── llm/
│       ├── client.go         # Provider interface
│       └── claude.go         # Claude implementation
└── examples/                  # Sample models
```

## Core Loop

```
Requirements → Claude → Model → Validator → Feedback
     ↑                                         │
     └─────────────────────────────────────────┘
```

1. **Generate**: Claude creates Petri net from natural language
2. **Validate**: go-pflow checks structure, reachability, sensitivity
3. **Feedback**: Errors/warnings formatted as refinement instructions
4. **Refine**: Claude fixes issues based on feedback
5. **Repeat**: Until validation passes

## CLI Reference

### generate

```bash
petri-pilot generate [options] "requirements"

Options:
  -o file       Output to file (default: stdout)
  -auto         Auto-validate and refine until valid
  -max-iter N   Max refinement iterations (default: 3)
  -v            Verbose output
  -model name   Claude model (default: claude-sonnet-4-20250514)
```

### validate

```bash
petri-pilot validate [options] model.json

Options:
  -full         Include sensitivity analysis
  -json         Output as JSON
```

### refine

```bash
petri-pilot refine [options] model.json "instructions"

Options:
  -o file       Output file (default: overwrite input)
  -v            Verbose output
  -model name   Claude model
```

## Key Types

### schema.Model

```go
type Model struct {
    Name        string       `json:"name"`
    Description string       `json:"description,omitempty"`
    Places      []Place      `json:"places"`
    Transitions []Transition `json:"transitions"`
    Arcs        []Arc        `json:"arcs"`
    Constraints []Constraint `json:"constraints,omitempty"`
}

type Place struct {
    ID          string `json:"id"`
    Description string `json:"description,omitempty"`
    Initial     int    `json:"initial"`
}

type Transition struct {
    ID          string `json:"id"`
    Description string `json:"description,omitempty"`
    Guard       string `json:"guard,omitempty"`
}

type Arc struct {
    From   string `json:"from"`
    To     string `json:"to"`
    Weight int    `json:"weight,omitempty"`
}
```

### schema.ValidationResult

```go
type ValidationResult struct {
    Valid    bool              `json:"valid"`
    Errors   []ValidationError `json:"errors,omitempty"`
    Warnings []ValidationError `json:"warnings,omitempty"`
    Analysis *AnalysisResult   `json:"analysis,omitempty"`
}
```

## Validation Pipeline

| Stage | Package | Checks |
|-------|---------|--------|
| Structural | validator | Empty model, unconnected elements, invalid refs |
| Behavioral | reachability | Deadlocks, liveness, boundedness |
| Sensitivity | metamodel/petri | Element importance, symmetry groups |

## Error Codes

| Code | Severity | Meaning |
|------|----------|---------|
| NO_PLACES | Error | Model has no places |
| NO_TRANSITIONS | Error | Model has no transitions |
| UNCONNECTED_PLACE | Error | Place has no arcs |
| UNCONNECTED_TRANSITION | Error | Transition has no arcs |
| INVALID_ARC_SOURCE | Error | Arc references unknown source |
| INVALID_ARC_TARGET | Error | Arc references unknown target |
| DEADLOCK_DETECTED | Warning | Terminal states exist (may be intentional) |

## LLM Provider Interface

```go
// internal/llm/client.go
type Client interface {
    Complete(ctx context.Context, req Request) (*Response, error)
}

type Request struct {
    Prompt      string
    Temperature float64
    MaxTokens   int
    System      string
}
```

### Adding New Providers

1. Create `internal/llm/<provider>.go`
2. Implement `Client` interface
3. Add initialization in `cmd/petri-pilot/main.go`

## Claude Integration

Uses official SDK: `github.com/anthropics/anthropic-sdk-go`

```go
// internal/llm/claude.go
client := llm.NewClaudeClient(llm.ClaudeOptions{
    APIKey: "",  // defaults to ANTHROPIC_API_KEY env
    Model:  "claude-sonnet-4-20250514",
})
```

### System Prompt

The generator uses a Petri net expert system prompt that:
- Explains places, transitions, arcs
- Enforces snake_case naming
- Requires proper connectivity
- Encourages conservation constraints

### JSON Extraction

Response parsing handles:
- JSON in markdown code blocks
- Raw JSON objects
- Nested brace matching

## Development

```bash
# Build
go build ./cmd/petri-pilot

# Test
go test ./...

# Run from source
go run ./cmd/petri-pilot/... generate "test"

# Validate example
go run ./cmd/petri-pilot/... validate examples/order-processing.json
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/pflow-xyz/go-pflow/petri` | Petri net builder |
| `github.com/pflow-xyz/go-pflow/reachability` | State space analysis |
| `github.com/pflow-xyz/go-pflow/metamodel/petri` | Sensitivity analysis |
| `github.com/anthropics/anthropic-sdk-go` | Claude API client |

## Example Model

```json
{
  "name": "order-processing",
  "places": [
    {"id": "received", "initial": 1},
    {"id": "validated", "initial": 0},
    {"id": "shipped", "initial": 0}
  ],
  "transitions": [
    {"id": "validate"},
    {"id": "ship"}
  ],
  "arcs": [
    {"from": "received", "to": "validate"},
    {"from": "validate", "to": "validated"},
    {"from": "validated", "to": "ship"},
    {"from": "ship", "to": "shipped"}
  ]
}
```

## Petri Net Primer

- **Place**: State or resource (holds tokens)
- **Transition**: Action or event (moves tokens)
- **Arc**: Connection (place→transition or transition→place)
- **Token**: Unit of state (integer count)
- **Firing**: Transition consumes input tokens, produces output tokens
- **Deadlock**: State where no transition can fire
- **Liveness**: All transitions can eventually fire
- **Boundedness**: Token counts stay finite

## Workspace Setup

This project uses go.work with go-pflow:

```
/Users/myork/Workspace/
├── go.work           # Workspace file
├── go-pflow/         # Core library
└── petri-pilot/      # This project
```
