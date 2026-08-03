
# predator-prey

Lotka-Volterra predator-prey dynamics modeled as a Petri net with ODE simulation

## Quick Start

```bash
# Build and run
go build -o server .
./server

# Server starts on http://localhost:8080
```

## Architecture

This application uses **event sourcing** with a **Petri net** state machine to model workflows. All state changes are captured as immutable events, enabling:

- Full audit trail of all transitions
- Time-travel debugging
- Event replay for recovery
- Deterministic state reconstruction

## State Machine

### Places (States)

| Place | Type | Initial | Description |
|-------|------|---------|-------------|
| `prey` | Token | 100 | Prey population (rabbits) |
| `predator` | Token | 20 | Predator population (foxes) |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `prey_reproduce` | `PreyReproduceed` | - | Prey natural reproduction (birth rate) |
| `predation` | `Predationed` | - | Predator eats prey (predation rate) |
| `predator_death` | `PredatorDeathed` | - | Predator natural death (death rate) |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "prey (100)" as PlacePrey
    state "predator (20)" as PlacePredator


    state "prey_reproduce" as t_TransitionPreyReproduce
    state "predation" as t_TransitionPredation
    state "predator_death" as t_TransitionPredatorDeath


    PlacePrey --> t_TransitionPreyReproduce
    t_TransitionPreyReproduce --> PlacePrey: 2

    PlacePrey --> t_TransitionPredation
    PlacePredator --> t_TransitionPredation
    t_TransitionPredation --> PlacePredator: 2

    PlacePredator --> t_TransitionPredatorDeath

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlacePrey[("prey<br/>initial: 100")]
        PlacePredator[("predator<br/>initial: 20")]
    end

    subgraph Transitions
        t_TransitionPreyReproduce["prey_reproduce"]
        t_TransitionPredation["predation"]
        t_TransitionPredatorDeath["predator_death"]
    end


    PlacePrey --> t_TransitionPreyReproduce
    t_TransitionPreyReproduce -->|2| PlacePrey

    PlacePrey --> t_TransitionPredation
    PlacePredator --> t_TransitionPredation
    t_TransitionPredation -->|2| PlacePredator

    PlacePredator --> t_TransitionPredatorDeath


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `PreyReproduceed` | `prey_reproduce` | `aggregate_id`, `timestamp` |
| `Predationed` | `predation` | `aggregate_id`, `timestamp` |
| `PredatorDeathed` | `predator_death` | `aggregate_id`, `timestamp` |


```mermaid
classDiagram
    class Event {
        +string ID
        +string StreamID
        +string Type
        +int Version
        +time.Time Timestamp
        +json.RawMessage Data
    }


    class PreyReproduceedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PreyReproduceedEvent

    class PredationedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PredationedEvent

    class PredatorDeathedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PredatorDeathedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/predator-prey` | Create new instance |
| GET | `/api/predator-prey/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/prey_reproduce` | `prey_reproduce` | Prey natural reproduction (birth rate) |
| POST | `/api/predation` | `predation` | Predator eats prey (predation rate) |
| POST | `/api/predator_death` | `predator_death` | Predator natural death (death rate) |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/predator-prey \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>"
```

#### Execute Transition
```bash
curl -X POST http://localhost:8080/api/<transition> \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "aggregate_id": "<instance-id>",
    "data": { ... }
  }'
```

#### Response Format
```json
{
  "success": true,
  "aggregate_id": "uuid",
  "version": 1,
  "state": { "place1": 1, "place2": 0 },
  "enabled_transitions": ["transition1", "transition2"]
}
```



## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `./predator-prey.db` | SQLite database path |
| `DEBUG` | `false` | Enable debug endpoints |


## Development

### Project Structure

```
.
├── main.go           # Application entry point
├── workflow.go       # Petri net definition
├── aggregate.go      # Event-sourced aggregate
├── events.go         # Event type definitions
├── api.go            # HTTP handlers
├── debug.go          # Debug handlers
├── frontend/         # Web UI (ES modules)
│   ├── index.html
│   └── src/
│       ├── main.js
│       ├── router.js
│       └── ...
└── go.mod
```

### Testing

```bash
# Run unit tests
go test ./...

# Run with test coverage
go test -cover ./...
```

---

Generated by [petri-pilot](https://github.com/pflow-xyz/petri-pilot)
