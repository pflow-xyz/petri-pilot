package metamodel

// Application represents a complete application specification.
// This is the top-level structure an LLM would generate.
type Application struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`

	// Entities are the core data models (each becomes an aggregate).
	Entities []Entity `json:"entities"`

	// Roles define access control.
	Roles []Role `json:"roles,omitempty"`

	// Pages define the UI structure.
	Pages []Page `json:"pages,omitempty"`

	// Integrations define external connections.
	Integrations []Integration `json:"integrations,omitempty"`

	// Workflows define cross-entity orchestration.
	Workflows []Workflow `json:"workflows,omitempty"`
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

// FieldReference defines a relationship to another entity.
type FieldReference struct {
	Entity   string `json:"entity"`            // Target entity ID
	Field    string `json:"field,omitempty"`   // Target field (default: id)
	OnDelete string `json:"on_delete,omitempty"` // cascade, restrict, set_null
}

// EntityState represents a lifecycle state.
type EntityState struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Initial     bool   `json:"initial,omitempty"`
	Terminal    bool   `json:"terminal,omitempty"`
}

// EntityAction represents an operation on an entity.
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

// ActionParam defines an input parameter for an action.
type ActionParam struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Default     any       `json:"default,omitempty"`
	Validation  string    `json:"validation,omitempty"`
}

// ActionEffect defines a field update resulting from an action.
type ActionEffect struct {
	Field string `json:"field"`      // Field to update
	Value string `json:"value"`      // Expression for new value (can reference inputs)
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

// AccessRule defines who can perform an action.
type AccessRule struct {
	Action string   `json:"action"`          // Action ID or "*" for all
	Roles  []string `json:"roles,omitempty"` // Allowed roles
	Guard  string   `json:"guard,omitempty"` // Additional condition (e.g., "user.id == owner")
}

// Role defines an access control role.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Inherits    []string `json:"inherits,omitempty"` // Parent roles
}

// Page defines a UI page.
type Page struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"` // URL path
	Icon        string `json:"icon,omitempty"`

	// Layout defines the page structure.
	Layout PageLayout `json:"layout"`

	// Access defines who can view this page.
	Access []string `json:"access,omitempty"` // Role IDs
}

// PageLayout defines the structure of a page.
type PageLayout struct {
	Type       string        `json:"type"` // list, detail, form, dashboard, custom
	Entity     string        `json:"entity,omitempty"`
	Components []UIComponent `json:"components,omitempty"`
}

// UIComponent defines a UI component within a page.
type UIComponent struct {
	Type   string         `json:"type"` // table, card, form, chart, stat, custom
	Config map[string]any `json:"config,omitempty"`
}

// Integration defines an external system connection.
type Integration struct {
	ID          string         `json:"id"`
	Name        string         `json:"name,omitempty"`
	Type        string         `json:"type"` // webhook, api, database, queue
	Config      map[string]any `json:"config"`
}

// Workflow defines cross-entity orchestration.
type Workflow struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// Trigger defines what starts the workflow.
	Trigger WorkflowTrigger `json:"trigger"`

	// Steps define the workflow sequence.
	Steps []WorkflowStep `json:"steps"`
}

// WorkflowTrigger defines what initiates a workflow.
type WorkflowTrigger struct {
	Type   string `json:"type"`   // event, schedule, manual
	Entity string `json:"entity,omitempty"`
	Action string `json:"action,omitempty"`
	Cron   string `json:"cron,omitempty"`
}

// WorkflowStep defines a single step in a workflow.
type WorkflowStep struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // action, condition, parallel, wait
	Entity    string `json:"entity,omitempty"`
	Action    string `json:"action,omitempty"`
	Condition string `json:"condition,omitempty"`
	Input     map[string]string `json:"input,omitempty"` // Mapping from workflow context
	OnSuccess string `json:"on_success,omitempty"` // Next step ID
	OnFailure string `json:"on_failure,omitempty"` // Step ID on failure
}

// ToSchema converts an Entity to a metamodel Schema.
func (e *Entity) ToSchema() *Schema {
	s := NewSchema(e.ID)
	s.Description = e.Description

	// Add data states from fields
	for _, f := range e.Fields {
		s.AddState(State{
			ID:          f.ID,
			Kind:        DataState,
			Type:        string(f.Type),
			Description: f.Description,
			Exported:    true,
		})
	}

	// Add token states from entity states
	for _, st := range e.States {
		initial := 0
		if st.Initial {
			initial = 1
		}
		s.AddState(State{
			ID:          st.ID,
			Kind:        TokenState,
			Initial:     initial,
			Description: st.Description,
		})
	}

	// Add actions
	for _, a := range e.Actions {
		bindings := make(map[string]string)
		for _, p := range a.Input {
			bindings[p.ID] = string(p.Type)
		}
		s.AddAction(Action{
			ID:          a.ID,
			Description: a.Description,
			Guard:       a.Guard,
			Bindings:    bindings,
		})

		// Add arcs for state transitions
		for _, from := range a.FromStates {
			s.AddArc(Arc{Source: from, Target: a.ID})
		}
		if a.ToState != "" {
			s.AddArc(Arc{Source: a.ID, Target: a.ToState})
		}

		// Add arcs for field effects
		for _, eff := range a.Effects {
			s.AddArc(Arc{
				Source: a.ID,
				Target: eff.Field,
				Value:  eff.Value,
			})
		}
	}

	// Add constraints
	for _, c := range e.Constraints {
		s.AddConstraint(c)
	}

	return s
}
