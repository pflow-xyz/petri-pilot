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

	// Original model for reference
	Model *schema.Model
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

// Options for creating a new context.
type ContextOptions struct {
	ModulePath  string
	PackageName string
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
