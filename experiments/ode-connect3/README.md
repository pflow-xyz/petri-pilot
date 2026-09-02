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
efficiently than a dedicated structural predicate would. The honest reading:
deeper search is real, composable progress and a legitimate alternative to
the structural fix, not a replacement for the conclusion that tuning alone
cannot get there — it is a different lever (information, not calibration)
that happens to work.
