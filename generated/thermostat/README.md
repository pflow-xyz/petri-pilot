
# thermostat

Bang-bang thermostat controller with ODE dynamics showing temperature regulation around a setpoint

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
| `temperature` | Token | 15 | Current room temperature (token count = degrees) |
| `heater_on` | Token | 0 | Heater is active (heating) |
| `heater_off` | Token | 1 | Heater is inactive (cooling only) |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `heat` | `Heated` | - | Heater warms the room (active when heater is on) |
| `cool` | `Cooled` | - | Room naturally loses heat to environment |
| `turn_on` | `TurnOned` | - | Turn heater on when temperature is below target |
| `turn_off` | `TurnOffed` | - | Turn heater off when temperature reaches target |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "temperature (15)" as PlaceTemperature
    state "heater_on" as PlaceHeaterOn
    state "heater_off (1)" as PlaceHeaterOff


    state "heat" as t_TransitionHeat
    state "cool" as t_TransitionCool
    state "turn_on" as t_TransitionTurnOn
    state "turn_off" as t_TransitionTurnOff


    PlaceHeaterOn --> t_TransitionHeat
    t_TransitionHeat --> PlaceHeaterOn
    t_TransitionHeat --> PlaceTemperature

    PlaceTemperature --> t_TransitionCool

    PlaceHeaterOff --> t_TransitionTurnOn
    t_TransitionTurnOn --> PlaceHeaterOn

    PlaceHeaterOn --> t_TransitionTurnOff
    t_TransitionTurnOff --> PlaceHeaterOff

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceTemperature[("temperature<br/>initial: 15")]
        PlaceHeaterOn[("heater_on")]
        PlaceHeaterOff[("heater_off<br/>initial: 1")]
    end

    subgraph Transitions
        t_TransitionHeat["heat"]
        t_TransitionCool["cool"]
        t_TransitionTurnOn["turn_on"]
        t_TransitionTurnOff["turn_off"]
    end


    PlaceHeaterOn --> t_TransitionHeat
    t_TransitionHeat --> PlaceHeaterOn
    t_TransitionHeat --> PlaceTemperature

    PlaceTemperature --> t_TransitionCool

    PlaceHeaterOff --> t_TransitionTurnOn
    t_TransitionTurnOn --> PlaceHeaterOn

    PlaceHeaterOn --> t_TransitionTurnOff
    t_TransitionTurnOff --> PlaceHeaterOff


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `Heated` | `heat` | `aggregate_id`, `timestamp` |
| `Cooled` | `cool` | `aggregate_id`, `timestamp` |
| `TurnOned` | `turn_on` | `aggregate_id`, `timestamp` |
| `TurnOffed` | `turn_off` | `aggregate_id`, `timestamp` |


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


    class HeatedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- HeatedEvent

    class CooledEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- CooledEvent

    class TurnOnedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- TurnOnedEvent

    class TurnOffedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- TurnOffedEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/thermostat` | Create new instance |
| GET | `/api/thermostat/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/heat` | `heat` | Heater warms the room (active when heater is on) |
| POST | `/api/cool` | `cool` | Room naturally loses heat to environment |
| POST | `/api/turn_on` | `turn_on` | Turn heater on when temperature is below target |
| POST | `/api/turn_off` | `turn_off` | Turn heater off when temperature reaches target |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/thermostat \
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
| `DB_PATH` | `./thermostat.db` | SQLite database path |
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
