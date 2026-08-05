
# counter

Front of house: orders are taken, drinks are made, drinks are served. Knows nothing about ingredients.

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
| `orders_pending` | Token | 0 | - |
| `espresso_ready` | Token | 0 | - |
| `latte_ready` | Token | 0 | - |
| `cappuccino_ready` | Token | 0 | - |
| `orders_complete` | Token | 0 | - |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `order_espresso` | `counter.order_espresso` | - | - |
| `order_latte` | `counter.order_latte` | - | - |
| `order_cappuccino` | `counter.order_cappuccino` | - | - |
| `make_espresso` | `counter.make_espresso` | - | - |
| `make_latte` | `counter.make_latte` | - | - |
| `make_cappuccino` | `counter.make_cappuccino` | - | - |
| `serve_espresso` | `counter.serve_espresso` | - | - |
| `serve_latte` | `counter.serve_latte` | - | - |
| `serve_cappuccino` | `counter.serve_cappuccino` | - | - |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "orders_pending" as PlaceOrdersPending
    state "espresso_ready" as PlaceEspressoReady
    state "latte_ready" as PlaceLatteReady
    state "cappuccino_ready" as PlaceCappuccinoReady
    state "orders_complete" as PlaceOrdersComplete


    state "order_espresso" as t_TransitionOrderEspresso
    state "order_latte" as t_TransitionOrderLatte
    state "order_cappuccino" as t_TransitionOrderCappuccino
    state "make_espresso" as t_TransitionMakeEspresso
    state "make_latte" as t_TransitionMakeLatte
    state "make_cappuccino" as t_TransitionMakeCappuccino
    state "serve_espresso" as t_TransitionServeEspresso
    state "serve_latte" as t_TransitionServeLatte
    state "serve_cappuccino" as t_TransitionServeCappuccino


    t_TransitionOrderEspresso --> PlaceOrdersPending

    t_TransitionOrderLatte --> PlaceOrdersPending

    t_TransitionOrderCappuccino --> PlaceOrdersPending

    PlaceOrdersPending --> t_TransitionMakeEspresso
    t_TransitionMakeEspresso --> PlaceEspressoReady

    PlaceOrdersPending --> t_TransitionMakeLatte
    t_TransitionMakeLatte --> PlaceLatteReady

    PlaceOrdersPending --> t_TransitionMakeCappuccino
    t_TransitionMakeCappuccino --> PlaceCappuccinoReady

    PlaceEspressoReady --> t_TransitionServeEspresso
    t_TransitionServeEspresso --> PlaceOrdersComplete

    PlaceLatteReady --> t_TransitionServeLatte
    t_TransitionServeLatte --> PlaceOrdersComplete

    PlaceCappuccinoReady --> t_TransitionServeCappuccino
    t_TransitionServeCappuccino --> PlaceOrdersComplete

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceOrdersPending[("orders_pending")]
        PlaceEspressoReady[("espresso_ready")]
        PlaceLatteReady[("latte_ready")]
        PlaceCappuccinoReady[("cappuccino_ready")]
        PlaceOrdersComplete[("orders_complete")]
    end

    subgraph Transitions
        t_TransitionOrderEspresso["order_espresso"]
        t_TransitionOrderLatte["order_latte"]
        t_TransitionOrderCappuccino["order_cappuccino"]
        t_TransitionMakeEspresso["make_espresso"]
        t_TransitionMakeLatte["make_latte"]
        t_TransitionMakeCappuccino["make_cappuccino"]
        t_TransitionServeEspresso["serve_espresso"]
        t_TransitionServeLatte["serve_latte"]
        t_TransitionServeCappuccino["serve_cappuccino"]
    end


    t_TransitionOrderEspresso --> PlaceOrdersPending

    t_TransitionOrderLatte --> PlaceOrdersPending

    t_TransitionOrderCappuccino --> PlaceOrdersPending

    PlaceOrdersPending --> t_TransitionMakeEspresso
    t_TransitionMakeEspresso --> PlaceEspressoReady

    PlaceOrdersPending --> t_TransitionMakeLatte
    t_TransitionMakeLatte --> PlaceLatteReady

    PlaceOrdersPending --> t_TransitionMakeCappuccino
    t_TransitionMakeCappuccino --> PlaceCappuccinoReady

    PlaceEspressoReady --> t_TransitionServeEspresso
    t_TransitionServeEspresso --> PlaceOrdersComplete

    PlaceLatteReady --> t_TransitionServeLatte
    t_TransitionServeLatte --> PlaceOrdersComplete

    PlaceCappuccinoReady --> t_TransitionServeCappuccino
    t_TransitionServeCappuccino --> PlaceOrdersComplete


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `counter.order_espresso` | `order_espresso` | `aggregate_id`, `timestamp` |
| `counter.order_latte` | `order_latte` | `aggregate_id`, `timestamp` |
| `counter.order_cappuccino` | `order_cappuccino` | `aggregate_id`, `timestamp` |
| `counter.make_espresso` | `make_espresso` | `aggregate_id`, `timestamp` |
| `counter.make_latte` | `make_latte` | `aggregate_id`, `timestamp` |
| `counter.make_cappuccino` | `make_cappuccino` | `aggregate_id`, `timestamp` |
| `counter.serve_espresso` | `serve_espresso` | `aggregate_id`, `timestamp` |
| `counter.serve_latte` | `serve_latte` | `aggregate_id`, `timestamp` |
| `counter.serve_cappuccino` | `serve_cappuccino` | `aggregate_id`, `timestamp` |


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


    class CounterOrderEspressoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterOrderEspressoEvent

    class CounterOrderLatteEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterOrderLatteEvent

    class CounterOrderCappuccinoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterOrderCappuccinoEvent

    class CounterMakeEspressoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterMakeEspressoEvent

    class CounterMakeLatteEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterMakeLatteEvent

    class CounterMakeCappuccinoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterMakeCappuccinoEvent

    class CounterServeEspressoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterServeEspressoEvent

    class CounterServeLatteEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterServeLatteEvent

    class CounterServeCappuccinoEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CounterServeCappuccinoEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/counter` | Create new instance |
| GET | `/api/counter/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/order_espresso` | `order_espresso` | - |
| POST | `/api/order_latte` | `order_latte` | - |
| POST | `/api/order_cappuccino` | `order_cappuccino` | - |
| POST | `/api/make_espresso` | `make_espresso` | - |
| POST | `/api/make_latte` | `make_latte` | - |
| POST | `/api/make_cappuccino` | `make_cappuccino` | - |
| POST | `/api/serve_espresso` | `serve_espresso` | - |
| POST | `/api/serve_latte` | `serve_latte` | - |
| POST | `/api/serve_cappuccino` | `serve_cappuccino` | - |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/counter \
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
| `DB_PATH` | `./counter.db` | SQLite database path |


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
