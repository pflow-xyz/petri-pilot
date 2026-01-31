// Package zkgo generates ZK circuit code from Petri net models.
//
// This package is a thin wrapper around go-pflow's zkcompile/petrigen package.
// It provides the same interface for petri-pilot's MCP tools.
package zkgo

import (
	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/zkcompile/petrigen"
)

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
	inner *petrigen.Generator
	opts  Options
}

// New creates a new Generator with the given options.
func New(opts Options) (*Generator, error) {
	inner, err := petrigen.New(petrigen.Options{
		PackageName:  opts.PackageName,
		OutputDir:    opts.OutputDir,
		IncludeTests: opts.IncludeTests,
	})
	if err != nil {
		return nil, err
	}

	return &Generator{
		inner: inner,
		opts:  opts,
	}, nil
}

// GenerateFiles generates ZK code files in memory.
func (g *Generator) GenerateFiles(model *metamodel.Model) ([]GeneratedFile, error) {
	files, err := g.inner.GenerateFiles(model)
	if err != nil {
		return nil, err
	}

	// Convert to our GeneratedFile type
	result := make([]GeneratedFile, len(files))
	for i, f := range files {
		result[i] = GeneratedFile{
			Name:    f.Name,
			Content: f.Content,
		}
	}
	return result, nil
}

// Generate generates ZK code files and writes them to the output directory.
func (g *Generator) Generate(model *metamodel.Model) ([]string, error) {
	files, err := g.inner.Generate(model)
	if err != nil {
		return nil, err
	}

	// Convert petrigen.GeneratedFile to paths
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = g.opts.OutputDir + "/" + f.Name
	}
	return paths, nil
}
