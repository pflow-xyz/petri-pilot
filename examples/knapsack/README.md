
# knapsack

0/1 Knapsack problem modeled as a Petri net with ODE-based optimization

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
| `item0` | Token | 1 | Item 0 available (weight=2, value=10, efficiency=5.0) |
| `item1` | Token | 1 | Item 1 available (weight=5, value=15, efficiency=3.0) |
| `item2` | Token | 1 | Item 2 available (weight=6, value=12, efficiency=2.0) |
| `item3` | Token | 1 | Item 3 available (weight=8, value=13, efficiency=1.625) |
| `capacity` | Token | 15 | Remaining knapsack capacity (weight budget) |
| `item0_taken` | Token | 0 | Item 0 has been taken |
| `item1_taken` | Token | 0 | Item 1 has been taken |
| `item2_taken` | Token | 0 | Item 2 has been taken |
| `item3_taken` | Token | 0 | Item 3 has been taken |
| `total_value` | Token | 0 | Accumulated value from taken items |
| `total_weight` | Token | 0 | Total weight used |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `take_item0` | `TakeItem0ed` | - | Take item 0 (weight=2, value=10) |
| `take_item1` | `TakeItem1ed` | - | Take item 1 (weight=5, value=15) |
| `take_item2` | `TakeItem2ed` | - | Take item 2 (weight=6, value=12) |
| `take_item3` | `TakeItem3ed` | - | Take item 3 (weight=8, value=13) |
| `reset` | `Reseted` | - | Reset knapsack to initial state |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "item0 (1)" as PlaceItem0
    state "item1 (1)" as PlaceItem1
    state "item2 (1)" as PlaceItem2
    state "item3 (1)" as PlaceItem3
    state "capacity (15)" as PlaceCapacity
    state "item0_taken" as PlaceItem0Taken
    state "item1_taken" as PlaceItem1Taken
    state "item2_taken" as PlaceItem2Taken
    state "item3_taken" as PlaceItem3Taken
    state "total_value" as PlaceTotalValue
    state "total_weight" as PlaceTotalWeight


    state "take_item0" as t_TransitionTakeItem0
    state "take_item1" as t_TransitionTakeItem1
    state "take_item2" as t_TransitionTakeItem2
    state "take_item3" as t_TransitionTakeItem3
    state "reset" as t_TransitionReset


    PlaceItem0 --> t_TransitionTakeItem0
    PlaceCapacity --> t_TransitionTakeItem0: 2
    t_TransitionTakeItem0 --> PlaceItem0Taken
    t_TransitionTakeItem0 --> PlaceTotalValue: 10
    t_TransitionTakeItem0 --> PlaceTotalWeight: 2

    PlaceItem1 --> t_TransitionTakeItem1
    PlaceCapacity --> t_TransitionTakeItem1: 5
    t_TransitionTakeItem1 --> PlaceItem1Taken
    t_TransitionTakeItem1 --> PlaceTotalValue: 15
    t_TransitionTakeItem1 --> PlaceTotalWeight: 5

    PlaceItem2 --> t_TransitionTakeItem2
    PlaceCapacity --> t_TransitionTakeItem2: 6
    t_TransitionTakeItem2 --> PlaceItem2Taken
    t_TransitionTakeItem2 --> PlaceTotalValue: 12
    t_TransitionTakeItem2 --> PlaceTotalWeight: 6

    PlaceItem3 --> t_TransitionTakeItem3
    PlaceCapacity --> t_TransitionTakeItem3: 8
    t_TransitionTakeItem3 --> PlaceItem3Taken
    t_TransitionTakeItem3 --> PlaceTotalValue: 13
    t_TransitionTakeItem3 --> PlaceTotalWeight: 8

    PlaceItem0Taken --> t_TransitionReset
    PlaceItem1Taken --> t_TransitionReset
    PlaceItem2Taken --> t_TransitionReset
    PlaceItem3Taken --> t_TransitionReset
    PlaceTotalValue --> t_TransitionReset: 50
    PlaceTotalWeight --> t_TransitionReset: 15
    t_TransitionReset --> PlaceItem0
    t_TransitionReset --> PlaceItem1
    t_TransitionReset --> PlaceItem2
    t_TransitionReset --> PlaceItem3
    t_TransitionReset --> PlaceCapacity: 15

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceItem0[("item0<br/>initial: 1")]
        PlaceItem1[("item1<br/>initial: 1")]
        PlaceItem2[("item2<br/>initial: 1")]
        PlaceItem3[("item3<br/>initial: 1")]
        PlaceCapacity[("capacity<br/>initial: 15")]
        PlaceItem0Taken[("item0_taken")]
        PlaceItem1Taken[("item1_taken")]
        PlaceItem2Taken[("item2_taken")]
        PlaceItem3Taken[("item3_taken")]
        PlaceTotalValue[("total_value")]
        PlaceTotalWeight[("total_weight")]
    end

    subgraph Transitions
        t_TransitionTakeItem0["take_item0"]
        t_TransitionTakeItem1["take_item1"]
        t_TransitionTakeItem2["take_item2"]
        t_TransitionTakeItem3["take_item3"]
        t_TransitionReset["reset"]
    end


    PlaceItem0 --> t_TransitionTakeItem0
    PlaceCapacity -->|2| t_TransitionTakeItem0
    t_TransitionTakeItem0 --> PlaceItem0Taken
    t_TransitionTakeItem0 -->|10| PlaceTotalValue
    t_TransitionTakeItem0 -->|2| PlaceTotalWeight

    PlaceItem1 --> t_TransitionTakeItem1
    PlaceCapacity -->|5| t_TransitionTakeItem1
    t_TransitionTakeItem1 --> PlaceItem1Taken
    t_TransitionTakeItem1 -->|15| PlaceTotalValue
    t_TransitionTakeItem1 -->|5| PlaceTotalWeight

    PlaceItem2 --> t_TransitionTakeItem2
    PlaceCapacity -->|6| t_TransitionTakeItem2
    t_TransitionTakeItem2 --> PlaceItem2Taken
    t_TransitionTakeItem2 -->|12| PlaceTotalValue
    t_TransitionTakeItem2 -->|6| PlaceTotalWeight

    PlaceItem3 --> t_TransitionTakeItem3
    PlaceCapacity -->|8| t_TransitionTakeItem3
    t_TransitionTakeItem3 --> PlaceItem3Taken
    t_TransitionTakeItem3 -->|13| PlaceTotalValue
    t_TransitionTakeItem3 -->|8| PlaceTotalWeight

    PlaceItem0Taken --> t_TransitionReset
    PlaceItem1Taken --> t_TransitionReset
    PlaceItem2Taken --> t_TransitionReset
    PlaceItem3Taken --> t_TransitionReset
    PlaceTotalValue -->|50| t_TransitionReset
    PlaceTotalWeight -->|15| t_TransitionReset
    t_TransitionReset --> PlaceItem0
    t_TransitionReset --> PlaceItem1
    t_TransitionReset --> PlaceItem2
    t_TransitionReset --> PlaceItem3
    t_TransitionReset -->|15| PlaceCapacity


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `TakeItem0ed` | `take_item0` | `aggregate_id`, `timestamp` |
| `TakeItem1ed` | `take_item1` | `aggregate_id`, `timestamp` |
| `TakeItem2ed` | `take_item2` | `aggregate_id`, `timestamp` |
| `TakeItem3ed` | `take_item3` | `aggregate_id`, `timestamp` |
| `Reseted` | `reset` | `aggregate_id`, `timestamp` |


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


    class TakeItem0edEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- TakeItem0edEvent

    class TakeItem1edEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- TakeItem1edEvent

    class TakeItem2edEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- TakeItem2edEvent

    class TakeItem3edEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- TakeItem3edEvent

    class ResetedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- ResetedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/knapsack` | Create new instance |
| GET | `/api/knapsack/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/take_item0` | `take_item0` | Take item 0 (weight=2, value=10) |
| POST | `/api/take_item1` | `take_item1` | Take item 1 (weight=5, value=15) |
| POST | `/api/take_item2` | `take_item2` | Take item 2 (weight=6, value=12) |
| POST | `/api/take_item3` | `take_item3` | Take item 3 (weight=8, value=13) |
| POST | `/api/reset` | `reset` | Reset knapsack to initial state |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/knapsack \
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
| `DB_PATH` | `./knapsack.db` | SQLite database path |
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
