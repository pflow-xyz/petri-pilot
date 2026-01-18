# Petri-Pilot Development Guide

## Project Overview

Petri-pilot generates complete applications from Petri net models. An LLM designs the model via MCP tools, then deterministic code generation produces Go backends and ES modules frontends.

## Architecture

```
JSON Model → Schema Parser → Codegen Context → Templates → Generated Code
```

Key packages:
- `pkg/schema/` - Model types (places, transitions, arcs, roles, views)
- `pkg/codegen/golang/` - Go backend generation
- `pkg/codegen/esmodules/` - Frontend generation
- `pkg/runtime/` - Runtime interfaces (EventStore, Aggregate)
- `pkg/mcp/` - MCP server and tools
- `cmd/` - CLI commands

## Code Generation Pattern

1. Parse JSON into `schema.Model`
2. Build `golang.Context` with computed fields (HasViews, HasAdmin, etc.)
3. Execute Go templates from `templates/*.tmpl`
4. Write generated files to output directory

## Adding New Features

When adding schema features:
1. Add types to `pkg/schema/schema.go`
2. Add context fields to `pkg/codegen/golang/context.go`
3. Add `Has*()` helper if conditionally generated
4. Create/update templates in `pkg/codegen/golang/templates/`
5. Update `generator.go` to include new template
6. Update examples in `examples/*.json`
7. Run `make build-examples` to verify

## Template Conventions

- Templates use Go's `text/template`
- Conditional generation: `{{if .HasFeature}}...{{end}}`
- Access context fields directly: `{{.ModelName}}`, `{{.Routes}}`
- Helper methods: `{{.TransitionRequiresAuth "id"}}`

## Testing

```bash
go test ./...           # All tests
make build-examples     # Regenerate and build all examples
```

## Generated File Structure

Each generated app contains:
- `main.go` - Entry point
- `workflow.go` - Petri net definition
- `aggregate.go` - Event-sourced aggregate
- `api.go` - HTTP handlers
- `events.go` - Event types
- `views.go` - View definitions (if views defined)
- `auth.go`, `middleware.go`, `permissions.go` - Auth (if roles defined)
- `navigation.go` - Navigation (if navigation defined)

## SQLite Only

This project uses SQLite exclusively. Do not add support for other databases.

## No React

Frontend uses vanilla ES modules only. Do not add React, Vue, or other frameworks.

## Import Conventions

Generated code imports from:
- `github.com/pflow-xyz/petri-pilot/pkg/runtime/api`
- `github.com/pflow-xyz/petri-pilot/pkg/runtime/eventstore`
- `github.com/pflow-xyz/petri-pilot/pkg/runtime/aggregate`

## Common Issues

- **Unused imports**: Make imports conditional with `{{if .HasFeature}}`
- **Undefined functions**: Add standalone helpers in template, don't reference non-existent methods
- **Test file count**: Update `generator_test.go` when adding new templates
