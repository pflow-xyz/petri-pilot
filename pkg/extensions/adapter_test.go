package extensions

import (
	"testing"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

func TestToLegacyModel(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		model := &goflowmodel.Model{
			Name: "test-workflow",
			Places: []goflowmodel.Place{
				{ID: "pending", Initial: 1, Kind: goflowmodel.TokenKind},
				{ID: "confirmed", Initial: 0, Kind: goflowmodel.TokenKind},
			},
			Transitions: []goflowmodel.Transition{
				{ID: "confirm"},
			},
			Arcs: []goflowmodel.Arc{
				{From: "pending", To: "confirm"},
				{From: "confirm", To: "confirmed"},
			},
		}

		app := NewApplicationSpec(model)

		// Add roles to extension
		roles := NewRoleExtension()
		roles.AddRole(Role{ID: "admin", Name: "Administrator"})
		roles.AddRole(Role{ID: "user", Name: "User", Inherits: []string{"admin"}})
		app.WithRoles(roles)

		// Add views to extension
		views := NewViewExtension()
		views.AddView(View{
			ID:   "order-detail",
			Name: "Order Detail",
			Kind: "detail",
		})
		views.SetAdmin(Admin{Enabled: true, Path: "/admin", Roles: []string{"admin"}})
		app.WithViews(views)

		// Add pages with navigation to extension
		pages := NewPageExtension()
		pages.SetNavigation(Navigation{
			Brand: "OrderApp",
			Items: []NavigationItem{
				{Label: "Orders", Path: "/orders"},
				{Label: "Admin", Path: "/admin", Roles: []string{"admin"}},
			},
		})
		app.WithPages(pages)

		// Convert to legacy model (now returns core model only)
		legacy := ToLegacyModel(app)

		// Verify core structure preserved
		if legacy.Name != "test-workflow" {
			t.Errorf("expected name 'test-workflow', got %q", legacy.Name)
		}
		if len(legacy.Places) != 2 {
			t.Errorf("expected 2 places, got %d", len(legacy.Places))
		}
		if len(legacy.Transitions) != 1 {
			t.Errorf("expected 1 transition, got %d", len(legacy.Transitions))
		}
		if len(legacy.Arcs) != 2 {
			t.Errorf("expected 2 arcs, got %d", len(legacy.Arcs))
		}

		// Verify application constructs are in extensions (not in model)
		if !app.HasRoles() {
			t.Error("expected HasRoles to be true")
		}
		if !app.HasViews() {
			t.Error("expected HasViews to be true")
		}
		if !app.HasAdmin() {
			t.Error("expected HasAdmin to be true")
		}
		if !app.HasNavigation() {
			t.Error("expected HasNavigation to be true")
		}
	})

	t.Run("nil app", func(t *testing.T) {
		app := &ApplicationSpec{ExtendedModel: goflowmodel.NewExtendedModel(nil)}
		legacy := ToLegacyModel(app)
		if legacy == nil {
			t.Error("expected non-nil model")
		}
	})
}

func TestEntityToModel(t *testing.T) {
	entity := Entity{
		ID:          "order",
		Name:        "Order",
		Description: "Order entity",
		Fields: []Field{
			{ID: "total", Type: FieldTypeInt64, Required: true},
			{ID: "customer", Type: FieldTypeString},
		},
		States: []EntityState{
			{ID: "pending", Initial: true},
			{ID: "confirmed"},
			{ID: "shipped"},
		},
		Actions: []EntityAction{
			{
				ID:         "confirm",
				FromStates: []string{"pending"},
				ToState:    "confirmed",
				Guard:      "total > 0",
				Input: []ActionParam{
					{ID: "note", Type: FieldTypeString},
				},
				Effects: []ActionEffect{
					{Field: "status", Value: "confirmed"},
				},
			},
			{
				ID:         "ship",
				FromStates: []string{"confirmed"},
				ToState:    "shipped",
			},
		},
		Access: []AccessRule{
			{Action: "confirm", Roles: []string{"admin"}},
		},
	}

	model := EntityToModel(entity)

	// Verify basic info
	if model.Name != "order" {
		t.Errorf("expected name 'order', got %q", model.Name)
	}

	// Verify places (fields + states)
	expectedPlaces := len(entity.Fields) + len(entity.States) // 2 fields + 3 states = 5
	if len(model.Places) != expectedPlaces {
		t.Errorf("expected %d places, got %d", expectedPlaces, len(model.Places))
	}

	// Verify data places (fields)
	dataPlaces := 0
	for _, p := range model.Places {
		if p.Kind == goflowmodel.DataKind {
			dataPlaces++
		}
	}
	if dataPlaces != len(entity.Fields) {
		t.Errorf("expected %d data places, got %d", len(entity.Fields), dataPlaces)
	}

	// Verify token places (states) with initial marking
	pendingFound := false
	for _, p := range model.Places {
		if p.ID == "pending" {
			pendingFound = true
			if p.Initial != 1 {
				t.Errorf("expected pending initial 1, got %d", p.Initial)
			}
			if p.Kind != goflowmodel.TokenKind {
				t.Errorf("expected pending to be TokenKind, got %v", p.Kind)
			}
		}
	}
	if !pendingFound {
		t.Error("expected to find pending place")
	}

	// Verify transitions
	if len(model.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(model.Transitions))
	}

	confirmFound := false
	for _, trans := range model.Transitions {
		if trans.ID == "confirm" {
			confirmFound = true
			if trans.Guard != "total > 0" {
				t.Errorf("expected guard 'total > 0', got %q", trans.Guard)
			}
			if len(trans.Bindings) != 1 {
				t.Errorf("expected 1 binding, got %d", len(trans.Bindings))
			}
		}
	}
	if !confirmFound {
		t.Error("expected to find confirm transition")
	}

	// Verify arcs (from state -> action, action -> to state, action -> effect field)
	// confirm: 1 from + 1 to + 1 effect = 3
	// ship: 1 from + 1 to = 2
	// Total = 5
	expectedArcs := 5
	if len(model.Arcs) != expectedArcs {
		t.Errorf("expected %d arcs, got %d", expectedArcs, len(model.Arcs))
	}

	// Note: Access rules are now stored in extensions, not in the model
	// The entity.Access is available for codegen to use via the EntityExtension
}

func TestNewApplicationSpecFromLegacy(t *testing.T) {
	model := &goflowmodel.Model{
		Name: "legacy-model",
		Places: []goflowmodel.Place{
			{ID: "p1", Initial: 1},
		},
		Transitions: []goflowmodel.Transition{
			{ID: "t1"},
		},
	}

	app := NewApplicationSpecFromLegacy(model)

	// Verify core model is wrapped
	if app.Net.Name != "legacy-model" {
		t.Errorf("expected name 'legacy-model', got %q", app.Net.Name)
	}
	if len(app.Net.Places) != 1 {
		t.Errorf("expected 1 place, got %d", len(app.Net.Places))
	}

	// Application constructs should be added via extensions
	// The model no longer has Roles, Views, etc. embedded
	if app.HasRoles() {
		t.Error("expected HasRoles to be false initially")
	}
	if app.HasViews() {
		t.Error("expected HasViews to be false initially")
	}

	// Add roles via extension
	roles := NewRoleExtension()
	roles.AddRole(Role{ID: "admin", Name: "Administrator"})
	app.WithRoles(roles)

	if !app.HasRoles() {
		t.Error("expected HasRoles to be true after adding roles")
	}
}

func TestMigrateModel(t *testing.T) {
	model := &goflowmodel.Model{
		Name: "migrate-me",
		Places: []goflowmodel.Place{
			{ID: "p1"},
		},
	}

	app := MigrateModel(model)

	// Verify model is wrapped
	if app.Net.Name != "migrate-me" {
		t.Errorf("expected name 'migrate-me', got %q", app.Net.Name)
	}

	// Extensions should be added separately
	if app.HasRoles() {
		t.Error("expected no roles initially")
	}
	if app.HasViews() {
		t.Error("expected no views initially")
	}
}

func TestApplicationSpecWithExtensions(t *testing.T) {
	// Create a core model with Petri net elements only
	model := &goflowmodel.Model{
		Name:        "extension-test",
		Description: "Test extensions",
		Places: []goflowmodel.Place{
			{ID: "pending", Initial: 1, Kind: goflowmodel.TokenKind},
		},
		Transitions: []goflowmodel.Transition{
			{ID: "confirm"},
		},
		Arcs: []goflowmodel.Arc{
			{From: "pending", To: "confirm"},
		},
	}

	app := NewApplicationSpec(model)

	// Add roles
	roles := NewRoleExtension()
	roles.AddRole(Role{ID: "admin", Name: "Administrator", Description: "Admin role"})
	app.WithRoles(roles)

	// Add views
	views := NewViewExtension()
	views.AddView(View{ID: "detail", Name: "Detail", Kind: "detail"})
	app.WithViews(views)

	// Add navigation
	pages := NewPageExtension()
	pages.SetNavigation(Navigation{
		Brand: "TestBrand",
		Items: []NavigationItem{
			{Label: "Home", Path: "/"},
		},
	})
	app.WithPages(pages)

	// Verify extensions are accessible
	if !app.HasRoles() {
		t.Error("expected HasRoles to be true")
	}
	rolesExt := app.Roles()
	if len(rolesExt.Roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(rolesExt.Roles))
	}
	if rolesExt.Roles[0].ID != "admin" {
		t.Errorf("expected role 'admin', got %q", rolesExt.Roles[0].ID)
	}

	if !app.HasViews() {
		t.Error("expected HasViews to be true")
	}
	viewsExt := app.Views()
	if len(viewsExt.Views) != 1 {
		t.Errorf("expected 1 view, got %d", len(viewsExt.Views))
	}

	if !app.HasNavigation() {
		t.Error("expected HasNavigation to be true")
	}
	pagesExt := app.Pages()
	if pagesExt.Navigation.Brand != "TestBrand" {
		t.Errorf("expected brand 'TestBrand', got %q", pagesExt.Navigation.Brand)
	}

	// Core model is still accessible
	if app.Net.Name != "extension-test" {
		t.Errorf("expected name 'extension-test', got %q", app.Net.Name)
	}
}
