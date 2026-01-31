# Petri Pilot

**An experimental workshop for Petri net tooling.**

This is where ideas get built, tested, and refined. Patterns that prove useful graduate upstream to [go-pflow](https://github.com/pflow-xyz/go-pflow).

## What's Here

Petri Pilot explores the question: *what if the model was the app?*

```
JSON Model ──> Deterministic Codegen ──> Running Application
```

A Petri net model defines **places** (states), **transitions** (actions), and **arcs** (connections). From that single file, codegen produces:

- **Go backend** — event-sourced aggregate, REST API, SQLite storage
- **ES modules frontend** — admin dashboard, simulation, event history
- **GraphQL API** — unified query layer with built-in playground
- **pflow viewer** — interactive Petri net visualization

No LLM-generated code. The LLM designs models. Templates produce apps.

**Live demos:** [pilot.pflow.xyz](https://pilot.pflow.xyz)

## Experiments in Progress

| Experiment | Status | Notes |
|------------|--------|-------|
| MCP-native model design | Active | LLM designs, validates, simulates models via MCP tools |
| ODE simulation | Active | Continuous-time analysis, optimization (knapsack, resource allocation) |
| Multi-model serving | Active | Multiple apps on one port, shared GraphQL layer |
| ZK circuits from Petri nets | Early | Tic-tac-toe with gnark proofs |
| Code generation patterns | Ongoing | Templates for auth, permissions, views, navigation |

## MCP Server

Petri Pilot runs as an MCP server. An LLM can design, validate, simulate, and generate without leaving the conversation.

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
| `petri_extend` | Modify existing models |
| `service_start/stop/logs` | Manage running services |

## Quick Start

```bash
# Install
go install github.com/pflow-xyz/petri-pilot/cmd/petri-pilot@latest

# Run the demo server
petri-pilot serve tic-tac-toe coffeeshop knapsack

# Or start the MCP server
petri-pilot mcp
```

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

Models can include roles, access rules, typed events, views, and navigation. See `services/` for examples.

## Project Structure

```
pkg/mcp/             MCP server and tools
pkg/codegen/         Go and ES modules templates
pkg/serve/           Multi-model HTTP server
pkg/validator/       Model analysis
services/            Example models (tic-tac-toe, coffeeshop, knapsack, texas-holdem)
frontends/           Custom frontends for demos
generated/           Output from codegen (derived, not source)
```

## Relationship to go-pflow

[go-pflow](https://github.com/pflow-xyz/go-pflow) is the stable Petri net library. Petri Pilot is the experimental counterpart where new ideas are prototyped.

When something works well here — a code generation pattern, an analysis technique, a runtime feature — it may be extracted and added to go-pflow as a stable API.

Think of this repo as a pilot project. Some experiments succeed and ship. Others teach us something and get archived. That's the point.

## License

MIT
