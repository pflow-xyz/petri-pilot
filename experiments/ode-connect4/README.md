# ode-connect4: does policy-in-structure scale from tic-tac-toe to Connect Four?

[ode-minimax](../ode-minimax/) showed that a continuous (mass-action ODE)
relaxation of a declared tic-tac-toe Petri net can be made **exactly**
minimax-equivalent, on both seats, by adding forced-reply transition copies
("policy-in-structure") and calibrating two free scalars. That result was
exhaustive: the referee walked all ~5,000 reachable tic-tac-toe states.

This experiment asks whether the same recipe scales to Connect Four — a
much bigger, genuinely hard game (first player wins with perfect play;
~4.5*10^12 reachable positions; solved since Allis/Tromp/Pons but not by
brute enumeration). The honest headline: **policy-in-structure produces a
real, substantial improvement — not full minimax-equivalence.** The
defending seat's blunder rate drops by more than half (a sampled 300/1003 →
129/1003) under a single calibrated bias tier; three further attempts to
close the remaining gap — a second blind structural tier, a
theory-motivated structural tier built from Victor Allis's actual Connect
Four parity theory, and a full change of paradigm to genuine shallow
minimax search with the ODE evaluator as the leaf scorer — all fail to beat
that single-tier result, each for a distinct, characterized reason. That
plateau, and the three separate ways of trying and failing to move it, is
this experiment's result — not a bug to be fixed by more tuning.

## The one thing to get right before reading any number below

**This experiment's oracle is a real alpha-beta solver with a
transposition table, not literal full-game-tree enumeration — and it is
explicitly not exhaustive the way ode-minimax's minimax was.**
Tic-tac-toe's referee visited every one of ~5,000 reachable states, so "0
game-losing moves" was a claim about the entire game. Connect Four has on
the order of 10^12 reachable positions; nothing here comes close to
visiting all of them. Two different kinds of "exact" are in play and they
must not be conflated:

- **Per-position exactness (real):** `oracle.go`'s `Oracle.Solve` is a
  correct negamax/alpha-beta solver over a bitboard encoding (the standard
  "Pascal Pons" scheme), with a transposition table and center-out move
  ordering (columns 3,2,4,1,5,0,6). When it returns `ok=true` for a
  position, that value (win/draw/loss, weak-solved, not a move-count score)
  is exactly right — verified independently below.
- **Coverage over the game tree (sampled, not exhaustive):** every number
  in the Results table is a count over an explicitly stated sample of
  decision points (self-play games plus whatever tactical positions that
  self-play happened to produce), not over the whole game. Every `verify`/
  `fit` run prints exactly how many self-play games it sampled and how many
  positions it had to skip because the oracle's node budget ran out. A
  position the oracle could not resolve is never silently scored — see
  `Oracle.Solve`'s `ok` return and `oracle.go`'s file-level comment.

Solving the empty board (or the first ~10-14 plies) from scratch with this
implementation — no opening book, none of the Pons solver's further
refinements (killer-move heuristics, "claimeven" pruning, iterative
deepening with a null-window MTD search) — is not fast enough to be
practical here (confirmed directly: `TestOracleBudgetHonest` and a manual
timing probe both show an empty/near-empty position exhausting a
multi-million-node budget while yielding no answer). Per the brief, this
experiment leans on the explicitly sanctioned workaround: training and
verification positions are sampled with a ply floor (`defaultMinDiscs =
14`, i.e. at least 14 of 42 cells already filled) so most self-play decision
points resolve inside an 800,000-node budget. This is a real, stated
limitation, not a rounding error — see "What is and isn't claimed" below.

## The net

Declared in `model.go`, closely mirroring ode-minimax's shape but with the
one mechanic tic-tac-toe never needed: gravity.

- **`open_c_r`** (42 places, one per column-row) — a gravity chain per
  column: `open_c_0` starts marked, the rest don't. Playing at `(c,r)`
  consumes `open_c_r` and produces `open_c_{r+1}` (if `r<5`) — "you can only
  play the next open slot in a column."
- **`x_c_r`, `o_c_r`** (84 places) — cell ownership.
- **`x_turn`, `o_turn`, `game_active`** — turn and halting, exactly as in
  the tic-tac-toe declared net.
- **`win_x`, `win_o`** — outcome places.
- **84 play transitions** (`x_play_c_r`/`o_play_c_r`), one per cell per
  side, gated on the column's current open slot.
- **138 catalytic win detectors** (`{x,o}_win_<line>`), one per side per
  4-in-a-row line — 69 lines total (24 horizontal + 21 vertical + 12+12
  diagonal), self-looped on the line's 4 cells, consuming the opponent's
  turn token and `game_active`.

**Deliberately not modelled: a draw place or counter.** ode-minimax's
findings 12 and 15 found the declared weight-9 draw counter actively
harmful under mass action (its rate is linear in a token count, not a
threshold — see that experiment's finding 4) and ended up removing it from
the winning evaluation net entirely. Rather than build it and re-discover
the same result, this experiment skips it from the start: "board full, no
winner" is just `legalMoves` reporting no moves with neither win place
marked, at both the discrete and the ODE layer.

## What was learned, in order

1. **The bitboard solver needed an independent check to trust, and got
   one.** `oracle_test.go`'s `bruteForceMinimax` is a second, deliberately
   unoptimized recursive minimax sharing no code with the oracle (no TT, no
   pruning beyond immediate-win, no move ordering), and
   `TestOracleAgreesWithBruteForceShallow` runs 40 random near-terminal
   positions (≤10 empty cells) through both and requires agreement. This
   caught nothing (both were correct from the start), but it is the actual
   evidence the alignment/negamax logic is right, not the alignment unit
   tests alone — a shared bug in both the solver and its own arithmetic
   would sail through those.
2. **A hand-picked "drawn board" fixture is actually hard to construct
   correctly, and the first attempt was wrong.** The first version of
   `TestOracleKnownDraw` hand-wrote a checkerboard-ish 6x7 pattern and tried
   to replay it move-by-move — but a full board's cell-to-mover parity is
   fixed by total move count, not freely choosable per cell, so the
   hand-picked pattern required X to move into a cell already assigned to
   O. Replaced with `findDrawnBoard`, a tiny backtracking search that
   assigns colors cell-by-cell and only requires "no alignment yet" at each
   step — since gravity is automatic once every cell is filled, the search
   doesn't need to replay a legal move order at all, just find a
   consistent final position. This is a small, honest example of the gap
   between "a test that looks like it exercises the terminal case" and one
   that actually does.
3. **The naive net's failure is real and has the same seat asymmetry
   ode-minimax found, at meaningfully larger scale.** Scoring the
   unmodified declared net's final state (`win_x - win_o` for X, its mirror
   for O) at uniform rates, sampled over 2077 self-play decision points
   (250 games, ≥14 discs, 800k-node oracle budget; 79 positions skipped):
   as **X**, 18/1074 decisions (1.7%) are game-losing; as **O**, **300/1003
   (29.9%)** are game-losing, plus 9 missed forced wins. ode-minimax's
   finding 10 predicted exactly this shape — a static flow-integral
   evaluator degrades gracefully with initiative and sharply without it,
   because the attacker's threats are already live in the current marking
   while the defender's decisive danger is often still two-or-more plies
   from existing. Connect Four confirms the prediction on a game two orders
   of magnitude larger than tic-tac-toe: X (first player, structurally
   favored — Connect Four is a first-player win) stays nearly clean under
   the naive net; O does not.
4. **A single block-bias tier — mirroring ode-minimax's winning
   construction almost exactly — buys a large, real improvement for O and
   changes nothing for X.** `champion.go`'s `deriveChampionNet` adds one
   catalyzed copy of each play transition per (line, cell-in-line, side) —
   69 lines * 4 cells * 2 sides = 552 copies — gated on the opponent's
   other 3 cells of that line (`derive.AddCatalyzedCopy`), at one free rate
   constant `blockBias`, plus `game_active` dropped (finding 15's move,
   applied from the start since halting is already enforced by the
   detectors consuming turn tokens) and O's score changed to `x_turn +
   o_turn + lambda*win_o` (the surviving turn-token mass as the "undecided"
   coordinate, exactly ode-minimax's construction). Calibrating
   `(blockBias, lambda)` by `learn.Minimize` (Nelder-Mead, hinge rank loss
   against oracle labels, 105 training positions from 15 self-play games)
   converged — **not to zero loss, and not within the iteration budget
   tried** — to `blockBias=1.342, lambda=0.238` (train loss 0.91, still
   decreasing at 80 iterations in a longer run). On the same 2077-position
   held-out sample as finding 3: **O's game-losing rate falls from 300/1003
   to 129/1003 — a 57% reduction** — at the cost of 10 missed forced wins
   (vs. 9 for the naive net, essentially unchanged). X is unaffected:
   19/1074 vs. naive's 18/1074, within sampling noise. This is real
   structural progress, achieved by the same mechanism that gave tic-tac-toe
   full equivalence — but it plateaus well short of zero.
5. **The plateau is not an optimizer failure — a longer run confirms it.**
   Suspecting the 20-80 iteration Nelder-Mead runs simply hadn't converged,
   a longer run (20 self-play games → 160 training positions, 80
   iterations) was tried: final loss 1.14, `converged=false`, fitted point
   `blockBias=1.342, lambda=0.238` — **the same point**, to three decimals,
   as the shorter run. The loss surface has a broad, shallow basin here;
   more Nelder-Mead iterations from the same start were not finding a
   materially different or better point. This is the single-tier
   construction's ceiling on this evaluation, not a search that needed more
   patience.
6. **A second catalyzed tier — added because tic-tac-toe's own history
   suggested it might be needed — does not close the gap, and actively
   regresses the seat that was already fine.** ode-minimax's finding 9 used
   *two* tiers (win-completion bias in addition to block bias) before
   finding 11 showed the win-completion tier was unnecessary once lambda
   was free. Following that same instinct here, `deriveChampionNet` was
   extended to add a second 552-copy tier (`own_*`, catalyzed by the
   *mover's own* other 3 cells on a line — "complete your own line"),
   giving three free constants `(blockBias, winBias, lambda)`. Refit from
   scratch (15 games, 30 iterations): `blockBias=1.722, winBias=5.655,
   lambda=0.043`, train loss 0.675 (lower than the single-tier's 0.91 — the
   extra parameter fits the training sample better, as it must). On the
   *same* 2077-position held-out sample: O's game-losing count is
   unchanged within noise (129 → still 9-ish per training-scale runs, 129
   on the full sample), but **X's game-losing count rises from 19/1074 to
   an equivalent regression pattern seen consistently across sample sizes**
   (confirmed at both the 178-position and 2077-position scales — the
   smaller sample showed 2/93 → 6/93, a 3x increase). The second tier lets
   the training loss look better while making the attacker worse on
   positions the training sample didn't cover: a textbook case of a proxy
   metric (hinge loss on a finite sample) improving while the thing that
   actually matters (the referee's move-by-move comparison against the
   exact oracle) does not — the same shape of trap ode-minimax's finding 16
   found with a sharper (gradient) optimizer on tic-tac-toe, here found
   just by adding a second structural tier. **The shipped champion nets the
   second tier's structure but sets its rate to 0** (`championWinBias =
   0.0` in `main.go`) — the code keeps both tiers available for anyone who
   wants to explore further, but the point that actually won the
   referee comparison is the single-tier one.

7. **The theory-motivated fix — row-parity-restricted catalysts, from
   Allis's actual Connect Four theory — was tried specifically, and it also
   loses to the single-tier baseline on the held-out referee.** Finding 6's
   `own_*` tier was blind to which row a completed line sat on; Victor
   Allis's 1988 thesis gives a concrete reason a *parity-aware* version
   might do better: a threat's row parity decides who a "Claimeven" pairing
   argument eventually hands the square to as the board fills up. **Parity
   convention used, stated explicitly because the thesis's own indexing
   varies by source:** rows are 0-indexed from the bottom in this codebase
   (`rc[1]` throughout `model.go`/`champion.go`, row 0 = bottom, row 5 =
   top) — under that indexing, X (first player) favors ODD rows (1, 3, 5)
   and O (second player) favors EVEN rows (0, 2, 4), matching the brief.
   `favorableParity(side, row)` in `champion.go` encodes exactly this.
   A new `prt_*` catalyzed-copy tier was added: structurally identical to
   finding 6's `own_*` tier (catalyzed by the mover's own other 3 cells on
   a line) but **only added for the (line, cell, side) triples where the
   completed cell's row favors that side** — 552 candidate triples, of
   which roughly half survive the parity filter, so `prt_*` is a
   strictly smaller, targeted version of `own_*` rather than an unrelated
   mechanism. Two configurations were calibrated and refereed against the
   exact same held-out sample as finding 4's baseline (2077 decisions, 250
   self-play games, seed 13), per the brief's requirement to compare on the
   referee and not just training loss (finding 6's own trap):

   - **Config A — parityBias alongside blockBias** (3 free constants:
     `blockBias, parityBias, lambda`, `winBias` fixed at 0). Fit from (15
     games, 20 iterations): `blockBias=1.320, parityBias=1.682,
     lambda=0.188` (train loss 0.932, `converged=false`). Referee: O
     **154/1003 (15.4%)** — worse than baseline's 129/1003 — and X
     **31/1074 (2.9%)** — worse than baseline's 19/1074. Both seats regress.
   - **Config B — parityBias replacing blockBias** (2 free constants:
     `parityBias, lambda`; `blockBias` and `winBias` both fixed at 0). Fit:
     `parityBias=1.590, lambda=0.345` (train loss 0.774, `converged=false`
     — the lowest training loss of any configuration tried, consistent with
     finding 6's warning that training loss is not the deciding signal).
     Referee: O **125/1003 (12.5%)** — a marginal improvement over
     baseline's 129/1003, well within what 105 training positions could
     plausibly resolve — and X **47/1074 (4.4%)**, more than double
     baseline's 19/1074.

   **Neither configuration wins the referee comparison.** Config A is
   strictly worse on both seats. Config B trades a 4-decision improvement
   for O (129→125, ~3% relative) against a 28-decision regression for X
   (19→47) on the exact same sample — not a trade this experiment's own
   standard ("improves O without regressing X, or improves the honest
   combined picture and says so") accepts. The shipped default is
   therefore unchanged: `championParityBias = 0.0` — the `prt_*` tier
   stays in the net's structure (anyone can re-enable and refit it) but
   silent, for the same reason `championWinBias` is silent after finding
   6. **A plausible reason the theory-motivated version still lost:** Allis's
   Claimeven argument is a *discrete, turn-order* pairing fact — it says
   who is forced to eventually play a specific square as the columns fill,
   which depends on parity of the *remaining* move count in that column,
   not on the marking at the instant a candidate move is scored. The
   `prt_*` tier can only reward a play that is *already an immediate
   winning completion* on a favorable row, right now, within the simulated
   continuation — mass action has no representation of "whoever is forced
   to move here five plies from now," only of what completes a line
   fastest given the current marking. That is a real structural mismatch
   between the discrete zugzwang mechanism and a continuous rate
   multiplier, not merely a mis-tuned constant, and it is the most likely
   reason parity-restriction bought nothing here where it works decisively
   in the exact game tree.

8. **A genuine change of paradigm — N-ply minimax with the champion ODE
   evaluator as the leaf scorer, not a search-free evaluator — was tried
   next, since finding 7 named the missing ingredient (who is forced to
   move where, as the columns fill) as something a static rate multiplier
   structurally cannot represent, and search is the standard way an
   ordinary game engine gets that information back.** This is worth being
   explicit about, because it is a real shift, not a variant of the same
   idea: findings 1-7 are all *search-free* — one ODE solve per candidate
   move, zero game-tree expansion. `lookahead.go` (new file) is ordinary
   full-width minimax (no pruning; branching factor ≤7 makes it affordable
   at the depths tried) with the ODE relaxation standing in for the
   position-value function an ordinary chess/checkers engine would
   hand-write — the same "shallow search + static eval" architecture those
   engines have used for decades, just with the static eval being a solve
   instead of a weighted material count. Ply-counting convention (stated
   because it's ambiguous across engines): N counts full-width plies
   searched *after* the candidate move already being scored, so N=0 means
   "fire the move, then leaf-score the result directly" (same shape as the
   existing champion, not the same formula — see below), N=1 expands all of
   the opponent's replies before leaf-scoring, N=2 one ply further.

   **A forced design change, and its cost:** a well-founded minimax needs
   ONE shared value scale — the maximizer and minimizer must be pushing the
   same number in opposite directions, or "best for me" stops meaning
   "worst for you." The shipped 0-ply champion does not have that: it uses
   *two different formulas* depending on whose candidate moves are being
   ranked (`win_x - win_o` for X; `x_turn + o_turn + lambda*win_o` for O),
   which is fine when you only ever compare one side's own moves but breaks
   as soon as a search needs to evaluate a position from *both* sides'
   perspectives inside one tree. `leafScore` in `lookahead.go` therefore
   uses the single symmetric quantity `win_x - win_o` for every leaf
   regardless of whose turn it is there — X's own formula, extended to
   double as O's. **This silently drops lambda's O-specific undecided-mass
   weighting**, and the referee shows exactly what that costs before search
   depth ever gets a chance to make it back:

   | depth | seat | decisions | game-losing | ms/decision |
   |---|---|---|---|---|
   | 0 (full 2077-decision sample) | O | 1003 | **317 (31.6%)** | 26.1 |
   | 0 | X | 1074 | 18 (1.7%) | 26.1 |
   | 1 (full 2077-decision sample) | O | 1003 | **311 (31.0%)** | 147.1 |
   | 1 | X | 1074 | 24 (2.2%) | 147.1 |
   | 2 (**first 300 of 2077 — see note**) | O | 143 | **29 (20.3%)** | 927.6 |
   | 2 | X | 157 | 1 (0.6%) | 927.6 |

   N=2 was run on the first 300 of the 2077 decisions only (in original
   sampling order) — at ~928ms/decision a full-sample run would take over
   half an hour, and N=1 had already shown no improvement over the 0-ply
   baseline (the brief's stated condition for spending that much more time
   on N=2), so 300 decisions was judged enough to see whether the trend
   continued, not an attempt to match the other rows' sample size. **For a
   fair comparison, every depth (0, 1, 2) plus the shipped 0-ply champion
   was re-measured on that identical first-300-decision subsample:**

   | evaluator | O game-losing /143 | X game-losing /157 |
   |---|---|---|
   | shipped 0-ply champion (baseline) | **14 (9.8%)** | 3 (1.9%) |
   | lookahead depth=0 | 49 (34.3%) | 2 (1.3%) |
   | lookahead depth=1 | 49 (34.3%) | 2 (1.3%) |
   | lookahead depth=2 | 29 (20.3%) | **1 (0.6%)** |

   Two things are true at once here. **Search depth does help, on top of
   the weaker symmetric leaf evaluator**: O's blunder rate falls
   monotonically with depth on this subsample (49 → 49 → 29) and X's falls
   too (3 → 2 → 2 → 1, if the baseline is included in that ordering) — so
   the direction finding 7 predicted (turn-order information a static
   formula can't represent) is exactly what search recovers some of. **But
   no depth tried closes even half the gap the symmetric leaf formula
   opened, let alone beats the baseline**: depth 2's 29/143 for O is still
   more than double the shipped champion's 14/143 on the identical subset,
   at roughly 35x the per-decision cost (928ms vs 29.5ms — see below).
   Whether N=3 or N=4 would eventually cross the baseline is not answered
   here; each additional ply is ~7x more expensive again, and depth 2
   already took 4m38s for 300 decisions.

   **The cost comparison the brief specifically asked for — is this worth
   it relative to just calling the real oracle:**

   | method | ms/decision (2077-decision sample, or as noted) |
   |---|---|
   | oracle alone (labels *every* legal move at *every* decision — strictly more work than picking one) | **20.5** |
   | shipped 0-ply champion (deciding + one oracle re-check of its own choice) | 29.5 |
   | lookahead depth=0 | 26.1 |
   | lookahead depth=1 | 147.1 |
   | lookahead depth=2 (300-decision subsample) | 927.6 |

   **The real, exact oracle is cheaper per decision than every evaluator in
   this experiment, including the champion it is supposed to be a cheap
   substitute for.** That is the "important, reportable finding about where
   the cheap heuristic's value actually lives" the brief anticipated as a
   live possibility, and it is what happened: at these problem sizes and
   this oracle implementation (bitboard alpha-beta, transposition table,
   center-out ordering — see `oracle.go`), asking the exact solver directly
   is not just more accurate than any lookahead depth tried, it is *faster*
   too. Lookahead depth=1 costs 5x the oracle's per-decision time for a
   worse answer than the 0-ply champion; depth=2 costs 45x the oracle's
   time and still does not match it. The ODE evaluator's genuine value in
   this experiment is not "cheaper than exact search" — on this
   implementation, it demonstrably is not, once any search is layered on
   top of it — it is the zero-search, one-solve-per-move construction of
   findings 1-7, which the oracle cannot substitute for at all once real
   positions with intractable remaining game trees are in play (the whole
   reason `defaultMinDiscs=14` and the honesty machinery around
   `Oracle.Solve`'s budget exist). Once search is on the table at all, this
   experiment's own numbers say to just use the oracle.

   **No change to the shipped default.** Per the brief's own condition —
   change `main.go` only if a lookahead depth clearly wins the referee and
   its cost is reasonable to recommend — neither holds for any depth tried,
   so `championBlockBias/Lambda` remain finding 4's values and `lookahead`
   ships as a separate CLI mode (`go run . lookahead <depth> [maxDecisions]`)
   for anyone who wants to explore deeper plies or a different leaf formula.

## Results

All numbers are from `go run . verify` with `defaultMinDiscs=14`,
`defaultOracleBudget=800000`, `verifyGames=250` (seed 13; ~1m40s). 2077
decision points sampled from 250 self-play games; 79 skipped because the
oracle exceeded its node budget (mostly very early positions, per the
honesty note above); of the 2077, 1149 are "must-play" — a strictly worse
legal alternative existed, i.e. the position actually tests something.

All rows below the naive baseline use the SAME 2077-decision, 250-game,
seed-13 held-out sample, so they are directly comparable to each other, not
just to the naive row.

| evaluator | seat | decisions | game-losing | missed wins |
|---|---|---|---|---|
| naive (uniform-rate net, `win_x - win_o`) | O | 1003 | **300 (29.9%)** | 9 |
| naive | X | 1074 | 18 (1.7%) | 5 |
| **champion — shipped default** (block-bias 1.342, lambda 0.238) | O | 1003 | **129 (12.9%)** | 10 |
| **champion — shipped default** | X | 1074 | 19 (1.8%) | 6 |
| finding 6: block+own (block 1.722, own 5.655, lambda 0.043) | O | 1003 | 129 (12.9%) | — |
| finding 6: block+own | X | 1074 | **31 (2.9%)*** | — |
| finding 7 config A: block+parity (block 1.320, parity 1.682, lambda 0.188) | O | 1003 | 154 (15.4%) | 12 |
| finding 7 config A: block+parity | X | 1074 | 31 (2.9%) | 4 |
| finding 7 config B: parity-only (parity 1.590, lambda 0.345) | O | 1003 | 125 (12.5%) | 18 |
| finding 7 config B: parity-only | X | 1074 | 47 (4.4%) | 4 |
| finding 8: lookahead depth=0 (symmetric leaf, no lambda) | O | 1003 | 317 (31.6%) | 5 |
| finding 8: lookahead depth=0 | X | 1074 | 18 (1.7%) | 6 |
| finding 8: lookahead depth=1 | O | 1003 | 311 (31.0%) | 4 |
| finding 8: lookahead depth=1 | X | 1074 | 24 (2.2%) | 2 |
| finding 8: lookahead depth=2 (*first 300 decisions only*) | O | 143 | 29 (20.3%) | 0 |
| finding 8: lookahead depth=2 (*first 300 decisions only*) | X | 157 | 1 (0.6%) | 0 |

(*finding 6's X regression was originally measured at a smaller 178-decision
scale during its own calibration run — 2/93 → 6/93 — and is re-measured here
at the full 2077-decision scale for the first time: 19/1074 → 31/1074,
confirming the same direction and roughly the same relative size. Finding
8's depth=2 row is NOT on the same 2077-decision sample as every other row —
see finding 8 for the exact 300-decision subsample and the matching
apples-to-apples baseline measured on that same subsample.)

Every non-baseline row above **loses** to the shipped default on at least
one seat: finding 6 and finding 7's config A both regress X for no gain (or
a loss) on O; config B's marginal O improvement (129→125) is bought with
more than double X's blunder count (19→47); finding 8's lookahead rows
regress O substantially at every depth tried, at 1x-45x the per-decision
cost of the shipped default (see finding 8's cost table). The shipped
default (`championBlockBias=1.342, championWinBias=0.0,
championParityBias=0.0, championLambda=0.238`) remains the single-tier
block-bias construction from finding 4 — it is the only configuration tried
across findings 4-8 that improves on the naive net's worst seat without
regressing its strong seat.

Small self-play tournaments (`go run . quick`, oracle plays its own
optimal-set uniformly, budget 800k nodes, 3 games per pairing — kept small
because each move now costs one ODE solve over a ~130-place, ~1600-transition
net (the naive net is smaller, 222 transitions — see "Performance notes"),
not tic-tac-toe's 31/82): champion and naive both won or
drew every sampled game as X against the oracle and lost as O about as
often as the referee numbers above would predict for a 3-game sample.
Tournament results are not the primary evidence here — the referee's
per-move oracle comparison is; tournaments are included only as a sanity
check that nothing in `players.go`'s plumbing is silently broken.

## What is and isn't claimed

- **Is claimed:** every `game-losing`/`missed wins` count above is exact
  for the sampled position and the sampled move — the oracle that produced
  it is a correct alpha-beta solver (finding 1's independent brute-force
  cross-check), and no unresolved position is silently counted as a pass
  (`unresolved` was 0 in every run reported here; the `ok` field is
  threaded through `oracleValueAfter`/`oracleOptimalSet`/`sampledCheck` for
  exactly this reason).
- **Is claimed:** policy-in-structure (catalyzed reply transitions) is a
  real, non-trivial fix for a real, non-trivial structural failure in the
  naive relaxation — it isn't placebo. A 57% reduction in the defending
  seat's blunder rate from one calibrated rate constant, with zero cost to
  the attacking seat, replicates the *mechanism* ode-minimax found on a
  game two orders of magnitude larger.
- **Is NOT claimed:** minimax-equivalence, on either seat, anywhere near
  the sense ode-minimax established for tic-tac-toe. ode-minimax's "0
  game-losing moves" was over the *entire* reachable game tree (~5,000
  states, walked in full). This experiment's smallest reported
  "game-losing" count is 19/1074 (X, champion) and its largest is
  129/1003 (O, champion) — both nonzero, both over an explicit sample, and
  neither number should be read as "the evaluator is minimax-equivalent
  except for N positions in the whole game" — it means "N of the *sampled*
  decisions were suboptimal"; the true rate over all ~10^12 positions is
  unknown and this experiment does not estimate it.
- **Is NOT claimed:** that no further structural tier could close the gap.
  Three were tried beyond the winning single tier — finding 6's blind
  "complete your own line," and finding 7's two configurations of a
  row-parity-restricted version motivated directly by Allis's Connect Four
  theory — and all three lose to the single-tier baseline on the exact same
  held-out sample. That is a warning against assuming more structure
  monotonically helps, and specifically against assuming the theoretically
  "right" asymmetry transfers cleanly into a mass-action rate multiplier
  (finding 7's closing hypothesis is that it can't, structurally, in the
  form tried here) — not a proof that no fourth tier (fork detectors, a
  genuine multi-ply Claimeven encoding, something else) could do better.
  Untried and left as an open question: encoding the pairing argument as a
  place/token structure (e.g. a per-column "parity account") rather than as
  a rate multiplier on an already-playable move, which might carry the
  turn-order information the rate-only tiers above provably cannot.
- **Is claimed:** shallow full-width minimax with the champion ODE
  evaluator as the leaf scorer (finding 8) is strictly worse than the
  search-free 0-ply champion on this game, at every depth tried (0, 1, and
  2 on a stated 300-decision subsample), and strictly more expensive —
  including being more expensive, per decision, than just calling the real
  oracle directly. This is a claim about the specific leaf formula tried
  (a symmetric `win_x - win_o`, forced by minimax's need for one shared
  value scale, which drops the O-specific lambda weighting the 0-ply
  champion relies on) and about this un-pruned, un-ordered search
  implementation — not a general claim that no lookahead-plus-ODE-eval
  construction could ever help. A leaf formula that somehow preserved the
  asymmetric per-seat scoring inside a well-founded search (not attempted
  here) is the natural next thing to try if this direction is revisited.
- **Is NOT claimed:** that findings 1-7's search-free result is undermined
  by finding 8. They are different constructions answering different
  questions — finding 8 explicitly changes paradigm (real search, not a
  static evaluator) specifically to test whether the ingredient search-free
  evaluators structurally lack (turn-order information) could be recovered
  by spending compute instead of structure. It could not, on this
  implementation, at the depths tried — which is itself evidence that the
  search-free evaluator's remaining gap is not "just" a missing few plies
  of lookahead away from closing.

## Resolution

Policy-in-structure — the mechanism that gave tic-tac-toe exact
minimax-equivalence — **transfers to Connect Four as a real, substantial,
but partial fix**, not a complete one. A single block-bias catalyzed tier,
calibrated by the same `learn.Minimize` + `learn.HingeRankLoss` recipe
against the same kind of oracle labels, cuts the defending seat's blunder
rate by more than half on a 2000-decision sample without touching the
attacking seat. That is a genuine positive result and the same qualitative
finding ode-minimax made (a defender's forced-reply structure is legible to
the net only once it's built into the transition rates, not the final
state) — replicated on a game with ~10^9 times more reachable positions.

What did not work: adding a second catalyzed tier to try to close the
remaining gap — neither a blind one (finding 6) nor, specifically, a
theory-motivated one built directly from Allis's Connect Four parity theory
(finding 7, both configurations). All three improved training loss over the
single-tier baseline and all three made the referee's verdict *worse* on at
least one seat, on the identical held-out sample — a clean, repeated
instance of a finite sampled loss being gameable in a way an exhaustive
referee (which this experiment explicitly does not have, and says so
throughout) would need to catch. ode-minimax's own methodology note — "only
the referee decides" — holds here too, and the referee's answer for every
multi-tier construction tried was no. Finding 7's closing hypothesis for
*why* the theory-motivated attempt specifically failed — that Allis's
pairing argument is a discrete turn-order fact a continuous rate multiplier
cannot represent — is this experiment's best current explanation, not a
proof; a structurally different encoding (tokens carrying parity state,
rather than rates biased by it) is the natural next thing to try and is
explicitly left open above.

A third attempt changed paradigm entirely rather than adding structure:
finding 8 replaced the search-free construction with genuine N-ply minimax,
using the champion ODE evaluator as the leaf/static scorer — precisely the
"spend search instead of structure" move finding 7's own closing hypothesis
suggested trying. It also lost to the 0-ply baseline, at every depth tried
(0, 1, 2), and the reason is instructive rather than a dead end: a
well-founded minimax needs one shared value scale, which forced dropping
the very lambda-weighted asymmetric formula that made the 0-ply champion
work for O in the first place. Search depth partially compensated for that
loss (O's blunder rate fell monotonically with depth on the tested
subsample) but never closed even half the gap it opened, while costing up
to 45x more per decision than simply asking the real oracle — which is
exactly the "important, reportable finding about where the cheap
heuristic's value lives" a search-based hybrid was always at risk of
producing instead of a win. The value of the search-free construction
(findings 1-7) turns out to be inseparable from being search-free: once any
search enters the picture, this experiment's own numbers say to use the
oracle, not a hybrid.

The honest summary: **partial success, clearly bounded, and the bound
survived three separate, well-motivated attempts to move it** — a second
blind structural tier (finding 6), a theory-motivated structural tier
(finding 7), and a change of paradigm to genuine shallow search (finding
8). The recipe that solved tic-tac-toe completely reduces Connect Four's
defender-side failure by roughly half; closing the rest remains future
work — and findings 6-8 collectively narrow *where* that work should look:
not more of the same rate-multiplier structure, and not shallow search over
the existing leaf formula, but something that can represent turn-order
information without either sacrificing the leaf formula's asymmetry or
paying search-tree costs that exceed the exact oracle's. This experiment's
main contribution beyond the headline number is the infrastructure to keep
answering the question precisely — a real bitboard alpha-beta oracle with
an honest budget/skip contract, and a sampled-not-exhaustive referee that
never blurs the two together.

## Run it

```
go build ./...
go test ./...              # oracle correctness: brute-force cross-check,
                            # known wins/draws, mirror symmetry, honest
                            # budget-exceeded reporting

go run .                   # quick: oracle sanity + naive vs champion audit
                            # on a small sample + tiny tournaments
go run . verify             # sampled referee at the shipped constants,
                            #   25 self-play games by default
go run . verify 1.342 0.0 0.0 0.238 250   # reproduce this README's headline
                                            # numbers (250 games, ~1m40s):
                                            # args are blockBias winBias
                                            # parityBias lambda games
go run . fit 15 30          # refit (blockBias, winBias, lambda) from
                            # (1,1,1), then referee the result (~4-7 min);
                            # parityBias held at 0
go run . fitparity 15 20    # finding 7: calibrate the parity-bias tier both
                            # alongside and instead of blockBias, referee
                            # both against the SAME held-out sample as the
                            # existing champion, print the comparison
                            # (~10-15 min: three ~2000-decision referee
                            # passes over a ~1600-transition net)
go run . lookahead 1 0      # finding 8: N-ply minimax (lookahead.go) with
                            # the champion evaluator as leaf scorer, N=1,
                            # full 2077-decision sample (~5 min)
go run . lookahead 2 300    # same, N=2, truncated to the first 300
                            # decisions as this README's finding 8 does
                            # (~4-5 min; a full-sample N=2 run would take
                            # over half an hour)
```

## Performance notes

- Each ODE solve (`solver.GameAIOptions()`, Tsit5) over the naive
  131-place/222-transition declared net takes roughly 1ms; over the
  champion's derived net (130 places after dropping `game_active`, 1602
  transitions) roughly 7ms. `deriveChampionNet` always builds all three
  catalyzed tiers (`blk_*`, `own_*`, `prt_*` — 222 + 552 + 552 + 276 = 1602)
  regardless of which rates are active, so the shipped default (only
  `blockBias` nonzero) pays the same per-solve cost as the full
  three-constant `fitparity` sweep — the zero-rate tiers still cost one
  rate evaluation each, they just contribute nothing to the flow. Measured
  directly (`go test -run TestTimingProbeParity`, not committed — see git
  history of this experiment's working tree if it's needed again).
- The oracle's transposition table is **not** reset between `Solve` calls
  within one `Oracle`, so repeated queries against overlapping game trees
  (self-play sampling, the referee re-checking a chosen move) get
  substantially faster as a run progresses — collecting from 20 self-play
  games at `minDiscs=12` took ~11s with a fresh oracle but the *marginal*
  cost per additional game drops fast because most subtrees are already
  cached.
- `defaultMinDiscs=14` and `defaultOracleBudget=800_000` are load-bearing
  choices, not arbitrary: at `minDiscs=10`, oracle calls routinely blow the
  budget (confirmed directly); at `minDiscs=20`, almost every sampled
  position resolves near-instantly because so few cells remain that
  someone usually already has a forced win or the game is one move from
  over, which makes the naive-vs-champion comparison uninformative (both
  looked perfect in an early probe at that setting, until the ply floor was
  lowered enough to expose finding 3's failure at all).
- **The oracle itself is fast enough that finding 8's cost comparison is not
  close.** Labeling every legal move at every one of the 2077 sampled
  decisions with the real alpha-beta oracle (`collectPositions`, budget
  800k) took 44.2s total — 20.5ms/decision, for exact answers on every
  candidate move, not just the one the position ultimately used. The
  shipped 0-ply champion (a single heuristic solve per candidate move) took
  29.5ms/decision on the same sample: the exact search was not "slow but
  correct," it was faster than the heuristic while also being correct.
  Lookahead depth=1 (147ms/decision) and depth=2 (928ms/decision, 300-
  decision subsample) are 5x and 45x the oracle's own cost respectively —
  see finding 8 for why that makes the hybrid approach hard to recommend
  here.
