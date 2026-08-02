// Package bundle is petri-pilot's authoring surface for composed
// applications: N entity nets joined into one go-pflow metamodel.Bundle,
// flattened for code generation.
//
// Two ways in:
//
//   - Load: a bundle document (JSON) whose subnets carry their models inline
//     or by file reference — the raw form for hand-authored bundles.
//   - CompileApplication (compile.go): the petri_application entity spec,
//     compiled into a Bundle — the form LLM-driven design produces.
//
// Either way the result is a validated *metamodel.Bundle; FlattenWithMap
// hands codegen the flat model plus the per-subnet rewrite map.
package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// SubnetDoc is one subnet in a bundle document. Exactly one of Model
// (inline) or ModelRef (file path, resolved by the loader) must be set.
type SubnetDoc struct {
	ID       string            `json:"id"`
	NetType  metamodel.NetType `json:"net_type,omitempty"`
	Model    *metamodel.Model  `json:"model,omitempty"`
	ModelRef string            `json:"model_ref,omitempty"`
	Ports    []metamodel.Port  `json:"ports,omitempty"`
}

// Doc is the on-disk bundle document. It mirrors metamodel.Bundle but allows
// subnet models by reference — something the upstream JSON form cannot
// express (Subnet.Model must be inline there).
type Doc struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Subnets     []SubnetDoc              `json:"subnets"`
	Links       []metamodel.Link         `json:"links,omitempty"`
	Constraints []metamodel.Constraint   `json:"constraints,omitempty"`
	Namespace   *bool                    `json:"namespace,omitempty"`
	ArcMerge    metamodel.ArcMergePolicy `json:"arc_merge,omitempty"`
}

// Resolver turns a model_ref into a model. LoadFile wires a file-relative
// resolver; tests can substitute their own.
type Resolver func(ref string) (*metamodel.Model, error)

// Load parses a bundle document, resolves model references, and returns a
// validated Bundle. Validation failures are returned as an error carrying
// every violation, not just the first.
func Load(data []byte, resolve Resolver) (*metamodel.Bundle, error) {
	var doc Doc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("bundle document: %w", err)
	}
	if doc.Name == "" {
		return nil, fmt.Errorf("bundle document: name is required")
	}
	if len(doc.Subnets) == 0 {
		return nil, fmt.Errorf("bundle document: at least one subnet is required")
	}

	b := metamodel.NewBundle(doc.Name)
	b.Description = doc.Description
	b.Links = doc.Links
	b.Constraints = doc.Constraints
	b.Namespace = doc.Namespace
	b.ArcMerge = doc.ArcMerge

	for _, sd := range doc.Subnets {
		model := sd.Model
		switch {
		case model != nil && sd.ModelRef != "":
			return nil, fmt.Errorf("subnet %q: model and model_ref are mutually exclusive", sd.ID)
		case model == nil && sd.ModelRef == "":
			return nil, fmt.Errorf("subnet %q: one of model or model_ref is required", sd.ID)
		case model == nil:
			if resolve == nil {
				return nil, fmt.Errorf("subnet %q: model_ref %q given but no resolver available", sd.ID, sd.ModelRef)
			}
			resolved, err := resolve(sd.ModelRef)
			if err != nil {
				return nil, fmt.Errorf("subnet %q: resolving model_ref %q: %w", sd.ID, sd.ModelRef, err)
			}
			model = resolved
		}
		b.AddSubnet(metamodel.Subnet{
			ID:      sd.ID,
			NetType: sd.NetType,
			Model:   model,
			Ports:   sd.Ports,
		})
	}

	if err := b.MustValidate(); err != nil {
		return nil, fmt.Errorf("bundle %q: %w", doc.Name, err)
	}
	return b, nil
}

// LoadFile loads a bundle document from disk, resolving model_ref paths
// relative to the document's directory. Referenced models are plain
// metamodel.Model JSON.
func LoadFile(path string) (*metamodel.Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	return Load(data, func(ref string) (*metamodel.Model, error) {
		if !filepath.IsAbs(ref) {
			ref = filepath.Join(dir, ref)
		}
		raw, err := os.ReadFile(ref)
		if err != nil {
			return nil, err
		}
		var m metamodel.Model
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("parsing model: %w", err)
		}
		return &m, nil
	})
}
