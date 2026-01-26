
# erc20-token

ERC-20 fungible token implementation using Petri net semantics

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
| `total_supply` | Data | 0 | Total tokens in circulation |
| `balances` | Data | 0 | Token balance per address |
| `allowances` | Data | 0 | Spending allowances: owner -> spender -> amount (nested map) |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `transfer` | `transfer_event` | `balances[from] >= amount` | Transfer tokens from sender to recipient |
| `approve` | `approval_event` | - | Approve spender to transfer tokens on owner's behalf |
| `transfer_from` | `transfer_from_event` | `balances[from] >= amount && allowances[from][caller] >= amount` | Transfer tokens using allowance (delegated transfer) |
| `mint` | `mint_event` | - | Create new tokens and add to recipient balance |
| `burn` | `burn_event` | `balances[from] >= amount` | Destroy tokens from holder's balance |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "total_supply" as PlaceTotalSupply
    state "balances" as PlaceBalances
    state "allowances" as PlaceAllowances


    state "transfer" as t_TransitionTransfer
    state "approve" as t_TransitionApprove
    state "transfer_from" as t_TransitionTransferFrom
    state "mint" as t_TransitionMint
    state "burn" as t_TransitionBurn







```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceTotalSupply[("total_supply")]
        PlaceBalances[("balances")]
        PlaceAllowances[("allowances")]
    end

    subgraph Transitions
        t_TransitionTransfer["transfer"]
        t_TransitionApprove["approve"]
        t_TransitionTransferFrom["transfer_from"]
        t_TransitionMint["mint"]
        t_TransitionBurn["burn"]
    end








    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `TransferEvent` | `transfer` | `aggregate_id`, `timestamp`, `from`, `to`, `amount` |
| `ApprovalEvent` | `approve` | `aggregate_id`, `timestamp`, `owner`, `spender`, `amount` |
| `TransferFromEvent` | `transfer_from` | `aggregate_id`, `timestamp`, `from`, `to`, `caller`, `amount` |
| `MintEvent` | `mint` | `aggregate_id`, `timestamp`, `to`, `amount` |
| `BurnEvent` | `burn` | `aggregate_id`, `timestamp`, `from`, `amount` |


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


    class TransferEvent {
        +string AggregateId
        +time.Time Timestamp
        +string From
        +string To
        +int Amount
    }
    Event <|-- TransferEvent

    class ApprovalEvent {
        +string AggregateId
        +time.Time Timestamp
        +string Owner
        +string Spender
        +int Amount
    }
    Event <|-- ApprovalEvent

    class TransferFromEvent {
        +string AggregateId
        +time.Time Timestamp
        +string From
        +string To
        +string Caller
        +int Amount
    }
    Event <|-- TransferFromEvent

    class MintEvent {
        +string AggregateId
        +time.Time Timestamp
        +string To
        +int Amount
    }
    Event <|-- MintEvent

    class BurnEvent {
        +string AggregateId
        +time.Time Timestamp
        +string From
        +int Amount
    }
    Event <|-- BurnEvent

```



## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/erc20-token` | Create new instance |
| GET | `/api/erc20-token/{id}` | Get instance state |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/transfer` | `transfer` | Transfer tokens from sender to recipient |
| POST | `/api/approve` | `approve` | Approve spender to transfer tokens on owner's behalf |
| POST | `/api/transfer_from` | `transfer_from` | Transfer tokens using allowance (delegated transfer) |
| POST | `/api/mint` | `mint` | Create new tokens and add to recipient balance |
| POST | `/api/burn` | `burn` | Destroy tokens from holder's balance |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/erc20-token \
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
| `DB_PATH` | `./erc20-token.db` | SQLite database path |


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
