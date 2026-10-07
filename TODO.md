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

### `pflow-engine.js` can't join the pflow-js lock as-is

`frontends/vet-clinic/pflow-engine.js` is *derived* from pflow-xyz's
`petri-solver.js` — a ~1000-line unified runtime that adds discrete
event-sourcing on top of the ODE solver — rather than a copy of it. So it cannot
be pinned the way bitwrap-io, stackedup-gg and modeldao-org pin their vendored
modules (`scripts/pflow-js.sh` + `pflow-js.lock`, and in bitwrap-io's case a
Bazel `git_override` against `@pflow_xyz//public:browser_modules`).

That matters because the derivation has already inherited an upstream bug once.
It summed the per-color token/weight vectors in **both** execution modes — the
mass-action field *and* `canFire` / `_applyEvent` — so a transition whose arc
named red would fire on a pool holding only blue. Fixed in `6bb0a8a` by adding
`petri-colors.js` and unfolding in both paths, but nothing prevents the next
upstream fix from missing this copy the same way.

`frontends/vet-clinic/petri-colors.js` is currently a near-copy of pflow-xyz's,
differing only in a header comment, so it is not byte-identical and therefore not
lockable either.

**To fix:** split `pflow-engine.js` into the part that is genuinely upstream (the
Place/Transition/Arc/PetriNet types, `fromJSON`, `setState`, `setRates`,
`buildODEFunction`, Tsit5, `solve`, `SVGPlotter`) and the part that is this
repo's own (`PflowEngine`, `MemoryEventStore`, the event-sourcing layer). Import
the first from a vendored, locked `petri-solver.js`; keep the second here. Then
add both it and `petri-colors.js` to a `pflow-js.lock`.

Same applies to `pflow-pilot/frontends/shared/pflow-engine.js`, which is
byte-identical to this one — though that repo is archived and a candidate for
retirement.

---

## Future Considerations

- Add more example workflows
- Performance benchmarks for simulation
- Visual workflow editor integration
- Multi-tenant support

## Vet-clinic: retire the legacy in-browser model

The What-If tab runs `services/vet-clinic.json` (café-calibrated: non-kinetic
pickup arcs, read-arc surgery gate, patience, emergency class) through the
server's scenario API. The Simulation/floor-plan tab still runs the older
`frontends/vet-clinic/model.json` through `pflow-engine.js`, which understands
none of those constructs — so the two tabs describe different clinics. Either
teach pflow-engine the shared firing rule (it is an acknowledged independent
runtime, same exposure as the retired five-firing-rules bug) or point the
floor-plan animation at server trajectories and delete the local engine.

## sim.pflow.xyz — simulation-as-a-service (planned fork)

Fork the what-if stack into a new **private** repo backing **sim.pflow.xyz**:
curated operational models (vet clinic, coffee shop, …) offered as a service —
pick a model, turn the knobs, inject disruptions, compare scenarios.

What the fork takes with it:

- The scenario surface (`pkg/runtime/sim`: `/api/scenario`, `/api/scenario/compare`,
  `/api/rates`, SSA metrics/contended/assumptions) — already generator-independent.
- The calibration discipline: two-firing resource seizure, non-kinetic queue→start
  pickup arcs, per-queue patience, read-arc gates, café/vet-clinic fitness gates
  as the template for every hosted model.
- `services/vet-clinic.json` + `services/bundles/cafe-*` as the first two catalog
  entries; the vet-clinic What-If view as the reference console UI.

**Architecture: serverless on Google Cloud.** Unlike the rest of the ecosystem
(long-lived Go services on pflow.dev), sim.pflow.xyz targets GCP serverless:

- **Cloud Run** is the natural fit for the scenario API — the Go handlers in
  `pkg/runtime/sim` are already stateless pure reads (model in, trajectory
  out), so they containerize as-is with no session or store to keep warm.
  Scale-to-zero suits a demo/service site's bursty traffic.
- Model catalog + per-model fitness baselines in **Cloud Storage** (or
  Firestore if catalog metadata gets queried); static consoles from a bucket
  behind **Cloud CDN** or served by the same Cloud Run service.
- Candidate uses for other GCP pieces: Cloud Tasks for long sweep jobs
  (many-realization parameter sweeps beyond request timeout), API Gateway or
  Firebase Auth for tenant quotas.

**Google Sheets as a surface.** Three distinct ideas, in ascending order of
fidelity honesty:

1. *Formulas-only ODE*: a fixed-step Euler/RK4 mass-action solve is genuinely
   expressible as a sheet — rows are time steps, columns are places, each
   transition's rate law is a formula over the previous row, the incidence
   matrix sits in a block. petri-pilot could *generate* this (a "spreadsheet
   form" alongside pflow-polyglot's interpreter/lambda/generated/contract
   forms) via the Sheets API. Honest caveat: this is the continuous engine, so
   everything Forecast refuses (read arcs, inhibitors, non-kinetic arcs —
   i.e. the vet-clinic/café calibrations) is silently unenforceable in cells.
   Fine for predator-prey/enzyme-kinetics-class models; wrong for the
   staffing/disruption models the site is about.
2. *Apps Script SSA*: possible but a dead end — per-event loops in Apps Script
   are slow, RAND() is unseedable (comparisons stop being comparisons), and
   the engine would be a third implementation of the firing rule.
3. *Sheets as a client of the Cloud Run API* (the one to build): custom
   functions / Apps Script call `/api/scenario/compare`, results land in
   ranges, native charts on top. The seed, the firing rule, contended-time
   accounting all stay server-side; the sheet is a console. Pairs naturally
   with the serverless plan — same API, one more surface, and a very natural
   "export this comparison to Sheets" button for the web console.

Open questions for the fork: multi-tenant scenario quotas, model catalog schema
(one JSON per model + fitness gates per model), whether consoles are generated
or hand-built per model, and how much of petri-pilot's codegen comes along vs.
just the runtime. Note the SQLite-only / Makefile conventions in CLAUDE.md are
ecosystem defaults — the serverless fork will need its own deviations documented
(no local SQLite on Cloud Run's ephemeral filesystem beyond caches).

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
