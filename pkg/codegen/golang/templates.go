package golang

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Template names
const (
	TemplateGoMod     = "go_mod"
	TemplateMain      = "main"
	TemplateService   = "service"
	TemplateWorkflow  = "workflow"
	TemplateEvents    = "events"
	TemplateAggregate = "aggregate"
	TemplateAPI       = "api"
	TemplateOpenAPI   = "openapi"
	TemplateTest      = "test"

	// Infrastructure templates (Phase 7)
	TemplateConfig     = "config"
	TemplateMigrations = "migrations"

	// Auth templates (Phase 9)
	TemplateAuth        = "auth"
	TemplateMiddleware  = "middleware"
	TemplatePermissions = "permissions"

	// Observability templates (Phase 10)
	TemplateObservability = "observability"

	// Deployment templates (Phase 10)
	TemplateK8sDeployment = "k8s_deployment"
	TemplateK8sService    = "k8s_service"
	TemplateGitHubCI      = "github_ci"

	// Real-time templates
	TemplateRealtime = "realtime"
	
	// Workflow orchestration templates (Phase 12)
	TemplateWorkflows = "workflows"

	// Webhook integration templates
	TemplateWebhooks = "webhooks"

	// Views templates (Phase 13)
	TemplateViews = "views"

	// Navigation templates (Phase 14)
	TemplateNavigation = "navigation"

	// Admin templates (Phase 14)
	TemplateAdmin = "admin"

	// Event replay templates (Phase 14)
	TemplateAPIEvents = "api_events"

	// Debug templates
	TemplateDebug = "debug"

	// SLA templates
	TemplateSLA = "sla"

	// Prediction templates
	TemplatePrediction = "prediction"

	// GraphQL templates
	TemplateGraphQLSchema   = "graphql_schema"
	TemplateGraphQLResolver = "graphql_resolver"
	TemplateGraphQLServer   = "graphql_server"
	TemplateGqlgenConfig    = "gqlgen_config"

	// Blobstore templates
	TemplateBlobstore = "blobstore"

	// Features templates (all higher-level abstractions)
	TemplateFeatures = "features"

	// Safemath templates (uint256 arithmetic)
	TemplateSafemath = "safemath"

	// Documentation templates
	TemplateReadme = "readme"
)

// templateInfo maps template names to their file names and output files.
var templateInfo = map[string]struct {
	File   string
	Output string
}{
	TemplateGoMod:     {File: "go_mod.tmpl", Output: "go.mod"},
	TemplateMain:      {File: "main.tmpl", Output: "main.go"},
	TemplateService:   {File: "service.tmpl", Output: "service.go"},
	TemplateWorkflow:  {File: "workflow.tmpl", Output: "workflow.go"},
	TemplateEvents:    {File: "events.tmpl", Output: "events.go"},
	TemplateAggregate: {File: "aggregate.tmpl", Output: "aggregate.go"},
	TemplateAPI:       {File: "api.tmpl", Output: "api.go"},
	TemplateOpenAPI:   {File: "openapi.tmpl", Output: "openapi.yaml"},
	TemplateTest:      {File: "test.tmpl", Output: "workflow_test.go"},

	// Infrastructure templates
	TemplateConfig:     {File: "config.tmpl", Output: "config.go"},
	TemplateMigrations: {File: "migrations.tmpl", Output: "migrations/001_init.sql"},

	// Auth templates
	TemplateAuth:        {File: "auth.tmpl", Output: "auth.go"},
	TemplateMiddleware:  {File: "middleware.tmpl", Output: "middleware.go"},
	TemplatePermissions: {File: "permissions.tmpl", Output: "permissions.go"},

	// Observability templates
	TemplateObservability: {File: "observability.tmpl", Output: "observability.go"},

	// Deployment templates
	TemplateK8sDeployment: {File: "k8s_deployment.tmpl", Output: "k8s/deployment.yaml"},
	TemplateK8sService:    {File: "k8s_service.tmpl", Output: "k8s/service.yaml"},
	TemplateGitHubCI:      {File: "github_ci.tmpl", Output: ".github/workflows/ci.yaml"},

	// Real-time templates
	TemplateRealtime: {File: "realtime.tmpl", Output: "realtime.go"},
	
	// Workflow orchestration templates (Phase 12)
	TemplateWorkflows: {File: "workflows.tmpl", Output: "workflows.go"},

	// Webhook integration templates
	TemplateWebhooks: {File: "webhooks.tmpl", Output: "webhooks.go"},

	// Views templates (Phase 13)
	TemplateViews: {File: "views.tmpl", Output: "views.go"},

	// Navigation templates (Phase 14)
	TemplateNavigation: {File: "navigation.tmpl", Output: "navigation.go"},

	// Admin templates (Phase 14)
	TemplateAdmin: {File: "admin.tmpl", Output: "admin.go"},

	// Event replay templates (Phase 14)
	TemplateAPIEvents: {File: "api_events.tmpl", Output: "api_events.go"},

	// Debug templates
	TemplateDebug: {File: "debug.tmpl", Output: "debug.go"},

	// SLA templates
	TemplateSLA: {File: "sla.tmpl", Output: "sla.go"},

	// Prediction templates
	TemplatePrediction: {File: "prediction.tmpl", Output: "prediction.go"},

	// GraphQL templates
	TemplateGraphQLSchema:   {File: "graphql_schema.tmpl", Output: "graph/schema.graphqls"},
	TemplateGraphQLResolver: {File: "graphql_resolver.tmpl", Output: "graph/resolver.go"},
	TemplateGraphQLServer:   {File: "graphql_server.tmpl", Output: "graphql.go"},
	TemplateGqlgenConfig:    {File: "gqlgen_config.tmpl", Output: "gqlgen.yml"},

	// Blobstore templates
	TemplateBlobstore: {File: "blobstore.tmpl", Output: "blobstore.go"},

	// Features templates
	TemplateFeatures: {File: "features.tmpl", Output: "features.go"},

	// Safemath templates
	TemplateSafemath: {File: "safemath.tmpl", Output: "safemath.go"},

	// Documentation templates
	TemplateReadme: {File: "readme.tmpl", Output: "README.md"},
}

// Templates holds parsed templates for code generation.
type Templates struct {
	templates *template.Template
}

// NewTemplates creates a new Templates instance with all embedded templates parsed.
func NewTemplates() (*Templates, error) {
	// Define custom template functions
	funcMap := template.FuncMap{
		"pascal":      ToPascalCase,
		"camel":       ToCamelCase,
		"constName":   ToConstName,
		"handler":     ToHandlerName,
		"eventType":   ToEventTypeName,
		"field":       ToFieldName,
		"varName":     ToVarName,
		"typeName":    ToTypeName,
		"sanitize":    SanitizePackageName,
		"graphqlType": GoTypeToGraphQL,
		"lower":       strings.ToLower,
		"upper":       strings.ToUpper,
	}

	// Parse all templates from embedded filesystem
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &Templates{templates: tmpl}, nil
}

// Execute executes a template with the given context.
func (t *Templates) Execute(name string, ctx *Context) ([]byte, error) {
	info, ok := templateInfo[name]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", name)
	}

	tmpl := t.templates.Lookup(info.File)
	if tmpl == nil {
		return nil, fmt.Errorf("template not found: %s", info.File)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", name, err)
	}

	return buf.Bytes(), nil
}

// OutputFileName returns the output file name for a template.
func (t *Templates) OutputFileName(name string) string {
	if info, ok := templateInfo[name]; ok {
		return info.Output
	}
	return name + ".go"
}

// AllTemplateNames returns all available template names.
func AllTemplateNames() []string {
	return []string{
		TemplateGoMod,
		TemplateMain,
		TemplateWorkflow,
		TemplateEvents,
		TemplateAggregate,
		TemplateAPI,
		TemplateOpenAPI,
		TemplateTest,
		TemplateConfig,
		TemplateMigrations,
	}
}

// CodeTemplateNames returns template names that generate code (excludes go.mod, tests, and infra).
// These templates generate standalone services with main.go.
func CodeTemplateNames() []string {
	return []string{
		TemplateMain,
		TemplateWorkflow,
		TemplateEvents,
		TemplateAggregate,
		TemplateAPI,
		TemplateOpenAPI,
		TemplateConfig,
	}
}

// SubmoduleCodeTemplateNames returns template names for submodule mode.
// Uses service.go instead of main.go for registration-based services.
func SubmoduleCodeTemplateNames() []string {
	return []string{
		TemplateService,
		TemplateWorkflow,
		TemplateEvents,
		TemplateAggregate,
		TemplateAPI,
		TemplateOpenAPI,
		TemplateConfig,
	}
}

// AuthTemplateNames returns template names for authentication files.
func AuthTemplateNames() []string {
	return []string{
		TemplateAuth,
		TemplateMiddleware,
		TemplatePermissions,
	}
}

// ObservabilityTemplateNames returns template names for observability files.
func ObservabilityTemplateNames() []string {
	return []string{
		TemplateObservability,
	}
}

// DeployTemplateNames returns template names for deployment files.
func DeployTemplateNames() []string {
	return []string{
		TemplateK8sDeployment,
		TemplateK8sService,
		TemplateGitHubCI,
	}
}

// RealtimeTemplateNames returns template names for real-time files.
func RealtimeTemplateNames() []string {
	return []string{
		TemplateRealtime,
	}
}

// WorkflowTemplateNames returns template names for workflow orchestration files.
func WorkflowTemplateNames() []string {
	return []string{
		TemplateWorkflows,
	}
}

// WebhookTemplateNames returns template names for webhook integration files.
func WebhookTemplateNames() []string {
	return []string{
		TemplateWebhooks,
	}
}

// ViewTemplateNames returns template names for view definition files.
func ViewTemplateNames() []string {
	return []string{
		TemplateViews,
	}
}

// TestTemplateNames returns template names that generate test files.
func TestTemplateNames() []string {
	return []string{
		TemplateTest,
	}
}

// InfraTemplateNames returns template names for infrastructure files.
func InfraTemplateNames() []string {
	return []string{
		TemplateMigrations,
	}
}

// NavigationTemplateNames returns template names for navigation files.
func NavigationTemplateNames() []string {
return []string{
TemplateNavigation,
}
}

// AdminTemplateNames returns template names for admin dashboard files.
func AdminTemplateNames() []string {
return []string{
TemplateAdmin,
}
}

// EventReplayTemplateNames returns template names for event replay files.
func EventReplayTemplateNames() []string {
return []string{
TemplateAPIEvents,
}
}

// DebugTemplateNames returns template names for debug files.
func DebugTemplateNames() []string {
	return []string{
		TemplateDebug,
	}
}

// SLATemplateNames returns template names for SLA tracking files.
func SLATemplateNames() []string {
	return []string{
		TemplateSLA,
	}
}

// PredictionTemplateNames returns template names for prediction/simulation files.
func PredictionTemplateNames() []string {
	return []string{
		TemplatePrediction,
	}
}

// GraphQLTemplateNames returns template names for GraphQL API files.
func GraphQLTemplateNames() []string {
	return []string{
		TemplateGraphQLSchema,
		TemplateGraphQLResolver,
		TemplateGraphQLServer,
		TemplateGqlgenConfig,
	}
}

// BlobstoreTemplateNames returns template names for blobstore files.
func BlobstoreTemplateNames() []string {
	return []string{
		TemplateBlobstore,
	}
}

// FeaturesTemplateNames returns template names for higher-level feature files.
func FeaturesTemplateNames() []string {
	return []string{
		TemplateFeatures,
	}
}

// SafemathTemplateNames returns template names for uint256 arithmetic files.
func SafemathTemplateNames() []string {
	return []string{
		TemplateSafemath,
	}
}

// DocTemplateNames returns template names for documentation files.
func DocTemplateNames() []string {
	return []string{
		TemplateReadme,
	}
}
