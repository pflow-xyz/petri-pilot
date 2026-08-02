package metamodel

import (
	"encoding/json"
	"fmt"

	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

// ToExtensions converts this Entity to the canonical entity type in
// pkg/extensions. The two types drifted apart as near-duplicates; the
// extensions one is canonical because it is what the bundle compiler and
// codegen consume. They share their JSON wire shape, so conversion goes
// through it — a field added to one side round-trips or drops loudly in
// tests rather than silently skewing a hand-written mapping.
//
// Deprecated paths: prefer authoring specs against extensions.Entity
// directly; Entity.ToSchema remains for the legacy per-entity generation
// path only.
func (e Entity) ToExtensions() (extensions.Entity, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return extensions.Entity{}, fmt.Errorf("entity %q: %w", e.ID, err)
	}
	var out extensions.Entity
	if err := json.Unmarshal(raw, &out); err != nil {
		return extensions.Entity{}, fmt.Errorf("entity %q: %w", e.ID, err)
	}
	return out, nil
}
