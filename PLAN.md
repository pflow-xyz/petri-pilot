# Code-to-Flows: Petri-Pilot MCP Tool

## Overview

Add a `petri_code_to_flow` tool to petri-pilot's MCP server that converts source code into Petri net models. This targets the "codetoflows" search query (position ~7 across blog/stackdump.com) where competitors are thin LLM wrappers over Gemini/Mistral generating Mermaid.js diagrams.

Our differentiator: real formal Petri net models that are executable, verifiable, and round-trippable.

## How It Works

```
Source Code → LLM Analysis → Petri Net Model (JSON-LD) → Validation → Visualization
```

1. User provides source code via MCP tool call
2. Claude analyzes the code to identify:
   - Control flow (if/else, loops, error paths)
   - State machines (enums, status fields, FSM patterns)
   - Resource management (acquire/release, pools, queues)
   - Concurrency patterns (goroutines, channels, mutexes)
3. Generates a Petri net model in pflow.xyz JSON-LD schema
4. Validates the model (deadlocks, liveness, boundedness)
5. Returns the model + validation results + visualization URL

## MCP Tool Design

### `petri_code_to_flow`

**Input:**
```json
{
  "code": "string — source code to analyze",
  "language": "string — go|javascript|python|java|etc (optional, auto-detect)",
  "focus": "string — control-flow|state-machine|resources|concurrency (optional)",
  "name": "string — model name (optional)"
}
```

**Output:**
```json
{
  "model": { /* pflow.xyz JSON-LD Petri net */ },
  "validation": { /* deadlocks, liveness, boundedness */ },
  "summary": "string — description of what was modeled",
  "preview_url": "string — pflow.xyz visualization link"
}
```

### Implementation Approach

The tool uses Claude (via anthropic SDK, already a dependency) to:
1. Parse the code and identify modelable patterns
2. Generate a Petri net JSON-LD model following the pflow.xyz schema
3. The generated model is then validated using the existing `petri_validate` / `petri_analyze` pipeline

This is NOT just "ask an LLM to draw a flowchart." The LLM's output is a formal Petri net that gets machine-verified for correctness.

## Implementation Steps

### Phase 1: Core Tool

1. **Add tool registration** in `pkg/mcp/server.go`
   - Register `petri_code_to_flow` tool with input schema
   - Add handler function

2. **Create code analyzer** in `pkg/mcp/codetoflow.go`
   - Build prompt that instructs Claude to:
     - Identify places (states, resources, queues)
     - Identify transitions (actions, events, state changes)
     - Identify arcs (dependencies, resource consumption/production)
     - Output valid pflow.xyz JSON-LD schema
   - Include few-shot examples from existing services (coffeeshop, tcp-handshake, producer-consumer)
   - Parse and validate the LLM response

3. **Validate generated model**
   - Run through existing `petri_validate` for structural correctness
   - Run `petri_analyze` for behavioral properties
   - Auto-fix common issues (unconnected nodes, missing arcs)

4. **Return enriched result**
   - Model JSON-LD
   - Validation report
   - Link to pflow.xyz visualization

### Phase 2: Refinement

5. **Focus modes** — specialized prompts for:
   - `control-flow`: if/else, loops, error handling → places as states, transitions as decisions
   - `state-machine`: enum/status tracking → places as states, transitions as events
   - `resources`: pools, connections, memory → places as resources, transitions as acquire/release
   - `concurrency`: goroutines, channels → places as buffers, transitions as send/receive

6. **Language-specific patterns**
   - Go: goroutine/channel detection, error handling patterns, interface dispatch
   - JavaScript: async/await, promise chains, event emitters
   - Python: generators, context managers, async patterns

### Phase 3: SEO & Content

7. **Blog post** on blog.stackdump.com targeting "code to flows" keyword
   - Compare LLM-wrapper approach vs formal Petri net modeling
   - Show examples of code → model → verification → code round-trip
   - Include interactive pflow.xyz embeds

8. **Landing page or example** on pflow.xyz
   - "Code to Flows" section showing the capability
   - Live demo with sample code snippets

## Key Files to Modify

| File | Change |
|------|--------|
| `pkg/mcp/server.go` | Register new tool, add handler |
| `pkg/mcp/codetoflow.go` | New file — code analysis + model generation |
| `pkg/mcp/prompts.go` | Add `code-to-flow` guided prompt |
| `services/` | Add example models generated from real code |

## Dependencies

- `anthropic-sdk-go` (already in go.mod) — for Claude API calls
- `go-pflow` (already in go.mod) — for model validation
- Existing MCP infrastructure in `pkg/mcp/server.go`

## Success Criteria

- Tool generates valid Petri net models from Go/JS/Python code
- Models pass structural validation (no unconnected nodes, valid arcs)
- Models demonstrate meaningful properties (not trivial linear flows)
- Round-trip works: generated model → codegen → produces compilable code
- "codetoflows" query improves from position ~7 to top 5
