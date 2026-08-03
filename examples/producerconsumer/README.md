
# producer-consumer

Bounded-buffer producer-consumer problem demonstrating synchronization via Petri net capacity constraints

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
| `producer_idle` | Token | 1 | Producer is idle, ready to produce |
| `producing` | Token | 0 | Producer is creating an item |
| `buffer` | Token | 0 | Items waiting in the bounded buffer |
| `buffer_space` | Token | 5 | Available slots in the buffer (capacity counter) |
| `consumer_idle` | Token | 1 | Consumer is idle, ready to consume |
| `consuming` | Token | 0 | Consumer is processing an item |
| `consumed` | Token | 0 | Total items consumed (completed) |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `start_produce` | `StartProduceed` | - | Producer begins creating an item (requires buffer space) |
| `finish_produce` | `FinishProduceed` | - | Producer finishes item, places it in buffer |
| `start_consume` | `StartConsumeed` | - | Consumer takes an item from the buffer |
| `finish_consume` | `FinishConsumeed` | - | Consumer finishes processing the item |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "producer_idle (1)" as PlaceProducerIdle
    state "producing" as PlaceProducing
    state "buffer" as PlaceBuffer
    state "buffer_space (5)" as PlaceBufferSpace
    state "consumer_idle (1)" as PlaceConsumerIdle
    state "consuming" as PlaceConsuming
    state "consumed" as PlaceConsumed


    state "start_produce" as t_TransitionStartProduce
    state "finish_produce" as t_TransitionFinishProduce
    state "start_consume" as t_TransitionStartConsume
    state "finish_consume" as t_TransitionFinishConsume


    PlaceProducerIdle --> t_TransitionStartProduce
    PlaceBufferSpace --> t_TransitionStartProduce
    t_TransitionStartProduce --> PlaceProducing

    PlaceProducing --> t_TransitionFinishProduce
    t_TransitionFinishProduce --> PlaceProducerIdle
    t_TransitionFinishProduce --> PlaceBuffer

    PlaceConsumerIdle --> t_TransitionStartConsume
    PlaceBuffer --> t_TransitionStartConsume
    t_TransitionStartConsume --> PlaceConsuming

    PlaceConsuming --> t_TransitionFinishConsume
    t_TransitionFinishConsume --> PlaceConsumerIdle
    t_TransitionFinishConsume --> PlaceBufferSpace
    t_TransitionFinishConsume --> PlaceConsumed

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceProducerIdle[("producer_idle<br/>initial: 1")]
        PlaceProducing[("producing")]
        PlaceBuffer[("buffer")]
        PlaceBufferSpace[("buffer_space<br/>initial: 5")]
        PlaceConsumerIdle[("consumer_idle<br/>initial: 1")]
        PlaceConsuming[("consuming")]
        PlaceConsumed[("consumed")]
    end

    subgraph Transitions
        t_TransitionStartProduce["start_produce"]
        t_TransitionFinishProduce["finish_produce"]
        t_TransitionStartConsume["start_consume"]
        t_TransitionFinishConsume["finish_consume"]
    end


    PlaceProducerIdle --> t_TransitionStartProduce
    PlaceBufferSpace --> t_TransitionStartProduce
    t_TransitionStartProduce --> PlaceProducing

    PlaceProducing --> t_TransitionFinishProduce
    t_TransitionFinishProduce --> PlaceProducerIdle
    t_TransitionFinishProduce --> PlaceBuffer

    PlaceConsumerIdle --> t_TransitionStartConsume
    PlaceBuffer --> t_TransitionStartConsume
    t_TransitionStartConsume --> PlaceConsuming

    PlaceConsuming --> t_TransitionFinishConsume
    t_TransitionFinishConsume --> PlaceConsumerIdle
    t_TransitionFinishConsume --> PlaceBufferSpace
    t_TransitionFinishConsume --> PlaceConsumed


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `StartProduceed` | `start_produce` | `aggregate_id`, `timestamp` |
| `FinishProduceed` | `finish_produce` | `aggregate_id`, `timestamp` |
| `StartConsumeed` | `start_consume` | `aggregate_id`, `timestamp` |
| `FinishConsumeed` | `finish_consume` | `aggregate_id`, `timestamp` |


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


    class StartProduceedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- StartProduceedEvent

    class FinishProduceedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- FinishProduceedEvent

    class StartConsumeedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- StartConsumeedEvent

    class FinishConsumeedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- FinishConsumeedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/producer-consumer` | Create new instance |
| GET | `/api/producer-consumer/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/start_produce` | `start_produce` | Producer begins creating an item (requires buffer space) |
| POST | `/api/finish_produce` | `finish_produce` | Producer finishes item, places it in buffer |
| POST | `/api/start_consume` | `start_consume` | Consumer takes an item from the buffer |
| POST | `/api/finish_consume` | `finish_consume` | Consumer finishes processing the item |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/producer-consumer \
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
| `DB_PATH` | `./producer-consumer.db` | SQLite database path |
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
