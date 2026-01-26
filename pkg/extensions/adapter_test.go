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

		// Add roles
		roles := NewRoleExtension()
		roles.AddRole(Role{ID: "admin", Name: "Administrator"})
		roles.AddRole(Role{ID: "user", Name: "User", Inherits: []string{"admin"}})
		app.WithRoles(roles)

		// Add views
		views := NewViewExtension()
		views.AddView(View{
			ID:   "order-detail",
			Name: "Order Detail",
			Kind: "detail",
		})
		views.SetAdmin(Admin{Enabled: true, Path: "/admin", Roles: []string{"admin"}})
		app.WithViews(views)

		// Add pages with navigation
		pages := NewPageExtension()
		pages.SetNavigation(Navigation{
			Brand: "OrderApp",
			Items: []NavigationItem{
				{Label: "Orders", Path: "/orders"},
				{Label: "Admin", Path: "/admin", Roles: []string{"admin"}},
			},
		})
		app.WithPages(pages)

		// Convert back to legacy
		legacy := ToLegacyModel(app)

		// Verify structure preserved
		if legacy.Name != "test-workflow" {
			t.Errorf("expected name 'test-workflow', got %q", legacy.Name)
		}

		// Verify roles transferred
		if len(legacy.Roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(legacy.Roles))
		}
		foundAdmin := false
		for _, r := range legacy.Roles {
			if r.ID == "admin" {
				foundAdmin = true
				if r.Name != "Administrator" {
					t.Errorf("expected admin name 'Administrator', got %q", r.Name)
				}
			}
		}
		if !foundAdmin {
			t.Error("expected to find admin role")
		}

		// Verify views transferred
		if len(legacy.Views) != 1 {
			t.Errorf("expected 1 view, got %d", len(legacy.Views))
		}

		// Verify admin transferred
		if legacy.Admin == nil {
			t.Fatal("expected admin to be set")
		}
		if !legacy.Admin.Enabled {
			t.Error("expected admin to be enabled")
		}

		// Verify navigation transferred
		if legacy.Navigation == nil {
			t.Fatal("expected navigation to be set")
		}
		if legacy.Navigation.Brand != "OrderApp" {
			t.Errorf("expected brand 'OrderApp', got %q", legacy.Navigation.Brand)
		}
		if len(legacy.Navigation.Items) != 2 {
			t.Errorf("expected 2 nav items, got %d", len(legacy.Navigation.Items))
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

	// Verify access rules
	if len(model.Access) != 1 {
		t.Errorf("expected 1 access rule, got %d", len(model.Access))
	}
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
		Roles: []goflowmodel.Role{
			{ID: "admin", Name: "Administrator"},
			{ID: "user", Name: "User"},
		},
		Views: []goflowmodel.View{
			{
				ID:   "detail",
				Name: "Detail View",
				Kind: "detail",
				Groups: []goflowmodel.ViewGroup{
					{
						ID:   "info",
						Name: "Information",
						Fields: []goflowmodel.ViewField{
							{Binding: "name", Label: "Name", Type: "text"},
						},
					},
				},
			},
		},
		Navigation: &goflowmodel.Navigation{
			Brand: "TestApp",
			Items: []goflowmodel.NavigationItem{
				{Label: "Home", Path: "/"},
				{Label: "Admin", Path: "/admin", Roles: []string{"admin"}},
			},
		},
		Admin: &goflowmodel.Admin{
			Enabled: true,
			Path:    "/admin",
			Roles:   []string{"admin"},
		},
	}

	app := NewApplicationSpecFromLegacy(model)

	// Verify roles extracted
	if !app.HasRoles() {
		t.Error("expected HasRoles to be true")
	}
	roles := app.Roles()
	if len(roles.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles.Roles))
	}

	// Verify views extracted
	if !app.HasViews() {
		t.Error("expected HasViews to be true")
	}
	views := app.Views()
	if len(views.Views) != 1 {
		t.Errorf("expected 1 view, got %d", len(views.Views))
	}
	if views.Admin == nil {
		t.Error("expected admin to be set")
	}

	// Verify navigation extracted
	if !app.HasNavigation() {
		t.Error("expected HasNavigation to be true")
	}
	pages := app.Pages()
	if pages.Navigation.Brand != "TestApp" {
		t.Errorf("expected brand 'TestApp', got %q", pages.Navigation.Brand)
	}
}

func TestMigrateModel(t *testing.T) {
	model := &goflowmodel.Model{
		Name: "migrate-me",
		Places: []goflowmodel.Place{
			{ID: "p1"},
		},
		Roles: []goflowmodel.Role{
			{ID: "admin"},
		},
		Views: []goflowmodel.View{
			{ID: "v1"},
		},
	}

	app := MigrateModel(model)

	// Verify extensions exist
	if !app.HasRoles() {
		t.Error("expected roles to be migrated")
	}
	if !app.HasViews() {
		t.Error("expected views to be migrated")
	}

	// Verify model was cleared
	if len(app.Net.Roles) != 0 {
		t.Errorf("expected roles to be cleared, got %d", len(app.Net.Roles))
	}
	if len(app.Net.Views) != 0 {
		t.Errorf("expected views to be cleared, got %d", len(app.Net.Views))
	}
}

func TestRoundTripLegacy(t *testing.T) {
	// Create a legacy model with application constructs
	original := &goflowmodel.Model{
		Name:        "roundtrip-test",
		Description: "Test round trip",
		Places: []goflowmodel.Place{
			{ID: "pending", Initial: 1, Kind: goflowmodel.TokenKind},
		},
		Transitions: []goflowmodel.Transition{
			{ID: "confirm"},
		},
		Arcs: []goflowmodel.Arc{
			{From: "pending", To: "confirm"},
		},
		Roles: []goflowmodel.Role{
			{ID: "admin", Name: "Administrator", Description: "Admin role"},
		},
		Views: []goflowmodel.View{
			{ID: "detail", Name: "Detail", Kind: "detail"},
		},
		Navigation: &goflowmodel.Navigation{
			Brand: "TestBrand",
			Items: []goflowmodel.NavigationItem{
				{Label: "Home", Path: "/"},
			},
		},
	}

	// Convert to extension-based format
	app := NewApplicationSpecFromLegacy(original)

	// Convert back to legacy
	result := ToLegacyModel(app)

	// Verify core structure preserved
	if result.Name != original.Name {
		t.Errorf("name mismatch: %q vs %q", result.Name, original.Name)
	}
	if len(result.Places) != len(original.Places) {
		t.Errorf("place count mismatch: %d vs %d", len(result.Places), len(original.Places))
	}

	// Verify roles preserved
	if len(result.Roles) != len(original.Roles) {
		t.Errorf("role count mismatch: %d vs %d", len(result.Roles), len(original.Roles))
	}
	if result.Roles[0].ID != original.Roles[0].ID {
		t.Errorf("role ID mismatch: %q vs %q", result.Roles[0].ID, original.Roles[0].ID)
	}

	// Verify views preserved
	if len(result.Views) != len(original.Views) {
		t.Errorf("view count mismatch: %d vs %d", len(result.Views), len(original.Views))
	}

	// Verify navigation preserved
	if result.Navigation == nil {
		t.Fatal("navigation should be preserved")
	}
	if result.Navigation.Brand != original.Navigation.Brand {
		t.Errorf("nav brand mismatch: %q vs %q", result.Navigation.Brand, original.Navigation.Brand)
	}
}
