package golang

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Template names
const (
	TemplateGoMod     = "go_mod"
	TemplateMain      = "main"
	TemplateWorkflow  = "workflow"
	TemplateEvents    = "events"
	TemplateAggregate = "aggregate"
	TemplateAPI       = "api"
	TemplateOpenAPI   = "openapi"
	TemplateTest      = "test"

	// Infrastructure templates (Phase 7)
	TemplateConfig        = "config"
	TemplateMigrations    = "migrations"
	TemplateDockerfile    = "dockerfile"
	TemplateDockerCompose = "docker-compose"

	// Auth templates (Phase 9)
	TemplateAuth       = "auth"
	TemplateMiddleware = "middleware"

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
)

// templateInfo maps template names to their file names and output files.
var templateInfo = map[string]struct {
	File   string
	Output string
}{
	TemplateGoMod:     {File: "go_mod.tmpl", Output: "go.mod"},
	TemplateMain:      {File: "main.tmpl", Output: "main.go"},
	TemplateWorkflow:  {File: "workflow.tmpl", Output: "workflow.go"},
	TemplateEvents:    {File: "events.tmpl", Output: "events.go"},
	TemplateAggregate: {File: "aggregate.tmpl", Output: "aggregate.go"},
	TemplateAPI:       {File: "api.tmpl", Output: "api.go"},
	TemplateOpenAPI:   {File: "openapi.tmpl", Output: "openapi.yaml"},
	TemplateTest:      {File: "test.tmpl", Output: "workflow_test.go"},

	// Infrastructure templates
	TemplateConfig:        {File: "config.tmpl", Output: "config.go"},
	TemplateMigrations:    {File: "migrations.tmpl", Output: "migrations/001_init.sql"},
	TemplateDockerfile:    {File: "dockerfile.tmpl", Output: "Dockerfile"},
	TemplateDockerCompose: {File: "docker-compose.tmpl", Output: "docker-compose.yaml"},

	// Auth templates
	TemplateAuth:       {File: "auth.tmpl", Output: "auth.go"},
	TemplateMiddleware: {File: "middleware.tmpl", Output: "middleware.go"},

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
}

// Templates holds parsed templates for code generation.
type Templates struct {
	templates *template.Template
}

// NewTemplates creates a new Templates instance with all embedded templates parsed.
func NewTemplates() (*Templates, error) {
	// Define custom template functions
	funcMap := template.FuncMap{
		"pascal":    ToPascalCase,
		"camel":     ToCamelCase,
		"constName": ToConstName,
		"handler":   ToHandlerName,
		"eventType": ToEventTypeName,
		"field":     ToFieldName,
		"varName":   ToVarName,
		"typeName":  ToTypeName,
		"sanitize":  SanitizePackageName,
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
		TemplateDockerfile,
		TemplateDockerCompose,
	}
}

// CodeTemplateNames returns template names that generate code (excludes go.mod, tests, and infra).
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

// AuthTemplateNames returns template names for authentication files.
func AuthTemplateNames() []string {
	return []string{
		TemplateAuth,
		TemplateMiddleware,
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
		TemplateDockerfile,
		TemplateDockerCompose,
	}
}
