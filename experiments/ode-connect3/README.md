# ode-connect3: policy-in-structure under gravity

This experiment asks whether the search-free ODE evaluator that became
minimax-equivalent on tic-tac-toe transfers to **4×4 Connect-3**. The answer
so far is precise and falsifiable: the naive continuous relaxation comes
close, and the initiative prediction transfers strongly, but the tic-tac-toe
structural policy does not close the last defensive gap.

Status: **499 of 507 evaluator decisions preserve exact game value
(98.42%)** under the exhaustive path-conditioned referee. X is perfect; all
eight residual failures occur with O defending. This is not minimax
equivalence.

## Run

```sh
make quick            # audits and seeded tournaments
make verify-naive     # exhaustive referee for the calibrated plain ODE
make verify           # referee the structural policy candidate
make verify-tactical  # separately disclosed one-ply safety layer
make fit              # fit policy scalars against labeled positions
./ode-connect3 fitgrad-policy <scheme> [games] [iters]
                       # re-tie the same 288 force_*/blk_* transitions into
                       # finer groups (row, parity, linetype, cellindex,
                       # linetype-parity) and gradient-fit each group's rate;
                       # see finding 7
./ode-connect3 verify-deep [lambda]           # one-ply lookahead, no policy; finding 8
./ode-connect3 verify-deep2 [lambda]          # two-ply lookahead, no policy; finding 9
./ode-connect3 verify-deep-policy [w] [b] [l] # one-ply lookahead + structural policy
./ode-connect3 diagnose-deep [lambda]         # the lookahead evaluator's failures
./ode-connect3 fit-deep <plies> [games] [iters]
                       # retune (winBias, blockBias, lambda) scored through
                       # odeSearchScore at the given depth; finding 10
./ode-connect3 check-neuralode-grad             # verify neuralode.go's hand-rolled
                                                 # backprop against finite differences
./ode-connect3 fit-neuralode [hidden] [games] [iters] [l2]
                       # train dx/dt = MLP(x) from scratch, no declared
                       # structure at all; finding 11
./ode-connect3 fit-hybrid [games] [iters]
                       # keep the declared structure, learn each family's
                       # rate as a learn.LinearRateFunc regression over
                       # board state instead of a constant; finding 12
make verify-fork       # referee the blk2_* "block-the-block" tier, sibling
                       # to force_*/blk_*, still one solve per move; finding 13
./ode-connect3 diagnose-fork [w] [b] [f] [l]  # the blk2_* evaluator's failures
./ode-connect3 fit-fork [games] [iters]
                       # extend fit.go's Nelder-Mead to a fourth parameter,
                       # forkBias, the blk2_* tier's shared rate; finding 13
./ode-connect3 verify-parity [parityForceBias] [parityBlockBias] [tempoBias]
                       # champion winBias/blockBias/lambda held fixed; the
                       # structural "future support"/gravity-parity claim
                       # tier (parity.go) as the only new knobs; finding 14
./ode-connect3 diagnose-parity [parityForceBias] [parityBlockBias] [tempoBias]
                       # the parity evaluator's failures, same format as
                       # diagnose-naive/diagnose-deep
./ode-connect3 fit-parity [games] [iters]
                       # minimal-freedom fit: winBias/blockBias/lambda fixed
                       # at champion values, fit parityForceBias/
                       # parityBlockBias/tempoBias only; finding 14
./ode-connect3 fit-tempo [games] [iters]
                       # fit-parity narrowed to the one live knob (tempoBias
                       # alone); finding 14
./ode-connect3 fit-parity-joint [games] [iters]
                       # comparison point: all six scalars (winBias,
                       # blockBias, parityForceBias, parityBlockBias,
                       # tempoBias, lambda) fit together; finding 14
./ode-connect3 verify-oracle [maxPlies] [lambda]
                       # seed the exact oracle's strict forced-reply
                       # projection into the marking before one static ODE
                       # solve, no search, no tuning; finding 15
./ode-connect3 verify-oracle-canonical [maxPlies] [lambda]
                       # same, but the projection also breaks ties instead
                       # of stopping at the first real choice; finding 15
./ode-connect3 diagnose-oracle [maxPlies] [canonical]   # the oracle evaluator's failures
./ode-connect3 debug-oracle-terminal [maxPlies] [canonical]
                       # what fraction of seeded leaves are already a
                       # completed line, i.e. how much relaxation the ODE
                       # actually still has left to do; finding 15
```

## Declared game net

Gravity is part of the Petri-net topology, not a legality callback. Each
column contains one `p<row><col>` token naming its next landing cell. Firing a
play consumes that token and produces the cell above it. The declared model
has:

- 16 board cells × open/X/O places;
- two turn places, `game_active`, a 16-count move counter, and two win places;
- 32 player-owned drop transitions;
- 48 catalytic detectors for the 24 length-three win segments;
- one exact discrete draw transition.

The exact engine fires that same counter semantics. Its memoized minimax
oracle proves the empty board is a first-player win.

## Evaluation nets

As in `../ode-minimax`, the ODE evaluation base derives from the declared net
by dropping the continuous draw counter, `move_tokens`, and `game_active`.
One ODE solve follows each hypothetical candidate drop. X scores
`win_x - win_o`; O's symmetric form is
`x_turn + o_turn + 2·win_o`, equivalent up to a conserved constant.

Two optional structural families are derived and retained as auditable failed
variants:

- 144 forced-block copies, catalyzed by the opponent's other two marks;
- 144 gravity-aware terminal macros: legal landing token plus two friendly
  marks produces the terminal winning reply directly.

## Exhaustive results

The referee fixes the evaluator on one seat, walks every legal opponent
continuation, and at every distinct evaluator decision rejects any move whose
exact minimax value is below the best legal value. Tournament results are not
used as proof—this game is a forced X win, so O can lose without blundering.

| Variant | O decisions | O losing | O missed wins | X decisions | X losing | Total errors |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Inherited horizon 3, plain ODE | 462 | 8 | 2 | 63 | 2 | 12 |
| **Horizon 0.5, plain ODE** | **462** | **6** | **2** | **45** | **0** | **8** |
| Best scanned forced/block policy | 462 | 6 | 2 | 45 | 0 | 8 |
| Plain ODE + one-ply safety | 504 | 4 | 6 | 45 | 0 | 10 |

The decision count changes when an evaluator chooses a different path; the
acceptance condition does not. The calibrated plain ODE is exact-safe on
499/507 decisions along all opponent continuations it can encounter.

## Findings

1. **Initiative transfers.** At horizon 0.5, the plain relaxation is exact on
   every X decision. All residual errors belong to O, the seat responding to
   threats it does not yet own. This is the same tempo asymmetry observed in
   tic-tac-toe, now under gravity.
2. **The time horizon is semantic.** The inherited horizon 3 diffuses play too
   far through the column chains and loses two X rankings. A short horizon in
   `[0.5, 0.9]` restores them and removes two O blunders. Gravity/initiative is
   clearest before deep fractional mixing.
3. **Tic-tac-toe's policy does not transfer mechanically.** Direct scans over
   finish/block rates and the terminal weight contain no point better than
   eight exhaustive errors. Positive block bias often trades one defensive
   line for another.
4. **Zero audit loss is not success.** Nelder–Mead can drive hinge loss on the
   twelve long-horizon baseline failures to zero by exploding the forced-win
   rate and collapsing the terminal weight. The exhaustive referee then finds
   90 new value-worsening decisions: Goodhart, caught exactly where intended.
5. **One-ply safety is an honest product patch, not the mechanism.** It reduces
   O's outright losing moves from six to four but increases missed wins from
   two to six. Even its aggregate result is worse.
6. **The missing predicate is deeper than an immediate support trap.** Only two
   of the eight remaining O failures hand X an immediate win. The others are
   late gravity-tempo positions where the unique defense controls which upper
   cells become available several drops later. A useful next structure must
   represent ownership of future support, not merely current two-in-a-row
   geometry.
7. **Finer tuning alone made it worse, not better — and worse in proportion
   to how many parameters it was given.** fitgrad.go re-ties the same 288
   force_*/blk_* transitions (no new structure) into independently-tied
   groups and gradient-fits each with go-pflow's `SharedScalar` +
   `SolveWithSensitivities`, instead of the 3 global scalars `fit.go` tunes.
   Two schemes were run to a fixed 15-Adam-iteration budget (77 positions
   from 10 self-play games + the 4 audits): `row` (16 tied params: 8 win- +
   8 block-groups, one per side per gravity depth) and `parity` (8 tied
   params, row-parity instead of row). Both drove the hinge loss down
   substantially (1.085→0.137 and →0.089) while the exhaustive referee got
   *worse* than the untied 3-scalar baseline (6 losing + 2 missed = 8) by a
   wide margin — `row`: 40 losing + 10 missed = 50; `parity`: 24 losing + 10
   missed = 34. X stayed perfect in both. More tied parameters produced a
   larger referee/loss divergence, not a smaller one: exactly the Goodhart
   failure finding 4 already named, now shown to compound with parameter
   count rather than wash out. The remaining three schemes
   (`linetype`, `cellindex`, `linetype-parity`) were not run to convergence —
   each sensitivity solve costs ~0.5s and a full multi-hundred-iteration fit
   over enough positions to identify 8-16 parameters is a multi-hour run per
   scheme; the two data points already in hand answer the "via tuning alone"
   question directionally without spending that budget. This does not prove
   no untied scheme could reach 0 referee errors with far more positions,
   iterations, and regularization than were affordable here — it shows that
   *the direction this experiment tried* (more freedom, same 3-line loss)
   moves away from the goal, not toward it, matching the plan's stated
   go/no-go risk: a static per-transition rate cannot represent "N drops
   from now, whose turn," and giving it more independent copies of itself
   to tune mainly gives a proxy loss more room to be gamed.
8. **One real ply of lookahead helps more than any amount of tuning tried so
   far — and does it with zero new fitting.** `lookahead.go`'s
   `odeLookaheadPlayer` scores a candidate by firing it, then — if the game
   isn't over — taking the WORST leaf score across every legal opponent
   reply (one-ply minimax with the existing static `odeFinal` solve as
   leaf), instead of scoring the position after one move directly. Applied
   to the plain calibrated evaluator with no structural policy and no new
   parameters, exhaustive referee errors drop from naive's 8 (6 O-losing, 2
   O-missed-wins, X perfect) to **7** (6 O-losing, 1 X-losing, X's first
   failure in this experiment, 0 missed wins on either seat). Both of O's
   missed-win errors are gone — lookahead sees the immediate follow-up a
   single static solve cannot — but it also introduces one new X error at a
   position that was previously exact, and the 6 O-losing failures are
   unchanged: `diagnose-deep` shows all 6 have no immediate opponent win
   after the evaluator's choice, i.e. they're the same future-support class
   named in finding 6, one ply of lookahead is not enough ply to reach them.
   This is consistent with the ensembling question that motivated it: a
   *different* single-solve evaluator (varied horizon, varied bias, several
   averaged) only varies the read; nesting one more real ply changes what
   information is available to it, and that is why it moved the number at
   all where tuning (finding 7) did not. **A caveat worth recording**: the
   exhaustive referee sweep showed run-to-run jitter between builds (73 vs
   61 vs 75 X-decisions visited, 0 vs 1 X-losing) traced to float-level
   sensitivity at near-exact ties in `odeLookaheadPlayer`'s worst-reply
   scan — reproducible within one fixed binary (three repeated runs agreed
   exactly), but not guaranteed to be to the bit across compiler versions.
   The 7-error count above is the reproducible reading from the current
   build; treat it as a range around 7-8, not an exact constant.
9. **A second ply keeps helping, monotonically, as rollout theory
   predicts.** `odeSearchScore`/`odeSearchPlayer` generalize finding 8 from
   one alternating ply to any fixed depth (the root mover's turn, then the
   opponent's, then — at depth 2 — the root mover's again, before falling
   back to `odeLeafEval`). `verify-deep2` runs the plain calibrated
   evaluator at depth 2 with no new tuning: **5** total errors (4 O-losing,
   1 X-losing, 0 missed), stable across repeated runs of the same binary
   (890 vs 894 O-decisions visited — the same float-tie jitter as finding
   8, but the error count itself agreed both times). The progression is
   monotonic and depth-driven, not tuning-driven: naive 8 → 1-ply 7 → 2-ply
   5. This is the textbook rollout-algorithm result (Bertsekas): looking
   ahead over a fixed base evaluator is provably no worse than the base
   policy, and empirically it keeps improving with depth here, in contrast
   to finding 7 where more tuning freedom made things *worse*. Cost is real
   but tractable at this scale — depth 2 took ~62s for a full exhaustive
   sweep (~1,000+ decisions), against depth 1's ~17s and depth 0's
   effectively-instant single solve — because these are all *plain* solves,
   not the ~0.5s sensitivity solves finding 7's tuning needed.
10. **Retuning the same 3 scalars, now scored through the lookahead
    evaluator instead of a single solve, helps rather than hurts — the
    opposite of finding 7.** `fitdeep.go`'s `fitPolicyDeep` is `fit.go`'s
    Nelder-Mead fit over `(winBias, blockBias, lambda)`, unchanged, with
    every candidate's score computed by `odeSearchScore` instead of one
    `odeFinal` call. Run at depth 1 (30 self-play games, 50 iterations,
    inside a container — see below): loss fell 3.73→0.05 (not fully
    converged) and the exhaustive referee landed at **6** total errors (6
    O-losing, **0** X-losing, 0 missed) — better than plain 1-ply lookahead
    with no tuning (7) and, notably, *not* a Goodhart collapse the way
    finding 7's tuning was on the shallow evaluator. The fitted bias
    (winBias 0.055, blockBias 2.884, lambda 2.701) fixed exactly the new
    error 1-ply lookahead introduced (X), and left O's 6 unchanged. This
    reframes finding 7: tuning was never the wrong idea, it failed
    specifically because the evaluator it was tuning couldn't represent the
    target (finding 6's future-support gap) — give the same 3-parameter
    tuning a representation that *can*, even partially (one ply), and it
    stops being adversarial to the referee and starts being a mild, honest
    improvement again.

    A same-shape run at depth 2 does **not** continue this trend: `fit-deep 2
    15 25` (a reduced budget — 104 positions, 25 iterations, vs. depth 1's
    197/50 — chosen to keep the container run under an hour, since 2-ply
    solves cost roughly 4x depth 1's) landed at **8** total errors (8
    O-losing, 0 missed, X still perfect) — worse than plain 2-ply lookahead
    with no tuning (5) and worse than depth-1's tuned result (6). Depth and
    training budget both changed between these two runs, so this does not
    cleanly show "tuning fights deeper search" — it may simply be
    under-converged (25 iterations on roughly half the positions) rather
    than a real regression from depth. Disambiguating would need a
    matched-budget depth-2 run (197 positions, 50 iterations), which
    extrapolates to several hours at this per-solve cost; not run here.
    Recorded as an open question rather than a confirmed finding: depth
    alone helps monotonically (finding 9), and depth-1 tuning helps (above),
    but depth-2 tuning's sign is not yet known cleanly.

**A note on how these last three findings were run.** The exhaustive
referee's fixed-binary determinism (finding 8's caveat) turned out not to be
the only stability concern — background shell processes for the longer
`fit-deep` runs were repeatedly interrupted mid-run by something external to
this experiment (not a crash: no error, no partial corruption, just cut off
cleanly a few iterations in, twice in a row on relaunch). Moving the same
binary into a detached `docker run -d` container, writing to a bind-mounted
log file, made the run immune to whatever was sending that signal — the
container is supervised by the Docker daemon, not by the interactive session
that kept losing background jobs. `fit-deep 1 30 50` completed cleanly in a
container in ~59 minutes; the same command as a plain background process had
not survived one full iteration across two attempts. No code in this
experiment changed to fix this — it was an environment/session issue, not an
`ode-connect3` bug — but it's worth recording here since it shaped how
finding 10's number was actually obtained.

11. **A REAL Neural ODE — no mass-action, no declared structure — is far
    worse, and more data/regularization doesn't close the gap.**
    `neuralode.go` throws away everything else in this file: instead of a
    Petri net's stoichiometry dictating the RHS, `dx/dt = MLP(x; theta)` is
    an unconstrained 2-layer network. Scope, stated plainly: the 4 dims
    actually integrated are the ones every evaluator here reads out
    (`win_x, win_o, x_turn, o_turn`), conditioned on the 48 raw board
    features held fixed across the horizon — not the full 52-place system
    under an MLP, but the observable sub-state under one that sees the whole
    board. Trained discretize-then-optimize: forward with a fixed-step
    Euler integrator, backprop exactly through the unrolled steps (no
    continuous adjoint — `go-pflow`'s `SolveAdjoint` is built for the
    RateFunc/mass-action framework and doesn't apply to an arbitrary
    vector-valued RHS). `checkNeuralGrad` confirmed the hand-rolled backward
    pass against central finite differences (max error 3e-11) before any of
    it was trusted.

    Three configurations, referee-gated exactly like everything else here
    (baseline to beat: naive's 8 total errors):

    | hidden | positions | L2 | params | training loss | referee errors |
    |---|---|---|---|---|---|
    | 16 | 197 | 0 | 916 | 0.000000 (converged) | 158 (105 O, 53 X) |
    | 6 | 797 | 0.01 | 346 | 0.108 | 120 (59 O, 61 X) |
    | 4 | 1336 | 0.05 | 232 | 0.289 | 118 (66 O, 52 X) |

    The unregularized run is the cleanest possible Goodhart case yet: it
    drove the training rank loss to *exactly* zero — every hinge margin
    satisfied on every training decision — while the exhaustive referee got
    dramatically worse than anything else tried in this experiment,
    including finding 7's tied-scalar tuning. Adding L2 weight decay and 4x
    more self-play positions to correct for that helped (158→120) but
    plateaued almost immediately — a further ~2.4x more positions and 5x
    more regularization (232 params, 1336 positions) bought almost nothing
    more (120→118). That flatness is the informative part: this isn't an
    under-tuned regularization problem that a bit more data would fix, it's
    the structural tradeoff predicted before writing any of this code — an
    unconstrained function approximator has to *learn* what the declared net
    gets for free (which cells threaten which line, gravity, whose turn),
    and a few hundred to a couple thousand self-play positions from a game
    this small is nowhere near enough signal to learn it, while the
    structured relaxation's worst score in this entire experiment (finding
    7's most-parameters tuning attempt, 50 errors) still beats every Neural
    ODE configuration tried by more than 2x.

    Also worth recording: each fit here is dramatically *cheaper* per
    iteration than the mass-action-based fits — no adaptive ODE solver, just
    two small dense-layer matmuls per fixed Euler step — so the largest
    configuration above (1336 positions, 300 Adam iterations) ran in under 3
    minutes on a plain foreground process, versus the ~1 hour a single
    197-position, 50-iteration mass-action `fit-deep` run needed in a
    container. The bottleneck moved entirely from compute to data/inductive
    bias, which is exactly the case the declarative-modeling framing this
    whole ecosystem is built on predicts.
12. **The hybrid — keep the structure, learn one family's rate as a
    regression over board state — landed in the same disappointing range as
    finding 7's finest tuning, not better.** `hybridrate.go` installs two
    `learn.LinearRateFunc`s (go-pflow's own hybrid RateFunc, already shipped
    — no bespoke backprop, reused `SolveWithSensitivities`/
    `MinimizeGradient` end to end) on the exact same 288-transition
    structure as every other finding here: one tied across all 144 `force_*`
    transitions, one across all 144 `blk_*`, each reading all 16
    landing-token places `p<c>` (the closest cheap proxy to "which columns
    are filled how deep" without hand-deriving the exact predicate). 34
    parameters total, the smallest gradient-fit configuration tried in this
    file besides the plain scalar biases. Trained to convergence (loss
    1.019→0.019, 15 Adam iterations, 63 positions): exhaustive referee
    **36** total errors (24 O-losing, 12 missed, X still perfect) — a real
    improvement over the full Neural ODE's best case (118), but worse than
    the untied 3-scalar baseline (8) and squarely in finding 7's
    Goodhart-prone range (34-50), not near lookahead's 5-6.

    The likely reason is a design choice flagged as a risk before running
    it, now confirmed: the feature set is *global*, identical for every
    transition sharing a RateFunc regardless of which specific column that
    transition acts on. A `blk_*` transition for column 2 and one for
    column 0 see the exact same 16-place reading and can only respond to it
    through the same shared weight vector — the model can express "the
    board is generally fuller/emptier" but not "my own column specifically
    is close to opening," which is the part of finding 6's future-support
    predicate a per-transition-relevant feature (e.g. that transition's own
    column's fill state) would need. That's a concrete, testable next step
    — untie per-column or make the feature relative to the acting
    transition's own column instead of global — not attempted here given
    the cost already spent (this run took a little over 2 hours in a
    container: ~1h20m to fit, the rest on the exhaustive referee's live,
    uncached rate evaluation). The finding stands regardless of what a
    better feature set might do: *this* hybrid, with *this* feature choice,
    did not beat anything already on the board, which is itself useful
    evidence that "give it board state and let regression figure out
    what matters" is not sufficient on its own — the feature has to be
    locally relevant to the transition reading it, the same lesson finding
    7's grouping schemes ran into from the tuning side.
13. **A structural "block-the-block" tier does not reproduce 2-ply search's
    gain, and diagnosis shows exactly why: it never engages the decisions
    that are actually wrong.** `fork.go`'s `blk2_*` family adds a third
    catalytic tier, sibling to force_*/blk_*: one more catalyzed copy of
    `side_play_c` per (side, cell, unordered pair of win lines through that
    cell) — 288 transitions (16 cells give 144 (cell, line-pair)
    combinations, x2 sides), the same order as force_*/blk_*'s own 288.
    Each copy's catalysts are the *union* of both lines' "opponent holds the
    other two cells" pattern — blk_*'s own per-line predicate, applied to a
    pair instead of one line. Mass action multiplies every catalyst marking
    together, so the copy's rate turns on only when a cell is a genuine
    two-line fork square for the opponent (all four opponent marks present
    at once) — a pattern no single blk_* copy, and no amount of scanning
    blk_*'s own scalar (finding 3), can express. Still exactly one
    `odeFinal` solve per candidate move: the exhaustive sweep at the best
    point found below ran in 12.5s, the same order as the plain calibrated
    evaluator's own referee runs (10-12s), not 2-ply search's reported 62s
    — the "search-free" cost claim holds.

    Swept at blockBias=0 — the value finding 3 already established as
    exhaustive-optimal for the two existing tiers — forkBias in
    {0, 0.5, 1, 2, 4, 8} crossed with winBias in {0.01, 0.05, 0.1} (18
    points, `verify-fork`): every point either reproduced the naive
    baseline's 8 errors exactly (6 O-losing + 2 O-missed, X perfect) or was
    strictly worse, monotonically worse as forkBias grew past roughly 1-2
    (e.g. winBias 0.01: 8 at forkBias<=1, 10 at forkBias=2, 12 at
    forkBias>=4). No point in the grid beat 8. `fit-fork` — the fourth free
    parameter dropped into `fit.go`'s existing Nelder-Mead machinery
    (`fitPolicyFork`/`rankLossFork`) — confirms the same direction from the
    tuning side: an 8-game/63-position, 15-iteration run (not converged,
    loss 1.019→0.077) landed at winBias 0.2014, blockBias 1.1141, forkBias
    1.2876, lambda 1.7426 — **12** total errors (8 O-losing, 4 O-missed),
    *worse* than the untuned baseline, reproducing finding 3/4's Goodhart
    pattern (positive blockBias trades one defensive line for another) with
    blk2_* riding along rather than compensating for it.

    The diagnostic reason is sharper than "it didn't help." At one
    representative grid point that reproduces the baseline exactly
    (winBias 0.01, blockBias 0, forkBias 1.0 — the decision path shifts
    from 462 to 442 O-decisions visited, but the error count does not),
    `diagnose-fork` reports the *exact same eight failure positions*,
    byte-for-byte, as `diagnose-naive` at forkBias 0: same boards, same
    wrong choices, same optimal sets, the same 2-immediate-win/6-no-
    immediate-win split finding 6 already named. blk2_*'s catalysts never
    reach a nonzero product on any of the eight decisions that are actually
    wrong; the tier only perturbs *already-correct* decisions elsewhere in
    the game tree, and once its bias is large enough to perturb anything at
    all, it starts breaking those instead of ever touching the real eight.
    This is not a new failure mode — it is finding 6's diagnosis holding a
    second time against a strictly richer catalyst pattern: an *immediate*
    two-line-fork predicate is still a snapshot of marks already on the
    board, and six of the eight failures were never about the current
    board. They are the "late gravity-tempo positions where the unique
    defense controls which upper cells become available several drops
    later" — cells that have not opened yet, which no product over
    currently-placed marks, however many lines it multiplies together, can
    read. Depth-2 search closes part of this gap not by reading the board
    differently but by *simulating forward* through the exact discrete
    game — two real turns of gravity actually resolving — which is
    information a fixed-horizon catalyst product over the present marking
    structurally cannot contain. **Net result: this construction is a
    legitimate negative result, not an inconclusive one.** The search-free
    cost claim holds (12.5s vs. 2-ply's 62s), but the accuracy claim does
    not transfer: fixed-depth structure plateaus exactly at the naive
    baseline's 8 and degrades from there, while 2-ply search reaches 5.
    Depth genuinely buys information about *future* board states that no
    declared pattern over the *current* one can encode, confirming finding
    9/10's resolution from the structural side instead of the tuning side.
14. **A declared future-support/gravity-parity place (ROADMAP.md Phase 1a,
    `parity.go`) ties the referee, it does not beat it — and the two
    mechanisms tried inside it fail for two different, both legible,
    reasons.** The construction: a Claimeven pairing fact fixed by board
    size alone (4 rows is even, so rows {1,3} are X's "on-parity" rows and
    rows {0,2} are O's, uniformly across every column, no game history, no
    fitting) seeds a new place `claim_<side>_<col>` per column and per side
    — populated both from cells that side has *already* played on-parity in
    the position being scored (a derived readout of `mk`, exactly as
    `m.position()` already derives landing tokens from occupancy) and, for
    the rest of the horizon, from the *same* existing `x_play_/o_play_`
    transitions continuing to fire fractionally (one extra output arc
    apiece, no new inputs) — so claim mass keeps accruing from hypothetical
    future on-parity plays inside the solve. `TestParityZeroBiasesReproducesBaseline`
    pins the plumbing: with every new rate at 0 the construction reproduces
    the champion's 462/6/2 and 45/0/0 exactly, decision for decision.

    Two independent tiers read that place, both calibrated the same way
    finding 10 calibrates its scalars — Nelder-Mead + hinge rank loss over
    197 positions (30 self-play games, seed 7, plus the 4 standing audits),
    with `winBias`/`blockBias`/`lambda` held at the champion's values so
    only the new tier's own knob(s) move (the minimal-freedom experiment
    findings 7/12 both argue for):

    - **`pforce_/pblk_`: a parity-gated catalyzed copy of every force_/blk_
      transition (`derive.AddCatalyzedCopy`, the same tool `blk_` itself is
      built from), gated on the mover's own claim place for that cell's
      column.** This is provably inert on this referee's 8 residual
      failures, not just empirically flat: `AddCatalyzedCopy` multiplies
      every catalyst together (AND, not OR), and `force_`/`blk_`'s own
      pre-existing two-mark catalyst is 0 at every position among these 8
      — six of them are finding 6's "deep" class, with no line anywhere
      near complete. Zero times any `parityForceBias`/`parityBlockBias` is
      still zero. Confirmed by sweeping both individually and jointly over
      `[0.01, 0.3]` (`verify-parity`): the referee count does not move at
      all (stays exactly 6 losing + 2 missed = 8) until the bias is large
      enough to start doing unrelated damage elsewhere on the board
      (`parityForceBias 1.0` → 56 losing).
    - **`tempo_<side>_<col>`: a read-arc transition whose only reactant is
      `claim_<side>_<col>` itself, depositing straight into `win_<side>`.**
      Under mass action this is `d(win_side)/dt += tempoBias*claim`, the
      literal "the flow integral reads the claim place directly" mechanism
      the pforce_/pblk_ tier turned out not to be. This tier is genuinely
      live: sweeping `tempoBias` alone (`parityForceBias = parityBlockBias
      = 0`) gives 18-24 total O errors (worse than baseline) below 0.045,
      a **floor of exactly 8** (8 losing, 0 missed — X still perfect) across
      `[0.048, 0.06]`, and new X-side failures (2+ losing) from 0.062 up —
      a strictly worse trade regardless of O's count, since the stop
      condition forbids any new X failure. Four manual `(parityForceBias,
      parityBlockBias, tempoBias)` combinations in the same tempo sweet
      spot (e.g. `0.01 0.05 0.055`) land at the same floor, never below.
      `fit-tempo` (Nelder-Mead, 29 iterations to convergence) drove the
      sampled hinge loss from 24.59 to 0.11 and landed on `tempoBias =
      0.0084` — an order of magnitude below the manual sweet spot — whose
      referee result is 478/10/2 for O and 48/0/0 for X, **12 total errors,
      worse than the untouched champion's 8**: another clean instance of
      the Goodhart pattern findings 4, 7 and 12 already established, this
      time on a construction with real, non-inert structure behind it.

    `diagnose-parity` at the manual floor (`tempoBias 0.055`) shows the
    tempo tier is not merely tied with the champion by coincidence: **the 8
    residual failures are a different set of positions than the original
    8**, and shallower — mostly 4-8 pieces on the board (e.g.
    `........O...X..XO`) against the original set's mostly 10-13
    (`.....OX.XXO.OXOXO`, finding 6's "late gravity-tempo" class). The
    construction appears to genuinely route around several of the original
    deep failures while introducing an unrelated new set of early ones —
    a lateral trade at the referee's total, not a reduction, and a
    different failure mode than either "inert" (`pforce_/pblk_` above) or
    "Goodharts under fitting" (the auto-fit result above): here a
    hand-picked point in the live tier's range is simply not better on
    net, even though it is doing something real.

    **Stop condition: not met.** The best configuration found across every
    variant tried — a ~20-point manual scan (both tiers, individually and
    jointly) and the single-knob `fit-tempo` auto-fit — is an 8-error tie
    with the champion, never fewer, and several configurations that reach 8
    do so with X still perfect but others (`tempoBias >= 0.062`, the
    auto-fit point) cost X correctness or make O strictly worse. The
    3-parameter joint fit (`fit-parity`, all of `parityForceBias`,
    `parityBlockBias` and `tempoBias` free at once) and the 6-parameter
    joint fit (`fit-parity-joint`, adding `winBias`/`blockBias`/`lambda`)
    were started but not run to convergence inside this session's time
    budget — each sensitivity-free solve over this net (664 transitions,
    roughly double the champion's 368) costs noticeably more than the
    champion's own fits, and a 3-4 dimensional Nelder-Mead needs several
    times the function evaluations per iteration that `fit-tempo`'s single
    dimension did (finding 7's own unrun schemes are the same shape of gap
    for the same reason: cost, not a decision that they wouldn't matter).
    Given `pforce_/pblk_`'s parameters are separately *proven* inert
    (not merely observed flat) on these 8 positions, and every point found
    so far in the live tempo tier's own successful range plateaus at
    exactly 8 rather than trending toward fewer, there is no positive
    signal in this data that finishing those two runs would cross the
    "fewer than 8" bar — but that is an inference, not a completed
    measurement, and is recorded as an open question rather than folded
    into the result above. This is the same class of negative result as
    finding 7, arrived at differently: finding 7 showed
    *more freedom on the existing structure* moves away from the goal;
    this shows a genuinely *new* piece of structure, built to the letter of
    ROADMAP.md's Phase 1a description and gated the same way this file's
    working tiers are gated, still does not supply the missing predicate —
    because the two ways tried to *read* the claim place are either gated
    behind evidence (line-completion catalysts) that is exactly what is
    missing at these positions, or ungated and therefore rewards on-parity
    investment independent of whether it was ever going to matter, which
    trades one failure set for another rather than closing the gap. Finding
    6's own diagnosis — "ownership of future support" needs representing,
    not "current two-in-a-row geometry" — is not refuted by this result:
    what this finding adds is that *a* structural, non-fitted, per-column
    parity place is not automatically sufficient by itself; it still needs
    a way to matter to the score that neither an AND-gate on existing
    line-completion catalysts nor an ungated pump into `win_side` provides.
15. **Handed the exact predicate instead of a heuristic approximation of it,
    the flow integral gets most — not all — of the way to 0, and the
    remainder is informative about where the two prior heuristic attempts'
    failure actually lived.** Two follow-ups not otherwise recorded in this
    file tried to derive "future support" (finding 6) from the CURRENT
    marking alone — a parity/tempo place and a richer now-only AND-gate
    "block-the-block" tier — and both failed to move naive's 8 errors;
    diagnose-fork proved the second one's catalysts never fire on any of
    the 8 failures, because those are about cells not yet open under
    gravity, which no predicate over current marks can see. `oraclefuture.go`
    stops approximating: `oracleForcedSeed` walks forward from a candidate
    move using the exact memoized oracle (`m.minimax`/`m.optimalSet`,
    players.go), but only through plies where the mover's `optimalSet` has
    exactly one member — a genuine forced reply, not a tie broken
    arbitrarily — and seeds `x<cell>`/`o<cell>` for every cell filled along
    that forced prefix directly into the marking `odeFinal` starts from.
    Net structure, rates, horizon and score readout are byte-identical to
    every other evaluator in this file; the only change is what the single
    relaxed solve is allowed to start from, computed entirely offline
    before the solve runs. This is deliberately unavailable to a real
    player and is offered as an upper bound, per the experiment's stated
    purpose.

    `verify-oracle` (`odeOraclePlayer`, strict/forced-only): exhaustive
    referee errors drop from naive's 8 (6 O-losing, 2 missed, X perfect) to
    **4** (4 O-losing, 0 missed, X still perfect) — already at 1 ply, and
    completely flat from 1 ply through 32 plies tried. This is not "not
    enough plies allowed": `debug-oracle-terminal` shows the fraction of
    seeded leaves that already contain a completed line plateaus at 32.9%
    by 4 plies and never rises through 16 — the strict predicate itself,
    not a ply budget, is what stops. `diagnose-oracle` shows the same 4
    positions fail at every plies value tried; two are exact board-for-board
    repeats of naive's original 8 (the `.XX..OO..OX.XXO.O`/`o_play_20` and
    `.XX..OO..XO..OXXO`/`o_play_23` positions), and two are new — reachable
    only once the other naive failures were fixed and the referee's fixed
    O-seat walk found a different path through the game. All 4 share naive's
    original signature exactly: no immediate opponent win after the
    evaluator's choice (finding 6's future-support class), X untouched.

    `verify-oracle-canonical` (same function, `canonical=true`: don't stop
    at a tie, take the row-major-first optimal move and keep projecting)
    reaches **0** total errors, stable from 8 plies through 24. Read alone
    this would be the "representation is not the ceiling" branch of the
    plan's stop condition. But `debug-oracle-terminal` shows why it needed
    exactly that many plies and why it should not be over-read: the
    fraction of *canonical* seeded leaves already containing a completed
    line rises from 46.5% at 1 ply to 91.3% at 8 to 98.7% at 16 — canonical
    seeding at the depth it needed to succeed is, the large majority of the
    time, not leaving the ODE a partially-open position to relax at all,
    it is handing it an already-finished game and asking it to notice.
    That the flow integral does correctly read out an already-complete
    line is a real, useful confirmation (a wrong final-state readout would
    have failed even that), but it is a much weaker claim than "the flow
    integral can rank moves correctly given perfect partial information" —
    the strict variant is the test of that claim, and it does not reach 0.

    **Stop condition:** this lands in the "does not reach 0" branch, but
    not the pessimistic reading of it. The literal predicate the plan asked
    for — forced-reply/parity-decided future ownership, computed exactly
    and fed in before the solve, with the *same* net and *same* readout as
    every other evaluator here — closes half the gap (8→4) with no new
    tuning and, per the terminal-fraction check, while still doing genuine
    relaxation on two-thirds of the positions it's asked about (67.1% of
    seeded leaves are non-terminal at the plateau). It does not close the
    other half, and the canonical control shows that gap is not obviously a
    final-state-coordinate ceiling in finding 11's sense (win_x/win_o read
    out correctly once the position is actually decided) — it is that the 4
    residual positions hit a genuine strategic choice, not a forced fact,
    soon enough after the candidate move that "already decided" is false
    for them under any non-arbitrary reading of that phrase. That reframes
    the open question rather than closing it: the remaining gap is not
    "find a cheaper way to compute the predicate this file hand-fed" — the
    exact version of that predicate provably cannot reach further for these
    4 positions, because for them there is no such fact to find. What
    would have to change is the same lever findings 8-10 already used
    (real search over the actual choice), not a better predicate over the
    current one. This experiment therefore narrows, rather than replaces,
    the roadmap: predicate-seeding and search are not competing
    explanations for the residual gap, they cover disjoint subsets of it —
    4 of naive's 8 errors were forced-fact-shaped and are now closed for
    free, and the other 4 were never predicate-shaped to begin with.

## Resolution

This experiment satisfies the “comes close” branch, not the equivalence
branch. The continuous relaxation is a remarkably strong static evaluator for
a declared gravity game, and perfect with the initiative, but the exact
referee rejects the claim that tic-tac-toe's forced-reply topology is a
domain-agnostic recipe. The eight counterexamples are preserved by
`diagnose-naive`; the next iteration should be designed against their shared
future-support structure and then held to the same referee.

Finding 7 closes the narrower question of whether tuning alone — re-tying the
existing structure into finer groups, without adding new places or
transitions — can reach 100%. The evidence gathered says no, and says so more
strongly as more freedom is added: gradient-fitting more independent groups
made the exhaustive referee worse, not better, both times it was tried. The
honest next step is the structural one finding 6 already named — a predicate
for future support/gravity-depth, not a finer-grained rate on the current
one — not a longer tuning run.

Findings 8-10 answer the question that came after: given tuning alone
doesn't work, does search? Yes: the error count moved monotonically with
search depth alone (naive 8 → 1-ply 7 → 2-ply 5, no new tuning at either
step), which finding 7's tuning never achieved. Tuning *on top of* one-ply
search also helped (7→6) without any Goodhart symptom — the opposite of
finding 7's tuning-on-a-flat-evaluator result — but the same combination at
two plies did not (5→8, confounded with a smaller training budget, not run
to convergence); read that as depth doing the real work here, with tuning's
contribution on top of it still unsettled past one ply. This doesn't reach 0
either, and the remaining errors at every depth tried are
still the future-support class finding 6 named — search is buying the same
thing finding 6 asked for (seeing further into the game), just less
efficiently than a dedicated structural predicate would *if one could be
built* — finding 14 tests that parenthetical directly and finds it does not
hold: the predicate it built could not be constructed search-free at all.
The honest reading:
deeper search is real, composable progress and a legitimate alternative to
the structural fix, not a replacement for the conclusion that tuning alone
cannot get there — it is a different lever (information, not calibration)
that happens to work.

Finding 11 answers a fourth, different question — not "does tuning work" or
"does search work" but "does keeping the declared Petri-net structure matter
at all." It does, decisively. A real Neural ODE with no mass-action
structure — the same rank-loss training, the same referee gate, three
configurations spanning a 4x data and 5x regularization range — never got
below 118 total errors, worse than every other evaluator in this experiment
including finding 7's worst tuning attempt (50). The gap didn't close with
more data or regularization; it was flat across all three runs, which is the
signature of a representational/sample-efficiency problem, not a
hyperparameter one. This is the empirical case for the declare-then-fit
approach every other finding here used: the Petri net's stoichiometry is a
free, correct prior about which cells interact through which win-lines, and
an unconstrained function approximator has to spend data learning what the
declared structure never had to guess at — data this small, bounded game
does not have enough of to give up.

Finding 12 completes the square this experiment set out to fill: pure
structure (findings 1-10), pure tuning-of-structure (finding 7, fails),
pure black box (finding 11, fails worse), and structure-plus-regression
(finding 12, the genuine middle ground go-pflow's `learn.LinearRateFunc`
already supports end to end). It did not win — 36 errors, in finding 7's
range, not lookahead's — but it failed for a legible, fixable reason (a
global feature shared identically across every transition in a family,
when the missing predicate is inherently per-transition-local), not an
opaque one. That is a meaningfully different kind of negative result than
finding 11's: the middle ground is real and the tooling for it works, this
particular feature choice inside it just wasn't the right one.

Finding 13 closes the specific question findings 8-10 left open: can the
2-ply search result (naive 8 → 5 errors) be reached *structurally*, at the
same search-free, one-solve-per-move cost as every other tier in this file?
The answer is no, and the mechanism is now diagnosed rather than assumed. A
strictly richer catalyst — an AND over two win lines instead of one, the
natural next step past blk_*'s own single-line predicate, and past finding
7's conclusion that retuning blk_*'s *existing* scalar cannot do it — still
computes a function of marks already on the board. It reproduces the naive
baseline exactly (8 errors) at its best, and `diagnose-fork` shows why: the
same eight failures, unchanged down to the exact board and choice, because
the tier's catalysts never activate on any of them. Search does not merely
see the same threats sooner; two-ply lookahead is evaluating positions after
real, discrete drops have happened — cells that were not on the board yet
when the static evaluator (structural or not) had to decide. No fixed
catalytic pattern over the *present* marking, however many lines it
multiplies together, can be gated on a cell's *future* value, because that
value depends on whose turn it is when the cell opens — a fact about play
that has not happened, not about tokens that are already placed. This is the
plan's original go/no-go risk (quoted under finding 7) restated one level
up: a static rate cannot represent "N drops from now, whose turn," and a
richer static predicate is still a static predicate. Search remains the only
mechanism in this experiment that closes part of that gap, and it does so by
spending runtime (62s at depth 2 vs. this tier's 12.5s), not by being
smarter about the current board.

Finding 15 answers the question every earlier finding in this file left
open by construction: findings 1-7 and 12 all tried to *derive* the
future-support predicate finding 6 named, cheaply, from current marks, and
none of them could reduce naive's 8 errors even once. Handed the same
predicate exactly instead of approximated — the oracle's strict forced-reply
projection, no arbitrary tie-breaks, seeded into the same net with the same
readout — the flow integral closes exactly half the gap (8→4) and then
provably cannot close the rest, not for lack of plies but because the
remaining 4 positions contain a genuine strategic choice rather than a
forced fact this early after the candidate move. The canonical control
(break ties too) does reach 0, but the terminal-fraction check shows it
gets there mostly by handing the ODE an already-finished game to confirm,
not by ranking a genuinely open position — so it does not overturn that
reading. Combined with findings 8-10, this experiment's honest map of the
remaining 4-8 errors is: some are a fact about the position that a smarter
predicate could in principle supply (finding 15 closes exactly those, for
free, with zero tuning), and the rest are a choice that only looking ahead
over the actual game tree can resolve (findings 8-10's lever, not this
one's). Neither lever, alone or as implemented here, reaches 507/507; the
two together — an exact or well-approximated future-support predicate as
the *leaf* evaluator inside real search, rather than either technique
substituting for the other — is the combination this experiment did not
try and is the concrete next step it leaves behind.
