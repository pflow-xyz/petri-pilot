package codegen

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Options configures the ZK ODE code generator.
type Options struct {
	PackageName  string
	OutputDir    string
	Scoring      *ScoringConfig
	IncludeTests bool
	AsSubmodule  bool // Skip go.mod generation
}

// GeneratedFile represents a single output file.
type GeneratedFile struct {
	Path    string
	Content []byte
}

// Generator produces ZK ODE circuit packages from Petri net models.
type Generator struct {
	opts      Options
	templates map[string]*template.Template
}

// New creates a new Generator with parsed templates.
func New(opts Options) (*Generator, error) {
	g := &Generator{
		opts:      opts,
		templates: make(map[string]*template.Template),
	}

	templateSources := map[string]string{
		TemplateTopology:       topologyTmpl,
		TemplateCircuit:        circuitTmpl,
		TemplateWitness:        witnessTmpl,
		TemplateState:          stateTmpl,
		TemplateScoringCircuit: scoringCircuitTmpl,
		TemplateScoringWitness: scoringWitnessTmpl,
	}

	for name, src := range templateSources {
		tmpl, err := template.New(name).Parse(src)
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", name, err)
		}
		g.templates[name] = tmpl
	}

	return g, nil
}

// Generate produces all files for the given model.
func (g *Generator) Generate(model *metamodel.Model) ([]GeneratedFile, error) {
	ctx, err := NewContext(model, g.opts.PackageName, g.opts.Scoring)
	if err != nil {
		return nil, fmt.Errorf("building context: %w", err)
	}

	var files []GeneratedFile

	// Always generate core files
	coreTemplates := []string{
		TemplateTopology,
		TemplateCircuit,
		TemplateWitness,
		TemplateState,
	}

	for _, name := range coreTemplates {
		content, err := g.executeTemplate(name, ctx)
		if err != nil {
			return nil, err
		}
		files = append(files, GeneratedFile{
			Path:    name,
			Content: content,
		})
	}

	// Conditionally generate scoring files
	if ctx.HasScoring {
		scoringTemplates := []string{
			TemplateScoringCircuit,
			TemplateScoringWitness,
		}
		for _, name := range scoringTemplates {
			content, err := g.executeTemplate(name, ctx)
			if err != nil {
				return nil, err
			}
			files = append(files, GeneratedFile{
				Path:    name,
				Content: content,
			})
		}
	}

	return files, nil
}

// GenerateToDir generates all files and writes them to the output directory.
// Returns the list of written file paths.
func (g *Generator) GenerateToDir(model *metamodel.Model) ([]string, error) {
	files, err := g.Generate(model)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(g.opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	var paths []string
	for _, f := range files {
		outPath := filepath.Join(g.opts.OutputDir, f.Path)
		if err := os.WriteFile(outPath, f.Content, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", outPath, err)
		}
		paths = append(paths, outPath)
	}

	return paths, nil
}

func (g *Generator) executeTemplate(name string, ctx *Context) ([]byte, error) {
	tmpl, ok := g.templates[name]
	if !ok {
		return nil, fmt.Errorf("template %q not found", name)
	}

	var buf []byte
	w := &byteWriter{buf: &buf}
	if err := tmpl.Execute(w, ctx); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", name, err)
	}

	// Format Go source
	formatted, err := format.Source(buf)
	if err != nil {
		// Return unformatted if formatting fails (useful for debugging)
		return buf, nil
	}

	return formatted, nil
}

// byteWriter wraps a byte slice as an io.Writer.
type byteWriter struct {
	buf *[]byte
}

func (w *byteWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
