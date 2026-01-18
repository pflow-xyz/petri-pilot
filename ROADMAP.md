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

## Phase 1: MCP Server ✅

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

## Phase 2: Runtime Interfaces ✅

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

## Phase 3: Schema Bridge ✅

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

## Phase 4: Multi-Target Code Generation ✅ (Go only)

Generated code uses runtime interfaces.

| Target | Output |
|--------|--------|
| Go | State machine + event handlers using `runtime/` interfaces |
| JavaScript (ES modules) | Event-driven handlers with storage adapters |
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

## Phase 5: CLI Integration ✅

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

## Phase 6: Validation for Implementability ✅

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

# E2E Application Pipeline

Phases 1-6 produce a backend with HTTP API. The following phases complete the full-stack pipeline.

## Phase 7: Backend Completion ✅

Infrastructure templates for production deployment.

| Component | Template | Description |
|-----------|----------|-------------|
| Database migrations | `migrations.sql.tmpl` | Event store schema, indexes, snapshots table |
| Configuration | `config.go.tmpl` | Env-based config struct with validation |
| Health endpoints | (in `api.go.tmpl`) | `/health`, `/ready` endpoints |
| Dockerfile | `Dockerfile.tmpl` | Multi-stage build, minimal runtime image |
| Docker Compose | `docker-compose.yaml.tmpl` | App + Postgres/SQLite for local dev |

### Generated Output

```
generated/
├── ...existing files...
├── migrations/
│   └── 001_init.sql       # Event store + projections schema
├── config.go              # Config struct + loader
├── Dockerfile
└── docker-compose.yaml
```

### Migration Schema

```sql
-- Event store
CREATE TABLE events (
    id UUID PRIMARY KEY,
    stream_id TEXT NOT NULL,
    type TEXT NOT NULL,
    version INT NOT NULL,
    data JSONB NOT NULL,
    metadata JSONB,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(stream_id, version)
);

-- Projections (generated per place)
CREATE TABLE order_state (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    version INT NOT NULL,
    updated_at TIMESTAMPTZ
);
```

---

## Phase 8: Frontend Generation ✅

Generate a vanilla JavaScript frontend application using ES modules from the same Petri net model.

| Component | Description |
|-----------|-------------|
| ES modules scaffold | Vite + ES modules + plain JavaScript project |
| API client | Generated fetch-based client with async/await |
| Transition forms | Vanilla JavaScript form components per transition |
| State display | Shows current place tokens / aggregate state |
| Routing | Pages per major workflow state |

### Generated Frontend Structure

```
generated-frontend/
├── package.json
├── vite.config.js
├── src/
│   ├── main.js                # Main application entry point
│   ├── api/
│   │   └── client.js          # Fetch-based API client
│   ├── components/
│   │   ├── state-display.js   # Current workflow state
│   │   └── transitions/
│   │       ├── validate-form.js
│   │       ├── ship-form.js
│   │       └── ...            # One per transition
│   └── utils/
│       └── workflow.js        # State + transition helpers
└── index.html
```

### MCP Tool Addition

```go
s.AddTool(frontendTool(), handleFrontend)

func frontendTool() mcp.Tool {
    return mcp.NewTool("petri_frontend",
        mcp.WithDescription("Generate vanilla JavaScript ES modules frontend from Petri net model"),
        mcp.WithString("model", mcp.Required()),
        mcp.WithString("project", mcp.Description("Project name (default: model name)")),
    )
}
```

---

## Phase 9: Real-Time Updates + Auth ✅

Enable live state synchronization and access control (GitHub OAuth implemented).

| Component | Description |
|-----------|-------------|
| WebSocket endpoint | `/ws` - broadcasts state changes |
| SSE alternative | `/events` - server-sent events stream |
| JWT middleware | Token validation, claims extraction |
| Permission guards | Transition-level authorization |
| Frontend auth | Login/logout, token storage, protected routes |

### Backend Additions

```go
// api.go additions
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Upgrade connection, subscribe to aggregate changes
}

// middleware.go additions
func JWTAuth(secret string) func(http.Handler) http.Handler
func RequirePermission(perm string) func(http.Handler) http.Handler
```

### Frontend Additions

```javascript
// utils/realtime.js
export function subscribeToState(streamId, callback) {
    // Server-Sent Events subscription, automatic reconnect
}

// utils/auth.js
export function initAuth() {
    // JWT storage, refresh, logout helpers
}
```

### Schema Extension

```json
{
  "transitions": [
    {
      "id": "ship",
      "permissions": ["orders:write", "shipping:execute"]
    }
  ]
}
```

---

## Phase 10: Observability + Production Deploy ✅

Production-ready infrastructure.

| Component | Description |
|-----------|-------------|
| Structured logging | `slog` with JSON output, request IDs |
| Prometheus metrics | Request counts, latencies, event store stats |
| OpenTelemetry traces | Distributed tracing across services |
| Kubernetes manifests | Deployment, Service, ConfigMap, Secrets |
| Helm chart | Parameterized K8s deployment |
| GitHub Actions | CI pipeline template |

### Generated Infrastructure

```
generated/
├── ...existing files...
├── k8s/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── ingress.yaml
├── helm/
│   └── petri-app/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
└── .github/
    └── workflows/
        └── ci.yaml
```

### Observability Integration

```go
// main.go additions
func main() {
    // Structured logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Prometheus metrics
    http.Handle("/metrics", promhttp.Handler())

    // OpenTelemetry
    tp := initTracer()
    defer tp.Shutdown(ctx)
}
```

---

## E2E Implementation Order

| Phase | Priority | Effort | Dependencies |
|-------|----------|--------|--------------|
| Phase 7: Backend Completion | High | Medium | None |
| Phase 8: Frontend Generation | High | Large | Phase 7 (needs stable API) |
| Phase 9: Real-Time + Auth | Medium | Medium | Phase 8 |
| Phase 10: Observability | Medium | Small | Phase 7 |

### Quick Win Path

Fastest route to deployable full-stack app:

1. Add `Dockerfile.tmpl` + `docker-compose.yaml.tmpl` (Phase 7 partial)
2. Generate vanilla JavaScript API client with fetch
3. Minimal ES modules template with generated client + forms

---

## Phase 11: LLM-Complete Application DSL ✅

Enable LLMs to design complete applications with a single JSON specification.

### Local Metamodel & DSL

Copied and extended go-pflow's metamodel into petri-pilot for full control:

| Package | Purpose |
|---------|---------|
| `pkg/metamodel/` | Schema, State, Action, Arc, Runtime, Snapshot |
| `pkg/dsl/` | Guard expression lexer, parser, evaluator |

### Application Schema

A complete application specification that an LLM can generate:

```json
{
  "name": "order-system",
  "entities": [
    {
      "id": "order",
      "fields": [
        {"id": "customer_id", "type": "string", "required": true},
        {"id": "total", "type": "amount"},
        {"id": "items", "type": "json"}
      ],
      "states": [
        {"id": "draft", "initial": true},
        {"id": "submitted"},
        {"id": "approved"},
        {"id": "shipped"},
        {"id": "delivered", "terminal": true}
      ],
      "actions": [
        {
          "id": "submit",
          "from_states": ["draft"],
          "to_state": "submitted",
          "guard": "len(items) > 0 && total > 0",
          "input": [{"id": "notes", "type": "text"}],
          "http": {"method": "POST", "path": "/orders/{id}/submit"}
        }
      ],
      "access": [
        {"action": "submit", "roles": ["customer"], "guard": "user.id == customer_id"}
      ]
    }
  ],
  "roles": [
    {"id": "customer"},
    {"id": "admin", "inherits": ["customer"]}
  ],
  "pages": [
    {"id": "orders", "path": "/orders", "layout": {"type": "list", "entity": "order"}},
    {"id": "order-detail", "path": "/orders/:id", "layout": {"type": "detail", "entity": "order"}}
  ],
  "workflows": [
    {
      "id": "order-fulfillment",
      "trigger": {"type": "event", "entity": "order", "action": "approve"},
      "steps": [
        {"id": "notify", "type": "action", "entity": "notification", "action": "send"},
        {"id": "ship", "type": "action", "entity": "order", "action": "ship"}
      ]
    }
  ]
}
```

### DSL Features

| Feature | Example | Status |
|---------|---------|--------|
| **Comparisons** | `total >= 100` | ✅ |
| **Boolean logic** | `a && b \|\| !c` | ✅ |
| **Map access** | `balances[from] >= amount` | ✅ |
| **Nested maps** | `allowances[owner][spender]` | ✅ |
| **Functions** | `len(items) > 0` | ✅ |
| **Aggregates** | `sum("balances") == totalSupply` | ✅ |
| **String ops** | `startsWith(name, "VIP")` | ✅ |

### View/Form Generation

```go
// pkg/metamodel/views.go
type View struct {
    ID     string      `json:"id"`
    Kind   ViewKind    `json:"kind"`   // form, card, table, detail
    Groups []ViewGroup `json:"groups"`
}

type ViewField struct {
    Binding    string          `json:"binding"`
    Label      string          `json:"label"`
    Type       FieldType       `json:"type"`  // text, number, address, amount, select
    Required   bool            `json:"required"`
    Validation *FieldValidation `json:"validation"`
}
```

### Remaining Work

| Component | Status | Priority |
|-----------|--------|----------|
| Entity → Schema converter | ✅ Done | - |
| Form generation from actions | ✅ Done | - |
| Access control codegen | ✅ Done | - |
| Page/navigation codegen | ✅ Done | - |
| Workflow orchestration | ✅ Done | - |
| Integration webhooks | ❌ TODO | Low |

### LLM Integration

The MCP server exposes:

```go
// MCP tool for full application generation
{Name: "petri_application", InputSchema: ApplicationInput{}}

// ApplicationInput accepts the full application spec
type ApplicationInput struct {
    Spec Application `json:"spec"`
    Options struct {
        Backend  string `json:"backend"`  // go, javascript
        Frontend string `json:"frontend"` // esm (ES modules), none
        Database string `json:"database"` // postgres, sqlite
    } `json:"options"`
}
```

---

## Phase 12: Integration & E2E Testing ✅

Complete integration of Phase 11 components and establish comprehensive testing.

| Component | Status |
|-----------|--------|
| Page/Navigation integration | ✅ Complete |
| Access control middleware wiring | ✅ Complete |
| Workflow orchestration wiring | ✅ Complete |
| End-to-end integration tests | ✅ Complete |
| Generated code validation tests | ✅ Complete |
| Documentation updates | ✅ Complete |

### Success Criteria

- [x] `petri_application` generates complete, compilable applications
- [x] Generated Go code validates syntax successfully
- [x] Generated frontend code validates successfully
- [x] Access control middleware properly enforces RBAC + guards
- [x] Pages and navigation are correctly generated from specs
- [x] Workflows execute on event triggers
- [x] All 14 integration and validation tests pass

### Implementation Summary

**Workflow Orchestration Wiring:**
- Added `WorkflowContext`, `WorkflowTriggerContext`, and `WorkflowStepContext` types
- Extended MCP server to build workflow contexts from Application spec
- Updated golang generator to include workflows.go when workflows defined
- Fixed template variable scoping in workflows.tmpl

**Middleware Integration:**
- Added `HasAccessControl()` and `TransitionRequiresAuth()` helper methods
- Updated main.tmpl to initialize middleware with access rules
- Updated api.tmpl to wrap protected routes with RequirePermission middleware
- Middleware uses pkg/dsl for dynamic guard evaluation

**Test Coverage:**
- 14 tests total (7 existing + 4 integration + 3 validation)
- TestCompleteApplicationGeneration: End-to-end app generation
- TestAccessControlIntegration: Middleware wiring verification
- TestPageNavigationIntegration: Frontend generation verification
- TestWorkflowIntegration: Workflow structure verification
- TestGeneratedGoCodeCompilation: Go syntax validation
- TestGeneratedFrontendValidation: Frontend syntax validation
- TestAllTemplatesGenerate: All 15 templates execute successfully

**Generated Output:**
Complete applications include 14 backend files + 7 frontend files with:
- Role-based access control
- Multi-step workflow orchestration
- Frontend pages and navigation
- Event-sourced backend
- Database migrations
- Docker deployment
- OAuth authentication
- OpenAPI documentation

See `PHASE12_COMPLETE.md` for detailed implementation notes.

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/pflow-xyz/go-pflow` | Petri net validation (reachability, sensitivity) |
| `github.com/mark3labs/mcp-go` | MCP server implementation |
| `github.com/mattn/go-sqlite3` | SQLite driver for runtime SDK |
| `github.com/anthropics/anthropic-sdk-go` | Claude API (existing, for legacy CLI) |

### Frontend Dependencies (Phase 8+)

| Package | Purpose |
|---------|---------|
| `vite` | Frontend build tooling and dev server |
| `gorilla/websocket` | WebSocket support (Phase 9) |
| `golang-jwt/jwt` | JWT validation (Phase 9) |
| `prometheus/client_golang` | Metrics (Phase 10) |
| `go.opentelemetry.io/otel` | Tracing (Phase 10) |
