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
cd examples/<app-name>
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

## Browser checks for hand-written frontends

The templates above are for *generated* apps. A custom frontend under
`frontends/` is a different problem: nothing generates it, so nothing regenerates
a test for it either.

`frontends/cafe/src/console.browser.mjs` is the pattern. It resolves Playwright
from this directory and serves the app itself:

```bash
make e2e-install     # once: Playwright and its Chromium
make test-browser    # builds, serves on a free port, checks, tears down
```

Pass `BASE=<url>` to check a server that is already running instead. Screenshots
go to `SHOT_DIR` (default: the system temp dir).

**Why it starts its own server.** A check that needs a second terminal is a check
that gets skipped. It picks a free port, waits for the app to answer, and kills
it on any exit — a failed assertion, an exception, a Ctrl-C, or a closed pipe.

**Why it is not in `make test`.** A fresh clone should not fail on a missing
browser. CI runs it as its own `browser` job instead, which is the only place it
runs automatically.

**Why there is only one of it.** There was briefly a second, cheaper check: a DOM
stub that imported the module and asserted the same bindings with no browser.
Every assertion it made is made here by loading the real page, and it was the
weaker implementation — the console fetched an absolute `/api/rates` while the
app is mounted under `/cafe/`, so every request 404'd and the page rendered
empty, and the stub passed the whole time because it was always handed a base URL
and never exercised the default. Its justification was running without a browser
in CI, and it never ran in CI at all.

**What the assertions still cannot do is judge.** This check passed a results
table that listed the barista pool under "Ran out" — a pool reaching zero is
every barista being busy, not the shop running out of staff. That was caught by
looking at the screenshot. Run it, then look at the picture.
