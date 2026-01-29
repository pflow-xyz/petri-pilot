// Package zkgo generates ZK circuit code from Petri net models.
//
// The generated code proves valid Petri net transition firing using gnark.
// Unlike board-only proofs, this encodes the full Petri net state and proves
// that transitions follow Petri net semantics.
package zkgo

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Options configures the ZK code generator.
type Options struct {
	// OutputDir is the directory where generated files will be written.
	OutputDir string

	// PackageName is the Go package name.
	PackageName string

	// IncludeTests generates test files if true.
	IncludeTests bool
}

// GeneratedFile represents a generated file's content.
type GeneratedFile struct {
	Name    string
	Content []byte
}

// Generator generates ZK circuit code from Petri net models.
type Generator struct {
	opts      Options
	templates *template.Template
}

// New creates a new Generator with the given options.
func New(opts Options) (*Generator, error) {
	tmpl, err := template.New("zkgo").Funcs(templateFuncs).ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &Generator{
		opts:      opts,
		templates: tmpl,
	}, nil
}

// GenerateFiles generates ZK code files in memory.
func (g *Generator) GenerateFiles(model *metamodel.Model) ([]GeneratedFile, error) {
	ctx, err := NewContext(model, g.opts.PackageName)
	if err != nil {
		return nil, fmt.Errorf("building context: %w", err)
	}

	var files []GeneratedFile

	// Generate state file
	content, err := g.executeTemplate("state.go.tmpl", ctx)
	if err != nil {
		return nil, fmt.Errorf("generating state.go: %w", err)
	}
	files = append(files, GeneratedFile{Name: "petri_state.go", Content: content})

	// Generate circuits file
	content, err = g.executeTemplate("circuits.go.tmpl", ctx)
	if err != nil {
		return nil, fmt.Errorf("generating circuits.go: %w", err)
	}
	files = append(files, GeneratedFile{Name: "petri_circuits.go", Content: content})

	// Generate game file
	content, err = g.executeTemplate("game.go.tmpl", ctx)
	if err != nil {
		return nil, fmt.Errorf("generating game.go: %w", err)
	}
	files = append(files, GeneratedFile{Name: "petri_game.go", Content: content})

	// Generate tests if requested
	if g.opts.IncludeTests {
		content, err = g.executeTemplate("test.go.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("generating test.go: %w", err)
		}
		files = append(files, GeneratedFile{Name: "petri_circuits_test.go", Content: content})
	}

	return files, nil
}

// Generate generates ZK code files and writes them to the output directory.
func (g *Generator) Generate(model *metamodel.Model) ([]string, error) {
	if g.opts.OutputDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	files, err := g.GenerateFiles(model)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(g.opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	var paths []string
	for _, file := range files {
		path := filepath.Join(g.opts.OutputDir, file.Name)
		if err := os.WriteFile(path, file.Content, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", file.Name, err)
		}
		paths = append(paths, path)
	}

	return paths, nil
}

func (g *Generator) executeTemplate(name string, ctx *Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, name, ctx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var templateFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
}
