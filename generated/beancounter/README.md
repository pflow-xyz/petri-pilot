
# coffeeshop

Coffee shop inventory and order processing with resource prediction

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
| `coffee_beans` | Token | 1000 | Coffee beans inventory (grams) |
| `milk` | Token | 500 | Milk inventory (ml) |
| `cups` | Token | 200 | Cup inventory |
| `orders_pending` | Token | 0 | Orders waiting to be made |
| `espresso_ready` | Token | 0 | Espresso drinks ready |
| `latte_ready` | Token | 0 | Latte drinks ready |
| `cappuccino_ready` | Token | 0 | Cappuccino drinks ready |
| `orders_complete` | Token | 0 | Completed and served orders |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `order_espresso` | `OrderEspressoed` | - | Customer orders espresso |
| `order_latte` | `OrderLatteed` | - | Customer orders latte |
| `order_cappuccino` | `OrderCappuccinoed` | - | Customer orders cappuccino |
| `make_espresso` | `MakeEspressoed` | - | Barista makes espresso |
| `make_latte` | `MakeLatteed` | - | Barista makes latte |
| `make_cappuccino` | `MakeCappuccinoed` | - | Barista makes cappuccino |
| `serve_espresso` | `ServeEspressoed` | - | Serve espresso to customer |
| `serve_latte` | `ServeLatteed` | - | Serve latte to customer |
| `serve_cappuccino` | `ServeCappuccinoed` | - | Serve cappuccino to customer |
| `restock_coffee_beans` | `RestockCoffeeBeansed` | - | Restock coffee beans inventory |
| `restock_milk` | `RestockMilked` | - | Restock milk inventory |
| `restock_cups` | `RestockCupsed` | - | Restock cup inventory |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "coffee_beans (1000)" as PlaceCoffeeBeans
    state "milk (500)" as PlaceMilk
    state "cups (200)" as PlaceCups
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
    state "restock_coffee_beans" as t_TransitionRestockCoffeeBeans
    state "restock_milk" as t_TransitionRestockMilk
    state "restock_cups" as t_TransitionRestockCups


    t_TransitionOrderEspresso --> PlaceOrdersPending

    t_TransitionOrderLatte --> PlaceOrdersPending

    t_TransitionOrderCappuccino --> PlaceOrdersPending

    PlaceOrdersPending --> t_TransitionMakeEspresso
    PlaceCoffeeBeans --> t_TransitionMakeEspresso: 20
    PlaceCups --> t_TransitionMakeEspresso
    t_TransitionMakeEspresso --> PlaceEspressoReady

    PlaceOrdersPending --> t_TransitionMakeLatte
    PlaceCoffeeBeans --> t_TransitionMakeLatte: 15
    PlaceMilk --> t_TransitionMakeLatte: 50
    PlaceCups --> t_TransitionMakeLatte
    t_TransitionMakeLatte --> PlaceLatteReady

    PlaceOrdersPending --> t_TransitionMakeCappuccino
    PlaceCoffeeBeans --> t_TransitionMakeCappuccino: 15
    PlaceMilk --> t_TransitionMakeCappuccino: 30
    PlaceCups --> t_TransitionMakeCappuccino
    t_TransitionMakeCappuccino --> PlaceCappuccinoReady

    PlaceEspressoReady --> t_TransitionServeEspresso
    t_TransitionServeEspresso --> PlaceOrdersComplete

    PlaceLatteReady --> t_TransitionServeLatte
    t_TransitionServeLatte --> PlaceOrdersComplete

    PlaceCappuccinoReady --> t_TransitionServeCappuccino
    t_TransitionServeCappuccino --> PlaceOrdersComplete

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
        t_TransitionRestockCoffeeBeans["restock_coffee_beans"]
        t_TransitionRestockMilk["restock_milk"]
        t_TransitionRestockCups["restock_cups"]
    end


    t_TransitionOrderEspresso --> PlaceOrdersPending

    t_TransitionOrderLatte --> PlaceOrdersPending

    t_TransitionOrderCappuccino --> PlaceOrdersPending

    PlaceOrdersPending --> t_TransitionMakeEspresso
    PlaceCoffeeBeans -->|20| t_TransitionMakeEspresso
    PlaceCups --> t_TransitionMakeEspresso
    t_TransitionMakeEspresso --> PlaceEspressoReady

    PlaceOrdersPending --> t_TransitionMakeLatte
    PlaceCoffeeBeans -->|15| t_TransitionMakeLatte
    PlaceMilk -->|50| t_TransitionMakeLatte
    PlaceCups --> t_TransitionMakeLatte
    t_TransitionMakeLatte --> PlaceLatteReady

    PlaceOrdersPending --> t_TransitionMakeCappuccino
    PlaceCoffeeBeans -->|15| t_TransitionMakeCappuccino
    PlaceMilk -->|30| t_TransitionMakeCappuccino
    PlaceCups --> t_TransitionMakeCappuccino
    t_TransitionMakeCappuccino --> PlaceCappuccinoReady

    PlaceEspressoReady --> t_TransitionServeEspresso
    t_TransitionServeEspresso --> PlaceOrdersComplete

    PlaceLatteReady --> t_TransitionServeLatte
    t_TransitionServeLatte --> PlaceOrdersComplete

    PlaceCappuccinoReady --> t_TransitionServeCappuccino
    t_TransitionServeCappuccino --> PlaceOrdersComplete

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
| `OrderEspressoed` | `order_espresso` | `aggregate_id`, `timestamp` |
| `OrderLatteed` | `order_latte` | `aggregate_id`, `timestamp` |
| `OrderCappuccinoed` | `order_cappuccino` | `aggregate_id`, `timestamp` |
| `MakeEspressoed` | `make_espresso` | `aggregate_id`, `timestamp` |
| `MakeLatteed` | `make_latte` | `aggregate_id`, `timestamp` |
| `MakeCappuccinoed` | `make_cappuccino` | `aggregate_id`, `timestamp` |
| `ServeEspressoed` | `serve_espresso` | `aggregate_id`, `timestamp` |
| `ServeLatteed` | `serve_latte` | `aggregate_id`, `timestamp` |
| `ServeCappuccinoed` | `serve_cappuccino` | `aggregate_id`, `timestamp` |
| `RestockCoffeeBeansed` | `restock_coffee_beans` | `aggregate_id`, `timestamp` |
| `RestockMilked` | `restock_milk` | `aggregate_id`, `timestamp` |
| `RestockCupsed` | `restock_cups` | `aggregate_id`, `timestamp` |


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


    class OrderEspressoedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- OrderEspressoedEvent

    class OrderLatteedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- OrderLatteedEvent

    class OrderCappuccinoedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- OrderCappuccinoedEvent

    class MakeEspressoedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- MakeEspressoedEvent

    class MakeLatteedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- MakeLatteedEvent

    class MakeCappuccinoedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- MakeCappuccinoedEvent

    class ServeEspressoedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- ServeEspressoedEvent

    class ServeLatteedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- ServeLatteedEvent

    class ServeCappuccinoedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- ServeCappuccinoedEvent

    class RestockCoffeeBeansedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- RestockCoffeeBeansedEvent

    class RestockMilkedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- RestockMilkedEvent

    class RestockCupsedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- RestockCupsedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/coffeeshop` | Create new instance |
| GET | `/api/coffeeshop/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/order_espresso` | `order_espresso` | Customer orders espresso |
| POST | `/api/order_latte` | `order_latte` | Customer orders latte |
| POST | `/api/order_cappuccino` | `order_cappuccino` | Customer orders cappuccino |
| POST | `/api/make_espresso` | `make_espresso` | Barista makes espresso |
| POST | `/api/make_latte` | `make_latte` | Barista makes latte |
| POST | `/api/make_cappuccino` | `make_cappuccino` | Barista makes cappuccino |
| POST | `/api/serve_espresso` | `serve_espresso` | Serve espresso to customer |
| POST | `/api/serve_latte` | `serve_latte` | Serve latte to customer |
| POST | `/api/serve_cappuccino` | `serve_cappuccino` | Serve cappuccino to customer |
| POST | `/api/restock_coffee_beans` | `restock_coffee_beans` | Restock coffee beans inventory |
| POST | `/api/restock_milk` | `restock_milk` | Restock milk inventory |
| POST | `/api/restock_cups` | `restock_cups` | Restock cup inventory |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/coffeeshop \
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
| `DB_PATH` | `./coffeeshop.db` | SQLite database path |
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
