# TODO

## Completed

### Schema Redesign: Events First ✅

Implemented in commits `afce0d0` and `40668e9`.

- Events are first-class schema citizens defining the complete data contract
- Bindings define operational data for state computation (arcnet pattern)
- Views validate field bindings against event fields
- Backward compatible with models that don't define explicit events

### MCP Tools ✅

- **petri_extend** - Modify models with operations (add/remove places, transitions, arcs, roles, events, bindings)
- **petri_preview** - Preview a specific generated file without full codegen
- **petri_diff** - Compare two models structurally
- **petri_simulate** - Fire transitions and see state changes without codegen (PR #32)

### MCP Prompts ✅

Implemented in PR #31.

- **design-workflow** - Guide through designing a new Petri net workflow
- **add-access-control** - Guide through adding roles and permissions
- **add-views** - Guide through creating views for data display

### E2E Testing ✅

Full test coverage implemented:

- **events.test.js** - Event field validation and binding tests (PR #33)
- **access-control.test.js** - Role-based access control tests (PR #34)
- **views.test.js** - View data projection tests (PR #35)
- **admin.test.js** - Admin dashboard tests (PR #36)
- **concurrency.test.js** - Concurrent access and event ordering (PR #37)
- **errors.test.js** - Error handling and validation (PR #38)

Test harness enhancements:
- `login()` accepts string or array of roles
- `fireTransition()` convenience method with error handling
- `getState()` direct API aggregate state retrieval
- `getView()` view data projection
- `getEventHistory()` API-based with sequence numbers
- `restartServer()` for recovery testing

### CI Matrix Strategy ✅

Parallel e2e test execution with 5 test groups:
- app-tests-1: blog-post, ecommerce-checkout, job-application
- app-tests-2: loan-application, order-processing, support-ticket
- app-tests-3: task-manager, workflow
- feature-tests-1: access-control, admin, auth
- feature-tests-2: concurrency, errors, events, views

### Documentation ✅

- Events First schema examples (PR #30)
- Binding patterns documentation (arcnet style)
- GitHub Actions monitoring commands in CLAUDE.md

---

## Success Metrics

- [x] LLM can design complete workflow using prompts alone
- [x] All example models pass simulation without codegen
- [x] E2E test coverage for generated app features
- [x] CI runs e2e suite in parallel
- [ ] Zero flaky tests (monitoring)

---

### MCP Headless Browser Testing (e2e_* tools) ✅

MCP tools for headless browser testing of generated apps. Allows LLM to run E2E tests via eval commands.

**Implemented:**
- `e2e_start_browser` - Launches headless Chrome via chromedp
- `e2e_list_sessions` - Lists active browser sessions
- `e2e_eval` - Evaluates JavaScript in browser via debug WebSocket
- `e2e_stop_browser` - Closes browser session
- `e2e_screenshot` - Captures browser screenshot

**Fixed issues:**
- WebSocket sessions now stay connected (fixed by using `context.Background()` for browser allocator)
- Port configuration via `E2E_PORT` environment variable

**Usage notes:**
- Eval code must use `return await` prefix for async functions
  - Example: `return await pilot.loginAs(['admin'])` not `loginAs(['admin'])`
- Token amounts should be small (1-9) due to 18-decimal scaling (int64 overflow with large amounts)
- Set `PUPPETEER_EXECUTABLE_PATH` for custom Chrome location

**Files:**
- `pkg/mcp/e2e.go` - E2E manager and tool handlers
- `pkg/mcp/e2e_test.go` - Tests
- `pkg/mcp/server.go` - Tool registration

---

## Known Issues

None currently tracked.

---

## Future Considerations

- Add more example workflows
- Performance benchmarks for simulation
- Visual workflow editor integration
- Multi-tenant support
