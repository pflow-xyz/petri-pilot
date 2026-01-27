# E2E Test Fixes - Remaining Work

## Completed
- ✅ Fixed access rules not being parsed from v1 JSON models (`parseModelWithExtensions` now copies `Access` to `model.Access`)
- ✅ Admin routes now generate correctly with access control middleware
- ✅ test-access.test.js now passes
- ✅ Admin tests (8/8) now pass

## Current Status
- **192 passed, 48 failed** (4 failing test suites)

## Remaining Failures

### 1. auth.test.js - Returns 400 instead of 401
**Issue:** Protected transition returns 400 (Bad Request) instead of 401 (Unauthorized) when called without auth.

**Root cause investigation needed:**
- The `RequirePermission` middleware should return 401 when no user is in context
- Either the middleware isn't running first, or the route isn't matching correctly
- Need to verify: Is the request hitting the transition handler or falling through to SPA handler?

**Potential fixes:**
- Check if Go ServeMux path parameter matching is working (`/items/{id}/submit`)
- Verify middleware chain order in router.Build()
- May need to move auth check earlier in the chain

### 2. access-control.test.js - Similar auth issues
**Issue:** Related to auth.test.js - access control checks not working as expected

### 3. views.test.js - Unknown failures
**Issue:** Need to investigate specific failures

### 4. coffeeshop-dashboard.test.js - Dashboard-specific issues
**Issue:** Need to investigate specific failures

### 5. concurrency.test.js - Intermittent failures
**Issue:** May be timing/race condition related

## How to Continue

1. Run individual failing tests with verbose output:
   ```bash
   cd e2e && npm test -- --testPathPattern="auth" --verbose
   ```

2. For auth issues, add debug logging to trace middleware execution:
   - Add `fmt.Println` in `RequirePermission` to see if it's being called
   - Check what path is actually being matched

3. Regenerate all examples after fixing:
   ```bash
   for f in examples/*.json; do
     name=$(basename "$f" .json | tr '-' '')
     ./petri-pilot codegen -frontend -submodule -o "generated/$name" "$f"
   done
   go build ./generated/...
   ```

4. Run full test suite:
   ```bash
   cd e2e && npm test
   ```

## Files Modified (Uncommitted)
- `cmd/petri-pilot/main.go` - Added access rule parsing in `parseModelWithExtensions`
- `pkg/codegen/golang/generator.go` - Added `GenerateFromApp` method
- `pkg/codegen/esmodules/generator.go` - Added `GenerateFromApp` method
- `e2e/tests/admin.test.js` - Fixed selectors
- Various regenerated files in `generated/`
