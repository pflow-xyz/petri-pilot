package esmodules

import (
	"strings"

	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Context holds all data needed for React code generation templates.
type Context struct {
	// Project configuration
	ProjectName string
	PackageName string // Sanitized name for API paths (no hyphens)
	APIBaseURL  string

	// Model metadata
	ModelName        string
	ModelDescription string

	// Places and transitions
	Places      []PlaceContext
	Transitions []TransitionContext

	// API routes for client generation
	Routes []RouteContext

	// State fields for type generation
	StateFields []StateFieldContext

	// Pages for navigation and routing
	Pages []PageContext

	// Feature flags
	HasEventSourcing bool
	HasSnapshots     bool
	HasViews         bool
	HasAdmin         bool
	HasDebug         bool

	// Original model for reference
	Model *schema.Model
}

// PageContext provides template-friendly access to page data.
type PageContext struct {
	ID            string
	Title         string
	Path          string
	Icon          string
	LayoutType    string   // list, detail, form, custom
	EntityID      string   // Entity this page displays
	RequiredRoles []string // Roles that can access this page
	HideInNav     bool     // Hide from navigation menu
	ComponentName string   // React component name
}

// PlaceContext provides template-friendly access to place data.
type PlaceContext struct {
	ID          string
	Description string
	Initial     int
	IsToken     bool

	// Computed names
	PascalName string // e.g., "OrderReceived"
	CamelName  string // e.g., "orderReceived"
}

// TransitionContext provides template-friendly access to transition data.
type TransitionContext struct {
	ID          string
	Description string
	HTTPMethod  string
	HTTPPath    string

	// Computed names
	PascalName  string // e.g., "ValidateOrder"
	CamelName   string // e.g., "validateOrder"
	ConstName   string // e.g., "VALIDATE_ORDER"
	DisplayName string // e.g., "Validate Order"
}

// RouteContext provides template-friendly access to API route data.
type RouteContext struct {
	Method       string
	Path         string
	Description  string
	TransitionID string
	PascalName   string
	CamelName    string
}

// StateFieldContext provides template-friendly access to aggregate state fields.
type StateFieldContext struct {
	Name       string
	PascalName string
	CamelName  string
	Type       string // TypeScript type
	IsToken    bool
}

// ContextOptions for creating a new context.
type ContextOptions struct {
	ProjectName string
	APIBaseURL  string
	Pages       []PageContext // Optional: for Application-based generation
}

// NewContext creates a Context from a model with computed template data.
func NewContext(model *schema.Model, opts ContextOptions) (*Context, error) {
	// Enrich the model with defaults
	enriched := bridge.EnrichModel(model)

	// Determine project name
	projectName := opts.ProjectName
	if projectName == "" {
		projectName = sanitizeProjectName(enriched.Name)
	}

	// Determine API base URL
	apiBaseURL := opts.APIBaseURL
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}

	ctx := &Context{
		ProjectName:      projectName,
		PackageName:      sanitizePackageName(projectName),
		APIBaseURL:       apiBaseURL,
		ModelName:        enriched.Name,
		ModelDescription: enriched.Description,
		Model:            enriched,
		// Feature flags - event sourcing is always enabled in generated backends
		HasEventSourcing: true,
		HasSnapshots:     true,
		HasViews:         len(enriched.Views) > 0,
		HasAdmin:         true, // Always generate admin dashboard
		HasDebug:         enriched.Debug != nil && enriched.Debug.Enabled,
	}

	// Build place contexts
	ctx.Places = buildPlaceContexts(enriched.Places)

	// Build transition contexts
	ctx.Transitions = buildTransitionContexts(enriched.Transitions)

	// Build route contexts from bridge inference
	apiRoutes := bridge.InferAPIRoutes(enriched)
	ctx.Routes = buildRouteContexts(apiRoutes)

	// Build state field contexts from bridge inference
	stateFields := bridge.InferAggregateState(enriched)
	ctx.StateFields = buildStateFieldContexts(stateFields)

	// Use provided pages or create default ones
	if len(opts.Pages) > 0 {
		ctx.Pages = opts.Pages
	} else {
		// Create default pages from model
		ctx.Pages = createDefaultPages(enriched.Name)
	}

	return ctx, nil
}

func buildPlaceContexts(places []schema.Place) []PlaceContext {
	result := make([]PlaceContext, len(places))
	for i, p := range places {
		result[i] = PlaceContext{
			ID:          p.ID,
			Description: p.Description,
			Initial:     p.Initial,
			IsToken:     p.IsToken(),
			PascalName:  toPascalCase(p.ID),
			CamelName:   toCamelCase(p.ID),
		}
	}
	return result
}

func buildTransitionContexts(transitions []schema.Transition) []TransitionContext {
	result := make([]TransitionContext, len(transitions))
	for i, t := range transitions {
		result[i] = TransitionContext{
			ID:          t.ID,
			Description: t.Description,
			HTTPMethod:  t.HTTPMethod,
			HTTPPath:    t.HTTPPath,
			PascalName:  toPascalCase(t.ID),
			CamelName:   toCamelCase(t.ID),
			ConstName:   toConstName(t.ID),
			DisplayName: toDisplayName(t.ID),
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
			PascalName:   toPascalCase(r.TransitionID),
			CamelName:    toCamelCase(r.TransitionID),
		}
	}
	return result
}

func buildStateFieldContexts(stateFields []bridge.StateField) []StateFieldContext {
	result := make([]StateFieldContext, len(stateFields))
	for i, f := range stateFields {
		result[i] = StateFieldContext{
			Name:       f.Name,
			PascalName: toPascalCase(f.Name),
			CamelName:  toCamelCase(f.Name),
			Type:       goTypeToTS(f.Type),
			IsToken:    f.IsToken,
		}
	}
	return result
}

// Naming utilities

func sanitizeProjectName(name string) string {
	// Convert to lowercase, replace special chars with hyphens
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, "_", "-")
	result = strings.ReplaceAll(result, " ", "-")
	return result
}

func sanitizePackageName(name string) string {
	// Convert to lowercase, remove special chars (Go package name style)
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, "-", "")
	result = strings.ReplaceAll(result, "_", "")
	result = strings.ReplaceAll(result, " ", "")
	return result
}

func toPascalCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, "")
}

func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

func toConstName(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w)
	}
	return strings.Join(words, "_")
}

func toDisplayName(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

func splitWords(s string) []string {
	// Split on underscores, hyphens, and camelCase boundaries
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")

	var words []string
	current := ""

	for i, r := range s {
		if r == '_' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else if i > 0 && isUpper(byte(r)) && !isUpper(s[i-1]) {
			// CamelCase boundary
			if current != "" {
				words = append(words, current)
			}
			current = string(r)
		} else {
			current += string(r)
		}
	}

	if current != "" {
		words = append(words, current)
	}

	return words
}

func isUpper(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func goTypeToTS(goType string) string {
	switch goType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "string":
		return "string"
	default:
		// For complex types, use any or the type name
		if strings.HasPrefix(goType, "[]") {
			elemType := goTypeToTS(goType[2:])
			return elemType + "[]"
		}
		if strings.HasPrefix(goType, "map[") {
			return "Record<string, any>"
		}
		return "any"
	}
}

// createDefaultPages creates default page contexts when no pages are specified.
func createDefaultPages(modelName string) []PageContext {
	entityName := strings.ToLower(modelName)
	return []PageContext{
		{
			ID:            entityName + "-list",
			Title:         strings.Title(entityName) + " List",
			Path:          "/" + entityName,
			LayoutType:    "list",
			EntityID:      entityName,
			ComponentName: toPascalCase(entityName) + "List",
		},
		{
			ID:            entityName + "-detail",
			Title:         strings.Title(entityName) + " Detail",
			Path:          "/" + entityName + "/:id",
			LayoutType:    "detail",
			EntityID:      entityName,
			ComponentName: toPascalCase(entityName) + "Detail",
			HideInNav:     true, // Don't show dynamic routes in nav
		},
		{
			ID:            entityName + "-new",
			Title:         "New " + strings.Title(entityName),
			Path:          "/" + entityName + "/new",
			LayoutType:    "form",
			EntityID:      entityName,
			ComponentName: toPascalCase(entityName) + "Form",
		},
	}
}
