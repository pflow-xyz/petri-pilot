# Petri-Pilot Development Guide

## Project Overview

Petri-pilot generates complete applications from Petri net models. An LLM designs the model via MCP tools, then deterministic code generation produces Go backends and ES modules frontends.

## Generating New Applications

The primary workflow is: **Design → Validate → Generate → Test → Iterate**

### 1. Design the Model

Start by designing a Petri net model. Use `petri_validate` to check structure:

```
petri_validate(model='{"name":"order","places":[{"id":"pending"},{"id":"shipped"}],"transitions":[{"id":"ship"}],"arcs":[{"from":"pending","to":"ship"},{"from":"ship","to":"shipped"}]}')
```

Use `petri_simulate` to verify workflow behavior before generating code:

```
petri_simulate(model='...', transitions='["ship"]')
```

Use `petri_analyze` for deeper analysis (reachability, deadlocks, liveness).

### 2. Generate Code

For a complete full-stack application with entities, roles, and pages:

```
petri_application(spec='{"name":"order-tracker","entities":[...],"roles":[...],"pages":[...],"workflows":[...]}')
```

For just a backend from a Petri net model:

```
petri_codegen(model='...', language='go', package='ordertracker')
```

For just a frontend:

```
petri_frontend(model='...', api_url='http://localhost:8080')
```

Preview individual files before full generation:

```
petri_preview(model='...', file='api')  # Preview API handlers
petri_preview(model='...', file='workflow')  # Preview Petri net definition
```

### 3. Start and Test the Service

Start the generated service:

```
service_start(directory='/path/to/generated/app', port=8080)
```

Launch a browser session to test the UI:

```
e2e_start_browser(url='http://localhost:8080')
```

Take screenshots to verify the UI:

```
e2e_screenshot(session_id='browser-1')
```

Execute JavaScript to interact with the app:

```
e2e_eval(session_id='browser-1', code='await window.pilot.loginAs(["admin"]); return window.pilot.getEnabled();')
```

Check for errors in console/network:

```
e2e_events(session_id='browser-1', types='console,exception')
```

### 4. Iterate

If something isn't working:

1. Check service logs: `service_logs(service_id='svc-1')`
2. Check browser events: `e2e_events(session_id='browser-1')`
3. Modify the model using `petri_extend`:
   ```
   petri_extend(model='...', operations='[{"op":"add_place","id":"cancelled"},{"op":"add_transition","id":"cancel"},{"op":"add_arc","from":"pending","to":"cancel"},{"op":"add_arc","from":"cancel","to":"cancelled"}]')
   ```
4. Regenerate and restart:
   ```
   service_stop(service_id='svc-1')
   petri_application(spec='...')  # or petri_codegen
   service_start(directory='...', port=8080)
   ```
5. Refresh browser and retest

### 5. Cleanup

When done testing:

```
e2e_stop_browser(session_id='browser-1')
service_stop(service_id='svc-1')
```

List running services/sessions:

```
service_list()
e2e_list_sessions()
```

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

### E2E Tests

E2E tests use Puppeteer with Chrome. On macOS, set the Chrome path:

```bash
cd e2e
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" npm test
```

On Linux (CI), the default `/usr/bin/chromium` is used.

### MCP Testing Tools

The MCP server provides tools for testing generated services interactively during development. These tools allow you to start services, launch headless browsers, and execute JavaScript in the browser context.

#### Service Management

```
# Start a generated service
mcp__petri-pilot__service_start(directory="/path/to/generated/service", port=8080)

# Check service health
mcp__petri-pilot__service_health(service_id="svc-1")

# View service logs
mcp__petri-pilot__service_logs(service_id="svc-1", lines=50)

# Stop a service
mcp__petri-pilot__service_stop(service_id="svc-1")

# List all running services
mcp__petri-pilot__service_list()
```

#### Browser Testing

```
# Start a headless browser session
mcp__petri-pilot__e2e_start_browser(url="http://localhost:8080")

# Take a screenshot
mcp__petri-pilot__e2e_screenshot(session_id="browser-1")

# Execute JavaScript directly in browser (preferred - works without debug session)
mcp__petri-pilot__e2e_run(session_id="browser-1", code="document.title")

# Execute JavaScript via debug API (requires debug session endpoint)
mcp__petri-pilot__e2e_eval(session_id="browser-1", code="return document.title")

# Navigate to a different URL
mcp__petri-pilot__e2e_navigate(session_id="browser-1", url="http://localhost:8080/dashboard")

# Click an element by CSS selector
mcp__petri-pilot__e2e_click(session_id="browser-1", selector=".btn-primary")

# Type text into an input field
mcp__petri-pilot__e2e_type(session_id="browser-1", selector="#username", text="admin")

# Wait for element to be visible or condition to be true
mcp__petri-pilot__e2e_wait(session_id="browser-1", selector=".success-message")
mcp__petri-pilot__e2e_wait(session_id="browser-1", condition="window.loaded === true")

# Get captured browser events (console logs, network requests, exceptions)
mcp__petri-pilot__e2e_events(session_id="browser-1", types="console,exception")

# Stop browser session
mcp__petri-pilot__e2e_stop_browser(session_id="browser-1")
```

**Note:** Prefer `e2e_run` over `e2e_eval` - it executes JavaScript directly via chromedp without requiring a debug session API endpoint.

#### Using the Pilot API

Generated frontends include a `window.pilot` API for testing:

```javascript
// Authentication
await window.pilot.loginAs(['admin', 'editor'])
await window.pilot.logout()

// Instance management
await window.pilot.create()
await window.pilot.view(instanceId)
await window.pilot.refresh()

// Execute transitions
await window.pilot.action('submit', { field: 'value' })

// Query state
window.pilot.getCurrentInstance()
window.pilot.getEnabled()
window.pilot.getEvents()

// Assertions
window.pilot.assertState({ place: 1 })
window.pilot.assertEnabled(['transition1', 'transition2'])
```

#### Example: Testing a Service

```javascript
// 1. Start service and browser
service_start(directory="/path/to/generated/blog-post", port=8080)
e2e_start_browser(url="http://localhost:8080")

// 2. Login using e2e_run (executes JS directly via chromedp)
e2e_run(session_id="browser-1", code=`(async () => {
  const resp = await fetch('/api/debug/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login: 'test', roles: ['admin', 'author'] })
  });
  const data = await resp.json();
  localStorage.setItem('auth', JSON.stringify(data));
  return data;
})()`)

// 3. Click to create a new instance
e2e_click(session_id="browser-1", selector=".btn-create")
e2e_wait(session_id="browser-1", selector=".instance-detail")

// 4. Execute a workflow transition via API
e2e_run(session_id="browser-1", code=`(async () => {
  const auth = JSON.parse(localStorage.getItem('auth'));
  const resp = await fetch('/api/submit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + auth.token },
    body: JSON.stringify({ aggregate_id: 'xxx', data: {} })
  });
  return await resp.json();
})()`)

// 5. Take screenshot to verify UI
e2e_screenshot(session_id="browser-1")

// 6. Cleanup
e2e_stop_browser(session_id="browser-1")
service_stop(service_id="svc-1")
```

#### Notes

- Browser session IDs are managed by the MCP server and increment globally
- **Use `e2e_run` for JavaScript execution** - it works directly via chromedp without needing a debug session
- `e2e_eval` requires the app to have a `/api/debug/sessions/:id/eval` endpoint (may not exist)
- For async code in `e2e_run`, wrap in an IIFE: `(async () => { ... })()`
- The pilot API methods are async - use `await`

## Monitoring GitHub Actions

Use `gh` CLI to monitor CI runs:

```bash
# List recent CI runs on main
gh run list --branch main --limit 5

# Watch a run in real-time (opens interactive view)
gh run watch

# View details of latest run
gh run view $(gh run list --branch main --limit 1 --json databaseId --jq '.[0].databaseId')

# Get failed test logs
gh run view <run-id> --log-failed

# Check job status for latest run
gh run view --json jobs,conclusion $(gh run list --branch main --limit 1 --json databaseId --jq '.[0].databaseId') \
  --jq '{conclusion: .conclusion, jobs: [.jobs[] | {name: .name, conclusion: .conclusion, status: .status}]}'

# List runs with status filtering
gh run list --branch main --status failure --limit 5
gh run list --branch main --status success --limit 5
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

## GitHub Copilot Delegation

The project includes a delegation library for working with GitHub Copilot coding agents.

### How to Assign Issues to Copilot

**Important:** Copilot assignment requires the GitHub web UI. API-based assignment does not work.

1. Create issue via CLI: `gh issue create --title "..." --body "..." --label copilot`
2. Open issue in GitHub web UI
3. Click "Assignees" → search for "Copilot" → assign
4. Copilot coding agent picks up the issue and creates a branch
5. Agent makes changes and creates a PR

### CLI Commands

```bash
# Check status of all delegated work
petri-pilot delegate status

# Wait for all Copilot agents to complete
petri-pilot delegate wait
```

### Creating Issues for Copilot

Use `gh` CLI to create well-structured issues:

```bash
gh issue create \
  --title "Implement feature X" \
  --label "copilot" \
  --body "$(cat <<'EOF'
## Summary
Description of what needs to be done.

## Implementation
- Step 1
- Step 2

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2
EOF
)"
```

Then assign to Copilot via the GitHub UI.

### Package Structure

- `pkg/delegate/client.go` - GitHub API client for status checking
- `pkg/delegate/batch.go` - Batch task utilities
- `cmd/petri-pilot/delegate.go` - CLI command implementations

### Environment

Requires `GITHUB_TOKEN` environment variable for status commands.

```bash
export GITHUB_TOKEN=$(gh auth token)
```
