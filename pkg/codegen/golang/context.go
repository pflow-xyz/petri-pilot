package golang

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

// Context holds all data needed for code generation templates.
type Context struct {
	// HasSimulation is true when the simulation endpoints are generated.
	HasSimulation bool

	// StreamPrefix namespaces the aggregate's event stream in bundle mode
	// ("<entity>/"); empty for single-net apps. See ContextOptions.
	StreamPrefix string

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

	// CrossEntity lists the transitions a bundle composition owns (see
	// crossentity.go). Nil for single-net apps.
	CrossEntity []CrossEntityContext

	// Inferred types
	Events      []EventContext
	Routes      []RouteContext
	StateFields []StateFieldContext

	// Entity domain fields (from petri-pilot/entities extension)
	EntityFields []EntityFieldContext
	EventData    []EventDataContext
	EntityRoutes []EntityRouteContext

	// ORM-specific data (for models with DataState places)
	Collections []CollectionContext
	DataArcs    []DataArcContext
	// BindingFields is the model's parameter vocabulary — the fields of the
	// generated Bindings struct. Derived, not hardcoded; see
	// buildBindingFieldContexts.
	BindingFields []BindingFieldContext
	Guards        []GuardContext

	// Access control (Phase 11)
	AccessRules []AccessRuleContext
	Roles       []RoleContext

	// Views (Phase 13)
	Views []ViewContext

	// Navigation (Phase 14)
	Navigation *NavigationContext

	// Admin Dashboard (Phase 14)
	Admin *AdminContext

	// Event Sourcing (Phase 14)
	EventSourcing *EventSourcingContext

	// Debug configuration
	Debug *DebugContext

	// GraphQL configuration
	GraphQL *GraphQLContext

	// Original model for reference
	Model *metamodel.Model

	// Schema JSON for schema viewer page
	SchemaJSON string

	// Bazel build spec, populated when IncludeBazel is set.
	Bazel *BazelBuildContext
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

// GraphQLContext provides template-friendly access to GraphQL configuration.
type GraphQLContext struct {
	Enabled    bool   // Enable GraphQL API
	Path       string // GraphQL endpoint path (default: "/graphql")
	Playground bool   // Enable GraphQL Playground
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

	// Typed event data (from entity fields)
	EventData *EventDataContext

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
	IsRead      bool   // True if this is a read arc (requires tokens, consumes none)
}

// IsReadOnly reports an arc that only tests the marking. Anything emitting a
// consuming input MUST skip these: a read arc rendered as an Input steals a
// token on every firing, and since Apply replays every event, the marking
// drifts further from the truth with each one.
func (a ArcContext) IsReadOnly() bool { return a.IsInhibitor || a.IsRead }

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

// EntityFieldContext provides template-friendly access to entity domain fields.
// These are fields from the petri-pilot/entities extension that should be
// included in the State struct alongside token counts.
type EntityFieldContext struct {
	ID          string // Field ID from entity definition
	FieldName   string // Go field name (e.g., "URL", "Title")
	Type        string // Go type (string, int64, bool, time.Time, etc.)
	JSONName    string // JSON field name
	Required    bool   // Whether this field is required
	Description string // Field description for comments
}

// EventDataContext provides template-friendly access to typed event data.
// Each transition can have typed input data based on entity fields.
type EventDataContext struct {
	TransitionID string // Transition this data applies to
	StructName   string // Go struct name (e.g., "SaveBookmarkData")
	Fields       []EventDataFieldContext
}

// EventDataFieldContext provides template-friendly access to event data fields.
type EventDataFieldContext struct {
	FieldName string // Go field name
	Type      string // Go type
	JSONName  string // JSON field name
	Required  bool   // Whether this field is required
}

// EntityRouteContext provides RESTful route aliases for entities.
// Maps CRUD operations to the appropriate transition handlers.
type EntityRouteContext struct {
	EntityID         string // Entity ID (e.g., "bookmark")
	EntityName       string // Display name (e.g., "Bookmark")
	PluralName       string // Plural name for routes (e.g., "bookmarks")
	BasePath         string // RESTful base path (e.g., "/api/bookmarks")
	CreateHandler    string // Handler name for POST (create)
	CreateTransition string // Transition ID for create
	UpdateHandler    string // Handler name for PUT (update)
	UpdateTransition string // Transition ID for update
	DeleteHandler    string // Handler name for DELETE
	DeleteTransition string // Transition ID for delete
	HasCreate        bool   // True if create transition exists
	HasUpdate        bool   // True if update transition exists
	HasDelete        bool   // True if delete transition exists
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
	TransitionID string   // Transition this guard belongs to
	Expression   string   // Original guard expression
	Collections  []string // Collections referenced by the guard

	// UsesMarking is true when the guard calls a marking-aware function
	// (tokens, sum, count, minOf, maxOf). Those need the current token counts
	// passed to the evaluator; without them the expression fails to resolve at
	// runtime with "unknown function: tokens". A composed GuardLink lowers to
	// exactly this shape, so it is what makes cross-subnet gating work in
	// generated code.
	UsesMarking bool
}

// markingFuncNames are the guard functions that need the marking.
// Kept in step with dsl.MakeAggregates.
var markingFuncNames = []string{"tokens", "sum", "count", "minOf", "maxOf"}

// guardUsesMarking reports whether an expression calls a marking-aware function.
func guardUsesMarking(expr string) bool {
	for _, name := range markingFuncNames {
		if callsFunction(expr, name) {
			return true
		}
	}
	return false
}

// callsFunction reports whether expr contains a call to name, requiring the
// opening paren and rejecting matches that are part of a longer identifier so
// "accounts(" does not count as "count(".
func callsFunction(expr, name string) bool {
	for i := 0; i+len(name) < len(expr); i++ {
		if expr[i:i+len(name)] != name {
			continue
		}
		if i > 0 && isIdentRune(expr[i-1]) {
			continue // part of a longer name
		}
		rest := strings.TrimLeft(expr[i+len(name):], " \t")
		if strings.HasPrefix(rest, "(") {
			return true
		}
	}
	return false
}

func isIdentRune(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// StreamExpr renders the Go expression naming an aggregate's event stream.
//
// In bundle mode it prefixes with the entity name, matching the coordinator's
// StreamID(entity, id); otherwise it is the bare id, so single-net apps generate
// exactly what they always did.
func (c *Context) StreamExpr(idVar string) string {
	if c.StreamPrefix == "" {
		return idVar
	}
	return fmt.Sprintf("%q+%s", c.StreamPrefix, idVar)
}

// Options for creating a new context.
type ContextOptions struct {
	ModulePath  string
	PackageName string

	// HasSimulation mounts the read-only forecast/simulate/rates endpoints.
	HasSimulation bool

	// StreamPrefix namespaces this aggregate's event stream, as "<entity>/".
	// Set only in bundle mode: a composed app's coordinator addresses member
	// streams as "<entity>/<id>", and without the same prefix here the entity
	// API and the coordinator would read and write DIFFERENT logs for the same
	// aggregate. Empty for single-net apps, whose streams stay bare ids.
	StreamPrefix string

	// CrossEntityTransitions lists this model's transitions that a bundle
	// composition has taken over. See crossentity.go. Empty for single-net
	// apps, which then generate exactly as they did before it existed.
	CrossEntityTransitions []CrossEntityTransition

	// Access control (Phase 11)
	AccessRules []AccessRuleContext
	Roles       []RoleContext
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
		HasSimulation:    opts.HasSimulation,
		StreamPrefix:     opts.StreamPrefix,
		PackageName:      packageName,
		ModulePath:       modulePath,
		LocalReplacePath: localReplacePath,
		APISlug:          apiSlug,
		ModelName:        enriched.Name,
		ModelDescription: enriched.Description,
		Model:            enriched,
		AccessRules:      opts.AccessRules,
		Roles:            opts.Roles,
	}

	// Identifier stems are allocated once per model and shared by every builder
	// below, so an arc and the place it points at agree on the disambiguated
	// name. See identScope.
	scopes := newIdentScopes()

	// Build place contexts
	ctx.Places = buildPlaceContexts(enriched.Places, scopes.place)

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
	ctx.Transitions = buildTransitionContexts(enriched.Transitions, enriched.Arcs, enriched.Events, placeIDs, dataPlaceIDs, scopes)

	// Build event contexts from bridge inference
	eventDefs := metamodel.InferEvents(enriched)
	ctx.Events = buildEventContexts(eventDefs, scopes.event)

	// Build route contexts from bridge inference
	apiRoutes := metamodel.InferAPIRoutes(enriched)
	ctx.Routes = buildRouteContexts(apiRoutes, scopes.transition)

	// Build state field contexts from bridge inference
	stateFields := metamodel.InferAggregateState(enriched)
	ctx.StateFields = buildStateFieldContexts(stateFields, scopes.place)

	// Build ORM-specific contexts
	ormSpec := bridge.ExtractORMSpec(enriched)
	ctx.Collections = buildCollectionContexts(ormSpec.Collections)
	ctx.DataArcs = buildDataArcContexts(ormSpec.Operations)
	ctx.BindingFields = buildBindingFieldContexts(enriched.Transitions, ctx.DataArcs)
	ctx.Guards = buildGuardContexts(enriched.Transitions, ormSpec.Collections)

	ctx.CrossEntity = buildCrossEntityContexts(opts.CrossEntityTransitions, ctx.Transitions)

	// Populate data arcs, guard info, and event data on transitions
	for i := range ctx.Transitions {
		tid := ctx.Transitions[i].ID
		ctx.Transitions[i].InputDataArcs = ctx.InputDataArcs(tid)
		ctx.Transitions[i].OutputDataArcs = ctx.OutputDataArcs(tid)
		ctx.Transitions[i].GuardInfo = ctx.GuardForTransition(tid)
		ctx.Transitions[i].EventData = ctx.EventDataForTransition(tid)
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

	// Entity fields and access rules from entities extension
	if entitiesExt := app.Entities(); entitiesExt != nil {
		ctx.AccessRules = buildAccessRuleContextsFromEntities(entitiesExt)
		ctx.EntityFields = buildEntityFieldContexts(entitiesExt)
		ctx.EventData = buildEventDataContexts(entitiesExt, ctx.Transitions)
		ctx.EntityRoutes = buildEntityRouteContexts(entitiesExt, ctx.Transitions)
		// Re-populate EventData on transitions after setting ctx.EventData
		for i := range ctx.Transitions {
			ctx.Transitions[i].EventData = ctx.EventDataForTransition(ctx.Transitions[i].ID)
		}
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
				HasGuard:     rule.Guard != "",
			})
		}
	}
	return result
}

// buildEntityFieldContexts extracts domain fields from entities for the State struct.
// These fields store actual business data (URL, Title, Tags) alongside token counts.
func buildEntityFieldContexts(ext *extensions.EntityExtension) []EntityFieldContext {
	var result []EntityFieldContext
	seen := make(map[string]bool) // Deduplicate fields across entities

	for _, entity := range ext.Entities {
		for _, field := range entity.Fields {
			// Skip if we've already seen this field ID
			if seen[field.ID] {
				continue
			}
			seen[field.ID] = true

			goType := fieldTypeToGoType(field.Type)
			result = append(result, EntityFieldContext{
				ID:          field.ID,
				FieldName:   ToPascalCase(field.ID),
				Type:        goType,
				JSONName:    field.ID, // Keep original snake_case ID as JSON name
				Required:    field.Required,
				Description: field.Description,
			})
		}
	}
	return result
}

// fieldTypeToGoType converts petri-pilot field types to Go types.
func fieldTypeToGoType(ft extensions.FieldType) string {
	switch ft {
	case extensions.FieldTypeString:
		return "string"
	case extensions.FieldTypeInt:
		return "int"
	case extensions.FieldTypeInt64:
		return "int64"
	case extensions.FieldTypeFloat64:
		return "float64"
	case extensions.FieldTypeBool:
		return "bool"
	case extensions.FieldTypeTime:
		return "time.Time"
	case extensions.FieldTypeJSON:
		return "json.RawMessage"
	case extensions.FieldTypeReference:
		return "string" // Store as ID reference
	default:
		return "any"
	}
}

// buildEventDataContexts creates typed event data structs for each transition.
// This allows transitions to have strongly-typed input data based on entity fields.
func buildEventDataContexts(ext *extensions.EntityExtension, transitions []TransitionContext) []EventDataContext {
	var result []EventDataContext

	// Build a map of action ID to entity fields for that action
	actionFields := make(map[string][]extensions.Field)
	for _, entity := range ext.Entities {
		for _, action := range entity.Actions {
			// Include all entity fields for this action
			actionFields[action.ID] = entity.Fields
		}
	}

	// Create EventDataContext for each transition that has associated entity fields
	for _, trans := range transitions {
		fields, hasFields := actionFields[trans.ID]
		if !hasFields || len(fields) == 0 {
			continue
		}

		var eventFields []EventDataFieldContext
		for _, f := range fields {
			eventFields = append(eventFields, EventDataFieldContext{
				FieldName: ToPascalCase(f.ID),
				Type:      fieldTypeToGoType(f.Type),
				JSONName:  f.ID, // Keep original snake_case ID as JSON name
				Required:  f.Required,
			})
		}

		result = append(result, EventDataContext{
			TransitionID: trans.ID,
			StructName:   trans.FuncName + "Data",
			Fields:       eventFields,
		})
	}

	return result
}

// buildEntityRouteContexts creates RESTful route aliases for entities.
// Maps CRUD operations like POST /api/bookmarks to the appropriate transition handlers.
func buildEntityRouteContexts(ext *extensions.EntityExtension, transitions []TransitionContext) []EntityRouteContext {
	var result []EntityRouteContext

	// Build a map of transition IDs to their handler names
	transitionHandlers := make(map[string]string)
	for _, t := range transitions {
		transitionHandlers[t.ID] = t.HandlerName
	}

	for _, entity := range ext.Entities {
		route := EntityRouteContext{
			EntityID:   entity.ID,
			EntityName: entity.Name,
			PluralName: entity.ID + "s", // Simple pluralization
			BasePath:   "/api/" + entity.ID + "s",
		}

		// Find CRUD transitions for this entity
		for _, action := range entity.Actions {
			handler := transitionHandlers[action.ID]
			if handler == "" {
				continue
			}

			// Match common CRUD patterns
			actionLower := strings.ToLower(action.ID)
			if strings.HasPrefix(actionLower, "save_") || strings.HasPrefix(actionLower, "create_") || strings.HasPrefix(actionLower, "add_") {
				route.CreateHandler = handler
				route.CreateTransition = action.ID
				route.HasCreate = true
			} else if strings.HasPrefix(actionLower, "edit_") || strings.HasPrefix(actionLower, "update_") || strings.HasPrefix(actionLower, "modify_") {
				route.UpdateHandler = handler
				route.UpdateTransition = action.ID
				route.HasUpdate = true
			} else if strings.HasPrefix(actionLower, "delete_") || strings.HasPrefix(actionLower, "remove_") {
				route.DeleteHandler = handler
				route.DeleteTransition = action.ID
				route.HasDelete = true
			}
		}

		// Only add if at least one CRUD operation was found
		if route.HasCreate || route.HasUpdate || route.HasDelete {
			result = append(result, route)
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

func buildPlaceContexts(places []metamodel.Place, scope *identScope) []PlaceContext {
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
			ConstName:   "Place" + scope.Stem(p.ID),
			FieldName:   scope.Stem(p.ID),
			VarName:     lowerFirst(scope.Stem(p.ID)),
		}
	}
	return result
}

func buildTransitionContexts(transitions []metamodel.Transition, arcs []metamodel.Arc, events []metamodel.Event, placeIDs, dataPlaceIDs map[string]bool, scopes *identScopes) []TransitionContext {
	// Build event lookup map for deriving bindings from event fields
	eventByID := make(map[string]metamodel.Event, len(events))
	for _, e := range events {
		eventByID[e.ID] = e
	}
	// Build arc maps for each transition
	// Inputs: arcs where arc.To == transition.ID (place -> transition)
	// Outputs: arcs where arc.From == transition.ID (transition -> place)
	// Note: Data state places are excluded from token counting - guards handle those
	// Inhibitor and read arcs are included in inputs but flagged, since neither
	// consumes; the templates must not emit them as consuming Inputs.
	// Use nested maps keyed by transition ID → arc key to deduplicate arcs.
	// Duplicate NORMAL arcs accumulate weights (two weight-1 arcs = one
	// weight-2 arc).
	//
	// The key carries the arc TYPE as well as the place. Keying on the place
	// alone would fold a read arc and a consuming arc over the same place into
	// one entry, and whichever came first would decide whether the other
	// consumes — a normal arc silently demoted to a test, or a read arc
	// promoted to token theft.
	inputArcMap := make(map[string]map[string]ArcContext)  // transition → key → arc
	outputArcMap := make(map[string]map[string]ArcContext) // transition → key → arc

	// Dedup keys in the order the model declares them. Ranging the maps below
	// would emit arcs in Go's randomized map order, making generated code differ
	// between runs for an identical model; declaration order keeps generation
	// reproducible and is stable under edits to unrelated transitions.
	inputArcOrder := make(map[string][]string)  // transition → arc keys, first-seen order
	outputArcOrder := make(map[string][]string) // transition → arc keys, first-seen order

	// arcKey is type-then-place; a normal arc's key is "|<place>", so keys for
	// models without read/inhibitor arcs are unchanged in content and order.
	arcKey := func(typ metamodel.ArcType, place string) string { return string(typ) + "|" + place }

	for _, arc := range arcs {
		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}

		// If arc goes from a place to something, and that something is not a place,
		// it's an input to a transition
		// Skip data state places - they don't use token counting
		if placeIDs[arc.From] && !placeIDs[arc.To] && !dataPlaceIDs[arc.From] {
			if inputArcMap[arc.To] == nil {
				inputArcMap[arc.To] = make(map[string]ArcContext)
			}
			key := arcKey(arc.Type, arc.From)
			if existing, ok := inputArcMap[arc.To][key]; ok {
				switch {
				case arc.IsRead():
					// A read arc is a lower bound, so two of them mean the
					// stricter one: summing would demand tokens neither arc
					// asked for.
					if weight > existing.Weight {
						existing.Weight = weight
					}
				case arc.IsInhibitor():
					// An inhibitor is an upper bound: the lower threshold is
					// the stricter one.
					if weight < existing.Weight {
						existing.Weight = weight
					}
				default:
					existing.Weight += weight
				}
				inputArcMap[arc.To][key] = existing
			} else {
				inputArcMap[arc.To][key] = ArcContext{
					PlaceID:     arc.From,
					ConstName:   "Place" + scopes.place.Stem(arc.From),
					Weight:      weight,
					IsInhibitor: arc.IsInhibitor(),
					IsRead:      arc.IsRead(),
				}
				inputArcOrder[arc.To] = append(inputArcOrder[arc.To], key)
			}
		}

		// If arc goes from something that's not a place to a place,
		// it's an output from a transition
		// Skip data state places - they don't use token counting
		// Note: read-only arcs (inhibitor, read) never produce outputs
		if !placeIDs[arc.From] && placeIDs[arc.To] && !dataPlaceIDs[arc.To] && !arc.IsReadOnly() {
			if outputArcMap[arc.From] == nil {
				outputArcMap[arc.From] = make(map[string]ArcContext)
			}
			key := arcKey(arc.Type, arc.To)
			if existing, ok := outputArcMap[arc.From][key]; ok {
				existing.Weight += weight
				outputArcMap[arc.From][key] = existing
			} else {
				outputArcMap[arc.From][key] = ArcContext{
					PlaceID:     arc.To,
					ConstName:   "Place" + scopes.place.Stem(arc.To),
					Weight:      weight,
					IsInhibitor: false,
				}
				outputArcOrder[arc.From] = append(outputArcOrder[arc.From], key)
			}
		}

		// A read-only arc pointing transition -> place is pflow-xyz's
		// long-standing spelling of a guard, and pkg/codegen/core lowers it to
		// exactly the same Reads entry a forward read arc gets. Here it used to
		// match neither branch above — not an input (source is not a place),
		// not an output (read-only) — so the full-app generator DROPPED the
		// precondition and emitted an aggregate that fires without it, while
		// core mode, petri_verify and the validator all enforce it.
		//
		// Normalising it into the transition's read set makes the two
		// generators agree. The forward key is reused so the two spellings of
		// one condition dedup against each other.
		if !placeIDs[arc.From] && placeIDs[arc.To] && !dataPlaceIDs[arc.To] && arc.IsReadOnly() {
			if inputArcMap[arc.From] == nil {
				inputArcMap[arc.From] = make(map[string]ArcContext)
			}
			key := arcKey(metamodel.ReadArc, arc.To)
			if existing, ok := inputArcMap[arc.From][key]; ok {
				// Two reads of one place mean the stricter lower bound.
				if weight > existing.Weight {
					existing.Weight = weight
					inputArcMap[arc.From][key] = existing
				}
			} else {
				inputArcMap[arc.From][key] = ArcContext{
					PlaceID:   arc.To,
					ConstName: "Place" + scopes.place.Stem(arc.To),
					Weight:    weight,
					IsRead:    true,
				}
				inputArcOrder[arc.From] = append(inputArcOrder[arc.From], key)
			}
		}
	}

	// Flatten deduped maps into slices, following declaration order.
	inputArcs := make(map[string][]ArcContext)
	for transID, order := range inputArcOrder {
		for _, key := range order {
			inputArcs[transID] = append(inputArcs[transID], inputArcMap[transID][key])
		}
	}
	outputArcs := make(map[string][]ArcContext)
	for transID, order := range outputArcOrder {
		for _, key := range order {
			outputArcs[transID] = append(outputArcs[transID], outputArcMap[transID][key])
		}
	}

	result := make([]TransitionContext, len(transitions))
	for i, t := range transitions {
		stem := scopes.transition.Stem(t.ID)
		eventType := t.EventType
		if eventType == "" {
			eventType = eventTypeFromStem(stem)
		}
		// Keyed by the event type, not the transition: several transitions
		// sharing one EventType share one generated struct (tic-tac-toe's nine
		// x_play_* transitions all emit XPlayed). Only two *different* event
		// types that sanitize to the same identifier need separating.
		eventName := scopes.event.Unique(eventType, ToEventStructName(eventType))

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
			ID:            t.ID,
			Description:   t.Description,
			Guard:         t.Guard,
			EventType:     eventType,
			EventRef:      t.Event,
			HTTPMethod:    t.HTTPMethod,
			HTTPPath:      t.HTTPPath,
			Bindings:      bindings,
			Inputs:        inputArcs[t.ID],
			Outputs:       outputArcs[t.ID],
			Duration:      t.Duration,
			MinDuration:   t.MinDuration,
			MaxDuration:   t.MaxDuration,
			HasSLATiming:  hasSLATiming,
			Rate:          t.Rate,
			ClearsHistory: t.ClearsHistory,
			ConstName:     "Transition" + stem,
			HandlerName:   "Handle" + stem,
			EventName:     eventName,
			FuncName:      stem,
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

// eventScope is the scope the transition contexts already allocated from, so an
// event and the transition that emits it name the same struct.
func buildEventContexts(eventDefs []metamodel.EventDef, eventScope *identScope) []EventContext {
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
			StructName:   eventScope.Unique(e.Type, ToEventStructName(e.Type)),
			TransitionID: e.TransitionID,
			Fields:       fields,
		}
	}
	return result
}

func buildRouteContexts(apiRoutes []metamodel.APIRoute, transScope *identScope) []RouteContext {
	result := make([]RouteContext, len(apiRoutes))
	for i, r := range apiRoutes {
		result[i] = RouteContext{
			Method:       r.Method,
			Path:         r.Path,
			Description:  r.Description,
			TransitionID: r.TransitionID,
			HandlerName:  "Handle" + transScope.Stem(r.TransitionID),
			EventType:    r.EventType,
		}
	}
	return result
}

// stateFields are named by place ID, so they share the place scope: the State
// struct field and the PlaceContext field name have to be the same identifier.
func buildStateFieldContexts(stateFields []metamodel.StateField, placeScope *identScope) []StateFieldContext {
	result := make([]StateFieldContext, len(stateFields))
	for i, f := range stateFields {
		result[i] = StateFieldContext{
			Name:      f.Name,
			FieldName: placeScope.Stem(f.Name),
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

// BindingFieldContext is one field of the generated Bindings struct — the
// model's parameter vocabulary. Previously the struct was hardcoded to the
// ERC-20 field set in aggregate.tmpl; it is now derived from the model:
// declared Transition.Bindings plus the key/value binding names on data arcs.
type BindingFieldContext struct {
	Name      string // metamodel binding key, e.g. "from"
	FieldName string // Go field name, e.g. "From"
	GoType    string // "string" or "U256JSON"
	IsNumeric bool
}

// isNumericBindingType reports whether a declared binding type is numeric.
// Unknown types default to string — the safe JSON shape.
func isNumericBindingType(t string) bool {
	switch t {
	case "int", "int64", "uint64", "float64", "number", "u256", "uint256":
		return true
	}
	return false
}

// buildBindingFieldContexts unions the binding names a model actually uses:
// every declared transition binding, every data-arc map key (string-typed by
// construction), and every data-arc value binding (numeric when the
// collection's value type is). aggregate_id and timestamp are implicit on
// every Bindings struct and excluded here. Sorted by name so generation is
// deterministic.
func buildBindingFieldContexts(transitions []metamodel.Transition, dataArcs []DataArcContext) []BindingFieldContext {
	numeric := map[string]bool{}
	seen := map[string]bool{}

	note := func(name string, isNumeric bool) {
		if name == "" || name == "aggregate_id" || name == "timestamp" {
			return
		}
		seen[name] = true
		if isNumeric {
			numeric[name] = true
		}
	}

	for _, t := range transitions {
		for _, b := range t.Bindings {
			note(b.Name, isNumericBindingType(b.Type) || b.Value)
		}
	}
	for _, a := range dataArcs {
		for _, k := range a.Keys {
			note(k, false)
		}
		note(a.ValueBinding, a.IsNumeric)
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	fields := make([]BindingFieldContext, 0, len(names))
	for _, name := range names {
		goType := "string"
		if numeric[name] {
			goType = "U256JSON"
		}
		fields = append(fields, BindingFieldContext{
			Name:      name,
			FieldName: ToPascalCase(name),
			GoType:    goType,
			IsNumeric: numeric[name],
		})
	}
	return fields
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
			Collections:  referencedCollections,
			UsesMarking:  guardUsesMarking(t.Guard),
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

// HasCrossEntity reports whether a bundle composition owns any of this
// model's transitions.
func (c *Context) HasCrossEntity() bool {
	return len(c.CrossEntity) > 0
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

// EventDataForTransition returns the EventDataContext for a transition, or nil.
func (c *Context) EventDataForTransition(transitionID string) *EventDataContext {
	for i := range c.EventData {
		if c.EventData[i].TransitionID == transitionID {
			return &c.EventData[i]
		}
	}
	return nil
}

// HasEventData returns true if the context has any EventData defined.
func (c *Context) HasEventData() bool {
	return len(c.EventData) > 0
}

// UsesMetamodelRuntime returns true if the generated code should use
// go-pflow's metamodel.Runtime for execution.
func (c *Context) UsesMetamodelRuntime() bool {
	// Use metamodel runtime when we have data places or guards
	return c.HasDataPlaces() || c.HasGuards()
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

// HasViews returns true if the context has any views defined.
func (c *Context) HasViews() bool {
	return len(c.Views) > 0
}

// HasEntityRoutes returns true if RESTful entity routes are available.
func (c *Context) HasEntityRoutes() bool {
	return len(c.EntityRoutes) > 0
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

// HasGraphQL returns true if the model has GraphQL enabled.
func (c *Context) HasGraphQL() bool {
	return c.GraphQL != nil && c.GraphQL.Enabled
}

// HasPlayground returns true if GraphQL Playground is enabled.
func (c *Context) HasPlayground() bool {
	return c.HasGraphQL() && c.GraphQL.Playground
}

// HasTimeFields returns true if any entity field uses time.Time type.
func (c *Context) HasTimeFields() bool {
	for _, f := range c.EntityFields {
		if f.Type == "time.Time" {
			return true
		}
	}
	return false
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
