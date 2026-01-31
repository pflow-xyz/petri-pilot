# E2E Test Templates

Templates for generated app testing. Copilot uses these as patterns.

## Structure

Each generated app gets an `e2e/` directory with:

```
e2e/
├── playwright.config.ts   # Playwright configuration
├── api.test.ts            # Backend API tests
├── app.test.ts            # Frontend E2E tests
└── fixtures/
    └── test-data.json     # Test fixtures
```

## Running Tests

```bash
cd generated/<app-name>
npm install
npx playwright install
npm test           # API tests
npm run test:e2e   # Playwright tests
```

## Test Patterns

### API Tests (api.test.ts)

Test each transition endpoint:
1. Create aggregate via POST /api/<model>
2. Execute transitions via POST /api/<model>/<transition>
3. Verify state via GET /api/<model>/{id}
4. Test access control (403 for unauthorized)

### E2E Tests (app.test.ts)

Test user flows:
1. Navigate to app
2. Create new instance
3. Execute transitions via UI
4. Verify state changes reflected in UI
5. Test role-based visibility
