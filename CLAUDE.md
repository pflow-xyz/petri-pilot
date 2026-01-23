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

Start the generated service via MCP or directly:

```bash
# Via MCP tool
service_start(directory='/path/to/generated/app', port=8080)

# Or directly
cd generated/my-app && go build && ./my-app
```

For UI testing, use the e2e test suite (see E2E Tests section below):

```bash
cd e2e && npm run test:headed
```

### 4. Iterate

If something isn't working:

1. Check service logs: `service_logs(service_id='svc-1')`
2. Modify the model using `petri_extend`:
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
service_stop(service_id='svc-1')
```

List running services:

```
service_list()
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

E2E tests use Jest + Puppeteer for browser automation. Run locally:

```bash
cd e2e
npm install        # First time only
npm test           # Run all tests
```

For interactive debugging:

```bash
npm run test:headed    # Watch tests run in browser
npm run test:debug     # Debug mode with visible browser
```

To test a specific app:

```bash
npm run test:app blog-post         # Test single app
npm test -- --testPathPattern="blog-post|task-manager"  # Multiple apps
```

On macOS, set the Chrome path if needed:

```bash
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" npm test
```

### MCP Service Tools

The MCP server provides tools for managing generated services during development.

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
