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

---

## Next Phase: Admin & Navigation

### 1. Navigation Menu System

Enhance generated applications with a proper navigation structure.

**Schema addition:**
```json
{
  "navigation": {
    "brand": "Order System",
    "items": [
      {"label": "Dashboard", "path": "/", "icon": "home"},
      {"label": "Orders", "path": "/orders", "icon": "list"},
      {"label": "Settings", "path": "/settings", "icon": "cog", "roles": ["admin"]}
    ]
  }
}
```

**Generated output:**
- Sidebar/header navigation component
- Role-based menu item visibility
- Active state highlighting
- Mobile-responsive menu

**Files to modify:**
- `pkg/schema/schema.go` - Add Navigation types
- `pkg/codegen/golang/templates/` - Add navigation endpoint
- `pkg/codegen/react/templates/` - Enhanced navigation component

---

### 2. Admin Dashboard

Auto-generate an admin interface for managing workflow instances.

**Features:**
- List all aggregate instances with current state
- Filter by state (place), date range, ID
- View instance detail with full event history
- Manual state transitions (with permission checks)
- Bulk operations (archive, delete)

**Schema addition:**
```json
{
  "admin": {
    "enabled": true,
    "path": "/admin",
    "roles": ["admin"],
    "features": ["list", "detail", "history", "transitions"]
  }
}
```

**Generated output:**
- `/admin` - Dashboard with instance counts per state
- `/admin/instances` - Paginated list with filters
- `/admin/instances/:id` - Detail view with event timeline
- `/admin/instances/:id/events` - Full event history

**Files to create:**
- `pkg/codegen/golang/templates/admin.tmpl` - Admin API handlers
- `pkg/codegen/react/templates/admin/` - Admin UI components

---

### 3. Event Replay & Snapshots

Add event sourcing tooling for debugging and recovery.

**Features:**
- Replay events to rebuild state
- Point-in-time snapshots
- Time-travel debugging (view state at any version)
- Event store compaction

**API additions:**
```
GET  /api/{model}/{id}/events          # Full event history
GET  /api/{model}/{id}/events?from=5   # Events from version
GET  /api/{model}/{id}/at/{version}    # State at version
POST /api/{model}/{id}/snapshot        # Create snapshot
POST /api/{model}/{id}/replay          # Replay from snapshot
```

**Schema addition:**
```json
{
  "eventSourcing": {
    "snapshots": {
      "enabled": true,
      "frequency": 100
    },
    "retention": {
      "events": "90d",
      "snapshots": "1y"
    }
  }
}
```

**Files to modify:**
- `pkg/runtime/eventstore/` - Add snapshot support
- `pkg/codegen/golang/templates/api.tmpl` - Event history endpoints
- `pkg/codegen/golang/templates/aggregate.tmpl` - Snapshot loading

---

## Implementation Order

| Feature | Priority | Effort | Dependencies |
|---------|----------|--------|--------------|
| Navigation Menu | High | Small | None |
| Admin Dashboard | High | Medium | Navigation |
| Event Replay | Medium | Medium | None |
| Snapshots | Medium | Medium | Event Replay |

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
