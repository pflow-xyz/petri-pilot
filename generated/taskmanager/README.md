
# task-manager

Task management workflow with navigation and admin dashboard

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
| `pending` | Token | 1 | - |
| `in_progress` | Token | 0 | - |
| `review` | Token | 0 | - |
| `completed` | Token | 0 | - |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `start` | `Started` | - | Start working on a task |
| `submit` | `Submited` | - | Submit task for review |
| `approve` | `Approveed` | - | Approve completed task |
| `reject` | `Rejected` | - | Reject task and send back |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "pending (1)" as PlacePending
    state "in_progress" as PlaceInProgress
    state "review" as PlaceReview
    state "completed" as PlaceCompleted


    state "start" as t_TransitionStart
    state "submit" as t_TransitionSubmit
    state "approve" as t_TransitionApprove
    state "reject" as t_TransitionReject


    PlacePending --> t_TransitionStart
    t_TransitionStart --> PlaceInProgress

    PlaceInProgress --> t_TransitionSubmit
    t_TransitionSubmit --> PlaceReview

    PlaceReview --> t_TransitionApprove
    t_TransitionApprove --> PlaceCompleted

    PlaceReview --> t_TransitionReject
    t_TransitionReject --> PlacePending

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlacePending[("pending<br/>initial: 1")]
        PlaceInProgress[("in_progress")]
        PlaceReview[("review")]
        PlaceCompleted[("completed")]
    end

    subgraph Transitions
        t_TransitionStart["start"]
        t_TransitionSubmit["submit"]
        t_TransitionApprove["approve"]
        t_TransitionReject["reject"]
    end


    PlacePending --> t_TransitionStart
    t_TransitionStart --> PlaceInProgress

    PlaceInProgress --> t_TransitionSubmit
    t_TransitionSubmit --> PlaceReview

    PlaceReview --> t_TransitionApprove
    t_TransitionApprove --> PlaceCompleted

    PlaceReview --> t_TransitionReject
    t_TransitionReject --> PlacePending


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `Started` | `start` | `aggregate_id`, `timestamp` |
| `Submited` | `submit` | `aggregate_id`, `timestamp` |
| `Approveed` | `approve` | `aggregate_id`, `timestamp` |
| `Rejected` | `reject` | `aggregate_id`, `timestamp` |


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


    class StartedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- StartedEvent

    class SubmitedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- SubmitedEvent

    class ApproveedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- ApproveedEvent

    class RejectedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- RejectedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/task-manager` | Create new instance |
| GET | `/api/task-manager/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/start` | `start` | Start working on a task |
| POST | `/api/submit` | `submit` | Submit task for review |
| POST | `/api/approve` | `approve` | Approve completed task |
| POST | `/api/reject` | `reject` | Reject task and send back |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/task-manager \
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
| `DB_PATH` | `./task-manager.db` | SQLite database path |


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
