package mcp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/esmodules"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

// TestPetriApplicationConversion tests Entity to Model conversion
func TestPetriApplicationConversion(t *testing.T) {
	// Load example Application spec
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}

	// Test conversion pipeline
	entity := app.Entities[0]
	
	// Convert Entity -> metamodel.Schema -> schema.Model
	metaSchema := entity.ToSchema()
	if metaSchema == nil {
		t.Fatal("ToSchema() returned nil")
	}
	
	model := metaSchema.ToModel()
	if model == nil {
		t.Fatal("ToModel() returned nil")
	}
	
	// Verify model structure
	if model.Name != "task" {
		t.Errorf("Expected model name 'task', got '%s'", model.Name)
	}
	
	// Verify places (states + fields)
	if len(model.Places) == 0 {
		t.Error("Expected model to have places")
	}
	
	// Verify transitions (actions)
	expectedActions := 5 // create, submit, assign, complete, cancel
	if len(model.Transitions) != expectedActions {
		t.Errorf("Expected %d transitions, got %d", expectedActions, len(model.Transitions))
	}
	
	t.Logf("Successfully converted Entity to Model with %d places and %d transitions",
		len(model.Places), len(model.Transitions))
}

// TestAccessControlContext tests access rule context building
func TestAccessControlContext(t *testing.T) {
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}

	entity := app.Entities[0]
	
	// Build access rule contexts
	var accessRules []golang.AccessRuleContext
	for _, rule := range entity.Access {
		accessRules = append(accessRules, golang.AccessRuleContext{
			TransitionID: rule.Action,
			Roles:        rule.Roles,
			Guard:        rule.Guard,
		})
	}
	
	// Verify access rules were built
	if len(accessRules) != 5 {
		t.Errorf("Expected 5 access rules, got %d", len(accessRules))
	}
	
	// Check specific rules
	foundCreateRule := false
	foundAssignRule := false
	for _, rule := range accessRules {
		if rule.TransitionID == "create" {
			foundCreateRule = true
			if len(rule.Roles) != 2 {
				t.Errorf("Expected create rule to have 2 roles, got %d", len(rule.Roles))
			}
		}
		if rule.TransitionID == "assign" {
			foundAssignRule = true
			if len(rule.Roles) != 1 || rule.Roles[0] != "admin" {
				t.Error("Expected assign rule to require admin role only")
			}
		}
	}
	
	if !foundCreateRule {
		t.Error("Expected to find create access rule")
	}
	if !foundAssignRule {
		t.Error("Expected to find assign access rule")
	}
	
	t.Logf("Successfully built %d access rule contexts", len(accessRules))
}

// TestRoleContext tests role context building with inheritance
func TestRoleContext(t *testing.T) {
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}
	
	// Build role contexts
	var roles []golang.RoleContext
	for _, role := range app.Roles {
		roles = append(roles, golang.RoleContext{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Inherits:    role.Inherits,
		})
	}
	
	// Verify roles
	if len(roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(roles))
	}
	
	// Check for admin role with inheritance
	foundAdminInheritance := false
	for _, role := range roles {
		if role.ID == "admin" {
			if len(role.Inherits) > 0 && role.Inherits[0] == "user" {
				foundAdminInheritance = true
			}
		}
	}
	
	if !foundAdminInheritance {
		t.Error("Expected admin role to inherit from user")
	}
	
	t.Logf("Successfully built %d role contexts with inheritance", len(roles))
}

// TestPageContext tests page context building for navigation
func TestPageContext(t *testing.T) {
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}
	
	// Build page contexts
	var pageContexts []esmodules.PageContext
	for _, page := range app.Pages {
		pageContexts = append(pageContexts, esmodules.PageContext{
			ID:            page.ID,
			Title:         page.Name,
			Path:          page.Path,
			Icon:          page.Icon,
			LayoutType:    page.Layout.Type,
			EntityID:      page.Layout.Entity,
			RequiredRoles: page.Access,
			ComponentName: toPascalCase(page.ID),
		})
	}
	
	// Verify pages
	if len(pageContexts) != 3 {
		t.Errorf("Expected 3 pages, got %d", len(pageContexts))
	}
	
	// Check specific pages
	foundListPage := false
	foundDetailPage := false
	foundFormPage := false
	
	for _, page := range pageContexts {
		switch page.LayoutType {
		case "list":
			foundListPage = true
			if page.Path != "/tasks" {
				t.Errorf("Expected list page path '/tasks', got '%s'", page.Path)
			}
		case "detail":
			foundDetailPage = true
			if page.Path != "/tasks/:id" {
				t.Errorf("Expected detail page path '/tasks/:id', got '%s'", page.Path)
			}
		case "form":
			foundFormPage = true
			if page.Path != "/tasks/new" {
				t.Errorf("Expected form page path '/tasks/new', got '%s'", page.Path)
			}
		}
	}
	
	if !foundListPage {
		t.Error("Expected to find list layout page")
	}
	if !foundDetailPage {
		t.Error("Expected to find detail layout page")
	}
	if !foundFormPage {
		t.Error("Expected to find form layout page")
	}
	
	t.Logf("Successfully built %d page contexts", len(pageContexts))
}

// TestCodeGenerationWithAccessControl tests that generators can accept access rules
func TestCodeGenerationWithAccessControl(t *testing.T) {
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}

	entity := app.Entities[0]
	metaSchema := entity.ToSchema()
	model := metaSchema.ToModel()
	
	// Build access rules and roles
	var accessRules []golang.AccessRuleContext
	for _, rule := range entity.Access {
		accessRules = append(accessRules, golang.AccessRuleContext{
			TransitionID: rule.Action,
			Roles:        rule.Roles,
			Guard:        rule.Guard,
		})
	}
	
	var roles []golang.RoleContext
	for _, role := range app.Roles {
		roles = append(roles, golang.RoleContext{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Inherits:    role.Inherits,
		})
	}
	
	// Create context with access rules
	ctx, err := golang.NewContext(model, golang.ContextOptions{
		PackageName: "testpkg",
		AccessRules: accessRules,
		Roles:       roles,
	})
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Verify context includes access rules
	if len(ctx.AccessRules) != 5 {
		t.Errorf("Expected 5 access rules in context, got %d", len(ctx.AccessRules))
	}
	
	if len(ctx.Roles) != 2 {
		t.Errorf("Expected 2 roles in context, got %d", len(ctx.Roles))
	}
	
	t.Logf("Successfully created golang context with %d access rules and %d roles",
		len(ctx.AccessRules), len(ctx.Roles))
}

// TestCodeGenerationWithPages tests that generators can accept pages
func TestCodeGenerationWithPages(t *testing.T) {
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read example: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}

	entity := app.Entities[0]
	metaSchema := entity.ToSchema()
	model := metaSchema.ToModel()
	
	// Build page contexts
	var pageContexts []esmodules.PageContext
	for _, page := range app.Pages {
		pageContexts = append(pageContexts, esmodules.PageContext{
			ID:            page.ID,
			Title:         page.Name,
			Path:          page.Path,
			Icon:          page.Icon,
			LayoutType:    page.Layout.Type,
			EntityID:      page.Layout.Entity,
			RequiredRoles: page.Access,
			ComponentName: toPascalCase(page.ID),
		})
	}
	
	// Create context with pages
	ctx, err := esmodules.NewContext(model, esmodules.ContextOptions{
		ProjectName: "test-app",
		Pages:       pageContexts,
	})
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Verify context includes pages
	if len(ctx.Pages) != 3 {
		t.Errorf("Expected 3 pages in context, got %d", len(ctx.Pages))
	}
	
	// Verify page details
	for _, page := range ctx.Pages {
		if page.ComponentName == "" {
			t.Error("Expected component name to be set")
		}
		if page.Path == "" {
			t.Error("Expected path to be set")
		}
	}
	
	t.Logf("Successfully created react context with %d pages", len(ctx.Pages))
}
