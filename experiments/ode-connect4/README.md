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
129/1003) under a single calibrated bias tier; four further attempts to
close the remaining gap — a second blind structural tier, a
theory-motivated RATE tier built from Victor Allis's actual Connect Four
parity theory, a full change of paradigm to genuine shallow minimax search
with the ODE evaluator as the leaf scorer, and a theory-motivated
STRUCTURAL (place/token) tier built from the same parity theory — all fail
to reliably beat that single-tier result, each for a distinct, characterized
reason (the last one is real but not reliable — see finding 9). A
robustness gate applied to that last, real-but-unreliable structural tier
(finding 10: 7 independently-seeded fits, refereed against a fixed primary
sample and gated against a second, independent one) confirms the
unreliability is the norm, not a one-off, and would have caught the single
fit that ever looked like a win before it was reported as one. That
plateau, and the five separate ways of trying and failing to move it, is
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

9. **The untried option the README explicitly left open — a structural,
   place-based encoding of Claimeven parity instead of a rate multiplier —
   was built and tested (ROADMAP "Phase 1a"), and it is genuinely not
   placebo: it moves the referee by a large margin in EITHER direction
   depending on which self-play sample calibrated it, which is itself the
   finding.** Finding 7's closing hypothesis was that `prt_*` fails because
   a rate multiplier can only reward a play that completes a
   favorable-parity line *right now*, with no memory of anything earlier in
   the column. `champion.go`'s new `clm_*` tier fixes exactly that gap
   structurally: 14 new places (`claim_<side>_<col>`, one per side per
   column) accumulate real token mass over the flow integral — every
   declared play transition landing on a row that favors that side gets one
   extra, *fixed-rate* (never calibrated) output arc depositing into that
   column's claim place, so favorable-parity moves earlier in the
   continuation literally leave more mass for `clm_*`'s own catalyzed
   copies (same triples as `prt_*`, i.e. own-3-cells-on-the-line, PLUS a
   read of that cell's column's claim account) to multiply on later. The
   deposit is wired *after* every catalyzed-copy tier is built in
   `deriveChampionNet` — `AddCatalyzedCopy` only copies a source
   transition's output arcs at call time, so `clm_*`'s own copies cannot
   retroactively inherit an arc into the very place they read, which is
   what keeps this a one-directional information flow (earlier deposit →
   later read) rather than a transition catalyzing its own supply into
   runaway self-reinforcement. This is topology carrying information across
   the continuation, not a coefficient computed from the instantaneous
   marking — the thing the brief asked for and finding 7 said was untried.

   **Calibrated exactly per the brief: `learn.Minimize` + `learn.HingeRankLoss`,
   two configurations mirroring finding 7's (alongside blockBias, replacing
   it), refereed on the identical 2077-decision/250-game/seed-13 held-out
   sample as every other finding** (`go run . fitclaim 15 20`; CLI doc in
   `main.go`, fit functions in `fit.go`, `go run . verifyclaim
   <blockBias> <claimBias> <lambda> [games]` is the equivalent verify
   invocation the brief asked for). First run, 15 self-play games / 105
   training positions / 20 iterations:

   | config | fitted | train loss | O game-losing | X game-losing |
   |---|---|---|---|---|
   | baseline (same sample, this run) | block 1.342, lambda 0.238 | — | 128/1003 (12.8%) | 19/1074 (1.8%) |
   | A: `clm_*` alongside `blk_*` | block 1.258, claim 2.202, lambda 0.259 | 0.8126 | **122/1003 (12.2%)** | 20/1074 (1.9%) |
   | B: `clm_*` replacing `blk_*` | claim 2.059, lambda 0.434 | 0.7413 | 123/1003 (12.3%) | 23/1074 (2.1%) |

   Config A beats the same-sample baseline on O (128→122, a 4.7% relative
   reduction) at a cost of exactly **+1** decision on X (19→20) — nowhere
   near finding 6/7's regression scale (+12 to +28 decisions, 63%-147%
   relative). Config B's O improvement is comparable but X's cost is larger
   (19→23, four decisions) — the same "alongside beats replacing" pattern
   finding 7 found for the rate-only `prt_*` tier repeats for the
   structural version, at this fit. Config A's numbers were independently
   reproduced — not just re-read from the same fit run — via `go run .
   verifyclaim 1.258 2.202 0.259 250` as a second, separate process: O
   123/1003 (one decision off config A's own internal re-check — a real,
   if small, per-process nondeterminism: identical constants, identical
   seed, different Go process, different answer on one close call, most
   likely map-iteration order feeding into floating-point summation order
   somewhere in the solve path), X 20/1074 (exact match).

   **A second, larger calibration run — done specifically because finding
   5 established that this repo does not trust one fit — does NOT confirm
   config A's win, and lands on the opposite side of the trade-off; config
   B, by contrast, replicates its direction (if not its exact margin).**
   20 self-play games / 160 training positions / 30 iterations (`go run .
   fitclaim 20 30`) fits materially different points than run 1 for BOTH
   configs — not "the same point to three decimals" the way finding 5's
   longer run reproduced the single-tier champion:

   | config | fitted (fit 2) | train loss | O game-losing | X game-losing |
   |---|---|---|---|---|
   | baseline (same sample, this run) | block 1.342, lambda 0.238 | — | 128/1003 (12.8%) | 19/1074 (1.8%) |
   | A: `clm_*` alongside `blk_*` | block 1.395, claim 1.845, lambda 0.146 | 0.8879 | **137/1003 (13.7%)** | 16/1074 (1.5%) |
   | B: `clm_*` replacing `blk_*` | claim 1.830, lambda 0.440 | 1.1594 | 127/1003 (12.7%) | 21/1074 (2.0%) |

   Config A flips sign entirely: fit 1 bought O -6/X +1, fit 2 buys O +9/X
   -3 — the SAME configuration, refereed on the SAME held-out sample, wins
   on O in one fit and loses on O in the other. Config B does not flip:
   both fits buy a small O improvement (-5, then -1) for a small X cost
   (+4, then +2) — consistent in *direction* across two independently fit
   training samples, unlike config A, even though neither fit's margin is
   large enough on its own to call a clean win.

   **Reading all four fits together, not any one of them alone, is the
   honest conclusion.** The claim-account tier is clearly not inert — four
   different fits (two configs × two training samples) pull its free
   constants to different points, and those points produce real,
   non-trivial movement on a 2077-decision held-out referee in every case.
   A placebo tier (one whose catalyst carries no real signal) would not do
   that; it would sit near the baseline regardless of which sample fit it.
   So the structural mechanism finding 7 asked for is real, and config B's
   direction (small, consistent O gain, small, consistent X cost) is the
   more trustworthy read of the two configurations tried. But neither
   configuration clears this experiment's own bar on its own: config A is
   unreliable (wins big, then loses), and config B's more reliable direction
   still spends X (+4, then +2 decisions) for less than it buys back on O
   in both fits — the same shape of trade this experiment has already
   rejected once, in finding 7's own config A. The specific recipe used to
   calibrate this tier — 15-20 self-play games, 20-30 Nelder-Mead
   iterations, the same recipe that reliably reproduced finding 4's single
   point for the block-only tier — does not reliably land either
   configuration of `clm_*` in a region that beats the baseline without
   qualification. That is a materially different, weaker claim than "the
   structural fix works," and this experiment reports it as such rather
   than keeping only the most favorable of the four fits. Per the stop
   condition given for this work — O must improve without X rising
   materially, *reliably*, not on a single fit — this does not clear the
   bar, and `championClaimBias` therefore ships at its declared default of
   `0.0`, same as `championWinBias` and `championParityBias` before it.

   **A plausible reason, left as an open question rather than a diagnosis:**
   the claim account's marking always starts at zero for every candidate-move
   ODE solve (`deriveChampionNet`'s comment says this explicitly) — nothing
   seeds it from the REAL moves already played earlier in the actual game,
   only from flow generated during the hypothetical continuation the solver
   integrates from the candidate move forward. That makes the account a
   record of "what the simulated continuation itself does with favorable-parity
   moves," not a record of "what this specific game's history already
   committed" — a real, stated scope gap (see `deriveChampionNet`'s own
   comment) that a from-history-seeded version does not share, and is the
   natural next thing to try if this direction is revisited.

10. **A robustness gate — refereeing every fit on one fixed sample and
    requiring a second, independent sample to agree — answers finding 9's
    open question directly: the instability is not a rare unlucky draw, it
    is close to the norm for this calibration recipe, and the gate would
    have caught the original apparent win before it was ever reported.**
    Finding 9 left two questions open: is the config A sign-flip (O
    122→137 across two same-recipe fits) typical or a one-off, and could a
    stricter calibration discipline rescue a reliable win from the `clm_*`
    tier. Both are answered here by (a) fitting config A on 5 MORE
    independently-seeded training samples (self-play seeds 101, 202, 303,
    404, 505 — 15 games / 20 Nelder-Mead iterations each, the same recipe as
    finding 9's fit 1; none reuses seed 7), (b) refereeing all 7 fitted
    points (the 2 already on record plus the 5 new ones) IN ONE PROCESS
    against the exact same primary held-out sample every other finding in
    this file uses (seed 13, 250 games, 2077 decisions — the identical
    sample `verify`/`verifyclaim` build), so the comparison is genuinely
    apples-to-apples rather than across separate `go run .` invocations (see
    finding 9's own ±1-decision cross-process noise note), and (c) applying
    a robustness gate: any point that appears to beat baseline on the
    primary sample is additionally checked against a SECOND, independent
    held-out sample (seed 29, 250 games, 1816 decisions) it was never fit or
    compared on before, and is only called a validated win if it beats
    baseline on BOTH. `robust.go`'s `runRobustClaim` (`go run . robustclaim
    15 20 250`, ~43 minutes) implements exactly this; `beatsBaseline` in
    that file states the acceptance rule precisely — O's blunder count
    strictly below the SAME-run baseline's, X's no more than +3 above it,
    the same margin finding 9 itself treated as noise-scale for its own fit
    1 (+1 decision).

    **Full table — all 7 fitted points, the baseline, and the ensemble,
    refereed on the SAME two samples throughout (not cherry-picked):**

    | point | block | claim | lambda | primary O | primary X | beats baseline (primary) | validation O | validation X | validation gate |
    |---|---|---|---|---|---|---|---|---|---|
    | baseline (block-only) | 1.342 | — | 0.238 | 128/1003 (12.8%) | 19/1074 (1.8%) | — | 79/869 (9.1%) | 19/947 (2.0%) | — |
    | finding 9 fit 1 (seed 7, 15g/20i) | 1.258 | 2.202 | 0.259 | **123/1003 (12.3%)** | 20/1074 (1.9%) | **yes** | 89/869 (10.2%) | 13/947 (1.4%) | **FAIL** |
    | finding 9 fit 2 (seed 7, 20g/30i) | 1.395 | 1.845 | 0.146 | 137/1003 (13.7%) | 16/1074 (1.5%) | no | 93/869 (10.7%) | 14/947 (1.5%) | n/a |
    | new fit 1 (seed 101, 15g/20i) | 1.601 | 2.559 | 0.189 | 133/1003 (13.3%) | 18/1074 (1.7%) | no | 93/869 (10.7%) | 14/947 (1.5%) | n/a |
    | new fit 2 (seed 202, 15g/20i) | 1.272 | 1.641 | 0.353 | 147/1003 (14.7%) | 18/1074 (1.7%) | no | 104/869 (12.0%) | 14/947 (1.5%) | n/a |
    | new fit 3 (seed 303, 15g/20i) | 1.392 | 1.509 | 0.386 | 164/1003 (16.4%) | 14/1074 (1.3%) | no | 109/869 (12.5%) | 15/947 (1.6%) | n/a |
    | new fit 4 (seed 404, 15g/20i) | 1.287 | 1.361 | 0.381 | 159/1003 (15.9%) | 18/1074 (1.7%) | no | 107/869 (12.3%) | 14/947 (1.5%) | n/a |
    | new fit 5 (seed 505, 15g/20i) | 1.369 | 1.542 | 0.302 | 144/1003 (14.4%) | 17/1074 (1.6%) | no | 92/869 (10.6%) | 13/947 (1.4%) | n/a |
    | ENSEMBLE (mean of all 7 above) | 1.368 | 1.808 | 0.288 | 136/1003 (13.6%) | 17/1074 (1.6%) | no | 97/869 (11.2%) | 14/947 (1.5%) | n/a |

    Training-sample sizes varied by seed the same way finding 9's own two
    fits did — 62 to 174 resolved positions from the same "15 self-play
    games" recipe, before the oracle-budget skip (finding 9's fits landed at
    105 and 160 from the same nominal recipe at different game counts) —
    further evidence this isn't a controlled ablation, just what a handful
    of random self-play games happens to produce.

    **Reading the table plainly, per the stop condition: 1 of 7 fitted
    points (14%) beats baseline on the primary referee sample; 0 of that 1
    survives the independent validation gate — so 0 of 7 total fits is a
    validated win.** Stronger still: on the validation sample alone, EVERY
    one of the 7 points — including the one that looked like a win on the
    primary sample — has a WORSE O blunder count than baseline's 79/869;
    there is no seed among the 7 tried at which config A's structural
    claim-account tier reliably beats the shipped single-tier champion once
    a second sample is consulted. Finding 9's instability was not an unlucky
    pair of draws: across 7 independently-fit points, only one ever even
    LOOKED like a win, and that one look is itself the noise this gate
    exists to catch.

    **Applied retroactively, this gate would have caught the original
    apparent win before it was ever reported as one.** Finding 9 reported
    config A's fit 1 as beating the same-sample baseline (its own
    contemporaneous numbers: O 122, then 123 via a second, independent
    `verifyclaim` process) without ever checking it against a second
    *sample* — exactly the gap this finding closes. Refereed here against
    seed 29 in the same process, that same fit 1 loses to baseline (89 vs.
    79) — the gate's FAIL verdict is not a new failure mode turning up on
    fresh data, it is the one fit finding 9 came closest to accepting,
    caught by the exact mechanism the brief asked for.

    **Ensembling narrows the spread but does not rescue a win.** The plain
    average of all 7 fitted `(block, claim, lambda)` triples —
    `1.368, 1.808, 0.288` — scores primary O 136, better than the raw fits'
    own mean O-count (≈144) but still worse than baseline's 128, and worse
    than baseline on the validation sample too (97 vs. 79). Averaging the
    constants pulls the wilder fits (O 164, O 159) back toward the pack,
    which is a real stabilizing effect on the *spread*, but the pack itself
    sits on the losing side of baseline: there is no free lunch here — the
    ensemble is a smoothed version of a construction that loses more often
    than it wins, not a way to manufacture a win the individual fits didn't
    have.

    **What this changes and what it doesn't.** `championClaimBias` remains
    `0.0`, exactly as finding 9 left it — this finding does not reopen that
    question, it closes finding 9's own follow-up ("is the instability
    typical?") with a much larger sample (7 fits instead of 2) and answers
    "can a robustness-checked discipline rescue a win?" with a clear no, at
    this calibration recipe (15 games / 20 Nelder-Mead iterations from
    `(1,1,1)`). It does NOT show the `clm_*` mechanism is placebo — the 7
    fitted points still land in visibly different places (claimBias ranges
    1.36 to 2.56, lambda 0.15 to 0.39) and produce a real spread of
    non-baseline referee outcomes, consistent with finding 9's own "not
    placebo" reading — it shows that THIS recipe, even with 7 independent
    draws and an averaging fallback, does not reliably find a point in that
    space that beats the single-tier baseline. Whether a larger training
    sample per fit (more self-play games, more Nelder-Mead iterations)
    would stabilize the fit enough to clear this bar is the natural next
    thing to try, and is a cheaper, more targeted experiment than either
    abandoning the tier outright or trusting a single favorable fit again.

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
| finding 9, fit 1: `clm_*` alongside `blk_*` (block 1.258, claim 2.202, lambda 0.259) † | O | 1003 | **122 (12.2%)** | 14 |
| finding 9, fit 1: `clm_*` alongside `blk_*` † | X | 1074 | 20 (1.9%) | 6 |
| finding 9, fit 1: `clm_*` replacing `blk_*` (claim 2.059, lambda 0.434) † | O | 1003 | 123 (12.3%) | 17 |
| finding 9, fit 1: `clm_*` replacing `blk_*` † | X | 1074 | 23 (2.1%) | 6 |
| finding 9, fit 2 (20 games/30 iters): `clm_*` alongside `blk_*` (block 1.395, claim 1.845, lambda 0.146) † | O | 1003 | 137 (13.7%) | 14 |
| finding 9, fit 2: `clm_*` alongside `blk_*` † | X | 1074 | **16 (1.5%)** | 5 |
| finding 9, fit 2: `clm_*` replacing `blk_*` (claim 1.830, lambda 0.440) † | O | 1003 | 127 (12.7%) | 18 |
| finding 9, fit 2: `clm_*` replacing `blk_*` † | X | 1074 | 21 (2.0%) | 6 |
| finding 10: config A, 7 fits total, best of them (seed 7, same as finding 9 fit 1) ‡ | O | 1003 | 123 (12.3%) | — |
| finding 10: best of 7 ‡ | X | 1074 | 20 (1.9%) | — |
| finding 10: config A, 7 fits total, worst of them (seed 303) ‡ | O | 1003 | 164 (16.4%) | — |
| finding 10: worst of 7 ‡ | X | 1074 | 14 (1.3%) | — |
| finding 10: ENSEMBLE, mean of all 7 fits (block 1.368, claim 1.808, lambda 0.288) ‡ | O | 1003 | 136 (13.6%) | — |
| finding 10: ensemble ‡ | X | 1074 | 17 (1.6%) | — |

(*finding 6's X regression was originally measured at a smaller 178-decision
scale during its own calibration run — 2/93 → 6/93 — and is re-measured here
at the full 2077-decision scale for the first time: 19/1074 → 31/1074,
confirming the same direction and roughly the same relative size. Finding
8's depth=2 row is NOT on the same 2077-decision sample as every other row —
see finding 8 for the exact 300-decision subsample and the matching
apples-to-apples baseline measured on that same subsample. † finding 9's rows
are from separate `go run .` process invocations, not this table's own run —
same seed 13 / 250 games / defaultMinDiscs / defaultOracleBudget, but a
same-sample rerun of the plain baseline inside those processes read
128/1003, one decision off this table's 129/1003, which is the size of the
run-to-run floating-point/map-iteration noise this experiment carries
throughout — see finding 9's own same-sample baseline comparison, not this
table's 129, for the apples-to-apples read. ‡ finding 10's rows are also
from a separate process — its own same-run baseline read O 128/1003, X
19/1074, matching this table's naming convention within the same ±1-decision
noise as finding 9's marker above. Finding 10's full table (all 7 fits, not
just the best/worst/ensemble excerpted here) plus the independent-sample
validation gate applied to each is in finding 10 itself — of the 7, only
the one shown as "best" here ever beat that run's own baseline at all, and
it failed the validation gate.)

Every non-baseline row above **loses** to the shipped default on at least
one seat: finding 6 and finding 7's config A both regress X for no gain (or
a loss) on O; config B's marginal O improvement (129→125) is bought with
more than double X's blunder count (19→47); finding 8's lookahead rows
regress O substantially at every depth tried, at 1x-45x the per-decision
cost of the shipped default (see finding 8's cost table). Finding 9's
structural `clm_*` tier beats the shipped default outright *in one of its
four fits* (config A, fit 1: O 122 vs 128 same-run baseline, X essentially
flat at 20 vs 19) — but the same configuration's second fit lands worse on
O (137) while better on X (16), a sign flip, not a smaller win. Config B
(claim-only) does not flip sign across its two fits (O 123 then 127,
both under baseline's 128; X 23 then 21, both over baseline's 19) but never
buys more on O than it spends on X either. Finding 9 does not change the
shipped default for either reason: config A is not reliable and config B is
reliable but not a net win. Finding 10 then repeated config A on 5 more
independently-seeded fits (7 total) and refereed every one of them against
the same primary sample plus a second, independent validation sample: only
1 of 7 ever beat baseline on the primary sample, that 1 failed against the
independent sample, and the mean-of-7 ensemble also loses to baseline on
both samples — so the instability finding 9 found in 2 fits is confirmed,
not resolved, by a larger and more rigorously refereed one. The shipped
default (`championBlockBias=1.342, championWinBias=0.0,
championParityBias=0.0, championClaimBias=0.0, championLambda=0.238`)
remains the single-tier block-bias construction from finding 4 — it is the
only configuration tried across findings 4-10 that reliably improves on the
naive net's worst seat without
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
  Four were tried beyond the winning single tier — finding 6's blind
  "complete your own line," finding 7's two configurations of a
  row-parity-restricted RATE version motivated directly by Allis's Connect
  Four theory, and finding 9's STRUCTURAL place/token version of the same
  theory (a per-column "parity claim account", the option finding 7 itself
  left open) — and none of the five configurations across those four
  findings reliably beats the single-tier baseline on the exact same
  held-out sample (finding 9's config A wins on one fit and loses on
  another; config B is directionally consistent but never a net win). That
  is a warning against assuming more structure monotonically helps, and
  specifically against assuming the theoretically "right" asymmetry
  transfers cleanly into either a rate multiplier OR a naively-built token
  account — not a proof that no fifth tier (fork detectors, a
  history-seeded claim account rather than one that starts at zero every
  solve, something else) could do better. Finding 9's own closing
  paragraph names the most likely next thing to try: seeding the claim
  account from the real game history already played, not only from the
  hypothetical continuation the solver integrates forward from the
  candidate move.
- **Is claimed:** finding 9's config A instability is not a small-sample
  fluke. Finding 10 refit it 5 more times (7 fits total, 5 different
  self-play seeds beyond the original 2 fits' shared seed 7), refereed
  every fit on one fixed primary sample and gated any apparent win against
  a second, independent sample — only 1 of 7 beat baseline on the primary
  sample, 0 of 7 beat baseline on the independent sample (including that
  1), and the mean-of-7 ensemble point loses to baseline on both. The
  calibration-reliability problem this experiment has been fighting since
  findings 3/4/7/11/16 is not specific to config A's two on-record fits —
  it is what this recipe does most of the time on this tier.
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

A fourth attempt returned to structure, this time building the exact thing
finding 7 named as untried: finding 9's `clm_*` tier replaces the rate-only
parity check with a real place/token account that a play transition
deposits into and a later transition reads, carrying information across the
continuation the way finding 7 said a coefficient on the instantaneous
marking cannot. It is not placebo — four independent fits (two
configurations, two training samples) all produce real, non-trivial,
non-baseline referee numbers — but it is not reliable either: the
"alongside blockBias" configuration wins clearly on one fit and loses
clearly on another, a sign flip on the exact same held-out sample, and the
directionally-consistent "replacing blockBias" configuration never buys
back more on O than it spends on X. Structure alone was not enough here;
this specific structure needed either a better calibration recipe or a
different scope (seeding the account from real game history, not only the
hypothetical continuation) that this experiment leaves as the next thing to
try, exactly as finding 7 left the structural idea itself.

A fifth attempt asked a different kind of question about the fourth: not
whether `clm_*` could win, but whether its one apparent win was trustworthy.
Finding 10 refit config A five more times on independently-seeded training
samples (seven fits total), refereed every one of them — in a single
process, so there is no cross-run noise to explain away — against the same
fixed primary sample plus a second, wholly independent validation sample.
One of seven fits beat baseline on the primary sample; none beat it on the
independent one, including that fit; and averaging all seven fitted points
into one ensemble still lost on both samples. Applied to finding 9's own
two fits before they were ever reported, the same gate would have flagged
the one that looked like a win. The calibration-instability problem this
experiment has carried since findings 3/4/7/11/16 is therefore not an
artifact of an unlucky pair of fits in finding 9 — it is this recipe's
typical behavior on a real, non-placebo structural tier, and a robustness
gate that compares against an independent sample (not just a better
optimizer, and not just more of the same sample) is what it takes to see
that reliably.

The honest summary: **partial success, clearly bounded, and the bound
survived five separate, well-motivated attempts to move it** — a second
blind structural tier (finding 6), a theory-motivated RATE tier (finding
7), a change of paradigm to genuine shallow search (finding 8), a
theory-motivated STRUCTURAL tier (finding 9) that is real but not reliable,
and a robustness-gated re-examination of that same structural tier (finding
10) that confirms the unreliability rather than resolving it.
The recipe that solved tic-tac-toe completely reduces Connect Four's
defender-side failure by roughly half; closing the rest remains future
work — and findings 6-10 collectively narrow *where* that work should look:
not more of the same rate-multiplier structure, not shallow search over the
existing leaf formula, not a first attempt at token-based structure without
also solving the calibration-stability problem that a place with real
memory turns out to introduce (finding 9), and — finding 10's addition —
not this same calibration recipe again, however many extra times it is
re-run, without either a larger per-fit training sample or a genuinely
different scope for the account. This experiment's main contribution beyond
the headline number is the infrastructure to keep answering the question
precisely — a real bitboard alpha-beta oracle with an honest budget/skip
contract, a sampled-not-exhaustive referee that never blurs the two
together, and now a robustness gate that separates a real structural effect
from a fit that merely looks like a win once.

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
go run . fitclaim 15 20     # finding 9: calibrate the STRUCTURAL clm_*
                            # tier (champion.go's column parity-claim
                            # account) both alongside and instead of
                            # blockBias, referee both against the SAME
                            # held-out sample as the existing champion,
                            # print the comparison (~10-25 min depending on
                            # CPU contention: same shape of run as
                            # fitparity, over a slightly larger net —
                            # 222+552+552+276+276 = 1878 transitions)
go run . verifyclaim 1.258 2.202 0.259 250   # finding 9's "equivalent
                                              # verify invocation": args are
                                              # blockBias claimBias lambda
                                              # games (winBias and
                                              # parityBias always 0)
go run . robustclaim 15 20 250   # finding 10: refit config A (clm_*
                                  # alongside blk_*) on 5 more independently-
                                  # seeded training samples, referee all 7
                                  # fits (2 on record + 5 new) plus their
                                  # ensemble average against a fixed primary
                                  # held-out sample (seed 13) AND gate any
                                  # apparent win against a second,
                                  # independent held-out sample (seed 29) —
                                  # args are training games, Nelder-Mead
                                  # iterations, referee games (~40-45 min:
                                  # 5 full fits plus two ~2000-decision
                                  # oracle-labeled samples reused across 9
                                  # referee evaluations). A trailing `n=<k>`
                                  # arg trims to the first k new seeds, for
                                  # a fast smoke test.
```

## Performance notes

- Each ODE solve (`solver.GameAIOptions()`, Tsit5) over the naive
  131-place/222-transition declared net takes roughly 1ms; over the
  three-tier champion's derived net (130 places after dropping
  `game_active`, 1602 transitions) roughly 7ms. `deriveChampionNet` always
  builds all three catalyzed-COPY tiers (`blk_*`, `own_*`, `prt_*` —
  222 + 552 + 552 + 276 = 1602) regardless of which rates are active, so the
  shipped default (only `blockBias` nonzero) pays the same per-solve cost as
  the full three-constant `fitparity` sweep — the zero-rate tiers still cost
  one rate evaluation each, they just contribute nothing to the flow.
  Measured directly (`go test -run TestTimingProbeParity`, not committed —
  see git history of this experiment's working tree if it's needed again).
  Finding 9's `clm_*` tier (276 more copies) plus 14 claim places brings the
  full net to 144 places / 1878 transitions — proportionally larger
  (+17% transitions), not separately re-measured with the same timing
  probe; `go run . fitclaim`'s own wall-clock (~10-25 min for two fits plus
  three held-out referee passes, printed by `time` when backgrounded) is
  the closest apples-to-apples comparison available in this checkout.
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
