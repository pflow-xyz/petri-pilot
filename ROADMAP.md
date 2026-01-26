# Example Improvements Roadmap

Improve examples to serve as living documentation. Each example should demonstrate specific patterns and be deployable.

## Current State

| Example | Deployed | Blog Post | Patterns Demonstrated |
|---------|----------|-----------|----------------------|
| tic-tac-toe | ✅ pilot.pflow.xyz | ✅ | Game logic, turn-based, win detection |
| coffeeshop | ✅ pilot.pflow.xyz | ⬜ | Queue management, order flow |
| order-processing | ⬜ | ⬜ | Full-featured reference (roles, views, admin) |
| erc20-token | ⬜ | ⬜ | Token ledger, allowances, safe math |
| blog-post | ⬜ | ⬜ | Content workflow, approval process |
| support-ticket | ⬜ | ⬜ | Ticket lifecycle, escalation |
| job-application | ⬜ | ⬜ | Multi-stage review, RBAC |
| loan-application | ⬜ | ⬜ | Document upload, underwriting |
| ecommerce-checkout | ⬜ | ⬜ | Cart, payment, fulfillment |
| task-manager | ⬜ | ⬜ | Basic CRUD workflow |

## Priority: Deploy More Examples

### Phase 1: Add to pilot.pflow.xyz

1. **erc20-token** - Showcase token/ledger patterns
   - [ ] Test locally
   - [ ] Add to serve command
   - [ ] Deploy

2. **order-processing** - Reference implementation
   - [ ] Test locally
   - [ ] Add to serve command
   - [ ] Deploy

### Phase 2: Blog Integration

Write posts on blog.stackdump.com exploring each example:

- [ ] coffeeshop - Queue theory meets coffee
- [ ] erc20-token - Token standards as Petri nets
- [ ] order-processing - E-commerce state machines

### Phase 3: Cleanup

Remove or consolidate redundant examples:

- [ ] `order-system.json` vs `order-processing.json` - pick one
- [ ] `task-manager.json` vs `task-manager-app.json` - pick one
- [ ] `tic-tac-toe-v2.json` - merge improvements into main
- [ ] `token-ledger.json` vs `erc20-token.json` - consolidate
- [ ] `test-access.json` - move to test fixtures or delete

## Example Quality Checklist

Each example should have:

- [ ] Clear model name and description
- [ ] Meaningful place/transition names
- [ ] At least one role defined
- [ ] At least one view defined
- [ ] Working frontend
- [ ] Passing e2e tests (if applicable)

## Ideas for New Examples

Potential additions to demonstrate more patterns:

- **approval-chain** - Multi-level approval with delegation
- **inventory** - Stock management with reservations
- **booking** - Calendar/scheduling with conflicts
- **auction** - Bidding with time constraints
