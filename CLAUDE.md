# Petri-Pilot Development Guide

## Project Overview

Petri-pilot generates complete applications from Petri net models. An LLM designs the model via MCP tools, then deterministic code generation produces Go backends and ES modules frontends.

## Model Formats

Petri-pilot supports two model formats:

### JSON Format
The standard JSON format for Petri net models:
```json
{"name":"order","places":[{"id":"pending"},{"id":"shipped"}],"transitions":[{"id":"ship"}],"arcs":[{"from":"pending","to":"ship"},{"from":"ship","to":"shipped"}]}
```

### Tokenmodel DSL Format (.pflow)
S-expression DSL for more readable model definitions:
```lisp
(schema erc20-token
  (version v1.0.0)

  (states
    (state balances :type map[string]int64 :exported)
    (state total_supply :type int64 :initial 0 :exported))

  (actions
    (action transfer :guard {balances[from] >= amount && amount > 0})
    (action mint :guard {amount > 0}))

  (arcs
    (arc balances -> transfer :keys (from) :value amount)
    (arc transfer -> balances :keys (to) :value amount)
    (arc mint -> balances :keys (to) :value amount)
    (arc mint -> total_supply :value amount)))
```

Both formats are supported by all MCP tools and CLI commands. The DSL format is detected by models starting with `(`.

## Generating New Applications

The primary workflow is: **Design → Validate → Generate → Test → Iterate**

### 1. Design the Model

Start by designing a Petri net model. Use `petri_validate` to check structure:

```
petri_validate(model='{"name":"order","places":[{"id":"pending"},{"id":"shipped"}],"transitions":[{"id":"ship"}],"arcs":[{"from":"pending","to":"ship"},{"from":"ship","to":"shipped"}]}')
```

Or with DSL format:
```
petri_validate(model='(schema order (states (state pending :kind token :initial 1) (state shipped :kind token)) (actions (action ship)) (arcs (arc pending -> ship) (arc ship -> shipped)))')
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

### Browser Testing with Playwright MCP

Use the Playwright MCP tools to interactively test frontends and model viewers in the browser.

#### Testing a Frontend

```
# Navigate to the frontend
mcp__plugin_playwright_playwright__browser_navigate(url="https://pilot.pflow.xyz/knapsack/")

# Take a snapshot to see the page structure (accessibility tree)
mcp__plugin_playwright_playwright__browser_snapshot()

# Take a screenshot to see the visual state
mcp__plugin_playwright_playwright__browser_take_screenshot(type="png", filename="test.png")

# Click on elements using ref from snapshot
mcp__plugin_playwright_playwright__browser_click(ref="e39", element="Item A card")

# Wait for animations or loading
mcp__plugin_playwright_playwright__browser_wait_for(time=2)

# Close browser when done
mcp__plugin_playwright_playwright__browser_close()
```

#### Testing the Petri Net Model Viewer

```
# Open model viewer for a specific model
mcp__plugin_playwright_playwright__browser_navigate(url="https://pilot.pflow.xyz/pflow?model=knapsack")

# Wait for model to load
mcp__plugin_playwright_playwright__browser_wait_for(time=3)

# Run ODE simulation (click play button)
mcp__plugin_playwright_playwright__browser_click(ref="e100", element="Play button")

# Pause simulation
mcp__plugin_playwright_playwright__browser_click(ref="e101", element="Pause button")
```

#### Tips

- **Chrome must be quit first**: If Chrome is already running, Playwright can't launch. Quit Chrome before testing.
- **Use snapshots over screenshots**: Snapshots give you the accessibility tree with refs for clicking elements.
- **Wait for loading**: Use `browser_wait_for(time=N)` after navigation for dynamic content.
- **Element refs change**: After interactions, refs in the snapshot may change. Take a new snapshot after clicks.
- **Local testing**: For local dev server, use `http://localhost:8083/app-name/` URLs.

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

**IMPORTANT:** This project uses a single-module architecture. The `generated/` directory contains subpackages (NOT standalone modules). Never create a `go.mod` file inside `generated/` - all generated apps are part of the main `github.com/pflow-xyz/petri-pilot` module.

When using the CLI to regenerate apps, always use the `-submodule` flag:
```bash
petri-pilot codegen -o generated/myapp -pkg myapp -submodule model.json
```

Note: Flags must come **before** the model file argument (Go's flag package requirement).

Each generated app contains:
- `main.go` - Entry point
- `workflow.go` - Petri net definition
- `aggregate.go` - Event-sourced aggregate
- `api.go` - HTTP handlers
- `events.go` - Event types
- `views.go` - View definitions (if views defined)
- `auth.go`, `middleware.go`, `permissions.go` - Auth (if roles defined)
- `navigation.go` - Navigation (if navigation defined)

### Frontend File Structure

```
frontend/
├── src/              # REGENERATED - core application code
│   ├── main.js       # Entry point, routing
│   ├── admin.js      # Admin dashboard
│   ├── views.js      # Instance views
│   └── ...
└── custom/           # PRESERVED - user customizations (SkipIfExists)
    ├── extensions.js # Hooks, custom actions, renderers
    ├── components.js # Custom web components
    └── theme.css     # Custom styling
```

## Customizing Generated Apps

Generated code supports customization without modifying regenerated files. The `custom/` directory contains files that are generated once and preserved across regeneration.

### Extension Points (`custom/extensions.js`)

Add custom functionality that survives regeneration:

```javascript
// Add custom action buttons to admin
adminExtensions.customActions.push({
  label: 'Export JSON',
  className: 'btn btn-secondary',
  onClick: (instance) => {
    const blob = new Blob([JSON.stringify(instance, null, 2)])
    // ... download logic
  }
})

// React to lifecycle events
registerHook('onInstanceDeleted', (id) => {
  console.log(`Instance ${id} was deleted`)
})

registerHook('onInstanceArchived', (id) => {
  analytics.track('instance_archived', { id })
})

// Custom state renderers
viewExtensions.stateRenderers['balance'] = (value) =>
  `<span class="currency">$${(value/100).toFixed(2)}</span>`
```

### Available Extension Points

| Extension | Purpose |
|-----------|---------|
| `adminExtensions.customActions` | Add buttons to admin instance detail |
| `adminExtensions.customColumns` | Add columns to instance table |
| `viewExtensions.stateRenderers` | Custom rendering for specific places |
| `viewExtensions.customSections` | Add sections to instance view |
| `hooks.onInstanceCreated` | Callback after instance creation |
| `hooks.onInstanceDeleted` | Callback after permanent deletion |
| `hooks.onInstanceArchived` | Callback after soft delete |
| `hooks.onInstanceRestored` | Callback after restore |
| `hooks.onTransitionExecuted` | Callback after transition fires |

### When to Use Each Approach

| Scenario | Approach |
|----------|----------|
| Generic feature for all apps | Modify templates in `pkg/codegen/` |
| App-specific customization | Use `custom/extensions.js` |
| Visual styling | Use `custom/theme.css` |
| Custom UI components | Use `custom/components.js` |

### Workflow for Adding Customizations

1. Generate the app: `petri_codegen(model='...', package='myapp')`
2. Edit `custom/extensions.js` to add your customizations
3. Regenerate when model changes - customizations are preserved
4. For universal features, add to templates instead

See `docs/CUSTOMIZATION_ARCHITECTURE.md` for detailed architecture documentation.

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
- **GitHub @-mentions in commits**: Never write `@main`, `@latest`, or similar `@word` patterns in commit messages or changelogs. GitHub renders these as user mentions (e.g. `@latest` tags a real user). Use backticks (`\`main\``) or write without the `@` prefix.

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

## ZkOde Contracts (Base Sepolia)

ZK-proven ODE state machine contracts deployed on Base Sepolia (2026-02-21).

### Deployed Contracts

| Contract | Address | Purpose |
|----------|---------|---------|
| Groth16Verifier | `0x6dE3168E133ed8EeD8953F97177962B0029D295A` | gnark-generated BN254 pairing verifier |
| Groth16VerifierAdapter | `0xa1be2e71c304aE2b0F9C7947170F71284Db94971` | Adapts fixed-size gnark interface to IVerifier |
| ZkOde | `0x9717edf857352DDC2f0c00c67282889F6d695346` | State commitment manager (enforceOptimal=true) |

- **Network:** Base Sepolia (chain ID 84532)
- **Explorer:** https://sepolia.basescan.org
- **Deployer/Prover:** `0x762593292f543948CA9A9a290adC1770746d059a`
- **Config:** genesis root=0, numTransitions=2, enforceOptimal=true, 5 public inputs

### Circuit Constraint

The current gnark circuit (`zk-ode/circuits.go`) is hard-coded for `NumPlaces=3, NumTransitions=2` (A→B→C linear cascade). A tic-tac-toe circuit with 35 transitions would require new topology constants and a circuit recompile.

### Architecture

```
ZkOde → IVerifier(Groth16VerifierAdapter) → Groth16Verifier (gnark BN254)
```

The adapter translates between IVerifier's structured proof format `(uint256[2] a, uint256[2][2] b, uint256[2] c, uint256[] inputs)` and gnark's flat format `(uint256[8] proof, uint256[5] input)`.

### Key Files

| File | Purpose |
|------|---------|
| `solidity/src/ZkOde.sol` | State commitment manager with optimal play enforcement |
| `solidity/src/IVerifier.sol` | Standard verifier interface |
| `solidity/src/Groth16Verifier.sol` | gnark-generated verifier with BN254 pairing |
| `solidity/src/Groth16VerifierAdapter.sol` | Adapts gnark verifier to IVerifier interface |
| `solidity/src/ZkOdeVerifier.sol` | Stub verifier (testing only) |
| `solidity/script/Deploy.s.sol` | Foundry deploy script (3-contract pattern) |
| `solidity/test/ZkOde.t.sol` | Tests (17 total: stub, optimal play, adapter) |

### Deployment Commands

```bash
# Deploy (requires PRIVATE_KEY, BASE_SEPOLIA_RPC_URL, BASESCAN_API_KEY in env)
cd solidity && PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY \
  forge script script/Deploy.s.sol --rpc-url $BASE_SEPOLIA_RPC_URL --broadcast --verify

# Verify a contract manually
BASESCAN_API_KEY=$BASESCAN_API_KEY forge verify-contract <address> src/ZkOde.sol:ZkOde \
  --chain base-sepolia --constructor-args $(cast abi-encode "constructor(address,uint256,uint256,bool)" <adapter> 0 2 true) --watch

# Query on-chain state
cast call 0x9717edf857352DDC2f0c00c67282889F6d695346 "currentStateRoot()" --rpc-url https://sepolia.base.org
cast call 0x9717edf857352DDC2f0c00c67282889F6d695346 "enforceOptimal()" --rpc-url https://sepolia.base.org
```

## Adding a New Service to Landing Page

To add a new service/app to the petri-pilot landing page and deployment:

### 1. Create the Model

Add your Petri net model JSON file to `examples/`:

```bash
examples/my-app.json
```

### 2. Generate the Service Module

Generate as a submodule (no separate go.mod):

```bash
./petri-pilot codegen -submodule -pkg myapp -o generated/myapp examples/my-app.json
```

### 3. Register the Service

Add an import to `generated/imports.go`:

```go
import (
    // ... existing imports
    _ "github.com/pflow-xyz/petri-pilot/generated/myapp"
)
```

### 4. Create Custom Frontend (Optional)

If you have a custom frontend, add it to `frontends/my-app/`:

```
frontends/my-app/
├── index.html
├── main.js
├── styles.css
└── ...
```

Custom frontends are served instead of generated ones when they exist.

### 5. Add Landing Page Card

Edit `landing/index.html` and add a card in the "Explore Models" section:

```html
<a href="/my-app/" class="demo-card">
  <span class="demo-icon">🎯</span>
  <h3 class="demo-name">My App</h3>
  <p class="demo-desc">Brief description of what this demo shows.</p>
  <div class="demo-meta">
    <span class="demo-tag">tag1</span>
    <span class="demo-tag">tag2</span>
  </div>
</a>
```

### 6. Update Makefile

Add the service to the `dev-run` target in `Makefile`:

```makefile
dev-run: build
    ./$(BINARY) serve -port 8083 tic-tac-toe coffeeshop ... my-app
```

### 7. Build and Test

```bash
go build ./...                    # Verify compilation
make dev-run                      # Test locally
```

### 8. Deploy

```bash
./publish.sh "Add my-app service"
```

## Deployment (pflow.dev)

All services run on pflow.dev behind nginx. Manage with the `~/services` command:

```bash
~/services list      # Show all services and status
~/services start     # Start all services
~/services stop      # Stop all services  
~/services restart   # Restart all services
```

### Service Ports

| Service | Port | URL |
|---------|------|-----|
| pflow-pilot | 8083 | pilot.pflow.xyz |
| pflow-xyz | 8081 | pflow.xyz |
| blog-stackdump | 8082 | blog.stackdump.com |
| modeldao-org | 8084 | modeldao.org |
| stackdump-com | 8085 | console.stackdump.com |

### This Service

```bash
# Check status
ssh pflow.dev "~/services list"

# View logs
ssh pflow.dev "tmux capture-pane -t servers:pflow-pilot -p | tail -50"

# Restart
ssh pflow.dev "~/services restart"

# Attach to tmux
ssh pflow.dev "tmux attach -t servers"
```

### Environment Variables

Environment variables are configured in `~/services`. This service uses:
- `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` - GitHub OAuth
- `GOOGLE_ANALYTICS_ID` - Analytics (G-TFGGN262Z3)
