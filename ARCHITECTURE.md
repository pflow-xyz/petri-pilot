# Architecture

Petri-pilot transforms declarative models into executable applications. The architecture follows a categorical pattern where the model is the source of truth and all artifacts are deterministic projections.

## Core Insight: Inside-Out Design

Traditional code generation is "outside-in": templates consume ad-hoc data structures. Petri-pilot inverts this. The **model** contains everything needed to describe an application:

- State machine topology (places, transitions, arcs)
- Access control (roles, guards)
- UI structure (views, navigation)
- Operational config (event sourcing, admin)

Code is a **projection** of this model into a target language. The model is human-readable, LLM-readable, and formally verifiable. Generated code is an implementation detail.

## Categorical Structure

The codebase implements a functor pattern:

```
Schema ──EnrichModel──▶ Context ──Template──▶ Artifact
  │                        │                      │
  │                        │                      │
Model                  Universal              Go/JS/YAML
(source)               Object                 (target)
```

### Schema → Context (Functor)

`pkg/codegen/golang/context.go`:

```go
func NewContext(model *schema.Model, opts ContextOptions) (*Context, error)
```

The `Context` struct is the **universal intermediate representation**. It contains:

1. **Primitives** - Places, Transitions, Arcs (from Petri net theory)
2. **Derived structures** - Events, Routes, Handlers (computed from primitives)
3. **Feature flags** - HasViews, HasAdmin, HasNavigation (conditional generation)
4. **Helper methods** - TransitionRequiresAuth, GetEnabledTransitions (template utilities)

This is the morphism carrier. Every piece of information needed by any template is accessible through Context.

### Context → Artifact (Projections)

Each `.tmpl` file is a projection:

```
Context ──api.tmpl──────▶ api.go
Context ──workflow.tmpl─▶ workflow.go
Context ──events.tmpl───▶ events.go
Context ──main.tmpl─────▶ main.js
Context ──admin.tmpl────▶ admin.js
```

Templates are pure functions of Context. Given the same Context, they produce identical output. No external state, no randomness.

### Composition

The full pipeline:

```go
// 1. Parse JSON into schema
model := schema.Parse(jsonBytes)

// 2. Validate Petri net properties
result := validator.Validate(model)

// 3. Transform to Context (functor application)
ctx := golang.NewContext(model, opts)

// 4. Project to artifacts (multiple projections from same object)
apiGo := templates.Execute("api", ctx)
workflowGo := templates.Execute("workflow", ctx)
mainJs := templates.Execute("main", ctx)
```

Each step is deterministic. The same model always produces the same code.

## Package Structure

```
pkg/
├── schema/           # Model types (the source of truth)
│   └── schema.go     # Place, Transition, Arc, Role, View, etc.
│
├── codegen/
│   ├── golang/       # Go backend generation
│   │   ├── context.go    # Universal object (Context struct)
│   │   ├── generator.go  # Orchestrates template execution
│   │   └── templates/    # Projections (*.tmpl files)
│   │
│   └── esmodules/    # Frontend generation (vanilla JS)
│       ├── context.go    # Frontend-specific Context
│       └── templates/    # Frontend projections
│
├── dsl/              # Guard expression language
│   ├── parser.go     # Parses "balances[from] >= amount"
│   └── evaluator.go  # Runtime evaluation
│
├── runtime/          # Runtime interfaces
│   ├── eventstore/   # Event storage abstraction
│   └── aggregate/    # Event-sourced aggregate pattern
│
└── mcp/              # MCP server for LLM interaction
    └── tools.go      # petri_validate, petri_codegen, etc.
```

## Key Design Decisions

### 1. Model Contains UI Structure

Views, navigation, and admin config are in the model, not templates. An LLM can reason about the complete application by reading the JSON.

```json
{
  "views": [{
    "id": "order-detail",
    "kind": "detail",
    "groups": [{ "fields": [...] }],
    "actions": ["approve", "reject"]
  }]
}
```

The LLM sees what the user will see.

### 2. Guards as Expressions

Access control uses a DSL, not code:

```json
{
  "guard": "balances[from] >= amount && from != to"
}
```

The expression is:
- **Readable** - Both humans and LLMs understand it
- **Verifiable** - Can be analyzed statically
- **Portable** - Same expression generates Go and JS code

### 3. Feature Flags from Model

Templates use `{{if .HasAdmin}}` conditionally. The flag is computed from model presence:

```go
func (c *Context) HasAdmin() bool {
    return c.Admin != nil && c.Admin.Enabled
}
```

No magic. If admin config exists, admin code generates.

### 4. Events from Transitions

Event types are derived, not declared:

```go
// Transition "validate_order" → Event "ValidateOrder"
// Transition "ship" → Event "Ship"
```

The event schema is the transition schema. No redundant definitions.

## Extension Points

### Adding a New Target

1. Create `pkg/codegen/newtarget/`
2. Define `Context` struct with target-specific fields
3. Implement `NewContext(model) Context`
4. Write templates in `templates/`
5. Register in generator

The model doesn't change. Only projections multiply.

### Adding Model Features

1. Add types to `pkg/schema/schema.go`
2. Add to JSON Schema (`schema/petri-model.schema.json`)
3. Add Context fields/methods
4. Update templates that need the feature
5. Feature flag pattern: `{{if .HasFeature}}`

### Adding Guard Functions

1. Add to `pkg/dsl/evaluator.go`
2. Functions are available in all guard expressions
3. Document in schema

## LLM Consumption

The model is designed for LLM interaction:

1. **JSON Schema** (`schema/petri-model.schema.json`) - LLMs can validate before submitting
2. **Examples** - `examples/*.json` show complete models
3. **MCP Tools** - `petri_validate` gives structured feedback

An LLM workflow:
1. Generate model JSON from requirements
2. Validate against schema (client-side)
3. Call `petri_validate` for Petri net analysis
4. Iterate on feedback
5. Call `petri_codegen` to generate application

The model is the contract. Everything else is derivable.

## Runtime

Generated applications use shared runtime packages:

- `pkg/runtime/eventstore` - Event storage interface (SQLite implementation)
- `pkg/runtime/aggregate` - Event-sourced aggregate pattern
- `pkg/runtime/api` - HTTP utilities (router, JSON helpers)

Runtime is not generated. It's imported. This keeps generated code focused on domain logic.

## Why Petri Nets?

Petri nets provide:

1. **Formal semantics** - States, transitions, concurrency are well-defined
2. **Verification** - Deadlock detection, reachability analysis
3. **Minimality** - Four concepts: places, transitions, arcs, tokens
4. **Visual intuition** - Easy to diagram and understand

The formal foundation enables both LLM reasoning (structured) and human reasoning (visual).
