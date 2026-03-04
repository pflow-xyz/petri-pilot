# petri-pilot

Generate full-stack applications from Petri net models.

## What it does

petri-pilot is an MCP server that lets LLMs design, validate, simulate, and generate complete applications from Petri net workflow specifications. It produces Go backends with event sourcing and vanilla JavaScript frontends.

## Capabilities

- **Model design**: Validate and analyze Petri net models for correctness, deadlocks, and liveness
- **Code to flow**: Convert source code into formal Petri net models (requires ANTHROPIC_API_KEY)
- **Simulation**: Fire transitions and trace state changes before generating code
- **Code generation**: Produce Go backends with SQLite, event sourcing, and REST APIs
- **Frontend generation**: Generate vanilla ES modules frontends (no React/npm bloat)
- **Full-stack apps**: Generate complete applications from high-level entity/role/page specs
- **Service management**: Start, stop, and monitor generated services
- **Model operations**: Diff, extend, migrate, visualize, and document models

## Installation

```bash
# Go
go install github.com/pflow-xyz/petri-pilot/cmd/petri-pilot@latest

# Docker
docker run -i ghcr.io/pflow-xyz/petri-pilot mcp
```

## MCP Configuration

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

## Example workflow

1. Design a model: `petri_validate(model='{"name":"order",...}')`
2. Simulate behavior: `petri_simulate(model='...', transitions='["ship"]')`
3. Generate code: `petri_codegen(model='...', language='go')`
4. Start service: `service_start(directory='/path/to/app')`
5. Test in browser and iterate
