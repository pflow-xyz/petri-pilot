# Petri-Pilot Development Guide

## Project Overview

Petri-pilot generates complete applications from Petri net models. An LLM designs the model via MCP tools, then deterministic code generation produces Go backends and ES modules frontends.

## Model Formats

Petri-pilot supports two model formats:

### JSON Format
The standard JSON format for Petri net models:
```json
{"name":"order","places":[{"id":"pending"},{"id":"shipped"}],"transitions":[{"id":"ship"}],"arcs":[{"from":"pending","to":"ship"},{"from":"ship","to":"shipped"}]}
```

### Tokenmodel DSL Format (.pflow)
S-expression DSL for more readable model definitions:
```lisp
(schema erc20-token
  (version v1.0.0)

  (states
    (state balances :type map[string]int64 :exported)
    (state total_supply :type int64 :initial 0 :exported))

  (actions
    (action transfer :guard {balances[from] >= amount && amount > 0})
    (action mint :guard {amount > 0}))

  (arcs
    (arc balances -> transfer :keys (from) :value amount)
    (arc transfer -> balances :keys (to) :value amount)
    (arc mint -> balances :keys (to) :value amount)
    (arc mint -> total_supply :value amount)))
```

Both formats are supported by all MCP tools and CLI commands. The DSL format is detected by models starting with `(`.

## Generating New Applications

The primary workflow is: **Design → Validate → Generate → Test → Iterate**

### 1. Design the Model

Start by designing a Petri net model. Use `petri_validate` to check structure:

```
petri_validate(model='{"name":"order","places":[{"id":"pending"},{"id":"shipped"}],"transitions":[{"id":"ship"}],"arcs":[{"from":"pending","to":"ship"},{"from":"ship","to":"shipped"}]}')
```

Or with DSL format:
```
petri_validate(model='(schema order (states (state pending :kind token :initial 1) (state shipped :kind token)) (actions (action ship)) (arcs (arc pending -> ship) (arc ship -> shipped)))')
```

Use `petri_simulate` to verify workflow behavior before generating code:

```
petri_simulate(model='...', transitions='["ship"]')
```

Use `petri_analyze` for deeper analysis (reachability, deadlocks, liveness).

### 1b. Verify Against Requirements

`petri_validate` says the model is well-formed; `petri_verify` says whether it does
what you asked. State the requirements as properties and get proved / refuted /
unknown back, with a replayable firing sequence on refutation:

```
petri_verify(model='...', properties='["deadlock-free","mutex:busy1,busy2","minted == circulating + burned"]')
```

Each verdict carries a `method`. `structural` means it was proved by linear algebra
on the incidence matrix and therefore holds for **any** initial marking; `exhaustive`
means the full state space of *this* marking was enumerated; `witness` means a finite
constructive witness decided it; `partial` means exploration was truncated, so only
refutations are sound. The report's `ok` is true only when every property was
*proved* — `unknown` is never a pass.

Once real execution data exists, `petri_conformance` closes the last gap — a model
can be deadlock-free, bounded and live while still not describing the actual process:

```
petri_conformance(model='...', log='[{"case":"o1","activity":"validate"},{"case":"o1","activity":"ship"}]')
```

It returns fitness (can the model reproduce observed traces?), precision (does it
permit behavior never seen?), and per-trace diagnostics naming the activities that
could not be replayed, worst-fitting traces first.

**Dependency note:** both tools need the go-pflow `verify` package, which ships
in the pinned release. This repo used to carry a `replace` pointing at a sibling
`../go-pflow` checkout, because it tracked unreleased go-pflow work — the
`metamodel` composition layer and everything after it. That work is now released
as **v0.22.0** and the `require` line names it, so there is one answer to what
you compile against instead of two. Adopting new go-pflow work means landing it
there, tagging, and bumping the `require` line here.

### 2. Generate Code

For a composed application where every entity is its own Petri net (the
entities compile into a metamodel.Bundle — one subnet per entity, cross-entity
FieldReferences validated, declared fusions become atomic cross-entity
commands with coordinators; see pkg/bundle):

```
petri_application(spec='{"name":"shop","entities":[...]}',
                  fusions='[{"id":"order_reserves_stock","members":[{"entity":"order","action":"place_order"},{"entity":"inventory","action":"reserve_stock"}]}]',
                  output_dir='examples/shop')
```

For a raw bundle document (subnets + token/data/event/guard links):

```
petri_bundle(bundle='{"name":"shop","subnets":[{"id":"order","model":{...}}],"links":[...]}', output_dir='...')
```

CLI equivalent — `*.bundle.json` routes to the composed generator
(`model_ref` resolves relative to the document):

```
petri-pilot codegen services/bundles/shop.bundle.json -o examples/shop
```

The generated layout is one Go subpackage per entity (own State, Aggregate,
event log) plus a root package: bundle.go (composition tables), flatmodel.go
(embedded flattened model), app.go (coordinators — each fused transition
fires atomically across member entities via eventsource.MultiAppender, HTTP
POST /fire/<transition>, refusals are 409 and append nothing). examples/shop
is the reference app; cmd/petri-pilot/bundle_freeze_test.go diffs the
generator's output against it byte-for-byte.

For just a backend from a Petri net model:

```
petri_codegen(model='...', language='go', package='ordertracker')
```

For a dependency-free, single-file state-machine core to embed in an existing
codebase — no API, no persistence, just marking/enablement/firing plus a demo
driver (`pkg/codegen/core`, seeded from pflow-polyglot's forms):

```
petri_codegen(model='...', language='rust')                    # also: python, javascript, go-core
petri_codegen(model='...', language='rust', form='contract')   # also: interpreter, lambda, generated (default)
petri_codegen(model='...', language='lean')                    # proof form
```

The four core forms follow pflow-polyglot's FORMS.md: **generated** (arcs
unrolled into straight-line conditionals), **interpreter** (net as runtime
data + one generic engine), **lambda** (pure per-transition functions in a
fixed schedule), **contract** (public entry points that refuse with a reason;
the caller owns sequencing). All print the identical canonical trace under
the same greedy driver.

`language='lean'` is the **proof form**: the generator model-checks the net
at generation time (BFS; refuses unbounded or >4096-state nets) and bakes its
findings — state count, per-place bounds, deadlock set — into Lean 4
`theorem`s discharged by `decide` (or `native_decide` past 128 states). The
Lean kernel re-derives the analyzer's claims by running the same search at
compile time, so a wrong finding is a file that does not compile — two
independent implementations must agree before `main` exists.

Core mode supports token places only (weights, capacities, inhibitor and read
arcs included); models with data places, expression guards, or bindings get a
descriptive error naming each offending element — those need the application
generator. Output is deterministic and gated by execution-parity tests
(`pkg/codegen/core/core_test.go`) that run every form x language cell — 17
programs, lean included when on PATH — and diff each trace against
pflow-polyglot's golden.

For just a frontend:

```
petri_frontend(model='...', api_url='http://localhost:8080')
```

Preview individual files before full generation:

```
petri_preview(model='...', file='api')  # Preview API handlers
petri_preview(model='...', file='workflow')  # Preview Petri net definition
```

### 3. Start and Test the Service

Start the generated service via MCP or directly:

```bash
# Via MCP tool
service_start(directory='/path/to/generated/app', port=8080)

# Or directly
cd generated/my-app && go build && ./my-app
```

For UI testing, use the e2e test suite (see E2E Tests section below):

```bash
cd e2e && npm run test:headed
```

### 4. Iterate

If something isn't working:

1. Check service logs: `service_logs(service_id='svc-1')`
2. Modify the model using `petri_extend`:
   ```
   petri_extend(model='...', operations='[{"op":"add_place","id":"cancelled"},{"op":"add_transition","id":"cancel"},{"op":"add_arc","from":"pending","to":"cancel"},{"op":"add_arc","from":"cancel","to":"cancelled"}]')
   ```
4. Regenerate and restart:
   ```
   service_stop(service_id='svc-1')
   petri_application(spec='...')  # or petri_codegen
   service_start(directory='...', port=8080)
   ```
5. Refresh browser and retest

### 5. Cleanup

When done testing:

```
service_stop(service_id='svc-1')
```

List running services:

```
service_list()
```

## Architecture

```
JSON Model → Schema Parser → Codegen Context → Templates → Generated Code
```

Key packages:
- `pkg/schema/` - Model types (places, transitions, arcs, roles, views)
- `pkg/codegen/golang/` - Go backend generation
- `pkg/codegen/esmodules/` - Frontend generation
- `pkg/runtime/` - Runtime interfaces (EventStore, Aggregate)
- `pkg/mcp/` - MCP server and tools
- `cmd/` - CLI commands

## Code Generation Pattern

1. Parse JSON into `schema.Model`
2. Build `golang.Context` with computed fields (HasViews, HasAdmin, etc.)
3. Execute Go templates from `templates/*.tmpl`
4. Write generated files to output directory

## Adding New Features

When adding schema features:
1. Add types to `pkg/schema/schema.go`
2. Add context fields to `pkg/codegen/golang/context.go`
3. Add `Has*()` helper if conditionally generated
4. Create/update templates in `pkg/codegen/golang/templates/`
5. Update `generator.go` to include new template
6. Update the model files in `services/*.json`
7. Regenerate the affected app and run `go build ./...` to verify

## Template Conventions

- Templates use Go's `text/template`
- Conditional generation: `{{if .HasFeature}}...{{end}}`
- Access context fields directly: `{{.ModelName}}`, `{{.Routes}}`
- Helper methods: `{{.TransitionRequiresAuth "id"}}`
- **Interpolate model strings with `{{printf "%q" .Field}}`, never `"{{.Field}}"`.**
  Guard expressions routinely contain double quotes (`tokens("stock") > 0`), and
  the raw form emitted code that did not compile. `%q` is byte-identical for
  quote-free values, so committed apps are unaffected.
- **Only declare a variable inside the branch that uses it.** `evaluateGuardX`
  declared `state` unconditionally while consuming it only in the collections
  loop, so any guard referencing no collection produced
  `declared and not used: state`.

## Cross-entity commands (bundles)

In a composed app each entity is its own Petri net with its own aggregate and
its own event log, and an aggregate replays **only that log**. So a transition
whose firing rule reaches outside its entity cannot be decided there. Those
transitions become **cross-entity commands**: they live on the composition root
(`App.Fire<Name>`, `POST /fire/<flat transition id>`) and the entity package
refuses them outright.

Three flattened shapes qualify (`buildCommands`, `pkg/codegen/golang/bundle_context.go`):

| Shape | Produced by | Detected via |
|---|---|---|
| fused transition | EventLink | `FlattenMap.FusedGroups` |
| read/inhibitor arc into a non-member entity | GuardLink with a structural condition (`> 0`, `== 0`, …) | the flattened arcs |
| expression guard naming a non-member entity's place | GuardLink with a non-structural condition (`!= n`) | `metamodel.PlaceRefs` on `Transition.Guard` |

**Do not key the command table on `Transition.Guard` alone.** go-pflow lowers
most guard links *structurally* — `> 0` becomes a read arc and `Guard` stays
empty. A table that only read `Guard` saw nothing for warehouse, and
`order.ship` shipped with no stock reserved.

A guard reading only its **own** entity's places stays local: the entity
aggregate already evaluates it, and lifting it would refuse something the
entity can decide alone.

**Refusal is total.** `CanFire` returns false, `EnabledTransitions` omits it,
`Fire`/`Execute` return an error naming the command. Reporting a transition
enabled and then refusing it is an Enabled/Execute divergence, and here it
would be invisible to any single-entity test. `Aggregate.FireComposed` is the
root's only way in.

**What the coordinator does**, in order: load every participating aggregate →
assemble a marking from the places the condition names → evaluate the condition
(`dsl.Evaluate` + `dsl.MakeAggregates`) → `FireComposed` each member → append
member events **and a zero-event read fence per read-only sibling** in one
`MultiAppend`. The fence carries the version the sibling was read at, so a
sibling that moved between the read and the append fails the whole thing.

**Witness.** Each appended event carries `petri.command` and `petri.witness`
metadata: the foreign token counts and the `{stream, version}` of every
participant. Three replay modes follow, and only the first is used at runtime:

- **ReplayPure** (default) — fold an entity's own events into its own state,
  ignoring the witness. This is what keeps one log sufficient to rebuild one
  entity. `generated/warehouse`'s `TestReplayIsPureAfterGuardedCommand` replays
  against a store that errors on every other stream; if it ever needs a sibling,
  the design has failed.
- **ReplayWitness** — re-check a decision from the embedded witness alone, after
  the sibling logs are gone.
- **ReplayAudit** (offline) — replay each sibling truncated to the recorded
  version and confirm the recorded counts. The only mode that catches a witness
  that was never true.

**Generation refuses** rather than emitting a coordinator that decides wrongly:
a cross-entity guard naming an unknown place (reads as zero tokens, so it
decides silently), a **prefix** reference like `sum("inventory")` (names a set;
the coordinator knows one aggregate id per entity), a place wired across subnets
by a TokenLink (no single aggregate owns its count), and a cross-entity arc that
**moves** tokens (would change an entity's state with no event behind it).

The embedded `flatmodel.go` is **documentation**, not the rule — nothing loads
it to decide enablement. Saying otherwise is what hid the gap.

## Guards and the marking

A guard may read its action's parameters *and* the current marking:

```
amount > 0                      # parameters only
tokens("available") > 0         # marking only — what a composed GuardLink lowers to
tokens("stock") >= amount       # both
```

Marking-aware functions are `tokens`, `sum`, `count`, `minOf`, `maxOf`
(`dsl.MakeAggregates`). Note `sum`/`count` match place IDs by **prefix**;
`tokens` matches exactly.

Wiring, all three of which have to line up:

| Layer | Mechanism |
|---|---|
| Runtime | `metamodel.MarkingAggregator`, an optional interface on `GuardEvaluator`; `dsl.Evaluator` implements it |
| Codegen | `GuardContext.UsesMarking` selects the `dsl.MakeAggregates(a.Places())` form in `aggregate.tmpl` |
| Composition | go-pflow's `Bundle.Flatten` lowers a GuardLink to `tokens("<flat place>") <cond>` unless the condition is `== 0`, which becomes an inhibitor arc |

**`Execute` vs `ExecuteWithBindings`.** `Execute` takes no bindings, so it
enforces only the marking-decidable part of a guard; a parameter guard is left
unenforced rather than failing the call. `ExecuteWithBindings` enforces the whole
guard. `Enabled` follows the same rule, so it and `Execute` agree — previously
`Enabled` ignored guards entirely and would offer a transition that `Execute`
then refused. Use `EnabledWithBindings` when parameters decide it.

**There are two Runtimes; use this one.** `go-pflow/tokenmodel` also has a
`Runtime`, and it is not equivalent: its enablement check is hardcoded to
`< 1` (so **arc weights are ignored**), it ignores **inhibitor arcs** entirely,
it moves exactly one token per arc whatever the weight says, and it never
evaluates a **guard**. `petri_simulate` used to run on it and would therefore
report firing sequences the model forbids; it now runs on `pkg/metamodel`
(`pkg/mcp/simulate.go`), and `pkg/mcp/simulate_firingrule_test.go` pins all four
behaviours. Anything that executes a net on behalf of a user belongs on
`pkg/metamodel`.

**Prefer the `== 0` (inhibitor) form when you can.** It is structural, so
`reachability` and `verify` see it. An expression-lowered guard is invisible to
both — go-pflow's `Validate` warns `W_GUARD_OPAQUE` — so it silently weakens
every static claim about the net.

## The firing rule has one home

`metamodel.Enabled`/`Fire`/`Inputs`/`Outputs`/`Tests` (go-pflow
`metamodel/firing.go`) is *the* firing rule. Four rules together:

| | Rule |
|---|---|
| consuming arc | needs `weight` tokens |
| **read arc** | needs `weight` present, consumes none |
| **inhibitor arc** | blocks at `>= weight` |
| **capacity** | a **post-firing** bound, netting out what the same firing consumes — a capacity-2 place holding 2 still admits a consume-1-produce-1 firing. Zero means unbounded, not a bound of zero |

Before it existed, five implementations disagreed. `pkg/mcp/simulate.go` and the
bundle setup search were right; `pkg/runtime/sim`'s SSA **dropped** read and
inhibitor arcs, and `pkg/mcp/stochastic.go` and `buildOdeNet` **consumed** them —
turning a read arc into a drain and an inhibitor into a *source* feeding the
transition it blocks. `buildOdeNet` is shared by twelve tools, so one line
mis-modelled every gated net across the whole analytic surface, silently and in
the same direction each time. Nothing compared them, so nothing failed.

**Never re-derive it from `arc.From`/`arc.To`.** Classify through `Inputs`,
`Outputs` and `Tests`; index-address the result if the inner loop needs speed
(`pkg/runtime/sim.compile` does). `pkg/runtime/sim`'s
`TestSSAAgreesWithTheSharedRule` pins the engine's copy to the definition.

**Capacity ≠ gating unless something can reach it.** `Model.Gating()` reports a
capacity only when a transition raises that place; a bound nothing can breach is
documentation. Otherwise every drain-down model would refuse a forecast it can
answer perfectly well.

## Scenarios: asking a hypothetical

`Forecast`/`Simulate` run forward from a marking something already holds.
**`sim.Run(model, Scenario)`** supplies its own — which is what a decision needs,
since "should I put a third barista on?" is about a shop that does not exist yet
and has no aggregate to point at.

```
POST /api/scenario          {"marking": {"staff/available": 3}, "schedule": {...}, "hours": 8}
POST /api/scenario/compare  {"scenarios": [{"name": "today", ...}, ...]}
GET  /api/rates             rates + initial marking + gating caveats
```

Mounted from `pkg/runtime/sim/http.go`, **not** from a template, so the
single-net and composed generators serve identical handlers. Duplicating them is
how the composed app came to ship with no simulation at all.

- **Marking is a sparse override.** Presence decides, not value: an unnamed place
  keeps what the model declares. An unknown name is a **400**, never a silent
  no-op — a scenario that ignores the knob you set and reports "no difference" is
  the worst possible answer.
- **`schedule`** is piecewise-constant, run segment by segment because SSA draws
  a waiting time from the current total propensity. The last segment holds to the
  horizon rather than reverting to the model's rate.
- **`Compare` forces one seed.** Two SSA runs of the same shop differ; without a
  shared seed a caller cannot tell staffing from dice. That is enforceable only
  server-side, which is why it is one request.
- **`Metrics`** is throughput / mean / P95 / utilization (`<pool>/busy` over
  `busy+available`) — SSA only. A continuous solution has no firings to count.
  **Mean and P95 are time-weighted over the trajectory**, not averages of the
  reported sample points, so they are a property of the run and not of the grid
  `Options.Samples` asked for. Averaging the points gave the empty shop at t=0
  the same weight as a state the run spent real time in: the same scenario read
  74.1% utilization at 8 samples, 81.7% at the default 60 and 82.8% converged,
  always low, and GATE 2 was spending half its 10% band on it. Time-weighted
  integration is the textbook estimator for a continuous-time Markov chain and
  needs no warm-up window to be guessed at; it removes the discretization, not
  the transient, so the residual gap to M/M/c theory is the shop filling up.
  `Series`/`Times` still report the sample grid — this is only about the metrics.
- **Depletion is "below the smallest weight drawn from it"**, not zero: ten beans
  against a weight-20 espresso arc is a shop that has run out.
- **`Contended` is what the run spent its time waiting for**, most of the run
  first: per place, the share of the horizon in which it was the *only* unmet
  input of some transition, and which transitions those were. Depletion cannot
  answer this and was being asked to — it needs a place to empty and stay empty,
  so a resource consumed exactly as fast as it is supplied is invisible to it.
  The café shipped in that state on milk (990 units an hour of demand against
  1000 of supply) and a run reported eight idle baristas losing half the trade,
  an empty `Depleted`, and nothing anywhere naming the cause. Both SSA engines
  report it; `pkg/mcp/stochastic.go` carries the same rule under `contended`.
  **A scheduled run reports it too**, over one ledger merged across every
  segment. It has to be merged rather than assembled from the segments' own:
  a segment's fractions are shares of that segment, so a two-hour rush's 100%
  and a six-hour lull's 0% are not averageable. `simulateScheduled` set every
  other field and not this one, so the café console's Rush box — the only
  control where staffing actually binds — read "waiting on nothing" for a shop
  at 87% utilization.

- **An empty work queue appears in that list too, and `kind` is what keeps it
  from being read as the answer.** "Waiting for customers" and "waiting for milk"
  are the same shape of fact and the opposite finding, and the raw fraction ranks
  them the wrong way round: at three baristas the café's emptiest order queue read
  90% against a staff pool at 26%, so the top four rows of a field documented as
  *what the run was waiting for* said the shop was busy when it was quiet.
  `sim.ClassifySupply` (`pkg/runtime/sim/supply.go`) decides it structurally —
  **conserved** (in a P-invariant *and* initially marked: a fixed pool, so the
  stock it was given is the stock it has — `staff/available`), **bounded**
  (a declared `Capacity`: a shelf with a size — `pantry/*`), or **queue**
  (everything else: unbounded, filled only by the net's own flow). Capacity kinds
  sort ahead of every queue whatever the fractions. The invariant half alone is
  not enough: `available + sum(brewing_X)` is *also* a P-invariant, so membership
  by itself makes "no espresso is brewing" a resource shortage — hence the
  initially-marked half, which is where an operator would add more. The café
  console used to do this with `!place.startsWith(ownSubnetPrefix)`, a naming
  convention that fit one bundle and called every place in a single-net model a
  queue; it now just drops `kind === 'queue'`.

**A resource pool only bites if the resource is held across two firings.**
`start_X` → `brewing_X` → `finish_X`. Seized and released in one firing, a
barista is never observably busy and headcount cannot change the outcome — see
`services/bundles/cafe-*.json`. Each fusion class takes **one** `Link.ID`: it
names the fused transition, so two links joining the same class must agree.

**A prerequisite is not an accelerant.** Mass action multiplies every input into
the rate, which is right for chemistry and wrong for a service system: with the
barista pool in the product, two drinks in progress made *both* finish twice as
fast, and a drink was favoured over its neighbour for using more milk. An arc
marked `"kinetic": false` (go-pflow `Arc.Kinetic`, absent means true, so every
existing model is byte-identical) still gates the firing and is still consumed by
it — it just leaves the product. Both SSA engines honour it (`pkg/runtime/sim`
and `pkg/mcp/stochastic.go`, which are separate copies of the propensity loop;
change them together).

The queue is one of these too, and less obviously. `pending_X -> start_X` kinetic
meant a waiting order was picked up at 60/h *per waiting customer* against
patience of 12/h per waiting customer: the two scaled together and cancelled, so
exactly five orders in six were started at every queue length and every
headcount. One arrival in six walked out with every barista idle, and "how many
baristas do I need to get walkouts under 10%" had no answer — the loss was a
Bernoulli split, not a function of the wait. A barista does not pick orders up
faster because more people are waiting. With that arc non-kinetic the loss is
`patience x mean queue`, which is what staffing moves. The paired change is the
rate: `start_X` is a free barista noticing a waiting order (720/h, about five
seconds), not a second service stage — the drink is `finish_X`. And because
fusion fires a rendezvous at its **slowest** member's rate, the pantry's
`brew_X` has to carry the same number or it silently caps the pickup.

**The fitness gates run the shipped model, with nothing overridden.**
`pkg/runtime/sim/cafe_fitness_test.go` sweeps headcount over the composed café
and asserts six things: service time is invariant to headcount, utilization
matches offered load, the served mix tracks the ordered mix, staffing has a knee
and gets walkouts under a tenth of arrivals somewhere in the sweep, the pantry
does not accelerate, and the shop is limited by its baristas rather than its
stock. That last one exists because the gates originally overrode the three
restock rates to open the pantry up, on the reasoning that a staffing gate should
not be measuring milk — which certified a shop nobody ships, at 224 served and 42
walked out against the shipped model's 190 and 75. The reasoning was right and the
fix was wrong: **size the model so the intended constraint binds, then assert it**,
rather than running a scenario the operator cannot. Every expectation is derived
from `sim.Rates(cafe.FlatModel())`, so a rate change moves what the gates expect
instead of breaking them.

**A method assumption is not an unenforced constraint, and they are separate
fields.** `Result.Caveats` is constraints *this model* expresses that the run
could not enforce, and an empty list is a claim: everything the net says was
applied. `Result.Assumptions` is what the engine assumes whatever the model
says. Every SSA scenario carries one — Gillespie draws every duration from an
exponential distribution, the most erratic a shop can be for a given average, so
work with a predictable duration queues roughly half as much and the waiting and
the walkouts here are the bad case, not the typical one. That note used to be
appended to `Caveats`, which mislabelled it (the café console rendered it under
"Not enforced in this run:") *and* cost the empty-list claim its meaning, since
the list could then never be empty for any scenario. No edit to a net removes an
assumption; only a different engine could. It is appended in `sim.Run` rather
than `Simulate` because it belongs to the answer a scenario gives, and
`Simulate` is also the segment engine behind a schedule. The console has a
heading per list, `pkg/mcp`'s comparison summary has a field per list, and
`pkg/runtime/sim/metrics_test.go` pins them apart.

`Forecast` **refuses** a model carrying read arcs, inhibitors, reachable
capacities, guards or non-kinetic arcs, and says which. The first four are
invisible to a continuous solve; a non-kinetic arc fails the other way — the
solver sees it and cannot help but put it in the rate law. A continuous solve has no firing instant to
test any of them, so it would answer a less constrained question — and a
dashboard plots a wrong smooth curve just as happily as a right one. The twelve
`buildOdeNet` tools carry the same text as a `caveats` field, added only when
there is something to say so unconstrained models are byte-identical.

## Building (Go + Bazel)

petri-pilot builds two ways. **Go tooling and Bazel coexist** — `go.mod`/`go.sum`
stay the source of truth for dependencies; Bazel reads them via Gazelle. Mirrors the
[go-pflow Bazel setup](../go-pflow/CLAUDE.md).

### Go (default for day-to-day dev)

```bash
make build              # go build -> ./petri-pilot
go test ./...           # All tests
```

### Bazel (pure Bzlmod, hermetic, with nogo static analysis)

Bazel is driven by [bazelisk](https://github.com/bazelbuild/bazelisk) (pinned to the
version in `.bazelversion`). If you don't have it: `go install github.com/bazelbuild/bazelisk@latest`
(installs to `$(go env GOPATH)/bin`; symlink/alias it to `bazel`).

```bash
bazel build //...                  # build everything (runs nogo: go vet + x/tools passes)
bazel test //...                   # run all tests
bazel run //cmd/petri-pilot -- help
bazel run //:gazelle               # regenerate BUILD.bazel files after adding/moving Go files
bazel mod tidy                     # sync go_deps use_repo list after editing go.mod
```

Layout:
- `MODULE.bazel` — Bzlmod: `rules_go` + `gazelle`; deps come from `go.mod` via the
  `go_deps` extension. The hermetic Go SDK and the `nogo` target are registered here.
- `.bazelrc` — Bzlmod-only (`--noenable_workspace`); sets `--@io_bazel_rules_go//go/config:tags=purego`.
- `BUILD.bazel` (root) — the `gazelle` target and the `nogo` target (`TOOLS_NOGO` analyzer set).
- Per-package `BUILD.bazel` files are Gazelle-generated; hand-added attributes are marked `# keep`.

**Gotchas / decisions baked in:**
- **rules_go 0.61.1 / Go SDK 1.26.0.** go.mod requires `go 1.25.6`, so the hermetic SDK
  must be ≥ 1.25 (`MODULE.bazel` pins 1.26.0 to share remote-cache keys with the other
  ecosystem consumers). rules_go ≤ 0.55.x hardcodes `GOEXPERIMENT=coverageredesign`, which
  Go 1.25 removed (`go: unknown GOEXPERIMENT coverageredesign`) — hence the newer
  rules_go/gazelle pin than go-pflow uses.
- **gnark-crypto asm built hermetically (F4 Tier 2).** `gnark-crypto`'s amd64/arm64 assembly
  uses relative cross-package `#include` directives that don't resolve in Bazel's sandbox (the
  included `field/asm/element_Nw/*.s` files live in a separate vendoring-hack package — see
  gnark-crypto issue #619). Rather than fall back to pure-Go via `-tags purego`,
  `bazel/patches/gnark-crypto-asm-hermetic.patch` (wired via `go_deps.module_override` in
  `MODULE.bazel`) **inlines** each included file's content directly into the consuming `.s`, so
  rules_go assembles it in-sandbox with no new files or BUILD/srcs changes. Bazel now compiles
  the **same asm field backend** `make build` ships — the hermetic artifact and the shipped
  binary are no longer different builds. Regenerate the patch after a gnark-crypto bump with
  `scripts/gen-gnark-asm-patch.sh` (it covers all 36 consuming files across `ecc/*` and
  `field/{babybear,koalabear}`). The pure-Go path is still available as a fallback via
  `bazel build --config=purego`.
- **Defense in depth — purego↔asm parity.** Independent of the build path, `scripts/zk-parity-check.sh`
  (run in CI; builds `cmd/zk-field-parity` with and without `-tags purego`) asserts the two field
  backends produce identical digests for raw Fp/Fr ops, native MiMC, the compiled R1CS, and a
  solved witness — so an asm/purego divergence can never silently reach the on-chain Groth16
  verifiers.
- **`//pkg/mcp:mcp_test`** runs with `-test.short` (`# keep` in its BUILD.bazel):
  `TestServiceManagerIntegration` shells out to `go build` (needs a Go dev env + GOCACHE →
  non-hermetic) and self-skips under short mode. The other ~25 cases still run.
- **`//pkg/codegen/zkgo:zkgo_test`** reads `../../../services/tic-tac-toe.json` from disk;
  that file is supplied as a `data` runfile (`services` `exports_files` it) so the test is
  hermetic.
- After editing `go.mod`, run `bazel mod tidy`; after adding/moving `.go` files, run
  `bazel run //:gazelle`. Apps under `examples/` and `generated/` are part of the main module
  (single-module architecture), so Gazelle picks them up automatically.
- **Adopting new go-pflow work means a release, not an edit.** With the `replace`
  gone there is no sibling to edit into this build: land the change in go-pflow,
  tag it, and bump the `require` line here. Slower on purpose — it is what makes
  local builds, Bazel and CI all compile the same go-pflow, which a sibling
  checkout at a hand-maintained CI ref never guaranteed.

### Generating Bazel-ready apps (`-bazel`)

The Go app generator can emit Bazel build files alongside the Go (and Solidity) output.
Pass `-bazel` to `codegen`:

```bash
# Submodule (default for in-repo apps): just a per-package BUILD.bazel
petri-pilot codegen -submodule -bazel -pkg myapp -o generated/myapp model.json

# Standalone module: BUILD.bazel + MODULE.bazel + .bazelrc + .bazelversion
petri-pilot codegen -bazel -o /path/to/myapp model.json
```

What it emits (driven by `AsSubmodule`, mirroring how `go.mod` is gated):
- **Submodule** → one `BUILD.bazel` (`go_library` + `go_test`), plus `graph/BUILD.bazel`
  when GraphQL is enabled. The parent module owns `MODULE.bazel`/gazelle/nogo.
- **Standalone** → additionally `MODULE.bazel` (rules_go + gazelle, hermetic SDK,
  `go_deps` from the generated `go.mod`), `.bazelrc`, `.bazelversion`, and a root
  `BUILD.bazel` carrying the `gazelle`/`nogo` targets and a `go_binary`.

Mechanics worth knowing (`pkg/codegen/golang/bazel.go`):
- `srcs` use `glob(["*.go"], exclude=["*_test.go"])` marked **`# keep`** so Gazelle
  doesn't error trying to merge a glob (it would otherwise report
  `could not merge expression`) — Gazelle still resolves/repairs `deps`.
- `deps` are feature-gated to match the imports each enabled template pulls in
  (graph subpkg ↔ GraphQL, `oauth2` ↔ auth, `//pkg/dsl` ↔ guards, etc.). rules_go
  doesn't fail on *unused* deps, only missing ones, so the list errs toward complete.
- The generated submodule `BUILD.bazel` is byte-compatible with what
  `bazel run //:gazelle` produces (only `glob`-vs-explicit `srcs` differs), so the
  two never fight. After regenerating, `bazel build //examples/<app>/...` (or `//generated/<app>/...`) works as-is.

### Checking a hand-written frontend

A custom frontend under `frontends/` is not generated, so nothing regenerates a
test for it. One check, and it drives the real page:

```bash
make e2e-install     # once: Playwright and its Chromium, into e2e/
make test-browser    # builds, serves on a free port, checks, tears down
```

`frontends/cafe/src/console.browser.mjs` starts the app itself, so it is one
command rather than a two-shell dance — pass `BASE=<url>` to point at a server
that is already running. It asserts the page renders, that no request 404s, that
requests land **under the mount prefix**, that every place and transition the
console names exists in the model, that the sliders reach the wire as
`staff/available` and an abandon rate on **every** per-drink queue, that no drink
is served more often than it was ordered, that a comparison shares one seed, and
that nothing scrolls sideways at 390px. CI runs it as its own job (`browser`),
which is the only place it runs automatically — `make test` stays free of a
browser dependency so a fresh clone works.

**There used to be a second, cheaper check** — a DOM stub that imported
`console.js` and asserted the same bindings with no browser. Every assertion it
made is made here by loading the real page, so it was a second implementation of
one definition, and it was the weaker one: the console fetched an absolute
`/api/rates` while the app is mounted under `/cafe/`, every request 404'd, the
page rendered empty, **and the stub passed throughout** — it was always handed a
base URL and so never exercised the default. Its stated advantage was running
without a browser in CI. It never ran in CI at all; CI runs `go test`, not
`make test`. Same lesson as the five firing rules: a second implementation of one
definition is a way to be confidently wrong, not a safety net.

The other half is judgement, which no assertion covers. The browser check
happily passed a results table listing the **barista pool** under "Ran out" — a
pool reaching zero is every barista being busy, not the shop running out of
staff. That was caught by looking at the screenshot the run writes to
`SHOT_DIR`. Run it, then look at the picture.

### E2E Tests

E2E tests use Jest + Puppeteer for browser automation. Run locally:

```bash
cd e2e
npm install        # First time only
npm test           # Run all tests
```

For interactive debugging:

```bash
npm run test:headed    # Watch tests run in browser
npm run test:debug     # Debug mode with visible browser
```

To test a specific app:

```bash
npm run test:app blog-post         # Test single app
npm test -- --testPathPattern="blog-post|task-manager"  # Multiple apps
```

On macOS, set the Chrome path if needed:

```bash
PUPPETEER_EXECUTABLE_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" npm test
```

### Browser Testing with Playwright MCP

Use the Playwright MCP tools to interactively test frontends and model viewers in the browser.

#### Testing a Frontend

```
# Navigate to the frontend
mcp__plugin_playwright_playwright__browser_navigate(url="https://pilot.pflow.xyz/knapsack/")

# Take a snapshot to see the page structure (accessibility tree)
mcp__plugin_playwright_playwright__browser_snapshot()

# Take a screenshot to see the visual state
mcp__plugin_playwright_playwright__browser_take_screenshot(type="png", filename="test.png")

# Click on elements using ref from snapshot
mcp__plugin_playwright_playwright__browser_click(ref="e39", element="Item A card")

# Wait for animations or loading
mcp__plugin_playwright_playwright__browser_wait_for(time=2)

# Close browser when done
mcp__plugin_playwright_playwright__browser_close()
```

#### Testing the Petri Net Model Viewer

```
# Open model viewer for a specific model
mcp__plugin_playwright_playwright__browser_navigate(url="https://pilot.pflow.xyz/pflow?model=knapsack")

# Wait for model to load
mcp__plugin_playwright_playwright__browser_wait_for(time=3)

# Run ODE simulation (click play button)
mcp__plugin_playwright_playwright__browser_click(ref="e100", element="Play button")

# Pause simulation
mcp__plugin_playwright_playwright__browser_click(ref="e101", element="Pause button")
```

#### Tips

- **Chrome must be quit first**: If Chrome is already running, Playwright can't launch. Quit Chrome before testing.
- **Use snapshots over screenshots**: Snapshots give you the accessibility tree with refs for clicking elements.
- **Wait for loading**: Use `browser_wait_for(time=N)` after navigation for dynamic content.
- **Element refs change**: After interactions, refs in the snapshot may change. Take a new snapshot after clicks.
- **Local testing**: For local dev server, use `http://localhost:8083/app-name/` URLs.

### MCP Service Tools

The MCP server provides tools for managing generated services during development.

#### Service Management

```
# Start a generated service
mcp__petri-pilot__service_start(directory="/path/to/generated/service", port=8080)

# Check service health
mcp__petri-pilot__service_health(service_id="svc-1")

# View service logs
mcp__petri-pilot__service_logs(service_id="svc-1", lines=50)

# Stop a service
mcp__petri-pilot__service_stop(service_id="svc-1")

# List all running services
mcp__petri-pilot__service_list()
```

## Monitoring GitHub Actions

Use `gh` CLI to monitor CI runs:

```bash
# List recent CI runs on main
gh run list --branch main --limit 5

# Watch a run in real-time (opens interactive view)
gh run watch

# View details of latest run
gh run view $(gh run list --branch main --limit 1 --json databaseId --jq '.[0].databaseId')

# Get failed test logs
gh run view <run-id> --log-failed

# Check job status for latest run
gh run view --json jobs,conclusion $(gh run list --branch main --limit 1 --json databaseId --jq '.[0].databaseId') \
  --jq '{conclusion: .conclusion, jobs: [.jobs[] | {name: .name, conclusion: .conclusion, status: .status}]}'

# List runs with status filtering
gh run list --branch main --status failure --limit 5
gh run list --branch main --status success --limit 5
```

## Generated File Structure

**IMPORTANT:** This project uses a single-module architecture. `examples/` and `generated/` contain subpackages (NOT standalone modules). Never create a `go.mod` inside either — every app is part of the main `github.com/pflow-xyz/petri-pilot` module.

When using the CLI to regenerate apps, always use the `-submodule` flag:
```bash
petri-pilot codegen -o generated/myapp -pkg myapp -submodule model.json
```

Flags may be written before or after the model file — the CLI reorders arguments
before parsing them.

Each generated app contains:
- `main.go` - Entry point
- `workflow.go` - Petri net definition
- `aggregate.go` - Event-sourced aggregate
- `api.go` - HTTP handlers
- `events.go` - Event types
- `views.go` - View definitions (if views defined)
- `auth.go`, `middleware.go`, `permissions.go` - Auth (if roles defined)
- `navigation.go` - Navigation (if navigation defined)

### Frontend File Structure

```
frontend/
├── src/              # REGENERATED - core application code
│   ├── main.js       # Entry point, routing
│   ├── admin.js      # Admin dashboard
│   ├── views.js      # Instance views
│   └── ...
└── custom/           # PRESERVED - user customizations (SkipIfExists)
    ├── extensions.js # Hooks, custom actions, renderers
    ├── components.js # Custom web components
    └── theme.css     # Custom styling
```

## Customizing Generated Apps

Generated code supports customization without modifying regenerated files. The `custom/` directory contains files that are generated once and preserved across regeneration.

### Extension Points (`custom/extensions.js`)

Add custom functionality that survives regeneration:

```javascript
// Add custom action buttons to admin
adminExtensions.customActions.push({
  label: 'Export JSON',
  className: 'btn btn-secondary',
  onClick: (instance) => {
    const blob = new Blob([JSON.stringify(instance, null, 2)])
    // ... download logic
  }
})

// React to lifecycle events
registerHook('onInstanceDeleted', (id) => {
  console.log(`Instance ${id} was deleted`)
})

registerHook('onInstanceArchived', (id) => {
  analytics.track('instance_archived', { id })
})

// Custom state renderers
viewExtensions.stateRenderers['balance'] = (value) =>
  `<span class="currency">$${(value/100).toFixed(2)}</span>`
```

### Available Extension Points

| Extension | Purpose |
|-----------|---------|
| `adminExtensions.customActions` | Add buttons to admin instance detail |
| `adminExtensions.customColumns` | Add columns to instance table |
| `viewExtensions.stateRenderers` | Custom rendering for specific places |
| `viewExtensions.customSections` | Add sections to instance view |
| `hooks.onInstanceCreated` | Callback after instance creation |
| `hooks.onInstanceDeleted` | Callback after permanent deletion |
| `hooks.onInstanceArchived` | Callback after soft delete |
| `hooks.onInstanceRestored` | Callback after restore |
| `hooks.onTransitionExecuted` | Callback after transition fires |

### When to Use Each Approach

| Scenario | Approach |
|----------|----------|
| Generic feature for all apps | Modify templates in `pkg/codegen/` |
| App-specific customization | Use `custom/extensions.js` |
| Visual styling | Use `custom/theme.css` |
| Custom UI components | Use `custom/components.js` |

### Workflow for Adding Customizations

1. Generate the app: `petri_codegen(model='...', package='myapp')`
2. Edit `custom/extensions.js` to add your customizations
3. Regenerate when model changes - customizations are preserved
4. For universal features, add to templates instead

See `docs/CUSTOMIZATION_ARCHITECTURE.md` for detailed architecture documentation.

## SQLite Only

This project uses SQLite exclusively. Do not add support for other databases.

## No React

Frontend uses vanilla ES modules only. Do not add React, Vue, or other frameworks.

## Import Conventions

Generated code imports from:
- `github.com/pflow-xyz/petri-pilot/pkg/runtime/api`
- `github.com/pflow-xyz/petri-pilot/pkg/runtime/eventstore`
- `github.com/pflow-xyz/petri-pilot/pkg/runtime/aggregate`

## Element IDs → Go identifiers

A flattened bundle model names elements `orders/ship`, `fused:a/t+b/t`,
`wire:b/ready` — none of `/`, `:`, `+` is legal in a Go identifier. Two rules
keep that compilable, both in `pkg/codegen/golang`:

1. **`ToPascalCase` splits on every non-alphanumeric rune** (`naming.go`), not on
   an enumerated `_ - .` set, so a new namespacing separator in go-pflow cannot
   silently emit uncompilable code.
2. **Sanitizing is many-to-one, so every derived identifier is allocated from an
   `identScope`** (`identifiers.go`): `orders/ship_now` and `orders_ship/now`
   both stem to `OrdersShipNow`, and the second gets `OrdersShipNow2`. Scopes are
   per-model and shared by every builder — an arc, a state field and the place
   they name must resolve to the same identifier. Places, transitions and event
   structs have separate scopes; the event scope is keyed by **event type**, not
   transition, because several transitions may share one event struct
   (tic-tac-toe's nine `x_play_*` all emit `XPlayed`).

Never derive an identifier from a raw ID inside a template — take the field the
context already allocated (`ConstName`, `FieldName`, `FuncName`, `HandlerName`,
`EventName`). `{{pascal .ID}}` bypasses the scope.

**Why this is enforced rather than trusted:** `go/format` accepts a const block
that declares the same identifier twice, and `formatGo` was the only validity
gate — so a collision used to *pass* generation and fail later at `go build`
with "redeclared in this block", pointing at generated code instead of at the
model. `checkGeneratedPackage` (`check.go`) now type-checks each generated
package (grouped by directory) after generation. Imports are stubbed, since the
generated module does not exist on disk yet, so only diagnostics decidable from
the files alone — redeclarations — are reported.

## Common Issues

- **Unused imports**: Make imports conditional with `{{if .HasFeature}}`
- **Undefined functions**: Add standalone helpers in template, don't reference non-existent methods
- **Test file count**: Update `generator_test.go` when adding new templates
- **GitHub @-mentions in commits**: Never write `@main`, `@latest`, or similar `@word` patterns in commit messages or changelogs. GitHub renders these as user mentions (e.g. `@latest` tags a real user). Use backticks (`\`main\``) or write without the `@` prefix.

## GitHub Copilot Delegation

The project includes a delegation library for working with GitHub Copilot coding agents.

### How to Assign Issues to Copilot

**Important:** Copilot assignment requires the GitHub web UI. API-based assignment does not work.

1. Create issue via CLI: `gh issue create --title "..." --body "..." --label copilot`
2. Open issue in GitHub web UI
3. Click "Assignees" → search for "Copilot" → assign
4. Copilot coding agent picks up the issue and creates a branch
5. Agent makes changes and creates a PR

### CLI Commands

```bash
# Check status of all delegated work
petri-pilot delegate status

# Wait for all Copilot agents to complete
petri-pilot delegate wait
```

### Creating Issues for Copilot

Use `gh` CLI to create well-structured issues:

```bash
gh issue create \
  --title "Implement feature X" \
  --label "copilot" \
  --body "$(cat <<'EOF'
## Summary
Description of what needs to be done.

## Implementation
- Step 1
- Step 2

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2
EOF
)"
```

Then assign to Copilot via the GitHub UI.

### Package Structure

- `pkg/delegate/client.go` - GitHub API client for status checking
- `pkg/delegate/batch.go` - Batch task utilities
- `cmd/petri-pilot/delegate.go` - CLI command implementations

### Environment

Requires `GITHUB_TOKEN` environment variable for status commands.

```bash
export GITHUB_TOKEN=$(gh auth token)
```

## Verifiable Computation via Petri Net ODE

The `zk-ode/` package implements a general pattern for turning any Petri net into a verifiable computation. The technique is domain-agnostic — tic-tac-toe is one application, but the same pipeline applies to any system expressible as a Petri net.

### Pipeline

```
1. Define Topology     →  places, transitions, stoichiometry matrix, rate constants
2. Native ODE Step     →  Tsit5 (7-stage Runge-Kutta) over mass-action kinetics
3. ZK Circuit          →  gnark circuit proving the ODE step was computed correctly
4. On-Chain Contract   →  ZkOde.sol verifies proofs and chains state roots
```

### How to Add a New Verifiable Computation

To apply this pattern to a new domain (e.g., supply chain, auctions, resource allocation):

#### Step 1: Define the Topology

Create a `*_topology.go` file with:
- `NumPlaces`, `NumTransitions` constants
- `Stoichiometry[NumPlaces][NumTransitions]` matrix (net change per firing)
- `TransitionInputs[NumTransitions][]int` (which places feed each transition)
- `RateConstants[NumTransitions]*big.Int` (fixed-point rate for mass-action kinetics)
- `DefaultInitialMarking()` function

The stoichiometry matrix encodes the Petri net structure. For each (place, transition) pair, the value is the net token change when that transition fires. Rate constants control relative transition speeds — they encode domain-specific priorities (e.g., position weights in TTT).

See `zk-ode/topology.go` (3-place cascade) and `zk-ode/ttt_topology.go` (33-place TTT) as examples.

#### Step 2: Implement Native Witness Computation

Create a `*_witness.go` file with:
- A state struct holding marking + MiMC root
- A native ODE step function using `NativeFixMul/Add/Sub` (big.Int arithmetic)
- A `ComputeStep()` that runs the ODE and returns a witness struct
- A `ToCircuitAssignment()` that maps the witness to circuit variables

The native computation must exactly mirror what the circuit will verify. Both use the same Tsit5 tableau (`tsit5A`, `tsit5B`, `tsit5C` from `topology.go`) and the same fixed-point scale (10^18).

Mass-action rate formula: `rate[t] = k[t] * product(marking[inputs[t]])`

#### Step 3: Build the ZK Circuit

Create a `*_circuit.go` file defining a gnark circuit struct with:
- **Public inputs**: `PreStateRoot`, `PostStateRoot`, `StepSize`, plus any domain-specific outputs (e.g., scores)
- **Private inputs**: `PreMarking[N]`, `PostMarking[N]`

The circuit must:
1. Verify `PreStateRoot == MiMC(PreMarking)`
2. Compute the Tsit5 ODE step using `FixMul` (circuit-constraint arithmetic)
3. Verify `PostMarking` matches the computed result
4. Verify `PostStateRoot == MiMC(PostMarking)`
5. Compute any domain-specific outputs from the marking

Use `FixMul(api, a, b)` for circuit multiplication — it generates constraints that verify `a*b/Scale` via hint + range check.

#### Step 4: Export Verifier and Deploy

```bash
# Compile circuit and export Solidity verifier
# See zk-ode/cmd/export-ttt-verifier/main.go as template
go run ./zk-ode/cmd/export-my-verifier/main.go > solidity/src/MyVerifier.sol

# Deploy: Verifier → Adapter → ZkOde
cd solidity && forge script script/DeployMy.s.sol --rpc-url $RPC --broadcast --verify
```

The `ZkOde.sol` contract is reusable — it tracks `currentStateRoot` and calls any `IVerifier` to check proofs. Configure with `numTransitions` and `enforceOptimal` (whether to require the highest-scoring action).

### Key Files (zk-ode/)

| File | Purpose |
|------|---------|
| `topology.go` | Cascade example (3 places, 2 transitions) + Tsit5 Butcher tableau |
| `ttt_topology.go` | TTT topology (33 places, 35 transitions) with position-weighted rates |
| `fixedpoint.go` | Fixed-point arithmetic for both native (big.Int) and circuit (gnark) |
| `state.go` | State struct with MiMC root computation |
| `circuits.go` | Cascade ZK circuit (Tsit5 step verification) |
| `ttt_heatmap_circuit.go` | TTT circuit with tactical win/block scoring (177k constraints) |
| `witness.go` | Cascade witness generation |
| `ttt_witness.go` | TTT native ODE step and board-to-state conversion |
| `ttt_heatmap_witness.go` | TTT heatmap witness with tactical scoring |
| `evaluate.go` | TTT move evaluation using go-pflow ODE solver |
| `service.go` | Prover service integration |
| `httpservice.go` | HTTP API for proving and evaluation |

### Fixed-Point Arithmetic

All values use 10^18 scale over the BN254 scalar field. Key functions:

| Function | Context | Purpose |
|----------|---------|---------|
| `FixFromFloat(f)` | Native | Convert float64 to fixed-point big.Int |
| `FixToFloat(x)` | Native | Convert fixed-point big.Int to float64 |
| `NativeFixMul(a, b)` | Native | `(a * b) / Scale` with field reduction |
| `NativeFixAdd(a, b)` | Native | `(a + b) mod P` |
| `NativeFixSub(a, b)` | Native | `(a - b + P) mod P` |
| `FixMul(api, a, b)` | Circuit | Constrained multiplication via hint |

### State Root Chaining

Each proof advances the on-chain `currentStateRoot`:
```
Genesis: MiMC(initialMarking)
Step 1:  proof.PreStateRoot == currentStateRoot → verify → currentStateRoot = proof.PostStateRoot
Step 2:  proof.PreStateRoot == currentStateRoot → verify → currentStateRoot = proof.PostStateRoot
...
```

For discrete systems (like TTT), the post-state root is the MiMC hash of the discrete post-move marking, not the raw ODE output. This enables multi-step proof chains where each step starts from a clean integer state.

## ZkOde Contracts (Base Sepolia)

Deployed instances of the verifiable computation pattern.

### Verifier provenance (F4 Tier 3)

`zk-ode/provenance.json` binds each deployed Groth16 verifier to the circuit it
attests and surfaces when the source has drifted away from what's on-chain.
Each verifier carries an immutable **`deployed`** baseline — the circuit (commit,
public inputs, constraints, R1CS digest) captured from its deploy commit — and a
**`current`** block recomputed by hermetically compiling the circuit (reproducible
since F4 Tier 2). `inSyncWithDeployment` is true iff `current` == `deployed`.

`cmd/zk-verifier-provenance` (run in CI):
- recomputes `current` + the committed verifier's source hash and **fails** if
  they don't match the manifest (a circuit/verifier edit that wasn't reconciled);
- asserts the verifier's hardcoded `input[]` arity matches the circuit's public
  inputs;
- **warns** (does not fail) when a deployment is stale — that's a real,
  acknowledged state, not a manifest error. Flip `failOnDrift` in the command to
  gate CI on it once the drift is reconciled.

The verifying key is baked into the committed `.sol` as constants, so the
committed verifier is the vk of record.

```bash
go run ./cmd/zk-verifier-provenance          # check (CI)
go run ./cmd/zk-verifier-provenance -write    # recompute `current` after a circuit/verifier change
```

> **Known drift (2026-06):** `ttt_heatmap` is stale. The on-chain verifier
> `0x97a6…` attests the **176,891-constraint** circuit at commit `a0d6eb5`, but
> the source was refactored afterwards (`780d9f2` generic topology compiler,
> `0ac24bb` added the `move_tokens` place + `draw` transition) and now compiles to
> **180,253** constraints. Reconcile by redeploying the verifier from the current
> circuit (then re-baseline) or reverting those circuit changes. `cascade` is in
> sync.

The remaining leg — deployed *bytecode* == committed Solidity — needs `forge` +
an RPC, so it's a manual/opt-in helper rather than a CI gate:

```bash
RPC_URL=https://sepolia.base.org scripts/zk-onchain-bytecode-check.sh
```

Full reproducibility of the verifier from scratch would also require pinning the
verifying key (the trusted setup is randomized); that's the next hardening step.

### Cascade Contracts (3 places, 2 transitions)

| Contract | Address | Purpose |
|----------|---------|---------|
| Groth16Verifier | `0xA675a162C5097e5eBa2968C918D4D0530b7005Ae` | gnark BN254 verifier (5 inputs) |
| Groth16VerifierAdapter | `0xf0aB1678309B12fd02CFD8bABf08ec87238B2E03` | Adapts gnark interface to IVerifier |
| ZkOde | `0x2084d59f9797d96ddAA3BaE2E38745D2a5D0f6F8` | State manager (2 transitions) |

- **Genesis root:** MiMC([1,0,0]) = `0x2cc32c87522be4b588f26301aef43e600ea46d912b6d781416c83074185892aa`
- **Config:** numTransitions=2, enforceOptimal=true, 5 public inputs
- **First on-chain proof:** [tx `0xeaa4bae9...`](https://sepolia.basescan.org/tx/0xeaa4bae92172acb2e4c024142b279eb5fb0417631c698a6e16a39e306a41ba0e)

### TTT Heatmap Contracts (33 places, 35 transitions)

| Contract | Address | Purpose |
|----------|---------|---------|
| HeatmapVerifier | `0x97a6Bb8FBBbBb81BF36456829A6a41e29030f351` | gnark BN254 verifier (12 inputs, 177k constraints) |
| Groth16VerifierAdapter | `0x3211ac2a941d357819EdC2b4ce0D0888953b950E` | Adapts gnark interface to IVerifier |
| ZkOde (Heatmap) | `0xF5d9cB0247698361D561faA2E30dDA7855fC25Db` | State manager (9 transitions, enforceOptimal=true) |

- **Genesis root:** MiMC(empty board) = `0x133e015bd26233707d7a1778a30a0f8de5e0b684c8e88705d770f1ba5cb3d27c`
- **Config:** numTransitions=9, enforceOptimal=true, 12 public inputs
- **Circuit:** 176,891 constraints, 12 public inputs (PreStateRoot, PostStateRoot, StepSize, HeatmapScores[9])
- **Heatmap scoring:** `score[i] = base_rate + 10.0*win_flag - 1.5*block_flag*(1-win_flag)` — tactical win/block detection in ZK
- **Rate constants:** center k=4, corners k=3, edges k=2, wins k=1 (breaks symmetry for optimal play)
- **Discrete board chaining:** After each proof, `currentStateRoot` advances to the MiMC hash of the discrete post-move board (piece staked), not the ODE post-state. This enables multi-move proof chains.
- **On-chain proofs (2 steps):**
  - Step 1: [tx `0xa01f44fc...`](https://sepolia.basescan.org/tx/0xa01f44fcbe73a7598127e5cf25c57210c000e0e249ff7b041f6af4f17b5b709a) — X plays center (cell 4, score=4.0), 475k gas
  - Step 2: [tx `0x628b0ba6...`](https://sepolia.basescan.org/tx/0x628b0ba6b54d7a0cf16112ef2fc929d650d2e24480e9cfcc14cb7a59fcf9cb27) — O plays corner (cell 0, score=3.0), 438k gas

### Common

- **Network:** Base Sepolia (chain ID 84532)
- **Explorer:** https://sepolia.basescan.org
- **Deployer/Prover:** `0x762593292f543948CA9A9a290adC1770746d059a`

### Architecture

```
ZkOde → IVerifier(Groth16VerifierAdapter) → Groth16Verifier (gnark BN254)
```

The adapter translates between IVerifier's structured proof format `(uint256[2] a, uint256[2][2] b, uint256[2] c, uint256[] inputs)` and gnark's flat format `(uint256[8] proof, uint256[N] input)`.

### Key Files

| File | Purpose |
|------|---------|
| `solidity/src/ZkOde.sol` | State commitment manager with optimal play enforcement |
| `solidity/src/IVerifier.sol` | Standard verifier interface |
| `solidity/src/Groth16Verifier.sol` | Cascade gnark verifier (5 inputs) |
| `solidity/src/TTTHeatmapVerifier.sol` | TTT heatmap gnark verifier (12 inputs, 177k constraints) |
| `solidity/src/Groth16VerifierAdapter.sol` | Adapts gnark verifier to IVerifier interface |
| `solidity/src/ZkOdeVerifier.sol` | Stub verifier (testing only) |
| `solidity/script/Deploy.s.sol` | Cascade deploy script |
| `solidity/script/DeployHeatmap.s.sol` | TTT heatmap deploy script (9 transitions) |
| `solidity/test/ZkOde.t.sol` | Tests (17 total: stub, optimal play, adapter) |
| `zk-ode/cmd/export-ttt-verifier/main.go` | Export TTT heatmap Solidity verifier from gnark keys |

### Deployment Commands

```bash
# Deploy cascade (requires PRIVATE_KEY, BASE_SEPOLIA_RPC_URL, BASESCAN_API_KEY in env)
cd solidity && PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY \
  forge script script/Deploy.s.sol --rpc-url $BASE_SEPOLIA_RPC_URL --broadcast --verify

# Deploy TTT Heatmap
cd solidity && PRIVATE_KEY=$DEPLOYER_PRIVATE_KEY \
  forge script script/DeployHeatmap.s.sol --rpc-url $BASE_SEPOLIA_RPC_URL --broadcast --verify

# Query TTT Heatmap on-chain state
cast call 0xF5d9cB0247698361D561faA2E30dDA7855fC25Db "currentStateRoot()" --rpc-url https://sepolia.base.org
cast call 0xF5d9cB0247698361D561faA2E30dDA7855fC25Db "enforceOptimal()" --rpc-url https://sepolia.base.org
cast call 0xF5d9cB0247698361D561faA2E30dDA7855fC25Db "stepCount()" --rpc-url https://sepolia.base.org
```

## Adding a New Service to Landing Page

To add a new service/app to the petri-pilot landing page and deployment:

### 1. Create the Model

Add your Petri net model JSON file to `services/`:

```bash
services/my-app.json
```

### 2. Generate the Service Module

Generate as a submodule (no separate go.mod):

```bash
./petri-pilot codegen -submodule -pkg myapp -o examples/myapp services/my-app.json
```

### 3. Register the Service

Add an import to `examples/imports.go`:

```go
import (
    // ... existing imports
    _ "github.com/pflow-xyz/petri-pilot/examples/myapp"
)
```

### 4. Create Custom Frontend (Optional)

If you have a custom frontend, add it to `frontends/my-app/`:

```
frontends/my-app/
├── index.html
├── main.js
├── styles.css
└── ...
```

Custom frontends are served instead of generated ones when they exist.

### 5. Add Landing Page Card

Edit `landing/index.html` and add a card in the "Explore Models" section:

```html
<a href="/my-app/" class="demo-card">
  <span class="demo-icon">🎯</span>
  <h3 class="demo-name">My App</h3>
  <p class="demo-desc">Brief description of what this demo shows.</p>
  <div class="demo-meta">
    <span class="demo-tag">tag1</span>
    <span class="demo-tag">tag2</span>
  </div>
</a>
```

### 6. Update Makefile

Add the service to the `dev-run` target in `Makefile`:

```makefile
dev-run: build
    ./$(BINARY) serve -port 8083 tic-tac-toe coffeeshop ... my-app
```

### 7. Build and Test

```bash
go build ./...                    # Verify compilation
make dev-run                      # Test locally
```

### 8. Deploy

```bash
./publish.sh "Add my-app service"
```

## Deployment (pflow.dev)

All services run on pflow.dev behind nginx. Manage with the `~/services` command:

```bash
~/services list      # Show all services and status
~/services start     # Start all services
~/services stop      # Stop all services  
~/services restart   # Restart all services
```

### Service Ports

| Service | Port | URL | Service name on pflow.dev |
|---------|------|-----|---------------------------|
| petri-pilot | 8083 | pilot.pflow.xyz | `pilot-xyz` |
| pflow-xyz | 8081 | pflow.xyz | `pflow-xyz` |
| blog-stackdump | 8082 | blog.stackdump.com | `blog-stackdump` |
| modeldao-org | 8084 | modeldao.org | `modeldao-org` |
| stackdump-com | 8085 | console.stackdump.com | `stackdump-com` |

petri-pilot exposes MCP two ways:
- **stdio** via `petri-pilot mcp` (for Claude Desktop / Cursor local connections)
- **HTTP** at `pilot.pflow.xyz/mcp` (Streamable HTTP transport for remote clients; OpenAPI at `/mcp/openapi.json`)

The HTTP path replaces the retired `pilot.pflow.dev` (formerly `stackdump/pflow-pilot`, now archived). The Smithery listing is being migrated from `stackdump/pflow-pilot` to `pflow-xyz/petri-pilot` via the `smithery.yaml` in this repo.

### This Service

```bash
# Check status
ssh pflow.dev "~/services list"

# View logs
ssh pflow.dev "tmux capture-pane -t servers:pilot-xyz -p | tail -50"

# Restart
ssh pflow.dev "~/services restart"

# Attach to tmux
ssh pflow.dev "tmux attach -t servers"
```

### Environment Variables

Environment variables are configured in `~/services`. This service uses:
- `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` - GitHub OAuth
- `GOOGLE_ANALYTICS_ID` - Analytics (G-TFGGN262Z3)
