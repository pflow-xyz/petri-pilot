# ode-* family roadmap: from a tic-tac-toe trick to a general technique

Three experiments so far, all asking the same question at increasing scale —
can a search-free continuous (mass-action ODE) relaxation of a declared
game's Petri net be made minimax-equivalent by adding structure, not by
rescoring the outputs?

| Experiment | Game | Result |
|---|---|---|
| [ode-minimax](ode-minimax/) | Tic-tac-toe (~5,000 states) | **Full minimax-equivalence**, both seats, exhaustive referee. Policy-in-structure (catalyzed forced-reply transitions) + 2 calibrated scalars. |
| [ode-connect3](ode-connect3/) | 4×4 Connect-3 (~5,000 states) | **98.4%** (499/507 decisions). Initiative transfers exactly (X perfect); tic-tac-toe's policy tier does not close O's remaining gap. |
| [ode-connect4](ode-connect4/) | Connect Four (~10^12 states) | **Partial.** One block-bias tier cuts O's blunder rate 57% (300/1003 → 129/1003) with zero cost to X. Three further attempts (second blind tier, Allis-parity-motivated tier, N-ply search with the ODE as leaf) all plateau or regress. |

All three independently name the same missing ingredient: a rate multiplier
on the *current* marking cannot represent **who is forced to move where, N
plies from now** — ode-connect3 finding 6 ("ownership of future support"),
ode-connect4 finding 7 ("a discrete turn-order fact a continuous rate
multiplier structurally cannot represent"). ode-connect4 finding 8 also
closed off the obvious fallback: literal N-ply minimax with the ODE as leaf
scorer is both worse *and* slower than calling the exact oracle directly —
spending compute does not substitute for spending structure.

That convergence, found independently on two different games, is the actual
lead. This roadmap is the queue of experiments that test it and try to
turn "a recipe that happens to solve tic-tac-toe" into a technique that
transfers by construction, not by re-discovery per game.

Every experiment below inherits the existing methodology and must not
weaken it: an **exhaustive or explicitly-scoped referee** (never trust
training/hinge loss alone — ode-minimax findings 11 & 16 and ode-connect4
findings 5-7 all show it can be gamed in both directions), a **stated,
falsifiable hypothesis**, and a **resolution section that says whether it
worked, including if it didn't**.

## Phase 1 — attack the named gap directly

### 1a. Token-based parity/tempo structure (highest priority)

**Hypothesis:** the missing ingredient is representable as *topology*, not
as a rate. Replace connect4's `prt_*` rate tier (finding 7, failed) with an
actual place per column/line that accumulates forced-reply commitment as
plays happen — a structural encoding of Allis's Claimeven pairing, read
directly by the flow integral instead of inferred through a coefficient.

- **Where:** new tier in `ode-connect4/champion.go`, sibling to `blk_*`.
  Candidate shape: a `claim_<col>` place per column that fills
  deterministically from the *parity of remaining open cells* in that
  column (derivable from the existing `open_c_r` chain — no new inputs, just
  a derived readout place), consumed/read by a play transition scored
  differently depending on whether the mover already "owns" the parity.
- **Referee:** the same 2077-decision, seed-13 held-out sample connect4
  already uses (`go run . verify 1.342 0.0 0.0 0.238 250` is the baseline to
  beat: O 129/1003, X 19/1074).
- **Stop condition / falsification:** if O's blunder rate does not improve
  over the single-tier baseline without regressing X (finding 4/6/7's own
  bar), the "structure vs. rate" distinction is not sufficient by itself —
  write that up as clearly as the win case. This is explicitly the untried
  option ode-connect4's own "Is NOT claimed" section names.
- **Also apply to connect3** once the shape is validated on connect4 —
  connect3's "ownership of future support" gap (finding 6) is the same
  diagnosis at smaller scale, and connect3's exhaustive (not sampled)
  referee makes it the cheaper, more conclusive place to confirm the
  mechanism before trusting connect4's sampled numbers.

### 1b. Localize the hybrid regression feature

**Hypothesis:** connect3 finding 12 already diagnosed its own failure —
the `learn.LinearRateFunc` feature set was *global* (identical board
reading for every transition in a tied family), when the missing predicate
is inherently local to the acting transition's own column/line. Untie the
feature per acting transition (its own column's fill depth, not the whole
board) and refit.

- **Where:** `ode-connect3/hybridrate.go`, minimal change — same
  `learn.LinearRateFunc` machinery, narrower feature vector per transition.
- **Cost:** cheap relative to 1a — no new topology, infra already proven
  (`SolveWithSensitivities`/`MinimizeGradient` already wired). Good
  parallel/independent track to 1a since it tests the same diagnosis by a
  different mechanism (learned local weighting vs. declared structure).
- **Referee:** connect3's exhaustive referee (`make verify`), baseline to
  beat is finding 12's 36 total errors and the plain single-tier's 8.
- **Stop condition:** if a local feature still lands in finding 7/12's
  Goodhart-prone range (worse than 8), that rules out "regression was just
  missing the right feature" and strengthens the case that the predicate
  needs to be declared structure (1a), not fit.

## Phase 2 — structural depth without search

### 2. A second catalytic tier gated on the opponent's reply-to-a-threat

**Hypothesis:** connect4 finding 8 ruled out literal tree search (worse and
slower than the oracle) as the way to buy lookahead — but that doesn't rule
out baking *one more ply's worth of pattern* into the topology itself,
still as a single ODE solve. A `block_the_block` tier, catalyzed by "the
opponent's reply would create a second threat," is structure encoding what
depth-2 search discovers, without the runtime cost of expanding a tree.

- **Where:** new derive step, `ode-connect3` first (exhaustive referee,
  smaller net — connect3 finding 9 already shows 2-ply search closes real
  ground, so this asks whether the same ground closes via topology instead).
- **Referee:** connect3's exhaustive walk; compare directly against finding
  9's 2-ply-search number (5 total errors) as the target to match *without*
  paying search's per-decision cost.
- **Stop condition:** if the tier can't approach finding 9's number, that's
  evidence depth genuinely needs runtime tree structure and no fixed-depth
  topology addition substitutes for it — a real, useful negative result,
  not a failure to report.

## Phase 3 — generalize the recipe, don't just fix one game

### 3. Cross-game transfer test

**This is the actual "general technique" checkpoint**, and it only means
something once Phase 1 or 2 produces something that beats the current
plateau on a held-out sample. Take whichever construction wins (1a, 1b, or
2) and apply it to a **fourth game never used to tune it** — candidates,
cheapest first:

- **Nim / a misère variant** — trivial state space, different game
  structure (no board geometry, no "lines"), tests whether the technique's
  vocabulary (catalyzed forced-reply copies, parity places) even has an
  analogue outside line-completion games.
- **A bigger gravity board (5×5 or 6×6 Connect-3)** — same mechanic family
  as connect3/connect4, tests whether the *fitted constants* need
  per-size recalibration (expected, and fine) versus whether the
  *structural shape* needs rediscovery (would mean the recipe isn't
  general yet).
- **A non-gravity line game with a different win-line count** (e.g. 5×5
  tic-tac-toe-to-4) — isolates line-count/board-size scaling from gravity's
  specific mechanic.

**Rule for this phase, non-negotiable given ode-connect4 findings 5-7's
Goodhart lessons:** the fourth game's referee criterion is written *before*
any fitting starts, and the construction is applied with the same
topology-generation logic, not hand-designed per game. If hand-tuning is
required to make it work on the new game, that's a finding about the
current recipe's generality, not a disqualifier for reporting it.

## What not to re-attempt

Recorded so nobody re-spends budget re-deriving these:

- **Rescoring the unmodified net.** Proved impossible for tic-tac-toe by a
  dominance argument (ode-minimax finding 6) — the fork position dominates
  the optimal move on every final-state coordinate simultaneously, for any
  monotone scoring function. Any new game should expect the same wall and
  go straight to structural tiers.
- **Finer-grained tuning of existing structure with no new predicate.**
  ode-connect3 finding 7: more tied parameters over the same 288
  transitions made the exhaustive referee *worse*, not better, and worse in
  proportion to how much freedom was given — a repeated, budget-costly
  Goodhart pattern, not a convergence problem.
- **A real Neural ODE (unconstrained `dx/dt = MLP(x)`) as a shortcut past
  all of this.** ode-connect3 finding 11: flat at 118-158 referee errors
  across a 4x data / 5x regularization range — worse than every structured
  variant tried, including the worst hand-tuned one. The declared net's
  stoichiometry is a free, correct prior; an unconstrained approximator has
  to learn it from far too little self-play data on games this small.
- **Literal N-ply minimax with the ODE as a leaf scorer, at these problem
  sizes.** ode-connect4 finding 8: strictly worse than the 0-ply champion
  at every depth tried, and more expensive per decision than just calling
  the real oracle. If a future game's oracle is *not* cheap enough to call
  directly (unlike connect4's, per finding 8's own note), this may be worth
  revisiting — but state that precondition explicitly before trying it
  again.

## Sequencing

Phase 1a and 1b can run in parallel — different mechanisms, same
diagnosis, useful either way. Phase 2 is independent of both and can start
any time. Phase 3 blocks on having something from Phase 1 or 2 that beats
the current plateau on its own game's referee; starting it earlier just
re-asks "does the *existing, already-plateaued* recipe generalize," which
findings 6-8 in connect4 already partially answered (no).

## Run log

Four tasks run 2026-09-19, one per isolated worktree/branch off this
checkout, targeting 1a (connect4), 1a (connect3), 1b, and 2. **None
confirmed its hypothesis; none is recommended for merge.** Branches are
left for human review, not merged/rebased/pushed here. Of the four, only
two branches exist in this local checkout as of closeout — see caveat
below the table.

| # | Roadmap item | Branch | Worktree | Hypothesis confirmed | Recommend merge |
|---|---|---|---|---|---|
| 1 | 1a, connect4 | `experiment/connect4-parity-token` | `../pp-wt-connect4-parity` | No | No |
| 2 | 1a, connect3 | `experiment/connect3-parity-token` | `../pp-wt-connect3-parity` | No | No |
| 3 | 1b | `experiment/connect3-local-hybrid` | `../pp-wt-connect3-hybrid` | No (incomplete — no referee number obtained) | No |
| 4 | 2 | `experiment/connect3-block-the-block` | `../pp-wt-connect3-blockblock` | No | No |

**1. connect4 structural parity/tempo tier (`claim_<side>_<col>` place, `clm_*` catalyzed-copy tier reading it).**
Baseline reproduced fresh in-process: O 128/1003 (12.8%), X 19/1074 (1.8%)
— 1 decision off the committed README's O=129/1003, a repeatable
build-level offset, not noise. Config A (tier alongside `blk_*`), fit 1
(15 games/105 positions): O 122/1003, X 20/1074 — clears the stop
condition (O<129, X not materially >19), independently reproduced via a
second `verifyclaim` process. Same config A, fit 2 (20 games/160
positions, same recipe): O 137/1003 (worse than baseline), X 16/1074 —
sign flip on the identical held-out sample. Config B (tier replacing
`blk_*`) is directionally consistent across both fits (O improves, X
worsens) but never nets a win (O 123→127 vs X 20→23... /21). Verdict: the
tier is not placebo (moves the referee by large, differently-signed
amounts by training sample) but does not reliably clear the bar — mixed
result, `championClaimBias` ships at 0.0. Written up as finding 9 in the
worktree's `ode-connect4/README.md`.

**2. connect3 structural parity tier (Claimeven `claim_<side>_<col>` place, exhaustive 507-decision referee).**
Baseline reproduced exactly: 462/6/2 O + 45/0/0 X = 8 total errors (499/507,
matches the checked-in champion and its regression test). Catalyzed-copy
tier (`pforce_/pblk_`, mirrors `blk_*`'s own construction): provably inert
— flat at 8 errors across `parityForceBias`/`parityBlockBias` ∈ [0.01,0.3],
AND-gated against the same line-completion catalysts that are zero at 6 of
the 8 residual failure positions. Direct tempo tier (claim → `win_<side>`
pump): best manual point `verify-parity 0 0 0.055` → 228/8/0 O +
58/0/0 X = 8 total errors — a **tie**, not an improvement, and
`diagnose-parity` shows a different, shallower set of 8 failures than
baseline (lateral trade). Auto-fit (`fit-tempo 30 50`, Nelder-Mead,
converged) lands on tempoBias=0.0084 → 12 total errors, worse than
baseline (Goodhart, matching findings 4/7/12's pattern); tempoBias≥0.062
starts failing X. No configuration — manual or fit — went below 8. 3- and
6-parameter joint fits were started but did not converge in-session and
are reported as not-run, not as a result. Written up as finding 13 in the
worktree's `ode-connect3/README.md`.

**3. connect3 localized hybrid-regression feature (Phase 1b, retrain finding 12 with a per-column local feature instead of the global 16-place reading).**
The requested construction (`hybridcolumn.go`, 16 tied `LinearRateFunc`s
untied per column, each reading only its own 4 landing-token places) was
already committed in the branch's prior history at `6be978e`, correctly
wired to `derivePolicyNetGrouped`/`SolveAdjoint`. This task's only new
work is a `count-positions` diagnostic (confirms finding 12's unstated
recipe was 8 games → 63 positions, not the 30-game/197-position default)
and launching `fit-hybrid-col 8 15 0` to match finding 12's exact recipe.
**No referee number was obtained** — the fit's Adam phase (~2900
`SolveAdjoint` calls, ~2s each, measured directly) needs an estimated
1.5–2+ hours and had not completed a single iteration when the session
ended, worsened by concurrent sibling worktrees sharing an 8-core sandbox.
No comparison to finding 12's 36 errors or the 8-error baseline can be
made honestly either way; none is claimed in the worktree's README (no
finding was appended there). Whoever resumes this needs a real compute
budget, not a code change — `experiments/ode-connect3/fit-hybrid-col-run.log`
in the worktree has the in-progress log.

**4. connect3 structural 2-ply tier (`block_the_block`, Phase 2 — reach finding 9's 2-ply-search number (5 errors) via topology instead of runtime search).**
Baseline reproduced: 462/6/2 O + 45/0/0 X = 8 total errors (same
`verify`/`verify-fork forkBias=0` sanity check). `blk2_*` tier (288
catalyzed copies of `side_play_c`, one per cell × pair of distinct win
lines through it, gated on all four opponent marks across both lines at
once — a richer AND than `blk_*`'s single-line predicate, still one
`odeFinal` solve per candidate move): best of an 18-point
(`winBias`×`forkBias`) grid, at `verify-fork 0.01 0.0 1.0 2.0`, is 442/6/2
O + 45/0/0 X = **8 total errors — ties baseline, never beats it**; rises
to 10 at `forkBias=2`, 12 at `forkBias≥4`. `fit-fork 8 15`
(Nelder-Mead, not converged) lands worse, at 12 total errors (Goodhart,
matching findings 3/4/7). `diagnose-fork` at the best grid point reports
the **identical eight failure positions, byte-for-byte**, as the untuned
baseline — the new tier's catalysts never activate on the decisions that
are actually wrong, which are finding 6's "future support" class (cells
not yet open under gravity), not immediate two-line forks. This is a
structural ceiling, not a tuning shortfall: search-free cost is confirmed
cheap (~11–12.5s exhaustive sweep vs. 2-ply search's reported 62s), but
accuracy does not transfer. Written up as finding 13 in the worktree's
`ode-connect3/README.md` (independent finding-13 numbering from task 2's
worktree — different branches, never merged, so no collision on disk).

**Phase 3 (cross-game transfer test): explicitly skipped.** Roadmap's own
sequencing rule blocks Phase 3 on Phase 1/2 producing something that both
confirms its hypothesis and beats the current plateau on its own referee.
None of the four tasks above did (task 1's config A won on one fit and
lost on another; the rest tied or lost outright), so no construction was
eligible to carry to a fourth game. Re-running Phase 3 on the
already-plateaued recipe would only re-ask what findings 6-8 already
partially answered — correctly skipped, not deferred by oversight.

**Caveat on branches/worktrees:** this local checkout (`valoper`) has
`experiment/connect4-parity-token` and `experiment/connect3-local-hybrid`
as real local branches with worktrees at `../pp-wt-connect4-parity` and
`../pp-wt-connect3-hybrid`, both committed and matching the numbers
above. `experiment/connect3-parity-token` and
`experiment/connect3-block-the-block` (tasks 2 and 4) do **not** exist
here — no local branch, no worktree directory, no remote ref, confirmed
via `git fetch --all`. Those two tasks evidently ran in a different
(ephemeral) environment whose branches never reached this host. The
findings above are reported as given by those two tasks' own structured
output, including their exact evidence, but a human reviewing this log
should locate or re-derive those two branches before trusting them as
committed, reviewable work — right now only tasks 1 and 3's branches are
actually inspectable from this machine.

## Run log — round 2 (2026-09-19, follow-up)

Three more tasks run the same day as a second wave, each targeting a
question the first wave's own findings left open: is the connect4 `clm_*`
sign-flip (task 1 above) a fluke or the norm, does the ode-connect3
representation have a ceiling independent of predicate quality, and does
the still-running hybrid-column retrain (task 3 above, left incomplete)
finally land a number. **Branch verification done first, per the standing
lesson from round 1** (two of that round's four claimed branches never
reached this host): `git branch -a`, `git log -1 --oneline` and (for the
worktree still mid-run) direct process inspection, run from this checkout
before trusting any number below.

| # | Follow-up to | Branch | Worktree | HEAD matches self-report | Hypothesis confirmed | Recommend merge |
|---|---|---|---|---|---|---|
| 5 | Task 1 (connect4 `clm_*`) | `experiment/connect4-parity-token` | `../pp-wt-connect4-parity` | Yes — `20d168a`, verified | No | No |
| 6 | Roadmap 1b resume (task 3 above) | `experiment/connect3-local-hybrid` | `../pp-wt-connect3-hybrid` | Yes — `7898908`, verified | Incomplete at closeout, see below | No |
| 7 | New: representation-ceiling test | `experiment/connect3-oracle-ceiling` | `../pp-wt-connect3-oracle-ceiling` | Yes — `8a90e74`, verified | No (does not reach 0), but resolves the ceiling question — see below | No |

All three branches exist locally with a worktree, and all three `head_commit`
values self-reported by the tasks match `git log -1 --oneline <branch>`
exactly — unlike round 1, nothing here rests on an unverifiable self-report.

**5. connect4 robustness gate for finding 9's `clm_*` claim-account tier (worktree's finding 10).**
Verified independently reproducible: `robust.go`'s `runRobustClaim` (`go run
. robustclaim 15 20 250`) refits config A (`clm_*` alongside `blk_*`) on 5
more independently-seeded training samples (seeds 101/202/303/404/505,
none reusing finding 9's seed 7), referees all 7 fitted points — the 2
already on record plus the 5 new — **in one process** against finding 9's
fixed primary sample (seed 13, 250 games, 2077 decisions) and gates any
apparent win against a second, independent held-out sample (seed 29, 250
games, 1816 decisions) never used for fitting or the primary comparison.
Baseline (same-run): primary O 128/1003, X 19/1074; validation O 79/869, X
19/947. Full 7-point table, primary O counts: finding 9 fit 1 (the one
on-record apparent win) 123, finding 9 fit 2 137, new seeds 133/147/164/159/144
— **1 of 7 beats baseline on the primary sample.** On the validation
sample, all 7 — including that one apparent win (89 vs. baseline's 79) —
score worse than baseline: **0 of 7 survive the gate.** The mean-of-7
ensemble (block 1.368, claim 1.808, lambda 0.288) also loses to baseline on
both samples (primary O 136, validation O 97), though it narrows the raw
fits' spread toward the pack. Applied retroactively, the gate would have
flagged finding 9's own apparent win before it was ever reported as one.
**Answers the round-1 open question directly: the sign-flip is not an
unlucky pair of draws, it is this calibration recipe's typical behavior on
a real (non-placebo) structural tier.** `championClaimBias` stays `0.0`.
Written up as finding 10 in the worktree's `ode-connect4/README.md`
(`robust.go`, `main.go`, `README.md` changed; commit `20d168a`).

**6. connect3 localized hybrid-regression feature, resumed (Phase 1b, retrain finding 12's global feature as a per-column local one, `hybridcolumn.go`).**
The requested construction and recipe (`fit-hybrid-col 8 15 0` — 8
self-play games → 63 training positions, 15 Adam iterations, l2=0, 81
parameters: 16 per-column-untied `LinearRateFunc`s × 5 + lambda, matching
finding 12's recipe exactly) had already been launched as a disowned
`nohup` background process (PID 1618804) by the *prior* session and
survived across the session boundary — still running, ~96% CPU, when this
round's task attached. Direct verification at round-2 closeout time
(`ps -p 1618804`): **still running, elapsed ~2h09m, no Final loss / fitted
params / referee line written to `fit-hybrid-col-run.log`** (only
`Initial loss: 1.018578`, matching finding 12's own reported initial
loss — go-pflow's `learn.MinimizeGradient` logs Initial/Final only, no
per-iteration trace). `hybridcolumn.go`'s own comment records "~2h40m in a
container" for this exact SolveAdjoint-based construction, so this is
consistent with the documented cost, not an anomaly — **but it means
round 2 also closes with no completed fit+referee pair for this task.**
No comparison to finding 12's 36 errors or the untied baseline's 8 can be
made honestly. This closeout extended the wait and re-verified the process
directly (`ps -p 1618804`, load average confirmed low/uncontended — the
concurrency that slowed round 1's sibling runs is gone) up to ~2h12m
elapsed with the log still showing only `Initial loss`; the run was left
running (still a legitimate, disowned `nohup` process, not stalled or
crashed — steady ~98% CPU, small stable RSS) rather than killed, since
killing it would discard real progress toward the one number this task
still owes the roadmap. No code changed in this worktree this round; HEAD
is still `7898908`. **Whoever next opens `experiment/connect3-local-hybrid`
should check `ps -p 1618804` and `tail fit-hybrid-col-run.log` before
relaunching anything** — the run may have completed by then.

**7. connect3 representation-ceiling test: exact oracle future-support predicate as evaluator input (new roadmap item, not in the original three phases — directly answers "is there a floor independent of predicate quality").**
Built `oraclefuture.go`: seeds `x<cell>`/`o<cell>` marks from the exact
memoized minimax oracle's strict, non-tie-broken forced-reply projection
into the marking before one unmodified `odeFinal` solve — same net, rates,
horizon (0.5), lambda (2.0) as every other evaluator in the file. Baseline
(naive, uncalibrated-predicate ODE): O 462/6 losing/2 missed, X 45/0/0 — 8
total errors. **Strict oracle-seeded predicate (`verify-oracle`, plies 1
through 32): O ~478-480 decisions/4 losing/0 missed, X 45/0/0 — 4 total
errors, flat across the entire plies range tested — this is not a
plies-budget artifact.** `debug-oracle-terminal` confirms directly: the
fraction of seeded leaves that are already a completed game plateaus at
32.9% by 4 plies and never rises through 16 — 67.1% of leaves are still
genuinely unresolved, i.e. the ODE is doing real relaxation work, not just
reading off a finished board. A stronger **canonical** control (same
predicate, ties broken too) does reach **0/507** at plies 8-24, but the
same terminal-fraction diagnostic shows why that doesn't overturn the
strict reading: canonical's seeded-leaf terminal fraction rises 46.5%
(1 ply) → 91.3% (8 plies) → 98.7% (16 plies) — it wins mostly by handing
the ODE an already-finished game to confirm, not by ranking a genuinely
open position. **Direct, unhedged answer to the theoretical question this
round asked:** the representation does *not* have a fixed ceiling
independent of predicate quality — canonical's 0/507 proves the flow
integral can read out a correct win/loss once a position is actually
decided. What's floored is naive/cheap *predicates* this early after the
candidate move: 4 of naive's original 8 errors are genuinely forced-fact-
shaped and close for free, with zero tuning, once handed the exact fact;
the other 4 sit on a real strategic choice too soon after the candidate
move for any non-arbitrary "already decided" predicate to exist — closing
those needs actual search (findings 8-10's lever), not a better predicate.
**Predicate-seeding and search are disjoint levers on disjoint parts of the
residual gap**, not competing explanations for the same one. Written up as
finding 13 in the worktree's `ode-connect3/README.md`
(`oraclefuture.go`, `main.go`, `Makefile`, `README.md`; commit `8a90e74`).

**Given both rounds together, recommendation for where this roadmap points
next:** stop Phase 1a/1b as currently scoped — rate-multiplier structure
(1a, connect4 findings 6/7/9/10) and feature-fit tuning (1b, connect3
findings 7/12, and task 3/6 above once it finally completes) have now
failed or landed unreliable across six independent attempts on two games,
and finding 10 shows the one attempt that looked like a partial win
(`clm_*`) does not survive a second held-out sample. The one genuinely new,
positive lever either round produced is finding 13: an *exact* future-
support predicate, handed to the same unmodified flow integral, closes
exactly the forced-fact half of connect3's gap (8→4) for free. The
concrete next construction is therefore Phase 2's `block_the_block` idea,
corrected in light of finding 13 — not more topology guessing at a
blind pattern, but real (even shallow) search whose *leaf* evaluator is the
oracle-seeded ODE from finding 13, since finding 13 also shows the
remaining 4 connect3 errors are a genuine choice that no predicate, however
exact, can resolve without looking ahead. Prove that combination on
connect3's exhaustive referee (cheap, conclusive) before spending any more
budget on connect4's 10^12-state, sampled-referee regime, where finding 10
just showed calibration instability eats real effect sizes.
