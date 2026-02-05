// Package extensions provides petri-pilot application-level extensions
// that work with go-pflow's metamodel extension system.
package extensions

import (
	"encoding/json"
	"fmt"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

const (
	// EntitiesExtensionName is the extension name for entity definitions.
	EntitiesExtensionName = "petri-pilot/entities"
)

// EntityExtension adds entity definitions to a Petri net model.
// Entities are the core data models that become event-sourced aggregates.
type EntityExtension struct {
	goflowmodel.BaseExtension
	Entities []Entity `json:"entities"`
}

// Entity represents a domain entity with its state machine.
// Each entity generates an aggregate with event sourcing.
type Entity struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// Fields define the data schema.
	Fields []Field `json:"fields"`

	// States define the lifecycle states (token places).
	States []EntityState `json:"states,omitempty"`

	// Actions define operations that can be performed.
	Actions []EntityAction `json:"actions"`

	// Constraints define invariants.
	Constraints []Constraint `json:"constraints,omitempty"`

	// Access defines who can do what.
	Access []AccessRule `json:"access,omitempty"`
}

// Field represents a data field in an entity.
type Field struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Unique      bool      `json:"unique,omitempty"`
	Indexed     bool      `json:"indexed,omitempty"`
	Default     any       `json:"default,omitempty"`

	// Reference links to another entity (foreign key).
	Reference *FieldReference `json:"reference,omitempty"`

	// Computed indicates this field is derived from an expression.
	Computed string `json:"computed,omitempty"`

	// Validation is a guard expression for field-level validation.
	Validation string `json:"validation,omitempty"`
}

// UnmarshalJSON handles Field where Gemini may use "name" instead of "id".
func (f *Field) UnmarshalJSON(data []byte) error {
	type Alias Field
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*f = Field(a)
	// If id is empty but name is set, use name as id
	if f.ID == "" && f.Name != "" {
		f.ID = f.Name
	}
	return nil
}

// FieldType represents the type of a field.
type FieldType string

const (
	FieldTypeString    FieldType = "string"
	FieldTypeInt       FieldType = "int"
	FieldTypeInt64     FieldType = "int64"
	FieldTypeFloat64   FieldType = "float64"
	FieldTypeBool      FieldType = "bool"
	FieldTypeTime      FieldType = "time"
	FieldTypeJSON      FieldType = "json"
	FieldTypeReference FieldType = "reference"
)

// FieldReference defines a relationship to another entity.
type FieldReference struct {
	Entity   string `json:"entity"`             // Target entity ID
	Field    string `json:"field,omitempty"`    // Target field (default: id)
	OnDelete string `json:"on_delete,omitempty"` // cascade, restrict, set_null
}

// EntityState represents a lifecycle state.
// Supports unmarshaling from either a string ("active") or an object ({"id": "active", ...}).
type EntityState struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Initial     bool   `json:"initial,omitempty"`
	Terminal    bool   `json:"terminal,omitempty"`
}

// UnmarshalJSON handles both string and object formats for EntityState.
func (s *EntityState) UnmarshalJSON(data []byte) error {
	// Try string format first: "active"
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.ID = str
		return nil
	}
	// Object format: {"id": "active", ...}
	type Alias EntityState
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = EntityState(a)
	return nil
}

// EntityAction represents an operation on an entity.
// Supports unmarshaling from either a string ("create") or an object ({"id": "create", ...}).
type EntityAction struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// FromStates lists states where this action is available.
	// Empty means available from any state.
	FromStates []string `json:"from_states,omitempty"`

	// ToState is the resulting state after this action.
	ToState string `json:"to_state,omitempty"`

	// Guard is a precondition expression.
	Guard string `json:"guard,omitempty"`

	// Input defines the action parameters.
	Input []ActionParam `json:"input,omitempty"`

	// Effects define field updates.
	Effects []ActionEffect `json:"effects,omitempty"`

	// Triggers define side effects (webhooks, other actions).
	Triggers []ActionTrigger `json:"triggers,omitempty"`

	// HTTP defines the API endpoint.
	HTTP *HTTPEndpoint `json:"http,omitempty"`
}

// UnmarshalJSON handles both string and object formats for EntityAction.
func (a *EntityAction) UnmarshalJSON(data []byte) error {
	// Try string format first: "create_task"
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		a.ID = str
		return nil
	}
	// Object format: {"id": "create_task", ...}
	type Alias EntityAction
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*a = EntityAction(alias)
	return nil
}

// ActionParam defines an input parameter for an action.
type ActionParam struct {
	ID         string    `json:"id"`
	Name       string    `json:"name,omitempty"`
	Type       FieldType `json:"type"`
	Required   bool      `json:"required,omitempty"`
	Default    any       `json:"default,omitempty"`
	Validation string    `json:"validation,omitempty"`
}

// ActionEffect defines a field update resulting from an action.
type ActionEffect struct {
	Field string `json:"field"`        // Field to update
	Value string `json:"value"`        // Expression for new value
	Op    string `json:"op,omitempty"` // set, add, subtract, append (default: set)
}

// ActionTrigger defines a side effect of an action.
type ActionTrigger struct {
	Type      string         `json:"type"` // webhook, action, workflow
	Condition string         `json:"condition,omitempty"`
	Config    map[string]any `json:"config"`
}

// HTTPEndpoint defines REST API mapping for an action.
type HTTPEndpoint struct {
	Method string `json:"method"` // GET, POST, PUT, PATCH, DELETE
	Path   string `json:"path"`   // e.g., "/orders/{id}/confirm"
}

// Constraint represents a property that must hold.
type Constraint struct {
	ID   string `json:"id"`
	Expr string `json:"expr"`
}

// AccessRule defines who can perform an action.
type AccessRule struct {
	Action string   `json:"action"`          // Action ID or "*" for all
	Roles  []string `json:"roles,omitempty"` // Allowed roles
	Guard  string   `json:"guard,omitempty"` // Additional condition
}

// NewEntityExtension creates a new EntityExtension.
func NewEntityExtension() *EntityExtension {
	return &EntityExtension{
		BaseExtension: goflowmodel.NewBaseExtension(EntitiesExtensionName),
		Entities:      make([]Entity, 0),
	}
}

// Validate checks that all entities are valid and consistent with the model.
func (e *EntityExtension) Validate(model *goflowmodel.Model) error {
	seen := make(map[string]bool)
	for _, entity := range e.Entities {
		if seen[entity.ID] {
			return fmt.Errorf("duplicate entity ID: %s", entity.ID)
		}
		seen[entity.ID] = true

		// Validate field IDs are unique within entity
		fieldIDs := make(map[string]bool)
		for _, field := range entity.Fields {
			if fieldIDs[field.ID] {
				return fmt.Errorf("entity %s: duplicate field ID: %s", entity.ID, field.ID)
			}
			fieldIDs[field.ID] = true
		}

		// Validate state IDs are unique within entity
		stateIDs := make(map[string]bool)
		for _, state := range entity.States {
			if stateIDs[state.ID] {
				return fmt.Errorf("entity %s: duplicate state ID: %s", entity.ID, state.ID)
			}
			stateIDs[state.ID] = true
		}

		// Validate action IDs are unique within entity
		actionIDs := make(map[string]bool)
		for _, action := range entity.Actions {
			if actionIDs[action.ID] {
				return fmt.Errorf("entity %s: duplicate action ID: %s", entity.ID, action.ID)
			}
			actionIDs[action.ID] = true

			// Validate FromStates reference existing states
			for _, from := range action.FromStates {
				if len(entity.States) > 0 && !stateIDs[from] {
					return fmt.Errorf("entity %s action %s: unknown from_state: %s",
						entity.ID, action.ID, from)
				}
			}

			// Validate ToState references existing state
			if action.ToState != "" && len(entity.States) > 0 && !stateIDs[action.ToState] {
				return fmt.Errorf("entity %s action %s: unknown to_state: %s",
					entity.ID, action.ID, action.ToState)
			}
		}
	}
	return nil
}

// MarshalJSON serializes the entities.
func (e *EntityExtension) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Entities)
}

// UnmarshalJSON deserializes the entities.
func (e *EntityExtension) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &e.Entities)
}

// AddEntity adds an entity to the extension.
func (e *EntityExtension) AddEntity(entity Entity) {
	e.Entities = append(e.Entities, entity)
}

// EntityByID returns an entity by ID, or nil if not found.
func (e *EntityExtension) EntityByID(id string) *Entity {
	for i := range e.Entities {
		if e.Entities[i].ID == id {
			return &e.Entities[i]
		}
	}
	return nil
}

// init registers the entity extension with the default registry.
func init() {
	goflowmodel.Register(EntitiesExtensionName, func() goflowmodel.ModelExtension {
		return NewEntityExtension()
	})
}
