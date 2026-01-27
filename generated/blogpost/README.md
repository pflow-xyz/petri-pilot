
# blog-post

A blog post workflow with persistence and search

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
| `draft` | Token | 1 | Post is being written |
| `in_review` | Token | 0 | Post is awaiting editorial review |
| `published` | Token | 0 | Post is live on the site |
| `archived` | Token | 0 | Post has been taken down |


### Transitions (Actions)

| Transition | Event | Guard | Description |
|------------|-------|-------|-------------|
| `create_post` | `post_created` | - | Create a new blog post |
| `update` | `post_updated` | - | Update post content |
| `submit` | `post_submitted` | - | Submit draft for review |
| `approve` | `post_approved` | - | Approve and publish the post |
| `reject` | `post_rejected` | - | Reject and return to draft |
| `unpublish` | `post_unpublished` | - | Take down a published post |
| `restore` | `post_restored` | - | Restore archived post to draft |


### Petri Net Diagram

```mermaid
stateDiagram-v2
    direction LR

    state "draft (1)" as PlaceDraft
    state "in_review" as PlaceInReview
    state "published" as PlacePublished
    state "archived" as PlaceArchived


    state "create_post" as t_TransitionCreatePost
    state "update" as t_TransitionUpdate
    state "submit" as t_TransitionSubmit
    state "approve" as t_TransitionApprove
    state "reject" as t_TransitionReject
    state "unpublish" as t_TransitionUnpublish
    state "restore" as t_TransitionRestore


    t_TransitionCreatePost --> PlaceDraft

    PlaceDraft --> t_TransitionUpdate
    t_TransitionUpdate --> PlaceDraft

    PlaceDraft --> t_TransitionSubmit
    t_TransitionSubmit --> PlaceInReview

    PlaceInReview --> t_TransitionApprove
    t_TransitionApprove --> PlacePublished

    PlaceInReview --> t_TransitionReject
    t_TransitionReject --> PlaceDraft

    PlacePublished --> t_TransitionUnpublish
    t_TransitionUnpublish --> PlaceArchived

    PlaceArchived --> t_TransitionRestore
    t_TransitionRestore --> PlaceDraft

```

### Workflow Diagram

```mermaid
flowchart TD
    subgraph Places
        PlaceDraft[("draft<br/>initial: 1")]
        PlaceInReview[("in_review")]
        PlacePublished[("published")]
        PlaceArchived[("archived")]
    end

    subgraph Transitions
        t_TransitionCreatePost["create_post"]
        t_TransitionUpdate["update"]
        t_TransitionSubmit["submit"]
        t_TransitionApprove["approve"]
        t_TransitionReject["reject"]
        t_TransitionUnpublish["unpublish"]
        t_TransitionRestore["restore"]
    end


    t_TransitionCreatePost --> PlaceDraft

    PlaceDraft --> t_TransitionUpdate
    t_TransitionUpdate --> PlaceDraft

    PlaceDraft --> t_TransitionSubmit
    t_TransitionSubmit --> PlaceInReview

    PlaceInReview --> t_TransitionApprove
    t_TransitionApprove --> PlacePublished

    PlaceInReview --> t_TransitionReject
    t_TransitionReject --> PlaceDraft

    PlacePublished --> t_TransitionUnpublish
    t_TransitionUnpublish --> PlaceArchived

    PlaceArchived --> t_TransitionRestore
    t_TransitionRestore --> PlaceDraft


    style Places fill:#e1f5fe
    style Transitions fill:#fff3e0
```


## Events

Events are immutable records of state transitions. Each event captures the transition that occurred and any associated data.

| Event Type | Transition | Fields |
|------------|------------|--------|
| `PostCreated` | `create_post` | `aggregate_id`, `timestamp`, `title`, `content`, `author_id`, `author_name`, `tags` |
| `PostUpdated` | `update` | `aggregate_id`, `timestamp`, `title`, `content`, `tags` |
| `PostSubmitted` | `submit` | `aggregate_id`, `timestamp` |
| `PostApproved` | `approve` | `aggregate_id`, `timestamp`, `approved_by` |
| `PostRejected` | `reject` | `aggregate_id`, `timestamp`, `rejected_by`, `reason` |
| `PostUnpublished` | `unpublish` | `aggregate_id`, `timestamp` |
| `PostRestored` | `restore` | `aggregate_id`, `timestamp` |


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


    class PostCreatedEvent {
        +string AggregateId
        +time.Time Timestamp
        +string Title
        +string Content
        +string AuthorId
        +string AuthorName
        +[]string Tags
    }
    Event <|-- PostCreatedEvent

    class PostUpdatedEvent {
        +string AggregateId
        +time.Time Timestamp
        +string Title
        +string Content
        +[]string Tags
    }
    Event <|-- PostUpdatedEvent

    class PostSubmittedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PostSubmittedEvent

    class PostApprovedEvent {
        +string AggregateId
        +time.Time Timestamp
        +string ApprovedBy
    }
    Event <|-- PostApprovedEvent

    class PostRejectedEvent {
        +string AggregateId
        +time.Time Timestamp
        +string RejectedBy
        +string Reason
    }
    Event <|-- PostRejectedEvent

    class PostUnpublishedEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PostUnpublishedEvent

    class PostRestoredEvent {
        +string AggregateId
        +time.Time Timestamp
    }
    Event <|-- PostRestoredEvent

```



## Access Control

Role-based access control (RBAC) restricts which users can execute transitions.


### Roles

| Role | Description | Inherits |
|------|-------------|----------|
| `author` | Content creator who writes and submits posts | - |
| `editor` | Reviews and approves/rejects submitted posts | - |
| `admin` | Full access to all operations | `author`, `editor` |



### Permissions

| Transition | Required Roles | Guard |
|------------|----------------|-------|
| `create_post` | `author` | - |
| `update` | `author` | - |
| `submit` | `author` | - |
| `approve` | `editor` | - |
| `reject` | `editor` | - |
| `unpublish` | `editor` | - |
| `restore` | `admin` | - |


```mermaid
graph TD
    subgraph Roles
        role_author["author"]
        role_editor["editor"]
        role_admin["admin"]
    end

    subgraph Transitions
        t_create_post["create_post"]
        t_update["update"]
        t_submit["submit"]
        t_approve["approve"]
        t_reject["reject"]
        t_unpublish["unpublish"]
        t_restore["restore"]
    end


    role_author -.->|can execute| t_create_post

    role_author -.->|can execute| t_update

    role_author -.->|can execute| t_submit

    role_editor -.->|can execute| t_approve

    role_editor -.->|can execute| t_reject

    role_editor -.->|can execute| t_unpublish

    role_admin -.->|can execute| t_restore

```


## API Endpoints

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| POST | `/api/blog-post` | Create new instance |
| GET | `/api/blog-post/{id}` | Get instance state |
| GET | `/api/navigation` | Get navigation menu |
| GET | `/admin/stats` | Admin statistics |
| GET | `/admin/instances` | List all instances |
| GET | `/admin/instances/{id}` | Get instance detail |
| GET | `/admin/instances/{id}/events` | Get instance events |


### Transition Endpoints

| Method | Path | Transition | Description |
|--------|------|------------|-------------|
| POST | `/api/create_post` | `create_post` | Create a new blog post |
| POST | `/api/update` | `update` | Update post content |
| POST | `/api/submit` | `submit` | Submit draft for review |
| POST | `/api/approve` | `approve` | Approve and publish the post |
| POST | `/api/reject` | `reject` | Reject and return to draft |
| POST | `/api/unpublish` | `unpublish` | Take down a published post |
| POST | `/api/restore` | `restore` | Restore archived post to draft |


### Request/Response Format

#### Create Instance
```bash
curl -X POST http://localhost:8080/api/blog-post \
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


## Navigation

| Label | Path | Icon | Roles |
|-------|------|------|-------|
| Posts | `/` | 📝 | * |
| Admin | `/admin` | ⚙️ | `admin`, `editor` |




## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `./blog-post.db` | SQLite database path |
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
├── auth.go           # Authentication
├── middleware.go     # HTTP middleware
├── permissions.go    # Permission checks
├── navigation.go     # Navigation menu
├── admin.go          # Admin handlers
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
