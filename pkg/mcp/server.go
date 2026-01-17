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
	s.AddTool(visualizeTool(), handleVisualize)

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
			mcp.Description("Target language: go, typescript, python (default: go)"),
		),
		mcp.WithString("package",
			mcp.Description("Package/module name for generated code"),
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
