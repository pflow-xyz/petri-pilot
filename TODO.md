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

### E2E Testing

Browser testing uses Jest + Puppeteer. See `e2e/` directory for test examples.

```bash
cd e2e
npm install     # First time only
npm test        # Run all tests
npm run test:headed  # Watch tests in browser
```

---

## Known Issues

None currently tracked.

---

## Future Considerations

- Add more example workflows
- Performance benchmarks for simulation
- Visual workflow editor integration
- Multi-tenant support

---

## ZK Tic-Tac-Toe Integration

The ZK-enabled tic-tac-toe service is deployed and working on pflow.dev:

- **Base frontend**: https://pilot.pflow.xyz/zk-tic-tac-toe/
- **ZK endpoints**: https://pilot.pflow.xyz/zk-tic-tac-toe/zk/

### Completed

- [x] ZK circuits (MoveCircuit, WinCircuit) with gnark
- [x] Game state tracking with MiMC state roots
- [x] HTTP integration layer (`zk-tictactoe/integration.go`)
- [x] Service wrapper combining base tic-tac-toe with ZK endpoints
- [x] Production deployment on pilot.pflow.xyz

### Circuit Stats (Groth16 on BN254)

| Circuit | Constraints | Public Inputs | Private Inputs |
|---------|-------------|---------------|----------------|
| Move    | 6,012       | 4             | 10             |
| Win     | 3,036       | 2             | 9              |

### Phase 1: Frontend ZK Integration ✅

Update the tic-tac-toe frontend to use ZK endpoints and display proof information.

- [x] Create ZK-aware game client in frontend
  - [x] Call `POST /zk/game` to create games
  - [x] Call `POST /zk/game/{id}/move` for moves
  - [x] Call `POST /zk/game/{id}/check-win` after potential winning moves

- [x] Display ZK state in UI
  - [x] Show current state root (truncated hash)
  - [x] Show state root history (breadcrumb trail)
  - [x] Indicate proof verification status per move

- [x] Add proof details panel
  - [x] Show proof hex (collapsible)
  - [x] Show public inputs
  - [x] Show circuit used (move/win)

### Phase 2: Proof Export & Verification ✅

Enable users to export proofs for on-chain or off-chain verification.

- [x] Add "Export Proof" button to UI
  - [x] Export as JSON (proof + public inputs + Solidity-compatible A/B/C points)
  - [x] Export as calldata for Solidity verifier

- [x] Generate Solidity verifier contracts
  - [x] Move verifier contract (`GET /zk/verifier/move`)
  - [x] Win verifier contract (`GET /zk/verifier/win`)
  - [ ] Deploy to testnet (Sepolia)

- [x] Add verification endpoint
  - [x] `POST /zk/verify` - delegates to prover service

### Phase 3: On-Chain Game State

Enable fully on-chain ZK tic-tac-toe games.

- [ ] Smart contract for game state
  - [ ] Store state root on-chain
  - [ ] Verify move proofs before state transitions
  - [ ] Verify win proofs to determine winner

- [ ] Frontend integration
  - [ ] Connect wallet (wagmi/viem)
  - [ ] Submit moves as transactions
  - [ ] Display on-chain state

- [ ] Gas optimization
  - [ ] Batch proof verification
  - [ ] State compression

### Phase 4: Advanced Features

- [x] Replay verification - verify entire game history (`POST /zk/replay`)
- [ ] Tournament mode with prize pool
- [ ] Spectator mode with live proof streaming
- [ ] Mobile-optimized UI

### ZK API Reference

```
GET  /zk/health              - Health check, lists circuits
POST /zk/game                - Create new ZK game
GET  /zk/game/{id}           - Get game state with roots
POST /zk/game/{id}/move      - Make move, returns proof
POST /zk/game/{id}/check-win - Check winner, returns proof
GET  /zk/circuits            - List available circuits
POST /zk/verify              - Verify a proof
GET  /zk/verifier/{circuit}  - Download Solidity verifier contract
POST /zk/replay              - Verify entire game history (state chain)
```

### Example Move Response

```json
{
  "success": true,
  "position": 4,
  "player": 1,
  "pre_state_root": "5703935289983219918...",
  "post_state_root": "2441967026828943748...",
  "board": [0, 0, 0, 0, 1, 0, 0, 0, 0],
  "turn_count": 1,
  "is_over": false,
  "proof": {
    "circuit": "move",
    "proof_hex": "e3ef7d261dad6dbf...",
    "public_inputs": [
      "0x0c9c501e9b7739eb...",
      "0x05661ab7282a768b...",
      "0x00000000...0000",
      "0x00000000...0001"
    ],
    "verified": true
  }
}
```

### ZK Files

| File | Description |
|------|-------------|
| `zk-tictactoe/circuits.go` | MoveCircuit and WinCircuit definitions |
| `zk-tictactoe/state.go` | BoardState, MiMC hashing |
| `zk-tictactoe/game.go` | Game struct, move/win witnesses |
| `zk-tictactoe/service.go` | Prover service, witness factory |
| `zk-tictactoe/integration.go` | HTTP endpoints |
| `zk-tictactoe/zkservice.go` | Service wrapper for registration |
| `frontends/tic-tac-toe/zk.js` | Frontend ZK client module |

---

## Entity-Based Code Generation ✅

Completed in commit `add4864`.

### Implementation

#### 1. EntityFieldContext and EventDataContext (context.go)
- Added `EntityFieldContext` struct for entity domain fields
- Added `EventDataContext` and `EventDataFieldContext` structs for typed event data
- Added `buildEntityFieldContexts()` function to extract fields from entities extension
- Added `buildEventDataContexts()` function to create typed event data for transitions
- Added `EventDataForTransition()` and `HasEventData()` helper methods
- Added `EventData *EventDataContext` field to `TransitionContext`
- Updated `NewContextFromApp()` to populate EventData on transitions

#### 2. aggregate.tmpl Updates
- State struct now includes EntityFields from entity definitions
- Added EventData struct generation after NewState() function:
  ```go
  // SaveBookmarkData holds the input data for the save_bookmark transition.
  type SaveBookmarkData struct {
      Url   string `json:"url"`
      Title string `json:"title,omitempty"`
      Tags  string `json:"tags,omitempty"`
  }
  ```
- Updated apply functions to use typed EventData when available:
  ```go
  func applySaveBookmark(state *State, event *eventsource.Event) error {
      var data SaveBookmarkData
      if err := json.Unmarshal(event.Data, &data); err != nil {
          return fmt.Errorf("unmarshaling event data: %w", err)
      }
      state.Url = data.Url
      state.Title = data.Title
      state.Tags = data.Tags
      return nil
  }
  ```

#### 3. Verified Working ✅
- All petri-pilot tests pass
- All pflow-pilot tests pass
- Generated bookmark-manager app compiles and includes:
  - State struct with entity fields (Url, Title, Tags, Notes)
  - Typed EventData structs (SaveBookmarkData, EditBookmarkData, DeleteBookmarkData)
  - Apply functions that unmarshal into typed structs and copy to state

### API Enhancements ✅

Completed in commit `218c474`.

- **RESTful API aliases**: POST /api/bookmarks, PUT /api/bookmarks/{id}, DELETE /api/bookmarks/{id}
- **Required field validation**: Apply functions validate required fields before copying to state
- **Typed OpenAPI schemas**: EventData schemas with required markers, entity fields in State schema

### Files Modified
- `pkg/codegen/golang/context.go` - Added EntityFieldContext, EventDataContext, helper methods
- `pkg/codegen/golang/templates/aggregate.tmpl` - Added EventData structs and typed apply functions
- `pkg/codegen/zkgo/generator_test.go` - Updated test counts for tic-tac-toe model
