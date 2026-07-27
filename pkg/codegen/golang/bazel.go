package golang

import (
	"path"
	"sort"
	"strings"
)

// Bazel build configuration shared by all generated apps. Mirrors the
// petri-pilot / go-pflow Bazel setup (rules_go + gazelle, Bzlmod).
const (
	// petriPilotModule is the Go module path of the petri-pilot monorepo. In
	// submodule mode the generated app lives underneath it, so its in-repo Bazel
	// labels are derived by stripping this prefix from the import path.
	petriPilotModule = "github.com/pflow-xyz/petri-pilot"

	// bazelGoSDKVersion is the hermetic Go SDK pinned for standalone modules.
	// Tracks the ecosystem-wide line (go-pflow + consumers) for cache parity.
	bazelGoSDKVersion = "1.26.0"

	// petriPilotRepo is the Bzlmod repo name petri-pilot resolves to when a
	// standalone app depends on it as an external module.
	petriPilotRepo = "@com_github_pflow_xyz_petri_pilot"
)

// BazelBuildContext holds everything the Bazel templates need to render a
// generated app's BUILD.bazel (and, for standalone modules, MODULE.bazel et al).
type BazelBuildContext struct {
	// Submodule is true when the app is part of the petri-pilot module (no
	// go.mod / MODULE.bazel of its own).
	Submodule bool

	ImportPath string // go_library importpath
	LibName    string // go_library target name
	Deps       []string
	HasTest    bool
	TestDeps   []string

	// Standalone-only fields.
	Binary       bool   // emit a go_binary (package main)
	BinName      string // go_binary target name
	ModuleName   string // Bzlmod module() name
	GoSDKVersion string // hermetic Go SDK version

	// GraphQL subpackage (graph/), emitted as its own BUILD.bazel when present.
	HasGraph        bool
	GraphLabel      string // dep label the parent uses to reach graph/
	GraphImportPath string
	GraphDeps       []string

	// UseRepos lists the Bzlmod go_deps repos referenced by Deps; drives the
	// MODULE.bazel use_repo() call for standalone modules.
	UseRepos []string
}

// computeBazelBuild derives the Bazel build spec for a generated app from the
// codegen context and the active generator options. Dependency labels are
// feature-gated to match the imports each enabled template pulls in.
func computeBazelBuild(ctx *Context, opts Options) *BazelBuildContext {
	b := &BazelBuildContext{
		Submodule:    opts.AsSubmodule,
		ImportPath:   ctx.ModulePath,
		HasTest:      opts.IncludeTests,
		GoSDKVersion: bazelGoSDKVersion,
	}

	// Label prefix for petri-pilot's own runtime packages: a bare in-repo label
	// for submodules, an external-repo label for standalone modules.
	ppPrefix := "//"
	if !opts.AsSubmodule {
		ppPrefix = petriPilotRepo + "//"
	}

	if opts.AsSubmodule {
		b.LibName = ctx.PackageName
	} else {
		// Standalone apps are package main: a foo_lib library embedded by a foo
		// binary, following gazelle's convention for command packages.
		name := ctx.PackageName
		if name == "" || name == "main" {
			name = SanitizePackageName(ctx.ModelName)
		}
		if name == "" {
			name = "app"
		}
		b.Binary = true
		b.BinName = name
		b.LibName = name + "_lib"
		b.ModuleName = strings.ReplaceAll(name, "-", "_")
	}

	deps := []string{
		ppPrefix + "pkg/runtime/api",
		ppPrefix + "pkg/serve",
		"@com_github_google_uuid//:uuid",
		"@com_github_gorilla_websocket//:websocket",
		"@com_github_pflow_xyz_go_pflow//eventsource",
	}

	if ctx.HasGraphQL() {
		b.HasGraph = true
		b.GraphImportPath = ctx.ModulePath + "/graph"
		b.GraphDeps = []string{"@com_github_pflow_xyz_go_pflow//eventsource"}
		if opts.AsSubmodule {
			rel := strings.TrimPrefix(ctx.ModulePath, petriPilotModule+"/")
			b.GraphLabel = "//" + path.Join(rel, "graph")
		} else {
			b.GraphLabel = "//graph"
		}
		deps = append(deps, b.GraphLabel)
	}

	if ctx.HasAccessControl() || opts.IncludeAuth {
		deps = append(deps, "@org_golang_x_oauth2//:oauth2", "@org_golang_x_oauth2//github")
	}
	if ctx.HasGuards() {
		deps = append(deps, ppPrefix+"pkg/dsl")
	}
	if ctx.UsesMetamodelRuntime() {
		deps = append(deps, "@com_github_holiman_uint256//:uint256")
	}
	if ctx.HasPrediction() {
		deps = append(deps,
			"@com_github_pflow_xyz_go_pflow//petri",
			"@com_github_pflow_xyz_go_pflow//solver",
		)
	}

	b.Deps = sortBazelLabels(dedupe(deps))
	b.TestDeps = []string{"@com_github_pflow_xyz_go_pflow//eventsource"}

	if !opts.AsSubmodule {
		// Standalone modules need a use_repo() entry for every external repo their
		// labels reference (petri-pilot is a direct dep via the generated go.mod).
		repos := []string{strings.TrimPrefix(petriPilotRepo, "@")}
		for _, d := range append(append([]string{}, b.Deps...), b.TestDeps...) {
			if r := repoFromLabel(d); r != "" {
				repos = append(repos, r)
			}
		}
		sort.Strings(repos)
		b.UseRepos = dedupe(repos)
	}
	return b
}

// repoFromLabel extracts the Bzlmod repo name from an external label, e.g.
// "@com_github_google_uuid//:uuid" -> "com_github_google_uuid". Returns "" for
// in-repo labels.
func repoFromLabel(label string) string {
	if !strings.HasPrefix(label, "@") {
		return ""
	}
	rest := strings.TrimPrefix(label, "@")
	if i := strings.Index(rest, "//"); i >= 0 {
		return rest[:i]
	}
	return rest
}

// dedupe removes duplicate labels, preserving none of the order (caller sorts).
func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := in[:0:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// sortBazelLabels orders labels buildifier-style: in-repo labels (// or :)
// first, then external (@) labels, each group sorted alphabetically.
func sortBazelLabels(labels []string) []string {
	sort.SliceStable(labels, func(i, j int) bool {
		ext := func(s string) bool { return strings.HasPrefix(s, "@") }
		if ext(labels[i]) != ext(labels[j]) {
			return !ext(labels[i]) // local labels sort first
		}
		return labels[i] < labels[j]
	})
	return labels
}
