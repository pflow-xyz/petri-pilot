package metamodel

import (
	"sort"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// SchemaFromModel converts a go-pflow metamodel.Model into the local Schema,
// which is the surface Runtime executes.
//
// It is the inverse of Schema.ToModel, and exists so that anything holding a
// Model can be *executed* with the real firing rule. That matters more than it
// sounds: go-pflow's tokenmodel.Runtime — which petri_simulate used to use —
// ignores arc weights (it hardcodes a "< 1" enablement check), ignores
// inhibitor arcs entirely, moves exactly one token per arc regardless of
// weight, and never evaluates a guard. This Runtime honours all four.
//
// The one asymmetry worth knowing: the two packages disagree on what an unset
// Kind means. go-pflow reads "" as token (Place.IsToken); this package reads it
// as data (State.IsData). Conversion therefore always stamps an explicit Kind,
// so a model that never annotated its places still executes as its author
// intended rather than silently becoming a net of data cells.
func SchemaFromModel(m *goflowmodel.Model) *Schema {
	if m == nil {
		return nil
	}

	s := NewSchema(m.Name)
	s.Version = m.Version
	s.Description = m.Description

	for _, p := range m.Places {
		state := State{
			ID:          p.ID,
			Type:        p.Type,
			Exported:    p.Exported,
			Description: p.Description,
		}
		if p.IsToken() {
			state.Kind = TokenState
			state.Initial = p.Initial
		} else {
			state.Kind = DataState
			if p.InitialValue != nil {
				state.Initial = p.InitialValue
			}
		}
		s.AddState(state)
	}

	for _, t := range m.Transitions {
		s.AddAction(Action{
			ID:          t.ID,
			Guard:       t.Guard,
			Description: t.Description,
			Bindings:    bindingsToMap(t.Bindings),
		})
	}

	for _, a := range m.Arcs {
		weight := a.Weight
		if weight == 0 {
			weight = 1
		}
		s.AddArc(Arc{
			Source: a.From,
			Target: a.To,
			Weight: weight,
			Keys:   append([]string(nil), a.Keys...),
			Value:  a.Value,
			Type:   ArcType(a.Type),
		})
	}

	for _, c := range m.Constraints {
		s.AddConstraint(Constraint{ID: c.ID, Expr: c.Expr})
	}

	return s
}

// bindingsToMap flattens []Binding into the name→type map Action carries.
//
// Sorted by name so the result is reproducible; Action.Bindings is a map, but
// anything that walks it downstream should see a stable order.
func bindingsToMap(in []goflowmodel.Binding) map[string]string {
	if len(in) == 0 {
		return nil
	}
	names := make([]string, 0, len(in))
	byName := make(map[string]string, len(in))
	for _, b := range in {
		if _, seen := byName[b.Name]; !seen {
			names = append(names, b.Name)
		}
		byName[b.Name] = b.Type
	}
	sort.Strings(names)

	out := make(map[string]string, len(names))
	for _, n := range names {
		out[n] = byName[n]
	}
	return out
}
