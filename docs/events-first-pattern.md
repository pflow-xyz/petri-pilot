# Events First Pattern

The Events First pattern separates complete business records from operational computation data, enabling clean event sourcing while maintaining efficient state management.

## Overview

In Petri-pilot, the schema distinguishes between:

1. **Events** - Complete data contract for audit, replay, and business records
2. **Bindings** - Operational data subset for guards and arc transformations

This separation allows:
- Complete audit trails via event sourcing
- Efficient state computation with minimal data
- Clear separation of concerns between recording and computing
- Flexible evolution of both independently

## Events: The Complete Record

Events define the full business record shape. Every transition execution emits an event with:

### Event Schema

```json
{
  "events": [
    {
      "id": "order_validated",
      "name": "Order Validated",
      "description": "Order has passed validation checks",
      "fields": [
        {"name": "order_id", "type": "string", "required": true, "description": "Unique order identifier"},
        {"name": "customer_name", "type": "string", "required": true},
        {"name": "customer_email", "type": "string"},
        {"name": "shipping_address", "type": "string"},
        {"name": "total", "type": "number", "required": true},
        {"name": "status", "type": "string"},
        {"name": "created_at", "type": "time"}
      ]
    }
  ]
}
```

### Event Characteristics

- **Complete**: Contains all business-relevant information
- **Immutable**: Once emitted, never modified
- **Auditable**: Full context for compliance and debugging
- **Replayable**: Can reconstruct state from event history

### System Fields

Events automatically include system-populated fields:
- `aggregate_id` - The workflow instance identifier
- `timestamp` - When the event occurred
- `version` - Event sequence number

These fields are added by the runtime and don't need to be declared in the schema.

## Bindings: Operational Data

Bindings extract only the data needed for state computation. They define:

### Binding Schema

```json
{
  "transitions": [
    {
      "id": "validate",
      "description": "Check order validity",
      "event": "order_validated",
      "bindings": [
        {"name": "order_id", "type": "string"},
        {"name": "customer_name", "type": "string"},
        {"name": "customer_email", "type": "string"},
        {"name": "shipping_address", "type": "string"},
        {"name": "total", "type": "number", "value": true}
      ]
    }
  ]
}
```

### Binding Characteristics

- **Minimal**: Only data needed for this transition
- **Typed**: Type annotations for validation
- **Purposeful**: Each binding has a specific computational role

### Binding Attributes

#### `name` and `type`

Every binding declares its name and type:

```json
{"name": "order_id", "type": "string"}
{"name": "total", "type": "number"}
{"name": "is_priority", "type": "bool"}
```

Supported types: `string`, `number`, `int64`, `bool`, `time`, and map types.

#### `value: true` - Transfer to State

When `"value": true`, the binding transfers its value to a data place:

```json
{
  "bindings": [
    {"name": "amount", "type": "number", "value": true}
  ]
}
```

This is used with **data places** to update state values during transitions.

#### `keys: [...]` - Map Lookups (Arcnet Pattern)

For map-based state (the "arcnet" pattern), bindings can specify keys:

```json
{
  "bindings": [
    {"name": "from", "type": "string", "keys": ["from"]},
    {"name": "to", "type": "string", "keys": ["to"]},
    {"name": "amount", "type": "number", "value": true}
  ]
}
```

This enables lookups like `balances[from]` in guards and arc expressions.

## Relationship: Events vs Bindings

### What Goes in Events

Events should contain:
- All business-relevant data for the domain action
- Context needed for audit and compliance
- Optional fields that might be added later
- Human-readable descriptions and metadata

**Example**: An order validation event includes customer details, order items, pricing, shipping info, and status.

### What Goes in Bindings

Bindings should contain:
- Data used in guard expressions
- Values transferred to data places
- Keys for map lookups
- Data needed for arc weight calculations

**Example**: The validate transition only needs `order_id`, `customer_name`, and `total` for its guard and state update.

### Mapping Events to Bindings

Each transition specifies its event type and extracts bindings:

```json
{
  "id": "transfer",
  "event": "tokens_transferred",
  "guard": "balances[from] >= amount && amount > 0",
  "bindings": [
    {"name": "from", "type": "string", "keys": ["from"]},
    {"name": "to", "type": "string", "keys": ["to"]},
    {"name": "amount", "type": "number", "value": true}
  ]
}
```

The `tokens_transferred` event contains complete transfer details (timestamp, transaction ID, memo, etc.), but the binding only extracts `from`, `to`, and `amount` for state computation.

## Binding Patterns

### Simple Value Transfer

Transfer a single value to state:

```json
{
  "bindings": [
    {"name": "status", "type": "string", "value": true}
  ]
}
```

### Map Key Lookup

Use a field as a map key:

```json
{
  "bindings": [
    {"name": "user_id", "type": "string", "keys": ["user_id"]},
    {"name": "balance", "type": "number", "value": true}
  ]
}
```

Enables guards like: `balances[user_id] >= amount`

### Multiple Keys (Nested Maps)

For nested map structures:

```json
{
  "bindings": [
    {"name": "owner", "type": "string", "keys": ["owner"]},
    {"name": "spender", "type": "string", "keys": ["owner", "spender"]},
    {"name": "amount", "type": "number", "value": true}
  ]
}
```

Enables: `allowances[owner][spender] >= amount`

### Computation Without Transfer

Some bindings are used only in guards, not transferred:

```json
{
  "bindings": [
    {"name": "requester_id", "type": "string"}
  ],
  "guard": "requester_id == owner_id"
}
```

No `"value": true` means the binding is available for guards but not written to state.

## Complete Example: Token Ledger

Here's a complete example showing Events First with bindings:

```json
{
  "name": "erc20-token",
  "events": [
    {
      "id": "tokens_transferred",
      "name": "Tokens Transferred",
      "description": "Tokens moved from one address to another",
      "fields": [
        {"name": "from", "type": "string", "required": true},
        {"name": "to", "type": "string", "required": true},
        {"name": "amount", "type": "number", "required": true},
        {"name": "memo", "type": "string"},
        {"name": "transaction_id", "type": "string"},
        {"name": "timestamp", "type": "time"}
      ]
    }
  ],
  "places": [
    {
      "id": "balances",
      "kind": "data",
      "type": "map[string]int64",
      "description": "Token balances by address"
    }
  ],
  "transitions": [
    {
      "id": "transfer",
      "description": "Transfer tokens between addresses",
      "event": "tokens_transferred",
      "guard": "balances[from] >= amount && amount > 0",
      "bindings": [
        {"name": "from", "type": "string", "keys": ["from"]},
        {"name": "to", "type": "string", "keys": ["to"]},
        {"name": "amount", "type": "number", "value": true}
      ]
    }
  ]
}
```

### Event Record

When a transfer executes, the full event is stored:

```json
{
  "event_type": "tokens_transferred",
  "aggregate_id": "ledger-001",
  "timestamp": "2024-01-19T10:30:00Z",
  "version": 42,
  "data": {
    "from": "alice",
    "to": "bob",
    "amount": 100,
    "memo": "Payment for services",
    "transaction_id": "txn-abc123"
  }
}
```

### State Computation

Only the bindings are used for state update:
- `from` (as key) → decrements `balances[alice]`
- `to` (as key) → increments `balances[bob]`
- `amount` (as value) → the transfer amount

The memo and transaction_id are preserved in the event for audit but don't affect state computation.

## Benefits

### Separation of Concerns

- **Business logic**: Captured in events
- **State logic**: Defined by bindings
- **Each can evolve independently**

### Efficient Computation

- State updates use minimal data
- Guards evaluate only necessary fields
- No overhead from audit fields

### Complete Audit Trail

- Every event is a complete record
- Can reconstruct business history
- Compliance and debugging support

### Flexible Evolution

- Add event fields without changing state logic
- Modify bindings without changing event schema
- Both sides remain loosely coupled

## Best Practices

### Design Events for Business

Events should answer: "What happened in the business domain?"

✅ Good:
```json
{
  "id": "order_shipped",
  "fields": [
    {"name": "order_id", "type": "string", "required": true},
    {"name": "tracking_number", "type": "string"},
    {"name": "carrier", "type": "string"},
    {"name": "estimated_delivery", "type": "time"},
    {"name": "warehouse_location", "type": "string"}
  ]
}
```

❌ Bad (too technical):
```json
{
  "id": "state_changed",
  "fields": [
    {"name": "old_state", "type": "string"},
    {"name": "new_state", "type": "string"}
  ]
}
```

### Design Bindings for Computation

Bindings should answer: "What data do I need to validate and update state?"

✅ Good:
```json
{
  "bindings": [
    {"name": "from", "type": "string", "keys": ["from"]},
    {"name": "amount", "type": "number", "value": true}
  ],
  "guard": "balances[from] >= amount"
}
```

❌ Bad (unnecessary data):
```json
{
  "bindings": [
    {"name": "from", "type": "string"},
    {"name": "to", "type": "string"},
    {"name": "amount", "type": "number"},
    {"name": "memo", "type": "string"},
    {"name": "timestamp", "type": "time"},
    {"name": "transaction_id", "type": "string"}
  ]
}
```

### Keep Events Stable

Once an event schema is deployed:
- Don't remove fields (breaks replay)
- Add new fields as optional
- Use versioning for breaking changes

Bindings can change more freely since they don't affect the event store.

### Use Descriptive Names

Events use past tense (what happened):
- `order_validated`
- `payment_processed`
- `tokens_transferred`

Bindings use present tense (what data):
- `order_id`
- `amount`
- `from`, `to`

## See Also

- [Order Processing Example](../examples/order-processing.json) - Complete Events First example
- [ERC-20 Token Example](../examples/erc20-token.json) - Token ledger with map bindings
- [Architecture](../ARCHITECTURE.md) - How Events First fits into the overall design
