// Package schema defines the intermediate representation for Petri net models.
package schema

// StateKind discriminates between token-counting and data-holding places.
type StateKind string

const (
	// TokenKind holds an integer count (classic Petri net semantics).
	TokenKind StateKind = "token"

	// DataKind holds structured data (maps, structs).
	DataKind StateKind = "data"
)

// Model represents a Petri net model in an LLM-friendly format.
type Model struct {
	Name        string       `json:"name"`
	Version     string       `json:"version,omitempty"`
	Description string       `json:"description,omitempty"`
	Places      []Place      `json:"places"`
	Transitions []Transition `json:"transitions"`
	Arcs        []Arc        `json:"arcs"`
	Constraints []Constraint `json:"constraints,omitempty"`

	// Events define the data contract for transitions (Events First schema)
	Events []Event `json:"events,omitempty"`

	// Access control (Phase 11)
	Roles  []Role       `json:"roles,omitempty"`
	Access []AccessRule `json:"access,omitempty"`

	// Views (Phase 13)
	Views []View `json:"views,omitempty"`

	// Navigation (Phase 14)
	Navigation *Navigation `json:"navigation,omitempty"`

	// Admin Dashboard (Phase 14)
	Admin *Admin `json:"admin,omitempty"`

	// Event Sourcing (Phase 14)
	EventSourcing *EventSourcing `json:"eventSourcing,omitempty"`

	// Debug configuration
	Debug *Debug `json:"debug,omitempty"`
}

// Event represents an explicit event definition with typed fields.
// Events define the data contract; the workflow constrains valid sequences.
type Event struct {
	ID          string       `json:"id"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Fields      []EventField `json:"fields"`
}

// EventField represents a typed field within an event.
// Events capture the complete record including optional fields for audit/replay.
type EventField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`               // string, number, integer, boolean, array, object, time
	Of          string `json:"of,omitempty"`       // element type for array/object
	Required    bool   `json:"required,omitempty"` // true if must be in event record
	Description string `json:"description,omitempty"`
}

// Binding represents operational data needed for state computation.
// Bindings define the data required to evaluate guards and apply arc transformations.
// This mirrors arcnet's Arc pattern with keys for map access and value for transfers.
type Binding struct {
	Name  string   `json:"name"`            // binding name (e.g., "from", "to", "amount")
	Type  string   `json:"type"`            // data type: string, number, map[string]number, etc.
	Keys  []string `json:"keys,omitempty"`  // map access path (e.g., ["owner", "spender"] for nested maps)
	Value bool     `json:"value,omitempty"` // true if this is the transfer value (like Arc.Value)
	Place string   `json:"place,omitempty"` // place ID this binding reads from/writes to
}

// View represents a UI view definition for presenting workflow data.
type View struct {
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Kind        string      `json:"kind,omitempty"` // form, card, table, detail
	Description string      `json:"description,omitempty"`
	Groups      []ViewGroup `json:"groups,omitempty"`
	Actions     []string    `json:"actions,omitempty"` // Transition IDs that can be triggered from this view
}

// ViewGroup represents a logical grouping of fields within a view.
type ViewGroup struct {
	ID     string      `json:"id"`
	Name   string      `json:"name,omitempty"`
	Fields []ViewField `json:"fields"`
}

// ViewField represents a single field within a view group.
type ViewField struct {
	Binding     string `json:"binding"`
	Label       string `json:"label,omitempty"`
	Type        string `json:"type,omitempty"` // text, number, select, date, etc.
	Required    bool   `json:"required,omitempty"`
	ReadOnly    bool   `json:"readonly,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// Role defines a named role for access control.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Inherits    []string `json:"inherits,omitempty"` // Parent role IDs for inheritance
}

// AccessRule defines who can execute a transition.
type AccessRule struct {
	Transition string   `json:"transition"`        // Transition ID or "*" for all
	Roles      []string `json:"roles,omitempty"`   // Allowed roles (empty = any authenticated user)
	Guard      string   `json:"guard,omitempty"`   // Guard expression (e.g., "user.id == customer_id")
}

// Place represents a state/resource in the model.
type Place struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Initial     int    `json:"initial"`

	// Extended fields for metamodel compatibility
	Kind      StateKind `json:"kind,omitempty"`      // "token" or "data" (default: "token")
	Type      string    `json:"type,omitempty"`      // Data type for DataKind places
	Exported  bool      `json:"exported,omitempty"`  // Externally visible state
	Persisted bool      `json:"persisted,omitempty"` // Should be stored in event store

	// InitialValue is the initial value for data places (JSON-encoded for complex types).
	// For simple types: "hello" for string, 0 for int64, true for bool
	// For maps: {} or {"key": value}
	InitialValue any `json:"initial_value,omitempty"`
}

// Supported Type values for DataKind places:
//   Simple types (values from bindings):
//     - "string"  - text value
//     - "int64"   - integer value
//     - "float64" - floating point
//     - "bool"    - boolean
//   Collection types (key-value access via arc Keys/Value):
//     - "map[string]int64"           - balance ledger
//     - "map[string]string"          - key-value store
//     - "map[string]map[string]int64" - allowances (nested map)

// IsToken returns true if this is a token-counting place.
func (p *Place) IsToken() bool {
	return p.Kind == TokenKind || p.Kind == ""
}

// IsData returns true if this is a data-holding place.
func (p *Place) IsData() bool {
	return p.Kind == DataKind
}

// IsSimpleType returns true if this data place holds a simple type (string, int64, etc.)
// rather than a collection type (map).
func (p *Place) IsSimpleType() bool {
	if !p.IsData() {
		return false
	}
	switch p.Type {
	case "string", "int64", "int", "float64", "bool", "time.Time":
		return true
	default:
		return false
	}
}

// IsMapType returns true if this data place holds a map type.
func (p *Place) IsMapType() bool {
	if !p.IsData() {
		return false
	}
	return len(p.Type) > 4 && p.Type[:4] == "map["
}

// Transition represents an action/event in the model.
type Transition struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Guard       string `json:"guard,omitempty"`

	// Event reference (Events First schema) - references Event.ID
	Event string `json:"event,omitempty"`

	// Bindings define operational data for state computation (arcnet pattern)
	// These are used to evaluate guards and apply arc transformations
	Bindings []Binding `json:"bindings,omitempty"`

	// Extended fields for API routing
	HTTPMethod string `json:"http_method,omitempty"` // GET, POST, etc.
	HTTPPath   string `json:"http_path,omitempty"`   // API path, e.g., "/orders/{id}/confirm"

	// Deprecated fields (for backward compatibility)
	EventType      string            `json:"event_type,omitempty"`       // Use Event instead
	LegacyBindings map[string]string `json:"legacy_bindings,omitempty"` // Use Bindings instead
}

// Arc represents a flow between place and transition.
type Arc struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight,omitempty"` // default 1

	// Extended fields for data flow
	Keys  []string `json:"keys,omitempty"`  // Map access keys for data places
	Value string   `json:"value,omitempty"` // Value binding name (default: "amount")
}

// Constraint represents an invariant on the model.
type Constraint struct {
	ID   string `json:"id"`
	Expr string `json:"expr"`
}

// ValidationResult contains the outcome of model validation.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
	Analysis *AnalysisResult   `json:"analysis,omitempty"`
}

// ValidationError describes a specific validation issue.
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Element string `json:"element,omitempty"` // affected element ID
	Fix     string `json:"fix,omitempty"`     // suggested fix
}

// AnalysisResult contains detailed model analysis.
type AnalysisResult struct {
	Bounded        bool              `json:"bounded"`
	Live           bool              `json:"live"`
	HasDeadlocks   bool              `json:"has_deadlocks"`
	Deadlocks      []string          `json:"deadlocks,omitempty"`
	StateCount     int               `json:"state_count"`
	SymmetryGroups []SymmetryGroup   `json:"symmetry_groups,omitempty"`
	CriticalPath   []string          `json:"critical_path,omitempty"`
	Isolated       []string          `json:"isolated,omitempty"`
	Importance     []ElementAnalysis `json:"importance,omitempty"`
}

// SymmetryGroup represents elements with identical behavioral impact.
type SymmetryGroup struct {
	Elements []string `json:"elements"`
	Impact   float64  `json:"impact"`
}

// ElementAnalysis contains importance metrics for a single element.
type ElementAnalysis struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"` // place, transition, arc
	Importance float64 `json:"importance"`
	Category   string  `json:"category"` // critical, important, minor, redundant
}

// FeedbackPrompt generates structured feedback for LLM refinement.
type FeedbackPrompt struct {
	OriginalRequirements string           `json:"original_requirements"`
	CurrentModel         *Model           `json:"current_model"`
	ValidationResult     *ValidationResult `json:"validation_result"`
	Instructions         string           `json:"instructions"`
}

// Navigation represents the navigation menu configuration.
type Navigation struct {
	Brand string           `json:"brand"`
	Items []NavigationItem `json:"items"`
}

// NavigationItem represents a single navigation menu item.
type NavigationItem struct {
	Label string   `json:"label"`
	Path  string   `json:"path"`
	Icon  string   `json:"icon,omitempty"`
	Roles []string `json:"roles,omitempty"` // empty = visible to all
}

// Admin represents admin dashboard configuration.
type Admin struct {
	Enabled  bool     `json:"enabled"`
	Path     string   `json:"path"`
	Roles    []string `json:"roles"`
	Features []string `json:"features"` // list, detail, history, transitions
}

// EventSourcing represents event sourcing configuration.
type EventSourcing struct {
	Snapshots *SnapshotConfig  `json:"snapshots,omitempty"`
	Retention *RetentionConfig `json:"retention,omitempty"`
}

// SnapshotConfig controls automatic snapshot creation.
type SnapshotConfig struct {
	Enabled   bool `json:"enabled"`
	Frequency int  `json:"frequency"` // Every N events
}

// RetentionConfig controls event and snapshot retention.
type RetentionConfig struct {
	Events    string `json:"events"`    // e.g., "90d"
	Snapshots string `json:"snapshots"` // e.g., "1y"
}

// Debug represents debug configuration for development and testing.
type Debug struct {
	Enabled bool `json:"enabled"`
	Eval    bool `json:"eval,omitempty"` // Enable eval endpoint for remote code execution
}
