package mcp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/react"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
)

// TestCompleteApplicationGeneration tests end-to-end generation of a complete application
func TestCompleteApplicationGeneration(t *testing.T) {
	// Load the task-manager-app.json example
	data, err := os.ReadFile("../../examples/task-manager-app.json")
	if err != nil {
		t.Fatalf("Failed to read task-manager-app.json: %v", err)
	}

	var app metamodel.Application
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatalf("Failed to parse Application: %v", err)
	}

	// Verify application structure
	if len(app.Entities) == 0 {
		t.Fatal("Application must have at least one entity")
	}

	entity := app.Entities[0]
	
	// Convert Entity to Model
	metaSchema := entity.ToSchema()
	model := metaSchema.ToModel()

	// Build access rules
	var accessRules []golang.AccessRuleContext
	for _, rule := range entity.Access {
		accessRules = append(accessRules, golang.AccessRuleContext{
			TransitionID: rule.Action,
			Roles:        rule.Roles,
			Guard:        rule.Guard,
		})
	}

	// Build roles
	var roles []golang.RoleContext
	for _, role := range app.Roles {
		roles = append(roles, golang.RoleContext{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Inherits:    role.Inherits,
		})
	}

	// Build workflows
	var workflows []golang.WorkflowContext
	for _, wf := range app.Workflows {
		trigger := golang.WorkflowTriggerContext{
			Type:   wf.Trigger.Type,
			Entity: wf.Trigger.Entity,
			Action: wf.Trigger.Action,
			Cron:   wf.Trigger.Cron,
		}
		
		var steps []golang.WorkflowStepContext
		for _, step := range wf.Steps {
			steps = append(steps, golang.WorkflowStepContext{
				ID:         step.ID,
				PascalName: toPascalCase(step.ID),
				Type:       step.Type,
				Entity:     step.Entity,
				Action:     step.Action,
				Condition:  step.Condition,
				Input:      step.Input,
				OnSuccess:  step.OnSuccess,
				OnFailure:  step.OnFailure,
			})
		}
		
		workflows = append(workflows, golang.WorkflowContext{
			ID:          wf.ID,
			Name:        wf.Name,
			Description: wf.Description,
			PascalName:  toPascalCase(wf.ID),
			CamelName:   toCamelCase(wf.ID),
			TriggerType: wf.Trigger.Type,
			Trigger:     trigger,
			Steps:       steps,
		})
	}

	// Generate backend
	gen, err := golang.New(golang.Options{
		PackageName:  entity.ID,
		IncludeTests: true,
		IncludeInfra: true,
		IncludeAuth:  true,
	})
	if err != nil {
		t.Fatalf("Failed to create golang generator: %v", err)
	}

	files, err := generateBackendWithAccessControl(gen, model, accessRules, roles, workflows)
	if err != nil {
		t.Fatalf("Failed to generate backend: %v", err)
	}

	// Verify expected files are generated
	expectedFiles := []string{
		"go.mod",
		"main.go",
		"workflow.go",
		"events.go",
		"aggregate.go",
		"api.go",
		"openapi.yaml",
		"workflow_test.go",
		"migrations/001_init.sql",
		"Dockerfile",
		"docker-compose.yaml",
		"auth.go",
		"middleware.go",
		"workflows.go", // Should be generated because we have workflows
	}

	fileMap := make(map[string]bool)
	for _, file := range files {
		fileMap[file.Name] = true
	}

	for _, expected := range expectedFiles {
		if !fileMap[expected] {
			t.Errorf("Expected file not generated: %s", expected)
		}
	}

	t.Logf("Successfully generated %d backend files", len(files))
}

// TestAccessControlIntegration tests that access control is properly wired
func TestAccessControlIntegration(t *testing.T) {
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

	// Build access rules
	var accessRules []golang.AccessRuleContext
	for _, rule := range entity.Access {
		accessRules = append(accessRules, golang.AccessRuleContext{
			TransitionID: rule.Action,
			Roles:        rule.Roles,
			Guard:        rule.Guard,
		})
	}

	// Build roles
	var roles []golang.RoleContext
	for _, role := range app.Roles {
		roles = append(roles, golang.RoleContext{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Inherits:    role.Inherits,
		})
	}

	gen, err := golang.New(golang.Options{
		PackageName: entity.ID,
		IncludeAuth: true,
	})
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	files, err := generateBackendWithAccessControl(gen, model, accessRules, roles, nil)
	if err != nil {
		t.Fatalf("Failed to generate backend: %v", err)
	}

	// Verify middleware.go is generated
	var middlewareFile *golang.GeneratedFile
	var mainFile *golang.GeneratedFile
	var apiFile *golang.GeneratedFile
	
	for i := range files {
		switch files[i].Name {
		case "middleware.go":
			middlewareFile = &files[i]
		case "main.go":
			mainFile = &files[i]
		case "api.go":
			apiFile = &files[i]
		}
	}

	if middlewareFile == nil {
		t.Fatal("middleware.go not generated")
	}
	if mainFile == nil {
		t.Fatal("main.go not generated")
	}
	if apiFile == nil {
		t.Fatal("api.go not generated")
	}

	// Verify middleware content
	middlewareContent := string(middlewareFile.Content)
	if !strings.Contains(middlewareContent, "RequirePermission") {
		t.Error("middleware.go missing RequirePermission function")
	}
	if !strings.Contains(middlewareContent, "dsl.Evaluate") {
		t.Error("middleware.go not using dsl.Evaluate for guard evaluation")
	}

	// Verify main.go initializes middleware
	mainContent := string(mainFile.Content)
	if !strings.Contains(mainContent, "NewMiddleware") {
		t.Error("main.go not initializing middleware")
	}
	if !strings.Contains(mainContent, "accessRules") {
		t.Error("main.go not defining access rules")
	}

	// Verify api.go uses RequirePermission for protected routes
	apiContent := string(apiFile.Content)
	if !strings.Contains(apiContent, "RequirePermission") {
		t.Error("api.go not using RequirePermission middleware")
	}

	t.Logf("Access control integration verified successfully")
}

// TestPageNavigationIntegration tests that pages and navigation are properly wired
func TestPageNavigationIntegration(t *testing.T) {
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
	var pageContexts []react.PageContext
	for _, page := range app.Pages {
		if page.Layout.Entity == entity.ID || page.Layout.Entity == "" {
			pageContexts = append(pageContexts, react.PageContext{
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
	}

	gen, err := react.New(react.Options{
		ProjectName: app.Name + "-" + entity.ID,
		APIBaseURL:  "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("Failed to create react generator: %v", err)
	}

	files, err := generateFrontendWithPages(gen, model, pageContexts)
	if err != nil {
		t.Fatalf("Failed to generate frontend: %v", err)
	}

	// Verify expected files
	expectedFiles := []string{
		"package.json",
		"vite.config.js",
		"index.html",
		"src/main.js",
		"src/router.js",
		"src/navigation.js",
		"src/pages.js",
	}

	fileMap := make(map[string]bool)
	for _, file := range files {
		fileMap[file.Name] = true
	}

	for _, expected := range expectedFiles {
		if !fileMap[expected] {
			t.Errorf("Expected frontend file not generated: %s", expected)
		}
	}

	// Verify router.js contains page routes
	var routerFile *react.GeneratedFile
	var navigationFile *react.GeneratedFile
	var pagesFile *react.GeneratedFile

	for i := range files {
		switch files[i].Name {
		case "src/router.js":
			routerFile = &files[i]
		case "src/navigation.js":
			navigationFile = &files[i]
		case "src/pages.js":
			pagesFile = &files[i]
		}
	}

	if routerFile == nil {
		t.Fatal("router.js not generated")
	}
	if navigationFile == nil {
		t.Fatal("navigation.js not generated")
	}
	if pagesFile == nil {
		t.Fatal("pages.js not generated")
	}

	routerContent := string(routerFile.Content)
	navigationContent := string(navigationFile.Content)
	pagesContent := string(pagesFile.Content)

	// Verify routes are defined
	for _, page := range pageContexts {
		if !strings.Contains(routerContent, page.Path) {
			t.Errorf("router.js missing route for page: %s", page.ID)
		}
	}

	// Verify navigation includes pages
	if !strings.Contains(navigationContent, "navigation") {
		t.Error("navigation.js missing navigation logic")
	}

	// Verify pages.js has page components
	if !strings.Contains(pagesContent, "List") || !strings.Contains(pagesContent, "Detail") {
		t.Error("pages.js missing list/detail layouts")
	}

	t.Logf("Page/navigation integration verified successfully")
}

// TestWorkflowIntegration tests that workflows are properly wired
func TestWorkflowIntegration(t *testing.T) {
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

	// Build workflow contexts
	var workflows []golang.WorkflowContext
	for _, wf := range app.Workflows {
		trigger := golang.WorkflowTriggerContext{
			Type:   wf.Trigger.Type,
			Entity: wf.Trigger.Entity,
			Action: wf.Trigger.Action,
			Cron:   wf.Trigger.Cron,
		}
		
		var steps []golang.WorkflowStepContext
		for _, step := range wf.Steps {
			steps = append(steps, golang.WorkflowStepContext{
				ID:         step.ID,
				PascalName: toPascalCase(step.ID),
				Type:       step.Type,
				Entity:     step.Entity,
				Action:     step.Action,
				Condition:  step.Condition,
				Input:      step.Input,
				OnSuccess:  step.OnSuccess,
				OnFailure:  step.OnFailure,
			})
		}
		
		workflows = append(workflows, golang.WorkflowContext{
			ID:          wf.ID,
			Name:        wf.Name,
			Description: wf.Description,
			PascalName:  toPascalCase(wf.ID),
			CamelName:   toCamelCase(wf.ID),
			TriggerType: wf.Trigger.Type,
			Trigger:     trigger,
			Steps:       steps,
		})
	}

	gen, err := golang.New(golang.Options{
		PackageName: entity.ID,
	})
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	files, err := generateBackendWithAccessControl(gen, model, nil, nil, workflows)
	if err != nil {
		t.Fatalf("Failed to generate backend: %v", err)
	}

	// Verify workflows.go is generated
	var workflowsFile *golang.GeneratedFile
	for i := range files {
		if files[i].Name == "workflows.go" {
			workflowsFile = &files[i]
			break
		}
	}

	if workflowsFile == nil {
		t.Fatal("workflows.go not generated despite having workflows")
	}

	workflowsContent := string(workflowsFile.Content)

	// Verify workflow structures exist
	if !strings.Contains(workflowsContent, "WorkflowExecutor") {
		t.Error("workflows.go missing WorkflowExecutor")
	}
	if !strings.Contains(workflowsContent, "WorkflowRegistry") {
		t.Error("workflows.go missing WorkflowRegistry")
	}

	// Verify specific workflow from task-manager-app
	if !strings.Contains(workflowsContent, "TaskNotification") {
		t.Error("workflows.go missing TaskNotification workflow")
	}

	// Verify event trigger handling
	if !strings.Contains(workflowsContent, "OnEvent") {
		t.Error("workflows.go missing OnEvent handler")
	}

	t.Logf("Workflow integration verified successfully")
}
