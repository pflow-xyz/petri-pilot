# MCP Prompts Usage Guide

Petri-pilot provides MCP prompts that guide LLMs through complex multi-step design tasks. This guide explains how to use each prompt effectively.

## Overview

MCP (Model Context Protocol) prompts are interactive guides that help LLMs design Petri net models. They provide:
- Step-by-step guidance through complex tasks
- JSON Schema examples and patterns
- Context-aware suggestions based on your model
- Best practices and common patterns

## Available Prompts

| Prompt | Purpose | When to Use |
|--------|---------|-------------|
| `design-workflow` | Design a new Petri net model from requirements | Starting a new workflow |
| `add-access-control` | Add roles and permissions to a model | Implementing security |
| `add-views` | Design UI views and forms | Creating user interfaces |

## Using Prompts

Prompts are accessed through MCP clients like Claude Desktop or Cursor that support the MCP protocol.

### Basic Usage

```
# In Claude Desktop or Cursor
Use the design-workflow prompt with description: "order processing system"
```

The LLM will receive structured guidance and examples for designing the workflow.

## Prompt: design-workflow

**Purpose**: Guide through creating a new Petri net model from natural language requirements.

### When to Use

- Starting a new workflow design
- Converting requirements to a formal model
- Learning Petri net modeling basics

### Input Parameters

- `description` (optional): Natural language description of the workflow
  - Default: "a new workflow"
  - Example: "order processing system with validation and shipping"

### What You Get

The prompt provides:

1. **Step 1: Identify Places (States)**
   - Prompts for initial, intermediate, and terminal states
   - Examples: pending, in_progress, completed

2. **Step 2: Identify Transitions (Actions/Events)**
   - Prompts for user actions and system events
   - Examples: start_task, submit, approve

3. **Step 3: Define Arcs (Flow)**
   - How to connect places and transitions
   - Examples of valid arc patterns

4. **Step 4: Set Initial Marking**
   - How to specify starting state
   - Convention: one place with `initial: 1`

5. **JSON Schema Reference**
   - Complete schema structure
   - Properly formatted JSON examples

### Example Usage

#### Simple Workflow

```
Prompt: design-workflow with description: "task management"

Response includes guidance for:
- Places: pending, in_progress, review, completed
- Transitions: start, submit, approve, reject
- Arcs: connecting the workflow
```

#### Complex Workflow

```
Prompt: design-workflow with description: "multi-stage approval process with parallel reviews"

Response includes guidance for:
- Concurrent places for parallel review
- Synchronization transitions
- Advanced arc patterns
```

### Follow-up Questions

The prompt asks:
1. What are the main states/stages in your workflow?
2. Which state should be the starting point?
3. What are the possible end states?

Answer these to build a complete model iteratively.

### Output

A complete JSON model:

```json
{
  "name": "task-workflow",
  "version": "1.0.0",
  "description": "Simple task management workflow",
  "places": [
    {"id": "pending", "description": "Task not yet started", "initial": 1},
    {"id": "in_progress", "description": "Task being worked on", "initial": 0},
    {"id": "completed", "description": "Task finished", "initial": 0}
  ],
  "transitions": [
    {"id": "start", "description": "Begin work on task"},
    {"id": "complete", "description": "Mark task as done"}
  ],
  "arcs": [
    {"from": "pending", "to": "start"},
    {"from": "start", "to": "in_progress"},
    {"from": "in_progress", "to": "complete"},
    {"from": "complete", "to": "completed"}
  ]
}
```

## Prompt: add-access-control

**Purpose**: Guide through adding role-based access control to a Petri net model.

### When to Use

- Adding security to an existing model
- Defining user roles and permissions
- Implementing authorization rules

### Input Parameters

- `model` (optional): JSON string of the existing model
  - If omitted: General RBAC guidance
  - If provided: Specific guidance for your model's transitions

### What You Get

#### Without a Model

General guidance on:

1. **Step 1: Identify Actors/Users**
   - Types of users in the system
   - Their responsibilities
   - Role hierarchies

2. **Step 2: Define Roles**
   - Role schema with id, name, description
   - Inheritance patterns
   - Example role definitions

3. **Step 3: Map Transitions to Roles**
   - Access control rules
   - Wildcard patterns
   - Permission examples

4. **Step 4: Add Guard Conditions**
   - Expression-based access control
   - Examples: owner-only access

#### With a Model

Specific guidance including:
- List of all transitions in your model
- Suggested role structure for your domain
- Specific access mapping recommendations
- Guard expression examples for your transitions

### Example Usage

#### General Guidance

```
Prompt: add-access-control

Provides: Generic RBAC patterns and best practices
```

#### Model-Specific Guidance

```
Prompt: add-access-control with model: {
  "name": "order-processing",
  "transitions": [
    {"id": "validate"},
    {"id": "ship"},
    {"id": "cancel"}
  ]
}

Provides:
- "Your model has these transitions: validate, ship, cancel"
- Suggested roles: customer, fulfillment, admin
- Specific mapping recommendations
```

### Suggested Workflow

1. Start with the prompt to understand RBAC structure
2. Identify your user types
3. Define roles with clear responsibilities
4. Map each transition to appropriate roles
5. Add guard conditions for fine-grained control
6. Test with `petri_validate` tool

### Output

Extended model with RBAC:

```json
{
  "name": "order-processing",
  "roles": [
    {
      "id": "customer",
      "name": "Customer",
      "description": "End user placing orders"
    },
    {
      "id": "fulfillment",
      "name": "Fulfillment",
      "description": "Warehouse staff"
    },
    {
      "id": "admin",
      "name": "Administrator",
      "description": "Full access",
      "inherits": ["fulfillment"]
    }
  ],
  "access": [
    {"transition": "validate", "roles": ["fulfillment"]},
    {"transition": "ship", "roles": ["fulfillment"]},
    {"transition": "cancel", "roles": ["customer", "admin"], "guard": "user.id == customer_id || user.role == 'admin'"}
  ]
}
```

## Prompt: add-views

**Purpose**: Guide through designing UI views and forms for a Petri net model.

### When to Use

- Creating user interfaces for workflows
- Designing forms and tables
- Mapping UI actions to transitions

### Input Parameters

- `model` (optional): JSON string of the existing model
  - If omitted: General UI design guidance
  - If provided: Specific guidance for your model's structure

### What You Get

#### Without a Model

General guidance on:

1. **Step 1: Identify Data Fields**
   - What information to display/collect
   - Required vs optional fields
   - Field types and purposes

2. **Step 2: Design View Types**
   - Table views for lists
   - Form/Detail views for editing
   - Card views for dashboards

3. **Step 3: Map UI Actions to Transitions**
   - Action arrays in views
   - Transition availability

4. **Field Type Options**
   - Complete list of supported types
   - When to use each type

#### With a Model

Specific guidance including:
- All transitions available for actions
- Places/states in your workflow
- Suggested view structure for your domain
- Field mapping recommendations

### Example Usage

#### General Guidance

```
Prompt: add-views

Provides: Generic UI design patterns
```

#### Model-Specific Guidance

```
Prompt: add-views with model: {
  "name": "task-manager",
  "places": [
    {"id": "pending"},
    {"id": "in_progress"},
    {"id": "completed"}
  ],
  "transitions": [
    {"id": "start"},
    {"id": "complete"}
  ]
}

Provides:
- "States: pending, in_progress, completed"
- "Available actions: start, complete"
- Suggested views for task list and detail
```

### View Types

#### Table View

For displaying multiple instances:

```json
{
  "id": "task-list",
  "name": "Task List",
  "kind": "table",
  "groups": [
    {
      "id": "columns",
      "fields": [
        {"binding": "title", "label": "Title", "type": "text"},
        {"binding": "status", "label": "Status", "type": "text"},
        {"binding": "assignee", "label": "Assignee", "type": "text"}
      ]
    }
  ]
}
```

#### Form View

For creating/editing single instances:

```json
{
  "id": "task-form",
  "name": "Task Details",
  "kind": "form",
  "groups": [
    {
      "id": "basic",
      "name": "Basic Information",
      "fields": [
        {"binding": "title", "label": "Title", "type": "text", "required": true},
        {"binding": "description", "label": "Description", "type": "textarea"}
      ]
    }
  ],
  "actions": ["start", "complete"]
}
```

#### Detail View

For read-only instance display:

```json
{
  "id": "task-detail",
  "name": "Task Detail",
  "kind": "detail",
  "groups": [
    {
      "id": "info",
      "fields": [
        {"binding": "title", "label": "Title", "type": "text", "readonly": true},
        {"binding": "created_at", "label": "Created", "type": "date", "readonly": true}
      ]
    }
  ],
  "actions": ["start", "complete"]
}
```

### Field Types

| Type | Use For | Example |
|------|---------|---------|
| `text` | Short strings | Names, IDs, statuses |
| `textarea` | Long text | Descriptions, notes |
| `number` | Numeric values | Quantities, prices |
| `date` | Date only | Birth dates, deadlines |
| `datetime` | Date and time | Timestamps, appointments |
| `select` | Dropdown choices | Categories, statuses |
| `checkbox` | Boolean flags | Active, enabled |
| `email` | Email addresses | Contact info |
| `url` | Web addresses | Links, references |

### Best Practices

1. **Group related fields** - Use groups to organize forms
2. **Mark required fields** - Set `"required": true`
3. **Use readonly for display** - Prevent editing of computed/historical data
4. **Map actions to transitions** - Only include valid transitions
5. **Consider mobile** - Keep forms concise

### Output

Complete model with views:

```json
{
  "name": "task-manager",
  "views": [
    {
      "id": "task-list",
      "name": "Task List",
      "kind": "table",
      "groups": [
        {
          "id": "columns",
          "fields": [
            {"binding": "title", "label": "Title", "type": "text"},
            {"binding": "status", "label": "Status", "type": "text"}
          ]
        }
      ],
      "actions": ["start", "complete"]
    },
    {
      "id": "task-detail",
      "name": "Task Details",
      "kind": "detail",
      "groups": [
        {
          "id": "info",
          "name": "Information",
          "fields": [
            {"binding": "title", "label": "Title", "type": "text", "required": true},
            {"binding": "description", "label": "Description", "type": "textarea"},
            {"binding": "assignee", "label": "Assigned To", "type": "text"}
          ]
        }
      ],
      "actions": ["start", "complete"]
    }
  ]
}
```

## Combining Prompts

Use prompts in sequence to build complete models:

### Workflow

1. **design-workflow** - Create basic structure
   ```
   → Get: places, transitions, arcs
   ```

2. **add-access-control** - Add security
   ```
   → Get: roles, access rules, guards
   ```

3. **add-views** - Add UI
   ```
   → Get: views, forms, tables
   ```

### Example Session

```
# Step 1: Design
Prompt: design-workflow with description: "order processing"
Result: Basic model with states and transitions

# Step 2: Security
Prompt: add-access-control with model: <from step 1>
Result: Model + roles + access rules

# Step 3: UI
Prompt: add-views with model: <from step 2>
Result: Complete model ready for codegen
```

## Validation

After using prompts, validate your model:

```bash
# Validate the model
petri-pilot validate model.json

# Generate code
petri-pilot codegen model.json -o ./app/ --frontend
```

## Tips

### Start Simple

Begin with minimal models and add complexity incrementally:
1. Basic workflow (design-workflow)
2. Add one role (add-access-control)
3. Add one view (add-views)
4. Test and iterate

### Use Examples

Reference example models:
- `examples/order-processing.json` - Events First with views
- `examples/task-manager.json` - Simple workflow
- `examples/token-ledger.json` - Arcnet pattern

### Iterate

Prompts are designed for iteration:
- Use a prompt multiple times
- Refine based on validation feedback
- Build complexity gradually

### Combine with Tools

Use MCP tools alongside prompts:
- `petri_validate` - Check for errors
- `petri_analyze` - Verify reachability
- `petri_simulate` - Test execution

## See Also

- [Events First Pattern](events-first-pattern.md) - Schema design patterns
- [E2E Testing Guide](e2e-testing-guide.md) - Testing generated apps
- [Examples](../examples/) - Reference models
- [Architecture](../ARCHITECTURE.md) - Understanding the design
