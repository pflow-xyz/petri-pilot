# Texas Hold'em Poker - ODE Strategic Analysis

A custom frontend for Texas Hold'em poker with real-time strategic value computation using Ordinary Differential Equations (ODE) and Petri net modeling.

## Features

- **Visual Poker Table**: 5-player layout with green felt, cards, and chips
- **ODE-Based Strategy**: Compute expected value for every action using pflow.xyz solver
- **Real-Time Analysis**: Strategic values update as game progresses
- **Heatmap Overlay**: Color-coded action recommendations
- **Event History**: Track all game events
- **Responsive Design**: Works on desktop and mobile

## Architecture

This frontend follows the same ODE integration pattern as the tic-tac-toe implementation:

### ODE Strategic Analysis Flow

1. **Build Petri Net Model** (`buildPokerODEPetriNet`)
   - Creates a Petri net representing the game state
   - Models active/folded players
   - Tracks pot value
   - Represents hypothetical actions

2. **Solve ODE** (`solveODE`)
   - Uses pflow.xyz Tsit5 ODE solver
   - Simulates game progression over time
   - Computes final state probabilities

3. **Compute Expected Values** (`runODESimulation`)
   - For each available action (fold/check/call/raise)
   - Build Petri net with that action applied
   - Solve ODE to get expected pot value
   - Return strategic values for all actions

### Files

- **`index.html`** - Main interface with poker table layout
- **`main.js`** - Game logic, API integration, ODE computation
- **`styles.css`** - Poker table styling (green felt, cards, chips)
- **`cards.js`** - Card rendering utilities (suits, ranks, colors)

## Usage

### Local Development

1. Start the Texas Hold'em backend:
   ```bash
   cd generated/texas-holdem
   go run *.go
   ```

2. Open the frontend:
   ```bash
   # Navigate to http://localhost:8080/frontends/texas-holdem/
   ```

### Game Flow

1. **Create Game**: Click "New Game" to create a game instance
2. **Start Hand**: Click "Start Hand" to begin a poker hand
3. **View ODE Analysis**: Strategic values appear for each available action
4. **Make Decisions**: Click action buttons (fold/check/call/raise)
5. **Toggle Heatmap**: View full strategic analysis overlay

## ODE Solver Configuration

The solver uses the following parameters (configurable in `main.js`):

```javascript
let solverParams = {
  tspan: 2.0,      // Simulation time span
  dt: 0.2,         // Time step
  adaptive: false, // Use fixed time step
  abstol: 1e-4,    // Absolute tolerance
  reltol: 1e-3     // Relative tolerance
}
```

## API Integration

The frontend integrates with the generated Texas Hold'em backend API:

### Endpoints Used

- `POST /api/texasholdem` - Create new game
- `POST /api/start_hand` - Start a hand
- `POST /api/deal_preflop` - Deal hole cards
- `POST /api/deal_flop` - Deal flop (3 community cards)
- `POST /api/deal_turn` - Deal turn (4th community card)
- `POST /api/deal_river` - Deal river (5th community card)
- `POST /api/p{0-4}_fold` - Player folds
- `POST /api/p{0-4}_check` - Player checks
- `POST /api/p{0-4}_call` - Player calls
- `POST /api/p{0-4}_raise` - Player raises

### Response Format

```json
{
  "success": true,
  "aggregate_id": "uuid",
  "version": 1,
  "state": {
    "waiting": 0,
    "preflop": 1,
    "p0_turn": 1,
    "p0_active": 1,
    ...
  },
  "enabled_transitions": ["p0_fold", "p0_check", "p0_call", "p0_raise"]
}
```

## ODE Mode Toggle

Switch between local JavaScript ODE solver and backend Go API:

- **Local Mode** (default): Runs ODE solver in browser using pflow.xyz library
- **API Mode**: Calls backend `/api/heatmap` endpoint for ODE computation

Toggle via the "Toggle ODE" button or console:
```javascript
window.setODEMode(true)  // Local
window.setODEMode(false) // API
```

## Testing

### Playwright E2E Tests

```bash
cd e2e
npm test -- texas-holdem
```

Tests cover:
- Game creation and initialization
- ODE integration and Petri net building
- UI interactions (buttons, overlays)
- Responsive design

### Manual Testing

1. Create game and start hand
2. Verify ODE values appear for each action
3. Check heatmap overlay shows color-coded recommendations
4. Test full hand progression (preflop → flop → turn → river → showdown)
5. Verify event history updates

## Console API

Exposed for debugging:

```javascript
// Game state
window.gameState

// ODE functions
window.runODESimulation()
window.buildPokerODEPetriNet(gameState, action)
window.solveODE(model)

// Action execution
window.performAction(transitionId, amount)
```

## Strategic Analysis Details

### Expected Value Calculation

For each action, the ODE solver computes:

1. **Hypothetical State**: Apply action to current game state
2. **Petri Net Model**: Build model with updated state
3. **ODE Solution**: Simulate game progression
4. **Final State**: Extract win probabilities from final marking
5. **Expected Value**: Compute from pot value and win probability

### Action Recommendations

- **Positive EV** (green): Expected to win money
- **Neutral EV** (yellow): Break-even
- **Negative EV** (red): Expected to lose money

The action with highest EV is highlighted as "recommended".

## Dependencies

- **pflow.xyz Solver**: ODE solver for Petri nets
  ```html
  <script type="module" src="https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@1.11.0/public/petri-solver.js"></script>
  ```

## Comparison with Tic-Tac-Toe

Both implementations share the same ODE integration pattern:

| Feature | Tic-Tac-Toe | Texas Hold'em |
|---------|-------------|---------------|
| **State Model** | 3×3 board | 5 players + cards + pot |
| **Petri Net Places** | 30 (positions + history + wins) | ~100+ (players + cards + bets) |
| **ODE Heatmap** | Cell strategic values | Action expected values |
| **Turns** | X/O alternating | Multiple players, round-based |
| **Terminal States** | Win/Draw | Win/Fold patterns |
| **Solver** | Tsit5, tspan=2.0 | Tsit5, tspan=2.0 |

## Performance Notes

- ODE computation runs asynchronously to avoid blocking UI
- Results cached and only recomputed when game state changes
- Solver typically completes in < 500ms for typical game states

## Future Enhancements

- [ ] Hand strength evaluation display
- [ ] Pot odds calculator
- [ ] Historical hand replay
- [ ] Multiple simultaneous games
- [ ] AI opponent using ODE recommendations
- [ ] Advanced Petri net model with card probabilities
- [ ] Monte Carlo simulation integration

## References

- Tic-Tac-Toe implementation: `frontends/tic-tac-toe/`
- Backend API: `generated/texas-holdem/README.md`
- pflow.xyz Solver: https://github.com/pflow-xyz/pflow-xyz
- Petri Nets: https://en.wikipedia.org/wiki/Petri_net
- ODE Solvers: https://en.wikipedia.org/wiki/Ordinary_differential_equation
