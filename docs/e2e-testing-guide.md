# E2E Testing Guide

This guide explains how to write end-to-end tests for generated Petri-pilot applications using the provided test harness and templates.

## Overview

Generated applications include test infrastructure for:
- **API Testing** - Testing backend endpoints with Vitest
- **E2E Testing** - Testing full user flows with Playwright
- **Test Harness** - Helper utilities for test setup and assertions

## Quick Start

```bash
# Navigate to generated app
cd generated/order-processing

# Install dependencies
npm install

# Install Playwright browsers (first time only)
npx playwright install

# Run API tests
npm test

# Run E2E tests
npm run test:e2e

# Run tests in headed mode (see browser)
HEADLESS=false npm run test:e2e

# Run tests with debug output
DEBUG=true npm test
```

## Test Structure

Each generated app includes:

```
generated/<app-name>/
├── e2e/
│   ├── api.test.ts          # Backend API tests
│   ├── app.test.ts          # Frontend E2E tests
│   ├── playwright.config.ts  # Playwright configuration
│   └── .auth/               # Authentication state (generated)
```

## API Testing

API tests use Vitest to test backend endpoints directly.

### Template: api.test.template.ts

Located at `e2e/api.test.template.ts`, this template provides the structure for API tests.

### Basic Pattern

```typescript
import { describe, it, expect, beforeAll } from 'vitest';

const BASE_URL = process.env.API_URL || 'http://localhost:8080';

describe('Order Processing API', () => {
  let aggregateId: string;

  describe('POST /api/order-processing', () => {
    it('creates a new aggregate', async () => {
      const res = await fetch(`${BASE_URL}/api/order-processing`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
      });

      expect(res.status).toBe(201);
      const data = await res.json();
      expect(data.aggregate_id).toBeDefined();
      expect(data.enabled_transitions).toContain('validate');

      aggregateId = data.aggregate_id;
    });
  });

  describe('GET /api/order-processing/{id}', () => {
    it('returns aggregate state', async () => {
      const res = await fetch(`${BASE_URL}/api/order-processing/${aggregateId}`);

      expect(res.status).toBe(200);
      const data = await res.json();
      expect(data.aggregate_id).toBe(aggregateId);
      expect(data.places).toBeDefined();
    });
  });
});
```

### Testing Transitions

Test each transition endpoint:

```typescript
describe('POST /api/order-processing/validate', () => {
  it('executes validate transition', async () => {
    const res = await fetch(`${BASE_URL}/api/order-processing/validate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: aggregateId,
        data: {
          order_id: 'order-123',
          customer_name: 'Alice',
          total: 100
        }
      })
    });

    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.enabled_transitions).toContain('process_payment');
  });

  it('rejects invalid data', async () => {
    const res = await fetch(`${BASE_URL}/api/order-processing/validate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: aggregateId,
        data: {} // Missing required fields
      })
    });

    expect(res.status).toBe(400);
  });
});
```

### Testing Access Control

Test authentication and authorization:

```typescript
describe('Access Control', () => {
  it('requires authentication for protected transitions', async () => {
    const res = await fetch(`${BASE_URL}/api/order-processing/ship`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: aggregateId })
    });

    expect(res.status).toBe(401);
  });

  it('allows authorized roles', async () => {
    // Login as fulfillment role
    const loginRes = await fetch(`${BASE_URL}/api/debug/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login: 'warehouse-user', roles: ['fulfillment'] })
    });
    const { token } = await loginRes.json();

    const res = await fetch(`${BASE_URL}/api/order-processing/ship`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        aggregate_id: aggregateId,
        data: { tracking_number: 'TRACK-123' }
      })
    });

    expect(res.status).toBe(200);
  });
});
```

## E2E Testing with Playwright

E2E tests use Playwright to test the full application including the frontend.

### Template: app.test.template.ts

Located at `e2e/app.test.template.ts`, this template provides the structure for E2E tests.

### Basic Pattern

```typescript
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.APP_URL || 'http://localhost:8080';

test.describe('Order Processing', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(BASE_URL);
  });

  test('displays navigation menu', async ({ page }) => {
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.getByRole('link', { name: /orders/i })).toBeVisible();
  });

  test('creates new instance', async ({ page }) => {
    await page.getByRole('button', { name: /new order/i }).click();
    await expect(page.locator('[data-testid="aggregate-id"]')).toBeVisible();
  });
});
```

### Testing Workflow Execution

Test complete user flows:

```typescript
test('executes workflow transitions', async ({ page }) => {
  // Create new order
  await page.goto(BASE_URL);
  await page.getByRole('button', { name: /new order/i }).click();

  // Get aggregate ID
  const aggregateId = await page.locator('[data-testid="aggregate-id"]').textContent();

  // Execute validate transition
  await page.getByRole('button', { name: /validate/i }).click();
  
  // Fill validation form
  await page.fill('[name="order_id"]', 'order-123');
  await page.fill('[name="customer_name"]', 'Alice');
  await page.fill('[name="total"]', '100');
  await page.getByRole('button', { name: /submit/i }).click();

  // Verify state changed
  await expect(page.locator('[data-state="validated"]')).toBeVisible();
  await expect(page.getByRole('button', { name: /process payment/i })).toBeEnabled();
});
```

### Testing with Authentication

Use Playwright's authentication features:

```typescript
test.describe('with authentication', () => {
  test.use({
    storageState: 'e2e/.auth/user.json'
  });

  test('shows admin navigation', async ({ page }) => {
    await page.goto(BASE_URL);
    await expect(page.getByRole('link', { name: /admin/i })).toBeVisible();
  });
});
```

### Setup Authentication State

Create authentication state file:

```typescript
// e2e/auth.setup.ts
import { test as setup } from '@playwright/test';

setup('authenticate', async ({ page, request }) => {
  // Login via API
  const response = await request.post('/api/debug/login', {
    data: { login: 'test-user', roles: ['admin', 'fulfillment'] }
  });
  const { token } = await response.json();

  // Set token in browser storage
  await page.goto('/');
  await page.evaluate(token => {
    localStorage.setItem('auth', JSON.stringify({ token }));
  }, token);

  // Save authentication state
  await page.context().storageState({ path: 'e2e/.auth/user.json' });
});
```

## Test Harness (Legacy)

The test harness provides utilities for integration tests. Note: This is from an earlier iteration and may be deprecated in favor of Playwright.

### TestHarness Class

Located at `e2e/lib/test-harness.js`.

```javascript
const { TestHarness } = require('./lib/test-harness');

describe('Order Processing', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('order-processing');
    await harness.setup();
  });

  afterAll(async () => {
    await harness.teardown();
  });

  test('creates instance', async () => {
    const instance = await harness.createInstance({});
    expect(instance.aggregate_id).toBeDefined();
  });
});
```

### Test Harness API

#### Setup and Teardown

```javascript
// Create harness
const harness = new TestHarness('order-processing', options);

// Setup (starts server, browser)
await harness.setup();

// Teardown (stops everything)
await harness.teardown();
```

#### Instance Management

```javascript
// Create instance
const instance = await harness.createInstance({ data });

// Get instance state
const state = await harness.getInstance(instanceId);

// Execute transition
const result = await harness.executeTransition(
  'validate',
  aggregateId,
  { order_id: 'order-123', total: 100 }
);
```

#### Browser Interaction

```javascript
// Execute JavaScript in browser
const result = await harness.eval('window.location.pathname');

// Navigate
await harness.navigate('/orders/123');

// Get page state
const pageState = await harness.getPageState();
```

#### Authentication

```javascript
// Login (gets token and sets in browser)
const auth = await harness.login(['admin', 'fulfillment']);

// Auth token available for API calls
const token = harness.authToken;
```

#### Direct API Calls

```javascript
// Make API call
const data = await harness.apiCall(
  'POST',
  '/api/order-processing/validate',
  { aggregate_id, data: { ... } },
  { 'Authorization': `Bearer ${token}` }
);
```

## Custom Jest Matchers

Located at `e2e/jest.setup.js`, these matchers simplify test assertions.

### toHaveTokenIn

Check if state has a token in a specific place:

```javascript
expect(state).toHaveTokenIn('validated');
expect(state).not.toHaveTokenIn('received');
```

Implementation:

```javascript
expect.extend({
  toHaveTokenIn(received, placeName) {
    const places = received.places || received.state || received;
    const hasToken = places[placeName] > 0;

    return {
      pass: hasToken,
      message: () =>
        `expected state ${hasToken ? 'not ' : ''}to have token in "${placeName}"\n` +
        `Received places: ${JSON.stringify(places, null, 2)}`,
    };
  }
});
```

### toHaveTransitionEnabled

Check if a transition is enabled:

```javascript
expect(state).toHaveTransitionEnabled('validate');
expect(state).not.toHaveTransitionEnabled('ship');
```

Implementation:

```javascript
expect.extend({
  toHaveTransitionEnabled(received, transitionId) {
    const enabled = received.enabled || received.enabled_transitions || [];
    const isEnabled = enabled.includes(transitionId);

    return {
      pass: isEnabled,
      message: () =>
        `expected transition "${transitionId}" ${isEnabled ? 'not ' : ''}to be enabled\n` +
        `Enabled transitions: ${JSON.stringify(enabled)}`,
    };
  }
});
```

## Example Test Patterns

### Happy Path Testing

Test the complete successful workflow:

```typescript
test('complete order workflow', async ({ page }) => {
  // Create order
  await page.goto(BASE_URL);
  await page.click('[data-action="create-order"]');
  
  const orderId = await page.locator('[data-testid="order-id"]').textContent();

  // Validate
  await page.click('[data-action="validate"]');
  await page.fill('[name="customer_name"]', 'Alice');
  await page.click('[type="submit"]');
  await expect(page.locator('[data-state="validated"]')).toBeVisible();

  // Process payment
  await page.click('[data-action="process_payment"]');
  await page.fill('[name="payment_method"]', 'credit_card');
  await page.click('[type="submit"]');
  await expect(page.locator('[data-state="paid"]')).toBeVisible();

  // Ship
  await page.click('[data-action="ship"]');
  await page.fill('[name="tracking_number"]', 'TRACK-123');
  await page.click('[type="submit"]');
  await expect(page.locator('[data-state="shipped"]')).toBeVisible();

  // Confirm
  await page.click('[data-action="confirm"]');
  await expect(page.locator('[data-state="completed"]')).toBeVisible();
});
```

### Error Handling

Test validation and error conditions:

```typescript
test('validates required fields', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.click('[data-action="create-order"]');
  await page.click('[data-action="validate"]');
  
  // Submit without filling required fields
  await page.click('[type="submit"]');
  
  // Should show validation errors
  await expect(page.locator('.error-message')).toContainText('required');
});

test('prevents invalid transitions', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.click('[data-action="create-order"]');
  
  // Ship button should be disabled (not yet validated)
  await expect(page.locator('[data-action="ship"]')).toBeDisabled();
});
```

### Guard Condition Testing

Test guard expressions:

```typescript
test('enforces guard conditions', async () => {
  // Attempt transfer with insufficient balance
  const res = await fetch(`${BASE_URL}/api/ledger/transfer`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      aggregate_id: ledgerId,
      data: {
        from: 'alice',
        to: 'bob',
        amount: 1000000 // More than balance
      }
    })
  });

  expect(res.status).toBe(409); // Conflict - guard failed
  const data = await res.json();
  expect(data.error).toContain('guard');
});
```

### Event History Testing

Test event sourcing features:

```typescript
test('displays event history', async ({ page }) => {
  // Create and execute some transitions
  await page.goto(BASE_URL);
  await page.click('[data-action="create-order"]');
  const orderId = await page.locator('[data-testid="order-id"]').textContent();
  
  await page.click('[data-action="validate"]');
  await page.fill('[name="customer_name"]', 'Alice');
  await page.click('[type="submit"]');

  // View events
  await page.goto(`${BASE_URL}/orders/${orderId}/events`);
  
  // Should show event timeline
  await expect(page.locator('[data-testid="event-timeline"]')).toBeVisible();
  await expect(page.getByText('order_validated')).toBeVisible();
});

test('supports time travel', async ({ page }) => {
  const orderId = '...'; // From previous test
  
  // Get current state
  await page.goto(`${BASE_URL}/orders/${orderId}`);
  await expect(page.locator('[data-state="validated"]')).toBeVisible();

  // View state at earlier version
  await page.goto(`${BASE_URL}/orders/${orderId}/at/1`);
  await expect(page.locator('[data-state="received"]')).toBeVisible();
});
```

## Best Practices

### Organize Tests by Feature

```typescript
describe('Order Management', () => {
  describe('Order Creation', () => {
    test('creates order with valid data', ...);
    test('validates required fields', ...);
  });

  describe('Order Validation', () => {
    test('accepts valid orders', ...);
    test('rejects invalid orders', ...);
  });

  describe('Order Shipping', () => {
    test('ships validated orders', ...);
    test('requires tracking number', ...);
  });
});
```

### Use Descriptive Test Names

✅ Good:
```typescript
test('creates order and validates customer email format', ...)
test('prevents shipping before payment is processed', ...)
```

❌ Bad:
```typescript
test('test 1', ...)
test('it works', ...)
```

### Setup and Cleanup

```typescript
describe('Order Processing', () => {
  let orderId: string;

  beforeEach(async () => {
    // Create fresh order for each test
    const res = await fetch(`${BASE_URL}/api/order-processing`, {
      method: 'POST',
      body: JSON.stringify({})
    });
    const data = await res.json();
    orderId = data.aggregate_id;
  });

  afterEach(async () => {
    // Cleanup if needed
  });
});
```

### Use Data-Driven Tests

```typescript
const testCases = [
  { name: 'Alice', email: 'alice@example.com', valid: true },
  { name: 'Bob', email: 'invalid-email', valid: false },
  { name: '', email: 'test@example.com', valid: false },
];

testCases.forEach(({ name, email, valid }) => {
  test(`validates ${name} with email ${email}`, async () => {
    const res = await fetch(`${BASE_URL}/api/order-processing/validate`, {
      method: 'POST',
      body: JSON.stringify({
        aggregate_id: orderId,
        data: { customer_name: name, customer_email: email }
      })
    });

    expect(res.status).toBe(valid ? 200 : 400);
  });
});
```

## Debugging Tests

### Run in Headed Mode

```bash
HEADLESS=false npm run test:e2e
```

### Enable Debug Output

```bash
DEBUG=true npm test
```

### Use Playwright Inspector

```bash
npx playwright test --debug
```

### Take Screenshots

```typescript
test('my test', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.screenshot({ path: 'screenshot.png' });
});
```

### Console Logging

```typescript
test('my test', async ({ page }) => {
  page.on('console', msg => console.log('Browser:', msg.text()));
  await page.goto(BASE_URL);
});
```

## See Also

- [API Test Template](../e2e/api.test.template.ts) - Template for API tests
- [E2E Test Template](../e2e/app.test.template.ts) - Template for E2E tests
- [Test Harness](../e2e/lib/test-harness.js) - Helper utilities
- [Playwright Documentation](https://playwright.dev) - Playwright reference
- [Vitest Documentation](https://vitest.dev) - Vitest reference
