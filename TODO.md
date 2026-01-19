# TODO

## Completed

### Schema Redesign: Events First ✅

Implemented in commits `afce0d0` and `40668e9`.

- Events are first-class schema citizens defining the complete data contract
- Bindings define operational data for state computation (arcnet pattern)
- Views validate field bindings against event fields
- Backward compatible with models that don't define explicit events

### MCP Tools ✅

- **petri_extend** - Modify models with operations (add/remove places, transitions, arcs, roles, events, bindings)
- **petri_preview** - Preview a specific generated file without full codegen
- **petri_diff** - Compare two models structurally

---

## Remaining

### MCP Prompts

Add guided workflows that help LLMs design models step-by-step.

#### Design Workflow Prompt
```
petri://prompts/design-workflow
```
Guide: "Design a workflow for: {description}"
- Ask about states/places
- Ask about transitions between states
- Ask about terminal states
- Generate initial model

#### Add Access Control Prompt
```
petri://prompts/add-access-control
```
Guide: "Add roles and permissions to this model"
- Identify actors in the workflow
- Map actors to roles
- Assign transitions to roles
- Add guard conditions if needed

#### Add Views Prompt
```
petri://prompts/add-views
```
Guide: "Design UI views for this workflow"
- Identify data fields per place
- Create list/table views
- Create detail/form views
- Map actions to transitions

---

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

---

## Priority

1. **MCP Prompts** - Highest value for guided model design
2. **petri_simulate** - Verify behavior without codegen

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
