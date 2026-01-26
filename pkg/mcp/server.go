// Package mcp provides an MCP server exposing Petri net tools.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pflow-xyz/petri-pilot/examples"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/esmodules"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/delegate"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
	jsonschema "github.com/pflow-xyz/petri-pilot/schema"
)

// NewServer creates a new MCP server with Petri net tools.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"petri-pilot",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	// Register tools
	s.AddTool(validateTool(), handleValidate)
	s.AddTool(analyzeTool(), handleAnalyze)
	s.AddTool(simulateTool(), handleSimulateWithSteps)
	s.AddTool(previewTool(), handlePreview)
	s.AddTool(diffTool(), handleDiff)
	s.AddTool(extendTool(), handleExtend)
	s.AddTool(codegenTool(), handleCodegen)
	s.AddTool(frontendTool(), handleFrontend)
	s.AddTool(visualizeTool(), handleVisualize)
	s.AddTool(applicationTool(), handleApplication)
	s.AddTool(docsTool(), handleDocs)

	// Delegate tools for GitHub Copilot integration
	s.AddTool(delegateAppTool(), handleDelegateApp)
	s.AddTool(delegateStatusTool(), handleDelegateStatus)
	s.AddTool(delegateTasksTool(), handleDelegateTasks)

	// Service management tools for controlling generated services
	for _, st := range ServiceTools() {
		s.AddTool(st.Tool, st.Handler)
	}

	// Register prompts for guided workflows
	s.AddPrompt(
		mcp.NewPrompt("design-workflow",
			mcp.WithPromptDescription("Guide through designing a new workflow from requirements"),
			mcp.WithArgument("description", mcp.ArgumentDescription("What the workflow should do")),
		),
		handleDesignWorkflowPrompt,
	)

	s.AddPrompt(
		mcp.NewPrompt("add-access-control",
			mcp.WithPromptDescription("Guide adding roles and permissions to an existing model"),
			mcp.WithArgument("model", mcp.ArgumentDescription("Optional: The Petri net model JSON to add access control to")),
		),
		handleAddAccessControlPrompt,
	)

	s.AddPrompt(
		mcp.NewPrompt("add-views",
			mcp.WithPromptDescription("Guide designing UI views for a model"),
			mcp.WithArgument("model", mcp.ArgumentDescription("Optional: The Petri net model JSON to add views to")),
		),
		handleAddViewsPrompt,
	)

	// Register resources
	registerResources(s)

	return s
}

// Serve starts the MCP server on stdio.
func Serve() error {
	s := NewServer()
	return server.ServeStdio(s)
}

// --- Resource Definitions ---

func registerResources(s *server.MCPServer) {
	// JSON Schema resource
	s.AddResource(
		mcp.NewResource(
			"petri://schema",
			"Petri Net Model JSON Schema",
			mcp.WithResourceDescription("JSON Schema (Draft 2020-12) for validating Petri net model definitions. Use this to validate models before calling petri_validate."),
			mcp.WithMIMEType("application/schema+json"),
		),
		handleSchemaResource,
	)

	// Example models index
	s.AddResource(
		mcp.NewResource(
			"petri://examples",
			"Example Models Index",
			mcp.WithResourceDescription("List of available example Petri net models. Each example demonstrates different model features."),
			mcp.WithMIMEType("application/json"),
		),
		handleExamplesIndexResource,
	)

	// Individual example resources
	for _, name := range examples.List() {
		exampleName := name // capture for closure
		s.AddResource(
			mcp.NewResource(
				"petri://examples/"+exampleName,
				exampleName+" example",
				mcp.WithResourceDescription("Example Petri net model: "+exampleName),
				mcp.WithMIMEType("application/json"),
			),
			func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return handleExampleResource(ctx, request, exampleName)
			},
		)
	}
}

func handleSchemaResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "petri://schema",
			MIMEType: "application/schema+json",
			Text:     string(jsonschema.SchemaJSON),
		},
	}, nil
}

func handleExamplesIndexResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	exampleList := examples.List()
	index := struct {
		Examples []string `json:"examples"`
		Count    int      `json:"count"`
	}{
		Examples: exampleList,
		Count:    len(exampleList),
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "petri://examples",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func handleExampleResource(ctx context.Context, request mcp.ReadResourceRequest, name string) ([]mcp.ResourceContents, error) {
	content, err := examples.Get(name)
	if err != nil {
		return nil, fmt.Errorf("example not found: %s", name)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "petri://examples/" + name,
			MIMEType: "application/json",
			Text:     string(content),
		},
	}, nil
}

// --- Tool Definitions ---

func validateTool() mcp.Tool {
	return mcp.NewTool("petri_validate",
		mcp.WithDescription("Validate a Petri net model for structural correctness. Checks for empty models, unconnected elements, and invalid arc references."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as a JSON string"),
		),
	)
}

func analyzeTool() mcp.Tool {
	return mcp.NewTool("petri_analyze",
		mcp.WithDescription("Analyze a Petri net model for behavioral properties including reachability, deadlocks, liveness, boundedness, and element importance."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as a JSON string"),
		),
		mcp.WithBoolean("full",
			mcp.Description("Include sensitivity analysis (element importance, symmetry groups)"),
		),
	)
}

func simulateTool() mcp.Tool {
	return mcp.NewTool("petri_simulate",
		mcp.WithDescription("Simulate firing transitions and see state changes. Returns detailed step-by-step state trace. Use this to verify workflow behavior before code generation."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON"),
		),
		mcp.WithString("steps",
			mcp.Description("JSON array of simulation steps with optional bindings: [{\"transition\":\"id\",\"bindings\":{...}}]. For simple cases, you can also use 'transitions' parameter."),
		),
		mcp.WithString("transitions",
			mcp.Description("JSON array of transition IDs to fire in order (simple alternative to 'steps')"),
		),
	)
}

func previewTool() mcp.Tool {
	return mcp.NewTool("petri_preview",
		mcp.WithDescription("Preview a single generated file without full code generation. Use this to check specific files before committing to full generation. Available templates: main, workflow, events, aggregate, api, openapi, test, config, migrations, auth, middleware, permissions, views, navigation, admin, debug"),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON"),
		),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("Template name to preview (e.g., 'api', 'workflow', 'events', 'aggregate', 'main')"),
		),
	)
}

func diffTool() mcp.Tool {
	return mcp.NewTool("petri_diff",
		mcp.WithDescription("Compare two Petri net models and show structural differences. Reports added, removed, and modified places, transitions, arcs, roles, and access rules."),
		mcp.WithString("model_a",
			mcp.Required(),
			mcp.Description("First model as JSON (the 'before' or 'base' model)"),
		),
		mcp.WithString("model_b",
			mcp.Required(),
			mcp.Description("Second model as JSON (the 'after' or 'new' model)"),
		),
	)
}

func extendTool() mcp.Tool {
	return mcp.NewTool("petri_extend",
		mcp.WithDescription("Modify an existing Petri net model by applying operations. Operations: add_place, add_transition, add_arc, add_role, add_access, add_event, add_event_field, add_binding, remove_place, remove_transition, remove_arc, remove_role, remove_access, remove_event, remove_binding. Returns the modified model."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON"),
		),
		mcp.WithString("operations",
			mcp.Required(),
			mcp.Description("JSON array of operations. Each operation has 'op' (operation type) and operation-specific fields. Examples: {\"op\":\"add_place\",\"id\":\"new_state\"}, {\"op\":\"add_transition\",\"id\":\"transfer\",\"event\":\"transferred\",\"guard\":\"balances[from] >= amount\",\"bindings\":[{\"name\":\"from\",\"type\":\"string\",\"keys\":[\"from\"]},{\"name\":\"amount\",\"type\":\"number\",\"value\":true}]}, {\"op\":\"add_arc\",\"from\":\"pending\",\"to\":\"approve\"}, {\"op\":\"add_event\",\"id\":\"transferred\",\"fields\":[{\"name\":\"from\",\"type\":\"string\"},{\"name\":\"amount\",\"type\":\"number\"}]}, {\"op\":\"add_binding\",\"transition\":\"transfer\",\"name\":\"to\",\"type\":\"string\",\"keys\":[\"to\"]}"),
		),
	)
}

func codegenTool() mcp.Tool {
	return mcp.NewTool("petri_codegen",
		mcp.WithDescription("Generate executable code from a validated Petri net model. Produces event-sourced application code with state machine, events, and API handlers."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as a JSON string"),
		),
		mcp.WithString("language",
			mcp.Description("Target language: go, javascript, python (default: go)"),
		),
		mcp.WithString("package",
			mcp.Description("Package/module name for generated code"),
		),
	)
}

func frontendTool() mcp.Tool {
	return mcp.NewTool("petri_frontend",
		mcp.WithDescription("Generate a vanilla JavaScript ES modules frontend application from a Petri net model. Produces a Vite + ES modules project with API client, state display, and transition forms using plain JavaScript."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as a JSON string"),
		),
		mcp.WithString("project",
			mcp.Description("Project name for package.json (default: model name)"),
		),
		mcp.WithString("api_url",
			mcp.Description("Backend API base URL (default: http://localhost:8080)"),
		),
	)
}

func visualizeTool() mcp.Tool {
	return mcp.NewTool("petri_visualize",
		mcp.WithDescription("Generate an SVG visualization of a Petri net model showing places, transitions, and arcs."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as a JSON string"),
		),
	)
}

func docsTool() mcp.Tool {
	return mcp.NewTool("petri_docs",
		mcp.WithDescription("Generate markdown documentation from a Petri net model with mermaid diagrams for visualization. Useful for exploring and understanding models."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as a JSON string"),
		),
		mcp.WithBoolean("include_metadata",
			mcp.Description("Include model metadata in documentation (default: true)"),
		),
	)
}

func applicationTool() mcp.Tool {
	return mcp.NewTool("petri_application",
		mcp.WithDescription("Generate a complete full-stack application from an Application specification. This accepts the high-level Application DSL with entities, roles, pages, and workflows."),
		mcp.WithString("spec",
			mcp.Required(),
			mcp.Description("Complete Application specification as JSON (with entities, roles, pages, workflows)"),
		),
		mcp.WithString("backend",
			mcp.Description("Backend language: go, javascript (default: go)"),
		),
		mcp.WithString("frontend",
			mcp.Description("Frontend framework: esm (ES modules), none (default: esm)"),
		),
		mcp.WithString("database",
			mcp.Description("Database: postgres, sqlite (default: sqlite)"),
		),
	)
}

// --- Tool Handlers ---

func handleValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false // structural validation only
	v := validator.New(opts)
	result, err := v.Validate(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

func handleAnalyze(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	full := request.GetBool("full", false)

	opts := validator.DefaultOptions()
	opts.EnableSensitivity = full
	v := validator.New(opts)
	result, err := v.Validate(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("analysis error: %v", err)), nil
	}

	// Run implementability analysis
	implResult := v.ValidateImplementability(model)

	// Return analysis-focused output
	output := struct {
		Valid             bool                              `json:"valid"`
		Analysis          *goflowmetamodel.AnalysisResult            `json:"analysis,omitempty"`
		Errors            []goflowmetamodel.ValidationError          `json:"errors,omitempty"`
		Warnings          []goflowmetamodel.ValidationError          `json:"warnings,omitempty"`
		Implementability  *validator.ImplementabilityResult `json:"implementability,omitempty"`
	}{
		Valid:            result.Valid,
		Analysis:         result.Analysis,
		Errors:           result.Errors,
		Warnings:         result.Warnings,
		Implementability: implResult,
	}

	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(outputJSON)), nil
}

func handlePreview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	templateName, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing file parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	// Create generator
	gen, err := golang.New(golang.Options{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create generator: %v", err)), nil
	}

	// Preview the requested file
	content, err := gen.Preview(model, templateName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to preview %s: %v", templateName, err)), nil
	}

	// Get the output filename
	outputName := gen.GetTemplates().OutputFileName(templateName)

	// Return result with filename and content
	result := struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}{
		File:    outputName,
		Content: string(content),
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(outputJSON)), nil
}

func handleDiff(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelAJSON, err := request.RequireString("model_a")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model_a parameter: %v", err)), nil
	}

	modelBJSON, err := request.RequireString("model_b")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model_b parameter: %v", err)), nil
	}

	modelA, err := parseModel(modelAJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model_a JSON: %v", err)), nil
	}

	modelB, err := parseModel(modelBJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model_b JSON: %v", err)), nil
	}

	// Compare models
	diff := compareModels(modelA, modelB)

	outputJSON, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(outputJSON)), nil
}

// ModelDiff represents the differences between two models.
type ModelDiff struct {
	PlacesAdded        []string `json:"places_added,omitempty"`
	PlacesRemoved      []string `json:"places_removed,omitempty"`
	TransitionsAdded   []string `json:"transitions_added,omitempty"`
	TransitionsRemoved []string `json:"transitions_removed,omitempty"`
	ArcsAdded          []string `json:"arcs_added,omitempty"`
	ArcsRemoved        []string `json:"arcs_removed,omitempty"`
	RolesAdded         []string `json:"roles_added,omitempty"`
	RolesRemoved       []string `json:"roles_removed,omitempty"`
	AccessAdded        []string `json:"access_added,omitempty"`
	AccessRemoved      []string `json:"access_removed,omitempty"`
	HasChanges         bool     `json:"has_changes"`
}

func compareModels(a, b *goflowmetamodel.Model) ModelDiff {
	diff := ModelDiff{}

	// Compare places
	placesA := make(map[string]bool)
	for _, p := range a.Places {
		placesA[p.ID] = true
	}
	placesB := make(map[string]bool)
	for _, p := range b.Places {
		placesB[p.ID] = true
	}
	for id := range placesB {
		if !placesA[id] {
			diff.PlacesAdded = append(diff.PlacesAdded, id)
		}
	}
	for id := range placesA {
		if !placesB[id] {
			diff.PlacesRemoved = append(diff.PlacesRemoved, id)
		}
	}

	// Compare transitions
	transA := make(map[string]bool)
	for _, t := range a.Transitions {
		transA[t.ID] = true
	}
	transB := make(map[string]bool)
	for _, t := range b.Transitions {
		transB[t.ID] = true
	}
	for id := range transB {
		if !transA[id] {
			diff.TransitionsAdded = append(diff.TransitionsAdded, id)
		}
	}
	for id := range transA {
		if !transB[id] {
			diff.TransitionsRemoved = append(diff.TransitionsRemoved, id)
		}
	}

	// Compare arcs
	arcKey := func(arc goflowmetamodel.Arc) string {
		return fmt.Sprintf("%s->%s", arc.From, arc.To)
	}
	arcsA := make(map[string]bool)
	for _, arc := range a.Arcs {
		arcsA[arcKey(arc)] = true
	}
	arcsB := make(map[string]bool)
	for _, arc := range b.Arcs {
		arcsB[arcKey(arc)] = true
	}
	for key := range arcsB {
		if !arcsA[key] {
			diff.ArcsAdded = append(diff.ArcsAdded, key)
		}
	}
	for key := range arcsA {
		if !arcsB[key] {
			diff.ArcsRemoved = append(diff.ArcsRemoved, key)
		}
	}

	// Compare roles
	rolesA := make(map[string]bool)
	for _, r := range a.Roles {
		rolesA[r.ID] = true
	}
	rolesB := make(map[string]bool)
	for _, r := range b.Roles {
		rolesB[r.ID] = true
	}
	for id := range rolesB {
		if !rolesA[id] {
			diff.RolesAdded = append(diff.RolesAdded, id)
		}
	}
	for id := range rolesA {
		if !rolesB[id] {
			diff.RolesRemoved = append(diff.RolesRemoved, id)
		}
	}

	// Compare access rules
	accessKey := func(acc goflowmetamodel.AccessRule) string {
		return fmt.Sprintf("%s:%v", acc.Transition, acc.Roles)
	}
	accessA := make(map[string]bool)
	for _, acc := range a.Access {
		accessA[accessKey(acc)] = true
	}
	accessB := make(map[string]bool)
	for _, acc := range b.Access {
		accessB[accessKey(acc)] = true
	}
	for key := range accessB {
		if !accessA[key] {
			diff.AccessAdded = append(diff.AccessAdded, key)
		}
	}
	for key := range accessA {
		if !accessB[key] {
			diff.AccessRemoved = append(diff.AccessRemoved, key)
		}
	}

	// Check if there are any changes
	diff.HasChanges = len(diff.PlacesAdded) > 0 || len(diff.PlacesRemoved) > 0 ||
		len(diff.TransitionsAdded) > 0 || len(diff.TransitionsRemoved) > 0 ||
		len(diff.ArcsAdded) > 0 || len(diff.ArcsRemoved) > 0 ||
		len(diff.RolesAdded) > 0 || len(diff.RolesRemoved) > 0 ||
		len(diff.AccessAdded) > 0 || len(diff.AccessRemoved) > 0

	return diff
}

func handleExtend(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	opsJSON, err := request.RequireString("operations")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing operations parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	// Parse operations
	var operations []map[string]any
	if err := json.Unmarshal([]byte(opsJSON), &operations); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid operations JSON: %v", err)), nil
	}

	// Apply operations
	var applied []string
	var errors []string

	for i, op := range operations {
		opType, ok := op["op"].(string)
		if !ok {
			errors = append(errors, fmt.Sprintf("operation %d: missing 'op' field", i))
			continue
		}

		if err := applyOperation(model, opType, op); err != nil {
			errors = append(errors, fmt.Sprintf("operation %d (%s): %v", i, opType, err))
		} else {
			applied = append(applied, opType)
		}
	}

	// Validate result
	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false
	v := validator.New(opts)
	validationResult, _ := v.Validate(model)

	// Return result
	modelOutput, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal model: %v", err)), nil
	}

	result := struct {
		Success    bool     `json:"success"`
		Applied    []string `json:"applied"`
		Errors     []string `json:"errors,omitempty"`
		Valid      bool     `json:"valid"`
		Model      string   `json:"model"`
	}{
		Success:    len(errors) == 0,
		Applied:    applied,
		Errors:     errors,
		Valid:      validationResult.Valid,
		Model:      string(modelOutput),
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(outputJSON)), nil
}

func applyOperation(model *goflowmetamodel.Model, opType string, op map[string]any) error {
	switch opType {
	case "add_place":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for add_place")
		}
		desc, _ := op["description"].(string)
		initial, _ := op["initial"].(float64)
		model.Places = append(model.Places, goflowmetamodel.Place{
			ID:          id,
			Description: desc,
			Initial:     int(initial),
		})

	case "add_transition":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for add_transition")
		}
		desc, _ := op["description"].(string)
		event, _ := op["event"].(string)
		guard, _ := op["guard"].(string)

		// Parse bindings if provided
		var bindings []goflowmetamodel.Binding
		if bindingsRaw, ok := op["bindings"].([]any); ok {
			for _, b := range bindingsRaw {
				if bindingMap, ok := b.(map[string]any); ok {
					binding := parseBinding(bindingMap)
					if binding.Name != "" {
						bindings = append(bindings, binding)
					}
				}
			}
		}

		model.Transitions = append(model.Transitions, goflowmetamodel.Transition{
			ID:          id,
			Description: desc,
			Event:       event,
			Guard:       guard,
			Bindings:    bindings,
		})

	case "add_arc":
		from, _ := op["from"].(string)
		to, _ := op["to"].(string)
		if from == "" || to == "" {
			return fmt.Errorf("missing 'from' or 'to' for add_arc")
		}
		model.Arcs = append(model.Arcs, goflowmetamodel.Arc{
			From: from,
			To:   to,
		})

	case "add_role":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for add_role")
		}
		name, _ := op["name"].(string)
		desc, _ := op["description"].(string)
		model.Roles = append(model.Roles, goflowmetamodel.Role{
			ID:          id,
			Name:        name,
			Description: desc,
		})

	case "add_access":
		transition, _ := op["transition"].(string)
		if transition == "" {
			return fmt.Errorf("missing 'transition' for add_access")
		}
		rolesRaw, _ := op["roles"].([]any)
		var roles []string
		for _, r := range rolesRaw {
			if s, ok := r.(string); ok {
				roles = append(roles, s)
			}
		}
		if len(roles) == 0 {
			return fmt.Errorf("missing 'roles' for add_access")
		}
		model.Access = append(model.Access, goflowmetamodel.AccessRule{
			Transition: transition,
			Roles:      roles,
		})

	case "remove_place":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for remove_place")
		}
		newPlaces := make([]goflowmetamodel.Place, 0, len(model.Places))
		for _, p := range model.Places {
			if p.ID != id {
				newPlaces = append(newPlaces, p)
			}
		}
		model.Places = newPlaces

	case "remove_transition":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for remove_transition")
		}
		newTrans := make([]goflowmetamodel.Transition, 0, len(model.Transitions))
		for _, t := range model.Transitions {
			if t.ID != id {
				newTrans = append(newTrans, t)
			}
		}
		model.Transitions = newTrans

	case "remove_arc":
		from, _ := op["from"].(string)
		to, _ := op["to"].(string)
		if from == "" || to == "" {
			return fmt.Errorf("missing 'from' or 'to' for remove_arc")
		}
		newArcs := make([]goflowmetamodel.Arc, 0, len(model.Arcs))
		for _, a := range model.Arcs {
			if a.From != from || a.To != to {
				newArcs = append(newArcs, a)
			}
		}
		model.Arcs = newArcs

	case "remove_role":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for remove_role")
		}
		newRoles := make([]goflowmetamodel.Role, 0, len(model.Roles))
		for _, r := range model.Roles {
			if r.ID != id {
				newRoles = append(newRoles, r)
			}
		}
		model.Roles = newRoles

	case "remove_access":
		transition, _ := op["transition"].(string)
		if transition == "" {
			return fmt.Errorf("missing 'transition' for remove_access")
		}
		newAccess := make([]goflowmetamodel.AccessRule, 0, len(model.Access))
		for _, a := range model.Access {
			if a.Transition != transition {
				newAccess = append(newAccess, a)
			}
		}
		model.Access = newAccess

	case "add_event":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for add_event")
		}
		name, _ := op["name"].(string)
		desc, _ := op["description"].(string)
		var fields []goflowmetamodel.EventField
		if fieldsRaw, ok := op["fields"].([]any); ok {
			for _, f := range fieldsRaw {
				if fieldMap, ok := f.(map[string]any); ok {
					fieldName, _ := fieldMap["name"].(string)
					fieldType, _ := fieldMap["type"].(string)
					fieldOf, _ := fieldMap["of"].(string)
					fieldRequired, _ := fieldMap["required"].(bool)
					fieldDesc, _ := fieldMap["description"].(string)
					if fieldName != "" && fieldType != "" {
						fields = append(fields, goflowmetamodel.EventField{
							Name:        fieldName,
							Type:        fieldType,
							Of:          fieldOf,
							Required:    fieldRequired,
							Description: fieldDesc,
						})
					}
				}
			}
		}
		model.Events = append(model.Events, goflowmetamodel.Event{
			ID:          id,
			Name:        name,
			Description: desc,
			Fields:      fields,
		})

	case "add_event_field":
		eventID, _ := op["event"].(string)
		if eventID == "" {
			return fmt.Errorf("missing 'event' for add_event_field")
		}
		fieldName, _ := op["name"].(string)
		if fieldName == "" {
			return fmt.Errorf("missing 'name' for add_event_field")
		}
		fieldType, _ := op["type"].(string)
		if fieldType == "" {
			return fmt.Errorf("missing 'type' for add_event_field")
		}
		fieldOf, _ := op["of"].(string)
		fieldRequired, _ := op["required"].(bool)
		fieldDesc, _ := op["description"].(string)

		// Find the event and add the field
		found := false
		for i := range model.Events {
			if model.Events[i].ID == eventID {
				model.Events[i].Fields = append(model.Events[i].Fields, goflowmetamodel.EventField{
					Name:        fieldName,
					Type:        fieldType,
					Of:          fieldOf,
					Required:    fieldRequired,
					Description: fieldDesc,
				})
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("event '%s' not found for add_event_field", eventID)
		}

	case "remove_event":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for remove_event")
		}
		newEvents := make([]goflowmetamodel.Event, 0, len(model.Events))
		for _, e := range model.Events {
			if e.ID != id {
				newEvents = append(newEvents, e)
			}
		}
		model.Events = newEvents

	case "add_binding":
		transitionID, _ := op["transition"].(string)
		if transitionID == "" {
			return fmt.Errorf("missing 'transition' for add_binding")
		}
		bindingName, _ := op["name"].(string)
		if bindingName == "" {
			return fmt.Errorf("missing 'name' for add_binding")
		}
		bindingType, _ := op["type"].(string)
		if bindingType == "" {
			return fmt.Errorf("missing 'type' for add_binding")
		}

		binding := goflowmetamodel.Binding{
			Name:  bindingName,
			Type:  bindingType,
			Place: getOptString(op, "place"),
			Value: getOptBool(op, "value"),
			Keys:  getOptStringArray(op, "keys"),
		}

		// Find the transition and add the binding
		found := false
		for i := range model.Transitions {
			if model.Transitions[i].ID == transitionID {
				model.Transitions[i].Bindings = append(model.Transitions[i].Bindings, binding)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("transition '%s' not found for add_binding", transitionID)
		}

	case "remove_binding":
		transitionID, _ := op["transition"].(string)
		if transitionID == "" {
			return fmt.Errorf("missing 'transition' for remove_binding")
		}
		bindingName, _ := op["name"].(string)
		if bindingName == "" {
			return fmt.Errorf("missing 'name' for remove_binding")
		}

		// Find the transition and remove the binding
		found := false
		for i := range model.Transitions {
			if model.Transitions[i].ID == transitionID {
				newBindings := make([]goflowmetamodel.Binding, 0, len(model.Transitions[i].Bindings))
				for _, b := range model.Transitions[i].Bindings {
					if b.Name != bindingName {
						newBindings = append(newBindings, b)
					}
				}
				model.Transitions[i].Bindings = newBindings
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("transition '%s' not found for remove_binding", transitionID)
		}

	default:
		return fmt.Errorf("unknown operation: %s", opType)
	}

	return nil
}

// parseBinding parses a binding from a map.
func parseBinding(m map[string]any) goflowmetamodel.Binding {
	name, _ := m["name"].(string)
	typ, _ := m["type"].(string)
	place, _ := m["place"].(string)
	value, _ := m["value"].(bool)
	var keys []string
	if keysRaw, ok := m["keys"].([]any); ok {
		for _, k := range keysRaw {
			if key, ok := k.(string); ok {
				keys = append(keys, key)
			}
		}
	}
	return goflowmetamodel.Binding{
		Name:  name,
		Type:  typ,
		Place: place,
		Value: value,
		Keys:  keys,
	}
}

// getOptString extracts an optional string from a map.
func getOptString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// getOptBool extracts an optional bool from a map.
func getOptBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// getOptStringArray extracts an optional string array from a map.
func getOptStringArray(m map[string]any, key string) []string {
	if raw, ok := m[key].([]any); ok {
		result := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func handleCodegen(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	language := request.GetString("language", "go")
	pkgName := request.GetString("package", model.Name)

	// Only Go is supported for now
	if language != "go" && language != "golang" {
		return mcp.NewToolResultError(fmt.Sprintf("unsupported language: %s (only 'go' is currently supported)", language)), nil
	}

	// Validate first
	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false
	v := validator.New(opts)
	result, err := v.Validate(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}
	if !result.Valid {
		errJSON, _ := json.MarshalIndent(result.Errors, "", "  ")
		return mcp.NewToolResultError(fmt.Sprintf("model validation failed, fix errors before generating code:\n%s", errJSON)), nil
	}

	// Check implementability
	implResult := v.ValidateImplementability(model)
	if !implResult.Implementable {
		errJSON, _ := json.MarshalIndent(implResult.Errors, "", "  ")
		return mcp.NewToolResultError(fmt.Sprintf("model not implementable:\n%s", errJSON)), nil
	}

	// Create generator
	gen, err := golang.New(golang.Options{
		PackageName:  pkgName,
		IncludeTests: true,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create generator: %v", err)), nil
	}

	// Generate files in memory
	files, err := gen.GenerateFiles(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("code generation failed: %v", err)), nil
	}

	// Build output showing all generated files
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Generated %d files for package '%s':\n\n", len(files), pkgName))

	for _, file := range files {
		sb.WriteString(fmt.Sprintf("=== %s ===\n", file.Name))
		sb.WriteString(string(file.Content))
		sb.WriteString("\n\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleFrontend(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	projectName := request.GetString("project", "")
	apiURL := request.GetString("api_url", "http://localhost:8080")

	// Validate first
	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false
	v := validator.New(opts)
	result, err := v.Validate(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}
	if !result.Valid {
		errJSON, _ := json.MarshalIndent(result.Errors, "", "  ")
		return mcp.NewToolResultError(fmt.Sprintf("model validation failed, fix errors before generating frontend:\n%s", errJSON)), nil
	}

	// Create React generator
	gen, err := esmodules.New(esmodules.Options{
		ProjectName: projectName,
		APIBaseURL:  apiURL,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create generator: %v", err)), nil
	}

	// Generate files in memory
	files, err := gen.GenerateFiles(model)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("frontend generation failed: %v", err)), nil
	}

	// Build output showing all generated files
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Generated %d files for React frontend:\n\n", len(files)))

	for _, file := range files {
		sb.WriteString(fmt.Sprintf("=== %s ===\n", file.Name))
		sb.WriteString(string(file.Content))
		sb.WriteString("\n\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleVisualize(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	// Generate simple SVG visualization
	svg := generateSVG(model)

	return mcp.NewToolResultText(svg), nil
}

func handleDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	includeMetadata := request.GetBool("include_metadata", true)

	// Generate markdown documentation
	md := generateMarkdownDocs(model, modelJSON, includeMetadata)

	return mcp.NewToolResultText(md), nil
}

// generateMarkdownDocs creates markdown documentation with mermaid diagrams
func generateMarkdownDocs(model *goflowmetamodel.Model, rawJSON string, includeMetadata bool) string {
	var sb strings.Builder

	// Title and description
	name := model.Name
	if name == "" {
		name = "Petri Net Model"
	}
	sb.WriteString(fmt.Sprintf("# %s\n\n", titleCase(strings.ReplaceAll(name, "-", " "))))

	if model.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", model.Description))
	}

	if model.Version != "" {
		sb.WriteString(fmt.Sprintf("**Version:** %s\n\n", model.Version))
	}

	// Summary stats
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Element | Count |\n"))
	sb.WriteString(fmt.Sprintf("|---------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Places | %d |\n", len(model.Places)))
	sb.WriteString(fmt.Sprintf("| Transitions | %d |\n", len(model.Transitions)))
	sb.WriteString(fmt.Sprintf("| Arcs | %d |\n", len(model.Arcs)))
	if len(model.Events) > 0 {
		sb.WriteString(fmt.Sprintf("| Events | %d |\n", len(model.Events)))
	}
	if len(model.Roles) > 0 {
		sb.WriteString(fmt.Sprintf("| Roles | %d |\n", len(model.Roles)))
	}
	if len(model.Access) > 0 {
		sb.WriteString(fmt.Sprintf("| Access Rules | %d |\n", len(model.Access)))
	}
	sb.WriteString("\n")

	// Mermaid state diagram
	sb.WriteString("## State Diagram\n\n")
	sb.WriteString("```mermaid\n")
	sb.WriteString("stateDiagram-v2\n")

	// Add places as states
	for _, p := range model.Places {
		label := p.ID
		if p.Initial > 0 {
			label = fmt.Sprintf("%s [%d]", p.ID, p.Initial)
		}
		sb.WriteString(fmt.Sprintf("    %s: %s\n", sanitizeMermaidID(p.ID), label))
	}

	// Add transitions as notes and connections
	for _, arc := range model.Arcs {
		from := sanitizeMermaidID(arc.From)
		to := sanitizeMermaidID(arc.To)
		sb.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
	}

	sb.WriteString("```\n\n")

	// Flowchart (alternative view showing transitions as nodes)
	sb.WriteString("## Petri Net Flow\n\n")
	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart LR\n")

	// Style definitions
	sb.WriteString("    classDef place fill:#e3f2fd,stroke:#1976d2,stroke-width:2px\n")
	sb.WriteString("    classDef transition fill:#333,stroke:#333,color:#fff\n")
	sb.WriteString("    classDef initial fill:#c8e6c9,stroke:#388e3c,stroke-width:2px\n\n")

	// Add places (circles)
	for _, p := range model.Places {
		id := sanitizeMermaidID(p.ID)
		label := p.ID
		if p.Description != "" {
			label = p.Description
		}
		sb.WriteString(fmt.Sprintf("    %s((%s))\n", id, truncate(label, 20)))
	}

	// Add transitions (rectangles)
	for _, t := range model.Transitions {
		id := sanitizeMermaidID(t.ID)
		label := t.ID
		if t.Description != "" {
			label = t.Description
		}
		sb.WriteString(fmt.Sprintf("    %s[%s]\n", id, truncate(label, 25)))
	}

	sb.WriteString("\n")

	// Add arcs
	for _, arc := range model.Arcs {
		from := sanitizeMermaidID(arc.From)
		to := sanitizeMermaidID(arc.To)
		sb.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
	}

	sb.WriteString("\n")

	// Apply classes
	var placeIDs, transIDs, initialIDs []string
	for _, p := range model.Places {
		id := sanitizeMermaidID(p.ID)
		if p.Initial > 0 {
			initialIDs = append(initialIDs, id)
		} else {
			placeIDs = append(placeIDs, id)
		}
	}
	for _, t := range model.Transitions {
		transIDs = append(transIDs, sanitizeMermaidID(t.ID))
	}

	if len(placeIDs) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s place\n", strings.Join(placeIDs, ",")))
	}
	if len(initialIDs) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s initial\n", strings.Join(initialIDs, ",")))
	}
	if len(transIDs) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s transition\n", strings.Join(transIDs, ",")))
	}

	sb.WriteString("```\n\n")

	// Places table
	sb.WriteString("## Places\n\n")
	sb.WriteString("| ID | Description | Initial Tokens |\n")
	sb.WriteString("|----|-------------|----------------|\n")
	for _, p := range model.Places {
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d |\n", p.ID, desc, p.Initial))
	}
	sb.WriteString("\n")

	// Transitions table
	sb.WriteString("## Transitions\n\n")
	sb.WriteString("| ID | Description | Event | Guard |\n")
	sb.WriteString("|----|-------------|-------|-------|\n")
	for _, t := range model.Transitions {
		desc := t.Description
		if desc == "" {
			desc = "-"
		}
		event := t.Event
		if event == "" {
			event = "-"
		}
		guard := t.Guard
		if guard == "" {
			guard = "-"
		} else {
			guard = fmt.Sprintf("`%s`", guard)
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", t.ID, desc, event, guard))
	}
	sb.WriteString("\n")

	// Arcs table
	sb.WriteString("## Arcs\n\n")
	sb.WriteString("| From | To | Type |\n")
	sb.WriteString("|------|-----|------|\n")
	for _, arc := range model.Arcs {
		arcType := "normal"
		if arc.IsInhibitor() {
			arcType = "inhibitor"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", arc.From, arc.To, arcType))
	}
	sb.WriteString("\n")

	// Events section
	if len(model.Events) > 0 {
		sb.WriteString("## Events\n\n")
		for _, e := range model.Events {
			eventName := e.Name
			if eventName == "" {
				eventName = e.ID
			}
			sb.WriteString(fmt.Sprintf("### %s\n\n", eventName))
			if e.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", e.Description))
			}
			if len(e.Fields) > 0 {
				sb.WriteString("| Field | Type | Required | Description |\n")
				sb.WriteString("|-------|------|----------|-------------|\n")
				for _, f := range e.Fields {
					required := "No"
					if f.Required {
						required = "Yes"
					}
					desc := f.Description
					if desc == "" {
						desc = "-"
					}
					sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", f.Name, f.Type, required, desc))
				}
				sb.WriteString("\n")
			}
		}
	}

	// Roles section
	if len(model.Roles) > 0 {
		sb.WriteString("## Roles\n\n")
		sb.WriteString("| ID | Name | Description | Inherits |\n")
		sb.WriteString("|----|------|-------------|----------|\n")
		for _, r := range model.Roles {
			name := r.Name
			if name == "" {
				name = r.ID
			}
			desc := r.Description
			if desc == "" {
				desc = "-"
			}
			inherits := "-"
			if len(r.Inherits) > 0 {
				inherits = strings.Join(r.Inherits, ", ")
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", r.ID, name, desc, inherits))
		}
		sb.WriteString("\n")
	}

	// Access control section
	if len(model.Access) > 0 {
		sb.WriteString("## Access Control\n\n")
		sb.WriteString("| Transition | Allowed Roles |\n")
		sb.WriteString("|------------|---------------|\n")
		for _, a := range model.Access {
			roles := strings.Join(a.Roles, ", ")
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", a.Transition, roles))
		}
		sb.WriteString("\n")
	}

	// Metadata section
	if includeMetadata {
		// Try to extract metadata from raw JSON
		var rawModel map[string]any
		if err := json.Unmarshal([]byte(rawJSON), &rawModel); err == nil {
			if metadata, ok := rawModel["metadata"].(map[string]any); ok && len(metadata) > 0 {
				sb.WriteString("## Metadata\n\n")

				// Strategic values
				if sv, ok := metadata["strategicValues"].(map[string]any); ok {
					sb.WriteString("### Strategic Values\n\n")
					sb.WriteString("Position values derived from Petri net topology:\n\n")
					sb.WriteString("| Position | Value | Type | Patterns |\n")
					sb.WriteString("|----------|-------|------|----------|\n")
					for pos, data := range sv {
						if d, ok := data.(map[string]any); ok {
							value := d["value"]
							posType := d["type"]
							patterns := d["patterns"]
							sb.WriteString(fmt.Sprintf("| `%s` | %.3f | %v | %v |\n", pos, value, posType, patterns))
						}
					}
					sb.WriteString("\n")
				}

				// Win patterns
				if wp, ok := metadata["winPatterns"].([]any); ok {
					sb.WriteString("### Win Patterns\n\n")
					sb.WriteString("```\n")
					for i, pattern := range wp {
						sb.WriteString(fmt.Sprintf("Pattern %d: %v\n", i+1, pattern))
					}
					sb.WriteString("```\n\n")
				}

				// ODE simulation info
				if ode, ok := metadata["odeSimulation"].(map[string]any); ok {
					sb.WriteString("### ODE Simulation\n\n")
					if desc, ok := ode["description"].(string); ok {
						sb.WriteString(fmt.Sprintf("%s\n\n", desc))
					}
					if solver, ok := ode["solver"].(string); ok {
						sb.WriteString(fmt.Sprintf("**Solver:** %s\n\n", solver))
					}
					if interp, ok := ode["interpretation"].(string); ok {
						sb.WriteString(fmt.Sprintf("**Interpretation:** %s\n\n", interp))
					}
				}
			}
		}
	}

	return sb.String()
}

// sanitizeMermaidID converts an ID to be valid for mermaid diagrams
func sanitizeMermaidID(id string) string {
	// Replace hyphens and dots with underscores
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, " ", "_")
	return id
}

// truncate shortens a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// titleCase converts a string to title case (first letter of each word capitalized)
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// --- Helpers ---

// pflowPlace is for parsing pflow.xyz format where places is an object
type pflowPlace struct {
	Initial  []int  `json:"initial"`
	Capacity []int  `json:"capacity"`
	Offset   int    `json:"offset"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

type pflowTransition struct {
	Role string `json:"role"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

type pflowArc struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Weight []int  `json:"weight"`
}

type pflowModel struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Places      map[string]pflowPlace      `json:"places"`
	Transitions map[string]pflowTransition `json:"transitions"`
	Arcs        []pflowArc                 `json:"arcs"`
}

func parseModel(jsonStr string) (*goflowmetamodel.Model, error) {
	// First try pflow.xyz format (places as object with string keys)
	var pflow pflowModel
	if err := json.Unmarshal([]byte(jsonStr), &pflow); err == nil && len(pflow.Places) > 0 {
		model := &goflowmetamodel.Model{
			Name:        pflow.Name,
			Description: pflow.Description,
		}

		// Convert places
		for id, p := range pflow.Places {
			initial := 0
			if len(p.Initial) > 0 {
				initial = p.Initial[0]
			}
			model.Places = append(model.Places, goflowmetamodel.Place{
				ID:      id,
				Initial: initial,
				X:       p.X,
				Y:       p.Y,
			})
		}

		// Convert transitions
		for id, t := range pflow.Transitions {
			model.Transitions = append(model.Transitions, goflowmetamodel.Transition{
				ID: id,
				X:  t.X,
				Y:  t.Y,
			})
		}

		// Convert arcs
		for _, a := range pflow.Arcs {
			weight := 1
			if len(a.Weight) > 0 {
				weight = a.Weight[0]
			}
			model.Arcs = append(model.Arcs, goflowmetamodel.Arc{
				From:   a.Source,
				To:     a.Target,
				Weight: weight,
			})
		}

		return model, nil
	}

	// Standard go-pflow format (places as array)
	var model goflowmetamodel.Model
	if err := json.Unmarshal([]byte(jsonStr), &model); err != nil {
		return nil, err
	}
	return &model, nil
}

func generateSVG(model *goflowmetamodel.Model) string {
	// Check if model has explicit positions
	hasPositions := false
	maxX, maxY := 0, 0
	for _, p := range model.Places {
		if p.X != 0 || p.Y != 0 {
			hasPositions = true
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	for _, t := range model.Transitions {
		if t.X != 0 || t.Y != 0 {
			hasPositions = true
			if t.X > maxX {
				maxX = t.X
			}
			if t.Y > maxY {
				maxY = t.Y
			}
		}
	}

	// Calculate dimensions
	var width, height int
	if hasPositions {
		width = maxX + 100
		height = maxY + 100
	} else {
		spacing := 120
		width = max(len(model.Places), len(model.Transitions))*spacing + 100
		height = 250
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">
  <defs>
    <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
      <polygon points="0 0, 10 3.5, 0 7" fill="#333"/>
    </marker>
  </defs>
  <style>
    .place { fill: #e3f2fd; stroke: #1976d2; stroke-width: 2; }
    .transition { fill: #333; stroke: #333; }
    .label { font-family: system-ui, sans-serif; font-size: 12px; text-anchor: middle; }
    .arc { stroke: #333; stroke-width: 1.5; fill: none; marker-end: url(#arrowhead); }
  </style>
`, width, height)

	// Default layout values for auto-positioning
	defaultPlaceY := 50
	defaultTransY := 150
	spacing := 120

	// Place positions
	placePos := make(map[string][2]int)
	for i, p := range model.Places {
		var x, y int
		if p.X != 0 || p.Y != 0 {
			// Use explicit position
			x, y = p.X, p.Y
		} else {
			// Auto-layout
			x = 50 + i*spacing
			y = defaultPlaceY
		}
		placePos[p.ID] = [2]int{x, y}
		svg += fmt.Sprintf(`  <circle cx="%d" cy="%d" r="25" class="place"/>
  <text x="%d" y="%d" class="label">%s</text>
`, x, y, x, y+45, p.ID)
		// Show initial tokens
		if p.Initial > 0 {
			svg += fmt.Sprintf(`  <text x="%d" y="%d" class="label" style="font-weight:bold">%d</text>
`, x, y+5, p.Initial)
		}
	}

	// Transition positions
	transPos := make(map[string][2]int)
	for i, t := range model.Transitions {
		var x, y int
		if t.X != 0 || t.Y != 0 {
			// Use explicit position
			x, y = t.X, t.Y
		} else {
			// Auto-layout
			x = 50 + i*spacing
			y = defaultTransY
		}
		transPos[t.ID] = [2]int{x, y}
		svg += fmt.Sprintf(`  <rect x="%d" y="%d" width="10" height="40" class="transition"/>
  <text x="%d" y="%d" class="label">%s</text>
`, x-5, y-20, x, y+40, t.ID)
	}

	// Draw arcs
	for _, arc := range model.Arcs {
		var x1, y1, x2, y2 int
		if pos, ok := placePos[arc.From]; ok {
			x1, y1 = pos[0], pos[1]
			if pos2, ok := transPos[arc.To]; ok {
				x2, y2 = pos2[0], pos2[1]
			}
		} else if pos, ok := transPos[arc.From]; ok {
			x1, y1 = pos[0], pos[1]
			if pos2, ok := placePos[arc.To]; ok {
				x2, y2 = pos2[0], pos2[1]
			}
		}
		if x1 != 0 && x2 != 0 {
			svg += fmt.Sprintf(`  <path d="M%d,%d L%d,%d" class="arc"/>
`, x1, y1, x2, y2)
		}
	}

	svg += "</svg>"
	return svg
}

func handleApplication(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	specJSON, err := request.RequireString("spec")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing spec parameter: %v", err)), nil
	}

	// Parse Application spec
	var app metamodel.Application
	if err := json.Unmarshal([]byte(specJSON), &app); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid application spec JSON: %v", err)), nil
	}

	backend := request.GetString("backend", "go")
	frontend := request.GetString("frontend", "esm")
	database := request.GetString("database", "sqlite")

	// Validate application spec
	if len(app.Entities) == 0 {
		return mcp.NewToolResultError("application spec must contain at least one entity"), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Generating full-stack application '%s':\n", app.Name))
	sb.WriteString(fmt.Sprintf("- Backend: %s\n", backend))
	sb.WriteString(fmt.Sprintf("- Frontend: %s\n", frontend))
	sb.WriteString(fmt.Sprintf("- Database: %s\n", database))
	sb.WriteString(fmt.Sprintf("- Entities: %d\n", len(app.Entities)))
	if len(app.Roles) > 0 {
		sb.WriteString(fmt.Sprintf("- Roles: %d\n", len(app.Roles)))
	}
	if len(app.Pages) > 0 {
		sb.WriteString(fmt.Sprintf("- Pages: %d\n", len(app.Pages)))
	}
	if len(app.Workflows) > 0 {
		sb.WriteString(fmt.Sprintf("- Workflows: %d\n", len(app.Workflows)))
	}
	sb.WriteString("\n")

	// Generate code for each entity
	for i, entity := range app.Entities {
		sb.WriteString(fmt.Sprintf("=== Entity %d: %s ===\n", i+1, entity.ID))
		
		// Convert Entity to metamodel.Schema then to goflowmetamodel.Model
		metaSchema := entity.ToSchema()
		model := metaSchema.ToModel()
		
		sb.WriteString(fmt.Sprintf("- States: %d\n", len(entity.States)))
		sb.WriteString(fmt.Sprintf("- Actions: %d\n", len(entity.Actions)))
		sb.WriteString(fmt.Sprintf("- Fields: %d\n", len(entity.Fields)))
		
		// Display access rules
		if len(entity.Access) > 0 {
			sb.WriteString(fmt.Sprintf("- Access rules: %d\n", len(entity.Access)))
			for _, rule := range entity.Access {
				roles := "any authenticated"
				if len(rule.Roles) > 0 {
					roles = strings.Join(rule.Roles, ", ")
				}
				sb.WriteString(fmt.Sprintf("  * %s: %s", rule.Action, roles))
				if rule.Guard != "" {
					sb.WriteString(fmt.Sprintf(" (guard: %s)", rule.Guard))
				}
				sb.WriteString("\n")
			}
		}
		
		// Generate backend code if requested
		if backend == "go" {
			sb.WriteString("\n--- Backend Code ---\n")
			
			// Build access rule contexts
			var accessRules []golang.AccessRuleContext
			for _, rule := range entity.Access {
				accessRules = append(accessRules, golang.AccessRuleContext{
					TransitionID: rule.Action,
					Roles:        rule.Roles,
					Guard:        rule.Guard,
				})
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
			
			// Build webhook contexts
			var webhooks []golang.WebhookContext
			for _, wh := range app.Webhooks {
				var retryPolicy *golang.WebhookRetryPolicyContext
				if wh.RetryPolicy != nil {
					retryPolicy = &golang.WebhookRetryPolicyContext{
						MaxAttempts: wh.RetryPolicy.MaxAttempts,
						BackoffMs:   wh.RetryPolicy.BackoffMs,
					}
				}
				webhooks = append(webhooks, golang.WebhookContext{
					ID:          wh.ID,
					URL:         wh.URL,
					Events:      wh.Events,
					Secret:      wh.Secret,
					Enabled:     wh.Enabled,
					RetryPolicy: retryPolicy,
				})
			}
			
			// Build workflow contexts (Phase 12)
			var workflows []golang.WorkflowContext
			for _, wf := range app.Workflows {
				// Only include workflows that trigger on this entity
				if wf.Trigger.Entity == entity.ID || wf.Trigger.Entity == "" {
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
							Duration:   step.Duration,
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
			}
			
			gen, err := golang.New(golang.Options{
				PackageName:          entity.ID,
				IncludeTests:         true,
				IncludeInfra:         true,
				IncludeAuth:          len(entity.Access) > 0 || len(app.Roles) > 0,
				IncludeObservability: false,
				IncludeDeploy:        false,
				IncludeRealtime:      false,
			})
			if err != nil {
				sb.WriteString(fmt.Sprintf("Error creating backend generator: %v\n", err))
				continue
			}
			
			// Generate files with access control, workflows, and webhooks context
			files, err := generateBackendWithAccessControl(gen, model, accessRules, roles, workflows, webhooks)
			if err != nil {
				sb.WriteString(fmt.Sprintf("Error generating backend: %v\n", err))
				continue
			}
			
			sb.WriteString(fmt.Sprintf("Generated %d backend files\n", len(files)))
			for _, file := range files {
				sb.WriteString(fmt.Sprintf("  - %s\n", file.Name))
			}
			if len(workflows) > 0 {
				sb.WriteString(fmt.Sprintf("  - Workflows: %d\n", len(workflows)))
			}
		}
		
		// Generate frontend code if requested
		if frontend == "esm" {
			sb.WriteString("\n--- Frontend Code ---\n")
			
			// Build page contexts from application pages
			var pageContexts []esmodules.PageContext
			for _, page := range app.Pages {
				// Only include pages for this entity
				if page.Layout.Entity == entity.ID || page.Layout.Entity == "" {
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
			}
			
			gen, err := esmodules.New(esmodules.Options{
				ProjectName: app.Name + "-" + entity.ID,
				APIBaseURL:  "http://localhost:8080",
			})
			if err != nil {
				sb.WriteString(fmt.Sprintf("Error creating frontend generator: %v\n", err))
				continue
			}
			
			// Generate files with page contexts
			files, err := generateFrontendWithPages(gen, model, pageContexts)
			if err != nil {
				sb.WriteString(fmt.Sprintf("Error generating frontend: %v\n", err))
				continue
			}
			
			sb.WriteString(fmt.Sprintf("Generated %d frontend files\n", len(files)))
			for _, file := range files {
				sb.WriteString(fmt.Sprintf("  - %s\n", file.Name))
			}
		}
		
		sb.WriteString("\n")
	}

	sb.WriteString("✅ Application generation complete!\n")
	sb.WriteString("\nGenerated components:\n")
	sb.WriteString("- Event-sourced aggregates\n")
	sb.WriteString("- State machines with guards\n")
	sb.WriteString("- HTTP API handlers\n")
	if len(app.Roles) > 0 {
		sb.WriteString("- Role-based access control middleware\n")
	}
	if len(app.Pages) > 0 {
		sb.WriteString("- Frontend routing and navigation\n")
		sb.WriteString("- Page components (list, detail, form)\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// Helper to generate backend with access control, workflows, and webhooks
func generateBackendWithAccessControl(gen *golang.Generator, model *goflowmetamodel.Model, accessRules []golang.AccessRuleContext, roles []golang.RoleContext, workflows []golang.WorkflowContext, webhooks []golang.WebhookContext) ([]golang.GeneratedFile, error) {
	// Build context with access rules, workflows, and webhooks
	ctx, err := golang.NewContext(model, golang.ContextOptions{
		PackageName: model.Name,
		AccessRules: accessRules,
		Roles:       roles,
		Workflows:   workflows,
		Webhooks:    webhooks,
	})
	if err != nil {
		return nil, err
	}
	
	// Generate files manually to inject access control context
	var files []golang.GeneratedFile
	
	// Get template names
	templates := gen.GetTemplates()
	templateNames := []string{
		golang.TemplateGoMod,
		golang.TemplateMain,
		golang.TemplateWorkflow,
		golang.TemplateEvents,
		golang.TemplateAggregate,
		golang.TemplateAPI,
		golang.TemplateOpenAPI,
		golang.TemplateTest,
		golang.TemplateMigrations,
	}
	
	// Add auth templates if we have access rules or roles
	if len(accessRules) > 0 || len(roles) > 0 {
		templateNames = append(templateNames, golang.TemplateAuth, golang.TemplateMiddleware)
	}
	
	// Add workflows template if we have workflows (Phase 12)
	if len(workflows) > 0 {
		templateNames = append(templateNames, golang.TemplateWorkflows)
	}
	
	for _, name := range templateNames {
		content, err := templates.Execute(name, ctx)
		if err != nil {
			return nil, fmt.Errorf("executing template %s: %w", name, err)
		}
		
		files = append(files, golang.GeneratedFile{
			Name:    templates.OutputFileName(name),
			Content: content,
		})
	}
	
	return files, nil
}

// Helper to generate frontend with pages
func generateFrontendWithPages(gen *esmodules.Generator, model *goflowmetamodel.Model, pages []esmodules.PageContext) ([]esmodules.GeneratedFile, error) {
	// Build context with pages
	ctx, err := esmodules.NewContext(model, esmodules.ContextOptions{
		ProjectName: model.Name,
		Pages:       pages,
	})
	if err != nil {
		return nil, err
	}
	
	// Generate files manually to inject page context
	var files []esmodules.GeneratedFile
	
	templates := gen.GetTemplates()
	for _, name := range esmodules.AllTemplateNames() {
		content, err := templates.Execute(name, ctx)
		if err != nil {
			return nil, fmt.Errorf("executing template %s: %w", name, err)
		}
		
		files = append(files, esmodules.GeneratedFile{
			Name:    templates.OutputFileName(name),
			Content: content,
		})
	}
	
	return files, nil
}

// Helper function for PascalCase conversion
func toPascalCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, "")
}

// Helper function for camelCase conversion
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// Helper to split words by various delimiters
func splitWords(s string) []string {
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return strings.Split(s, "_")
}

// --- Delegate Tool Definitions ---

func delegateAppTool() mcp.Tool {
	return mcp.NewTool("delegate_app",
		mcp.WithDescription("Request GitHub Copilot to generate a new application. Creates an issue and assigns it to Copilot coding agent for autonomous implementation."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Short name for the app (lowercase, hyphens allowed)"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Natural language description of what the app should do"),
		),
		mcp.WithString("features",
			mcp.Description("Comma-separated features: auth,rbac,admin,events,e2e"),
		),
		mcp.WithString("complexity",
			mcp.Description("Complexity level: simple, medium, complex (default: medium)"),
		),
		mcp.WithString("owner",
			mcp.Description("GitHub repository owner (default: pflow-xyz)"),
		),
		mcp.WithString("repo",
			mcp.Description("GitHub repository name (default: petri-pilot)"),
		),
	)
}

func delegateStatusTool() mcp.Tool {
	return mcp.NewTool("delegate_status",
		mcp.WithDescription("Check the status of GitHub Copilot coding agents. Shows active runs, open PRs, and recent app requests."),
		mcp.WithString("owner",
			mcp.Description("GitHub repository owner (default: pflow-xyz)"),
		),
		mcp.WithString("repo",
			mcp.Description("GitHub repository name (default: petri-pilot)"),
		),
	)
}

func delegateTasksTool() mcp.Tool {
	return mcp.NewTool("delegate_tasks",
		mcp.WithDescription("Delegate multiple tasks to GitHub Copilot in parallel. Each task becomes an issue assigned to Copilot."),
		mcp.WithString("tasks",
			mcp.Required(),
			mcp.Description("JSON array of tasks, each with 'title' and 'description' fields"),
		),
		mcp.WithString("owner",
			mcp.Description("GitHub repository owner (default: pflow-xyz)"),
		),
		mcp.WithString("repo",
			mcp.Description("GitHub repository name (default: petri-pilot)"),
		),
	)
}

// --- Delegate Tool Handlers ---

func handleDelegateApp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	description, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError("description is required"), nil
	}

	owner := request.GetString("owner", "pflow-xyz")
	repo := request.GetString("repo", "petri-pilot")
	features := request.GetString("features", "")
	complexity := request.GetString("complexity", "medium")

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return mcp.NewToolResultError("GITHUB_TOKEN environment variable is required"), nil
	}

	client := delegate.NewClient(owner, repo, token)

	var featureList []string
	if features != "" {
		for _, f := range strings.Split(features, ",") {
			featureList = append(featureList, strings.TrimSpace(f))
		}
	}

	req := delegate.AppRequest{
		Name:        name,
		Description: description,
		Features:    featureList,
		Complexity:  complexity,
	}

	issue, err := client.CreateAppRequest(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create app request: %v", err)), nil
	}

	result := fmt.Sprintf(`App request created successfully!

Issue: #%d
URL: %s

Copilot will work on this autonomously and create a PR when ready.

To check status: delegate_status
`, issue.Number, issue.HTMLURL)

	return mcp.NewToolResultText(result), nil
}

func handleDelegateStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	owner := request.GetString("owner", "pflow-xyz")
	repo := request.GetString("repo", "petri-pilot")

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return mcp.NewToolResultError("GITHUB_TOKEN environment variable is required"), nil
	}

	client := delegate.NewClient(owner, repo, token)

	status, err := client.GetAgentStatus(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get status: %v", err)), nil
	}

	var sb strings.Builder

	if len(status.ActiveRuns) > 0 {
		sb.WriteString(fmt.Sprintf("## Active Copilot Agents: %d\n\n", len(status.ActiveRuns)))
		for _, run := range status.ActiveRuns {
			duration := time.Since(run.CreatedAt).Round(time.Second)
			sb.WriteString(fmt.Sprintf("- **%s** (running %s)\n", run.HeadBranch, duration))
			sb.WriteString(fmt.Sprintf("  %s\n", run.HTMLURL))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("## No active Copilot agents\n\n")
	}

	if len(status.OpenPRs) > 0 {
		sb.WriteString(fmt.Sprintf("## Open PRs from Copilot: %d\n\n", len(status.OpenPRs)))
		for _, pr := range status.OpenPRs {
			draft := ""
			if pr.Draft {
				draft = " [DRAFT]"
			}
			sb.WriteString(fmt.Sprintf("- **#%d**: %s%s\n", pr.Number, pr.Title, draft))
			sb.WriteString(fmt.Sprintf("  +%d -%d | %s\n", pr.Additions, pr.Deletions, pr.HTMLURL))
		}
		sb.WriteString("\n")
	}

	if len(status.RecentIssues) > 0 {
		sb.WriteString(fmt.Sprintf("## Recent App Requests: %d\n\n", len(status.RecentIssues)))
		for _, issue := range status.RecentIssues {
			sb.WriteString(fmt.Sprintf("- **#%d**: %s [%s]\n", issue.Number, issue.Title, issue.State))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleDelegateTasks(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasksJSON, err := request.RequireString("tasks")
	if err != nil {
		return mcp.NewToolResultError("tasks is required (JSON array)"), nil
	}

	owner := request.GetString("owner", "pflow-xyz")
	repo := request.GetString("repo", "petri-pilot")

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return mcp.NewToolResultError("GITHUB_TOKEN environment variable is required"), nil
	}

	// Parse tasks JSON
	var tasks []delegate.Task
	if err := json.Unmarshal([]byte(tasksJSON), &tasks); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid tasks JSON: %v", err)), nil
	}

	if len(tasks) == 0 {
		return mcp.NewToolResultError("at least one task is required"), nil
	}

	client := delegate.NewClient(owner, repo, token)

	result, err := client.DelegateTasks(ctx, tasks)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delegate tasks: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Delegated %d tasks\n\n", len(result.Succeeded)))

	for _, issue := range result.Succeeded {
		sb.WriteString(fmt.Sprintf("- **#%d**: %s\n", issue.Number, issue.Title))
	}

	if len(result.Failed) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Failed: %d tasks\n\n", len(result.Failed)))
		for _, f := range result.Failed {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Task.Title, f.Error))
		}
	}

	sb.WriteString("\nCopilot agents will work on these autonomously.")

	return mcp.NewToolResultText(sb.String()), nil
}
