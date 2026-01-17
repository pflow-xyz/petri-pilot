# Petri Pilot

**From requirements to running applications, through verified models.**

## The Pattern

Every event-driven system follows the same shape: events trigger state transitions, transitions update aggregated state. Workflow engines, smart contracts, microservice orchestration, game state machines, IoT protocols—different domains, identical structure.

Petri nets capture this structure precisely. Places hold state. Transitions move it. The formalism is minimal, but the behavior it expresses is not.

## The Gap

Building event-driven applications means implementing this pattern from scratch, every time. The state machine lives in your head until it lives in code—where it's hard to see, harder to verify, and scattered across files.

Models should come first. But modeling requires expertise most teams don't have time to build.

## The Bridge

Petri Pilot lets you describe your system in natural language. An LLM designs the model in conversation. Formal tools validate structure and behavior. Deterministic code generation produces the application.

```
"A checkout workflow with cart, payment, and shipping"
```

becomes a verified Petri net, then a running application with event store, aggregate, and API—ready to deploy.

## The Principle

**No LLM-generated code.**

The LLM designs models. It understands intent, handles ambiguity, iterates on feedback. But it doesn't write your application.

Code generation is deterministic. Same model, same output, every time. The generated code uses clean runtime interfaces you can implement for any backend—SQLite for testing, Postgres for production, blockchain for smart contracts.

## The Architecture

```
┌─────────────────────────────────────────┐
│  MCP Client (Claude Desktop, Cursor)   │
│  LLM designs model in conversation      │
└─────────────────┬───────────────────────┘
                  │ MCP tools
    ┌─────────────▼─────────────┐
    │  petri-pilot MCP server   │
    │  validate │ analyze │ gen │
    └─────────────┬─────────────┘
                  │ deterministic
    ┌─────────────▼─────────────┐
    │  Generated Application    │
    │  workflow │ events │ api  │
    └─────────────┬─────────────┘
                  │ implements
    ┌─────────────▼─────────────┐
    │  Runtime Interfaces       │
    │  EventStore │ Aggregate   │
    └───────────────────────────┘
```

## What Gets Generated

From a single validated model:

- **State machine** with guards and transition logic
- **Event types** derived from transitions
- **Aggregate** for event-sourced state
- **HTTP handlers** per transition
- **OpenAPI spec** for the API
- **Tests** using the SQLite runtime

The generated project is complete. Run the tests, wire up your storage, deploy.

## Target Applications

The same architecture serves:

- **Workflow engines**: order processing, approval flows, document pipelines
- **Smart contracts**: tokens, governance, vesting schedules
- **Microservice orchestration**: sagas, compensation, distributed state
- **Game state machines**: turn logic, resource management, multiplayer sync
- **IoT protocols**: device state, command sequences, sensor pipelines

All share: **events → transitions → state**

## The Loop

Design is iterative. The LLM proposes, the validator checks, feedback drives refinement.

```
Intent → Design → Validate → Feedback → Refine → Generate
   ↑                                       │
   └───────────────────────────────────────┘
```

Deadlocks, unreachable states, unconnected elements—caught before code exists. The model is correct by construction. The code follows.

## Why This Matters

**For designers**: Describe systems in words. See them take shape as formal models.

**For developers**: Skip the boilerplate. Get event-sourced architecture from a specification.

**For teams**: A shared model everyone can read, simulate, and extend—then generate the implementation.

## Try It

```bash
# Design via MCP (Claude Desktop, Cursor, etc.)
petri-pilot mcp

# Or generate directly
petri-pilot generate -auto "order processing workflow"
petri-pilot codegen model.json -lang go -o ./myworkflow/
cd myworkflow && go test ./...
```

Watch the model evolve. Generate the application. Ship it.
