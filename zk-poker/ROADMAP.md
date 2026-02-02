# ZK Poker Roadmap

Zero-knowledge Texas Hold'em for 5 players with full game provability.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        ZK Poker Stack                           │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (ES Modules)                                          │
│  - Wallet integration (commitment signing)                      │
│  - Real-time game state                                         │
│  - Proof submission UI                                          │
├─────────────────────────────────────────────────────────────────┤
│  Backend (Go + GraphQL)                                         │
│  - Game state management                                        │
│  - Proof verification                                           │
│  - Event sourcing                                                │
├─────────────────────────────────────────────────────────────────┤
│  ZK Circuits (gnark)                                            │
│  - ActionCircuit: valid betting actions                         │
│  - DealCircuit: cards match deck commitment                     │
│  - HoleCommitmentCircuit: reveal matches commitment             │
│  - ShowdownCircuit: correct winner determination                │
├─────────────────────────────────────────────────────────────────┤
│  Smart Contracts (optional, for on-chain settlement)            │
│  - Groth16 verifier                                             │
│  - Pot/chip escrow                                              │
│  - Dispute resolution                                           │
└─────────────────────────────────────────────────────────────────┘
```

## Phase 1: Core Circuits (Current)

**Goal**: Prove all game actions are valid without revealing hidden info.

### 1.1 State Schema ✅
- [x] Define place indices (deck, dealt, holes, community, player states)
- [x] Define transition indices (deal, fold, check, call, raise, skip, phases)
- [x] Card representation and encoding
- [x] Deck commitment structure
- [x] Hole card commitment structure
- [x] Topology with arc definitions (147 places, 84 transitions)

### 1.2 ActionCircuit ⚠️
- [x] Port PetriTransitionCircuit pattern from tic-tac-toe
- [x] Encode full poker topology (147 places, 84 transitions)
- [x] Test with simulated games (start_hand, fold, check, deal_flop)
- [x] Test invalid transitions rejected
- [x] Benchmark: **123K constraints** (target was <15K)
- [ ] Add betting amount validation
- [ ] **TODO: Optimize constraint count**

### 1.3 DealCircuit ✅
- [x] Verify deck commitment hash
- [x] Prove dealt cards match deck positions
- [x] Test valid/invalid deals
- [x] Benchmark: **18.5K constraints** (target was <5K)
- [ ] Support incremental deals (hole cards, flop, turn, river separately)

### 1.4 HoleCommitmentCircuit ✅
- [x] Basic structure defined
- [x] Add tests for commitment/reveal
- [x] Test invalid card claims rejected
- [x] Test wrong salt rejected
- [x] Benchmark: **991 constraints** (target: <1K) ✅

### 1.5 ShowdownCircuit 🚧
- [ ] Encode hand evaluation from buildPokerHandModel()
  - [ ] Pair detection (78 transitions)
  - [ ] Trips detection (52 transitions)
  - [ ] Quads detection (13 transitions)
  - [ ] Straight detection (10 patterns)
  - [ ] Flush detection (suit counting)
  - [ ] Straight flush detection (40 transitions)
  - [ ] Full house detection
  - [ ] Two pair detection
- [ ] Best 5-of-7 selection
- [ ] Multi-player comparison (up to 5)
- [ ] Tiebreaker logic
- [ ] Test against known hands
- [ ] Benchmark constraint count (target: <20K)

## Phase 2: Integration

**Goal**: Connect circuits to existing texas-holdem backend.

### 2.1 Proof Generation Service
- [ ] Witness generation from game state
- [ ] Proving key management
- [ ] Async proof generation (background workers)
- [ ] Proof caching

### 2.2 Verification Integration
- [ ] Add verification endpoints to GraphQL API
- [ ] Store proofs in event store
- [ ] Verification on state transitions

### 2.3 Frontend Integration
- [ ] Commitment generation in browser
- [ ] Proof submission flow
- [ ] Verification status display

## Phase 3: Verifiable Shuffle (Future)

**Goal**: Remove trusted dealer assumption.

### 3.1 Mental Poker Protocol
- [ ] Research existing protocols (SRA, Barnett-Smart)
- [ ] Design shuffle circuit
- [ ] Multi-party commitment scheme
- [ ] Incremental reveal protocol

### 3.2 Shuffle Circuit
- [ ] Permutation commitment
- [ ] Joint randomness generation
- [ ] Shuffle verification
- [ ] Estimated: 50K+ constraints

## Phase 4: On-Chain Settlement (Future)

**Goal**: Trustless chip/pot management on Ethereum.

### 4.1 Verifier Contract
- [ ] Deploy Groth16 verifier (auto-generated from gnark)
- [ ] Gas optimization
- [ ] Multi-proof batching

### 4.2 Game Contract
- [ ] Chip escrow on game start
- [ ] Proof-gated pot claims
- [ ] Timeout handling
- [ ] Dispute resolution

## Constraint Budget

| Circuit | Target | Actual | Status |
|---------|--------|--------|--------|
| ActionCircuit | <15K | **123K** | ⚠️ Needs optimization |
| DealCircuit | <5K | **18.5K** | ⚠️ Higher than target |
| HoleCommitmentCircuit | <1K | **991** | ✅ On target |
| ShowdownCircuit | <20K | **25** (stub) | 🚧 Not implemented |
| **Total per hand** | <40K | TBD | - |

**Optimization needed**: ActionCircuit's 123K constraints come from O(places × transitions) loops.
Tic-tac-toe has 33×35=1,155 iterations; poker has 147×84=12,348 iterations.
Options:
1. Sparse topology encoding (only encode non-zero arcs)
2. Split into multiple smaller circuits
3. Use lookup tables (Plookup) for transition selection

For comparison:
- Tic-tac-toe Move: 6,012 constraints
- Tic-tac-toe Win: 3,036 constraints
- Groth16 verify on BN254: ~200K gas

## Testing Strategy

### Unit Tests
- Each circuit in isolation
- Known hand evaluation cases
- Edge cases (split pots, wheel straights, etc.)

### Integration Tests
- Full game simulation with proof generation
- Verify all state transitions
- Performance benchmarks

### Adversarial Tests
- Invalid action attempts
- Commitment forgery attempts
- Hand rank manipulation attempts

## Dependencies

- `github.com/consensys/gnark` - ZK circuit framework
- `github.com/consensys/gnark-crypto` - Cryptographic primitives
- Existing `zk-tictactoe` package - Reference implementation
- Existing `texas-holdem` service - Game logic

## Open Questions

1. **Betting amounts**: Encode as field elements or decompose to bits?
2. **Pot calculation**: In-circuit or off-chain with commitment?
3. **Side pots**: Support in v1 or defer?
4. **Timeout handling**: Cryptographic vs. social?
5. **Player elimination**: Reset active state between hands?

## References

- [Mental Poker Paper](https://en.wikipedia.org/wiki/Mental_poker)
- [gnark Documentation](https://docs.gnark.consensys.io/)
- [Poker Hand Rankings](https://en.wikipedia.org/wiki/List_of_poker_hands)
- [tic-tac-toe ZK implementation](../zk-tictactoe/)
