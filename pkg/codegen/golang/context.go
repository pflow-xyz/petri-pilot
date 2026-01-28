package golang

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

// Context holds all data needed for code generation templates.
type Context struct {
	// Package configuration
	PackageName      string
	ModulePath       string
	LocalReplacePath string // Optional: path for replace directive in go.mod
	APISlug          string // URL-safe name for API paths, derived from model name

	// Model metadata
	ModelName        string
	ModelDescription string

	// Places and transitions
	Places      []PlaceContext
	Transitions []TransitionContext

	// Inferred types
	Events      []EventContext
	Routes      []RouteContext
	StateFields []StateFieldContext

	// ORM-specific data (for models with DataState places)
	Collections []CollectionContext
	DataArcs    []DataArcContext
	Guards      []GuardContext

	// Access control (Phase 11)
	AccessRules []AccessRuleContext
	Roles       []RoleContext

	// Views (Phase 13)
	Views []ViewContext

	// Workflow orchestration (Phase 12)
	Workflows []WorkflowContext

	// Webhook integrations
	Webhooks []WebhookContext

	// Navigation (Phase 14)
	Navigation *NavigationContext

	// Admin Dashboard (Phase 14)
	Admin *AdminContext

	// Event Sourcing (Phase 14)
	EventSourcing *EventSourcingContext

	// Debug configuration
	Debug *DebugContext

	// SLA configuration
	SLA *SLAConfigContext

	// Prediction configuration
	Prediction *PredictionContext

	// GraphQL configuration
	GraphQL *GraphQLContext

	// Blobstore configuration
	Blobstore *BlobstoreContext

	// Timers configuration
	Timers []TimerContext

	// Notifications configuration
	Notifications []NotificationContext

	// Relationships configuration
	Relationships []RelationshipContext

	// Computed fields configuration
	Computed []ComputedFieldContext

	// Indexes configuration
	Indexes []IndexContext

	// Approvals configuration
	Approvals map[string]*ApprovalChainContext

	// Templates configuration
	Templates []TemplateContext

	// Batch operations configuration
	Batch *BatchContext

	// Inbound webhooks configuration
	InboundWebhooks []InboundWebhookContext

	// Documents configuration
	Documents []DocumentContext

	// Comments configuration
	Comments *CommentsContext

	// Tags configuration
	Tags *TagsContext

	// Activity configuration
	Activity *ActivityContext

	// Favorites configuration
	Favorites *FavoritesContext

	// Export configuration
	Export *ExportContext

	// Soft delete configuration
	SoftDelete *SoftDeleteContext

	// Original model for reference
	Model *metamodel.Model

	// Schema JSON for schema viewer page
	SchemaJSON string
}

// WorkflowContext provides template-friendly access to workflow orchestration data.
type WorkflowContext struct {
	ID          string
	Name        string
	Description string
	PascalName  string // e.g., "TaskNotification"
	CamelName   string // e.g., "taskNotification"
	TriggerType string // event, schedule, manual
	Trigger     WorkflowTriggerContext
	Steps       []WorkflowStepContext
}

// WorkflowTriggerContext provides template-friendly access to workflow trigger data.
type WorkflowTriggerContext struct {
	Type   string // event, schedule, manual
	Entity string // Entity ID for event triggers
	Action string // Action ID for event triggers
	Cron   string // Cron expression for schedule triggers
}

// WorkflowStepContext provides template-friendly access to workflow step data.
type WorkflowStepContext struct {
	ID         string
	PascalName string
	Type       string            // action, condition, parallel, wait
	Entity     string            // Entity ID for action steps
	Action     string            // Action ID for action steps
	Condition  string            // Guard expression for condition steps
	Duration   string            // Duration for wait steps (e.g., "5m", "1h")
	Input      map[string]string // Input field mappings
	OnSuccess  string            // Next step ID on success
	OnFailure  string            // Next step ID on failure
}

// ViewContext provides template-friendly access to view definitions.
type ViewContext struct {
	ID          string
	Name        string
	Kind        string // form, card, table, detail
	Description string
	Groups      []ViewGroupContext
	Actions     []string // Transition IDs
}

// ViewGroupContext provides template-friendly access to view groups.
type ViewGroupContext struct {
	ID     string
	Name   string
	Fields []ViewFieldContext
}

// ViewFieldContext provides template-friendly access to view fields.
type ViewFieldContext struct {
	Binding     string
	Label       string
	Type        string // text, number, select, date, etc.
	Required    bool
	ReadOnly    bool
	Placeholder string
}

// AccessRuleContext provides template-friendly access to access control rules.
type AccessRuleContext struct {
	TransitionID string   // Transition this rule applies to
	Roles        []string // Required roles
	Guard        string   // Optional guard expression
	GuardGoCode  string   // Generated Go code for guard evaluation
	HasGuard     bool     // True if guard expression is present
}

// RoleContext provides template-friendly access to role definitions.
type RoleContext struct {
	ID              string
	Name            string
	Description     string
	Inherits        []string // Parent role IDs
	ConstName       string   // Go constant name (e.g., "RoleAdmin")
	AllRoles        []string // Flattened inheritance (this role + all inherited)
	DynamicGrant    string   // Expression to dynamically grant role (e.g., "balances[user.login] > 0")
	HasDynamicGrant bool     // True if DynamicGrant is set
}

// WebhookContext provides template-friendly access to webhook configuration.
type WebhookContext struct {
	ID          string
	URL         string
	Events      []string
	Secret      string
	Enabled     bool
	RetryPolicy *WebhookRetryPolicyContext
}

// WebhookRetryPolicyContext provides template-friendly access to retry policy.
type WebhookRetryPolicyContext struct {
	MaxAttempts int
	BackoffMs   int
}

// NavigationContext provides template-friendly access to navigation configuration.
type NavigationContext struct {
	Brand string
	Items []NavigationItemContext
}

// NavigationItemContext provides template-friendly access to navigation items.
type NavigationItemContext struct {
	Label string
	Path  string
	Icon  string
	Roles []string
}

// AdminContext provides template-friendly access to admin configuration.
type AdminContext struct {
	Enabled  bool
	Path     string
	Roles    []string
	Features []string
}

// EventSourcingContext provides template-friendly access to event sourcing configuration.
type EventSourcingContext struct {
	Snapshots *SnapshotConfigContext
	Retention *RetentionConfigContext
}

// SnapshotConfigContext provides template-friendly access to snapshot configuration.
type SnapshotConfigContext struct {
	Enabled   bool
	Frequency int
}

// RetentionConfigContext provides template-friendly access to retention configuration.
type RetentionConfigContext struct {
	Events    string
	Snapshots string
}

// DebugContext provides template-friendly access to debug configuration.
type DebugContext struct {
	Enabled bool
	Eval    bool
}

// SLAConfigContext provides template-friendly access to SLA configuration.
type SLAConfigContext struct {
	Default       string            // Default SLA duration string (e.g., "5m")
	ByPriority    map[string]string // Priority -> duration string
	WarningAt     float64           // Warning threshold (0.0-1.0), default 0.8
	CriticalAt    float64           // Critical threshold (0.0-1.0), default 0.95
	OnBreach      string            // Breach action: "alert", "log", "webhook"
	HasPriorities bool              // True if priority-based SLAs defined
}

// PredictionContext provides template-friendly access to prediction configuration.
type PredictionContext struct {
	Enabled   bool    // Enable ODE-based prediction
	TimeHours float64 // Default simulation duration in hours
	RateScale float64 // Rate scaling factor for numerical stability
}

// GraphQLContext provides template-friendly access to GraphQL configuration.
type GraphQLContext struct {
	Enabled    bool   // Enable GraphQL API
	Path       string // GraphQL endpoint path (default: "/graphql")
	Playground bool   // Enable GraphQL Playground
}

// BlobstoreContext provides template-friendly access to blobstore configuration.
type BlobstoreContext struct {
	Enabled      bool     // Enable blobstore for binary/JSON attachments
	MaxSize      int64    // Maximum blob size in bytes
	AllowedTypes []string // Allowed content types
}

// TimerContext provides template-friendly access to timer configuration.
type TimerContext struct {
	ID         string // Timer ID
	Transition string // Transition to fire
	After      string // Duration after entering state
	Cron       string // Cron expression
	From       string // Place that triggers the timer
	Condition  string // Condition expression
	Repeat     bool   // Whether to repeat
	PascalName string // e.g., "SendReminder"
}

// NotificationContext provides template-friendly access to notification configuration.
type NotificationContext struct {
	ID        string            // Notification ID
	On        string            // Trigger (transition or place)
	Channel   string            // email, sms, slack, webhook, in_app
	To        string            // Recipient expression
	Template  string            // Template ID or inline
	Subject   string            // Subject line
	Webhook   string            // Webhook URL
	Condition string            // Condition expression
	Data      map[string]string // Additional data
	PascalName string           // e.g., "ApprovalNotification"
}

// RelationshipContext provides template-friendly access to relationship configuration.
type RelationshipContext struct {
	Name        string // Relationship name
	Type        string // hasMany, hasOne, belongsTo
	Target      string // Target model name
	ForeignKey  string // Foreign key field
	Cascade     string // Cascade behavior
	PascalName  string // e.g., "LineItems"
	TargetPascal string // e.g., "OrderItem"
	IsHasMany   bool
	IsHasOne    bool
	IsBelongsTo bool
}

// ComputedFieldContext provides template-friendly access to computed field configuration.
type ComputedFieldContext struct {
	Name        string   // Field name
	Type        string   // Result type
	Expr        string   // Expression
	GoType      string   // Go type
	DependsOn   []string // Dependencies
	Persisted   bool     // Whether to store
	Description string   // Description
	PascalName  string   // e.g., "TotalAmount"
}

// IndexContext provides template-friendly access to index configuration.
type IndexContext struct {
	Name       string   // Index name
	Fields     []string // Fields to index
	Type       string   // Index type
	Unique     bool     // Unique index
	PascalName string   // e.g., "ByStatusCreatedAt"
}

// ApprovalChainContext provides template-friendly access to approval chain configuration.
type ApprovalChainContext struct {
	ID            string                 // Approval chain ID
	Levels        []ApprovalLevelContext // Approval levels
	EscalateAfter string                 // Escalation duration
	OnReject      string                 // Rejection transition
	OnApprove     string                 // Final approval transition
	Parallel      bool                   // Parallel approvals
	PascalName    string                 // e.g., "PurchaseRequest"
}

// ApprovalLevelContext provides template-friendly access to approval level configuration.
type ApprovalLevelContext struct {
	Role       string // Required role
	User       string // Specific user expression
	Condition  string // Level condition
	Required   int    // Approvals required
	Transition string // Custom transition
	Level      int    // Level number (1-indexed)
}

// TemplateContext provides template-friendly access to preset template configuration.
type TemplateContext struct {
	ID          string         // Template ID
	Name        string         // Display name
	Description string         // Description
	Data        map[string]any // Pre-filled data
	Roles       []string       // Allowed roles
	Default     bool           // Is default
	PascalName  string         // e.g., "StandardOrder"
	DataJSON    string         // JSON-encoded data
}

// BatchContext provides template-friendly access to batch operations configuration.
type BatchContext struct {
	Enabled     bool     // Enable batch operations
	Transitions []string // Allowed transitions
	MaxSize     int      // Maximum batch size
}

// InboundWebhookContext provides template-friendly access to inbound webhook configuration.
type InboundWebhookContext struct {
	ID         string            // Webhook ID
	Path       string            // URL path
	Secret     string            // Validation secret
	Transition string            // Transition to fire
	Map        map[string]string // Field mapping
	Condition  string            // Processing condition
	Method     string            // HTTP method
	PascalName string            // e.g., "StripePayment"
}

// DocumentContext provides template-friendly access to document generation configuration.
type DocumentContext struct {
	ID          string // Document ID
	Name        string // Display name
	Template    string // Template file
	Format      string // Output format
	Trigger     string // Trigger transition
	StoreTo     string // Storage blob field
	Filename    string // Filename expression
	Description string // Description
	PascalName  string // e.g., "Invoice"
}

// CommentsContext provides template-friendly access to comments configuration.
type CommentsContext struct {
	Enabled    bool     // Enable comments
	Roles      []string // Allowed roles
	Moderation bool     // Require moderation
	MaxLength  int      // Maximum length
}

// TagsContext provides template-friendly access to tags configuration.
type TagsContext struct {
	Enabled    bool     // Enable tags
	Predefined []string // Predefined tags
	FreeForm   bool     // Allow free-form tags
	MaxTags    int      // Maximum tags per instance
	Colors     bool     // Enable colors
}

// ActivityContext provides template-friendly access to activity feed configuration.
type ActivityContext struct {
	Enabled       bool     // Enable activity feed
	IncludeEvents []string // Events to include
	ExcludeEvents []string // Events to exclude
	MaxItems      int      // Maximum items
}

// FavoritesContext provides template-friendly access to favorites configuration.
type FavoritesContext struct {
	Enabled      bool // Enable favorites
	Notify       bool // Notify on changes
	MaxFavorites int  // Maximum favorites
}

// ExportContext provides template-friendly access to export configuration.
type ExportContext struct {
	Enabled bool     // Enable export
	Formats []string // Allowed formats
	MaxRows int      // Maximum rows
	Roles   []string // Allowed roles
}

// SoftDeleteContext provides template-friendly access to soft delete configuration.
type SoftDeleteContext struct {
	Enabled       bool     // Enable soft delete
	RetentionDays int      // Retention period
	RestoreRoles  []string // Roles that can restore
}

// PlaceContext provides template-friendly access to place data.
type PlaceContext struct {
	ID          string
	Description string
	Initial     int
	Kind        string // "token" or "data"
	Type        string // Go type
	IsToken     bool
	IsData      bool
	Persisted   bool
	Exported    bool

	// Resource tracking for prediction/simulation
	Capacity int  // Maximum tokens (for inventory modeling)
	Resource bool // True if this is a consumable resource

	// Computed names
	ConstName string // e.g., "PlaceReceived"
	FieldName string // e.g., "Received"
	VarName   string // e.g., "received"
}

// TransitionContext provides template-friendly access to transition data.
type TransitionContext struct {
	ID          string
	Description string
	Guard       string
	EventType   string
	EventRef    string // reference to Event.ID (Events First schema)
	HTTPMethod  string
	HTTPPath    string

	// Bindings for state computation (arcnet pattern)
	Bindings []BindingContext

	// Petri net connections (derived from arcs)
	Inputs  []ArcContext // Places that feed into this transition
	Outputs []ArcContext // Places that this transition feeds into

	// Data arcs for ORM patterns
	InputDataArcs  []DataArcContext // DataState input arcs
	OutputDataArcs []DataArcContext // DataState output arcs

	// Guard info (if present)
	GuardInfo *GuardContext

	// SLA timing fields
	Duration     string // Expected duration (e.g., "30s")
	MinDuration  string // Minimum expected duration
	MaxDuration  string // Maximum allowed duration (SLA breach)
	HasSLATiming bool   // True if any timing field is set

	// Prediction/simulation fields
	Rate float64 // Firing rate for ODE simulation (events/minute)

	// ClearsHistory when true causes this transition to delete all events,
	// resetting the aggregate to its initial state
	ClearsHistory bool

	// Computed names
	ConstName   string // e.g., "TransitionValidate"
	HandlerName string // e.g., "HandleValidate"
	EventName   string // e.g., "ValidatedEvent"
	FuncName    string // e.g., "Validate"
}

// ArcContext provides template-friendly access to arc data.
type ArcContext struct {
	PlaceID     string // The place ID
	ConstName   string // e.g., "PlaceReceived"
	Weight      int    // Token weight (default 1)
	IsInhibitor bool   // True if this is an inhibitor arc (blocks if place has tokens)
}

// BindingContext provides template-friendly access to transition bindings.
// Bindings are operational data needed for state computation (arcnet pattern).
type BindingContext struct {
	Name      string   // binding name (e.g., "from", "to", "amount")
	Type      string   // Go type (e.g., "string", "int64", "map[string]int64")
	FieldName string   // Go field name (e.g., "From", "Amount")
	JSONName  string   // JSON field name (e.g., "from", "amount")
	Keys      []string // map access path for nested lookups
	IsValue   bool     // true if this is the transfer value
	Place     string   // place ID this binding reads from/writes to
}

// EventContext provides template-friendly access to event data.
type EventContext struct {
	Type         string // Event type name (e.g., "OrderValidated")
	StructName   string // Go struct name (e.g., "OrderValidatedEvent")
	TransitionID string
	Fields       []EventFieldContext
}

// EventFieldContext provides template-friendly access to event fields.
type EventFieldContext struct {
	Name     string // Go field name (e.g., "Amount")
	Type     string // Go type (e.g., "int")
	JSONName string // JSON field name (e.g., "amount")
}

// RouteContext provides template-friendly access to API route data.
type RouteContext struct {
	Method       string // HTTP method
	Path         string // URL path
	Description  string
	TransitionID string
	HandlerName  string
	EventType    string
}

// StateFieldContext provides template-friendly access to aggregate state fields.
type StateFieldContext struct {
	Name      string // Place ID
	FieldName string // Go field name (e.g., "OrderReceived")
	Type      string // Go type
	IsToken   bool
	Persisted bool
	JSONName  string // JSON field name
}

// CollectionContext provides template-friendly access to DataState collections.
type CollectionContext struct {
	PlaceID       string // Original place ID
	Name          string // Go name (e.g., "Balances")
	FieldName     string // Go field name (e.g., "Balances")
	VarName       string // Go variable name (e.g., "balances")
	KeyType       string // Map key type (e.g., "string") - empty for simple types
	ValueType     string // Value type (e.g., "int64", "string")
	GoType        string // Full Go type (e.g., "map[string]int64" or "string")
	IsSimple      bool   // True for simple types (string, int64, bool)
	IsMap         bool   // True if this is a map type
	IsNested      bool   // True if this is a nested map
	NestedKeyType string // Key type of nested map (if IsNested)
	Description   string
	Exported      bool
	Initializer   string // Go initializer (e.g., "make(map[string]int64)" or `""`)
	ZeroValue     string // Go zero value (e.g., "0", `""`, "nil")
}

// DataArcContext provides template-friendly access to data arcs.
type DataArcContext struct {
	TransitionID     string   // Transition this arc belongs to
	PlaceID          string   // Collection place ID
	FieldName        string   // Go field name of collection
	ValueType        string   // Go type of the value (e.g., "int64", "string")
	IsSimple         bool     // True for simple types (direct assignment)
	Keys             []string // Key binding names - empty for simple types
	KeyFields        []string // Go field names for keys (e.g., ["From"])
	ValueBinding     string   // Value binding name (e.g., "amount" or "name")
	ValueField       string   // Go field name for value (e.g., "Amount")
	IsInput          bool     // True for input arcs (subtract/read)
	IsOutput         bool     // True for output arcs (add/write)
	IsNumeric        bool     // True if value is numeric (can use += / -=)
	UsesCompositeKey bool     // True if multiple keys are combined into a single string key
}

// GuardContext provides template-friendly access to guard conditions.
type GuardContext struct {
	TransitionID string // Transition this guard belongs to
	Expression   string // Original guard expression
	GoCode       string // Generated Go code (placeholder for complex guards)
	Collections  []string // Collections referenced by the guard
}

// Options for creating a new context.
type ContextOptions struct {
	ModulePath  string
	PackageName string
	// Access control (Phase 11)
	AccessRules []AccessRuleContext
	Roles       []RoleContext
	// Workflow orchestration (Phase 12)
	Workflows []WorkflowContext
	// Webhook integrations
	Webhooks []WebhookContext
}

// NewContext creates a Context from a model with computed template data.
func NewContext(model *metamodel.Model, opts ContextOptions) (*Context, error) {
	// Enrich the model with defaults
	enriched := metamodel.EnrichModel(model)

	// Determine package name
	packageName := opts.PackageName
	if packageName == "" {
		packageName = SanitizePackageName(enriched.Name)
	}

	// Determine module path
	modulePath := opts.ModulePath
	if modulePath == "" {
		modulePath = SanitizeModulePath(enriched.Name, "github.com/example")
	}

	// Check for local replace path from environment variable
	localReplacePath := os.Getenv("PETRI_PILOT_LOCAL_PATH")

	// APISlug is derived from model name for consistent API paths
	apiSlug := sanitizeAPISlug(enriched.Name)

	ctx := &Context{
		PackageName:      packageName,
		ModulePath:       modulePath,
		LocalReplacePath: localReplacePath,
		APISlug:          apiSlug,
		ModelName:        enriched.Name,
		ModelDescription: enriched.Description,
		Model:            enriched,
		AccessRules:      opts.AccessRules,
		Roles:            opts.Roles,
		Workflows:        opts.Workflows,
		Webhooks:         opts.Webhooks,
	}

	// Build place contexts
	ctx.Places = buildPlaceContexts(enriched.Places)

	// Build place ID set for quick lookups and track data state places
	placeIDs := make(map[string]bool)
	dataPlaceIDs := make(map[string]bool)
	for _, p := range enriched.Places {
		placeIDs[p.ID] = true
		if p.IsData() {
			dataPlaceIDs[p.ID] = true
		}
	}

	// Build transition contexts with arc information
	// Data state places are excluded from token counting (guards handle those)
	ctx.Transitions = buildTransitionContexts(enriched.Transitions, enriched.Arcs, enriched.Events, placeIDs, dataPlaceIDs)

	// Build event contexts from bridge inference
	eventDefs := metamodel.InferEvents(enriched)
	ctx.Events = buildEventContexts(eventDefs)

	// Build route contexts from bridge inference
	apiRoutes := metamodel.InferAPIRoutes(enriched)
	ctx.Routes = buildRouteContexts(apiRoutes)

	// Build state field contexts from bridge inference
	stateFields := metamodel.InferAggregateState(enriched)
	ctx.StateFields = buildStateFieldContexts(stateFields)

	// Build ORM-specific contexts
	ormSpec := bridge.ExtractORMSpec(enriched)
	ctx.Collections = buildCollectionContexts(ormSpec.Collections)
	ctx.DataArcs = buildDataArcContexts(ormSpec.Operations)
	ctx.Guards = buildGuardContexts(enriched.Transitions, ormSpec.Collections)

	// Populate data arcs and guard info on transitions
	for i := range ctx.Transitions {
		tid := ctx.Transitions[i].ID
		ctx.Transitions[i].InputDataArcs = ctx.InputDataArcs(tid)
		ctx.Transitions[i].OutputDataArcs = ctx.OutputDataArcs(tid)
		ctx.Transitions[i].GuardInfo = ctx.GuardForTransition(tid)
	}

	// Note: Application-level constructs (debug, admin, navigation, roles, access, views)
	// are now in extensions. Use NewContextFromApp with an ApplicationSpec for full support.
	// This function only handles core Petri net elements.

	// Serialize schema JSON for schema viewer (base64 encoded to avoid escaping issues)
	schemaBytes, err := json.MarshalIndent(enriched, "", "  ")
	if err != nil {
		return nil, err
	}
	ctx.SchemaJSON = base64.StdEncoding.EncodeToString(schemaBytes)

	return ctx, nil
}

// NewContextFromApp creates a Context from an ApplicationSpec.
// This uses the extension-based model where application constructs
// (roles, views, navigation, etc.) are stored in extensions rather
// than embedded in the Model.
func NewContextFromApp(app *extensions.ApplicationSpec, opts ContextOptions) (*Context, error) {
	if app == nil || app.Net == nil {
		return nil, nil
	}

	// Use the adapter to convert to legacy model for now
	// This preserves compatibility with existing template logic
	legacyModel := extensions.ToLegacyModel(app)

	// Create context using the legacy path
	ctx, err := NewContext(legacyModel, opts)
	if err != nil {
		return nil, err
	}

	// Override with extension data where available
	// This allows extensions to take precedence over legacy model fields

	// Roles from extension
	if rolesExt := app.Roles(); rolesExt != nil {
		ctx.Roles = buildRoleContextsFromExtension(rolesExt)
	}

	// Views from extension
	if viewsExt := app.Views(); viewsExt != nil {
		ctx.Views = buildViewContextsFromExtension(viewsExt)
		if viewsExt.Admin != nil {
			ctx.Admin = buildAdminContextFromExtension(viewsExt.Admin)
		}
	}

	// Navigation from extension
	if pagesExt := app.Pages(); pagesExt != nil && pagesExt.Navigation != nil {
		ctx.Navigation = buildNavigationContextFromExtension(pagesExt.Navigation)
	}

	// Access rules from entities extension
	if entitiesExt := app.Entities(); entitiesExt != nil {
		ctx.AccessRules = buildAccessRuleContextsFromEntities(entitiesExt)
	}

	// Debug config from app
	if app.HasDebug() {
		ctx.Debug = buildDebugContext(app.Debug())
	}

	// GraphQL config from app
	if app.HasGraphQL() {
		ctx.GraphQL = buildGraphQLContext(app.GraphQL())
	}

	return ctx, nil
}

// buildRoleContextsFromExtension converts extension roles to RoleContexts.
func buildRoleContextsFromExtension(ext *extensions.RoleExtension) []RoleContext {
	result := make([]RoleContext, len(ext.Roles))
	for i, r := range ext.Roles {
		// Flatten hierarchy
		allRoles := ext.FlattenHierarchy(r.ID)

		result[i] = RoleContext{
			ID:              r.ID,
			Name:            r.Name,
			Description:     r.Description,
			Inherits:        r.Inherits,
			ConstName:       ToConstName("Role", r.ID),
			AllRoles:        allRoles,
			DynamicGrant:    r.DynamicGrant,
			HasDynamicGrant: r.DynamicGrant != "",
		}
	}
	return result
}

// buildAccessRuleContextsFromEntities extracts access rules from entities.
func buildAccessRuleContextsFromEntities(ext *extensions.EntityExtension) []AccessRuleContext {
	var result []AccessRuleContext
	for _, entity := range ext.Entities {
		for _, rule := range entity.Access {
			result = append(result, AccessRuleContext{
				TransitionID: rule.Action,
				Roles:        rule.Roles,
				Guard:        rule.Guard,
				GuardGoCode:  GuardExpressionToGo(rule.Guard, "state", "bindings"),
				HasGuard:     rule.Guard != "",
			})
		}
	}
	return result
}

// buildViewContextsFromExtension converts extension views to ViewContexts.
func buildViewContextsFromExtension(ext *extensions.ViewExtension) []ViewContext {
	result := make([]ViewContext, len(ext.Views))
	for i, v := range ext.Views {
		groups := make([]ViewGroupContext, len(v.Groups))
		for j, g := range v.Groups {
			fields := make([]ViewFieldContext, len(g.Fields))
			for k, f := range g.Fields {
				fields[k] = ViewFieldContext{
					Binding:     f.Binding,
					Label:       f.Label,
					Type:        f.Type,
					Required:    f.Required,
					ReadOnly:    f.ReadOnly,
					Placeholder: f.Placeholder,
				}
			}
			groups[j] = ViewGroupContext{
				ID:     g.ID,
				Name:   g.Name,
				Fields: fields,
			}
		}
		result[i] = ViewContext{
			ID:          v.ID,
			Name:        v.Name,
			Kind:        v.Kind,
			Description: v.Description,
			Groups:      groups,
			Actions:     v.Actions,
		}
	}
	return result
}

// buildAdminContextFromExtension converts extension Admin to AdminContext.
func buildAdminContextFromExtension(admin *extensions.Admin) *AdminContext {
	if admin == nil {
		return nil
	}
	return &AdminContext{
		Enabled:  admin.Enabled,
		Path:     admin.Path,
		Roles:    admin.Roles,
		Features: admin.Features,
	}
}

// buildNavigationContextFromExtension converts extension Navigation to NavigationContext.
func buildNavigationContextFromExtension(nav *extensions.Navigation) *NavigationContext {
	if nav == nil {
		return nil
	}
	items := make([]NavigationItemContext, len(nav.Items))
	for i, item := range nav.Items {
		items[i] = NavigationItemContext{
			Label: item.Label,
			Path:  item.Path,
			Icon:  item.Icon,
			Roles: item.Roles,
		}
	}
	return &NavigationContext{
		Brand: nav.Brand,
		Items: items,
	}
}

// buildRoleContexts converts bridge RoleSpecs to RoleContexts.
func buildRoleContexts(roles []bridge.RoleSpec) []RoleContext {
	result := make([]RoleContext, len(roles))
	for i, r := range roles {
		result[i] = RoleContext{
			ID:              r.ID,
			Name:            r.Name,
			Description:     r.Description,
			Inherits:        r.Inherits,
			ConstName:       ToConstName("Role", r.ID),
			AllRoles:        r.AllRoles,
			DynamicGrant:    r.DynamicGrant,
			HasDynamicGrant: r.DynamicGrant != "",
		}
	}
	return result
}

// buildAccessRuleContexts converts bridge AccessRuleSpecs to AccessRuleContexts.
func buildAccessRuleContexts(rules []bridge.AccessRuleSpec) []AccessRuleContext {
	result := make([]AccessRuleContext, len(rules))
	for i, r := range rules {
		result[i] = AccessRuleContext{
			TransitionID: r.TransitionID,
			Roles:        r.Roles,
			Guard:        r.Guard,
			GuardGoCode:  GuardExpressionToGo(r.Guard, "state", "bindings"),
			HasGuard:     r.HasGuard,
		}
	}
	return result
}

// buildRoleContextsFromModel converts metamodel Roles to RoleContexts.
func buildRoleContextsFromModel(roles []metamodel.Role) []RoleContext {
	result := make([]RoleContext, len(roles))
	for i, r := range roles {
		// Compute all inherited roles
		allRoles := computeAllRoles(r.ID, roles)
		result[i] = RoleContext{
			ID:              r.ID,
			Name:            r.Name,
			Description:     r.Description,
			Inherits:        r.Inherits,
			ConstName:       ToConstName("Role", r.ID),
			AllRoles:        allRoles,
			DynamicGrant:    r.DynamicGrant,
			HasDynamicGrant: r.DynamicGrant != "",
		}
	}
	return result
}

// computeAllRoles returns all roles including inherited roles.
func computeAllRoles(roleID string, allRoleDefs []metamodel.Role) []string {
	seen := make(map[string]bool)
	var result []string

	var traverse func(id string)
	traverse = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		result = append(result, id)

		// Find this role's definition to get its inherits
		for _, r := range allRoleDefs {
			if r.ID == id {
				for _, parent := range r.Inherits {
					traverse(parent)
				}
				break
			}
		}
	}

	traverse(roleID)
	return result
}

// buildAccessRuleContextsFromModel converts metamodel AccessRules to AccessRuleContexts.
func buildAccessRuleContextsFromModel(rules []metamodel.AccessRule) []AccessRuleContext {
	result := make([]AccessRuleContext, len(rules))
	for i, r := range rules {
		result[i] = AccessRuleContext{
			TransitionID: r.Transition,
			Roles:        r.Roles,
			Guard:        r.Guard,
			GuardGoCode:  GuardExpressionToGo(r.Guard, "state", "bindings"),
			HasGuard:     r.Guard != "",
		}
	}
	return result
}

// buildViewContexts converts schema Views to ViewContexts.
func buildViewContexts(views []metamodel.View) []ViewContext {
	result := make([]ViewContext, len(views))
	for i, v := range views {
		groups := make([]ViewGroupContext, len(v.Groups))
		for j, g := range v.Groups {
			fields := make([]ViewFieldContext, len(g.Fields))
			for k, f := range g.Fields {
				fields[k] = ViewFieldContext{
					Binding:     f.Binding,
					Label:       f.Label,
					Type:        f.Type,
					Required:    f.Required,
					ReadOnly:    f.ReadOnly,
					Placeholder: f.Placeholder,
				}
			}
			groups[j] = ViewGroupContext{
				ID:     g.ID,
				Name:   g.Name,
				Fields: fields,
			}
		}
		result[i] = ViewContext{
			ID:          v.ID,
			Name:        v.Name,
			Kind:        v.Kind,
			Description: v.Description,
			Groups:      groups,
			Actions:     v.Actions,
		}
	}
	return result
}

func buildPlaceContexts(places []metamodel.Place) []PlaceContext {
	result := make([]PlaceContext, len(places))
	for i, p := range places {
		isToken := p.IsToken()
		goType := "int"
		if p.IsData() && p.Type != "" {
			goType = p.Type
		}

		result[i] = PlaceContext{
			ID:          p.ID,
			Description: p.Description,
			Initial:     p.Initial,
			Kind:        string(p.Kind),
			Type:        goType,
			IsToken:     isToken,
			IsData:      p.IsData(),
			Persisted:   p.Persisted,
			Exported:    p.Exported,
			Capacity:    p.Capacity,
			Resource:    p.Resource,
			ConstName:   ToConstName("Place", p.ID),
			FieldName:   ToFieldName(p.ID),
			VarName:     ToVarName(p.ID),
		}
	}
	return result
}

func buildTransitionContexts(transitions []metamodel.Transition, arcs []metamodel.Arc, events []metamodel.Event, placeIDs, dataPlaceIDs map[string]bool) []TransitionContext {
	// Build event lookup map for deriving bindings from event fields
	eventByID := make(map[string]metamodel.Event, len(events))
	for _, e := range events {
		eventByID[e.ID] = e
	}
	// Build arc maps for each transition
	// Inputs: arcs where arc.To == transition.ID (place -> transition)
	// Outputs: arcs where arc.From == transition.ID (transition -> place)
	// Note: Data state places are excluded from token counting - guards handle those
	// Inhibitor arcs are included in inputs but marked with IsInhibitor=true
	inputArcs := make(map[string][]ArcContext)
	outputArcs := make(map[string][]ArcContext)

	for _, arc := range arcs {
		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}

		// If arc goes from a place to something, and that something is not a place,
		// it's an input to a transition
		// Skip data state places - they don't use token counting
		if placeIDs[arc.From] && !placeIDs[arc.To] && !dataPlaceIDs[arc.From] {
			inputArcs[arc.To] = append(inputArcs[arc.To], ArcContext{
				PlaceID:     arc.From,
				ConstName:   ToConstName("Place", arc.From),
				Weight:      weight,
				IsInhibitor: arc.IsInhibitor(),
			})
		}

		// If arc goes from something that's not a place to a place,
		// it's an output from a transition
		// Skip data state places - they don't use token counting
		// Note: Inhibitor arcs don't have outputs (they're read-only)
		if !placeIDs[arc.From] && placeIDs[arc.To] && !dataPlaceIDs[arc.To] && !arc.IsInhibitor() {
			outputArcs[arc.From] = append(outputArcs[arc.From], ArcContext{
				PlaceID:     arc.To,
				ConstName:   ToConstName("Place", arc.To),
				Weight:      weight,
				IsInhibitor: false,
			})
		}
	}

	result := make([]TransitionContext, len(transitions))
	for i, t := range transitions {
		eventType := t.EventType
		if eventType == "" {
			eventType = ToEventTypeName(t.ID)
		}

		// Build binding contexts from explicit bindings or event fields
		var bindings []BindingContext
		if len(t.Bindings) > 0 {
			bindings = make([]BindingContext, len(t.Bindings))
			for j, b := range t.Bindings {
				bindings[j] = BindingContext{
					Name:      b.Name,
					Type:      bindingTypeToGo(b.Type),
					FieldName: ToPascalCase(b.Name),
					JSONName:  b.Name,
					Keys:      b.Keys,
					IsValue:   b.Value,
					Place:     b.Place,
				}
			}
		} else if t.Event != "" && len(t.Fields) > 0 {
			// Fall back to event fields when transition has UI fields but no explicit bindings.
			// The presence of transition fields indicates user input is expected.
			// Transitions without fields (like positional moves) should not get bindings
			// even if their event has fields.
			if evt, ok := eventByID[t.Event]; ok {
				bindings = make([]BindingContext, 0, len(evt.Fields))
				for _, f := range evt.Fields {
					bindings = append(bindings, BindingContext{
						Name:      f.Name,
						Type:      eventFieldTypeToGo(f.Type),
						FieldName: ToPascalCase(f.Name),
						JSONName:  f.Name,
					})
				}
			}
		}

		// Check if transition has SLA timing
		hasSLATiming := t.Duration != "" || t.MinDuration != "" || t.MaxDuration != ""

		result[i] = TransitionContext{
			ID:           t.ID,
			Description:  t.Description,
			Guard:        t.Guard,
			EventType:    eventType,
			EventRef:     t.Event,
			HTTPMethod:   t.HTTPMethod,
			HTTPPath:     t.HTTPPath,
			Bindings:     bindings,
			Inputs:       inputArcs[t.ID],
			Outputs:      outputArcs[t.ID],
			Duration:     t.Duration,
			MinDuration:  t.MinDuration,
			MaxDuration:  t.MaxDuration,
			HasSLATiming:  hasSLATiming,
			Rate:          t.Rate,
			ClearsHistory: t.ClearsHistory,
			ConstName:     ToConstName("Transition", t.ID),
			HandlerName:  ToHandlerName(t.ID),
			EventName:    ToEventStructName(eventType),
			FuncName:     ToPascalCase(t.ID),
		}
	}
	return result
}

// bindingTypeToGo converts schema binding types to Go types.
func bindingTypeToGo(typ string) string {
	switch typ {
	case "string":
		return "string"
	case "number":
		return "float64"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "time":
		return "time.Time"
	default:
		// Pass through Go types and map types as-is
		return typ
	}
}

// eventFieldTypeToGo converts an event field type to a Go type string.
// Event fields use Go-style types (string, int64, []string) directly.
func eventFieldTypeToGo(typ string) string {
	switch typ {
	case "":
		return "string"
	case "number":
		return "float64"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	default:
		return typ
	}
}

// sanitizeAPISlug converts a model name to a URL-safe slug for API paths.
// This removes hyphens, underscores, and spaces to create a consistent identifier.
func sanitizeAPISlug(name string) string {
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, "-", "")
	result = strings.ReplaceAll(result, "_", "")
	result = strings.ReplaceAll(result, " ", "")
	return result
}

func buildEventContexts(eventDefs []metamodel.EventDef) []EventContext {
	result := make([]EventContext, len(eventDefs))
	for i, e := range eventDefs {
		fields := make([]EventFieldContext, len(e.Fields))
		for j, f := range e.Fields {
			fields[j] = EventFieldContext{
				Name:     ToPascalCase(f.Name),
				Type:     ToTypeName(f.Type),
				JSONName: f.Name,
			}
		}

		result[i] = EventContext{
			Type:         e.Type,
			StructName:   ToEventStructName(e.Type),
			TransitionID: e.TransitionID,
			Fields:       fields,
		}
	}
	return result
}

func buildRouteContexts(apiRoutes []metamodel.APIRoute) []RouteContext {
	result := make([]RouteContext, len(apiRoutes))
	for i, r := range apiRoutes {
		result[i] = RouteContext{
			Method:       r.Method,
			Path:         r.Path,
			Description:  r.Description,
			TransitionID: r.TransitionID,
			HandlerName:  ToHandlerName(r.TransitionID),
			EventType:    r.EventType,
		}
	}
	return result
}

func buildStateFieldContexts(stateFields []metamodel.StateField) []StateFieldContext {
	result := make([]StateFieldContext, len(stateFields))
	for i, f := range stateFields {
		result[i] = StateFieldContext{
			Name:      f.Name,
			FieldName: ToPascalCase(f.Name),
			Type:      ToTypeName(f.Type),
			IsToken:   f.IsToken,
			Persisted: f.Persisted,
			JSONName:  f.Name,
		}
	}
	return result
}

func buildCollectionContexts(collections []bridge.CollectionSpec) []CollectionContext {
	result := make([]CollectionContext, len(collections))
	for i, c := range collections {
		var goType string
		if c.IsSimple {
			goType = TypeToGo(c.ValueType)
		} else if c.IsMap {
			goType = "map[" + TypeToGo(c.KeyType) + "]" + TypeToGo(c.ValueType)
			if c.IsNested {
				goType = "map[" + TypeToGo(c.KeyType) + "]map[" + TypeToGo(c.NestedKeyType) + "]" + TypeToGo(c.ValueType)
			}
		} else {
			goType = TypeToGo(c.ValueType)
		}

		// Determine initializer based on type
		var initializer string
		if c.IsSimple {
			initializer = GoZeroValue(goType)
		} else {
			initializer = GoMapInitializer(goType)
		}

		result[i] = CollectionContext{
			PlaceID:       c.PlaceID,
			Name:          c.Name,
			FieldName:     ToPascalCase(c.PlaceID),
			VarName:       ToCamelCase(c.PlaceID),
			KeyType:       TypeToGo(c.KeyType),
			ValueType:     TypeToGo(c.ValueType),
			GoType:        goType,
			IsSimple:      c.IsSimple,
			IsMap:         c.IsMap,
			IsNested:      c.IsNested,
			NestedKeyType: TypeToGo(c.NestedKeyType),
			Description:   c.Description,
			Exported:      c.Exported,
			Initializer:   initializer,
			ZeroValue:     GoZeroValue(goType),
		}
	}
	return result
}

// extractFinalValueType recursively extracts the final value type from a map type.
// For nested maps like map[string]map[string]int64, returns int64.
// For simple maps like map[string]int64, returns int64.
// For non-map types, returns the type itself.
func extractFinalValueType(typ string) string {
	_, vt, isMap := ParseMapType(typ)
	if !isMap {
		return TypeToGo(typ)
	}
	// Recursively extract if nested
	return extractFinalValueType(vt)
}

func buildDataArcContexts(operations []bridge.OperationSpec) []DataArcContext {
	var result []DataArcContext

	for _, op := range operations {
		// Process read arcs (inputs)
		for _, read := range op.Reads {
			keyFields := make([]string, len(read.Keys))
			for i, k := range read.Keys {
				keyFields[i] = ToPascalCase(k)
			}

			// For checking IsNumeric, we need the final value type (for nested maps)
			finalValueType := extractFinalValueType(read.CollectionType)
			// For the ValueType field, we need the immediate value type for codegen
			valueType := TypeToGo(read.CollectionType)
			if !read.IsSimple {
				// For maps, the value type is the map's value type
				_, vt, _ := ParseMapType(read.CollectionType)
				valueType = TypeToGo(vt)
			}

			// Use composite key when we have 2+ keys but the type is not a nested map
			usesCompositeKey := len(read.Keys) > 1 && !IsNestedMap(read.CollectionType)

			result = append(result, DataArcContext{
				TransitionID:     op.TransitionID,
				PlaceID:          read.Collection,
				FieldName:        ToPascalCase(read.Collection),
				ValueType:        valueType,
				IsSimple:         read.IsSimple,
				Keys:             read.Keys,
				KeyFields:        keyFields,
				ValueBinding:     read.ValueBinding,
				ValueField:       ToPascalCase(read.ValueBinding),
				IsInput:          true,
				IsOutput:         false,
				IsNumeric:        IsNumericType(finalValueType),
				UsesCompositeKey: usesCompositeKey,
			})
		}

		// Process write arcs (outputs)
		for _, write := range op.Writes {
			keyFields := make([]string, len(write.Keys))
			for i, k := range write.Keys {
				keyFields[i] = ToPascalCase(k)
			}

			// For checking IsNumeric, we need the final value type (for nested maps)
			finalValueType := extractFinalValueType(write.CollectionType)
			// For the ValueType field, we need the immediate value type for codegen
			valueType := TypeToGo(write.CollectionType)
			if !write.IsSimple {
				// For maps, the value type is the map's value type
				_, vt, _ := ParseMapType(write.CollectionType)
				valueType = TypeToGo(vt)
			}

			// Use composite key when we have 2+ keys but the type is not a nested map
			usesCompositeKey := len(write.Keys) > 1 && !IsNestedMap(write.CollectionType)

			result = append(result, DataArcContext{
				TransitionID:     op.TransitionID,
				PlaceID:          write.Collection,
				FieldName:        ToPascalCase(write.Collection),
				ValueType:        valueType,
				IsSimple:         write.IsSimple,
				Keys:             write.Keys,
				KeyFields:        keyFields,
				ValueBinding:     write.ValueBinding,
				ValueField:       ToPascalCase(write.ValueBinding),
				IsInput:          false,
				IsOutput:         true,
				IsNumeric:        IsNumericType(finalValueType),
				UsesCompositeKey: usesCompositeKey,
			})
		}
	}

	return result
}

func buildGuardContexts(transitions []metamodel.Transition, collections []bridge.CollectionSpec) []GuardContext {
	var result []GuardContext

	// Build collection lookup
	collectionIDs := make(map[string]bool)
	for _, c := range collections {
		collectionIDs[c.PlaceID] = true
	}

	for _, t := range transitions {
		if t.Guard == "" {
			continue
		}

		// Find collections referenced in the guard
		var referencedCollections []string
		for _, c := range collections {
			if containsIdentifier(t.Guard, c.PlaceID) {
				referencedCollections = append(referencedCollections, c.PlaceID)
			}
		}

		result = append(result, GuardContext{
			TransitionID: t.ID,
			Expression:   t.Guard,
			GoCode:       GuardExpressionToGo(t.Guard, "state", "bindings"),
			Collections:  referencedCollections,
		})
	}

	return result
}

// containsIdentifier checks if an expression contains a specific identifier.
// This is a simple check - a full implementation would use a proper parser.
func containsIdentifier(expr, identifier string) bool {
	// Simple substring check for now
	// A proper implementation would parse the expression
	return len(identifier) > 0 && len(expr) > 0 &&
		(expr == identifier ||
			containsWord(expr, identifier))
}

// containsWord checks if expr contains identifier as a word (not part of another word).
func containsWord(expr, word string) bool {
	for i := 0; i <= len(expr)-len(word); i++ {
		if expr[i:i+len(word)] == word {
			// Check that it's a word boundary
			before := i == 0 || !isIdentChar(rune(expr[i-1]))
			after := i+len(word) >= len(expr) || !isIdentChar(rune(expr[i+len(word)]))
			if before && after {
				return true
			}
		}
	}
	return false
}

func isIdentChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// InitialPlaces returns a map of place IDs to their initial token counts.
func (c *Context) InitialPlaces() map[string]int {
	result := make(map[string]int)
	for _, p := range c.Places {
		if p.Initial > 0 {
			result[p.ID] = p.Initial
		}
	}
	return result
}

// HasDataPlaces returns true if the model has any data places.
func (c *Context) HasDataPlaces() bool {
	for _, p := range c.Places {
		if p.IsData {
			return true
		}
	}
	return false
}

// HasGuards returns true if any transition has a guard condition.
func (c *Context) HasGuards() bool {
	for _, t := range c.Transitions {
		if t.Guard != "" {
			return true
		}
	}
	return false
}

// HasCollections returns true if the model has any DataState collections.
func (c *Context) HasCollections() bool {
	return len(c.Collections) > 0
}

// HasDataArcs returns true if any transition has data arcs.
func (c *Context) HasDataArcs() bool {
	return len(c.DataArcs) > 0
}

// HasNestedMaps returns true if any collection uses nested maps.
func (c *Context) HasNestedMaps() bool {
	for _, coll := range c.Collections {
		if coll.IsNested {
			return true
		}
	}
	return false
}

// CollectionByID returns a collection by its place ID.
func (c *Context) CollectionByID(placeID string) *CollectionContext {
	for i := range c.Collections {
		if c.Collections[i].PlaceID == placeID {
			return &c.Collections[i]
		}
	}
	return nil
}

// DataArcsForTransition returns all data arcs for a transition.
func (c *Context) DataArcsForTransition(transitionID string) []DataArcContext {
	var result []DataArcContext
	for _, arc := range c.DataArcs {
		if arc.TransitionID == transitionID {
			result = append(result, arc)
		}
	}
	return result
}

// InputDataArcs returns input data arcs for a transition.
func (c *Context) InputDataArcs(transitionID string) []DataArcContext {
	var result []DataArcContext
	for _, arc := range c.DataArcs {
		if arc.TransitionID == transitionID && arc.IsInput {
			result = append(result, arc)
		}
	}
	return result
}

// OutputDataArcs returns output data arcs for a transition.
func (c *Context) OutputDataArcs(transitionID string) []DataArcContext {
	var result []DataArcContext
	for _, arc := range c.DataArcs {
		if arc.TransitionID == transitionID && arc.IsOutput {
			result = append(result, arc)
		}
	}
	return result
}

// GuardForTransition returns the guard context for a transition, or nil.
func (c *Context) GuardForTransition(transitionID string) *GuardContext {
	for i := range c.Guards {
		if c.Guards[i].TransitionID == transitionID {
			return &c.Guards[i]
		}
	}
	return nil
}

// UsesMetamodelRuntime returns true if the generated code should use
// go-pflow's metamodel.Runtime for execution.
func (c *Context) UsesMetamodelRuntime() bool {
	// Use metamodel runtime when we have data places or guards
	return c.HasDataPlaces() || c.HasGuards()
}

// HasWorkflows returns true if the context has any workflows defined.
func (c *Context) HasWorkflows() bool {
	return len(c.Workflows) > 0
}

// HasAccessControl returns true if any access rules or roles are defined.
func (c *Context) HasAccessControl() bool {
	return len(c.AccessRules) > 0 || len(c.Roles) > 0
}

// HasRoles returns true if any roles are defined.
func (c *Context) HasRoles() bool {
	return len(c.Roles) > 0
}

// TransitionRequiresAuth returns true if a transition has access control rules.
func (c *Context) TransitionRequiresAuth(transitionID string) bool {
	for _, rule := range c.AccessRules {
		if rule.TransitionID == transitionID {
			return true
		}
	}
	return false
}

// TransitionHasDynamicRoles returns true if a transition's access control involves roles with dynamic grants.
func (c *Context) TransitionHasDynamicRoles(transitionID string) bool {
	// Find the access rule for this transition
	for _, rule := range c.AccessRules {
		if rule.TransitionID == transitionID {
			// Check if any of the required roles have dynamic grants
			for _, roleID := range rule.Roles {
				for _, role := range c.Roles {
					if role.ID == roleID && role.HasDynamicGrant {
						return true
					}
				}
			}
		}
	}
	// Also check if any role has dynamic grants (for transitions with empty role lists that use dynamic checking)
	for _, role := range c.Roles {
		if role.HasDynamicGrant {
			// If there's an access rule for this transition, it might use dynamic role checking
			for _, rule := range c.AccessRules {
				if rule.TransitionID == transitionID && len(rule.Roles) == 0 {
					return true
				}
			}
		}
	}
	return false
}

// HasWebhooks returns true if the context has any webhooks defined.
func (c *Context) HasWebhooks() bool {
	return len(c.Webhooks) > 0
}

// HasViews returns true if the context has any views defined.
func (c *Context) HasViews() bool {
	return len(c.Views) > 0
}

// buildNavigationContext converts metamodel.Navigation to NavigationContext.
func buildNavigationContext(nav *metamodel.Navigation) *NavigationContext {
if nav == nil {
return nil
}

items := make([]NavigationItemContext, len(nav.Items))
for i, item := range nav.Items {
items[i] = NavigationItemContext{
Label: item.Label,
Path:  item.Path,
Icon:  item.Icon,
Roles: item.Roles,
}
}

return &NavigationContext{
Brand: nav.Brand,
Items: items,
}
}

// buildAdminContext converts metamodel.Admin to AdminContext.
func buildAdminContext(admin *metamodel.Admin) *AdminContext {
if admin == nil {
return nil
}

return &AdminContext{
Enabled:  admin.Enabled,
Path:     admin.Path,
Roles:    admin.Roles,
Features: admin.Features,
}
}

// buildEventSourcingContext converts metamodel.EventSourcing to EventSourcingContext.
func buildEventSourcingContext(es *metamodel.EventSourcingConfig) *EventSourcingContext {
if es == nil {
return nil
}

ctx := &EventSourcingContext{}

if es.Snapshots != nil {
ctx.Snapshots = &SnapshotConfigContext{
Enabled:   es.Snapshots.Enabled,
Frequency: es.Snapshots.Frequency,
}
}

if es.Retention != nil {
ctx.Retention = &RetentionConfigContext{
Events:    es.Retention.Events,
Snapshots: es.Retention.Snapshots,
}
}

return ctx
}

// buildDebugContext converts metamodel.Debug to DebugContext.
func buildDebugContext(debug *metamodel.Debug) *DebugContext {
	if debug == nil {
		return nil
	}

	return &DebugContext{
		Enabled: debug.Enabled,
		Eval:    debug.Eval,
	}
}

// HasNavigation returns true if the model has navigation configuration.
func (c *Context) HasNavigation() bool {
return c.Navigation != nil
}

// HasAdmin returns true if the model has admin dashboard configuration.
func (c *Context) HasAdmin() bool {
return c.Admin != nil && c.Admin.Enabled
}

// HasEventSourcing returns true if event sourcing is enabled.
// Always returns true since the runtime always uses event sourcing.
func (c *Context) HasEventSourcing() bool {
	return true
}

// HasSnapshots returns true if automatic snapshots are enabled.
func (c *Context) HasSnapshots() bool {
return c.EventSourcing != nil && c.EventSourcing.Snapshots != nil && c.EventSourcing.Snapshots.Enabled
}

// HasDebug returns true if debug mode is enabled.
func (c *Context) HasDebug() bool {
	return c.Debug != nil && c.Debug.Enabled
}

// HasExplicitEvents returns true if the model has explicit event definitions.
func (c *Context) HasExplicitEvents() bool {
	return c.Model != nil && len(c.Model.Events) > 0
}

// HasSLAs returns true if the model has SLA configuration.
func (c *Context) HasSLAs() bool {
	return c.SLA != nil
}

// HasTransitionSLAs returns true if any transition has SLA timing.
func (c *Context) HasTransitionSLAs() bool {
	for _, t := range c.Transitions {
		if t.HasSLATiming {
			return true
		}
	}
	return false
}

// HasClearsHistoryTransitions returns true if any transition clears event history.
func (c *Context) HasClearsHistoryTransitions() bool {
	for _, t := range c.Transitions {
		if t.ClearsHistory {
			return true
		}
	}
	return false
}

// HasPrediction returns true if the model has prediction configuration enabled.
func (c *Context) HasPrediction() bool {
	return c.Prediction != nil && c.Prediction.Enabled
}

// HasGraphQL returns true if the model has GraphQL enabled.
func (c *Context) HasGraphQL() bool {
	return c.GraphQL != nil && c.GraphQL.Enabled
}

// HasPlayground returns true if GraphQL Playground is enabled.
func (c *Context) HasPlayground() bool {
	return c.HasGraphQL() && c.GraphQL.Playground
}

// HasBlobstore returns true if the model has blobstore enabled.
func (c *Context) HasBlobstore() bool {
	return c.Blobstore != nil && c.Blobstore.Enabled
}

// HasTimers returns true if the model has timers configured.
func (c *Context) HasTimers() bool {
	return len(c.Timers) > 0
}

// HasNotifications returns true if the model has notifications configured.
func (c *Context) HasNotifications() bool {
	return len(c.Notifications) > 0
}

// HasRelationships returns true if the model has relationships configured.
func (c *Context) HasRelationships() bool {
	return len(c.Relationships) > 0
}

// HasComputed returns true if the model has computed fields configured.
func (c *Context) HasComputed() bool {
	return len(c.Computed) > 0
}

// HasIndexes returns true if the model has indexes configured.
func (c *Context) HasIndexes() bool {
	return len(c.Indexes) > 0
}

// HasApprovals returns true if the model has approval chains configured.
func (c *Context) HasApprovals() bool {
	return len(c.Approvals) > 0
}

// HasTemplates returns true if the model has templates configured.
func (c *Context) HasTemplates() bool {
	return len(c.Templates) > 0
}

// HasBatch returns true if batch operations are enabled.
func (c *Context) HasBatch() bool {
	return c.Batch != nil && c.Batch.Enabled
}

// HasInboundWebhooks returns true if the model has inbound webhooks configured.
func (c *Context) HasInboundWebhooks() bool {
	return len(c.InboundWebhooks) > 0
}

// HasDocuments returns true if the model has document generation configured.
func (c *Context) HasDocuments() bool {
	return len(c.Documents) > 0
}

// HasComments returns true if comments are enabled.
func (c *Context) HasComments() bool {
	return c.Comments != nil && c.Comments.Enabled
}

// HasTags returns true if tags are enabled.
func (c *Context) HasTags() bool {
	return c.Tags != nil && c.Tags.Enabled
}

// HasActivity returns true if activity feed is enabled.
func (c *Context) HasActivity() bool {
	return c.Activity != nil && c.Activity.Enabled
}

// HasFavorites returns true if favorites are enabled.
func (c *Context) HasFavorites() bool {
	return c.Favorites != nil && c.Favorites.Enabled
}

// HasExport returns true if export is enabled.
func (c *Context) HasExport() bool {
	return c.Export != nil && c.Export.Enabled
}

// HasSoftDelete returns true if soft delete is enabled.
func (c *Context) HasSoftDelete() bool {
	return c.SoftDelete != nil && c.SoftDelete.Enabled
}

// HasAnyFeatures returns true if any of the higher-level features are enabled.
func (c *Context) HasAnyFeatures() bool {
	return c.HasTimers() || c.HasNotifications() || c.HasRelationships() ||
		c.HasComputed() || c.HasIndexes() || c.HasApprovals() ||
		c.HasTemplates() || c.HasBatch() || c.HasInboundWebhooks() ||
		c.HasDocuments() || c.HasComments() || c.HasTags() ||
		c.HasActivity() || c.HasFavorites() || c.HasExport() || c.HasSoftDelete()
}

// ResourcePlaces returns only the places marked as resources for prediction.
func (c *Context) ResourcePlaces() []PlaceContext {
	var result []PlaceContext
	for _, p := range c.Places {
		if p.Resource {
			result = append(result, p)
		}
	}
	return result
}

// buildSLAContext converts metamodel.SLAConfig to SLAConfigContext.
func buildSLAContext(sla *metamodel.SLAConfig) *SLAConfigContext {
	if sla == nil {
		return nil
	}

	ctx := &SLAConfigContext{
		Default:    sla.Default,
		ByPriority: sla.ByPriority,
		WarningAt:  sla.WarningAt,
		CriticalAt: sla.CriticalAt,
		OnBreach:   sla.OnBreach,
	}

	// Set default thresholds if not specified
	if ctx.WarningAt == 0 {
		ctx.WarningAt = 0.8 // Default: 80%
	}
	if ctx.CriticalAt == 0 {
		ctx.CriticalAt = 0.95 // Default: 95%
	}

	// Check if priorities are defined
	ctx.HasPriorities = len(sla.ByPriority) > 0

	return ctx
}

// buildPredictionContext converts metamodel.PredictionConfig to PredictionContext.
func buildPredictionContext(pred *metamodel.PredictionConfig) *PredictionContext {
	if pred == nil {
		return nil
	}

	ctx := &PredictionContext{
		Enabled:   pred.Enabled,
		TimeHours: pred.TimeHours,
		RateScale: pred.RateScale,
	}

	// Set default values if not specified
	if ctx.TimeHours == 0 {
		ctx.TimeHours = 8.0 // Default: 8 hours
	}
	if ctx.RateScale == 0 {
		ctx.RateScale = 0.0001 // Default: from go-pflow
	}

	return ctx
}

// buildGraphQLContext converts metamodel.GraphQL to GraphQLContext.
func buildGraphQLContext(gql *metamodel.GraphQLConfig) *GraphQLContext {
	if gql == nil {
		return nil
	}

	ctx := &GraphQLContext{
		Enabled:    gql.Enabled,
		Path:       gql.Path,
		Playground: gql.Playground,
	}

	// Set default values if not specified
	if ctx.Path == "" {
		ctx.Path = "/graphql"
	}

	return ctx
}

// buildBlobstoreContext converts metamodel.Blobstore to BlobstoreContext.
func buildBlobstoreContext(bs *metamodel.BlobstoreConfig) *BlobstoreContext {
	if bs == nil {
		return nil
	}

	ctx := &BlobstoreContext{
		Enabled:      bs.Enabled,
		MaxSize:      bs.MaxSize,
		AllowedTypes: bs.AllowedTypes,
	}

	// Set default values if not specified
	if ctx.MaxSize == 0 {
		ctx.MaxSize = 10 * 1024 * 1024 // Default: 10MB
	}
	if len(ctx.AllowedTypes) == 0 {
		ctx.AllowedTypes = []string{"application/json", "*/*"}
	}

	return ctx
}

// buildTimersContext converts metamodel.Timer slice to TimerContext slice.
func buildTimersContext(timers []metamodel.Timer) []TimerContext {
	if len(timers) == 0 {
		return nil
	}
	result := make([]TimerContext, len(timers))
	for i, t := range timers {
		id := t.ID
		if id == "" {
			id = t.Transition
		}
		result[i] = TimerContext{
			ID:         id,
			Transition: t.Transition,
			After:      t.After,
			Cron:       t.Cron,
			From:       t.From,
			Condition:  t.Condition,
			Repeat:     t.Repeat,
			PascalName: ToPascalCase(t.Transition),
		}
	}
	return result
}

// buildNotificationsContext converts metamodel.Notification slice to NotificationContext slice.
func buildNotificationsContext(notifications []metamodel.Notification) []NotificationContext {
	if len(notifications) == 0 {
		return nil
	}
	result := make([]NotificationContext, len(notifications))
	for i, n := range notifications {
		id := n.ID
		if id == "" {
			id = n.On + "_" + n.Channel
		}
		result[i] = NotificationContext{
			ID:         id,
			On:         n.On,
			Channel:    n.Channel,
			To:         n.To,
			Template:   n.Template,
			Subject:    n.Subject,
			Webhook:    n.Webhook,
			Condition:  n.Condition,
			Data:       n.Data,
			PascalName: ToPascalCase(id),
		}
	}
	return result
}

// buildRelationshipsContext converts metamodel.Relationship slice to RelationshipContext slice.
func buildRelationshipsContext(relationships []metamodel.Relationship) []RelationshipContext {
	if len(relationships) == 0 {
		return nil
	}
	result := make([]RelationshipContext, len(relationships))
	for i, r := range relationships {
		result[i] = RelationshipContext{
			Name:         r.Name,
			Type:         r.Type,
			Target:       r.Target,
			ForeignKey:   r.ForeignKey,
			Cascade:      r.Cascade,
			PascalName:   ToPascalCase(r.Name),
			TargetPascal: ToPascalCase(r.Target),
			IsHasMany:    r.Type == "hasMany",
			IsHasOne:     r.Type == "hasOne",
			IsBelongsTo:  r.Type == "belongsTo",
		}
	}
	return result
}

// buildComputedContext converts metamodel.ComputedField slice to ComputedFieldContext slice.
func buildComputedContext(computed []metamodel.ComputedField) []ComputedFieldContext {
	if len(computed) == 0 {
		return nil
	}
	result := make([]ComputedFieldContext, len(computed))
	for i, c := range computed {
		goType := "any"
		switch c.Type {
		case "string":
			goType = "string"
		case "number":
			goType = "float64"
		case "boolean":
			goType = "bool"
		case "array":
			goType = "[]any"
		}
		result[i] = ComputedFieldContext{
			Name:        c.Name,
			Type:        c.Type,
			Expr:        c.Expr,
			GoType:      goType,
			DependsOn:   c.DependsOn,
			Persisted:   c.Persisted,
			Description: c.Description,
			PascalName:  ToPascalCase(c.Name),
		}
	}
	return result
}

// buildIndexesContext converts metamodel.Index slice to IndexContext slice.
func buildIndexesContext(indexes []metamodel.Index) []IndexContext {
	if len(indexes) == 0 {
		return nil
	}
	result := make([]IndexContext, len(indexes))
	for i, idx := range indexes {
		name := idx.Name
		if name == "" {
			name = "idx_" + indexes[i].Fields[0]
		}
		result[i] = IndexContext{
			Name:       name,
			Fields:     idx.Fields,
			Type:       idx.Type,
			Unique:     idx.Unique,
			PascalName: ToPascalCase(name),
		}
	}
	return result
}

// buildApprovalsContext converts schema approval chains to ApprovalChainContext map.
func buildApprovalsContext(approvals map[string]*metamodel.ApprovalChain) map[string]*ApprovalChainContext {
	if len(approvals) == 0 {
		return nil
	}
	result := make(map[string]*ApprovalChainContext)
	for id, chain := range approvals {
		levels := make([]ApprovalLevelContext, len(chain.Levels))
		for j, lvl := range chain.Levels {
			levels[j] = ApprovalLevelContext{
				Role:       lvl.Role,
				User:       lvl.User,
				Condition:  lvl.Condition,
				Required:   lvl.Required,
				Transition: lvl.Transition,
				Level:      j + 1,
			}
			if levels[j].Required == 0 {
				levels[j].Required = 1
			}
		}
		result[id] = &ApprovalChainContext{
			ID:            id,
			Levels:        levels,
			EscalateAfter: chain.EscalateAfter,
			OnReject:      chain.OnReject,
			OnApprove:     chain.OnApprove,
			Parallel:      chain.Parallel,
			PascalName:    ToPascalCase(id),
		}
	}
	return result
}

// buildTemplatesContext converts metamodel.Template slice to TemplateContext slice.
func buildTemplatesContext(templates []metamodel.Template) []TemplateContext {
	if len(templates) == 0 {
		return nil
	}
	result := make([]TemplateContext, len(templates))
	for i, t := range templates {
		dataJSON := "{}"
		if t.Data != nil {
			// Simple JSON encoding for templates
			dataJSON = "{}"
		}
		result[i] = TemplateContext{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Data:        t.Data,
			Roles:       t.Roles,
			Default:     t.Default,
			PascalName:  ToPascalCase(t.ID),
			DataJSON:    dataJSON,
		}
	}
	return result
}

// buildBatchContext converts metamodel.BatchConfig to BatchContext.
func buildBatchContext(batch *metamodel.BatchConfig) *BatchContext {
	if batch == nil {
		return nil
	}
	maxSize := batch.MaxSize
	if maxSize == 0 {
		maxSize = 100
	}
	return &BatchContext{
		Enabled:     batch.Enabled,
		Transitions: batch.Transitions,
		MaxSize:     maxSize,
	}
}

// buildInboundWebhooksContext converts metamodel.InboundWebhook slice to InboundWebhookContext slice.
func buildInboundWebhooksContext(webhooks []metamodel.InboundWebhook) []InboundWebhookContext {
	if len(webhooks) == 0 {
		return nil
	}
	result := make([]InboundWebhookContext, len(webhooks))
	for i, w := range webhooks {
		id := w.ID
		if id == "" {
			id = w.Transition
		}
		method := w.Method
		if method == "" {
			method = "POST"
		}
		result[i] = InboundWebhookContext{
			ID:         id,
			Path:       w.Path,
			Secret:     w.Secret,
			Transition: w.Transition,
			Map:        w.Map,
			Condition:  w.Condition,
			Method:     method,
			PascalName: ToPascalCase(id),
		}
	}
	return result
}

// buildDocumentsContext converts metamodel.Document slice to DocumentContext slice.
func buildDocumentsContext(documents []metamodel.Document) []DocumentContext {
	if len(documents) == 0 {
		return nil
	}
	result := make([]DocumentContext, len(documents))
	for i, d := range documents {
		format := d.Format
		if format == "" {
			format = "pdf"
		}
		result[i] = DocumentContext{
			ID:          d.ID,
			Name:        d.Name,
			Template:    d.Template,
			Format:      format,
			Trigger:     d.Trigger,
			StoreTo:     d.StoreTo,
			Filename:    d.Filename,
			Description: d.Description,
			PascalName:  ToPascalCase(d.ID),
		}
	}
	return result
}

// buildCommentsContext converts metamodel.CommentsConfig to CommentsContext.
func buildCommentsContext(comments *metamodel.CommentsConfig) *CommentsContext {
	if comments == nil {
		return nil
	}
	maxLength := comments.MaxLength
	if maxLength == 0 {
		maxLength = 2000
	}
	return &CommentsContext{
		Enabled:    comments.Enabled,
		Roles:      comments.Roles,
		Moderation: comments.Moderation,
		MaxLength:  maxLength,
	}
}

// buildTagsContext converts metamodel.TagsConfig to TagsContext.
func buildTagsContext(tags *metamodel.TagsConfig) *TagsContext {
	if tags == nil {
		return nil
	}
	maxTags := tags.MaxTags
	if maxTags == 0 {
		maxTags = 10
	}
	freeForm := tags.FreeForm
	if !freeForm && len(tags.Predefined) == 0 {
		freeForm = true // Default to free-form if no predefined tags
	}
	return &TagsContext{
		Enabled:    tags.Enabled,
		Predefined: tags.Predefined,
		FreeForm:   freeForm,
		MaxTags:    maxTags,
		Colors:     tags.Colors,
	}
}

// buildActivityContext converts metamodel.ActivityConfig to ActivityContext.
func buildActivityContext(activity *metamodel.ActivityConfig) *ActivityContext {
	if activity == nil {
		return nil
	}
	maxItems := activity.MaxItems
	if maxItems == 0 {
		maxItems = 100
	}
	return &ActivityContext{
		Enabled:       activity.Enabled,
		IncludeEvents: activity.IncludeEvents,
		ExcludeEvents: activity.ExcludeEvents,
		MaxItems:      maxItems,
	}
}

// buildFavoritesContext converts metamodel.FavoritesConfig to FavoritesContext.
func buildFavoritesContext(favorites *metamodel.FavoritesConfig) *FavoritesContext {
	if favorites == nil {
		return nil
	}
	maxFavorites := favorites.MaxFavorites
	if maxFavorites == 0 {
		maxFavorites = 100
	}
	return &FavoritesContext{
		Enabled:      favorites.Enabled,
		Notify:       favorites.Notify,
		MaxFavorites: maxFavorites,
	}
}

// buildExportContext converts metamodel.ExportConfig to ExportContext.
func buildExportContext(export *metamodel.ExportConfig) *ExportContext {
	if export == nil {
		return nil
	}
	formats := export.Formats
	if len(formats) == 0 {
		formats = []string{"csv", "json"}
	}
	maxRows := export.MaxRows
	if maxRows == 0 {
		maxRows = 10000
	}
	return &ExportContext{
		Enabled: export.Enabled,
		Formats: formats,
		MaxRows: maxRows,
		Roles:   export.Roles,
	}
}

// buildSoftDeleteContext converts metamodel.SoftDeleteConfig to SoftDeleteContext.
func buildSoftDeleteContext(softDelete *metamodel.SoftDeleteConfig) *SoftDeleteContext {
	if softDelete == nil {
		return nil
	}
	return &SoftDeleteContext{
		Enabled:       softDelete.Enabled,
		RetentionDays: softDelete.RetentionDays,
		RestoreRoles:  softDelete.RestoreRoles,
	}
}
