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
	TemplateTest      = "test"
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
	TemplateTest:      {File: "test.tmpl", Output: "workflow_test.go"},
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
		TemplateTest,
	}
}

// CodeTemplateNames returns template names that generate Go code (excludes go.mod).
func CodeTemplateNames() []string {
	return []string{
		TemplateMain,
		TemplateWorkflow,
		TemplateEvents,
		TemplateAggregate,
		TemplateAPI,
	}
}

// TestTemplateNames returns template names that generate test files.
func TestTemplateNames() []string {
	return []string{
		TemplateTest,
	}
}
