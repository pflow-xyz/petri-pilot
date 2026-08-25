# d-hazard: ablation vs minimax over one tic-tac-toe net

Can a continuous relaxation of a declared game net pick moves? This
experiment holds every candidate evaluator to the same referee — exact
minimax over the net's own firing rule — and measures where each one's
honesty runs out. The name is the finding that started it: the
**d-hazard** net, where the draw is modelled as a hazard
(`game_active → tie`) instead of a move counter, is the structurally
correct form for continuous evaluation. The experiment's answer to its
own open question: with two further structural additions — win-bias and
block-bias play transitions — the ODE evaluator is **minimax-equivalent
on both seats, exhaustively verified**.

The net is the shipped `sim` tic-tac-toe model (catalytic win detectors,
`game_active`, `move_tokens`, weight-9 draw). The discrete game — referee,
tournaments, minimax — always runs the exact counter semantics; only the
ODE *evaluation* net varies.

## Run it

```
make quick     # audits + 1 seeded game per pairing (~1s)
make tie       # (win_x, win_o, tie) per candidate, both draw wirings, rates 1 and 720
make sweep     # does any (horizon, win-rate, draw-rate) fix the tactics? (27 cells)
make play100   # 100-game tournaments vs minimax, all variants
go run . tempo             # structural variants: coordinate tables + scorer scan
go run . biasscan          # one-knob block-bias window vs three audits
go run . bias2d            # two-knob (winBias x blockBias) vs four audits
go run . tempo100 policy 2 6   # tournament for the policy-bias evaluator
go run . verify 2 6 1.8    # exhaustive referee: every opponent line, both seats
go run . trace bias 6      # replay losses, report the first off-optimal move
```

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
this directory is where to answer it.
