# d-hazard: ablation vs minimax over one tic-tac-toe net

Can a continuous relaxation of a declared game net pick moves? This
experiment holds every candidate evaluator to the same referee — exact
minimax over the net's own firing rule — and measures where each one's
honesty runs out. The name is the finding: the **d-hazard** net, where the
draw is modelled as a hazard (`game_active → tie`) instead of a move
counter, is the structurally correct form for continuous evaluation.

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
```

## Results (100 games per matchup, seeded; minimax randomizes over its optimal set)

| evaluator (as O vs minimax X) | draws | losses |
|---|---|---|
| eliminate (rate-zero ablation), any objective | few | most |
| play (fire move, solve, `win_x − win_o`) | 93 | 7 |
| play, draw pays mover's opponent ("B") | 15 | 84 |
| play, draw neutral ("C") | 93 | 7 |
| play on d-hazard net, O maximizes `tie + win_o` ("D") | 93 | 7 |

As X, every play-scoring variant draws all 100. Elimination is an
importance measure, not a move picker — it never survives the referee.

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
4. **The counter draw is unexpressible in the relaxation.** The weight-9
   arc's rate is the binomial `C(move_tokens, 9)` — identically zero below
   9 — and detector leakage on fractional lines starves the counter, so it
   plateaus short and tie mass is exactly 0 at every horizon. The empty
   board, truly mostly a draw, evaluates as if draws don't exist.
5. **The d-hazard form fixes that structurally.** `game_active → tie`
   drains competitively against the win detectors: the empty board shows
   ~0.84 tie mass, must-block fattens from a hundredths margin to a wide
   one, and the bare tie coordinate picks the block even at rate 720 —
   "maximize the chance the game ends undecided" is the defender's true
   objective, finally measurable.
6. **The fork is information-theoretically out of reach.** At the
   double-corner trap the losing corners *dominate* the optimal moves on
   all three final-state coordinates (lower `win_x`, higher `win_o`,
   higher `tie`), at every rate configuration tried. No monotone scoring
   function over the final state can prefer the right move — the value of
   the fork defense lives only in move order, which the flow integral
   erases. Hence the stable 7% loss floor for every static evaluator.

## Open question

Can a further **structural adjustment** close the last 7% — make some
ODE-evaluable net rank-equivalent to minimax? The dominance result says
any such net must add *places whose final mass carries sequencing
information* (e.g. per-line threat/tempo places that charge a position for
threats the opponent completes first); rescoring the existing coordinates
is proven insufficient. Until then the resolved hierarchy stands:
incidence for the prior, search for the decision (prior-ordered alpha-beta
proves the same answer at ~2.7× fewer nodes), play-scoring when search is
infeasible, d-hazard as the evaluation form of the draw, elimination as a
diagnostic only.
