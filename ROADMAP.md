# Example Improvements Roadmap

Each example needs a custom frontend that showcases its unique workflow, similar to how a real app would use the generated backend.

## Custom Frontend Development

| Example | Custom UI Needed | Status |
|---------|-----------------|--------|
| tic-tac-toe | Game board, turn indicator, win display | ✅ |
| coffeeshop | Order queue, drink cards, barista view | ✅ |
| erc20-token | Wallet balances, transfer form, tx history | ✅ |
| order-processing | Order timeline, fulfillment dashboard | ✅ |
| blog-post | Editor, approval flow, preview | ✅ |
| support-ticket | Ticket thread, escalation view | ✅ |

### Priority 1: tic-tac-toe

Custom `<game-board>` component:
- 3x3 grid with clickable cells
- X/O rendering based on state
- Turn indicator (whose move)
- Win line highlight
- Game over overlay with result

### Priority 2: coffeeshop

Custom components:
- `<order-queue>` - Visual queue of pending orders
- `<drink-card>` - Order card with drink details
- `<barista-station>` - Drag-drop workflow view

### Priority 3: erc20-token

Custom components:
- `<wallet-balance>` - Token balance display with address
- `<transfer-form>` - Send tokens form
- `<allowance-manager>` - Approve/revoke allowances
- `<tx-history>` - Transaction list

## Deployment

| Example | pilot.pflow.xyz | Dogfooded | Blog Post |
|---------|-----------------|-----------|-----------|
| tic-tac-toe | ✅ | ✅ | ✅ |
| coffeeshop | ✅ | ⬜ | ⬜ |
| erc20-token | ✅ | ⬜ | ⬜ |
| order-processing | ✅ | ⬜ | ⬜ |
| blog-post | ✅ | ⬜ | ⬜ |
| support-ticket | ✅ | ⬜ | ⬜ |

## Dogfooding

Test each example in real usage before writing blog posts.

### Per-example checklist
- [ ] Create multiple instances, exercise full workflow
- [ ] Test error cases (invalid inputs, out-of-order transitions)
- [ ] Check mobile/responsive layout
- [ ] Verify custom dashboard displays state correctly
- [ ] Test admin panel (list, detail, history)
- [ ] Note any UX friction or bugs

### Known issues to investigate
- [x] Custom dashboards not loading for order-processing, blog-post, support-ticket, erc20-token
  - Fixed: Template used `pf-` prefix in element check but components registered without prefix
  - Fix: Updated `main.tmpl` to use `{{.PackageName}}-dashboard` matching component registration
- [ ] erc20-token: "No route found" error when clicking "+ Create New"
  - Generated code looks correct with `/erc20-token/new` routes
  - May be deployment-specific or caching issue - needs testing on production
- [x] coffeeshop: WebSocket connection error
  - Fixed: Debug WebSocket and realtime WebSocket were both at `/ws` causing collision
  - Fix: Moved debug WebSocket to `/ws/debug`, realtime stays at `/ws`
  - Also fixed message type mismatch in coffeeshop frontend (`state` vs `state_update`)
- [x] Missing favicon.ico (404 error on all apps)
  - Fixed: Added inline SVG favicon to `index_html.tmpl` template

### Improvements identified
- [ ] _Add improvement ideas from dogfooding_

## Cleanup

Consolidate redundant examples:
- [x] `order-system.json` → deleted (order-processing.json is the deployed version)
- [x] `task-manager.json` + `task-manager-app.json` → kept both (different formats: Petri net vs Application DSL)
- [x] `tic-tac-toe-v2.json` → deleted (tic-tac-toe.json is the complete version)
- [x] `token-ledger.json` → deleted (erc20-token.json is the deployed version)
- [x] `test-access.json` → kept (has generated code, used for access control testing)
