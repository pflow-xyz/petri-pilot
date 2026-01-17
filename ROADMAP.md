# Petri-Pilot Roadmap

**Vision**: Rapid prototyping of event-driven applications using Petri net models.

Every application built with petri-pilot shares the same event-sourced, state-machine architecture—whether it runs on a blockchain or a traditional backend. The LLM (via MCP) designs Petri net models from natural language. Deterministic code generation produces executable applications with pluggable runtime interfaces. No LLM-generated code—only structured models that drive codegen.

**Target applications:**
- Workflow engines (order processing, approval flows)
- Smart contracts (tokens, governance, vesting)
- Microservice orchestration
- Game state machines
- IoT device protocols

All share: **events → state transitions → aggregated state**

```
┌─────────────────────────────────────────────────────────────┐
│                    MCP Client (Claude, etc.)                │
│                    LLM designs model in conversation        │
└─────────────────────────────┬───────────────────────────────┘
                              │ petri_codegen tool
                    ┌─────────▼─────────┐
                    │  Code Generator   │
                    │  (deterministic)  │
                    └─────────┬─────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    Generated Project                         │
├─────────────────────────────────────────────────────────────┤
│  workflow.go    │  events.go    │  api.go    │  aggregate   │
├─────────────────┴───────────────┴────────────┴──────────────┤
│                    Runtime Interfaces                        │
│         eventstore.EventStore  │  aggregate.Aggregate        │
├─────────────────────────────────────────────────────────────┤
│                    SQLite SDK (testing)                      │
│              petri-pilot/pkg/runtime/sqlite                  │
└─────────────────────────────────────────────────────────────┘
```

---

## Phase 1: MCP Server

Expose petri-pilot as MCP tools for any LLM client.

| Tool | Description |
|------|-------------|
| `petri_validate` | Validate a model, return structured results |
| `petri_analyze` | Run reachability/sensitivity analysis |
| `petri_codegen` | Generate code from validated model |
| `petri_visualize` | Generate SVG diagram |

```go
// pkg/mcp/server.go
tools := []mcp.Tool{
    {Name: "petri_validate", InputSchema: ValidateInput{}},
    {Name: "petri_analyze", InputSchema: AnalyzeInput{}},
    {Name: "petri_codegen", InputSchema: CodegenInput{}},
}
```

**Why MCP first:**
- LLM designs models in conversation, calls tools to validate/generate
- No custom prompts needed—any MCP client works
- Clean separation: LLM = creative design, tools = deterministic operations
- Enables Claude Desktop, Cursor, other MCP clients

```
┌─────────────────────────────────────────┐
│  MCP Client (Claude Desktop, etc.)     │
│  LLM designs Petri net in conversation │
└─────────────────┬───────────────────────┘
                  │ MCP protocol
    ┌─────────────▼─────────────┐
    │  petri-pilot MCP server   │
    ├───────────────────────────┤
    │  petri_validate → go-pflow│
    │  petri_analyze  → results │
    │  petri_codegen  → Go/TS   │
    └───────────────────────────┘
```

---

## Phase 2: Runtime Interfaces

Pluggable abstractions for generated applications.

```
pkg/runtime/
├── eventstore/
│   ├── store.go        # EventStore interface
│   ├── sqlite/         # SQLite implementation (for testing)
│   └── memory/         # In-memory implementation
├── aggregate/
│   ├── aggregate.go    # Aggregate interface
│   ├── projector.go    # Event → State projection
│   └── sqlite/         # SQLite-backed aggregates
└── api/
    ├── handler.go      # HTTP handler interface
    ├── router.go       # Route generation from model
    └── openapi/        # OpenAPI spec generation
```

### Core Interfaces

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

// TransitionHandler - generated from transitions
type TransitionHandler interface {
    Handle(ctx context.Context, req TransitionRequest) (*TransitionResult, error)
}
```

### SQLite SDK for Testing

```go
// Quick start for generated projects
store := sqlite.NewEventStore("test.db")
aggregate := sqlite.NewAggregate[OrderState](store, "orders")

// Run generated workflow against real storage
engine := generated.NewOrderWorkflow(aggregate)
engine.Fire("validate", orderID)
```

---

## Phase 3: Schema Bridge

Connect petri-pilot schema to go-pflow metamodel.

| Task | Description |
|------|-------------|
| `pkg/bridge/converter.go` | `schema.Model` → `metamodel.Schema` |
| Extend places | Add `kind` (token/data), `type`, `persisted` flag |
| Extend transitions | Map to actions with guards + API bindings |
| Arc enrichment | Add `keys`, `value` bindings for data flow |

```
petri-pilot Model (LLM-generated)
        ↓
    bridge.Convert()
        ↓
go-pflow metamodel.Schema
        ↓
    codegen.Generate()
        ↓
    Application Code
```

---

## Phase 4: Multi-Target Code Generation

Generated code uses runtime interfaces.

| Target | Output |
|--------|--------|
| Go | State machine + event handlers using `runtime/` interfaces |
| TypeScript | Event-driven handlers with storage adapters |
| OpenAPI | API spec from model transitions |

### Generated Project Structure

```
generated-order-workflow/
├── go.mod
├── main.go              # Wires up runtime + starts API
├── workflow.go          # Generated state machine
├── events.go            # Event types from transitions
├── aggregate.go         # Order aggregate
├── api.go               # HTTP handlers per transition
├── api_openapi.yaml     # Generated spec
└── workflow_test.go     # Tests using sqlite runtime
```

### Example Generated Code

```go
// workflow.go (generated, not LLM)
type OrderWorkflow struct {
    store     eventstore.EventStore
    aggregate aggregate.Aggregate[OrderState]
}

func (w *OrderWorkflow) Validate(ctx context.Context, orderID string) error {
    // Check guard: current state must be "received"
    state := w.aggregate.State()
    if state.Status != "received" {
        return ErrInvalidTransition
    }

    // Fire transition, append event
    event := OrderValidated{OrderID: orderID, At: time.Now()}
    return w.store.Append(ctx, orderID, []Event{event})
}
```

---

## Phase 5: CLI Integration

```bash
# Run as MCP server
petri-pilot mcp

# Generate full project with runtime
petri-pilot codegen model.json -lang go -o ./myworkflow/
cd myworkflow && go test ./...  # Uses SQLite runtime

# Generate API spec only
petri-pilot codegen model.json -api-only -o openapi.yaml
```

---

## Phase 6: Validation for Implementability

Extend validator to check codegen feasibility.

| Check | Description |
|-------|-------------|
| Event derivation | Can events be inferred from transitions? |
| State schema | Are aggregate states well-typed? |
| API mappings | Do transitions have valid HTTP semantics? |
| Type consistency | All data states have valid types |
| Guard parsability | Expressions can be translated |
| Pattern detection | Identify workflow vs state machine |

---

## Implementation Order

1. **`pkg/mcp/`** - MCP server with tools
2. **`pkg/runtime/eventstore/`** - EventStore interface + SQLite impl
3. **`pkg/runtime/aggregate/`** - Aggregate interface + SQLite impl
4. **`pkg/runtime/api/`** - HTTP handler interfaces
5. **`pkg/bridge/`** - Schema → metamodel converter
6. **`pkg/codegen/golang/`** - Go project generator using runtime interfaces
7. **CLI commands** - `petri-pilot mcp`, `petri-pilot codegen`

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/pflow-xyz/go-pflow` | Petri net validation, metamodel, existing Solidity codegen |
| `github.com/mark3labs/mcp-go` | MCP server implementation |
| `github.com/mattn/go-sqlite3` | SQLite driver for runtime SDK |
| `github.com/anthropics/anthropic-sdk-go` | Claude API (existing, for legacy CLI) |
