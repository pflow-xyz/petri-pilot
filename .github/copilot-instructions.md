# Copilot Instructions for Petri-Pilot

> **Note:** Also read `CLAUDE.md` in the repo root for detailed MCP tooling, testing workflows, and codebase architecture.

## App Generation Workflow

When assigned an issue with the `app-request` label:

1. **Design the Model**: Create `examples/<app-name>.json` with:
   - Places (states in the workflow)
   - Transitions (actions/events)
   - Arcs (flow between places and transitions)
   - Roles and access rules
   - Views for UI rendering
   - Navigation menu

2. **Generate Code**: Run `./petri-pilot codegen examples/<app>.json -o generated/<app>/`

3. **Add E2E Tests**: Copy templates from `e2e/` and customize:
   - `api.test.ts` - Test each API endpoint
   - `app.test.ts` - Test UI workflows with Playwright
   - Replace `{{PLACEHOLDERS}}` with actual values

4. **Verify**: Run `cd generated/<app> && go build && go test ./...`

5. **Update PR**: Mark checkboxes complete, request review

## Quick Reference

- **Language**: Go 1.25+ for backend, vanilla ES modules for frontend
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

## Interactive Testing with MCP Tools

Use the petri-pilot MCP tools to manually test generated apps before committing.

### 1. Start the Service

```
service_start(directory="/absolute/path/to/generated/app", port=8080)
```

Check health and logs if needed:
```
service_health(service_id="svc-1")
service_logs(service_id="svc-1", lines=50)
```

### 2. Test with Headless Browser

Launch browser and take screenshots to verify UI:
```
e2e_start_browser(url="http://localhost:8080")
e2e_screenshot(session_id="browser-1")
```

Execute JavaScript to test workflows:
```
e2e_eval(session_id="browser-1", code=`
  await window.pilot.loginAs(['admin']);
  await window.pilot.create();
  return window.pilot.getEnabled();
`)
```

Check for console errors or failed requests:
```
e2e_events(session_id="browser-1", types="console,exception")
```

### 3. Validate README

Preview the generated README to ensure it renders correctly:
```
markdown_preview(file_path="/absolute/path/to/generated/app/README.md")
```

### 4. Cleanup

Always stop services and browsers when done:
```
e2e_stop_browser(session_id="browser-1")
service_stop(service_id="svc-1")
```

### Testing Checklist

Before marking a PR ready:
- [ ] Service starts without errors
- [ ] UI loads and displays correctly (screenshot)
- [ ] Can login and execute transitions
- [ ] No console errors or exceptions
- [ ] README renders correctly with Mermaid diagrams
- [ ] `go build` and `go test` pass

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
