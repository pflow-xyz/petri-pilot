package esmodules

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
	TemplatePackageJSON = "package_json"
	TemplateViteConfig  = "vite_config"
	TemplateIndexHTML   = "index_html"
	TemplateMain        = "main"
	TemplateRouter      = "router"
	TemplateNavigation  = "navigation"
	TemplatePages       = "pages"
	TemplateViews       = "views"
	TemplateEvents      = "events"
	TemplateAdmin       = "admin"
	TemplateSimulation  = "simulation"
	TemplateBlobs       = "blobs"
)

// templateInfo maps template names to their file names and output files.
var templateInfo = map[string]struct {
	File   string
	Output string
}{
	TemplatePackageJSON: {File: "package_json.tmpl", Output: "package.json"},
	TemplateViteConfig:  {File: "vite_config.tmpl", Output: "vite.config.js"},
	TemplateIndexHTML:   {File: "index_html.tmpl", Output: "index.html"},
	TemplateMain:        {File: "main.tmpl", Output: "src/main.js"},
	TemplateRouter:      {File: "router.tmpl", Output: "src/router.js"},
	TemplateNavigation:  {File: "navigation.tmpl", Output: "src/navigation.js"},
	TemplatePages:       {File: "pages.tmpl", Output: "src/pages.js"},
	TemplateViews:       {File: "views.tmpl", Output: "src/views.js"},
	TemplateEvents:      {File: "events.tmpl", Output: "src/events.js"},
	TemplateAdmin:       {File: "admin.tmpl", Output: "src/admin.js"},
	TemplateSimulation:  {File: "simulation.tmpl", Output: "src/simulation.js"},
	TemplateBlobs:       {File: "blobs.tmpl", Output: "src/blobs.js"},
}

// Templates holds parsed templates for code generation.
type Templates struct {
	templates *template.Template
}

// NewTemplates creates a new Templates instance with all embedded templates parsed.
func NewTemplates() (*Templates, error) {
	// Define custom template functions
	funcMap := template.FuncMap{
		"pascal":      toPascalCase,
		"camel":       toCamelCase,
		"constName":   toConstName,
		"displayName": toDisplayName,
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
	return name + ".js"
}

// AllTemplateNames returns template names for generation.
func AllTemplateNames() []string {
	return []string{
		TemplatePackageJSON,
		TemplateViteConfig,
		TemplateIndexHTML,
		TemplateMain,
		TemplateRouter,
		TemplateNavigation,
		TemplatePages,
		TemplateViews,
		TemplateEvents,
		TemplateAdmin,
		TemplateSimulation,
	}
}
