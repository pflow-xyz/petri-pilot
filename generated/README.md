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

### fulfillment

```bash
petri-pilot codegen services/bundles/fulfillment.bundle.json -o generated/fulfillment
```

Three subnets, and one transition that is **both** fused and guarded — the
shape neither `examples/shop` (fused) nor `warehouse` (guarded) has:

- an **event link** fusing `order.place_order` with `inventory.reserve_stock`;
- a **guard link** gating *that same fused transition* on `credit.cleared > 0`.

`credit` fires nothing. That is the point. The coordinator has to assemble a
marking for an entity that is not a member, decide from it, fire the two that
are, and **fence the non-member in the same atomic append** — an ordering the
`fused+guarded` branch of `bundle_app.tmpl` had always emitted and nothing had
ever run. `crosslink_test.go` exercises it: the command succeeds only when
credit is cleared, appends nothing anywhere on refusal, writes no event to the
entity it merely reads, loses to a writer that moves `credit` between the read
and the append, and leaves all three logs independently replayable (the
`onlyStream` harness from warehouse).

Adding it found one defect: the generated `app_test.go` imported every entity
package, which does not compile when an entity is a read-only participant
rather than a fusion member. `BundleContext.TestEntities` now filters to
members. shop and warehouse are byte-identical across that fix — every entity
in both is also a member, which is exactly why the bug survived.

## Regenerating

```bash
make build
./petri-pilot codegen services/bundles/warehouse.bundle.json -o generated/warehouse
./petri-pilot codegen services/bundles/fulfillment.bundle.json -o generated/fulfillment
go test ./generated/...
```
