# Petri Pilot

**From requirements to running applications, through verified Petri net models.**

Describe your system in plain language. An LLM designs a formal Petri net model. Validation catches deadlocks and structural errors. Deterministic codegen produces a complete application.

No LLM-generated code. The LLM designs models. Codegen produces apps.

## Quick Start

```bash
# Run as MCP server (for Claude Desktop, Cursor)
petri-pilot mcp

# Or CLI
petri-pilot generate -auto "order processing workflow"
petri-pilot codegen model.json -o ./app/
```

## What It Does

```
Natural Language → Petri Net Model → Validated → Generated App
     (LLM)           (formal)        (go-pflow)    (deterministic)
```

**Generated output**: workflow engine, event types, HTTP API, OpenAPI spec, tests, Docker setup.

## Use Cases

- Workflow engines (orders, approvals)
- Smart contracts (tokens, governance)
- Microservice orchestration (sagas)
- Game state machines
- IoT protocols

## MCP Tools

| Tool | Description |
|------|-------------|
| `petri_validate` | Check model for errors |
| `petri_analyze` | Reachability analysis |
| `petri_codegen` | Generate Go application |
| `petri_application` | Generate full-stack app |

## Why Petri Nets?

Minimal formalism: places (state), transitions (actions), arcs (connections). Four concepts, formal verification, expressive power.

## License

MIT
