package golang

import (
	"bytes"
	"fmt"
	"path"
	"text/template"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// Bundle template names. These render against *BundleContext, unlike the
// entity-level templates which render against *Context.
const (
	TemplateBundleRoot    = "bundle_root"
	TemplateFlatModel     = "flatmodel"
	TemplateBundleApp     = "bundle_app"
	TemplateBundleAppTest = "bundle_app_test"
)

var bundleTemplateOutputs = map[string]string{
	TemplateBundleRoot:    "bundle.go",
	TemplateFlatModel:     "flatmodel.go",
	TemplateBundleApp:     "app.go",
	TemplateBundleAppTest: "app_test.go",
}

// GenerateBundleFiles generates a composed application from a bundle: one Go
// subpackage per entity (the existing submodule pipeline on that subnet's own
// model) plus root-package files derived from the flattened model. File
// names are slash-relative to the app root ("<entity>/aggregate.go",
// "bundle.go").
func (g *Generator) GenerateBundleFiles(b *metamodel.Bundle) ([]GeneratedFile, error) {
	bc, err := NewBundleContext(b, ContextOptions{ModulePath: g.opts.ModulePath})
	if err != nil {
		return nil, err
	}

	var files []GeneratedFile

	// Entity subpackages: the single-net pipeline, unchanged, per subnet.
	for _, entity := range bc.Entities {
		sub, err := New(Options{
			PackageName:  entity.PackageName,
			ModulePath:   g.opts.ModulePath,
			AsSubmodule:  true,
			IncludeTests: g.opts.IncludeTests,
			// Must match app.go's StreamID(entity, id): without it the entity
			// API and the coordinator write different logs for one aggregate.
			StreamPrefix: entity.SubnetID + "/",
			// The transitions this entity may no longer fire on its own.
			CrossEntityTransitions: entity.CrossEntity,
		})
		if err != nil {
			return nil, fmt.Errorf("entity %q: %w", entity.SubnetID, err)
		}
		var model *metamodel.Model
		for _, sn := range b.Subnets {
			if sn.ID == entity.SubnetID {
				model = sn.Model
			}
		}
		entityFiles, err := sub.GenerateFiles(model)
		if err != nil {
			return nil, fmt.Errorf("entity %q: %w", entity.SubnetID, err)
		}
		for _, f := range entityFiles {
			files = append(files, GeneratedFile{
				Name:    path.Join(entity.PackageName, f.Name),
				Content: f.Content,
			})
		}
	}

	// Root package: composition metadata + the embedded flattened model.
	rootTmpl, err := template.New("").Funcs(templateFuncMap()).ParseFS(templateFS,
		"templates/bundle_root.tmpl", "templates/flatmodel.tmpl",
		"templates/bundle_app.tmpl", "templates/bundle_app_test.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing bundle templates: %w", err)
	}
	rootTemplates := []string{TemplateBundleRoot, TemplateFlatModel, TemplateBundleApp}
	if g.opts.IncludeTests {
		rootTemplates = append(rootTemplates, TemplateBundleAppTest)
	}
	for _, name := range rootTemplates {
		var buf bytes.Buffer
		if err := rootTmpl.ExecuteTemplate(&buf, name+".tmpl", bc); err != nil {
			return nil, fmt.Errorf("rendering %s: %w", name, err)
		}
		out := bundleTemplateOutputs[name]
		formatted, err := formatGo(out, buf.Bytes())
		if err != nil {
			return nil, err
		}
		files = append(files, GeneratedFile{Name: out, Content: formatted})
	}

	// Root and entity packages are checked together — grouped per directory, so
	// each is type-checked as its own package.
	if err := checkGeneratedPackage(files); err != nil {
		return nil, err
	}

	return files, nil
}
