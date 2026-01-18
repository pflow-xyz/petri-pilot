package golang

import (
	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Context holds all data needed for code generation templates.
type Context struct {
	// Package configuration
	PackageName string
	ModulePath  string

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
	
	// Workflow orchestration (Phase 12)
	Workflows []WorkflowContext

	// Original model for reference
	Model *schema.Model
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
	Input      map[string]string // Input field mappings
	OnSuccess  string            // Next step ID on success
	OnFailure  string            // Next step ID on failure
}

// AccessRuleContext provides template-friendly access to access control rules.
type AccessRuleContext struct {
	TransitionID string   // Transition this rule applies to
	Roles        []string // Required roles
	Guard        string   // Optional guard expression
}

// RoleContext provides template-friendly access to role definitions.
type RoleContext struct {
	ID          string
	Name        string
	Description string
	Inherits    []string // Parent role IDs
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
	HTTPMethod  string
	HTTPPath    string

	// Petri net connections (derived from arcs)
	Inputs  []ArcContext // Places that feed into this transition
	Outputs []ArcContext // Places that this transition feeds into

	// Data arcs for ORM patterns
	InputDataArcs  []DataArcContext // DataState input arcs
	OutputDataArcs []DataArcContext // DataState output arcs

	// Guard info (if present)
	GuardInfo *GuardContext

	// Computed names
	ConstName   string // e.g., "TransitionValidate"
	HandlerName string // e.g., "HandleValidate"
	EventName   string // e.g., "ValidatedEvent"
	FuncName    string // e.g., "Validate"
}

// ArcContext provides template-friendly access to arc data.
type ArcContext struct {
	PlaceID   string // The place ID
	ConstName string // e.g., "PlaceReceived"
	Weight    int    // Token weight (default 1)
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
	TransitionID string   // Transition this arc belongs to
	PlaceID      string   // Collection place ID
	FieldName    string   // Go field name of collection
	ValueType    string   // Go type of the value (e.g., "int64", "string")
	IsSimple     bool     // True for simple types (direct assignment)
	Keys         []string // Key binding names - empty for simple types
	KeyFields    []string // Go field names for keys (e.g., ["From"])
	ValueBinding string   // Value binding name (e.g., "amount" or "name")
	ValueField   string   // Go field name for value (e.g., "Amount")
	IsInput      bool     // True for input arcs (subtract/read)
	IsOutput     bool     // True for output arcs (add/write)
	IsNumeric    bool     // True if value is numeric (can use += / -=)
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
}

// NewContext creates a Context from a model with computed template data.
func NewContext(model *schema.Model, opts ContextOptions) (*Context, error) {
	// Enrich the model with defaults
	enriched := bridge.EnrichModel(model)

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

	ctx := &Context{
		PackageName:      packageName,
		ModulePath:       modulePath,
		ModelName:        enriched.Name,
		ModelDescription: enriched.Description,
		Model:            enriched,
		AccessRules:      opts.AccessRules,
		Roles:            opts.Roles,
		Workflows:        opts.Workflows,
	}

	// Build place contexts
	ctx.Places = buildPlaceContexts(enriched.Places)

	// Build place ID set for quick lookups
	placeIDs := make(map[string]bool)
	for _, p := range enriched.Places {
		placeIDs[p.ID] = true
	}

	// Build transition contexts with arc information
	ctx.Transitions = buildTransitionContexts(enriched.Transitions, enriched.Arcs, placeIDs)

	// Build event contexts from bridge inference
	eventDefs := bridge.InferEvents(enriched)
	ctx.Events = buildEventContexts(eventDefs)

	// Build route contexts from bridge inference
	apiRoutes := bridge.InferAPIRoutes(enriched)
	ctx.Routes = buildRouteContexts(apiRoutes)

	// Build state field contexts from bridge inference
	stateFields := bridge.InferAggregateState(enriched)
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

	return ctx, nil
}

func buildPlaceContexts(places []schema.Place) []PlaceContext {
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
			ConstName:   ToConstName("Place", p.ID),
			FieldName:   ToFieldName(p.ID),
			VarName:     ToVarName(p.ID),
		}
	}
	return result
}

func buildTransitionContexts(transitions []schema.Transition, arcs []schema.Arc, placeIDs map[string]bool) []TransitionContext {
	// Build arc maps for each transition
	// Inputs: arcs where arc.To == transition.ID (place -> transition)
	// Outputs: arcs where arc.From == transition.ID (transition -> place)
	inputArcs := make(map[string][]ArcContext)
	outputArcs := make(map[string][]ArcContext)

	for _, arc := range arcs {
		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}

		// If arc goes from a place to something, and that something is not a place,
		// it's an input to a transition
		if placeIDs[arc.From] && !placeIDs[arc.To] {
			inputArcs[arc.To] = append(inputArcs[arc.To], ArcContext{
				PlaceID:   arc.From,
				ConstName: ToConstName("Place", arc.From),
				Weight:    weight,
			})
		}

		// If arc goes from something that's not a place to a place,
		// it's an output from a transition
		if !placeIDs[arc.From] && placeIDs[arc.To] {
			outputArcs[arc.From] = append(outputArcs[arc.From], ArcContext{
				PlaceID:   arc.To,
				ConstName: ToConstName("Place", arc.To),
				Weight:    weight,
			})
		}
	}

	result := make([]TransitionContext, len(transitions))
	for i, t := range transitions {
		eventType := t.EventType
		if eventType == "" {
			eventType = ToEventTypeName(t.ID)
		}

		result[i] = TransitionContext{
			ID:          t.ID,
			Description: t.Description,
			Guard:       t.Guard,
			EventType:   eventType,
			HTTPMethod:  t.HTTPMethod,
			HTTPPath:    t.HTTPPath,
			Inputs:      inputArcs[t.ID],
			Outputs:     outputArcs[t.ID],
			ConstName:   ToConstName("Transition", t.ID),
			HandlerName: ToHandlerName(t.ID),
			EventName:   ToEventStructName(eventType),
			FuncName:    ToPascalCase(t.ID),
		}
	}
	return result
}

func buildEventContexts(eventDefs []bridge.EventDef) []EventContext {
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

func buildRouteContexts(apiRoutes []bridge.APIRoute) []RouteContext {
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

func buildStateFieldContexts(stateFields []bridge.StateField) []StateFieldContext {
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

func buildDataArcContexts(operations []bridge.OperationSpec) []DataArcContext {
	var result []DataArcContext

	for _, op := range operations {
		// Process read arcs (inputs)
		for _, read := range op.Reads {
			keyFields := make([]string, len(read.Keys))
			for i, k := range read.Keys {
				keyFields[i] = ToPascalCase(k)
			}

			valueType := TypeToGo(read.CollectionType)
			if !read.IsSimple {
				// For maps, the value type is the map's value type
				_, vt, _ := ParseMapType(read.CollectionType)
				valueType = TypeToGo(vt)
			}

			result = append(result, DataArcContext{
				TransitionID: op.TransitionID,
				PlaceID:      read.Collection,
				FieldName:    ToPascalCase(read.Collection),
				ValueType:    valueType,
				IsSimple:     read.IsSimple,
				Keys:         read.Keys,
				KeyFields:    keyFields,
				ValueBinding: read.ValueBinding,
				ValueField:   ToPascalCase(read.ValueBinding),
				IsInput:      true,
				IsOutput:     false,
				IsNumeric:    IsNumericType(valueType),
			})
		}

		// Process write arcs (outputs)
		for _, write := range op.Writes {
			keyFields := make([]string, len(write.Keys))
			for i, k := range write.Keys {
				keyFields[i] = ToPascalCase(k)
			}

			valueType := TypeToGo(write.CollectionType)
			if !write.IsSimple {
				// For maps, the value type is the map's value type
				_, vt, _ := ParseMapType(write.CollectionType)
				valueType = TypeToGo(vt)
			}

			result = append(result, DataArcContext{
				TransitionID: op.TransitionID,
				PlaceID:      write.Collection,
				FieldName:    ToPascalCase(write.Collection),
				ValueType:    valueType,
				IsSimple:     write.IsSimple,
				Keys:         write.Keys,
				KeyFields:    keyFields,
				ValueBinding: write.ValueBinding,
				ValueField:   ToPascalCase(write.ValueBinding),
				IsInput:      false,
				IsOutput:     true,
				IsNumeric:    IsNumericType(valueType),
			})
		}
	}

	return result
}

func buildGuardContexts(transitions []schema.Transition, collections []bridge.CollectionSpec) []GuardContext {
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
