# generated/

Output of the **current** generators, committed so it is compiled and tested.

This tree exists to exercise code generation end-to-end. In-memory hash checks
prove the generator is deterministic and stable, but they cannot prove the code
it emits builds, links and runs — and several defects were only ever going to
show up at `go build`. Everything here is regenerated from a model in
`services/`, so it is disposable by design.

## generated/ vs examples/

|  | `examples/` | `generated/` |
|---|---|---|
| Role | frozen reference apps, served in production | current generator output, exercised by CI |
| Reproducible from `services/`? | no — predates several template changes | yes, and a test enforces it |
| Safe to regenerate? | needs a deliberate decision | yes, that is the point |
| On drift | `TestCommittedAppsDivergeFromGenerator` reports, does not fail | the freeze test fails |

`examples/` is history: those apps are byte-for-byte what shipped, and
regenerating them today produces a diff for reasons that have nothing to do with
any change under test. `generated/` is the opposite contract — if it drifts, the
generator changed and the tree should be refreshed.

## Apps

### warehouse

```bash
petri-pilot codegen services/bundles/warehouse.bundle.json -o generated/warehouse
```

A two-entity composition that uses **two link kinds at once**, which no app in
`examples/` does:

- an **event link** fusing `order.place_order` with `inventory.reserve_stock`,
  so placing an order reserves stock atomically or not at all;
- a **guard link** gating `order.ship` on `inventory.reserved > 0` — a
  cross-subnet precondition that is checked but consumes nothing.

The guard link is the interesting half, and it found a gap. `crosslink_test.go`
records what actually happens today:

| Stage | Guard link survives? |
|---|---|
| `Flatten` lowers it to `tokens("inventory/reserved") > 0` | yes |
| Generator embeds it in `flatmodel.go` | yes |
| Coordinator in `app.go` consults the flat model | only for *fused* transitions, and `ship` is not fused |
| Order entity's aggregate enforces it | **no** — it replays only its own log |

So `ship` currently succeeds with no stock reserved. The model says otherwise.
`TestCrossEntityGuardIsNotEnforcedOnTheEntityPath` asserts the *current*
behaviour deliberately, so the gap stays visible; when it is closed that test
fails and should be inverted.

## Regenerating

```bash
make build
./petri-pilot codegen services/bundles/warehouse.bundle.json -o generated/warehouse
go test ./generated/...
```
