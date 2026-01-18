package golang

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Options configures the Go code generator.
type Options struct {
	// OutputDir is the directory where generated files will be written.
	// Required for Generate(), not used by GenerateFiles().
	OutputDir string

	// ModulePath is the Go module path (e.g., "github.com/myorg/myproject").
	// If empty, will be inferred from model name.
	ModulePath string

	// PackageName is the Go package name.
	// If empty, will be inferred from model name.
	PackageName string

	// IncludeTests generates workflow_test.go if true.
	IncludeTests bool

	// IncludeInfra generates Dockerfile, docker-compose.yaml, and migrations if true.
	IncludeInfra bool

	// IncludeAuth generates GitHub OAuth authentication files if true.
	IncludeAuth bool

	// IncludeObservability generates logging and metrics files if true.
	IncludeObservability bool

	// IncludeDeploy generates K8s manifests and CI workflow if true.
	IncludeDeploy bool

	// IncludeRealtime generates SSE and WebSocket handlers if true.
	IncludeRealtime bool
}

// GeneratedFile represents a generated file's content.
type GeneratedFile struct {
	Name    string // File name (e.g., "main.go")
	Content []byte // File content
}

// Generator generates Go code from Petri net models.
type Generator struct {
	opts      Options
	templates *Templates
}

// New creates a new Generator with the given options.
func New(opts Options) (*Generator, error) {
	templates, err := NewTemplates()
	if err != nil {
		return nil, fmt.Errorf("initializing templates: %w", err)
	}

	return &Generator{
		opts:      opts,
		templates: templates,
	}, nil
}

// Generate generates Go code files and writes them to the output directory.
// Returns the list of generated file paths.
func (g *Generator) Generate(model *schema.Model) ([]string, error) {
	if g.opts.OutputDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	// Validate model for code generation
	if issues := bridge.ValidateForCodegen(model); len(issues) > 0 {
		return nil, fmt.Errorf("model validation failed: %v", issues)
	}

	// Generate files in memory
	files, err := g.GenerateFiles(model)
	if err != nil {
		return nil, err
	}

	// Create output directory
	if err := os.MkdirAll(g.opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Write files to disk
	var paths []string
	for _, file := range files {
		path := filepath.Join(g.opts.OutputDir, file.Name)

		// Create subdirectories if needed (e.g., for migrations/)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}

		if err := os.WriteFile(path, file.Content, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", file.Name, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// GenerateFiles generates Go code files in memory without writing to disk.
// Useful for testing and preview functionality.
func (g *Generator) GenerateFiles(model *schema.Model) ([]GeneratedFile, error) {
	// Validate model for code generation
	if issues := bridge.ValidateForCodegen(model); len(issues) > 0 {
		return nil, fmt.Errorf("model validation failed: %v", issues)
	}

	// Build template context
	ctx, err := NewContext(model, ContextOptions{
		ModulePath:  g.opts.ModulePath,
		PackageName: g.opts.PackageName,
	})
	if err != nil {
		return nil, fmt.Errorf("building context: %w", err)
	}

	// Determine which templates to generate
	templateNames := append([]string{TemplateGoMod}, CodeTemplateNames()...)
	if g.opts.IncludeTests {
		templateNames = append(templateNames, TestTemplateNames()...)
	}
	if g.opts.IncludeInfra {
		templateNames = append(templateNames, InfraTemplateNames()...)
	}
	if g.opts.IncludeAuth {
		templateNames = append(templateNames, AuthTemplateNames()...)
	}
	if g.opts.IncludeObservability {
		templateNames = append(templateNames, ObservabilityTemplateNames()...)
	}
	if g.opts.IncludeDeploy {
		templateNames = append(templateNames, DeployTemplateNames()...)
	}
	if g.opts.IncludeRealtime {
		templateNames = append(templateNames, RealtimeTemplateNames()...)
	}
	
	// Include workflows template if context has workflows (Phase 12)
	if ctx.HasWorkflows() {
		templateNames = append(templateNames, WorkflowTemplateNames()...)
	}

	// Generate each file
	var files []GeneratedFile
	for _, name := range templateNames {
		content, err := g.templates.Execute(name, ctx)
		if err != nil {
			return nil, fmt.Errorf("generating %s: %w", name, err)
		}

		files = append(files, GeneratedFile{
			Name:    g.templates.OutputFileName(name),
			Content: content,
		})
	}

	return files, nil
}

// Preview generates a preview of a single template without writing to disk.
func (g *Generator) Preview(model *schema.Model, templateName string) ([]byte, error) {
	ctx, err := NewContext(model, ContextOptions{
		ModulePath:  g.opts.ModulePath,
		PackageName: g.opts.PackageName,
	})
	if err != nil {
		return nil, fmt.Errorf("building context: %w", err)
	}

	return g.templates.Execute(templateName, ctx)
}

// GetTemplates returns the template manager for this generator.
func (g *Generator) GetTemplates() *Templates {
	return g.templates
}

// ValidateModel checks if a model is suitable for Go code generation.
func ValidateModel(model *schema.Model) []string {
	return bridge.ValidateForCodegen(model)
}

// GenerateToDir is a convenience function that creates a generator and writes files.
func GenerateToDir(model *schema.Model, outputDir string, includeTests bool) ([]string, error) {
	gen, err := New(Options{
		OutputDir:    outputDir,
		IncludeTests: includeTests,
		IncludeInfra: true, // Include infrastructure by default
	})
	if err != nil {
		return nil, err
	}
	return gen.Generate(model)
}
