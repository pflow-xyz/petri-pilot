package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pflow-xyz/petri-pilot/zk-ode/codegen"
)

func cmdZkgen(args []string) {
	fs := flag.NewFlagSet("zkgen", flag.ExitOnError)
	output := fs.String("o", "./zk-generated", "Output directory for generated ZK ODE package")
	pkg := fs.String("pkg", "", "Package name (default: model name)")
	scoringFile := fs.String("scoring", "", "Path to scoring config JSON file")
	asSubmodule := fs.Bool("submodule", false, "Skip go.mod generation (treat as subpackage)")

	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, `petri-pilot zkgen - Generate ZK ODE circuit package from a Petri net model

Usage:
  petri-pilot zkgen [options] <model>

Arguments:
  model    Path to the Petri net model file (.json or .pflow)

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(w, `
Examples:
  petri-pilot zkgen -pkg cascade -o zk-cascade examples/cascade.json
  petri-pilot zkgen -pkg ttt -o zk-ttt -scoring ttt-scoring.json services/tic-tac-toe.json
  petri-pilot zkgen -submodule -pkg mynet -o generated/zk-mynet model.json`)
	}

	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: model file required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot zkgen [options] <model>")
		os.Exit(1)
	}

	modelPath := fs.Arg(0)

	// Read and parse model
	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	model, err := parseModelFile(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing model: %v\n", err)
		os.Exit(1)
	}

	// Determine package name
	pkgName := *pkg
	if pkgName == "" {
		pkgName = model.Name
	}

	// Load scoring config if provided
	var scoring *codegen.ScoringConfig
	if *scoringFile != "" {
		scoring, err = codegen.LoadScoringConfig(*scoringFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading scoring config: %v\n", err)
			os.Exit(1)
		}
	}

	// Create generator
	gen, err := codegen.New(codegen.Options{
		PackageName: pkgName,
		OutputDir:   *output,
		Scoring:     scoring,
		AsSubmodule: *asSubmodule,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating generator: %v\n", err)
		os.Exit(1)
	}

	// Generate files
	paths, err := gen.GenerateToDir(model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d files in %s:\n", len(paths), *output)
	for _, path := range paths {
		fmt.Printf("  %s\n", path)
	}
}
