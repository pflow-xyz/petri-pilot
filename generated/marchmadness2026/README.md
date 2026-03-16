# ncaa-bracket-model

Petri net + ODE simulation model for NCAA Tournament bracket ranking. Uses [go-pflow](https://github.com/pflow-xyz/go-pflow) to model multi-faceted team strength as token flows through weighted transitions.

Built on Selection Sunday 2026 to fill 25 ESPN Tournament Challenge brackets with varied picks.

## What It Does

Four interconnected models that rank teams by simulating how strength factors interact:

| Model | Approach | What It Shows |
|-------|----------|---------------|
| **Multi-Facet Accumulation** | Each team's offense/defense/record/momentum/depth tokens flow into a ranking pool at weighted rates | Overall power rankings |
| **Head-to-Head Competition** | Teams in the same region drain each other's tokens — stronger teams accumulate | Competitive equilibrium |
| **Weight Sensitivity** | Sweeps each facet weight 5%→45% to find which teams are robust | Ranking stability |
| **Probability Flow** | 100 probability tokens per team flow through R64→Champ with round-dependent win rates | Championship probabilities |

All models use ODE simulation (Tsitouras 5th-order) with token conservation verification.

## Quick Start

```bash
go run main.go
```

Outputs:
- Console rankings for all 4 models
- `model1_accumulation.svg` — strength accumulation curves
- `model2_competition.svg` — competitive dynamics over time
- `model4_championship_flow.svg` — championship probability flow
- `ranking_petri_net.svg` — Petri net structure diagram

## 2026 Results

```
Power Rankings (Multi-Facet ODE):
 1. Duke         43.9  (1-seed)
 2. Arizona      42.9  (1-seed)
 3. Michigan     42.3  (1-seed)
 4. Houston      42.3  (2-seed)
 5. Florida      41.9  (1-seed)
 6. Purdue       41.0  (2-seed)
 7. Iowa State   40.6  (2-seed)
 8. St John's    40.5  (5-seed)
```

Duke was #1 under every weight configuration tested — the most robust pick in the field.

## Honest Assessment

This is a proof-of-concept, not a serious prediction engine. The ODE simulation converges to roughly the same ranking a weighted-average spreadsheet would give. The Petri net formalism adds structural rigor (token conservation, flow analysis) but the inputs are hand-estimated stats on a 0-100 scale.

**What actually worked:** Sensitivity analysis showing Duke's robustness across all weighting schemes. That's a real, actionable signal.

**What's mostly theater:** The continuous ODE dynamics. Basketball tournaments are discrete elimination events, not chemical reactions. Mass-action kinetics (flux = rate × concentration) doesn't model bracket play.

## How to Make This Actually Good (2027 Roadmap)

### 1. Real Data Inputs

The 0-100 eyeballed stats are the biggest weakness. Replace with:

**KenPom** (kenpom.com — $20/year subscription)
- Adjusted Offensive Efficiency (AdjO) — points per 100 possessions, adjusted for opponent
- Adjusted Defensive Efficiency (AdjD)
- Adjusted Tempo
- Strength of Schedule
- Luck rating (how much record over/underperforms efficiency)

**Barttorvik T-Rank** (barttorvik.com — free)
- Similar efficiency metrics, free alternative to KenPom
- Includes "Barthag" — estimated probability of beating average D-I team
- Historical game-by-game results downloadable as CSV

**NCAA NET Rankings** (ncaa.com)
- The committee's actual ranking tool
- Quadrant record (Q1/Q2/Q3/Q4 wins and losses)
- Available via API or scraping

**Evan Miya** (evanmiya.com)
- Player-level Bayesian performance ratings
- Roster quality and depth metrics
- Transfer portal impact scores

**Data pipeline approach:**
```
KenPom CSV → normalize to 0-1 → plug into team stats struct
Barttorvik game logs → compute momentum (last-10 record, conf tourney)
NET quadrant records → compute record quality
Roster data → compute depth (minutes distribution, bench scoring)
```

### 2. Better Model Architecture

**Replace continuous ODE with Monte Carlo bracket simulation:**
- Use logistic regression on (seed difference, efficiency gap) to compute per-game win probability
- Historical data: since 2002, 1-seeds beat 16-seeds 99.4% of the time, 5v12 is 64.5/35.5, etc.
- Simulate 50,000 full brackets, count championship frequency
- The Petri net can still define the bracket structure and flow constraints

**Add a discrete event Petri net mode:**
- Transitions fire discretely (one game at a time) instead of continuous ODE
- Inhibitor arcs enforce "can't play Round 2 until Round 1 is decided"
- Token routing follows actual bracket matchup structure
- go-pflow's reachability analysis can verify bracket integrity (no impossible states)

### 3. Backtesting

Validate against historical tournaments:
- Run the model on 2015-2025 data
- Compare predicted rankings vs actual tournament results
- Measure: did the model's #1 team win it all? Make the Final Four? How often did top-8 predicted teams reach the Elite Eight?
- Tune facet weights to maximize historical accuracy

### 4. Automated Bracket Filling

The ESPN bracket automation (Playwright script) already works. Connect the model output to the bracket filler:
- Model produces ranked win probabilities per matchup
- Generate N brackets with controlled variation (chalk, moderate chaos, max chaos)
- Auto-submit via the existing `clickTeamInUnchecked` Playwright approach

## Architecture

```
main.go
├── Team data (stats struct)
├── Model 1: Multi-facet accumulation Petri net
│   ├── 5 facet places × 16 teams = 80 places
│   ├── 5 evaluation transitions × 16 teams = 80 transitions
│   └── ODE solve → power rankings
├── Model 2: Competition dynamics Petri net
│   ├── 16 team places + eliminated pool
│   ├── Bidirectional clash transitions (rate = win prob)
│   └── ODE solve → competitive equilibrium
├── Model 3: Weight sensitivity sweep
│   └── 5 facets × 5 weight levels = 25 scenarios
└── Model 4: Probability flow Petri net
    ├── 6 round places × 8 teams = 48 places
    ├── Advance/eliminate transitions per round
    └── ODE solve → championship probabilities
```

## Dependencies

- [go-pflow](https://github.com/pflow-xyz/go-pflow) — Petri net library with ODE solver, builder API, SVG visualization

## License

MIT
