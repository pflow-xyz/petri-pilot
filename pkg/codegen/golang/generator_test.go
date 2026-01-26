package golang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

func loadTestModel(t *testing.T) *metamodel.Model {
	t.Helper()

	data, err := os.ReadFile("../../../examples/order-processing.json")
	if err != nil {
		t.Fatalf("Failed to read example model: %v", err)
	}

	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("Failed to parse example model: %v", err)
	}

	return &model
}

func TestNaming(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		{"ToPascalCase basic", ToPascalCase, "validate_order", "ValidateOrder"},
		{"ToPascalCase kebab", ToPascalCase, "process-payment", "ProcessPayment"},
		{"ToPascalCase single", ToPascalCase, "validate", "Validate"},
		{"ToPascalCase empty", ToPascalCase, "", ""},

		{"ToCamelCase basic", ToCamelCase, "validate_order", "validateOrder"},
		{"ToCamelCase single", ToCamelCase, "validate", "validate"},

		{"SanitizePackageName basic", SanitizePackageName, "order-processing", "orderprocessing"},
		{"SanitizePackageName mixed", SanitizePackageName, "Order_Processing", "orderprocessing"},
		{"SanitizePackageName numbers", SanitizePackageName, "123workflow", "workflow"},
		{"SanitizePackageName empty", SanitizePackageName, "", "workflow"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.fn(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestToConstName(t *testing.T) {
	tests := []struct {
		prefix   string
		id       string
		expected string
	}{
		{"Place", "received", "PlaceReceived"},
		{"Transition", "validate", "TransitionValidate"},
		{"Place", "order_received", "PlaceOrderReceived"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := ToConstName(tc.prefix, tc.id)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestToHandlerName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"validate", "HandleValidate"},
		{"process_payment", "HandleProcessPayment"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := ToHandlerName(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestNewContext(t *testing.T) {
	model := loadTestModel(t)

	ctx, err := NewContext(model, ContextOptions{})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	// Verify basic fields
	if ctx.ModelName != "order-processing" {
		t.Errorf("expected model name 'order-processing', got %q", ctx.ModelName)
	}

	// Verify places
	if len(ctx.Places) != 6 {
		t.Errorf("expected 6 places, got %d", len(ctx.Places))
	}

	// Verify transitions
	if len(ctx.Transitions) != 5 {
		t.Errorf("expected 5 transitions, got %d", len(ctx.Transitions))
	}

	// Verify package name sanitization
	if ctx.PackageName != "orderprocessing" {
		t.Errorf("expected package name 'orderprocessing', got %q", ctx.PackageName)
	}
}

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
	roles.AddRole(extensions.Role{ID: "admin", Name: "Administrator"})
	roles.AddRole(extensions.Role{ID: "user", Name: "User", Inherits: []string{"admin"}})
	app.WithRoles(roles)

	// Add views extension
	views := extensions.NewViewExtension()
	views.AddView(extensions.View{
		ID:   "detail",
		Name: "Detail View",
		Kind: "detail",
	})
	views.SetAdmin(extensions.Admin{Enabled: true, Path: "/admin", Roles: []string{"admin"}})
	app.WithViews(views)

	// Add pages extension with navigation
	pages := extensions.NewPageExtension()
	pages.SetNavigation(extensions.Navigation{
		Brand: "TestApp",
		Items: []extensions.NavigationItem{
			{Label: "Home", Path: "/"},
			{Label: "Admin", Path: "/admin", Roles: []string{"admin"}},
		},
	})
	app.WithPages(pages)

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
			if r.Name != "Administrator" {
				t.Errorf("expected admin name 'Administrator', got %q", r.Name)
			}
		}
	}
	if !foundAdmin {
		t.Error("expected to find admin role")
	}

	// Verify views from extension
	if len(ctx.Views) != 1 {
		t.Errorf("expected 1 view, got %d", len(ctx.Views))
	}

	// Verify admin from extension
	if ctx.Admin == nil {
		t.Fatal("expected admin to be set")
	}
	if !ctx.Admin.Enabled {
		t.Error("expected admin to be enabled")
	}

	// Verify navigation from extension
	if ctx.Navigation == nil {
		t.Fatal("expected navigation to be set")
	}
	if ctx.Navigation.Brand != "TestApp" {
		t.Errorf("expected brand 'TestApp', got %q", ctx.Navigation.Brand)
	}
	if len(ctx.Navigation.Items) != 2 {
		t.Errorf("expected 2 nav items, got %d", len(ctx.Navigation.Items))
	}
}

func TestTemplates(t *testing.T) {
	tmpl, err := NewTemplates()
	if err != nil {
		t.Fatalf("NewTemplates failed: %v", err)
	}

	model := loadTestModel(t)
	ctx, err := NewContext(model, ContextOptions{})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	// Test each template renders without error
	for _, name := range AllTemplateNames() {
		t.Run(name, func(t *testing.T) {
			content, err := tmpl.Execute(name, ctx)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if len(content) == 0 {
				t.Error("expected non-empty content")
			}
		})
	}
}

func TestGenerateFiles(t *testing.T) {
	model := loadTestModel(t)

	gen, err := New(Options{IncludeTests: true, AsSubmodule: true})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	files, err := gen.GenerateFiles(model)
	if err != nil {
		t.Fatalf("GenerateFiles failed: %v", err)
	}

	// Core files always generated:
	// service.go, workflow.go, events.go, aggregate.go, api.go, openapi.yaml, config.go, workflow_test.go,
	// api_events.go, README.md
	// Note: Application construct files (auth.go, views.go, navigation.go, admin.go, etc.) are only
	// generated when the corresponding extensions are present. The test model has core Petri net
	// elements only; application constructs are now stored in extensions.
	// Note: AsSubmodule mode generates service.go instead of main.go and skips go.mod
	expectedFiles := []string{"service.go", "workflow.go", "events.go", "aggregate.go", "api.go", "openapi.yaml", "config.go", "workflow_test.go", "api_events.go", "README.md"}
	if len(files) != len(expectedFiles) {
		t.Errorf("expected %d files, got %d", len(expectedFiles), len(files))
		for _, f := range files {
			t.Logf("  generated: %s", f.Name)
		}
	}

	fileNames := make(map[string]bool)
	for _, f := range files {
		fileNames[f.Name] = true
	}

	for _, expected := range expectedFiles {
		if !fileNames[expected] {
			t.Errorf("missing expected file: %s", expected)
		}
	}
}

func TestGenerateFilesContent(t *testing.T) {
	model := loadTestModel(t)

	gen, err := New(Options{IncludeTests: true, AsSubmodule: true})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	files, err := gen.GenerateFiles(model)
	if err != nil {
		t.Fatalf("GenerateFiles failed: %v", err)
	}

	for _, f := range files {
		content := string(f.Content)

		switch f.Name {
		case "workflow.go":
			// Should contain place constants
			if !strings.Contains(content, "PlaceReceived") {
				t.Error("workflow.go missing PlaceReceived constant")
			}
			// Should contain transition constants
			if !strings.Contains(content, "TransitionValidate") {
				t.Error("workflow.go missing TransitionValidate constant")
			}

		case "events.go":
			// Should contain event structs
			if !strings.Contains(content, "struct") {
				t.Error("events.go missing event structs")
			}

		case "aggregate.go":
			// Should contain State struct
			if !strings.Contains(content, "type State struct") {
				t.Error("aggregate.go missing State struct")
			}
			// Should contain Aggregate struct
			if !strings.Contains(content, "type Aggregate struct") {
				t.Error("aggregate.go missing Aggregate struct")
			}

		case "api.go":
			// Should contain BuildRouter
			if !strings.Contains(content, "BuildRouter") {
				t.Error("api.go missing BuildRouter function")
			}
			// Should contain handler functions
			if !strings.Contains(content, "HandleValidate") {
				t.Error("api.go missing HandleValidate function")
			}

		case "service.go":
			// Should contain serve.Register call (submodule mode)
			if !strings.Contains(content, "serve.Register") {
				t.Error("service.go missing serve.Register call")
			}
			// Should contain NewService function
			if !strings.Contains(content, "func NewService()") {
				t.Error("service.go missing NewService function")
			}

		case "workflow_test.go":
			// Should contain test functions
			if !strings.Contains(content, "func Test") {
				t.Error("workflow_test.go missing test functions")
			}

		case "openapi.yaml":
			// Should contain OpenAPI version
			if !strings.Contains(content, "openapi:") {
				t.Error("openapi.yaml missing openapi version")
			}
			// Should contain model name in title
			if !strings.Contains(content, "order-processing") {
				t.Error("openapi.yaml missing model name")
			}
			// Should contain paths
			if !strings.Contains(content, "paths:") {
				t.Error("openapi.yaml missing paths section")
			}
			// Should contain transition endpoints
			if !strings.Contains(content, "transitions") {
				t.Error("openapi.yaml missing transitions tag")
			}

		case "config.go":
			// Should contain Config struct
			if !strings.Contains(content, "type Config struct") {
				t.Error("config.go missing Config struct")
			}
			// Should contain LoadConfig function
			if !strings.Contains(content, "func LoadConfig()") {
				t.Error("config.go missing LoadConfig function")
			}
			// Should contain Validate method
			if !strings.Contains(content, "func (c *Config) Validate()") {
				t.Error("config.go missing Validate method")
			}
		}
	}
}

func TestGenerateToDisk(t *testing.T) {
	model := loadTestModel(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "codegen-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate files
	paths, err := GenerateToDir(model, tmpDir, true)
	if err != nil {
		t.Fatalf("GenerateToDir failed: %v", err)
	}

	if len(paths) == 0 {
		t.Error("expected generated files")
	}

	// Verify all files exist
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("generated file does not exist: %s", path)
		}
	}
}

func TestGeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compilation test in short mode")
	}

	model := loadTestModel(t)

	// Get project root
	projectRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Create temp directory within the project (so it's part of the main module)
	tmpDir, err := os.MkdirTemp(filepath.Join(projectRoot, "generated"), "compiletest")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine the correct module path for the generated code
	tmpDirName := filepath.Base(tmpDir)
	modulePath := "github.com/pflow-xyz/petri-pilot/generated/" + tmpDirName

	// Generate files with tests (as submodule - no go.mod)
	gen, err := New(Options{
		OutputDir:    tmpDir,
		IncludeTests: true,
		AsSubmodule:  true,
		ModulePath:   modulePath,
		PackageName:  tmpDirName,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	_, err = gen.Generate(model)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Compile from project root (generated code is part of main module)
	buildCmd := exec.Command("go", "build", "./"+tmpDirName+"/...")
	buildCmd.Dir = filepath.Join(projectRoot, "generated")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Generated code failed to compile: %v\n%s", err, output)
	}
}

func TestGeneratedTestsPass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test execution in short mode")
	}

	model := loadTestModel(t)

	// Get project root
	projectRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	// Create temp directory within the project (so it's part of the main module)
	tmpDir, err := os.MkdirTemp(filepath.Join(projectRoot, "generated"), "testrun")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine the correct module path for the generated code
	tmpDirName := filepath.Base(tmpDir)
	modulePath := "github.com/pflow-xyz/petri-pilot/generated/" + tmpDirName

	// Generate files with tests (as submodule - no go.mod)
	gen, err := New(Options{
		OutputDir:    tmpDir,
		IncludeTests: true,
		AsSubmodule:  true,
		ModulePath:   modulePath,
		PackageName:  tmpDirName,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	_, err = gen.Generate(model)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Run generated tests from project root (generated code is part of main module)
	testCmd := exec.Command("go", "test", "-v", "./"+tmpDirName+"/...")
	testCmd.Dir = filepath.Join(projectRoot, "generated")
	if output, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("Generated tests failed: %v\n%s", err, output)
	}
}

func TestValidateModel(t *testing.T) {
	// Valid model
	model := loadTestModel(t)
	issues := ValidateModel(model)
	if len(issues) > 0 {
		t.Errorf("expected no issues for valid model, got: %v", issues)
	}

	// Invalid model - no places
	invalidModel := &metamodel.Model{
		Name: "invalid",
		Transitions: []metamodel.Transition{
			{ID: "t1"},
		},
	}
	issues = ValidateModel(invalidModel)
	if len(issues) == 0 {
		t.Error("expected issues for invalid model")
	}
}

func TestPreview(t *testing.T) {
	model := loadTestModel(t)

	gen, err := New(Options{})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	content, err := gen.Preview(model, TemplateWorkflow)
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if !strings.Contains(string(content), "PlaceReceived") {
		t.Error("preview missing expected content")
	}
}
