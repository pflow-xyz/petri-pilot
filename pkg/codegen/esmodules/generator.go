// Package esmodules generates vanilla JavaScript frontend applications from Petri net models.
package esmodules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Options configures the frontend code generator.
type Options struct {
	// OutputDir is the directory where generated files will be written.
	OutputDir string

	// ProjectName is the name for package.json (default: model name).
	ProjectName string

	// APIBaseURL is the backend API base URL (default: http://localhost:8080).
	APIBaseURL string
}

// GeneratedFile represents a generated file's content.
type GeneratedFile struct {
	Name         string // File path relative to output dir (e.g., "src/main.js")
	Content      []byte
	SkipIfExists bool // If true, don't overwrite existing files (user-customizable)
}

// Generator generates frontend code from Petri net models.
type Generator struct {
	opts      Options
	templates *Templates
}

// New creates a new Generator with the given options.
func New(opts Options) (*Generator, error) {
	if opts.APIBaseURL == "" {
		opts.APIBaseURL = "http://localhost:8080"
	}

	templates, err := NewTemplates()
	if err != nil {
		return nil, fmt.Errorf("initializing templates: %w", err)
	}

	return &Generator{
		opts:      opts,
		templates: templates,
	}, nil
}

// Generate generates frontend code files and writes them to the output directory.
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

		// Create subdirectories if needed
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}

		// Skip files marked as "generate once" if they already exist
		if file.SkipIfExists {
			if _, err := os.Stat(path); err == nil {
				// File exists, skip it to preserve user customizations
				continue
			}
		}

		if err := os.WriteFile(path, file.Content, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", file.Name, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

// GenerateFiles generates frontend code files in memory without writing to disk.
func (g *Generator) GenerateFiles(model *schema.Model) ([]GeneratedFile, error) {
	// Validate model for code generation
	if issues := bridge.ValidateForCodegen(model); len(issues) > 0 {
		return nil, fmt.Errorf("model validation failed: %v", issues)
	}

	// Build template context
	ctx, err := NewContext(model, ContextOptions{
		ProjectName: g.opts.ProjectName,
		APIBaseURL:  g.opts.APIBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("building context: %w", err)
	}

	// Determine which templates to generate
	templateNames := AllTemplateNames()

	// Include blobs template if blobstore is enabled
	if ctx.HasBlobstore {
		templateNames = append(templateNames, TemplateBlobs)
	}

	// Include wallet template if wallet is enabled
	if ctx.HasWallet {
		templateNames = append(templateNames, TemplateWallet)
	}

	// Generate each file
	var files []GeneratedFile
	for _, name := range templateNames {
		content, err := g.templates.Execute(name, ctx)
		if err != nil {
			return nil, fmt.Errorf("generating %s: %w", name, err)
		}

		files = append(files, GeneratedFile{
			Name:         g.templates.OutputFileName(name),
			Content:      content,
			SkipIfExists: g.templates.ShouldSkipIfExists(name),
		})
	}

	return files, nil
}

// GenerateToDir is a convenience function that creates a generator and writes files.
func GenerateToDir(model *schema.Model, outputDir string) ([]string, error) {
	gen, err := New(Options{
		OutputDir: outputDir,
	})
	if err != nil {
		return nil, err
	}
	return gen.Generate(model)
}

// GetTemplates returns the template manager for this generator.
func (g *Generator) GetTemplates() *Templates {
	return g.templates
}
