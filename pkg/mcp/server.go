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
	pflowMetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/examples"
	"github.com/pflow-xyz/petri-pilot/pkg/bridge"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/esmodules"
	"github.com/pflow-xyz/petri-pilot/pkg/delegate"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
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
	s.AddTool(simulateTool(), handleSimulate)
	s.AddTool(previewTool(), handlePreview)
	s.AddTool(diffTool(), handleDiff)
	s.AddTool(extendTool(), handleExtend)
	s.AddTool(codegenTool(), handleCodegen)
	s.AddTool(frontendTool(), handleFrontend)
	s.AddTool(visualizeTool(), handleVisualize)
	s.AddTool(applicationTool(), handleApplication)

	// Delegate tools for GitHub Copilot integration
	s.AddTool(delegateAppTool(), handleDelegateApp)
	s.AddTool(delegateStatusTool(), handleDelegateStatus)
	s.AddTool(delegateTasksTool(), handleDelegateTasks)

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
		mcp.WithDescription("Simulate firing transitions and return the resulting state. Use this to verify workflow behavior before code generation."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON"),
		),
		mcp.WithString("transitions",
			mcp.Required(),
			mcp.Description("JSON array of transition IDs to fire in order"),
		),
	)
}

func previewTool() mcp.Tool {
	return mcp.NewTool("petri_preview",
		mcp.WithDescription("Preview a single generated file without full code generation. Use this to check specific files before committing to full generation. Available templates: main, workflow, events, aggregate, api, openapi, test, config, migrations, dockerfile, docker-compose, auth, middleware, permissions, views, navigation, admin, debug"),
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
		mcp.WithDescription("Modify an existing Petri net model by applying operations. Operations: add_place, add_transition, add_arc, add_role, add_access, remove_place, remove_transition, remove_arc, remove_role, remove_access. Returns the modified model."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON"),
		),
		mcp.WithString("operations",
			mcp.Required(),
			mcp.Description("JSON array of operations. Each operation has 'op' (operation type) and operation-specific fields. Examples: {\"op\":\"add_place\",\"id\":\"new_state\"}, {\"op\":\"add_transition\",\"id\":\"approve\",\"description\":\"Approve request\"}, {\"op\":\"add_arc\",\"from\":\"pending\",\"to\":\"approve\"}, {\"op\":\"add_role\",\"id\":\"admin\",\"name\":\"Administrator\"}, {\"op\":\"add_access\",\"transition\":\"approve\",\"roles\":[\"admin\"]}"),
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
		Analysis          *schema.AnalysisResult            `json:"analysis,omitempty"`
		Errors            []schema.ValidationError          `json:"errors,omitempty"`
		Warnings          []schema.ValidationError          `json:"warnings,omitempty"`
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

func handleSimulate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	transitionsJSON, err := request.RequireString("transitions")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing transitions parameter: %v", err)), nil
	}

	// Parse model
	model, err := parseModel(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}

	// Parse transition sequence
	var transitions []string
	if err := json.Unmarshal([]byte(transitionsJSON), &transitions); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid transitions JSON: %v", err)), nil
	}

	// Convert to go-pflow metamodel
	metaSchema := bridge.ToMetamodel(model)

	// Create runtime
	runtime := pflowMetamodel.NewRuntime(metaSchema)

	// Capture initial marking
	initialMarking := make(map[string]int)
	for _, state := range metaSchema.States {
		if state.IsToken() {
			initialMarking[state.ID] = runtime.Tokens(state.ID)
		}
	}

	// Simulate firing transitions
	var fired []string
	var failed []struct {
		TransitionID string `json:"transition_id"`
		Reason       string `json:"reason"`
	}

	for _, transitionID := range transitions {
		// Check if action exists
		action := metaSchema.ActionByID(transitionID)
		if action == nil {
			failed = append(failed, struct {
				TransitionID string `json:"transition_id"`
				Reason       string `json:"reason"`
			}{
				TransitionID: transitionID,
				Reason:       "transition not found in model",
			})
			continue
		}

		// Check if enabled
		if !runtime.Enabled(transitionID) {
			// Find out why it's not enabled
			reason := "insufficient tokens in input places"
			inputArcs := metaSchema.InputArcs(transitionID)
			if len(inputArcs) == 0 {
				reason = "transition has no input arcs"
			} else {
				var missingTokens []string
				for _, arc := range inputArcs {
					st := metaSchema.StateByID(arc.Source)
					if st != nil && st.IsToken() {
						if runtime.Tokens(arc.Source) < 1 {
							missingTokens = append(missingTokens, arc.Source)
						}
					}
				}
				if len(missingTokens) > 0 {
					reason = fmt.Sprintf("insufficient tokens in: %s", strings.Join(missingTokens, ", "))
				}
			}

			failed = append(failed, struct {
				TransitionID string `json:"transition_id"`
				Reason       string `json:"reason"`
			}{
				TransitionID: transitionID,
				Reason:       reason,
			})
			continue
		}

		// Execute the transition
		if err := runtime.Execute(transitionID); err != nil {
			failed = append(failed, struct {
				TransitionID string `json:"transition_id"`
				Reason       string `json:"reason"`
			}{
				TransitionID: transitionID,
				Reason:       fmt.Sprintf("execution error: %v", err),
			})
			continue
		}

		fired = append(fired, transitionID)
	}

	// Capture final marking
	finalMarking := make(map[string]int)
	for _, state := range metaSchema.States {
		if state.IsToken() {
			finalMarking[state.ID] = runtime.Tokens(state.ID)
		}
	}

	// Check for deadlock (no enabled transitions)
	enabledTransitions := runtime.EnabledActions()
	isDeadlock := len(enabledTransitions) == 0

	// Build result
	result := struct {
		Success        bool              `json:"success"`
		InitialMarking map[string]int    `json:"initial_marking"`
		FinalMarking   map[string]int    `json:"final_marking"`
		Fired          []string          `json:"fired"`
		Failed         []struct {
			TransitionID string `json:"transition_id"`
			Reason       string `json:"reason"`
		} `json:"failed"`
		IsDeadlock bool     `json:"is_deadlock"`
		Enabled    []string `json:"enabled,omitempty"`
	}{
		Success:        len(failed) == 0,
		InitialMarking: initialMarking,
		FinalMarking:   finalMarking,
		Fired:          fired,
		Failed:         failed,
		IsDeadlock:     isDeadlock,
		Enabled:        enabledTransitions,
	}

	outputJSON, err := json.MarshalIndent(result, "", "  ")
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

func compareModels(a, b *schema.Model) ModelDiff {
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
	arcKey := func(arc schema.Arc) string {
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
	accessKey := func(acc schema.AccessRule) string {
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

func applyOperation(model *schema.Model, opType string, op map[string]any) error {
	switch opType {
	case "add_place":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for add_place")
		}
		desc, _ := op["description"].(string)
		initial, _ := op["initial"].(float64)
		model.Places = append(model.Places, schema.Place{
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
		model.Transitions = append(model.Transitions, schema.Transition{
			ID:          id,
			Description: desc,
		})

	case "add_arc":
		from, _ := op["from"].(string)
		to, _ := op["to"].(string)
		if from == "" || to == "" {
			return fmt.Errorf("missing 'from' or 'to' for add_arc")
		}
		model.Arcs = append(model.Arcs, schema.Arc{
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
		model.Roles = append(model.Roles, schema.Role{
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
		model.Access = append(model.Access, schema.AccessRule{
			Transition: transition,
			Roles:      roles,
		})

	case "remove_place":
		id, _ := op["id"].(string)
		if id == "" {
			return fmt.Errorf("missing 'id' for remove_place")
		}
		newPlaces := make([]schema.Place, 0, len(model.Places))
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
		newTrans := make([]schema.Transition, 0, len(model.Transitions))
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
		newArcs := make([]schema.Arc, 0, len(model.Arcs))
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
		newRoles := make([]schema.Role, 0, len(model.Roles))
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
		newAccess := make([]schema.AccessRule, 0, len(model.Access))
		for _, a := range model.Access {
			if a.Transition != transition {
				newAccess = append(newAccess, a)
			}
		}
		model.Access = newAccess

	default:
		return fmt.Errorf("unknown operation: %s", opType)
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

// --- Helpers ---

func parseModel(jsonStr string) (*schema.Model, error) {
	var model schema.Model
	if err := json.Unmarshal([]byte(jsonStr), &model); err != nil {
		return nil, err
	}
	return &model, nil
}

func generateSVG(model *schema.Model) string {
	// Calculate layout
	placeY := 50
	transY := 150
	spacing := 120
	width := max(len(model.Places), len(model.Transitions))*spacing + 100
	height := 250

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

	// Place positions
	placePos := make(map[string][2]int)
	for i, p := range model.Places {
		x := 50 + i*spacing
		placePos[p.ID] = [2]int{x, placeY}
		svg += fmt.Sprintf(`  <circle cx="%d" cy="%d" r="25" class="place"/>
  <text x="%d" y="%d" class="label">%s</text>
`, x, placeY, x, placeY+45, p.ID)
		// Show initial tokens
		if p.Initial > 0 {
			svg += fmt.Sprintf(`  <text x="%d" y="%d" class="label" style="font-weight:bold">%d</text>
`, x, placeY+5, p.Initial)
		}
	}

	// Transition positions
	transPos := make(map[string][2]int)
	for i, t := range model.Transitions {
		x := 50 + i*spacing
		transPos[t.ID] = [2]int{x, transY}
		svg += fmt.Sprintf(`  <rect x="%d" y="%d" width="10" height="40" class="transition"/>
  <text x="%d" y="%d" class="label">%s</text>
`, x-5, transY-20, x, transY+40, t.ID)
	}

	// Draw arcs
	for _, arc := range model.Arcs {
		var x1, y1, x2, y2 int
		if pos, ok := placePos[arc.From]; ok {
			x1, y1 = pos[0], pos[1]+25 // bottom of place
			if pos2, ok := transPos[arc.To]; ok {
				x2, y2 = pos2[0], pos2[1]-20 // top of transition
			}
		} else if pos, ok := transPos[arc.From]; ok {
			x1, y1 = pos[0], pos[1]+20 // bottom of transition
			if pos2, ok := placePos[arc.To]; ok {
				x2, y2 = pos2[0], pos2[1]-25 // top of place
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
		
		// Convert Entity to metamodel.Schema then to schema.Model
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
func generateBackendWithAccessControl(gen *golang.Generator, model *schema.Model, accessRules []golang.AccessRuleContext, roles []golang.RoleContext, workflows []golang.WorkflowContext, webhooks []golang.WebhookContext) ([]golang.GeneratedFile, error) {
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
		golang.TemplateDockerfile,
		golang.TemplateDockerCompose,
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
func generateFrontendWithPages(gen *esmodules.Generator, model *schema.Model, pages []esmodules.PageContext) ([]esmodules.GeneratedFile, error) {
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
