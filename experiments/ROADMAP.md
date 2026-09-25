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
