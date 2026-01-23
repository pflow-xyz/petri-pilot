# Tic Tac Toe

Tic-tac-toe game modeled as a Petri net with ODE-based strategic analysis

**Version:** 1.0.0

## Summary

| Element | Count |
|---------|-------|
| Places | 29 |
| Transitions | 19 |
| Arcs | 72 |
| Events | 3 |

## Petri Net Diagram

```mermaid
flowchart LR
    classDef place fill:#e3f2fd,stroke:#1976d2,stroke-width:2px
    classDef transition fill:#333,stroke:#333,color:#fff
    classDef initial fill:#c8e6c9,stroke:#388e3c,stroke-width:2px

    subgraph Board["Board Cells"]
        p00((p00))
        p01((p01))
        p02((p02))
        p10((p10))
        p11((p11))
        p12((p12))
        p20((p20))
        p21((p21))
        p22((p22))
    end

    subgraph Turn["Turn Control"]
        x_turn((x_turn [1]))
        o_turn((o_turn))
    end

    subgraph XMoves["X Moves (sample)"]
        x_play_00[x_play_00]
        x_play_01[x_play_01]
        x_play_02[x_play_02]
    end

    subgraph OMoves["O Moves (sample)"]
        o_play_00[o_play_00]
        o_play_01[o_play_01]
        o_play_02[o_play_02]
    end

    p00 --> x_play_00
    x_turn --> x_play_00
    x_play_00 --> x00
    x_play_00 --> o_turn
    p01 --> x_play_01
    x_turn --> x_play_01
    x_play_01 --> x01
    x_play_01 --> o_turn
    p00 --> o_play_00
    o_turn --> o_play_00
    o_play_00 --> o00
    o_play_00 --> x_turn
    p01 --> o_play_01
    o_turn --> o_play_01
    o_play_01 --> o01
    o_play_01 --> x_turn
```

*Note: Diagram shows sample transitions. Full model has 19 transitions (9 for X, 9 for O, 1 reset).*

## Places

### Empty Cell Places
| ID | Description | Initial |
|----|-------------|--------:|
| `p00` | Cell (0,0) empty | 1 |
| `p01` | Cell (0,1) empty | 1 |
| `p02` | Cell (0,2) empty | 1 |
| `p10` | Cell (1,0) empty | 1 |
| `p11` | Cell (1,1) empty - center | 1 |
| `p12` | Cell (1,2) empty | 1 |
| `p20` | Cell (2,0) empty | 1 |
| `p21` | Cell (2,1) empty | 1 |
| `p22` | Cell (2,2) empty | 1 |

### X Piece Places
| ID | Description | Initial |
|----|-------------|--------:|
| `x00` | X piece at (0,0) | 0 |
| `x01` | X piece at (0,1) | 0 |
| `x02` | X piece at (0,2) | 0 |
| `x10` | X piece at (1,0) | 0 |
| `x11` | X piece at (1,1) | 0 |
| `x12` | X piece at (1,2) | 0 |
| `x20` | X piece at (2,0) | 0 |
| `x21` | X piece at (2,1) | 0 |
| `x22` | X piece at (2,2) | 0 |

### O Piece Places
| ID | Description | Initial |
|----|-------------|--------:|
| `o00` | O piece at (0,0) | 0 |
| `o01` | O piece at (0,1) | 0 |
| `o02` | O piece at (0,2) | 0 |
| `o10` | O piece at (1,0) | 0 |
| `o11` | O piece at (1,1) | 0 |
| `o12` | O piece at (1,2) | 0 |
| `o20` | O piece at (2,0) | 0 |
| `o21` | O piece at (2,1) | 0 |
| `o22` | O piece at (2,2) | 0 |

### Turn Control Places
| ID | Description | Initial |
|----|-------------|--------:|
| `x_turn` | X's turn to play | 1 |
| `o_turn` | O's turn to play | 0 |

## Transitions

| ID | Description |
|----|-------------|
| `x_play_00` | X plays at (0,0) |
| `x_play_01` | X plays at (0,1) |
| `x_play_02` | X plays at (0,2) |
| `x_play_10` | X plays at (1,0) |
| `x_play_11` | X plays at (1,1) - center |
| `x_play_12` | X plays at (1,2) |
| `x_play_20` | X plays at (2,0) |
| `x_play_21` | X plays at (2,1) |
| `x_play_22` | X plays at (2,2) |
| `o_play_00` | O plays at (0,0) |
| `o_play_01` | O plays at (0,1) |
| `o_play_02` | O plays at (0,2) |
| `o_play_10` | O plays at (1,0) |
| `o_play_11` | O plays at (1,1) - center |
| `o_play_12` | O plays at (1,2) |
| `o_play_20` | O plays at (2,0) |
| `o_play_21` | O plays at (2,1) |
| `o_play_22` | O plays at (2,2) |
| `reset` | Reset game to initial state |

## Events

### XPlayed

X placed a piece on the board

| Field | Type | Required |
|-------|------|----------|
| `row` | integer | Yes |
| `col` | integer | Yes |

### OPlayed

O placed a piece on the board

| Field | Type | Required |
|-------|------|----------|
| `row` | integer | Yes |
| `col` | integer | Yes |

### GameReset

Game was reset to initial state

## Strategic Analysis

### Position Values (ODE-derived)

Values derived from Petri net topology using ODE simulation:

| Position | Value | Type | Win Patterns |
|:--------:|------:|------|-------------:|
| `00` | 0.316 | corner | 3 |
| `01` | 0.218 | edge | 2 |
| `02` | 0.316 | corner | 3 |
| `10` | 0.218 | edge | 2 |
| `11` | 0.430 | center | 4 |
| `12` | 0.218 | edge | 2 |
| `20` | 0.316 | corner | 3 |
| `21` | 0.218 | edge | 2 |
| `22` | 0.316 | corner | 3 |

### Win Patterns

8 possible winning lines (indices 0-8 represent board positions):

| # | Pattern | Type |
|:-:|---------|------|
| 1 | [0 1 2] | Top row |
| 2 | [3 4 5] | Middle row |
| 3 | [6 7 8] | Bottom row |
| 4 | [0 3 6] | Left col |
| 5 | [1 4 7] | Center col |
| 6 | [2 5 8] | Right col |
| 7 | [0 4 8] | Main diagonal |
| 8 | [2 4 6] | Anti-diagonal |

### ODE Simulation

- **Description:** Strategic values derived from Petri net ODE simulation
- **Solver:** pflow.xyz
- **Interpretation:** Higher values indicate positions with more winning potential

