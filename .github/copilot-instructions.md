# Copilot Instructions for Petri-Pilot

## Quick Reference

- **Language**: Go 1.21+ for backend, vanilla ES modules for frontend
- **Database**: SQLite only (no Postgres, MySQL, etc.)
- **Frontend**: No React/Vue/Angular - ES modules only
- **Templates**: Go text/template in `pkg/codegen/golang/templates/`

## Code Generation Architecture

This is a **code generator**, not a runtime application. Changes to generated output require modifying templates, not generated files.

### To add a new generated file:
1. Create template in `pkg/codegen/golang/templates/newfile.tmpl`
2. Register in `pkg/codegen/golang/templates.go`
3. Add to generator in `pkg/codegen/golang/generator.go`
4. Update test expected file count in `generator_test.go`

### To add a new schema field:
1. Add to `pkg/schema/schema.go`
2. Add context in `pkg/codegen/golang/context.go`
3. Add `Has*()` method if conditionally used
4. Update templates that need the field

## Template Best Practices

```go
// Conditional imports - avoid unused import errors
{{- if .HasFeature}}
import "package"
{{- end}}

// Conditional code blocks
{{if .HasFeature}}
func FeatureHandler() { ... }
{{end}}

// Don't reference undefined methods - add helpers in template
func localHelper() { ... }
```

## Testing Changes

After any template change:
```bash
make build-examples  # Regenerates all examples and builds them
go test ./...        # Runs all tests
```

## File Locations

| What | Where |
|------|-------|
| Schema types | `pkg/schema/schema.go` |
| Codegen context | `pkg/codegen/golang/context.go` |
| Templates | `pkg/codegen/golang/templates/*.tmpl` |
| Template registry | `pkg/codegen/golang/templates.go` |
| Generator | `pkg/codegen/golang/generator.go` |
| Examples | `examples/*.json` |
| Generated output | `generated/*/` |

## Common Mistakes to Avoid

1. **Don't edit files in `generated/`** - they're overwritten by codegen
2. **Don't add unused imports** - use conditional `{{if}}` blocks
3. **Don't reference `app.engine`** - it doesn't exist, use `NewAggregate()`
4. **Don't forget test updates** - file count changes need test updates
5. **Don't add database options** - SQLite only
6. **Don't add frontend frameworks** - ES modules only
