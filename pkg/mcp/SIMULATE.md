# petri_simulate MCP Tool

The `petri_simulate` tool allows you to fire transitions and see state changes without generating code. It returns detailed step-by-step state traces.

## Use Cases

- Verify workflow reaches terminal state
- Test guard conditions
- Explore branching paths
- Validate model before codegen

## API

```
petri_simulate(model, steps[]) → simulation result
```

### Parameters

- `model` (required): The Petri net model as JSON
- `steps` (optional): JSON array of simulation steps with optional bindings
- `transitions` (optional): JSON array of transition IDs (legacy, use `steps` instead)

### Example Using Steps (New API)

```json
{
  "model": "{...petri net model...}",
  "steps": [
    {
      "transition": "submit",
      "bindings": {
        "author": "alice",
        "title": "My Post"
      }
    },
    {
      "transition": "approve"
    }
  ]
}
```

### Example Using Transitions (Legacy API)

```json
{
  "model": "{...petri net model...}",
  "transitions": ["submit", "approve"]
}
```

## Response Format

```json
{
  "success": true,
  "final_state": {
    "draft": 0,
    "published": 1
  },
  "steps": [
    {
      "transition": "submit",
      "enabled": true,
      "state_before": {
        "draft": 1,
        "in_review": 0,
        "published": 0
      },
      "state_after": {
        "draft": 0,
        "in_review": 1,
        "published": 0
      }
    },
    {
      "transition": "approve",
      "enabled": true,
      "state_before": {
        "draft": 0,
        "in_review": 1,
        "published": 0
      },
      "state_after": {
        "draft": 0,
        "in_review": 0,
        "published": 1
      }
    }
  ]
}
```

## Error Handling

When a transition is not enabled, the result includes detailed error information:

```json
{
  "success": false,
  "steps": [
    {
      "transition": "approve",
      "enabled": false,
      "state_before": {"draft": 1},
      "state_after": {"draft": 1},
      "error": "insufficient tokens in: in_review"
    }
  ]
}
```

## Implementation Details

- Uses the go-pflow engine to execute transitions
- Captures state before and after each transition
- Reports disabled transitions with specific reasons
- Maintains backwards compatibility with legacy API
