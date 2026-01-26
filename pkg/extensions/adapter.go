// Package extensions provides adapters for integrating the new extension-based
// ApplicationSpec with the existing petri-pilot codegen system.
package extensions

import (
	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// ToLegacyModel returns the core Petri net Model from an ApplicationSpec.
// Application constructs (roles, views, etc.) are stored in extensions and
// should be accessed via the ApplicationSpec methods (Roles(), Views(), etc.).
//
// Example usage:
//
//	app := NewApplicationSpec(model)
//	app.WithEntities(entities)
//	app.WithRoles(roles)
//
//	coreModel := ToLegacyModel(app)
//	ctx, err := golang.NewContext(coreModel, opts)
//	// Use app.Roles(), app.Views() for application constructs
func ToLegacyModel(app *ApplicationSpec) *goflowmodel.Model {
	if app.Net == nil {
		return &goflowmodel.Model{}
	}
	return app.Net
}

// EntityToModel converts an Entity to a standalone go-pflow Model.
// This is useful when generating code for a single entity.
func EntityToModel(entity Entity) *goflowmodel.Model {
	model := &goflowmodel.Model{
		Name:        entity.ID,
		Description: entity.Description,
	}

	// Convert fields to data places
	for _, f := range entity.Fields {
		model.Places = append(model.Places, goflowmodel.Place{
			ID:          f.ID,
			Description: f.Description,
			Kind:        goflowmodel.DataKind,
			Type:        string(f.Type),
			Exported:    true,
		})
	}

	// Convert states to token places
	for _, s := range entity.States {
		initial := 0
		if s.Initial {
			initial = 1
		}
		model.Places = append(model.Places, goflowmodel.Place{
			ID:          s.ID,
			Description: s.Description,
			Kind:        goflowmodel.TokenKind,
			Initial:     initial,
		})
	}

	// Convert actions to transitions
	for _, a := range entity.Actions {
		bindings := make([]goflowmodel.Binding, 0)
		for _, p := range a.Input {
			bindings = append(bindings, goflowmodel.Binding{
				Name: p.ID,
				Type: string(p.Type),
			})
		}

		model.Transitions = append(model.Transitions, goflowmodel.Transition{
			ID:          a.ID,
			Description: a.Description,
			Guard:       a.Guard,
			Bindings:    bindings,
		})

		// Add arcs for state transitions
		for _, from := range a.FromStates {
			model.Arcs = append(model.Arcs, goflowmodel.Arc{
				From:   from,
				To:     a.ID,
				Weight: 1,
			})
		}
		if a.ToState != "" {
			model.Arcs = append(model.Arcs, goflowmodel.Arc{
				From:   a.ID,
				To:     a.ToState,
				Weight: 1,
			})
		}

		// Add arcs for field effects
		for _, effect := range a.Effects {
			model.Arcs = append(model.Arcs, goflowmodel.Arc{
				From:  a.ID,
				To:    effect.Field,
				Value: effect.Value,
			})
		}
	}

	// Note: Access rules are stored in extensions, not in the core model.
	// Use EntityExtension to retrieve access rules for this entity.

	return model
}

// NewApplicationSpecFromLegacy creates an ApplicationSpec from a Model.
// The Model now only contains core Petri net elements (places, transitions, arcs).
// Application constructs (roles, views, etc.) should be added via extensions.
//
// Example:
//
//	app := NewApplicationSpecFromLegacy(model)
//	roles := NewRoleExtension()
//	roles.AddRole(Role{ID: "admin", Name: "Administrator"})
//	app.WithRoles(roles)
func NewApplicationSpecFromLegacy(model *goflowmodel.Model) *ApplicationSpec {
	return NewApplicationSpec(model)
}

// MigrateModel creates an ApplicationSpec from a Model.
// This is an alias for NewApplicationSpecFromLegacy for backwards compatibility.
func MigrateModel(model *goflowmodel.Model) *ApplicationSpec {
	return NewApplicationSpec(model)
}
