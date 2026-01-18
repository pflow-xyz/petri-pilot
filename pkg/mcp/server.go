// Package mcp provides an MCP server exposing Petri net tools.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/react"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
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
	s.AddTool(codegenTool(), handleCodegen)
	s.AddTool(frontendTool(), handleFrontend)
	s.AddTool(visualizeTool(), handleVisualize)
	s.AddTool(applicationTool(), handleApplication)

	return s
}

// Serve starts the MCP server on stdio.
func Serve() error {
	s := NewServer()
	return server.ServeStdio(s)
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
	gen, err := react.New(react.Options{
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
			
			// Generate files with access control and workflows context
			files, err := generateBackendWithAccessControl(gen, model, accessRules, roles, workflows)
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
			var pageContexts []react.PageContext
			for _, page := range app.Pages {
				// Only include pages for this entity
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

// Helper to generate backend with access control
func generateBackendWithAccessControl(gen *golang.Generator, model *schema.Model, accessRules []golang.AccessRuleContext, roles []golang.RoleContext, workflows []golang.WorkflowContext) ([]golang.GeneratedFile, error) {
	// Build context with access rules and workflows
	ctx, err := golang.NewContext(model, golang.ContextOptions{
		PackageName: model.Name,
		AccessRules: accessRules,
		Roles:       roles,
		Workflows:   workflows,
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
func generateFrontendWithPages(gen *react.Generator, model *schema.Model, pages []react.PageContext) ([]react.GeneratedFile, error) {
	// Build context with pages
	ctx, err := react.NewContext(model, react.ContextOptions{
		ProjectName: model.Name,
		Pages:       pages,
	})
	if err != nil {
		return nil, err
	}
	
	// Generate files manually to inject page context
	var files []react.GeneratedFile
	
	templates := gen.GetTemplates()
	for _, name := range react.AllTemplateNames() {
		content, err := templates.Execute(name, ctx)
		if err != nil {
			return nil, fmt.Errorf("executing template %s: %w", name, err)
		}
		
		files = append(files, react.GeneratedFile{
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
