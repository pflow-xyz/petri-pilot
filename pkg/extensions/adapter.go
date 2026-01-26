// Package extensions provides adapters for integrating the new extension-based
// ApplicationSpec with the existing petri-pilot codegen system.
package extensions

import (
	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// ToLegacyModel converts an ApplicationSpec back to a legacy go-pflow Model.
// This allows the new extension-based workflow to integrate with the existing
// petri-pilot codegen Context which expects a Model.
//
// Example usage:
//
//	app := NewApplicationSpec(model)
//	app.WithEntities(entities)
//	app.WithRoles(roles)
//
//	legacyModel := ToLegacyModel(app)
//	ctx, err := golang.NewContext(legacyModel, opts)
func ToLegacyModel(app *ApplicationSpec) *goflowmodel.Model {
	if app.Net == nil {
		return &goflowmodel.Model{}
	}

	model := app.Net

	// Add roles from extension
	if roles := app.Roles(); roles != nil {
		for _, r := range roles.Roles {
			model.Roles = append(model.Roles, goflowmodel.Role{
				ID:           r.ID,
				Name:         r.Name,
				Description:  r.Description,
				Inherits:     r.Inherits,
				DynamicGrant: r.DynamicGrant,
			})
		}
	}

	// Add views from extension
	if views := app.Views(); views != nil {
		for _, v := range views.Views {
			view := goflowmodel.View{
				ID:          v.ID,
				Name:        v.Name,
				Kind:        v.Kind,
				Description: v.Description,
				Actions:     v.Actions,
			}
			for _, g := range v.Groups {
				group := goflowmodel.ViewGroup{
					ID:   g.ID,
					Name: g.Name,
				}
				for _, f := range g.Fields {
					group.Fields = append(group.Fields, goflowmodel.ViewField{
						Binding:     f.Binding,
						Label:       f.Label,
						Type:        f.Type,
						Required:    f.Required,
						ReadOnly:    f.ReadOnly,
						Placeholder: f.Placeholder,
					})
				}
				view.Groups = append(view.Groups, group)
			}
			model.Views = append(model.Views, view)
		}

		// Add admin config
		if views.Admin != nil {
			model.Admin = &goflowmodel.Admin{
				Enabled:  views.Admin.Enabled,
				Path:     views.Admin.Path,
				Roles:    views.Admin.Roles,
				Features: views.Admin.Features,
			}
		}
	}

	// Add navigation from page extension
	if pages := app.Pages(); pages != nil && pages.Navigation != nil {
		model.Navigation = &goflowmodel.Navigation{
			Brand: pages.Navigation.Brand,
		}
		for _, item := range pages.Navigation.Items {
			model.Navigation.Items = append(model.Navigation.Items, goflowmodel.NavigationItem{
				Label: item.Label,
				Path:  item.Path,
				Icon:  item.Icon,
				Roles: item.Roles,
			})
		}
	}

	// Add access rules from entities
	if entities := app.Entities(); entities != nil {
		for _, entity := range entities.Entities {
			for _, access := range entity.Access {
				model.Access = append(model.Access, goflowmodel.AccessRule{
					Transition: access.Action,
					Roles:      access.Roles,
					Guard:      access.Guard,
				})
			}
		}
	}

	return model
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

	// Add access rules
	for _, access := range entity.Access {
		model.Access = append(model.Access, goflowmodel.AccessRule{
			Transition: access.Action,
			Roles:      access.Roles,
			Guard:      access.Guard,
		})
	}

	return model
}

// NewApplicationSpecFromLegacy creates an ApplicationSpec from a legacy Model.
// This allows existing models to be wrapped in the new extension-based format.
func NewApplicationSpecFromLegacy(model *goflowmodel.Model) *ApplicationSpec {
	app := NewApplicationSpec(model)

	// Extract roles to extension
	if len(model.Roles) > 0 {
		roles := NewRoleExtension()
		for _, r := range model.Roles {
			roles.AddRole(Role{
				ID:           r.ID,
				Name:         r.Name,
				Description:  r.Description,
				Inherits:     r.Inherits,
				DynamicGrant: r.DynamicGrant,
			})
		}
		app.WithRoles(roles)
	}

	// Extract views to extension
	if len(model.Views) > 0 || model.Admin != nil {
		views := NewViewExtension()
		for _, v := range model.Views {
			view := View{
				ID:          v.ID,
				Name:        v.Name,
				Kind:        v.Kind,
				Description: v.Description,
				Actions:     v.Actions,
			}
			for _, g := range v.Groups {
				group := ViewGroup{
					ID:   g.ID,
					Name: g.Name,
				}
				for _, f := range g.Fields {
					group.Fields = append(group.Fields, ViewField{
						Binding:     f.Binding,
						Label:       f.Label,
						Type:        f.Type,
						Required:    f.Required,
						ReadOnly:    f.ReadOnly,
						Placeholder: f.Placeholder,
					})
				}
				view.Groups = append(view.Groups, group)
			}
			views.AddView(view)
		}
		if model.Admin != nil {
			views.SetAdmin(Admin{
				Enabled:  model.Admin.Enabled,
				Path:     model.Admin.Path,
				Roles:    model.Admin.Roles,
				Features: model.Admin.Features,
			})
		}
		app.WithViews(views)
	}

	// Extract navigation to pages extension
	if model.Navigation != nil {
		pages := NewPageExtension()
		nav := Navigation{
			Brand: model.Navigation.Brand,
		}
		for _, item := range model.Navigation.Items {
			nav.Items = append(nav.Items, NavigationItem{
				Label: item.Label,
				Path:  item.Path,
				Icon:  item.Icon,
				Roles: item.Roles,
			})
		}
		pages.SetNavigation(nav)
		app.WithPages(pages)
	}

	return app
}

// MigrateModel migrates a legacy Model to the modern ApplicationSpec format.
// This is a convenience wrapper around NewApplicationSpecFromLegacy that also
// clears the application constructs from the model to prevent duplication.
func MigrateModel(model *goflowmodel.Model) *ApplicationSpec {
	app := NewApplicationSpecFromLegacy(model)

	// Clear application constructs from the model since they're now in extensions
	// This prevents duplication when converting back
	app.Net.Roles = nil
	app.Net.Access = nil
	app.Net.Views = nil
	app.Net.Navigation = nil
	app.Net.Admin = nil

	return app
}
