package golang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

func loadTestModel(t *testing.T) *schema.Model {
	t.Helper()

	data, err := os.ReadFile("../../../examples/order-processing.json")
	if err != nil {
		t.Fatalf("Failed to read example model: %v", err)
	}

	var model schema.Model
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

	gen, err := New(Options{IncludeTests: true})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	files, err := gen.GenerateFiles(model)
	if err != nil {
		t.Fatalf("GenerateFiles failed: %v", err)
	}

	// Should generate: go.mod, main.go, workflow.go, events.go, aggregate.go, api.go, openapi.yaml, config.go, workflow_test.go
	// Plus auth.go, middleware.go, permissions.go (when access control is present), views.go (when views are present),
	// api_events.go (always generated), navigation.go (when navigation is configured), admin.go (when admin is enabled),
	// debug.go (when debug is enabled), sla.go (when SLA is configured), GraphQL files (when graphql is enabled),
	// README.md (always generated)
	expectedFiles := []string{"go.mod", "main.go", "workflow.go", "events.go", "aggregate.go", "api.go", "openapi.yaml", "config.go", "workflow_test.go", "auth.go", "middleware.go", "permissions.go", "views.go", "api_events.go", "navigation.go", "admin.go", "debug.go", "sla.go", "graph/schema.graphqls", "graph/resolver.go", "graphql.go", "gqlgen.yml", "README.md"}
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

	gen, err := New(Options{IncludeTests: true})
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

		case "main.go":
			// Should contain main function
			if !strings.Contains(content, "func main()") {
				t.Error("main.go missing main function")
			}

		case "go.mod":
			// Should contain module declaration
			if !strings.Contains(content, "module") {
				t.Error("go.mod missing module declaration")
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

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "codegen-compile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate files with tests
	_, err = GenerateToDir(model, tmpDir, true)
	if err != nil {
		t.Fatalf("GenerateToDir failed: %v", err)
	}

	// Update go.mod to use the local petri-pilot module
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	// Get the absolute path to petri-pilot
	petriPilotPath, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("Failed to get petri-pilot path: %v", err)
	}

	// Add replace directive for local development
	newGoMod := string(goModContent) + "\n\nreplace github.com/pflow-xyz/petri-pilot => " + petriPilotPath + "\n"
	if err := os.WriteFile(goModPath, []byte(newGoMod), 0644); err != nil {
		t.Fatalf("Failed to update go.mod: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, output)
	}

	// Try to compile
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = tmpDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Generated code failed to compile: %v\n%s", err, output)
	}
}

func TestGeneratedTestsPass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test execution in short mode")
	}

	model := loadTestModel(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "codegen-test-run-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate files with tests
	_, err = GenerateToDir(model, tmpDir, true)
	if err != nil {
		t.Fatalf("GenerateToDir failed: %v", err)
	}

	// Update go.mod with replace directive
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	petriPilotPath, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("Failed to get petri-pilot path: %v", err)
	}

	newGoMod := string(goModContent) + "\n\nreplace github.com/pflow-xyz/petri-pilot => " + petriPilotPath + "\n"
	if err := os.WriteFile(goModPath, []byte(newGoMod), 0644); err != nil {
		t.Fatalf("Failed to update go.mod: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, output)
	}

	// Run generated tests
	testCmd := exec.Command("go", "test", "-v", "./...")
	testCmd.Dir = tmpDir
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
	invalidModel := &schema.Model{
		Name: "invalid",
		Transitions: []schema.Transition{
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
