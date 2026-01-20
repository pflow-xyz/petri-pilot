# Petri Pilot

**The model is the application.**

Describe your system in plain language. An LLM designs a formal Petri net model. Validation catches deadlocks and structural errors. Deterministic codegen produces a complete application.

The model contains everything: state machine, access control, UI structure, admin dashboard. Code is a projection. Change the model, regenerate, deploy.

No LLM-generated code. The LLM designs models. Codegen produces apps.

## Quick Start

```bash
# Run as MCP server (for Claude Desktop, Cursor)
petri-pilot mcp

# Or generate from CLI
petri-pilot generate -auto "order processing workflow"
petri-pilot codegen model.json -o ./app/ --frontend
```

## Getting Started with Examples

The `examples/` directory contains ready-to-use models. The `generated/` directory contains pre-built applications with both backend and frontend.

### Run the Order Processing Example

```bash
# 1. Start the backend
cd generated/order-processing
./order-processing
# Server starts on http://localhost:8080

# 2. In another terminal, start the frontend
cd generated/order-processing/frontend
npm install
npm run dev
# Frontend starts on http://localhost:5173
```

### What's Included

Each generated app includes:

**Backend (Go)**
- `main.go` - Application entry point
- `api.go` - HTTP handlers for all transitions
- `workflow.go` - Petri net state machine
- `aggregate.go` - Event-sourced aggregate
- `auth.go` - GitHub OAuth authentication
- `middleware.go` - JWT validation, CORS
- `permissions.go` - Role-based access control
- `views.go` - View definitions for UI
- `navigation.go` - Navigation menu (if configured)
- `admin.go` - Admin dashboard handlers (if configured)
- `openapi.yaml` - OpenAPI 3.0 specification

**Frontend (ES Modules)**
- `src/main.js` - Application entry
- `src/router.js` - Client-side routing
- `src/navigation.js` - Dynamic navigation from API
- `src/views.js` - Dynamic forms from view definitions
- `src/events.js` - Event history with time-travel
- `src/admin.js` - Admin dashboard

### API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /api/{model}` | Create new instance |
| `GET /api/{model}/{id}` | Get instance state |
| `POST /api/{transition}` | Fire a transition |
| `GET /api/views` | Get view definitions |
| `GET /api/navigation` | Get navigation menu |
| `GET /api/{model}/{id}/events` | Event history |
| `GET /api/{model}/{id}/at/{version}` | State at version |
| `GET /admin/stats` | Dashboard statistics |
| `GET /admin/instances` | List all instances |

## Model Schema

The model format is formally specified in [`schema/petri-model.schema.json`](schema/petri-model.schema.json). LLMs can validate against this schema before submitting models.

For architectural details, see [`ARCHITECTURE.md`](ARCHITECTURE.md).

### Basic Model

```json
{
  "name": "order-processing",
  "places": [
    {"id": "received", "initial": 1},
    {"id": "validated"},
    {"id": "shipped"}
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

### Events First Schema

Petri-pilot uses an **Events First** design pattern where events define the complete data contract, while bindings define operational data for state computation.

#### Events: The Complete Record

Events capture the complete business record with:
- All required and optional fields for audit and replay
- Auto-populated system fields (timestamp, aggregate_id)
- Full domain context for event sourcing

```json
{
  "events": [
    {
      "id": "order_validated",
      "name": "Order Validated",
      "description": "Order has passed validation checks",
      "fields": [
        {"name": "order_id", "type": "string", "required": true},
        {"name": "customer_name", "type": "string", "required": true},
        {"name": "customer_email", "type": "string"},
        {"name": "total", "type": "number", "required": true},
        {"name": "status", "type": "string"}
      ]
    }
  ]
}
```

#### Bindings: Operational Data

Bindings extract just the operational data needed for:
- Guard expressions (state validation)
- Arc transformations (state updates)
- Map key lookups (arcnet pattern)

```json
{
  "transitions": [
    {
      "id": "validate",
      "event": "order_validated",
      "bindings": [
        {"name": "order_id", "type": "string"},
        {"name": "customer_name", "type": "string"},
        {"name": "total", "type": "number", "value": true}
      ]
    }
  ]
}
```

**Key differences:**
- **Events** = Complete business record (what happened)
- **Bindings** = Operational subset (what's needed for computation)
- **`"value": true`** = Transfer value to state (for data places)
- **`"keys": ["key"]`** = Use as map key for lookups

For more details, see [`docs/events-first-pattern.md`](docs/events-first-pattern.md).

### Full-Featured Model

See `examples/order-processing.json` for a complete example with:
- Events First schema with bindings
- Roles and access control
- Views for forms and tables
- Navigation configuration
- Event sourcing with snapshots
- Admin dashboard

## MCP Tools

| Tool | Description |
|------|-------------|
| `petri_validate` | Check model for errors |
| `petri_analyze` | Reachability analysis |
| `petri_simulate` | Simulate workflow execution |
| `petri_codegen` | Generate Go application |
| `petri_application` | Generate full-stack app |

### LLM Integration

The model format is designed for LLM consumption:

1. **JSON Schema** - [`schema/petri-model.schema.json`](schema/petri-model.schema.json) for client-side validation
2. **Structured feedback** - `petri_validate` returns actionable errors with fix suggestions
3. **Deterministic output** - Same model always produces same code

Workflow:
```
Requirements → LLM generates model → Validate → Fix errors → Generate code
```

The model is the contract between human intent and machine execution.

## CLI Commands

```bash
# Generate model from natural language
petri-pilot generate "order processing workflow"
petri-pilot generate -auto "user registration" -o model.json

# Validate a model
petri-pilot validate model.json

# Generate code
petri-pilot codegen model.json -o ./app/
petri-pilot codegen model.json -o ./app/ --frontend

# Generate frontend only
petri-pilot frontend model.json -o ./frontend/
```

## Documentation

- **[Events First Pattern](docs/events-first-pattern.md)** - Complete guide to Events First schema and binding patterns
- **[MCP Prompts Guide](docs/mcp-prompts-guide.md)** - How to use design-workflow, add-access-control, and add-views prompts
- **[E2E Testing Guide](docs/e2e-testing-guide.md)** - Writing tests for generated applications
- **[Architecture](ARCHITECTURE.md)** - Detailed architecture and design patterns

## Development

```bash
# Build the CLI
make build

# Run tests
make test

# Generate all examples
make codegen-all

# Build all examples (verifies generated code compiles)
make build-examples
```

## Why Petri Nets?

Minimal formalism: places (state), transitions (actions), arcs (connections). Four concepts, formal verification, expressive power.

## License

MIT
