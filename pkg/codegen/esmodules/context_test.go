package esmodules

import (
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

func TestNewContextFromApp(t *testing.T) {
	// Create a model with core Petri net elements
	model := &metamodel.Model{
		Name: "test-app",
		Places: []metamodel.Place{
			{ID: "pending", Initial: 1, Kind: metamodel.TokenKind},
			{ID: "completed", Initial: 0, Kind: metamodel.TokenKind},
		},
		Transitions: []metamodel.Transition{
			{ID: "complete"},
		},
		Arcs: []metamodel.Arc{
			{From: "pending", To: "complete"},
			{From: "complete", To: "completed"},
		},
	}

	// Create ApplicationSpec with extensions
	app := extensions.NewApplicationSpec(model)

	// Add roles extension
	roles := extensions.NewRoleExtension()
	roles.AddRole(extensions.Role{ID: "admin", Name: "Administrator", Description: "Admin role"})
	roles.AddRole(extensions.Role{ID: "user", Name: "User", Description: "User role"})
	app.WithRoles(roles)

	// Add views extension with admin
	views := extensions.NewViewExtension()
	views.SetAdmin(extensions.Admin{Enabled: true, Path: "/admin"})
	app.WithViews(views)

	// Create context from app
	ctx, err := NewContextFromApp(app, ContextOptions{})
	if err != nil {
		t.Fatalf("NewContextFromApp failed: %v", err)
	}

	// Verify basic fields
	if ctx.ModelName != "test-app" {
		t.Errorf("expected model name 'test-app', got %q", ctx.ModelName)
	}

	// Verify places
	if len(ctx.Places) != 2 {
		t.Errorf("expected 2 places, got %d", len(ctx.Places))
	}

	// Verify roles from extension
	if len(ctx.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(ctx.Roles))
	}
	foundAdmin := false
	for _, r := range ctx.Roles {
		if r.ID == "admin" {
			foundAdmin = true
			if r.Description != "Admin role" {
				t.Errorf("expected admin description 'Admin role', got %q", r.Description)
			}
		}
	}
	if !foundAdmin {
		t.Error("expected to find admin role")
	}

	// Verify feature flags
	if !ctx.HasAdmin {
		t.Error("expected HasAdmin to be true")
	}
}
