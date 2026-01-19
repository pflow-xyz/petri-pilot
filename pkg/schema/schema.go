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

	// SLA configuration
	SLA *SLAConfig `json:"sla,omitempty"`

	// Prediction configuration for ODE-based simulation
	Prediction *PredictionConfig `json:"prediction,omitempty"`

	// GraphQL API configuration
	GraphQL *GraphQL `json:"graphql,omitempty"`

	// Blobstore configuration for event attachments
	Blobstore *Blobstore `json:"blobstore,omitempty"`

	// Timers for scheduled/delayed transitions
	Timers []Timer `json:"timers,omitempty"`

	// Notifications triggered by state changes
	Notifications []Notification `json:"notifications,omitempty"`

	// Relationships between workflow instances
	Relationships []Relationship `json:"relationships,omitempty"`

	// Computed fields derived from state
	Computed []ComputedField `json:"computed,omitempty"`

	// Indexes for search and filtering
	Indexes []Index `json:"indexes,omitempty"`

	// Approval chains for multi-step approvals
	Approvals map[string]*ApprovalChain `json:"approvals,omitempty"`

	// Templates for pre-configured starting states
	Templates []Template `json:"templates,omitempty"`

	// Batch operations configuration
	Batch *BatchConfig `json:"batch,omitempty"`

	// Inbound webhooks for external event triggers
	InboundWebhooks []InboundWebhook `json:"inboundWebhooks,omitempty"`

	// Documents for PDF/document generation
	Documents []Document `json:"documents,omitempty"`

	// Comments configuration
	Comments *CommentsConfig `json:"comments,omitempty"`

	// Tags configuration
	Tags *TagsConfig `json:"tags,omitempty"`

	// Activity feed configuration
	Activity *ActivityConfig `json:"activity,omitempty"`

	// Favorites/watchlist configuration
	Favorites *FavoritesConfig `json:"favorites,omitempty"`

	// Export configuration
	Export *ExportConfig `json:"export,omitempty"`

	// Soft delete configuration
	SoftDelete *SoftDeleteConfig `json:"softDelete,omitempty"`

	// Token/currency display configuration
	// Decimals specifies precision for token amounts (e.g., 18 for ETH where 1 ETH = 10^18 wei)
	// UI displays values divided by 10^decimals, backend stores raw integers
	Decimals int `json:"decimals,omitempty"`

	// Unit is the display symbol for token amounts (e.g., "ETH", "USDC", "tokens")
	Unit string `json:"unit,omitempty"`
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

	// Resource tracking fields for prediction/simulation
	Capacity int  `json:"capacity,omitempty"` // Maximum tokens (for inventory modeling)
	Resource bool `json:"resource,omitempty"` // True if this is a consumable resource
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

	// SLA timing fields
	Duration    string `json:"duration,omitempty"`    // Expected duration (e.g., "30s", "2m")
	MinDuration string `json:"minDuration,omitempty"` // Minimum expected duration
	MaxDuration string `json:"maxDuration,omitempty"` // Maximum allowed duration (SLA breach)

	// Prediction/simulation fields
	Rate float64 `json:"rate,omitempty"` // Firing rate for ODE simulation (events/minute)

	// Deprecated fields (for backward compatibility)
	EventType      string            `json:"event_type,omitempty"`      // Use Event instead
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

// SLAConfig represents workflow-level SLA configuration.
type SLAConfig struct {
	Default    string            `json:"default,omitempty"`    // Default SLA duration (e.g., "5m", "1h")
	ByPriority map[string]string `json:"byPriority,omitempty"` // SLA by priority level (e.g., {"high": "2m", "normal": "5m"})
	WarningAt  float64           `json:"warningAt,omitempty"`  // Percentage (0.0-1.0) for warning status (default: 0.8)
	CriticalAt float64           `json:"criticalAt,omitempty"` // Percentage (0.0-1.0) for critical status (default: 0.95)
	OnBreach   string            `json:"onBreach,omitempty"`   // Action on breach: "alert", "log", "webhook"
}

// PredictionConfig represents ODE-based prediction configuration.
type PredictionConfig struct {
	Enabled   bool    `json:"enabled"`             // Enable ODE-based prediction
	TimeHours float64 `json:"timeHours,omitempty"` // Default simulation duration in hours (default: 8)
	RateScale float64 `json:"rateScale,omitempty"` // Rate scaling factor for numerical stability (default: 0.0001)
}

// GraphQL represents GraphQL API configuration.
type GraphQL struct {
	Enabled    bool   `json:"enabled"`              // Enable GraphQL API
	Path       string `json:"path,omitempty"`       // GraphQL endpoint path (default: "/graphql")
	Playground bool   `json:"playground,omitempty"` // Enable GraphQL Playground (default: true)
}

// Blobstore represents blobstore configuration for event attachments.
type Blobstore struct {
	Enabled      bool     `json:"enabled"`                // Enable blobstore for binary/JSON attachments
	MaxSize      int64    `json:"maxSize,omitempty"`      // Maximum blob size in bytes (default: 10MB)
	AllowedTypes []string `json:"allowedTypes,omitempty"` // Allowed content types (default: ["application/json", "*/*"])
}

// Timer represents a scheduled or delayed transition trigger.
type Timer struct {
	ID         string `json:"id,omitempty"`        // Optional timer ID
	Transition string `json:"transition"`          // Transition to fire
	After      string `json:"after,omitempty"`     // Duration after entering state (e.g., "24h", "30m")
	Cron       string `json:"cron,omitempty"`      // Cron expression for scheduled firing
	From       string `json:"from,omitempty"`      // Place that triggers the timer (for "after" timers)
	Condition  string `json:"condition,omitempty"` // Optional condition expression
	Repeat     bool   `json:"repeat,omitempty"`    // Whether to repeat (for cron timers)
}

// Notification represents a notification triggered by state changes.
type Notification struct {
	ID        string            `json:"id,omitempty"`        // Optional notification ID
	On        string            `json:"on"`                  // Transition or place that triggers notification
	Channel   string            `json:"channel"`             // email, sms, slack, webhook, in_app
	To        string            `json:"to,omitempty"`        // Recipient expression (e.g., "{{applicant_email}}")
	Template  string            `json:"template,omitempty"`  // Template ID or inline template
	Subject   string            `json:"subject,omitempty"`   // Subject line (for email)
	Webhook   string            `json:"webhook,omitempty"`   // Webhook URL (can use env vars like $SLACK_WEBHOOK)
	Condition string            `json:"condition,omitempty"` // Optional condition expression
	Data      map[string]string `json:"data,omitempty"`      // Additional data to include
}

// Relationship represents a link between workflow instances.
type Relationship struct {
	Name       string `json:"name"`                 // Relationship name (e.g., "line_items", "parent")
	Type       string `json:"type"`                 // hasMany, hasOne, belongsTo
	Target     string `json:"target"`               // Target model/workflow name
	ForeignKey string `json:"foreignKey,omitempty"` // Foreign key field name
	Cascade    string `json:"cascade,omitempty"`    // Cascade behavior: delete, nullify, restrict
}

// ComputedField represents a derived value from state.
type ComputedField struct {
	Name        string   `json:"name"`                  // Field name
	Type        string   `json:"type,omitempty"`        // Result type: string, number, boolean, array
	Expr        string   `json:"expr"`                  // Expression to compute value
	DependsOn   []string `json:"dependsOn,omitempty"`   // Fields this depends on (for caching)
	Persisted   bool     `json:"persisted,omitempty"`   // Whether to store computed value
	Description string   `json:"description,omitempty"` // Description of the computed field
}

// Index represents a searchable index on workflow data.
type Index struct {
	Name   string   `json:"name,omitempty"` // Index name
	Fields []string `json:"fields"`         // Fields to index
	Type   string   `json:"type,omitempty"` // Index type: btree, fulltext, hash (default: btree)
	Unique bool     `json:"unique,omitempty"`
}

// ApprovalChain represents a multi-step approval workflow.
type ApprovalChain struct {
	Levels        []ApprovalLevel `json:"levels"`                  // Approval levels in order
	EscalateAfter string          `json:"escalateAfter,omitempty"` // Duration before escalation (e.g., "48h")
	OnReject      string          `json:"onReject,omitempty"`      // Transition to fire on rejection
	OnApprove     string          `json:"onApprove,omitempty"`     // Transition to fire on final approval
	Parallel      bool            `json:"parallel,omitempty"`      // Whether levels can approve in parallel
}

// ApprovalLevel represents a single level in an approval chain.
type ApprovalLevel struct {
	Role       string `json:"role,omitempty"`       // Required role for this level
	User       string `json:"user,omitempty"`       // Specific user expression
	Condition  string `json:"condition,omitempty"`  // Condition for this level to apply
	Required   int    `json:"required,omitempty"`   // Number of approvals required (default: 1)
	Transition string `json:"transition,omitempty"` // Custom transition for this level
}

// Template represents a pre-configured starting state.
type Template struct {
	ID          string         `json:"id"`                    // Template ID
	Name        string         `json:"name,omitempty"`        // Display name
	Description string         `json:"description,omitempty"` // Description
	Data        map[string]any `json:"data,omitempty"`        // Pre-filled data
	Roles       []string       `json:"roles,omitempty"`       // Roles that can use this template
	Default     bool           `json:"default,omitempty"`     // Whether this is the default template
}

// BatchConfig represents batch operations configuration.
type BatchConfig struct {
	Enabled     bool     `json:"enabled"`               // Enable batch operations
	Transitions []string `json:"transitions,omitempty"` // Transitions allowed in batch
	MaxSize     int      `json:"maxSize,omitempty"`     // Maximum batch size (default: 100)
}

// InboundWebhook represents an external webhook endpoint.
type InboundWebhook struct {
	ID         string            `json:"id,omitempty"`         // Webhook ID
	Path       string            `json:"path"`                 // URL path (e.g., "/hooks/stripe")
	Secret     string            `json:"secret,omitempty"`     // Secret for validation (can use env vars)
	Transition string            `json:"transition"`           // Transition to fire
	Map        map[string]string `json:"map,omitempty"`        // Field mapping from payload to event data
	Condition  string            `json:"condition,omitempty"`  // Condition for processing
	Method     string            `json:"method,omitempty"`     // HTTP method (default: POST)
}

// Document represents a document/PDF generation configuration.
type Document struct {
	ID          string `json:"id"`                    // Document ID
	Name        string `json:"name,omitempty"`        // Display name
	Template    string `json:"template"`              // Template file or inline template
	Format      string `json:"format,omitempty"`      // Output format: pdf, html, docx (default: pdf)
	Trigger     string `json:"trigger,omitempty"`     // Transition that triggers generation
	StoreTo     string `json:"storeTo,omitempty"`     // Blob field to store generated document
	Filename    string `json:"filename,omitempty"`    // Filename expression
	Description string `json:"description,omitempty"` // Description
}

// CommentsConfig represents comments/notes configuration.
type CommentsConfig struct {
	Enabled    bool     `json:"enabled"`              // Enable comments
	Roles      []string `json:"roles,omitempty"`      // Roles that can comment (empty = all authenticated)
	Moderation bool     `json:"moderation,omitempty"` // Require moderation for comments
	MaxLength  int      `json:"maxLength,omitempty"`  // Maximum comment length (default: 2000)
}

// TagsConfig represents tags/labels configuration.
type TagsConfig struct {
	Enabled    bool     `json:"enabled"`              // Enable tags
	Predefined []string `json:"predefined,omitempty"` // Predefined tag options
	FreeForm   bool     `json:"freeForm,omitempty"`   // Allow free-form tags (default: true)
	MaxTags    int      `json:"maxTags,omitempty"`    // Maximum tags per instance (default: 10)
	Colors     bool     `json:"colors,omitempty"`     // Enable tag colors
}

// ActivityConfig represents activity feed configuration.
type ActivityConfig struct {
	Enabled       bool     `json:"enabled"`                 // Enable activity feed
	IncludeEvents []string `json:"includeEvents,omitempty"` // Event types to include (empty = all)
	ExcludeEvents []string `json:"excludeEvents,omitempty"` // Event types to exclude
	MaxItems      int      `json:"maxItems,omitempty"`      // Maximum items in feed (default: 100)
}

// FavoritesConfig represents favorites/watchlist configuration.
type FavoritesConfig struct {
	Enabled      bool `json:"enabled"`                // Enable favorites
	Notify       bool `json:"notify,omitempty"`       // Notify on changes to favorited items
	MaxFavorites int  `json:"maxFavorites,omitempty"` // Maximum favorites per user (default: 100)
}

// ExportConfig represents data export configuration.
type ExportConfig struct {
	Enabled bool     `json:"enabled"`              // Enable export
	Formats []string `json:"formats,omitempty"`    // Allowed formats: csv, json, xlsx (default: ["csv", "json"])
	MaxRows int      `json:"maxRows,omitempty"`    // Maximum rows per export (default: 10000)
	Roles   []string `json:"roles,omitempty"`      // Roles that can export (empty = all authenticated)
}

// SoftDeleteConfig represents soft delete configuration.
type SoftDeleteConfig struct {
	Enabled       bool   `json:"enabled"`                 // Enable soft delete
	RetentionDays int    `json:"retentionDays,omitempty"` // Days to retain before permanent delete (0 = forever)
	RestoreRoles  []string `json:"restoreRoles,omitempty"`  // Roles that can restore deleted items
}
