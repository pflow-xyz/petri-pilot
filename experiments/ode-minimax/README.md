# ode-minimax: an ODE-evaluated Petri net that plays perfectly

Can a continuous relaxation of a declared game net pick moves? This
experiment holds every candidate evaluator to the same referee — exact
minimax over the net's own firing rule — and measures where each one's
honesty runs out. The answer: with the forced-reply prior declared as
structure (block-bias play transitions) and two fitted scalars, the ODE
evaluator is **minimax-equivalent on both seats, exhaustively verified**.

(Formerly `d-hazard`, after the finding that started it — the draw
modelled as a hazard, `game_active → tie`. Findings 12 and 15 later
removed the draw from the winning net entirely, so the folder is named
for the result instead; the d-hazard *variant* keeps its name below.)

The net is the shipped `sim` tic-tac-toe model (catalytic win detectors,
`game_active`, `move_tokens`, weight-9 draw). The discrete game — referee,
tournaments, minimax — always runs the exact counter semantics; only the
ODE *evaluation* net varies.

## Run it

```
make quick     # tactical audits + 100-game tournaments vs minimax
make verify    # exhaustive referee: every legal opponent line, both seats
make fit       # refit (blockBias, lambda) from (1,1); referee the result
```

The working tree keeps only the champion (`champion.go`), the referee and
fitter (`fit.go`), and the declared game (`model.go`, `players.go`). The
variant zoo the findings below refer to — draw wirings, threat
coordinates, bias scans, incidence weightings — was pruned once the
experiment resolved; it is all recoverable from this branch's history.

The generic halves now live in **go-pflow** and this experiment consumes
them: the champion net is the declared net plus three `derive` transforms
(`DropTransitions`, `DropPlaces`, `AddCatalyzedCopy` ×48), and the fitter
is `learn.Minimize` over `learn.HingeRankLoss`. What stays here is
exactly the game knowledge: the discrete referee, minimax labels, and
the exhaustive walk. The `require` line names the release that ships
them (go-pflow v0.23.1, which also added `Solution.Truncated` — a
solve that exhausts Maxiters short of the horizon now says so instead
of silently answering for an earlier time).

## Results

100-game tournaments (seeded; minimax randomizes over its optimal set):

| evaluator (as O vs minimax X) | draws | losses |
|---|---|---|
| eliminate (rate-zero ablation), any objective | few | most |
| play (fire move, solve, `win_x − win_o`) | 93 | 7 |
| play, draw pays mover's opponent ("B") | 15 | 84 |
| play, draw neutral ("C") | 93 | 7 |
| play on d-hazard net, O maximizes `tie + win_o` ("D") | 93 | 7 |
| D + block-bias 6 or 10 (one knob) | ~half | ~half |
| **D + policy bias (win 2, block 6), O: `tie + 1.8·win_o`** | **100** | **0** |

As X, every play-scoring variant draws all 100. The final evaluator is
stronger than the tournaments can show — see finding 9.

## What was learned, in order

1. **Elimination ≠ play.** Rate-zero ablation measures what having an
   option is worth over the horizon; it mis-ranks forced tactics under
   every objective (own win place, opponent's, difference — all three are
   rank-identical here, because `win_x + win_o + game_active = 1` is a
   P-invariant: offense and denial are the same measurement).
2. **The ODE ranking is the incidence ranking.** Empty-board scores sit at
   4 : 3 : 2 — the live win-line counts. The solver numerically recovers a
   graph property the incidence matrix states exactly and for free.
3. **Detector rates are load-bearing.** At uniform rates play-scoring gets
   the must-block right; at detector rate 720 it flips. A picker whose
   tactical correctness depends on a solver rate choice was never seeing
   the tactic.
4. **The relaxation loses the counter's *threshold*, not its flow**
   *(corrects an earlier version of this finding).* go-pflow's mass-action
   rate is `k · Π(inputs)` with arc weight entering only the
   stoichiometry, so the weight-9 draw arc fires from the first fractional
   move at a rate linear in `move_tokens`, paying 1 tie per 9 counter
   tokens consumed — a mis-calibrated hazard, not a count-to-nine. The
   earlier claim that counter-net tie mass is identically zero was an
   artifact of a seeding bug (net-only places dropped from the solve)
   fixed later the same day; measured correctly, the counter net shows
   tie ≈ 0.82 on the empty board at uniform rates.
5. **The d-hazard form is the honest shape of what the relaxation already
   does.** Since the continuous counter degenerates into a hazard anyway,
   `game_active → tie` says so directly, with sane calibration: the tie
   channel drains competitively against the win detectors, must-block
   fattens from a hundredths margin to a wide one at every rate tried, and
   O's true objective — "the game ends without an X win" — becomes a
   measurable coordinate.
6. **The fork is out of reach for any scoring of the unmodified net.** At
   the double-corner trap the losing corners *dominate* the optimal moves
   on all three final-state coordinates (lower `win_x`, higher `win_o`,
   higher `tie`), at every rate configuration tried. No monotone scoring
   function over that final state can prefer the right move — hence the
   stable 7% loss floor for every static evaluator on the plain net.
7. **Integral threat coordinates inherit the dominance.** Adding per-line
   threat accumulators (2 marks + open third cell, catalytic) and
   line-pair fork detectors gives new final-state coordinates — and at
   the fork they point the wrong way too: the losing corners show *lower*
   `threat_x` and `fork_x` than the optimal edges. Penalizing them
   prefers the losers. Time-integrated exposure is still not sequencing.
8. **One-knob flow shaping trades one tactic for another.** Biasing play
   flow toward blocking (extra play transitions catalyzed by the
   opponent's two marks on the line) passes the fork audit for bias in
   [4, 10] — but the corner-reply audit fails at exactly bias ≥ 4: the
   joint window is empty, and tournaments confirm it (46–49 losses as O,
   all opposite-corner replies to a corner opening). Passing the audit
   set you have is not passing the game: the spotlight positions were
   necessary, never sufficient.
9. **Two-knob policy bias reaches full minimax equivalence.** Add *both*
   biased copies of each play transition — one catalyzed by the mover's
   own two marks on the line (win bias 2), one by the opponent's (block
   bias 6) — over the d-hazard base, and score X by `win_x − win_o`, O by
   `tie + 1.8·win_o`. The exhaustive referee (walk **every** legal
   opponent line, optimal or not; at each of the evaluator's decision
   points, its move must not worsen the exact game value) reports, over
   all distinct reachable decisions: **as X, 96 decisions, 0 value-losing
   moves, 0 missed wins; as O, 309 decisions, 0 and 0.** The evaluator
   never loses a drawn position and converts every won one, against
   arbitrary (not just optimal) opposition. The λ sweep shows the
   safety/greed boundary: λ=1 is safe but misses 40 punishes, λ=2 leaks
   2 losses; λ=1.8 achieves both. The structure carries the tactic —
   finding 6 proves no rescoring of the unmodified net could do this at
   any constants — while the three constants (2, 6, 1.8) are calibration,
   found against the referee, and the windows are narrow (win bias 4
   already breaks the fork; block bias needs [4, 10]).

10. **Static evaluators degrade gracefully with initiative and sharply
    without it.** The seat asymmetry (X perfect on the plain d-hazard
    net, O stuck at 93%) is not an accident of this net — it is tempo
    converting tactic depth. A move ahead, every threat X must answer is
    already on the board: a live detector rate, a depth-1 fact the flow
    measures directly, so greedy threat-maximization is complete for the
    attacker (verified: as X, zero misses at all 96 decision points,
    blunder-punishing included). A move behind, O's decisive danger — the
    fork — does not exist yet at O's decision point; it is assembled two
    plies later out of forced replies, so it is legible only in the
    continuation structure that an order-free flow integral erases. The
    attacker's game is legible in the current marking; the defender's is
    partly legible only in move order. That is also why the working fix
    had to be the policy-biased flow and could not be a rescoring: the
    block/win biases give the trajectory a model of *forced replies* —
    exactly the piece of continuation structure the defender needs and
    the attacker never did.

11. **The constants are discoverable by optimization — and the optimizer
    found a simpler solution than the hand search.** `fit` mode:
    Nelder-Mead in log space over (winBias, blockBias, λ) from a naive
    (1, 1, 1) start, hinge ranking loss against minimax labels on 241
    positions sampled from random self-play (plus the four audits).
    It converges to **(winBias → 0, blockBias 4.3, λ 1.7)** — and that
    triple passes the exhaustive referee with 0 game-losing moves and 0
    missed wins on both seats. Win bias is unnecessary: finding 8's
    "empty one-knob window" was an artifact of scanning at λ = 1 — the
    λ knob is what reconciles the fork with the corner reply, and the
    hand-found (2, 6, 1.8) is just a more complex point of the same
    solution region, reached because the hand search walked a 2-D slice
    of the 3-D space. Two free parameters suffice. Also worth keeping:
    training loss plateaus at 0.10 while every decision is correct —
    hinge margins stay sub-margin at some positions but the argmax is
    right, so the loss overstates failure. The optimizer is
    experiment-local because go-pflow's `learn` package exposes only a
    single-trajectory MSE `Fit`; the upstream refinement, if this
    pattern recurs, is exporting its optimizers behind a generic
    `Minimize(f, x0, opts)`.

12. **The draw structure is not load-bearing — but the declared counter
    is anti-load-bearing.** `fitv` refits (blockBias, λ) per draw
    variant and holds each to the exhaustive referee. Removing the draw
    transition entirely and scoring O on `game_active + λ·win_o` fits to
    (3.68, 1.84) and passes **0/0 on both seats**: the leftover
    `game_active` mass is already the undecided coordinate, and the
    hazard's `tie` was only ever drained `game_active` under its own
    name. Keeping the declared weight-9 counter in the evaluation net,
    however, cannot be rescued by calibration: its linear-rate payment
    into `win_o` (finding 4) pollutes both objectives, and the fit
    (2.13, 1.31) still loses 2 games as O — and 4 as X, the only
    variant to lose on that seat at all. So the relaxation's correct
    treatment of the declared draw is removal or re-routing, never
    faithful inclusion. What the hazard form still buys is margin:
    train loss 0.10 against nodraw's 0.51 — the same argmax with 5×
    the headroom, which is what robustness to rate perturbation is
    made of.

13. **The detector return arcs are load-bearing — by exactly two games.**
    `oneshot`: drop the cell-return arcs so a win detector consumes the
    line it detects (each line pays once; threats self-extinguish as
    they are counted). Refit: (5.66, 1.87), and the referee finds 2
    game-losing moves as O (X stays perfect). The read-loop form keeps
    threat pressure a persistent rate signal instead of a self-consuming
    pulse, and that persistence is worth the last two defensive
    rankings. Close enough to be tempting for diagram legibility; not
    equivalent.
14. **Incidence-weighting the deposits double-counts the prior.**
    Setting each play's cell deposit to the cell's win-line count
    (center 4, corners 3, edges 2) — the incidence prior in the
    stoichiometry — makes everything worse on both detector bases:
    train loss 1.38–1.57 (vs 0.25–0.51), 2 losses + 8–10 missed wins as
    O, and on the looped base even 4 missed wins as X, the only variant
    to degrade that seat. Finding 2 said the flow already recovers the
    incidence ranking from topology; feeding the counts in again
    multiplies them into every detector product — the prior squared,
    not the prior applied. The shared-resource reading ("deposit n, each
    line consumes its own token") fares even worse: normalizing the
    detector rates by the product of their cells' counts (`inc-norm`)
    removes the double-count but crushes the win channels' absolute
    rates, and the referee reports 59 game-losing moves as O and 19 as
    X — the worst variant tried. Mass action has no per-consumer
    shares: a place's value multiplies into every consuming rate at
    once, so out-degree deposits either inflate all the products or,
    normalized, starve them. The third direction — weight-1 deposits
    with detector rates *multiplied* by the count product (`inc-rate`) —
    fails as well: 6 game-losing moves and 22 missed wins as O, the
    worst training surface of any variant (loss 5.3), with λ fitted
    down to ~1.0. A prior the structure already computes must not also
    be written into the weights, the masses, or the rates — the
    topology applies it exactly once, and every duplicate is paid for.

15. **`game_active` is removable too.** With the draw and counter gone,
    the place had two remaining jobs, and both turn out to be duplicates:
    halting is already enforced by the detectors consuming the turn
    tokens, and the undecided coordinate is the surviving turn-token
    mass itself. `noga` — cells, turn tokens, win places, nothing else —
    fits to (2.72, 1.87) with O scoring `x_turn + o_turn + λ·win_o` and
    passes the referee **0/0 on both seats** (train loss 0.62 vs
    minimal's 0.51: slightly thinner margins, same perfect argmax). The
    champion evaluation net is now 31 places (27 board, 2 turn, 2 win)
    and 82 transitions (18 plays, 16 detectors, 48 block-bias copies):
    the board, whose turn it is, who won, and the forced-reply prior.

## Resolution

The open question — can a structural adjustment make some ODE-evaluable
net rank-equivalent to minimax? — is answered **yes** for tic-tac-toe.
The adjustment that works is *policy-in-structure*: extra copies of the
play transitions whose rates are catalyzed by the tactical pattern they
answer (complete your line, block theirs), so the flow integral plays
forward under a win > block > neutral policy instead of a uniform one.
Sequencing enters the trajectory, and the existing final-state
coordinates become sufficient. What did *not* work, and provably cannot:
rescoring the unmodified net (finding 6) and adding time-integrated
threat coordinates (finding 7).

The resolved hierarchy: incidence for the prior, search for the decision
(prior-ordered alpha-beta proves the same answer at ~2.7× fewer nodes),
**policy-biased d-hazard play-scoring where search is infeasible** —
now minimax-equivalent on this game at calibrated constants — plain
d-hazard play-scoring when nothing may be tuned (93% as O, perfect as X),
elimination as a diagnostic only. Whether the policy-bias construction
and its constants transfer to any other game is the next question, and
this directory is where to answer it. Finding 10 supplies the testable
prediction to carry there: a static (flow-integral) evaluator's quality
should track initiative — near-complete for the side attacking, failing
exactly at the defender's trap positions — so the seats to instrument
first in any new game are the ones without the tempo.
