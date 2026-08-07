
# pantry

Stock room: holds ingredients and draws them down. Knows nothing about orders or staffing — a brew transition only says what a drink costs. Fused with the counter's start_X, so ingredients are committed when the brew begins rather than when it finishes: a drink half-made has already cost its beans.

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
| `coffee_beans` | Token | 1000 | - |
| `milk` | Token | 500 | - |
| `cups` | Token | 200 | - |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `brew_espresso` | `pantry.brew_espresso` | - | - |
| `brew_latte` | `pantry.brew_latte` | - | - |
| `brew_cappuccino` | `pantry.brew_cappuccino` | - | - |
| `restock_coffee_beans` | `pantry.restock_coffee_beans` | - | - |
| `restock_milk` | `pantry.restock_milk` | - | - |
| `restock_cups` | `pantry.restock_cups` | - | - |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "coffee_beans (1000)" as PlaceCoffeeBeans
    state "milk (500)" as PlaceMilk
    state "cups (200)" as PlaceCups


    state "brew_espresso" as t_TransitionBrewEspresso
    state "brew_latte" as t_TransitionBrewLatte
    state "brew_cappuccino" as t_TransitionBrewCappuccino
    state "restock_coffee_beans" as t_TransitionRestockCoffeeBeans
    state "restock_milk" as t_TransitionRestockMilk
    state "restock_cups" as t_TransitionRestockCups


    PlaceCoffeeBeans --> t_TransitionBrewEspresso: 20
    PlaceCups --> t_TransitionBrewEspresso

    PlaceCoffeeBeans --> t_TransitionBrewLatte: 15
    PlaceMilk --> t_TransitionBrewLatte: 50
    PlaceCups --> t_TransitionBrewLatte

    PlaceCoffeeBeans --> t_TransitionBrewCappuccino: 15
    PlaceMilk --> t_TransitionBrewCappuccino: 30
    PlaceCups --> t_TransitionBrewCappuccino

    t_TransitionRestockCoffeeBeans --> PlaceCoffeeBeans: 500

    t_TransitionRestockMilk --> PlaceMilk: 500

    t_TransitionRestockCups --> PlaceCups: 100

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceCoffeeBeans[("coffee_beans<br/>initial: 1000")]
        PlaceMilk[("milk<br/>initial: 500")]
        PlaceCups[("cups<br/>initial: 200")]
    end

    subgraph Transitions
        t_TransitionBrewEspresso["brew_espresso"]
        t_TransitionBrewLatte["brew_latte"]
        t_TransitionBrewCappuccino["brew_cappuccino"]
        t_TransitionRestockCoffeeBeans["restock_coffee_beans"]
        t_TransitionRestockMilk["restock_milk"]
        t_TransitionRestockCups["restock_cups"]
    end


    PlaceCoffeeBeans -->|20| t_TransitionBrewEspresso
    PlaceCups --> t_TransitionBrewEspresso

    PlaceCoffeeBeans -->|15| t_TransitionBrewLatte
    PlaceMilk -->|50| t_TransitionBrewLatte
    PlaceCups --> t_TransitionBrewLatte

    PlaceCoffeeBeans -->|15| t_TransitionBrewCappuccino
    PlaceMilk -->|30| t_TransitionBrewCappuccino
    PlaceCups --> t_TransitionBrewCappuccino

    t_TransitionRestockCoffeeBeans -->|500| PlaceCoffeeBeans

    t_TransitionRestockMilk -->|500| PlaceMilk

    t_TransitionRestockCups -->|100| PlaceCups


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `pantry.brew_espresso` | `brew_espresso` | `aggregate_id`, `timestamp` |
| `pantry.brew_latte` | `brew_latte` | `aggregate_id`, `timestamp` |
| `pantry.brew_cappuccino` | `brew_cappuccino` | `aggregate_id`, `timestamp` |
| `pantry.restock_coffee_beans` | `restock_coffee_beans` | `aggregate_id`, `timestamp` |
| `pantry.restock_milk` | `restock_milk` | `aggregate_id`, `timestamp` |
| `pantry.restock_cups` | `restock_cups` | `aggregate_id`, `timestamp` |


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


    class PantryBrewEspressoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PantryBrewEspressoEvent

    class PantryBrewLatteEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PantryBrewLatteEvent

    class PantryBrewCappuccinoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PantryBrewCappuccinoEvent

    class PantryRestockCoffeeBeansEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PantryRestockCoffeeBeansEvent

    class PantryRestockMilkEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PantryRestockMilkEvent

    class PantryRestockCupsEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PantryRestockCupsEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/pantry` | Create new instance |
| GET | `/api/pantry/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/brew_espresso` | `brew_espresso` | - |
| POST | `/api/brew_latte` | `brew_latte` | - |
| POST | `/api/brew_cappuccino` | `brew_cappuccino` | - |
| POST | `/api/restock_coffee_beans` | `restock_coffee_beans` | - |
| POST | `/api/restock_milk` | `restock_milk` | - |
| POST | `/api/restock_cups` | `restock_cups` | - |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/pantry \
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
| `DB_PATH` | `./pantry.db` | SQLite database path |


## Development

### Project Structure

```
.
├── main.go           # Application entry point
├── workflow.go       # Petri net definition
├── aggregate.go      # Event-sourced aggregate
├── events.go         # Event type definitions
├── api.go            # HTTP handlers
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
