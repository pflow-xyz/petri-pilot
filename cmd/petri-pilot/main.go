// Command petri-pilot provides LLM-augmented Petri net model design and validation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pflow-xyz/petri-pilot/internal/llm"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/react"
	"github.com/pflow-xyz/petri-pilot/pkg/feedback"
	"github.com/pflow-xyz/petri-pilot/pkg/generator"
	"github.com/pflow-xyz/petri-pilot/pkg/mcp"
	"github.com/pflow-xyz/petri-pilot/pkg/schema"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "generate":
		cmdGenerate(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "refine":
		cmdRefine(os.Args[2:])
	case "codegen":
		cmdCodegen(os.Args[2:])
	case "frontend":
		cmdFrontend(os.Args[2:])
	case "mcp":
		cmdMcp()
	case "help", "-h", "--help":
		printUsage()
	case "version", "-v", "--version":
		fmt.Println("petri-pilot v0.1.0")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`petri-pilot - From requirements to running applications

Usage:
  petri-pilot <command> [arguments]

Commands:
  generate    Generate a Petri net model from natural language requirements
  validate    Validate an existing Petri net model
  refine      Refine a model based on validation feedback
  codegen     Generate backend application code from a validated model
  frontend    Generate React frontend from a validated model
  mcp         Run as MCP server (for Claude Desktop, Cursor, etc.)

Options:
  -h, --help      Show this help message
  -v, --version   Show version information

Environment:
  ANTHROPIC_API_KEY   API key for Claude (required for generate/refine)

Examples:
  # Generate a model from requirements
  petri-pilot generate "A simple order processing workflow"

  # Generate with auto-validation loop
  petri-pilot generate -auto -o model.json "User registration flow"

  # Validate a model file
  petri-pilot validate model.json

  # Generate backend application code
  petri-pilot codegen model.json -o ./myworkflow/

  # Generate React frontend
  petri-pilot frontend model.json -o ./myworkflow-frontend/

  # Generate OpenAPI spec only
  petri-pilot codegen -api-only model.json -o api.yaml

  # Run as MCP server
  petri-pilot mcp

For more information, see: https://github.com/pflow-xyz/petri-pilot`)
}

func cmdGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	output := fs.String("o", "", "Output file for generated model (default: stdout)")
	autoValidate := fs.Bool("auto", false, "Auto-validate and refine until valid")
	maxIter := fs.Int("max-iter", 3, "Maximum refinement iterations for -auto")
	verbose := fs.Bool("v", false, "Verbose output")
	model := fs.String("model", "claude-sonnet-4-20250514", "Claude model to use")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: requirements required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot generate [options] <requirements>")
		os.Exit(1)
	}

	requirements := fs.Arg(0)

	// Check for API key
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Create Claude client
	client := llm.NewClaudeClient(llm.ClaudeOptions{
		Model: *model,
	})

	gen := generator.New(client, generator.Options{
		MaxIterations: *maxIter,
		Temperature:   0.7,
		Verbose:       *verbose,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *verbose {
		fmt.Fprintf(os.Stderr, "Generating model from: %s\n", requirements)
	}

	generatedModel, err := gen.Generate(ctx, requirements)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating model: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Generated model: %s (%d places, %d transitions)\n",
			generatedModel.Name, len(generatedModel.Places), len(generatedModel.Transitions))
	}

	// Auto-validate and refine loop
	if *autoValidate {
		v := validator.New(validator.Options{
			MaxStates:         10000,
			EnableSensitivity: false, // Skip sensitivity for speed during iteration
			Parallel:          true,
		})

		for i := 0; i < *maxIter; i++ {
			result, err := v.Validate(generatedModel)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
				break
			}

			if result.Valid && len(result.Warnings) == 0 {
				if *verbose {
					fmt.Fprintf(os.Stderr, "Model validated successfully after %d iteration(s)\n", i+1)
				}
				break
			}

			if *verbose {
				fmt.Fprintf(os.Stderr, "Iteration %d: %d errors, %d warnings\n",
					i+1, len(result.Errors), len(result.Warnings))
			}

			// Build feedback and refine
			fb := feedback.New(requirements, generatedModel, result).Build()

			generatedModel, err = gen.Refine(ctx, fb)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Refinement error: %v\n", err)
				break
			}
		}
	}

	// Output the model
	jsonOutput, err := json.MarshalIndent(generatedModel, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding model: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		if err := os.WriteFile(*output, jsonOutput, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Model saved to: %s\n", *output)
	} else {
		fmt.Println(string(jsonOutput))
	}
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	full := fs.Bool("full", false, "Run full analysis including sensitivity")
	jsonOutput := fs.Bool("json", false, "Output results as JSON")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: model file required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot validate [options] <model.json>")
		os.Exit(1)
	}

	modelPath := fs.Arg(0)

	// Read model file
	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var model schema.Model
	if err := json.Unmarshal(data, &model); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing model: %v\n", err)
		os.Exit(1)
	}

	// Validate
	v := validator.New(validator.Options{
		MaxStates:         10000,
		EnableSensitivity: *full,
		Parallel:          true,
	})

	result, err := v.Validate(&model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	} else {
		printValidationResult(result)
	}

	if !result.Valid {
		os.Exit(1)
	}
}

func printValidationResult(result *schema.ValidationResult) {
	if result.Valid {
		fmt.Println("Validation: PASSED")
	} else {
		fmt.Println("Validation: FAILED")
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range result.Errors {
			fmt.Printf("  [%s] %s\n", err.Code, err.Message)
			if err.Fix != "" {
				fmt.Printf("    Fix: %s\n", err.Fix)
			}
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, warn := range result.Warnings {
			fmt.Printf("  [%s] %s\n", warn.Code, warn.Message)
			if warn.Fix != "" {
				fmt.Printf("    Fix: %s\n", warn.Fix)
			}
		}
	}

	if result.Analysis != nil {
		fmt.Println("\nAnalysis:")
		fmt.Printf("  Bounded: %v\n", result.Analysis.Bounded)
		fmt.Printf("  Live: %v\n", result.Analysis.Live)
		fmt.Printf("  State count: %d\n", result.Analysis.StateCount)

		if result.Analysis.HasDeadlocks {
			fmt.Printf("  Deadlocks: %d\n", len(result.Analysis.Deadlocks))
		}

		if len(result.Analysis.Isolated) > 0 {
			fmt.Printf("  Isolated elements: %v\n", result.Analysis.Isolated)
		}

		if len(result.Analysis.SymmetryGroups) > 0 {
			fmt.Println("  Symmetry groups:")
			for _, g := range result.Analysis.SymmetryGroups {
				fmt.Printf("    %v (impact: %.3f)\n", g.Elements, g.Impact)
			}
		}
	}
}

func cmdRefine(args []string) {
	fs := flag.NewFlagSet("refine", flag.ExitOnError)
	output := fs.String("o", "", "Output file for refined model (default: overwrite input)")
	verbose := fs.Bool("v", false, "Verbose output")
	model := fs.String("model", "claude-sonnet-4-20250514", "Claude model to use")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "Error: model file and instructions required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot refine [options] <model.json> <instructions>")
		os.Exit(1)
	}

	modelPath := fs.Arg(0)
	instructions := fs.Arg(1)

	// Check for API key
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Read model file
	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var currentModel schema.Model
	if err := json.Unmarshal(data, &currentModel); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing model: %v\n", err)
		os.Exit(1)
	}

	// First validate to get current state
	v := validator.New(validator.DefaultOptions())
	validationResult, _ := v.Validate(&currentModel)

	// Create Claude client
	client := llm.NewClaudeClient(llm.ClaudeOptions{
		Model: *model,
	})

	gen := generator.New(client, generator.Options{
		Temperature: 0.5,
		Verbose:     *verbose,
	})

	// Build feedback
	fb := &schema.FeedbackPrompt{
		OriginalRequirements: instructions,
		CurrentModel:         &currentModel,
		ValidationResult:     validationResult,
		Instructions:         instructions,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *verbose {
		fmt.Fprintf(os.Stderr, "Refining model with: %s\n", instructions)
	}

	refinedModel, err := gen.Refine(ctx, fb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error refining model: %v\n", err)
		os.Exit(1)
	}

	// Output the model
	jsonOutput, err := json.MarshalIndent(refinedModel, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding model: %v\n", err)
		os.Exit(1)
	}

	outputPath := *output
	if outputPath == "" {
		outputPath = modelPath
	}

	if err := os.WriteFile(outputPath, jsonOutput, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Refined model saved to: %s\n", outputPath)
}

func cmdCodegen(args []string) {
	fs := flag.NewFlagSet("codegen", flag.ExitOnError)
	output := fs.String("o", "./generated", "Output directory (or file path for -api-only)")
	lang := fs.String("lang", "go", "Target language (go)")
	pkg := fs.String("pkg", "", "Package name (default: model name)")
	includeTests := fs.Bool("tests", true, "Include test files")
	includeInfra := fs.Bool("infra", true, "Include infrastructure files (Dockerfile, docker-compose, migrations)")
	includeAuth := fs.Bool("auth", false, "Include GitHub OAuth authentication")
	includeObs := fs.Bool("observability", false, "Include logging and Prometheus metrics")
	includeDeploy := fs.Bool("deploy", false, "Include K8s manifests and GitHub Actions CI")
	apiOnly := fs.Bool("api-only", false, "Generate OpenAPI spec only")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: model file required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot codegen [options] <model.json>")
		os.Exit(1)
	}

	modelPath := fs.Arg(0)

	// Read model file
	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var model schema.Model
	if err := json.Unmarshal(data, &model); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing model: %v\n", err)
		os.Exit(1)
	}

	// Package name is determined by generator if not specified
	pkgName := *pkg

	// API-only mode: just generate OpenAPI spec
	if *apiOnly {
		gen, err := golang.New(golang.Options{
			PackageName: pkgName,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
			os.Exit(1)
		}

		content, err := gen.Preview(&model, golang.TemplateOpenAPI)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating OpenAPI spec: %v\n", err)
			os.Exit(1)
		}

		// Determine output path
		outPath := *output
		if outPath == "./generated" {
			outPath = "openapi.yaml"
		}

		if err := os.WriteFile(outPath, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Generated OpenAPI spec: %s\n", outPath)
		return
	}

	// Full codegen mode
	// Only Go is supported for now
	if *lang != "go" && *lang != "golang" {
		fmt.Fprintf(os.Stderr, "Error: unsupported language '%s' (only 'go' is currently supported)\n", *lang)
		os.Exit(1)
	}

	// Validate first
	v := validator.New(validator.Options{
		MaxStates:         10000,
		EnableSensitivity: false,
	})

	result, err := v.Validate(&model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	if !result.Valid {
		fmt.Fprintln(os.Stderr, "Error: model validation failed")
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] %s\n", e.Code, e.Message)
		}
		os.Exit(1)
	}

	// Create generator
	gen, err := golang.New(golang.Options{
		OutputDir:            *output,
		PackageName:          pkgName,
		IncludeTests:         *includeTests,
		IncludeInfra:         *includeInfra,
		IncludeAuth:          *includeAuth,
		IncludeObservability: *includeObs,
		IncludeDeploy:        *includeDeploy,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
		os.Exit(1)
	}

	// Generate files
	paths, err := gen.Generate(&model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d files:\n", len(paths))
	for _, path := range paths {
		fmt.Printf("  %s\n", path)
	}
}

func cmdFrontend(args []string) {
	fs := flag.NewFlagSet("frontend", flag.ExitOnError)
	output := fs.String("o", "./frontend", "Output directory for frontend files")
	project := fs.String("project", "", "Project name for package.json (default: model name)")
	apiURL := fs.String("api", "http://localhost:8080", "Backend API base URL")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: model file required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot frontend [options] <model.json>")
		os.Exit(1)
	}

	modelPath := fs.Arg(0)

	// Read model file
	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var model schema.Model
	if err := json.Unmarshal(data, &model); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing model: %v\n", err)
		os.Exit(1)
	}

	// Validate first
	v := validator.New(validator.Options{
		MaxStates:         10000,
		EnableSensitivity: false,
	})

	result, err := v.Validate(&model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	if !result.Valid {
		fmt.Fprintln(os.Stderr, "Error: model validation failed")
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] %s\n", e.Code, e.Message)
		}
		os.Exit(1)
	}

	// Create frontend generator
	gen, err := react.New(react.Options{
		OutputDir:   *output,
		ProjectName: *project,
		APIBaseURL:  *apiURL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
		os.Exit(1)
	}

	// Generate files
	paths, err := gen.Generate(&model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating frontend: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d frontend files:\n", len(paths))
	for _, path := range paths {
		fmt.Printf("  %s\n", path)
	}
	fmt.Printf("\nTo run the frontend:\n")
	fmt.Printf("  cd %s\n", *output)
	fmt.Printf("  npm install\n")
	fmt.Printf("  npm run dev\n")
}

func cmdMcp() {
	if err := mcp.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
