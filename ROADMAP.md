# Example Improvements Roadmap

Each example needs a custom frontend that showcases its unique workflow, similar to how a real app would use the generated backend.

## Custom Frontend Development

| Example | Custom UI Needed | Status |
|---------|-----------------|--------|
| tic-tac-toe | Game board, turn indicator, win display | ✅ |
| coffeeshop | Order queue, drink cards, barista view | ⬜ |
| erc20-token | Wallet balances, transfer form, tx history | ⬜ |
| order-processing | Order timeline, fulfillment dashboard | ⬜ |
| blog-post | Editor, approval flow, preview | ⬜ |
| support-ticket | Ticket thread, escalation view | ⬜ |

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

| Example | pilot.pflow.xyz | Blog Post |
|---------|-----------------|-----------|
| tic-tac-toe | ✅ | ✅ |
| coffeeshop | ✅ | ⬜ |
| erc20-token | ⬜ | ⬜ |
| order-processing | ⬜ | ⬜ |

## Cleanup

Consolidate redundant examples:
- [ ] `order-system.json` → merge into `order-processing.json`
- [ ] `task-manager.json` + `task-manager-app.json` → pick one
- [ ] `tic-tac-toe-v2.json` → merge into `tic-tac-toe.json`
- [ ] `token-ledger.json` → merge into `erc20-token.json`
- [ ] `test-access.json` → move to tests or delete
