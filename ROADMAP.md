# Petri-Pilot Roadmap

**From requirements to running applications, through verified Petri net models.**

An LLM (via MCP) designs Petri net models from natural language. Deterministic code generation produces complete applications. No LLM-generated code—only structured models that drive codegen.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│              MCP Client (Claude Desktop, Cursor)            │
│              LLM designs model in conversation              │
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
│  workflow.go  │  api.go  │  views.go  │  auth.go  │  ...    │
├─────────────────────────────────────────────────────────────┤
│                    Runtime Interfaces                        │
│           eventstore.EventStore  │  aggregate.Aggregate      │
├─────────────────────────────────────────────────────────────┤
│                       SQLite Runtime                         │
└─────────────────────────────────────────────────────────────┘
```

---

## What's Generated

From a single JSON model, petri-pilot generates:

### Backend (Go)

| File | Description |
|------|-------------|
| `main.go` | Application entry point, wires up runtime |
| `workflow.go` | State machine with places, transitions, arcs |
| `events.go` | Event types derived from transitions |
| `aggregate.go` | Event-sourced aggregate with state projection |
| `api.go` | HTTP handlers per transition |
| `views.go` | View definitions for UI rendering |
| `auth.go` | GitHub OAuth authentication |
| `middleware.go` | JWT validation, permission guards |
| `permissions.go` | Role-based access control |
| `config.go` | Environment-based configuration |
| `workflow_test.go` | Generated tests |
| `openapi.yaml` | OpenAPI 3.0 specification |

### Infrastructure

| File | Description |
|------|-------------|
| `migrations/001_init.sql` | Event store schema |
| `Dockerfile` | Multi-stage build |
| `docker-compose.yaml` | Local development setup |
| `k8s/deployment.yaml` | Kubernetes deployment |
| `k8s/service.yaml` | Kubernetes service |
| `.github/workflows/ci.yaml` | GitHub Actions CI |

### Frontend (ES Modules)

| File | Description |
|------|-------------|
| `src/main.js` | Application entry |
| `src/api/client.js` | Fetch-based API client |
| `src/pages/*.js` | Generated pages |
| `src/components/navigation.js` | Navigation component |

---

## Model Schema

### Basic Model

```json
{
  "name": "order-processing",
  "description": "Order workflow with validation and shipping",
  "places": [
    {"id": "received", "initial": 1},
    {"id": "validated"},
    {"id": "shipped"},
    {"id": "completed"}
  ],
  "transitions": [
    {"id": "validate", "description": "Check order validity"},
    {"id": "ship", "description": "Ship the order"},
    {"id": "confirm", "description": "Mark complete"}
  ],
  "arcs": [
    {"from": "received", "to": "validate"},
    {"from": "validate", "to": "validated"},
    {"from": "validated", "to": "ship"},
    {"from": "ship", "to": "shipped"},
    {"from": "shipped", "to": "confirm"},
    {"from": "confirm", "to": "completed"}
  ]
}
```

### With Access Control

```json
{
  "roles": [
    {"id": "customer", "name": "Customer"},
    {"id": "fulfillment", "name": "Fulfillment Staff"},
    {"id": "admin", "inherits": ["customer", "fulfillment"]}
  ],
  "access": [
    {"transition": "validate", "roles": ["fulfillment"]},
    {"transition": "ship", "roles": ["fulfillment"]},
    {"transition": "confirm", "roles": ["fulfillment"]}
  ]
}
```

### With Views

```json
{
  "views": [
    {
      "id": "order-table",
      "name": "Orders List",
      "kind": "table",
      "groups": [{
        "id": "columns",
        "fields": [
          {"binding": "order_id", "label": "Order ID", "type": "text"},
          {"binding": "status", "label": "Status", "type": "text", "readonly": true}
        ]
      }],
      "actions": ["validate", "ship", "confirm"]
    },
    {
      "id": "order-detail",
      "name": "Order Detail",
      "kind": "detail",
      "groups": [{
        "id": "info",
        "name": "Order Information",
        "fields": [
          {"binding": "order_id", "label": "Order ID", "type": "text"},
          {"binding": "customer_email", "label": "Email", "type": "email"},
          {"binding": "total", "label": "Total", "type": "number"}
        ]
      }],
      "actions": ["validate", "ship", "confirm"]
    }
  ]
}
```

---

## MCP Tools

| Tool | Description |
|------|-------------|
| `petri_validate` | Validate model structure and semantics |
| `petri_analyze` | Reachability and deadlock analysis |
| `petri_codegen` | Generate Go application |
| `petri_application` | Generate full-stack application |

---

## CLI Usage

```bash
# Run as MCP server
petri-pilot mcp

# Generate from model
petri-pilot codegen model.json -o ./app/

# Validate model
petri-pilot validate model.json

# Generate with auto-validation
petri-pilot generate -auto "order processing workflow"
```

---

## Completed Features

- [x] MCP server with validation, analysis, codegen tools
- [x] Runtime interfaces (EventStore, Aggregate)
- [x] SQLite runtime for testing
- [x] Go backend code generation
- [x] ES modules frontend generation
- [x] Role-based access control
- [x] Guard expressions (DSL with comparisons, boolean logic, map access)
- [x] Views and forms schema
- [x] GitHub OAuth authentication
- [x] WebSocket/SSE real-time updates
- [x] OpenAPI specification generation
- [x] Docker and Kubernetes deployment
- [x] Database migrations
- [x] Workflow orchestration (event-triggered, multi-step)
- [x] Webhook integrations with retry
- [x] Navigation menu backend (`/api/navigation` with role filtering)
- [x] Admin dashboard backend (`/admin/stats`, `/admin/instances`)
- [x] Event sourcing APIs (`/api/{model}/{id}/events`, `/api/{model}/{id}/at/{version}`)
- [x] Snapshot support (`/api/{model}/{id}/snapshot`, `/api/{model}/{id}/replay`)
- [x] CLI `--frontend` flag for codegen command
- [x] Frontend navigation rendering (fetches from `/api/navigation`)
- [x] Frontend views/forms rendering (fetches from `/api/views`)
- [x] Frontend admin dashboard UI (`src/admin.js`)
- [x] Frontend event history viewer (`src/events.js`)
- [x] Full-featured example model (`order-processing.json` with navigation, admin, event sourcing)
- [x] Examples include frontend (all generated with `--frontend` flag)
- [x] Getting-started documentation in README

---

## Pre-Release Checklist

Before making the repo public:

### Installation & Distribution
- [ ] Create GitHub release (enables `go install github.com/pflow-xyz/petri-pilot@latest`)
- [ ] Pre-built binaries for macOS, Linux, Windows
- [ ] Remove replace directive from generated go.mod (requires published module)

### Generated App Improvements
- [ ] Use SQLite by default instead of MemoryStore (admin dashboard requires it)
- [ ] Add `STORE_TYPE` env var to switch between memory/sqlite
- [ ] Ensure generated apps work without replace directive

### Documentation
- [ ] MCP setup instructions for Claude Desktop config:
  ```json
  {
    "mcpServers": {
      "petri-pilot": {
        "command": "petri-pilot",
        "args": ["mcp"]
      }
    }
  }
  ```
- [ ] GitHub OAuth App setup guide (GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET)
- [ ] Environment variable reference for generated apps

---

## Future Considerations

Potential enhancements (not yet scoped):

- GraphQL API generation
- Multi-tenancy patterns

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/pflow-xyz/go-pflow` | Petri net validation |
| `github.com/mark3labs/mcp-go` | MCP server |
| `github.com/mattn/go-sqlite3` | SQLite runtime |
