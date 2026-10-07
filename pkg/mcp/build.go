package mcp

// petri_build is the single application-generation tool: it replaces
// petri_application and petri_bundle outright (folding their compilation
// logic in unchanged) and takes over "language": "go" from petri_codegen,
// because it is the only one of the three that produces something with an
// HTTP surface worth runtime-verifying.
//
// Unlike its predecessors, output_dir is required — the whole point is a
// working app on disk, not a preview — and by default (verify: true) the
// tool actually builds the app, starts the real binary on a free port, and
// runs pkg/appverify against it before returning, mirroring how
// sim-pflow-xyz calibrates a model before publishing it.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/appverify"
	"github.com/pflow-xyz/petri-pilot/pkg/bundle"
	"github.com/pflow-xyz/petri-pilot/pkg/codegen/golang"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
	"github.com/pflow-xyz/petri-pilot/pkg/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
)

func buildTool() mcp.Tool {
	return mcp.NewTool("petri_build",
		mcp.WithDescription("Generate a full, runnable Go application from a Petri net model, an Application spec, or a raw bundle document, write it to output_dir, and (by default) actually build and run it and verify it behaves correctly — not just that it compiles. Accepts exactly one of 'model' (single-net), 'spec' (+optional 'fusions', composed via one subnet per entity), 'bundle' (raw bundle document), or 'id' (a previously stored spec — wins over the others if given). Replaces petri_application, petri_bundle, and petri_codegen's language='go' option. Whenever the build resolves through a spec id (given directly via 'id', or freshly stored from 'model'/'spec'/'bundle'), a lineage edge recording this build's outcome is written to the app store — pass 'prompt' to also attach free-text intent. The result carries the resolved spec 'id'."),
		mcp.WithString("model",
			mcp.Description("Single-net Petri net model as JSON or tokenmodel DSL. Mutually exclusive with 'spec', 'bundle' and 'id'."),
		),
		mcp.WithString("spec",
			mcp.Description("Application specification as JSON (entities with fields/states/actions). Mutually exclusive with 'model', 'bundle' and 'id'."),
		),
		mcp.WithString("fusions",
			mcp.Description("Optional JSON array of cross-entity rendezvous, used only with 'spec': [{\"id\":\"...\",\"members\":[{\"entity\":\"...\",\"action\":\"...\"}]}]"),
		),
		mcp.WithString("bundle",
			mcp.Description("Raw bundle document JSON: {name, subnets: [{id, net_type, model}], links: [...]}. Mutually exclusive with 'model', 'spec' and 'id'."),
		),
		mcp.WithString("id",
			mcp.Description("Optional: build from a previously stored spec id instead of 'model'/'spec'/'bundle'. Wins over the others if given."),
		),
		mcp.WithString("prompt",
			mcp.Description("Optional free-text description of intent, recorded alongside this build's lineage edge in the app store."),
		),
		mcp.WithString("output_dir",
			mcp.Required(),
			mcp.Description("Directory to write the generated, standalone (own go.mod) application into. Required — this tool always writes to disk."),
		),
		mcp.WithString("module_path",
			mcp.Description("Go module/import path of the generated app (default: app/<name>)"),
		),
		mcp.WithString("package",
			mcp.Description("Go package name, single-net models only (default: derived from model name)"),
		),
		mcp.WithString("extensions",
			mcp.Description("Optional JSON object with extensions for a single-net model: {\"roles\":[...], \"views\":[...], \"admin\":{...}, \"navigation\":{...}}"),
		),
		mcp.WithBoolean("verify",
			mcp.Description("Build and run the generated app and verify it against the model's own firing rule before returning (default: true). Set false to skip the Go toolchain / for fast iteration."),
		),
	)
}

// buildReport is petri_build's structured return value.
type buildReport struct {
	OutputDir    string   `json:"output_dir"`
	ID           string   `json:"id,omitempty"`
	FilesWritten []string `json:"files_written"`
	Build        struct {
		OK     bool   `json:"ok"`
		Errors string `json:"errors,omitempty"`
	} `json:"build"`
	Verify struct {
		Ran              bool                 `json:"ran"`
		Passed           bool                 `json:"passed"`
		TransitionsFired []string             `json:"transitions_fired,omitempty"`
		Mismatches       []appverify.Mismatch `json:"mismatches,omitempty"`
		Errors           []string             `json:"errors,omitempty"`
	} `json:"verify"`
}

func handleBuild(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idParam := request.GetString("id", "")
	promptParam := request.GetString("prompt", "")
	persist := idParam != "" || promptParam != ""

	modelJSON := request.GetString("model", "")
	specJSON := request.GetString("spec", "")
	bundleJSON := request.GetString("bundle", "")

	if idParam != "" {
		store, serr := getAppStore()
		if serr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", serr)), nil
		}
		kind, content, gerr := store.Get(idParam)
		if gerr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading spec %q: %v", idParam, gerr)), nil
		}
		modelJSON, specJSON, bundleJSON = "", "", ""
		switch kind {
		case "model":
			modelJSON = string(content)
		case "spec":
			specJSON = string(content)
		case "bundle":
			bundleJSON = string(content)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("spec %q has unrecognized kind %q", idParam, kind)), nil
		}
	} else {
		inputsGiven := 0
		for _, s := range []string{modelJSON, specJSON, bundleJSON} {
			if s != "" {
				inputsGiven++
			}
		}
		if inputsGiven != 1 {
			return mcp.NewToolResultError("petri_build requires exactly one of 'model', 'spec', 'bundle', or 'id'"), nil
		}
	}

	outputDir := request.GetString("output_dir", "")
	if outputDir == "" {
		return mcp.NewToolResultError("output_dir is required"), nil
	}

	verify := true
	if args, ok := request.Params.Arguments.(map[string]any); ok {
		if v, exists := args["verify"]; exists {
			if b, ok := v.(bool); ok {
				verify = b
			}
		}
	}

	var (
		files       []golang.GeneratedFile
		appName     string
		isBundle    bool
		verifyModel []byte // model JSON (single-net) or bundle JSON, for appverify
		compiledB   *goflowmetamodel.Bundle
	)

	switch {
	case modelJSON != "":
		parsed, err := parseModelV2(modelJSON)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
		}
		model := parsed.Model
		appName = model.Name

		opts := validator.DefaultOptions()
		opts.EnableSensitivity = false
		v := validator.New(opts)
		result, err := v.Validate(model)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
		}
		if !result.Valid {
			errJSON, _ := json.MarshalIndent(result.Errors, "", "  ")
			return mcp.NewToolResultError(fmt.Sprintf("model validation failed:\n%s", errJSON)), nil
		}
		implResult := v.ValidateImplementability(model)
		if !implResult.Implementable {
			errJSON, _ := json.MarshalIndent(implResult.Errors, "", "  ")
			return mcp.NewToolResultError(fmt.Sprintf("model not implementable:\n%s", errJSON)), nil
		}

		app := extensions.NewApplicationSpec(model)
		if parsed.Version == "2.0" && len(parsed.Extensions) > 0 {
			if err := parseV2Extensions(app, parsed.Extensions); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid v2 extensions: %v", err)), nil
			}
		}
		if extJSON := request.GetString("extensions", ""); extJSON != "" {
			if err := parseExtensions(app, extJSON); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid extensions JSON: %v", err)), nil
			}
		}

		modulePath := request.GetString("module_path", "app/"+bundle.PackageNameFor(model.Name))
		gen, err := golang.New(golang.Options{
			ModulePath:   modulePath,
			PackageName:  request.GetString("package", ""),
			IncludeTests: true,
			// AsSubmodule left false: this is a standalone module with its
			// own go.mod and main.go.
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating generator: %v", err)), nil
		}
		if app.HasRoles() || app.HasViews() || app.HasNavigation() || app.HasAdmin() {
			files, err = gen.GenerateFilesFromApp(app)
		} else {
			files, err = gen.GenerateFiles(model)
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("code generation failed: %v", err)), nil
		}

		verifyModel, err = json.Marshal(model)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshaling model for verification: %v", err)), nil
		}

	case specJSON != "":
		var appSpec metamodel.Application
		if err := json.Unmarshal([]byte(specJSON), &appSpec); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid application spec JSON: %v", err)), nil
		}
		if len(appSpec.Entities) == 0 {
			return mcp.NewToolResultError("application spec must contain at least one entity"), nil
		}
		appName = appSpec.Name
		isBundle = true

		input := bundle.ApplicationInput{Name: appSpec.Name}
		for _, e := range appSpec.Entities {
			converted, err := e.ToExtensions()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("entity %q: %v", e.ID, err)), nil
			}
			input.Entities = append(input.Entities, converted)
		}
		if fusionsJSON := request.GetString("fusions", ""); fusionsJSON != "" {
			if err := json.Unmarshal([]byte(fusionsJSON), &input.Fusions); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid fusions JSON: %v", err)), nil
			}
		}
		compiled, err := bundle.CompileApplication(input)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("compiling application bundle: %v", err)), nil
		}
		compiledB = compiled.Bundle

		modulePath := request.GetString("module_path", "app/"+bundle.PackageNameFor(appSpec.Name))
		gen, err := golang.New(golang.Options{ModulePath: modulePath, IncludeTests: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating generator: %v", err)), nil
		}
		files, err = gen.GenerateBundleFiles(compiled.Bundle)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("generating application: %v", err)), nil
		}

		verifyModel, err = json.Marshal(compiled.Bundle)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshaling bundle for verification: %v", err)), nil
		}

	default: // bundleJSON != ""
		b, err := bundle.Load([]byte(bundleJSON), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading bundle: %v", err)), nil
		}
		appName = b.Name
		isBundle = true
		compiledB = b

		modulePath := request.GetString("module_path", "app/"+bundle.PackageNameFor(b.Name))
		gen, err := golang.New(golang.Options{ModulePath: modulePath, IncludeTests: true})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating generator: %v", err)), nil
		}
		files, err = gen.GenerateBundleFiles(b)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("generating bundle app: %v", err)), nil
		}

		verifyModel, err = json.Marshal(b)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshaling bundle for verification: %v", err)), nil
		}
	}

	// Resolve the spec id this build is against, when persistence was
	// requested ('id' or 'prompt' given). 'model' may be DSL text rather
	// than JSON, so store the already-parsed-and-marshaled form
	// (verifyModel) under kind "model" rather than the raw input; 'spec' and
	// 'bundle' are stored as the original JSON text they were given as,
	// since that is what a later petri_build(id=...) needs to reconstruct
	// the same input path.
	var resolvedSpecID string
	if persist {
		if idParam != "" {
			resolvedSpecID = idParam
		} else {
			store, serr := getAppStore()
			if serr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", serr)), nil
			}
			var storeKind string
			var storeContent []byte
			switch {
			case modelJSON != "":
				storeKind, storeContent = "model", verifyModel
			case specJSON != "":
				storeKind, storeContent = "spec", []byte(specJSON)
			default:
				storeKind, storeContent = "bundle", []byte(bundleJSON)
			}
			resolvedSpecID, serr = store.Put(storeKind, storeContent)
			if serr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("storing spec: %v", serr)), nil
			}
		}
	}

	modulePath := request.GetString("module_path", "app/"+bundle.PackageNameFor(appName))

	report := &buildReport{OutputDir: outputDir, ID: resolvedSpecID}
	for _, f := range files {
		p := filepath.Join(outputDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating %s: %v", filepath.Dir(p), err)), nil
		}
		if err := os.WriteFile(p, f.Content, 0o644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("writing %s: %v", f.Name, err)), nil
		}
		report.FilesWritten = append(report.FilesWritten, f.Name)
	}

	// A bundle's root package (bundle.go/app.go/flatmodel.go) is a library
	// package, not "main" — GenerateBundleFiles never emits a go.mod or an
	// entry point, because the existing composed-app pipeline only ever
	// registered entities as separate services (see CLAUDE.md's "cross-entity
	// commands" section). petri_build needs one running binary that serves
	// every entity's API *and* the fused commands on one port, so it adds a
	// small cmd/server/main.go harness here.
	if isBundle {
		mainFiles, err := generateBundleHarness(compiledB, modulePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("generating run harness: %v", err)), nil
		}
		for _, f := range mainFiles {
			p := filepath.Join(outputDir, f.Name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("creating %s: %v", filepath.Dir(p), err)), nil
			}
			if err := os.WriteFile(p, f.Content, 0o644); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("writing %s: %v", f.Name, err)), nil
			}
			report.FilesWritten = append(report.FilesWritten, f.Name)
		}
	}
	if err := writeGoMod(outputDir, modulePath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("writing go.mod: %v", err)), nil
	}
	report.FilesWritten = append(report.FilesWritten, "go.mod")

	buildEnv := envWithLocalReplace()

	if out, err := appverify.Tidy(outputDir, buildEnv...); err != nil {
		report.Build.OK = false
		report.Build.Errors = fmt.Sprintf("%v\n%s", err, string(out))
		return finishBuild(report, persist, resolvedSpecID, promptParam)
	}
	if out, err := appverify.BuildAll(outputDir, buildEnv...); err != nil {
		report.Build.OK = false
		report.Build.Errors = fmt.Sprintf("%v\n%s", err, string(out))
		return finishBuild(report, persist, resolvedSpecID, promptParam)
	}
	report.Build.OK = true

	if verify {
		report.Verify.Ran = true
		binPath := filepath.Join(outputDir, ".petri-build-run")
		buildTarget := "."
		if isBundle {
			buildTarget = "./cmd/server"
		}
		_, err := appverify.BuildBinary(outputDir, binPath, buildTarget, buildEnv...)
		if err != nil {
			report.Verify.Errors = append(report.Verify.Errors, fmt.Sprintf("building runnable binary: %v", err))
		} else {
			proc, err := appverify.StartProcess(outputDir, binPath, 20*time.Second, buildEnv...)
			if err != nil {
				stdout, stderr := proc.Output()
				report.Verify.Errors = append(report.Verify.Errors, fmt.Sprintf("starting app: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr))
			} else {
				defer proc.Stop()
				vctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				vreport, err := appverify.VerifyApp(vctx, proc.URL, verifyModel, isBundle)
				if err != nil {
					report.Verify.Errors = append(report.Verify.Errors, err.Error())
				} else {
					report.Verify.Passed = vreport.Passed
					report.Verify.TransitionsFired = vreport.TransitionsFired
					report.Verify.Mismatches = vreport.Mismatches
					report.Verify.Errors = append(report.Verify.Errors, vreport.Errors...)
				}
			}
			os.Remove(binPath)
		}
	}

	return finishBuild(report, persist, resolvedSpecID, promptParam)
}

// finishBuild records this build's outcome as a lineage edge (when
// persistence was requested) and marshals the final report. Called from
// every exit point past spec resolution — including the early returns on a
// failed `go mod tidy`/`go build` — so a failed build is recorded too, not
// just a successful one.
func finishBuild(report *buildReport, persist bool, resolvedSpecID, promptParam string) (*mcp.CallToolResult, error) {
	if persist && resolvedSpecID != "" {
		store, serr := getAppStore()
		if serr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("opening app store: %v", serr)), nil
		}

		var note string
		switch {
		case !report.Build.OK:
			errs := report.Build.Errors
			if len(errs) > 200 {
				errs = errs[:200] + "..."
			}
			note = fmt.Sprintf("build failed: %s", errs)
		case report.Verify.Ran && report.Verify.Passed:
			note = fmt.Sprintf("build ok, verify passed, %d transition(s) fired", len(report.Verify.TransitionsFired))
		case report.Verify.Ran:
			note = fmt.Sprintf("build ok, verify failed, %d mismatch(es)", len(report.Verify.Mismatches))
		default:
			note = "build ok, verify skipped"
		}

		// A build doesn't derive a new spec from a parent — it annotates an
		// existing one — so the lineage edge carries no parent; any real
		// parent (from an earlier petri_extend) is already recorded on that
		// spec id and History finds it there.
		var promptID *string
		if promptParam != "" {
			pid, perr := store.RecordPrompt(promptParam, resolvedSpecID, resolvedSpecID)
			if perr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("recording prompt: %v", perr)), nil
			}
			promptID = &pid
		}
		if lerr := store.RecordLineage(resolvedSpecID, nil, "petri_build", promptID, note); lerr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("recording lineage: %v", lerr)), nil
		}
	}

	resultJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling report: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// envWithLocalReplace passes PETRI_PILOT_LOCAL_PATH through to `go build` /
// the generator unchanged if the MCP server's own process has it set (the
// existing dev convenience go_mod.tmpl already reads); petri_build does not
// invent one.
func envWithLocalReplace() []string {
	if v := os.Getenv("PETRI_PILOT_LOCAL_PATH"); v != "" {
		return []string{"PETRI_PILOT_LOCAL_PATH=" + v}
	}
	return nil
}

// writeGoMod writes a standalone go.mod for a bundle app's output_dir.
// GenerateBundleFiles never emits one (see generateBundleHarness's doc
// comment), and the single-net path already gets one from go_mod.tmpl, so
// this only actually runs for bundle/spec input — but it is harmless (and
// skipped) if a go.mod already exists.
func writeGoMod(dir, modulePath string) error {
	path := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(path); err == nil {
		return nil // single-net path already wrote one via go_mod.tmpl
	}

	var replace string
	if local := os.Getenv("PETRI_PILOT_LOCAL_PATH"); local != "" {
		replace = fmt.Sprintf("\nreplace github.com/pflow-xyz/petri-pilot => %s\n", local)
	}
	content := fmt.Sprintf(`module %s

go 1.25.6

require github.com/pflow-xyz/petri-pilot v0.17.1
%s`, modulePath, replace)
	return os.WriteFile(path, []byte(content), 0o644)
}

// generateBundleHarness renders cmd/server/main.go: a combined HTTP entry
// point for a composed app. It mounts the bundle-level /health, /ready and
// /api/schema, the composition root's /fire/<transition> (cross-entity
// commands), and each entity at "/<subnet-id>/..." (its own generated
// BuildRouter, unmodified, reached via http.StripPrefix so its internal
// routes — "/api/<slug>", "/api/<transition>", etc — resolve exactly as
// they do when that entity is served standalone).
//
// This is new generated surface, not a rewrite of an existing template:
// until now a composed app's root only ever mounted the fused-command
// handler (bundle_service.tmpl's App.Handler()), registered as a *separate*
// service from each entity's own (see CLAUDE.md's cross-entity commands
// section) — so there was no single HTTP entry point that could create an
// order or reserve stock, only fire an already-fused rendezvous. That gap is
// exactly what blocks runtime verification (and any real client) from
// reaching a composed app's entities at all, so closing it is in scope here.
func generateBundleHarness(b *goflowmetamodel.Bundle, modulePath string) ([]golang.GeneratedFile, error) {
	bc, err := golang.NewBundleContext(b, golang.ContextOptions{ModulePath: modulePath})
	if err != nil {
		return nil, fmt.Errorf("building bundle context: %w", err)
	}

	tmpl := template.Must(template.New("bundle_main").Parse(bundleMainTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, bc); err != nil {
		return nil, fmt.Errorf("rendering run harness: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("generated cmd/server/main.go is not valid Go: %w\n%s", err, buf.String())
	}
	return []golang.GeneratedFile{
		{Name: filepath.Join("cmd", "server", "main.go"), Content: formatted},
	}, nil
}

const bundleMainTemplate = `// Code generated by petri-pilot (petri_build run harness). DO NOT EDIT.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/pflow-xyz/go-pflow/eventsource"
	root "{{.ModulePath}}"
{{- range .Entities}}
	{{.PackageName}} "{{$.ModulePath}}/{{.PackageName}}"
{{- end}}
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store := eventsource.NewMemoryStore()
	defer store.Close()

	app, err := root.NewApp(store)
	if err != nil {
		log.Fatalf("failed to create composed app: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\":\"ok\"}"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\":\"ok\"}"))
	})
	mux.HandleFunc("/api/schema", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(root.FlatModel())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(body)
	})
	// Cross-entity commands: the composition root already registers exact
	// "POST /fire/<transition>" patterns, unchanged by mounting under this
	// prefix.
	mux.Handle("/fire/", app.Handler())

{{range .Entities}}
	{{.PackageName}}App := {{.PackageName}}.NewApplication(store)
	mux.Handle("/{{.SubnetID}}/", http.StripPrefix("/{{.SubnetID}}", {{.PackageName}}.BuildRouter({{.PackageName}}App)))
{{end}}

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
`
