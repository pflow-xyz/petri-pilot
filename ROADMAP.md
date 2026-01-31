# Petri-Pilot Roadmap

## Vision

Move the GraphQL infrastructure from petri-pilot into go-pflow as a first-class feature. Rather than operating as a traditional ORM that maps models to database tables, the GraphQL layer will **serve the Petri net models themselves** - exposing schema introspection, simulation, analysis, and state management directly over GraphQL.

## Current State (petri-pilot)

Petri-pilot currently provides:
- `/graphql` - Unified GraphQL endpoint for multiple services
- `/graphql/i` - Interactive GraphQL playground with:
  - Schema explorer
  - Query editor with autocomplete
  - Models tab showing registered Petri nets
  - Documentation panel
- Code generation producing Go backends with GraphQL APIs
- Event-sourced aggregates with state reconstruction

### What Exists Today

| Component | Status | Location |
|-----------|--------|----------|
| Unified GraphQL endpoint | ✅ Working | `pkg/serve/graphql.go` |
| Schema merging/namespacing | ✅ Working | `pkg/serve/graphql.go` |
| Introspection support | ✅ Working | `buildIntrospection()` |
| Interactive playground | ✅ Working | `/graphql/i` |
| Model-driven schema generation | ✅ Working | `templates/graphql_*.tmpl` |
| Generated resolvers | ✅ Working | `generated/*/graphql.go` |

## Target State (go-pflow)

go-pflow will provide a standalone GraphQL server that:

### 1. Model-First Schema Generation

Instead of writing GraphQL schemas by hand, derive them directly from Petri net definitions:

```graphql
# Auto-generated from Petri net model
type OrderWorkflow {
  # Places become state fields
  pending: Int!
  processing: Int!
  shipped: Int!
  delivered: Int!

  # Transitions become mutations
  ship(orderId: ID!): OrderWorkflow!
  deliver(orderId: ID!): OrderWorkflow!

  # Computed from model
  enabledTransitions: [String!]!
  currentState: JSON!
}
```

### 2. Simulation API

Run ODE simulations and explore reachability directly via GraphQL:

```graphql
type Query {
  # Run ODE simulation
  simulate(
    model: ID!
    initialMarking: JSON
    duration: Float
    steps: Int
  ): SimulationResult!

  # Analyze model properties
  analyze(model: ID!): AnalysisResult!

  # Explore reachability graph
  reachability(
    model: ID!
    maxDepth: Int
  ): ReachabilityGraph!
}

type SimulationResult {
  timeline: [TimeStep!]!
  finalState: JSON!
  convergence: Float
}

type AnalysisResult {
  bounded: Boolean!
  live: Boolean!
  deadlocks: [Marking!]!
  invariants: [Invariant!]!
}
```

### 3. Model Registry

Manage Petri net models as first-class resources:

```graphql
type Query {
  models: [PetriNet!]!
  model(id: ID!): PetriNet
}

type Mutation {
  createModel(input: PetriNetInput!): PetriNet!
  updateModel(id: ID!, input: PetriNetInput!): PetriNet!
  deleteModel(id: ID!): Boolean!

  # Model operations
  extend(id: ID!, operations: [Operation!]!): PetriNet!
  validate(id: ID!): ValidationResult!
  migrate(id: ID!, toVersion: Int!): PetriNet!
}

type PetriNet {
  id: ID!
  name: String!
  version: Int!
  places: [Place!]!
  transitions: [Transition!]!
  arcs: [Arc!]!

  # Computed
  schema: String!  # Generated GraphQL schema for this model
  openapi: String! # Generated OpenAPI spec
  documentation: String! # Generated markdown docs
}
```

### 4. Instance Management

Create and manage instances of workflows:

```graphql
type Query {
  instances(model: ID!): [Instance!]!
  instance(id: ID!): Instance
}

type Mutation {
  createInstance(model: ID!, initialData: JSON): Instance!
  fireTransition(
    instance: ID!
    transition: String!
    bindings: JSON
  ): Instance!

  # Batch operations
  fireSequence(
    instance: ID!
    transitions: [TransitionInput!]!
  ): Instance!
}

type Instance {
  id: ID!
  model: PetriNet!
  marking: JSON!
  state: JSON!
  events: [Event!]!
  enabledTransitions: [String!]!

  # Subscriptions for real-time updates
  subscribe: InstanceSubscription!
}
```

### 5. Real-time Subscriptions

```graphql
type Subscription {
  # Watch instance state changes
  instanceUpdated(id: ID!): Instance!

  # Watch simulation progress
  simulationProgress(id: ID!): SimulationStep!

  # Watch for enabled transitions
  transitionsEnabled(instance: ID!): [String!]!
}
```

## Migration Plan

### Phase 1: Extract Core GraphQL Infrastructure

**Goal:** Standalone GraphQL server in go-pflow that serves any Petri net model.

**Tasks:**

1. **Create `pkg/graphql/` in go-pflow**
   ```
   pkg/graphql/
   ├── server.go       # HTTP handler, middleware
   ├── schema.go       # Schema generation from model
   ├── resolver.go     # Resolver interfaces
   ├── introspect.go   # Introspection support
   └── playground.go   # GraphiQL/playground UI
   ```

2. **Port from petri-pilot:**
   - `pkg/serve/graphql.go` → `pkg/graphql/server.go`
   - Schema SDL generation logic → `pkg/graphql/schema.go`
   - Introspection builder → `pkg/graphql/introspect.go`

3. **New interfaces:**
   ```go
   // pkg/graphql/resolver.go
   type ModelResolver interface {
       Query(ctx context.Context, name string, args map[string]any) (any, error)
       Mutate(ctx context.Context, name string, args map[string]any) (any, error)
   }
   ```

4. **Update petri-pilot** to import go-pflow's GraphQL package

### Phase 2: Model-Driven Schema Generation

1. Implement schema generator that reads `schema.Model`
2. Auto-generate types for places, transitions, arcs
3. Generate query/mutation resolvers from model definition
4. Support custom extensions (roles, views, admin)

### Phase 3: Simulation & Analysis API

1. Expose ODE solver via GraphQL
2. Add reachability analysis queries
3. Implement analysis result types (boundedness, liveness, deadlocks)
4. Add sensitivity analysis for element importance

### Phase 4: Subscriptions & Real-time

1. Implement GraphQL subscriptions
2. Add WebSocket transport
3. Real-time instance state updates
4. Simulation progress streaming

### Phase 5: Deprecate petri-pilot GraphQL

1. Update petri-pilot to use go-pflow's GraphQL
2. Remove duplicate code from petri-pilot
3. Petri-pilot becomes a thin wrapper/demo layer

## Design Principles

### Models as APIs

Every Petri net model automatically becomes a fully-typed GraphQL API. No manual schema writing required.

### Introspection-First

The GraphQL schema itself is introspectable, but so are the underlying Petri net models. Query the structure, analyze properties, understand behavior.

### Simulation as a Service

ODE simulation isn't just a local computation - it's an API you can call. Run simulations, compare scenarios, explore what-if analyses.

### Event Sourcing Native

All state changes are events. GraphQL mutations produce events. Subscriptions emit events. The event log is queryable.

### Zero Configuration

Point go-pflow at a model JSON file, get a working GraphQL API. No code generation step required for basic usage.

## Example Usage

```go
package main

import (
    "github.com/pflow-xyz/go-pflow/pkg/graphql"
    "github.com/pflow-xyz/go-pflow/pkg/schema"
)

func main() {
    // Load models
    models := []schema.Model{
        schema.MustLoad("order-workflow.json"),
        schema.MustLoad("coffee-shop.json"),
    }

    // Create GraphQL server
    server := graphql.NewServer(
        graphql.WithModels(models...),
        graphql.WithPlayground("/graphql/i"),
        graphql.WithSubscriptions(),
        graphql.WithPersistence(sqlite.New("workflows.db")),
    )

    // Serve
    http.ListenAndServe(":8080", server)
}
```

## Success Criteria

1. **Zero-config GraphQL**: Load a model, get an API
2. **Full introspection**: Query model structure via GraphQL
3. **Simulation access**: Run ODE simulations via GraphQL queries
4. **Real-time updates**: Subscriptions for state changes
5. **Backwards compatible**: Existing petri-pilot demos continue to work
6. **Documentation**: Auto-generated docs from model metadata

## Current Progress (Q1 2026)

### Phase 1 Status: ✅ Complete

The `graphql/` package has been created in go-pflow with:

| Component | Status | File |
|-----------|--------|------|
| Schema generation from `petri.PetriNet` | ✅ Done | `graphql/schema.go` |
| Unified schema (multi-model) | ✅ Done | `GenerateUnifiedSchema()` |
| Introspection handler | ✅ Done | `graphql/introspect.go` |
| Playground UI | ✅ Done | `graphql/playground.go` |
| HTTP server | ✅ Done | `graphql/server.go` |
| Resolver interfaces | ✅ Done | `graphql/resolver.go` |
| Query/argument parser | ✅ Done | `graphql/parser.go` |
| EventSource adapter | ✅ Done | `graphql/eventsource.go` |
| Tests (17 passing) | ✅ Done | `graphql/*_test.go` |
| Example server | ✅ Done | `graphql/example/main.go` |

### What Still Needs Work

| Component | Priority | Status |
|-----------|----------|--------|
| Port petri-pilot to use go-pflow | High | ✅ Complete |
| Replace UnifiedGraphQL with go-pflow | High | ✅ Complete |
| Migrate petri-pilot services to go-pflow Server | Medium | ✅ Complete (multi-service mode) |
| SQLite persistence integration test | Medium | Not started |
| Subscriptions (WebSocket) | Low | Not started |

**Note:** Single-service mode uses the generated GraphQL handler (pre-existing). Multi-service mode now uses go-pflow's Server with full introspection support.

### Next Steps (Phase 2)

1. ✅ **Use go-pflow introspection** - petri-pilot now uses `gopflowgql.BuildIntrospection()` instead of local implementation
2. ✅ **Use go-pflow helpers** - petri-pilot now uses `gopflowgql.IsIntrospectionQuery()` and `gopflowgql.ParseOperationNames()`
3. ✅ **Add ExternalService support** - go-pflow's Server now supports external services with custom schemas/resolvers via `WithExternalService()`
4. **Migrate petri-pilot to go-pflow Server** - Update petri-pilot to use `gopflowgql.Server` with `ExternalService` for its generated services
5. **Remove duplicate code** - Delete petri-pilot's schema combination logic after migration
6. **Integration test** - End-to-end test with SQLite persistence
7. **Add simulation API** - Expose ODE solver via GraphQL queries

**Code changes:**
- petri-pilot's `pkg/serve/graphql.go`: 654 → ~450 lines (31% smaller, removed duplicate introspection code)
- `NewUnifiedGraphQLFromGoPflow()` is now the default constructor (replaces `NewUnifiedGraphQL()`)
- Added `ToExternalService()` converter for integrating with go-pflow
- `ServeHTTP` delegates to go-pflow's Server when available
- Multi-service mode now has full GraphQL introspection support

**go-pflow additions:**
- `ExternalService` type for integrating external schemas/resolvers
- `ExternalResolver` type matching petri-pilot's `GraphQLResolver`
- `WithExternalService()` option for Server
- `CombineSchemas()` for merging base + external schemas
- `extractSchemaComponents()` and `extractAndNamespaceSchema()` helpers
- 19 passing tests (2 new for external service support, 1 with introspection verification)

## Timeline

- **Q1 2026**: Phase 1-2 (Core infrastructure, schema generation)
- **Q2 2026**: Phase 3 (Simulation & analysis API)
- **Q3 2026**: Phase 4-5 (Subscriptions, migration)

## Technical Decisions

### GraphQL Library Choice

**Options:**
1. **gqlgen** - Code generation, type-safe, most popular
2. **graphql-go** - Runtime schema, more flexible, less boilerplate
3. **Custom** - Current petri-pilot approach, minimal dependencies

**Recommendation:** Use **graphql-go** for go-pflow because:
- Runtime schema generation fits model-driven approach
- No code generation step required
- Petri net models already define types at runtime
- Lower barrier to "load model, get API"

### Schema Generation Strategy

Two approaches for generating GraphQL types from Petri nets:

**Option A: Direct mapping**
```
Place → Field (Int!)
Transition → Mutation
Binding → Input argument
```

**Option B: Generic wrapper**
```graphql
type Instance {
  marking: JSON!
  fire(transition: String!, bindings: JSON): Instance!
}
```

**Recommendation:** Start with **Option B** (simpler), evolve to **Option A** (better DX) in Phase 2.

### Persistence Layer

The current petri-pilot uses SQLite event stores. go-pflow GraphQL should:
- Accept a `Persister` interface
- Default to in-memory for zero-config
- Allow SQLite/Postgres via dependency injection

```go
type Persister interface {
    SaveEvent(streamID string, event Event) error
    LoadEvents(streamID string) ([]Event, error)
}
```

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Scope creep (adding too many features) | High | Focus on zero-config use case first |
| Breaking petri-pilot during migration | Medium | Maintain compatibility layer |
| Performance with large models | Low | Lazy schema generation, caching |
| GraphQL spec compliance | Medium | Use established library, test with standard clients |

## Related Work

- [go-pflow](https://github.com/pflow-xyz/go-pflow) - Core Petri net library
- [petri-pilot](https://github.com/pflow-xyz/petri-pilot) - Current demo/tutorial platform
- [graphql-go](https://github.com/graphql-go/graphql) - Runtime GraphQL for Go (recommended)
- [gqlgen](https://github.com/99designs/gqlgen) - Code-gen GraphQL (alternative)

---

## Appendix: Example Improvements (Completed)

Previous roadmap items for reference:

| Example | Custom Frontend | Deployed | Status |
|---------|-----------------|----------|--------|
| tic-tac-toe | Game board, turn indicator | pilot.pflow.xyz | ✅ |
| coffeeshop | Order queue, barista view | pilot.pflow.xyz | ✅ |
| texas-holdem | Poker table, card rendering | pilot.pflow.xyz | ✅ |
| erc20-token | Wallet balances, transfers | pilot.pflow.xyz | ✅ |
| blog-post | Editor, approval flow | pilot.pflow.xyz | ✅ |
| support-ticket | Ticket thread view | pilot.pflow.xyz | ✅ |
