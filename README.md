# Petri Pilot

**Compile Petri net models into running applications.**

A Petri net model defines places (states), transitions (actions), and arcs (connections). Petri-pilot compiles that model into a complete, deployable application. The generation is deterministic — the same model always produces the same code.

```
Model ──> Context ──> Templates ──> Running Application
```

The model is the source of truth. Code is a derived artifact.

**Live demos:** [pilot.pflow.xyz](https://pilot.pflow.xyz) | **Book:** [book.pflow.xyz](https://book.pflow.xyz)

## What It Generates

From a single model file, petri-pilot produces:

- **Go backend** — event-sourced aggregate, REST API, SQLite storage
- **ES modules frontend** — admin dashboard, simulation, event history
- **GraphQL API** — unified query layer with built-in playground
- **pflow viewer** — interactive Petri net visualization

No LLM-generated code. The LLM designs models. Templates compile apps.

## Install

```bash
# From source
go install github.com/pflow-xyz/petri-pilot/cmd/petri-pilot@latest

# Or via Docker
docker run ghcr.io/pflow-xyz/petri-pilot version

# Or download a binary from GitHub Releases
# https://github.com/pflow-xyz/petri-pilot/releases
```

## Quick Start

```bash
# Run the demo server
petri-pilot serve tic-tac-toe coffeeshop knapsack

# Or start the MCP server
petri-pilot mcp
```

## MCP Server

Petri-pilot runs as an MCP server. An LLM can design, validate, simulate, and generate without leaving the conversation.

### Connect to the hosted server (no install)

The server is hosted at `https://pilot.pflow.xyz/mcp` over Streamable HTTP. In Claude Code:

```bash
claude mcp add --transport http petri-pilot https://pilot.pflow.xyz/mcp
```

### Run locally

```bash
petri-pilot mcp
```

| Tool | Purpose |
|------|---------|
| `petri_validate` | Structural correctness, conservation laws |
| `petri_analyze` | Reachability, deadlocks, liveness, P/T-invariants |
| `petri_verify` | Check stated properties — proved/refuted/unknown + counterexample |
| `petri_conformance` | Replay a real event log against the model (fitness/precision) |
| `petri_simulate` | Fire transitions, trace state |
| `petri_code_to_flow` | Convert source code into a Petri net model |
| `petri_codegen` | Generate Go backend |
| `petri_frontend` | Generate ES modules frontend |
| `petri_application` | Full-stack from high-level spec |
| `petri_extend` | Modify existing models |
| `service_start/stop/logs` | Manage running services |

### Claude Desktop / Cursor

For clients that take JSON config, point at the hosted HTTP server:

```json
{
  "mcpServers": {
    "petri-pilot": {
      "type": "http",
      "url": "https://pilot.pflow.xyz/mcp"
    }
  }
}
```

Or run the server locally:

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

Or with Docker:

```json
{
  "mcpServers": {
    "petri-pilot": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "ghcr.io/pflow-xyz/petri-pilot", "mcp"]
    }
  }
}
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

## Ecosystem

Petri-pilot is part of the [pflow](https://pflow.xyz) toolchain. All three tools share the same JSON-LD model format.

| Tool | Role |
|------|------|
| [pflow.xyz](https://pflow.xyz) | Visual editor — design and simulate nets in the browser |
| [go-pflow](https://github.com/pflow-xyz/go-pflow) | Core library — ODE simulation, reachability analysis, P-invariants |
| **petri-pilot** | Code generator — compiles models into running applications |

A net designed in the editor can be analyzed by the library and compiled by petri-pilot without format conversion.

## Project Structure

```
cmd/petri-pilot/     CLI and MCP server entry point
pkg/mcp/             MCP server and tools
pkg/codegen/         Go and ES modules templates
pkg/serve/           Multi-model HTTP server
pkg/validator/       Model analysis
services/            Example models (tic-tac-toe, coffeeshop, knapsack, texas-holdem)
frontends/           Custom frontends for demos
generated/           Output from codegen (derived, not source)
```

## License

MIT
