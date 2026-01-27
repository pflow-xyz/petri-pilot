package esmodules

import (
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
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

	// Available roles for login selector
	Roles []RoleContext

	// Token/currency display configuration
	Decimals int    // Decimal places for token amounts (e.g., 18 for ETH)
	Unit     string // Display unit symbol (e.g., "ETH", "USDC")

	// Feature flags
	HasEventSourcing bool
	HasSnapshots     bool
	HasViews         bool
	HasAdmin         bool
	HasDebug         bool
	HasPrediction    bool
	HasBlobstore     bool
	HasWallet        bool

	// Prediction configuration
	Prediction *PredictionContext

	// Wallet configuration
	Wallet *WalletContext

	// Status configuration for human-readable status labels
	Status *StatusContext

	// Original model for reference
	Model *metamodel.Model
}

// PredictionContext provides template-friendly access to prediction configuration.
type PredictionContext struct {
	Enabled   bool
	TimeHours float64
	RateScale float64
}

// WalletContext provides template-friendly access to wallet configuration.
type WalletContext struct {
	Enabled      bool
	Accounts     []WalletAccountContext
	BalanceField string
	ShowInNav    bool
	AutoConnect  bool
}

// WalletAccountContext provides template-friendly access to wallet account data.
type WalletAccountContext struct {
	Address        string
	Name           string
	Roles          []string
	RolesJSON      string // JSON array for template
	InitialBalance string
}

// StatusContext provides template-friendly access to status configuration.
type StatusContext struct {
	// Places maps place IDs to human-readable status labels (as JSON for template)
	PlacesJSON string
	// Default is the status label shown when no place-specific label matches
	Default string
	// HasConfig is true if any status configuration is provided
	HasConfig bool
}

// RoleContext provides template-friendly access to role data for login selector.
type RoleContext struct {
	ID          string
	Description string
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

	// Input fields for action form
	Fields []TransitionFieldContext

	// Access control - roles that can fire this transition
	RequiredRoles     []string // e.g., ["customer", "admin"]
	RequiredRolesJSON string   // JSON array for template: '["customer", "admin"]'

	// API path for frontend (e.g., "/api/submit" or custom path)
	APIPath string // The actual path to call for this transition
}

// TransitionFieldContext provides template-friendly access to transition field data.
type TransitionFieldContext struct {
	Name        string
	Label       string
	Type        string // text, number, address, amount, select, hidden
	Required    bool
	Default     string
	AutoFill    string // wallet, user, or state path
	Placeholder string
	Description string
	Options     []FieldOptionContext
}

// FieldOptionContext provides template-friendly access to select field options.
type FieldOptionContext struct {
	Value string
	Label string
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
func NewContext(model *metamodel.Model, opts ContextOptions) (*Context, error) {
	// Enrich the model with defaults
	enriched := metamodel.EnrichModel(model)

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
		// Token/currency display
		Decimals: enriched.Decimals,
		Unit:     enriched.Unit,
		// Feature flags - event sourcing is always enabled in generated backends
		HasEventSourcing: true,
		HasSnapshots:     true,
		// Feature flags from model
		HasDebug:  enriched.Debug != nil && enriched.Debug.Enabled,
		HasAdmin:  enriched.Admin != nil && enriched.Admin.Enabled,
		HasViews:  len(enriched.Views) > 0,
		// Initialize Status with default empty config
		Status: &StatusContext{
			PlacesJSON: "{}",
			Default:    "Unknown",
			HasConfig:  false,
		},
	}

	// Build place contexts
	ctx.Places = buildPlaceContexts(enriched.Places)

	// Build transition contexts (with access control from model)
	ctx.Transitions = buildTransitionContexts(enriched.Transitions, enriched.Access)

	// Build route contexts from bridge inference
	apiRoutes := metamodel.InferAPIRoutes(enriched)
	ctx.Routes = buildRouteContexts(apiRoutes)

	// Build state field contexts from bridge inference
	stateFields := metamodel.InferAggregateState(enriched)
	ctx.StateFields = buildStateFieldContexts(stateFields)

	// Note: Roles are populated by NewContextFromApp when using extensions

	// Use provided pages or create default ones
	if len(opts.Pages) > 0 {
		ctx.Pages = opts.Pages
	} else {
		// Create default pages from model
		ctx.Pages = createDefaultPages(enriched.Name)
	}

	return ctx, nil
}

// NewContextFromApp creates a Context from an ApplicationSpec.
// This uses the extension-based model where application constructs
// are stored in extensions rather than embedded in the Model.
func NewContextFromApp(app *extensions.ApplicationSpec, opts ContextOptions) (*Context, error) {
	if app == nil || app.Net == nil {
		return nil, nil
	}

	// Use the adapter to convert to legacy model
	legacyModel := extensions.ToLegacyModel(app)

	// Create context using the legacy path
	ctx, err := NewContext(legacyModel, opts)
	if err != nil {
		return nil, err
	}

	// Override with extension data where available

	// Roles from extension
	if rolesExt := app.Roles(); rolesExt != nil {
		ctx.Roles = buildRoleContextsFromExtension(rolesExt)
	}

	// Feature flags from extensions
	if app.HasViews() {
		ctx.HasViews = true
	}
	if app.HasAdmin() {
		ctx.HasAdmin = true
	}

	return ctx, nil
}

// buildRoleContextsFromExtension converts extension roles to RoleContexts.
func buildRoleContextsFromExtension(ext *extensions.RoleExtension) []RoleContext {
	result := make([]RoleContext, len(ext.Roles))
	for i, r := range ext.Roles {
		result[i] = RoleContext{
			ID:          r.ID,
			Description: r.Description,
		}
	}
	return result
}

func buildPlaceContexts(places []metamodel.Place) []PlaceContext {
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

func buildTransitionContexts(transitions []metamodel.Transition, access []metamodel.AccessRule) []TransitionContext {
	// Build access map: transition ID -> required roles
	accessMap := make(map[string][]string)
	for _, rule := range access {
		accessMap[rule.Transition] = rule.Roles
	}

	result := make([]TransitionContext, len(transitions))
	for i, t := range transitions {
		roles := accessMap[t.ID]
		rolesJSON := "[]"
		if len(roles) > 0 {
			quotedRoles := make([]string, len(roles))
			for j, r := range roles {
				quotedRoles[j] = `"` + r + `"`
			}
			rolesJSON = "[" + strings.Join(quotedRoles, ", ") + "]"
		}

		// Determine the API path - use HTTPPath if specified, otherwise default to /api/{id}
		apiPath := t.HTTPPath
		if apiPath == "" {
			apiPath = "/api/" + t.ID
		}

		result[i] = TransitionContext{
			ID:                t.ID,
			Description:       t.Description,
			HTTPMethod:        t.HTTPMethod,
			HTTPPath:          t.HTTPPath,
			PascalName:        toPascalCase(t.ID),
			CamelName:         toCamelCase(t.ID),
			ConstName:         toConstName(t.ID),
			DisplayName:       toDisplayName(t.ID),
			Fields:            buildTransitionFieldContexts(t.Fields),
			RequiredRoles:     roles,
			RequiredRolesJSON: rolesJSON,
			APIPath:           apiPath,
		}
	}
	return result
}

func buildTransitionFieldContexts(fields []metamodel.TransitionField) []TransitionFieldContext {
	result := make([]TransitionFieldContext, len(fields))
	for i, f := range fields {
		// Build options for select fields
		options := make([]FieldOptionContext, len(f.Options))
		for j, opt := range f.Options {
			label := opt.Label
			if label == "" {
				label = opt.Value
			}
			options[j] = FieldOptionContext{
				Value: opt.Value,
				Label: label,
			}
		}

		// Default label from name if not specified
		label := f.Label
		if label == "" {
			label = toDisplayName(f.Name)
		}

		// Default type based on name patterns
		fieldType := f.Type
		if fieldType == "" {
			fieldType = inferFieldType(f.Name)
		}

		result[i] = TransitionFieldContext{
			Name:        f.Name,
			Label:       label,
			Type:        fieldType,
			Required:    f.Required,
			Default:     f.Default,
			AutoFill:    f.AutoFill,
			Placeholder: f.Placeholder,
			Description: f.Description,
			Options:     options,
		}
	}
	return result
}

// inferFieldType guesses the field type based on name patterns
func inferFieldType(name string) string {
	nameLower := strings.ToLower(name)
	switch {
	case nameLower == "amount" || nameLower == "value" || nameLower == "balance" ||
		strings.HasSuffix(nameLower, "_amount") || strings.HasSuffix(nameLower, "_value"):
		return "amount"
	case nameLower == "from" || nameLower == "to" || nameLower == "owner" ||
		nameLower == "spender" || nameLower == "caller" || nameLower == "recipient" ||
		nameLower == "address" || strings.HasSuffix(nameLower, "_address"):
		return "address"
	default:
		return "text"
	}
}

func buildRouteContexts(apiRoutes []metamodel.APIRoute) []RouteContext {
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

func buildStateFieldContexts(stateFields []metamodel.StateField) []StateFieldContext {
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

func buildRoleContexts(roles []metamodel.Role) []RoleContext {
	result := make([]RoleContext, len(roles))
	for i, r := range roles {
		result[i] = RoleContext{
			ID:          r.ID,
			Description: r.Description,
		}
	}
	return result
}

func buildStatusContext(status *metamodel.StatusConfig) *StatusContext {
	ctx := &StatusContext{
		PlacesJSON: "{}",
		Default:    "In Progress",
		HasConfig:  false,
	}

	if status == nil {
		return ctx
	}

	ctx.HasConfig = true

	// Build JSON object for places map
	if len(status.Places) > 0 {
		pairs := make([]string, 0, len(status.Places))
		for placeID, label := range status.Places {
			pairs = append(pairs, `"`+placeID+`": "`+label+`"`)
		}
		ctx.PlacesJSON = "{" + strings.Join(pairs, ", ") + "}"
	}

	if status.Default != "" {
		ctx.Default = status.Default
	}

	return ctx
}

func buildWalletContext(wallet *metamodel.WalletConfig) *WalletContext {
	accounts := make([]WalletAccountContext, len(wallet.Accounts))
	for i, acc := range wallet.Accounts {
		// Build JSON array of roles for template
		rolesJSON := "[]"
		if len(acc.Roles) > 0 {
			roles := make([]string, len(acc.Roles))
			for j, r := range acc.Roles {
				roles[j] = `"` + r + `"`
			}
			rolesJSON = "[" + strings.Join(roles, ", ") + "]"
		}

		accounts[i] = WalletAccountContext{
			Address:        acc.Address,
			Name:           acc.Name,
			Roles:          acc.Roles,
			RolesJSON:      rolesJSON,
			InitialBalance: acc.InitialBalance,
		}
	}

	return &WalletContext{
		Enabled:      wallet.Enabled,
		Accounts:     accounts,
		BalanceField: wallet.BalanceField,
		ShowInNav:    wallet.ShowInNav,
		AutoConnect:  wallet.AutoConnect,
	}
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
