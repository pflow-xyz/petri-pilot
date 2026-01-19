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

---

## Next Steps

### Phase 1: MCP Prompts

Add guided workflows that help LLMs design models step-by-step.

#### 1.1 Design Workflow Prompt
```
petri://prompts/design-workflow
```

**Implementation:**
```go
// pkg/mcp/prompts.go
func designWorkflowPrompt() mcp.Prompt {
    return mcp.NewPrompt("design-workflow",
        mcp.WithPromptDescription("Guide through designing a new workflow"),
        mcp.WithArgument("description", "What the workflow should do"),
    )
}

func handleDesignWorkflow(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
    description := req.Arguments["description"]

    prompt := fmt.Sprintf(`You are designing a Petri net workflow for: %s

Step 1: Identify States
- What are the possible states an item can be in?
- Which state is the starting state?
- Which states are terminal (end states)?

Step 2: Identify Transitions
- What actions move items between states?
- Who can perform each action?
- Are there any conditions/guards?

Step 3: Define Events
- What data is captured when each transition fires?
- Which fields are required vs optional?
- What types are the fields?

Use petri_create to generate the initial model, then petri_extend to refine it.`, description)

    return &mcp.GetPromptResult{
        Messages: []mcp.PromptMessage{
            {Role: "user", Content: mcp.TextContent{Text: prompt}},
        },
    }, nil
}
```

#### 1.2 Add Access Control Prompt
```
petri://prompts/add-access-control
```

**Guides LLM through:**
- Identifying actors/personas in the workflow
- Creating role definitions
- Mapping transitions to roles
- Adding guard expressions for row-level security

#### 1.3 Add Views Prompt
```
petri://prompts/add-views
```

**Guides LLM through:**
- Identifying data to display per workflow state
- Creating table views for lists
- Creating detail views for single items
- Mapping form actions to transitions

---

### Phase 2: petri_simulate

Fire transitions and see state changes without generating code.

```
petri_simulate(model, steps[]) → simulation result
```

**Implementation:**

```go
// pkg/mcp/simulate.go
type SimulationStep struct {
    Transition string         `json:"transition"`
    Bindings   map[string]any `json:"bindings,omitempty"`
}

type SimulationResult struct {
    Success     bool              `json:"success"`
    FinalState  map[string]int    `json:"final_state"`  // place -> token count
    Steps       []StepResult      `json:"steps"`
    Error       string            `json:"error,omitempty"`
}

type StepResult struct {
    Transition  string         `json:"transition"`
    Enabled     bool           `json:"enabled"`
    StateBefore map[string]int `json:"state_before"`
    StateAfter  map[string]int `json:"state_after"`
    Error       string         `json:"error,omitempty"`
}

func handleSimulate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. Parse model
    // 2. Convert to go-pflow metamodel
    // 3. Execute each step, tracking state
    // 4. Return simulation trace
}
```

**Use cases:**
- Verify workflow reaches terminal state
- Test guard conditions
- Explore branching paths
- Validate model before codegen

---

### Phase 3: E2E Testing Enhancement

Current e2e tests cover basic workflow execution. Enhance to test full app features.

#### 3.1 Current Infrastructure

```
e2e/
├── jest.config.js          # Jest + Puppeteer config
├── jest.setup.js           # Custom matchers (toHaveTokenIn, toHaveTransitionEnabled)
├── lib/
│   ├── test-harness.js     # App lifecycle management
│   ├── server.js           # Go server spawning
│   ├── app-server.js       # App server wrapper
│   └── debug-client.js     # WebSocket debug API client
└── tests/
    ├── workflow.test.js    # Generic workflow tests
    ├── auth.test.js        # Authentication tests
    └── {app}.test.js       # App-specific tests
```

#### 3.2 Test Categories to Add

**Category A: Event & Binding Tests**
```javascript
// e2e/tests/events.test.js
describe('Events First Schema', () => {
    it('should validate required event fields', async () => {
        // Attempt transition without required binding
        const result = await harness.fireTransition('submit', {
            // missing required 'customer_name'
            total: 100
        });
        expect(result.error).toContain('customer_name');
    });

    it('should capture all event fields including optional', async () => {
        await harness.fireTransition('submit', {
            customer_name: 'Alice',
            total: 100,
            memo: 'Rush order'  // optional field
        });

        const events = await harness.getEventHistory();
        expect(events[0].data.memo).toBe('Rush order');
    });

    it('should auto-populate system fields', async () => {
        await harness.fireTransition('submit', { customer_name: 'Bob', total: 50 });

        const events = await harness.getEventHistory();
        expect(events[0].data.timestamp).toBeDefined();
        expect(events[0].data.aggregate_id).toBeDefined();
    });
});
```

**Category B: Access Control Tests**
```javascript
// e2e/tests/access-control.test.js
describe('Role-Based Access Control', () => {
    it('should deny transition to unauthorized role', async () => {
        await harness.login('customer');

        // Customer cannot validate orders (fulfillment role required)
        const result = await harness.fireTransition('validate', {});
        expect(result.error).toContain('forbidden');
    });

    it('should allow inherited role permissions', async () => {
        await harness.login('admin');  // admin inherits from fulfillment

        const result = await harness.fireTransition('validate', {});
        expect(result.success).toBe(true);
    });

    it('should enforce guard expressions', async () => {
        await harness.login('customer');

        // Guard: user.id == order.customer_id
        const result = await harness.fireTransition('cancel', { order_id: 'other-user-order' });
        expect(result.error).toContain('guard');
    });
});
```

**Category C: View & Data Projection Tests**
```javascript
// e2e/tests/views.test.js
describe('Views and Data Projection', () => {
    it('should return correct fields for table view', async () => {
        await harness.createInstance({ customer_name: 'Alice', total: 100 });
        await harness.createInstance({ customer_name: 'Bob', total: 200 });

        const tableData = await harness.getView('order-table');

        expect(tableData.rows).toHaveLength(2);
        expect(tableData.rows[0]).toHaveProperty('customer_name');
        expect(tableData.rows[0]).toHaveProperty('total');
        expect(tableData.rows[0]).not.toHaveProperty('internal_notes');  // not in view
    });

    it('should respect view field bindings from events', async () => {
        await harness.fireTransition('submit', {
            customer_name: 'Alice',
            customer_email: 'alice@example.com',
            total: 100
        });

        const detail = await harness.getView('order-detail');
        expect(detail.customer_name).toBe('Alice');
        expect(detail.customer_email).toBe('alice@example.com');
    });
});
```

**Category D: Admin Dashboard Tests**
```javascript
// e2e/tests/admin.test.js
describe('Admin Dashboard', () => {
    beforeEach(async () => {
        await harness.login('admin');
    });

    it('should list all aggregates', async () => {
        await harness.createInstance({ name: 'Test 1' });
        await harness.createInstance({ name: 'Test 2' });

        const list = await page.goto('/admin');
        const rows = await page.$$('table tbody tr');
        expect(rows.length).toBeGreaterThanOrEqual(2);
    });

    it('should show event history for aggregate', async () => {
        const id = await harness.createInstance({ name: 'Test' });
        await harness.fireTransition('approve', {}, id);

        await page.goto(`/admin/${id}/history`);
        const events = await page.$$('.event-entry');
        expect(events.length).toBe(2);  // create + approve
    });

    it('should allow manual transition firing', async () => {
        const id = await harness.createInstance({ name: 'Test' });

        await page.goto(`/admin/${id}`);
        await page.click('[data-action="approve"]');

        const state = await harness.getState(id);
        expect(state).toHaveTokenIn('approved');
    });
});
```

**Category E: Concurrent Access Tests**
```javascript
// e2e/tests/concurrency.test.js
describe('Concurrent Access', () => {
    it('should handle concurrent transitions safely', async () => {
        const id = await harness.createInstance({ balance: 100 });

        // Two concurrent withdrawals of 60 each (only one should succeed)
        const [result1, result2] = await Promise.all([
            harness.fireTransition('withdraw', { amount: 60 }, id),
            harness.fireTransition('withdraw', { amount: 60 }, id)
        ]);

        const successes = [result1.success, result2.success].filter(Boolean);
        expect(successes.length).toBe(1);  // Only one should succeed

        const state = await harness.getState(id);
        expect(state.balance).toBe(40);  // 100 - 60
    });

    it('should maintain event ordering under load', async () => {
        const id = await harness.createInstance({});

        // Fire 10 transitions rapidly
        const promises = Array(10).fill().map((_, i) =>
            harness.fireTransition('increment', { value: i }, id)
        );
        await Promise.all(promises);

        const events = await harness.getEventHistory(id);
        // Events should have monotonically increasing sequence numbers
        for (let i = 1; i < events.length; i++) {
            expect(events[i].sequence).toBeGreaterThan(events[i-1].sequence);
        }
    });
});
```

**Category F: Error Handling Tests**
```javascript
// e2e/tests/errors.test.js
describe('Error Handling', () => {
    it('should return helpful error for disabled transition', async () => {
        const id = await harness.createInstance({});
        // Try to ship before payment
        const result = await harness.fireTransition('ship', {}, id);

        expect(result.error).toContain('not enabled');
        expect(result.hint).toContain('process_payment');  // suggest next valid transition
    });

    it('should validate binding types', async () => {
        const result = await harness.fireTransition('submit', {
            total: 'not-a-number'  // should be number
        });

        expect(result.error).toContain('total');
        expect(result.error).toContain('number');
    });

    it('should handle server restart gracefully', async () => {
        const id = await harness.createInstance({ name: 'Test' });
        await harness.fireTransition('approve', {}, id);

        await harness.restartServer();

        // State should be recovered from event store
        const state = await harness.getState(id);
        expect(state).toHaveTokenIn('approved');
    });
});
```

#### 3.3 Test Harness Enhancements

```javascript
// e2e/lib/test-harness.js additions

class TestHarness {
    // Existing methods...

    // New methods for enhanced testing
    async getEventHistory(aggregateId) {
        return this.debugClient.call('getEvents', { id: aggregateId });
    }

    async getView(viewId, aggregateId) {
        return this.debugClient.call('getView', { view: viewId, id: aggregateId });
    }

    async login(role) {
        // Set auth context for subsequent requests
        this.authContext = { role };
        return this.debugClient.call('setAuth', { role });
    }

    async restartServer() {
        await this.server.stop();
        await this.server.start();
        await this.waitForHealth();
    }

    async getEnabledTransitions(aggregateId) {
        return this.debugClient.call('getEnabled', { id: aggregateId });
    }
}
```

#### 3.4 CI Integration

```yaml
# .github/workflows/ci.yml additions
e2e-full:
  runs-on: ubuntu-latest
  strategy:
    matrix:
      app: [order-processing, task-manager, loan-application]
      category: [events, access-control, views, admin, concurrency, errors]
  steps:
    - uses: actions/checkout@v4
    - name: Setup
      run: |
        make build
        make generate-${{ matrix.app }}
    - name: Run E2E Tests
      run: |
        npm run test:e2e -- --testPathPattern="${{ matrix.category }}" \
          --app=${{ matrix.app }}
```

---

### Phase 4: Documentation

- Update README with Events First schema examples
- Add binding pattern documentation (arcnet style)
- Document MCP prompts usage
- Add e2e test writing guide

---

## Priority Order

1. **Phase 1.1** - Design Workflow Prompt (enables better model creation)
2. **Phase 2** - petri_simulate (verify models without codegen)
3. **Phase 3.2 A-B** - Event & Access Control tests (core functionality)
4. **Phase 1.2-1.3** - Remaining prompts
5. **Phase 3.2 C-F** - Remaining test categories
6. **Phase 4** - Documentation

---

## Success Metrics

- [ ] LLM can design complete workflow using prompts alone
- [ ] All example models pass simulation without codegen
- [ ] E2E test coverage > 80% of generated app features
- [ ] CI runs full e2e suite in < 10 minutes
- [ ] Zero flaky tests
