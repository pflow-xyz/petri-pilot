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

## Resolution

This experiment satisfies the “comes close” branch, not the equivalence
branch. The continuous relaxation is a remarkably strong static evaluator for
a declared gravity game, and perfect with the initiative, but the exact
referee rejects the claim that tic-tac-toe's forced-reply topology is a
domain-agnostic recipe. The eight counterexamples are preserved by
`diagnose-naive`; the next iteration should be designed against their shared
future-support structure and then held to the same referee.
