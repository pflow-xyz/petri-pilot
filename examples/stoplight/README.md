
# stoplight

Traffic light state machine cycling through red, yellow, and green phases

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
| `red` | Token | 1 | Red light is on |
| `green` | Token | 0 | Green light is on |
| `yellow` | Token | 0 | Yellow light is on |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `go` | `Go` | - | Red to green |
| `slow` | `Slowed` | - | Green to yellow |
| `stop` | `Stoped` | - | Yellow to red |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "red (1)" as PlaceRed
    state "green" as PlaceGreen
    state "yellow" as PlaceYellow


    state "go" as t_TransitionGo
    state "slow" as t_TransitionSlow
    state "stop" as t_TransitionStop


    PlaceRed --> t_TransitionGo
    t_TransitionGo --> PlaceGreen

    PlaceGreen --> t_TransitionSlow
    t_TransitionSlow --> PlaceYellow

    PlaceYellow --> t_TransitionStop
    t_TransitionStop --> PlaceRed

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceRed[("red<br/>initial: 1")]
        PlaceGreen[("green")]
        PlaceYellow[("yellow")]
    end

    subgraph Transitions
        t_TransitionGo["go"]
        t_TransitionSlow["slow"]
        t_TransitionStop["stop"]
    end


    PlaceRed --> t_TransitionGo
    t_TransitionGo --> PlaceGreen

    PlaceGreen --> t_TransitionSlow
    t_TransitionSlow --> PlaceYellow

    PlaceYellow --> t_TransitionStop
    t_TransitionStop --> PlaceRed


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `Go` | `go` | `aggregate_id`, `timestamp` |
| `Slowed` | `slow` | `aggregate_id`, `timestamp` |
| `Stoped` | `stop` | `aggregate_id`, `timestamp` |


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


    class GoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- GoEvent

    class SlowedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- SlowedEvent

    class StopedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- StopedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/stoplight` | Create new instance |
| GET | `/api/stoplight/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/go` | `go` | Red to green |
| POST | `/api/slow` | `slow` | Green to yellow |
| POST | `/api/stop` | `stop` | Yellow to red |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/stoplight \
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
| `DB_PATH` | `./stoplight.db` | SQLite database path |
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
