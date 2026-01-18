# TODO

## Schema Redesign: Events First

Restructure the schema so events (form submissions) are the foundation, with workflow as a layer on top.

**Current approach:** Transitions have `event_type` and `bindings` as attributes - events are subordinate to workflow.

**Proposed approach:** Events define the data contract; workflow constrains valid event sequences.

```
Events (form submissions with typed payloads)
    ↓
Workflow (Petri net constraining valid event sequences)
    ↓
State (projection/fold over events)
    ↓
Views (derived from event schemas)
```

**Example structure:**
```json
{
  "events": [
    {
      "id": "order_submitted",
      "fields": [
        {"name": "customer_name", "type": "string", "required": true},
        {"name": "items", "type": "array", "of": "line_item"},
        {"name": "total", "type": "number"}
      ]
    },
    {
      "id": "order_validated",
      "fields": [
        {"name": "validated_by", "type": "string"}
      ]
    }
  ],
  "workflow": {
    "transitions": [
      {"id": "submit", "event": "order_submitted"},
      {"id": "validate", "event": "order_validated"}
    ],
    "places": [...],
    "arcs": [...]
  }
}
```

**Benefits:**
- Events define the data contract (what can be submitted)
- Workflow defines sequencing (when it's valid to submit)
- Views/forms derive from event schemas automatically
- Current state is `fold(events)` - pure event sourcing
- Closes the gap where views reference fields not defined in the model

---

# MCP Enhancements

## MCP Prompts

Add guided workflows that help LLMs design models step-by-step.

### Design Workflow Prompt
```
petri://prompts/design-workflow
```
Guide: "Design a workflow for: {description}"
- Ask about states/places
- Ask about transitions between states
- Ask about terminal states
- Generate initial model

### Add Access Control Prompt
```
petri://prompts/add-access-control
```
Guide: "Add roles and permissions to this model"
- Identify actors in the workflow
- Map actors to roles
- Assign transitions to roles
- Add guard conditions if needed

### Add Views Prompt
```
petri://prompts/add-views
```
Guide: "Design UI views for this workflow"
- Identify data fields per place
- Create list/table views
- Create detail/form views
- Map actions to transitions

---

## New MCP Tools

### petri_simulate
Fire transitions and see state changes without generating code.

```
petri_simulate(model, transitions[]) → resulting state
```

**Use case:** LLM verifies workflow behavior before codegen.

**Implementation:**
- Use existing Petri net engine from go-pflow
- Accept model + sequence of transition IDs
- Return final marking (token counts per place)
- Report if any transition was not enabled

### petri_extend
Modify existing model based on natural language instruction.

```
petri_extend(model, instruction) → modified model
```

**Use case:** "Add an approval step before shipping" without regenerating entire model.

**Implementation:**
- Parse instruction to identify operation (add place, add transition, add role, etc.)
- Apply modification to model
- Validate result
- Return modified model

### petri_preview
Preview a specific generated file without full codegen.

```
petri_preview(model, file: "api.go") → file content
```

**Use case:** LLM checks generated code before committing to full generation.

**Implementation:**
- Use existing template infrastructure
- Generate single file in memory
- Return content

### petri_diff
Compare two models structurally.

```
petri_diff(model_a, model_b) → differences
```

**Use case:** Understand what changed between model versions.

**Implementation:**
- Compare places, transitions, arcs
- Report added/removed/modified elements
- Show role and access changes

---

## Priority

1. **MCP Prompts** - Highest value for guided model design
2. **petri_simulate** - Verify behavior without codegen
3. **petri_preview** - Quick feedback on generation
4. **petri_extend** - Incremental refinement
5. **petri_diff** - Version comparison (lower priority)

---

## Implementation Notes

### Prompts Registration
```go
s.AddPrompt(
    mcp.NewPrompt("design-workflow",
        mcp.WithPromptDescription("Guide through designing a new workflow"),
        mcp.WithArgument("description", "What the workflow should do"),
    ),
    handleDesignWorkflowPrompt,
)
```

### Prompt Handler
```go
func handleDesignWorkflowPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
    description := request.Arguments["description"]
    return &mcp.GetPromptResult{
        Messages: []mcp.PromptMessage{
            {Role: "user", Content: mcp.TextContent{Text: "...guided prompt..."}},
        },
    }, nil
}
```
