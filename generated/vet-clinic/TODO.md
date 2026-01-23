# TODO

## Known Issues

### e2e_eval session mismatch

**Problem:** When using `e2e_eval` MCP tool, there's a session mismatch between the MCP-created WebSocket session and the browser's JavaScript WebSocket session.

**Details:**
- `e2e_start_browser` creates a WebSocket connection and gets assigned `session-1`
- The browser's `debug.js` also connects to `/ws` and gets assigned `session-2`
- `e2e_eval` sends commands to `session-1`, but the browser is listening on `session-2`
- Result: eval commands return `{"type": "undefined"}` because they go to the wrong session

**Workaround:** Use `curl` or direct API calls for testing instead of `e2e_eval`.

**Potential fixes:**
1. Have `e2e_start_browser` wait for the browser's debug.js to connect and use that session ID
2. Have the browser's debug.js reuse a session ID passed via URL parameter from the MCP tool
3. Modify the MCP tool to not create its own WebSocket session, only track the browser's session
