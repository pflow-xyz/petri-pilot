# Petri Pilot

From requirements to running applications, through verified Petri net models.

## The Idea

Describe your system in plain language. An LLM designs a formal Petri net model. Validation tools check for deadlocks, unreachable states, and structural errors. Deterministic code generation produces a complete application.

**No LLM-generated code.** The LLM designs models. Codegen produces applications. Same model, same output, every time.

## How It Works

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

## Quick Start

```bash
# Install
go install github.com/pflow-xyz/petri-pilot/cmd/petri-pilot@latest

# Run as MCP server (for Claude Desktop, Cursor, etc.)
petri-pilot mcp

# Or use CLI directly
export ANTHROPIC_API_KEY="your-key"

# Generate a validated model
petri-pilot generate -auto "order processing with validation and shipping"

# Generate application code
petri-pilot codegen model.json -lang go -o ./myworkflow/
cd myworkflow && go test ./...
```

## What Gets Generated

From a single validated model:

| Output | Description |
|--------|-------------|
| `workflow.go` | State machine with guards and transition logic |
| `events.go` | Event types derived from transitions |
| `aggregate.go` | Event-sourced aggregate |
| `api.go` | HTTP handlers per transition |
| `api_openapi.yaml` | OpenAPI specification |
| `workflow_test.go` | Tests using SQLite runtime |

The generated project is complete. Run the tests, wire up your storage, deploy.

## Target Applications

The same architecture serves:

- **Workflow engines** — order processing, approval flows, document pipelines
- **Smart contracts** — tokens, governance, vesting schedules
- **Microservice orchestration** — sagas, compensation, distributed state
- **Game state machines** — turn logic, resource management, multiplayer sync
- **IoT protocols** — device state, command sequences, sensor pipelines

All share: **events → transitions → state**

## Validation

Before code generation, models are validated:

| Check | What It Catches |
|-------|-----------------|
| Structural | Unconnected places, invalid arc references |
| Reachability | Deadlocks, unreachable states |
| Boundedness | Unbounded token accumulation |
| Sensitivity | Critical elements, symmetry groups |

Errors feed back to the LLM for refinement. The loop continues until validation passes.

## CLI Commands

| Command | Description |
|---------|-------------|
| `petri-pilot mcp` | Run as MCP server |
| `petri-pilot generate "requirements"` | Generate model from natural language |
| `petri-pilot validate model.json` | Validate an existing model |
| `petri-pilot codegen model.json` | Generate application code |
| `petri-pilot refine model.json "fix X"` | Refine model with instructions |

## MCP Tools

When running as an MCP server, these tools are available:

| Tool | Description |
|------|-------------|
| `petri_validate` | Validate model, return structured results |
| `petri_analyze` | Run reachability/sensitivity analysis |
| `petri_codegen` | Generate code from validated model |
| `petri_visualize` | Generate SVG diagram |

## Example

```bash
$ petri-pilot generate -auto "user registration with email verification"

Generating model...
✓ Created: 4 places, 3 transitions, 6 arcs

Validating...
✓ Structural: All elements connected
✓ Reachability: No deadlocks
✓ Bounded: Max 1 token per place

Model saved to: user-registration.json

$ petri-pilot codegen user-registration.json -o ./registration/

Generated:
  registration/workflow.go
  registration/events.go
  registration/aggregate.go
  registration/api.go
  registration/api_openapi.yaml
  registration/workflow_test.go

$ cd registration && go test ./...
PASS
```

## Why Petri Nets?

Petri nets are a minimal formalism for modeling concurrent systems:

- **Places** hold state (tokens)
- **Transitions** move state (fire when inputs available)
- **Arcs** connect places and transitions

Four concepts, one firing rule. This simplicity enables formal verification while expressing complex behavior: mutex locks, producer-consumer queues, checkout flows, approval chains.

## Dependencies

| Package | Purpose |
|---------|---------|
| [go-pflow](https://github.com/pflow-xyz/go-pflow) | Petri net validation and analysis |
| [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) | Claude API client |
| [mcp-go](https://github.com/mark3labs/mcp-go) | MCP server implementation |

## License

MIT
