# Petri Pilot

An SDK for building applications from Petri net models.

Define a model. Generate an app. The model is the source of truth.

## How It Works

```
JSON Model ──> Deterministic Codegen ──> Running Application
                                              │
                                    ┌─────────┼─────────┐
                                    │         │         │
                                 Go API   Frontend   GraphQL
```

A Petri net model is a JSON file with **places** (states), **transitions** (actions), and **arcs** (connections). From that single file, codegen produces:

- **Go backend** -- event-sourced aggregate, REST API, SQLite storage, auth, permissions
- **ES modules frontend** -- admin dashboard, schema viewer, simulation, event history
- **GraphQL API** -- unified query layer across all models with built-in playground
- **pflow viewer** -- interactive Petri net visualization via [pflow.xyz](https://pflow.xyz)

No LLM-generated code. The LLM designs models. Templates produce apps.

## MCP-Native

Petri Pilot runs as an MCP server. An LLM can design, validate, simulate, generate, and iterate without leaving the conversation.

```bash
petri-pilot mcp
```

| Tool | Purpose |
|------|---------|
| `petri_validate` | Structural correctness |
| `petri_analyze` | Reachability, deadlocks, liveness |
| `petri_simulate` | Fire transitions, trace state |
| `petri_codegen` | Generate Go backend |
| `petri_frontend` | Generate ES modules frontend |
| `petri_application` | Full-stack from high-level spec |
| `petri_extend` | Add places, transitions, arcs to existing model |
| `petri_visualize` | SVG rendering |
| `service_start` | Build and launch a generated app |
| `service_logs` | Inspect runtime output |

## Multi-Model Server

Run multiple models on a single port. Each gets its own API, frontend, and dashboard.

```bash
petri-pilot serve tic-tac-toe coffeeshop erc20-token blog-post
```

| Route | What |
|-------|------|
| `/` | Landing page |
| `/{model}/` | Custom frontend + REST API |
| `/app/{model}/` | Generated dashboard |
| `/graphql/i` | GraphQL playground |
| `/pflow` | Petri net viewer |

The GraphQL playground, schema viewer, and pflow viewer all read the same model data. Three lenses on one system.

**Live instance:** [pilot.pflow.xyz](https://pilot.pflow.xyz)

## Model Format

```json
{
  "name": "order",
  "places": [
    {"id": "pending", "initial": 1},
    {"id": "shipped"}
  ],
  "transitions": [
    {"id": "ship", "event": "order_shipped"}
  ],
  "arcs": [
    {"from": "pending", "to": "ship"},
    {"from": "ship", "to": "shipped"}
  ]
}
```

Models can also include roles, access rules, events with typed fields, views, navigation, and admin configuration. See `examples/` for full-featured models.

The format is specified in [`schema/petri-model.schema.json`](schema/petri-model.schema.json).

## Install

```bash
go install github.com/pflow-xyz/petri-pilot/cmd/petri-pilot@latest
```

## CLI

```bash
petri-pilot generate "order processing workflow"   # LLM designs a model
petri-pilot validate model.json                    # Check structure
petri-pilot codegen model.json -o ./app/ --frontend # Generate full app
petri-pilot serve order coffeeshop                 # Run models
petri-pilot mcp                                    # MCP server mode
```

## Architecture

```
pkg/schema/          Model types
pkg/codegen/golang/  Go backend templates
pkg/codegen/esmodules/ Frontend templates
pkg/serve/           Multi-model HTTP server, GraphQL, pflow viewer
pkg/mcp/             MCP server and tools
pkg/runtime/         EventStore, Aggregate interfaces
```

Templates are the source of truth. `generated/` is derived output. Change a template, regenerate, every app picks it up.

For details see [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Why Petri Nets

Four concepts: places, transitions, arcs, tokens. Formal enough for verification. Simple enough for an LLM to design. Expressive enough for real applications.

## License

MIT
