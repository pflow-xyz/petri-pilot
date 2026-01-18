# Petri Pilot

From requirements to running applications, through verified Petri net models.

**Principle**: LLM designs models. Deterministic codegen produces applications. No LLM-generated code.

## Quick Start

```bash
# Run as MCP server (for Claude Desktop, Cursor, etc.)
petri-pilot mcp

# Or use CLI directly
export ANTHROPIC_API_KEY="your-key"

# Generate and validate a model
petri-pilot generate -auto "order processing workflow"

# Generate application code
petri-pilot codegen model.json -lang go -o ./myworkflow/
cd myworkflow && go test ./...
```

## Architecture

```
petri-pilot/
├── cmd/petri-pilot/main.go      # CLI entry point
├── pkg/
│   ├── schema/schema.go         # Model IR and validation types
│   ├── generator/generator.go   # LLM-based model generation
│   ├── validator/validator.go   # Formal validation (go-pflow)
│   ├── feedback/feedback.go     # Structured refinement prompts
│   ├── mcp/server.go            # MCP server implementation
│   ├── bridge/converter.go      # schema.Model → metamodel.Schema
│   ├── codegen/
│   │   ├── golang/              # Go code generator
│   │   └── openapi/             # OpenAPI spec generator
│   └── runtime/
│       ├── eventstore/          # EventStore interface + impls
│       ├── aggregate/           # Aggregate interface + impls
│       └── api/                 # HTTP handler interfaces
├── internal/
│   └── llm/
│       ├── client.go            # Provider interface
│       └── claude.go            # Claude implementation
└── examples/                    # Sample models
```

## Core Pipeline

```
┌─────────────────────────────────────────┐
│  MCP Client (Claude Desktop, Cursor)   │
│  LLM designs model in conversation      │
└─────────────────┬───────────────────────┘
                  │ MCP tools
    ┌─────────────▼─────────────┐
    │  petri-pilot MCP server   │
    │  validate │ analyze │ gen │
    └─────────────┬─────────────┘
                  │ deterministic
    ┌─────────────▼─────────────┐
    │  Generated Application    │
    │  workflow │ events │ api  │
    └─────────────┬─────────────┘
                  │ implements
    ┌─────────────▼─────────────┐
    │  Runtime Interfaces       │
    │  EventStore │ Aggregate   │
    └───────────────────────────┘
```

1. **Design**: LLM creates Petri net from natural language (via MCP or CLI)
2. **Validate**: go-pflow checks structure, reachability, sensitivity
3. **Feedback**: Errors/warnings formatted as refinement instructions
4. **Refine**: LLM fixes issues based on feedback
5. **Generate**: Deterministic codegen produces application

## CLI Reference

### mcp

```bash
petri-pilot mcp

# Exposes tools: petri_validate, petri_analyze, petri_codegen, petri_visualize
```

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

### codegen

```bash
petri-pilot codegen [options] model.json

Options:
  -lang string  Target language: go, javascript (default: go)
  -o dir        Output directory (default: ./generated)
  -api-only     Generate OpenAPI spec only
```

### refine

```bash
petri-pilot refine [options] model.json "instructions"

Options:
  -o file       Output file (default: overwrite input)
  -v            Verbose output
  -model name   Claude model
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `petri_validate` | Validate model, return structured results |
| `petri_analyze` | Run reachability/sensitivity analysis |
| `petri_codegen` | Generate code from validated model |
| `petri_visualize` | Generate SVG diagram |

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

### Runtime Interfaces

```go
// EventStore - append-only event log
type EventStore interface {
    Append(ctx context.Context, streamID string, events []Event) error
    Read(ctx context.Context, streamID string, fromVersion int) ([]Event, error)
    Subscribe(ctx context.Context, handler EventHandler) error
}

// Aggregate - event-sourced state
type Aggregate interface {
    ID() string
    Version() int
    Apply(event Event) error
    State() any
}
```

## Generated Output

From a validated model, codegen produces:

| File | Contents |
|------|----------|
| `workflow.go` | State machine with guards and transition logic |
| `events.go` | Event types derived from transitions |
| `aggregate.go` | Event-sourced aggregate |
| `api.go` | HTTP handlers per transition |
| `api_openapi.yaml` | OpenAPI specification |
| `workflow_test.go` | Tests using SQLite runtime |

## Validation Pipeline

| Stage | Package | Checks |
|-------|---------|--------|
| Structural | validator | Empty model, unconnected elements, invalid refs |
| Behavioral | reachability | Deadlocks, liveness, boundedness |
| Sensitivity | metamodel/petri | Element importance, symmetry groups |
| Implementability | codegen | Event derivation, type consistency, guard parsability |

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

## Target Applications

All share the pattern: **events → transitions → state**

- Workflow engines (order processing, approval flows)
- Smart contracts (tokens, governance, vesting)
- Microservice orchestration (sagas, compensation)
- Game state machines (turn logic, resource management)
- IoT protocols (device state, command sequences)

## Development

```bash
# Build
go build ./cmd/petri-pilot

# Test
go test ./...

# Run MCP server
go run ./cmd/petri-pilot/... mcp

# Generate and validate
go run ./cmd/petri-pilot/... generate -auto "test workflow"

# Generate code
go run ./cmd/petri-pilot/... codegen examples/order-processing.json -o ./out
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/pflow-xyz/go-pflow/petri` | Petri net builder |
| `github.com/pflow-xyz/go-pflow/reachability` | State space analysis |
| `github.com/pflow-xyz/go-pflow/metamodel/petri` | Sensitivity analysis |
| `github.com/anthropics/anthropic-sdk-go` | Claude API client |
| `github.com/mark3labs/mcp-go` | MCP server implementation |
| `github.com/mattn/go-sqlite3` | SQLite driver for runtime |

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
