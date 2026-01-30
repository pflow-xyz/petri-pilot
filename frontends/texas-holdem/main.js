// Texas Hold'em Poker - Main Application
// Uses pflow ODE solver for strategic value computation

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@1.11.0/public/petri-solver.js'
import { renderCard, renderCards, renderCommunityCards, parseCard } from './cards.js'

// Read API_BASE dynamically
function getApiBase() {
  return window.API_BASE || ''
}

// Blind schedule - increases every N hands
const BLIND_SCHEDULE = [
  { small: 10, big: 20 },
  { small: 15, big: 30 },
  { small: 25, big: 50 },
  { small: 50, big: 100 },
  { small: 75, big: 150 },
  { small: 100, big: 200 },
  { small: 150, big: 300 },
  { small: 200, big: 400 },
  { small: 300, big: 600 },
  { small: 500, big: 1000 }
]
const HANDS_PER_BLIND_LEVEL = 5

// Game state
let gameState = {
  id: null,
  version: 0,
  places: {},
  enabled: [],
  events: [],
  players: [
    { name: 'Player 0 (You)', chips: 1000, cards: [], bet: 0, folded: false },
    { name: 'Player 1', chips: 1000, cards: [], bet: 0, folded: false },
    { name: 'Player 2', chips: 1000, cards: [], bet: 0, folded: false },
    { name: 'Player 3', chips: 1000, cards: [], bet: 0, folded: false },
    { name: 'Player 4', chips: 1000, cards: [], bet: 0, folded: false }
  ],
  communityCards: { flop: [], turn: [], river: [] },
  pot: 0,
  currentRound: 'waiting',
  dealer: 0,
  currentPlayer: 0,
  // Blind tracking
  blindLevel: 0,
  handsPlayed: 0,
  smallBlind: 10,
  bigBlind: 20,
  currentBet: 0  // The current bet to call
}

// ODE simulation results cache
let odeValues = null
let useLocalODE = true

// Configurable ODE solver parameters
let solverParams = {
  tspan: 2.0,
  dt: 0.2,
  adaptive: false,
  abstol: 1e-4,
  reltol: 1e-3
}

// Auto-play state
let autoPlayActive = false
let autoPlaySpeed = 1000 // ms between actions

// Card deck for dealing random cards
const RANKS = ['2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A']
const SUITS = ['h', 'd', 'c', 's'] // hearts, diamonds, clubs, spades
let deck = []
let randomSeed = null

/**
 * Seeded random number generator (Mulberry32)
 */
function seededRandom() {
  if (randomSeed === null) {
    return Math.random()
  }
  randomSeed |= 0
  randomSeed = randomSeed + 0x6D2B79F5 | 0
  let t = Math.imul(randomSeed ^ randomSeed >>> 15, 1 | randomSeed)
  t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t
  return ((t ^ t >>> 14) >>> 0) / 4294967296
}

/**
 * Set the random seed for reproducible shuffles
 */
function setRandomSeed(seed) {
  randomSeed = seed
  console.log(`Random seed set to: ${seed}`)
}
window.setRandomSeed = setRandomSeed

/**
 * Create and shuffle a new deck
 */
function createDeck(seed = null) {
  deck = []
  for (const rank of RANKS) {
    for (const suit of SUITS) {
      deck.push(rank + suit)
    }
  }
  // Use provided seed or generate a new random one
  if (seed !== null) {
    randomSeed = seed
  } else {
    randomSeed = Math.floor(Math.random() * 2147483647)
  }
  console.log(`Deck created with seed: ${randomSeed}`)
  shuffleDeck()
}

/**
 * Shuffle the deck using Fisher-Yates algorithm with seeded random
 */
function shuffleDeck() {
  for (let i = deck.length - 1; i > 0; i--) {
    const j = Math.floor(seededRandom() * (i + 1))
    ;[deck[i], deck[j]] = [deck[j], deck[i]]
  }
}

/**
 * Deal n cards from the deck
 */
function dealCards(n) {
  return deck.splice(0, n)
}

// Initialize application
document.addEventListener('DOMContentLoaded', () => {
  console.log('Texas Hold\'em Poker initialized')

  // Set up event listeners
  document.getElementById('new-game-btn').addEventListener('click', createNewGame)
  document.getElementById('start-hand-btn').addEventListener('click', startHand)
  document.getElementById('auto-play-btn').addEventListener('click', startAutoPlay)
  document.getElementById('stop-play-btn').addEventListener('click', stopAutoPlay)
  document.getElementById('speed-slider').addEventListener('input', updateSpeed)
  document.getElementById('toggle-heatmap-btn').addEventListener('click', toggleHeatmap)
  document.getElementById('toggle-ode-btn').addEventListener('click', toggleODEMode)

  // Render initial state
  renderPokerTable()
})

// ========================================================================
// API Integration
// ========================================================================

/**
 * Create a new game instance
 */
async function createNewGame() {
  try {
    showStatus('Creating new game...', 'info')
    
    const response = await fetch(`${getApiBase()}/api/texasholdem`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    })
    
    if (!response.ok) {
      throw new Error(`Failed to create game: ${response.status}`)
    }
    
    const data = await response.json()
    console.log('Game created:', data)
    
    // Initialize game state
    gameState.id = data.aggregate_id
    gameState.version = data.version
    gameState.places = data.state
    gameState.enabled = data.enabled_transitions || []
    
    // Reset local state
    gameState.events = []
    gameState.pot = 0
    gameState.currentRound = 'waiting'
    gameState.currentBet = 0
    gameState.players.forEach(p => {
      p.chips = 1000
      p.cards = []
      p.bet = 0
      p.folded = false
    })
    gameState.communityCards = { flop: [], turn: [], river: [] }

    // Reset blinds for new game
    gameState.blindLevel = 0
    gameState.handsPlayed = 0
    gameState.smallBlind = BLIND_SCHEDULE[0].small
    gameState.bigBlind = BLIND_SCHEDULE[0].big
    
    showStatus('Game created! Click "Start Hand" to begin.', 'success')
    document.getElementById('start-hand-btn').style.display = 'inline-block'
    
    renderPokerTable()
    renderGameState()
  } catch (err) {
    console.error('Failed to create game:', err)
    showStatus(`Error: ${err.message}`, 'error')
  }
}

/**
 * Start a new hand
 */
async function startHand() {
  try {
    if (!gameState.id) {
      showStatus('Please create a game first', 'error')
      return
    }
    
    showStatus('Starting new hand...', 'info')
    
    const response = await fetch(`${getApiBase()}/api/start_hand`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: gameState.id
      })
    })
    
    if (!response.ok) {
      throw new Error(`Failed to start hand: ${response.status}`)
    }
    
    const data = await response.json()
    console.log('Hand started:', data)
    
    updateGameState(data)

    // Update blind level and post blinds
    updateBlindLevel()
    postBlinds()

    showStatus(`Preflop - Blinds: $${gameState.smallBlind}/$${gameState.bigBlind}`, 'success')

    // Create a new shuffled deck and deal hole cards to all players
    createDeck()
    for (let i = 0; i < 5; i++) {
      gameState.players[i].cards = dealCards(2)
    }
    console.log('Dealt hole cards:', gameState.players.map(p => p.cards))

    // Show auto-play button
    document.getElementById('auto-play-btn').style.display = 'inline-block'

    // Update UI
    renderPokerTable()
    renderEventHistory()

    // Run ODE analysis
    await runODESimulation()
  } catch (err) {
    console.error('Failed to start hand:', err)
    showStatus(`Error: ${err.message}`, 'error')
  }
}

/**
 * Post blinds at the start of a hand
 */
function postBlinds() {
  // Small blind is player after dealer, big blind is next
  const sbPosition = (gameState.dealer + 1) % 5
  const bbPosition = (gameState.dealer + 2) % 5

  // Find active players for blind positions
  let sbPlayer = sbPosition
  let bbPlayer = bbPosition

  // Post small blind
  const sbAmount = Math.min(gameState.smallBlind, gameState.players[sbPlayer].chips)
  gameState.players[sbPlayer].chips -= sbAmount
  gameState.players[sbPlayer].bet = sbAmount
  gameState.pot += sbAmount

  // Post big blind
  const bbAmount = Math.min(gameState.bigBlind, gameState.players[bbPlayer].chips)
  gameState.players[bbPlayer].chips -= bbAmount
  gameState.players[bbPlayer].bet = bbAmount
  gameState.pot += bbAmount

  // Set current bet to big blind
  gameState.currentBet = gameState.bigBlind

  // Log blinds
  gameState.events.push({
    type: `${gameState.players[sbPlayer].name} posts SB $${sbAmount}`,
    timestamp: new Date().toISOString()
  })
  gameState.events.push({
    type: `${gameState.players[bbPlayer].name} posts BB $${bbAmount}`,
    timestamp: new Date().toISOString()
  })

  console.log(`Blinds posted: SB=$${sbAmount} (P${sbPlayer}), BB=$${bbAmount} (P${bbPlayer})`)
}

/**
 * Update blind level based on hands played
 */
function updateBlindLevel() {
  gameState.handsPlayed++
  const newLevel = Math.min(
    Math.floor(gameState.handsPlayed / HANDS_PER_BLIND_LEVEL),
    BLIND_SCHEDULE.length - 1
  )

  if (newLevel > gameState.blindLevel) {
    gameState.blindLevel = newLevel
    gameState.smallBlind = BLIND_SCHEDULE[newLevel].small
    gameState.bigBlind = BLIND_SCHEDULE[newLevel].big

    showStatus(`Blinds increased! SB: $${gameState.smallBlind}, BB: $${gameState.bigBlind}`, 'info')
    gameState.events.push({
      type: `Blinds increased to $${gameState.smallBlind}/$${gameState.bigBlind}`,
      timestamp: new Date().toISOString()
    })
  }
}

/**
 * Deal preflop cards (deprecated - start_hand now handles preflop directly)
 */
async function dealPreflop() {
  // In the new model, start_hand transitions directly to preflop with p0_turn
  // This function is kept for compatibility but does nothing
  console.log('dealPreflop called - no longer needed in new model')
}

/**
 * Execute a transition (generic action handler)
 */
async function executeTransition(transitionId, data = {}) {
  try {
    if (!gameState.id) {
      throw new Error('No active game')
    }
    
    const response = await fetch(`${getApiBase()}/api/${transitionId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: gameState.id,
        ...data
      })
    })
    
    if (!response.ok) {
      throw new Error(`Transition failed: ${response.status}`)
    }
    
    const result = await response.json()
    console.log(`Transition ${transitionId}:`, result)
    
    updateGameState(result)
    return result
  } catch (err) {
    console.error(`Execute transition ${transitionId} failed:`, err)
    showStatus(`Error: ${err.message}`, 'error')
    throw err
  }
}

/**
 * Update game state from API response
 */
function updateGameState(data) {
  if (data.state) {
    gameState.places = data.state
  }
  if (data.version) {
    gameState.version = data.version
  }
  if (data.enabled_transitions) {
    gameState.enabled = data.enabled_transitions
  }
  if (data.enabled) {
    gameState.enabled = data.enabled
  }

  // Parse state to extract game info
  parseGameState()

  // Add event to history - handle different event formats
  if (data.event) {
    gameState.events.push(data.event)
  } else if (data.event_type) {
    gameState.events.push({ type: data.event_type, timestamp: new Date().toISOString() })
  } else if (data.transition) {
    gameState.events.push({ type: data.transition, timestamp: new Date().toISOString() })
  }

  renderPokerTable()
  renderGameState()
  renderEventHistory()
}

/**
 * Parse places to extract high-level game state
 */
function parseGameState() {
  const { places } = gameState

  // Determine current round
  if (places.waiting > 0) gameState.currentRound = 'waiting'
  else if (places.preflop > 0) gameState.currentRound = 'preflop'
  else if (places.flop > 0) gameState.currentRound = 'flop'
  else if (places.turn_round > 0) gameState.currentRound = 'turn'
  else if (places.river > 0) gameState.currentRound = 'river'
  else if (places.showdown > 0) gameState.currentRound = 'showdown'
  else if (places.complete > 0) gameState.currentRound = 'complete'
  
  // Determine current player
  for (let i = 0; i < 5; i++) {
    if (places[`p${i}_turn`] > 0) {
      gameState.currentPlayer = i
      break
    }
  }
  
  // Update player folded status
  for (let i = 0; i < 5; i++) {
    gameState.players[i].folded = places[`p${i}_folded`] > 0
  }
}

// ========================================================================
// ODE Strategic Analysis - Hand Strength via Petri Net
// ========================================================================

// Hand rank values (higher = better)
const HAND_RANKS = {
  HIGH_CARD: 0,
  PAIR: 1,
  TWO_PAIR: 2,
  THREE_OF_A_KIND: 3,
  STRAIGHT: 4,
  FLUSH: 5,
  FULL_HOUSE: 6,
  FOUR_OF_A_KIND: 7,
  STRAIGHT_FLUSH: 8,
  ROYAL_FLUSH: 9
}

/**
 * Analyze draws for a hand
 * Returns draw potential information for ODE model
 */
function analyzeDraws(holeCards, communityCards) {
  const allCommunity = [
    ...(communityCards.flop || []),
    ...(communityCards.turn || []),
    ...(communityCards.river || [])
  ]

  const allCards = [...holeCards, ...allCommunity].map(parseCard)
  const holeCardsParsed = holeCards.map(parseCard)

  // Count suits for flush draw detection
  const suitCounts = { h: 0, d: 0, c: 0, s: 0 }
  const holeSuits = { h: 0, d: 0, c: 0, s: 0 }

  for (const card of allCards) {
    suitCounts[card.suit]++
  }
  for (const card of holeCardsParsed) {
    holeSuits[card.suit]++
  }

  // Flush draw detection
  let flushDraw = 0
  let flushMade = false
  let flushSuit = null
  for (const [suit, count] of Object.entries(suitCounts)) {
    if (count >= 5) {
      flushMade = true
      flushSuit = suit
    } else if (count === 4 && holeSuits[suit] >= 1) {
      flushDraw = 9 // 9 outs for flush draw
      flushSuit = suit
    }
  }

  // Get rank values for straight detection
  const rankValues = allCards.map(c => '23456789TJQKA'.indexOf(c.rank))
  const uniqueRanks = [...new Set(rankValues)].sort((a, b) => a - b)

  // Straight draw detection (OESD = 8 outs, gutshot = 4 outs)
  let straightDraw = 0
  let straightMade = false

  // Check for made straight or draws
  if (uniqueRanks.length >= 4) {
    // Count consecutive sequences
    let maxConsec = 1
    let consec = 1
    let gaps = []

    for (let i = 1; i < uniqueRanks.length; i++) {
      const diff = uniqueRanks[i] - uniqueRanks[i-1]
      if (diff === 1) {
        consec++
        maxConsec = Math.max(maxConsec, consec)
      } else if (diff === 2) {
        gaps.push(uniqueRanks[i-1] + 1) // Gutshot gap
        consec = 1
      } else {
        consec = 1
      }
    }

    if (maxConsec >= 5) {
      straightMade = true
    } else if (maxConsec === 4) {
      // Open-ended straight draw (8 outs)
      const minInSeq = Math.min(...uniqueRanks.filter((_, i, arr) => {
        if (i >= 3) return arr[i] - arr[i-3] === 3
        return false
      }))
      if (minInSeq > 0 && minInSeq < 9) {
        straightDraw = 8 // OESD
      } else {
        straightDraw = 4 // Gutshot at the ends
      }
    } else if (gaps.length > 0 && maxConsec >= 3) {
      straightDraw = 4 // Gutshot
    }
  }

  // Check for wheel draw (A-2-3-4-5)
  const hasAce = rankValues.includes(12)
  const lowCards = uniqueRanks.filter(r => r <= 3)
  if (hasAce && lowCards.length >= 3) {
    straightDraw = Math.max(straightDraw, 4)
  }

  // Pair/set draw - overcards to board
  let overcardDraw = 0
  if (allCommunity.length > 0) {
    const boardRanks = allCommunity.map(c => parseCard(c)).map(c => '23456789TJQKA'.indexOf(c.rank))
    const maxBoard = Math.max(...boardRanks)
    const holeRanks = holeCardsParsed.map(c => '23456789TJQKA'.indexOf(c.rank))

    for (const rank of holeRanks) {
      if (rank > maxBoard) {
        overcardDraw += 3 // 3 outs per overcard to pair
      }
    }
  }

  // Cards remaining in deck
  const cardsDealt = holeCards.length + allCommunity.length
  const cardsRemaining = 52 - cardsDealt
  const cardsToCome = allCommunity.length < 3 ? 5 - allCommunity.length :
                      allCommunity.length < 4 ? 2 :
                      allCommunity.length < 5 ? 1 : 0

  return {
    flushDraw: flushMade ? 0 : flushDraw,
    flushMade,
    straightDraw: straightMade ? 0 : straightDraw,
    straightMade,
    overcardDraw,
    cardsRemaining,
    cardsToCome,
    totalOuts: (flushMade ? 0 : flushDraw) + (straightMade ? 0 : straightDraw) + overcardDraw
  }
}

/**
 * Build Petri net model for hand strength computation
 * Models current hand + draw completion probabilities
 */
function buildHandStrengthPetriNet(holeCards, communityCards) {
  const draws = analyzeDraws(holeCards, communityCards)
  const currentHand = evaluateCurrentHand(holeCards, communityCards)

  // Calculate draw completion probability using rule of 2/4
  // Flop to river: outs * 4%, Turn to river: outs * 2%
  const outMultiplier = draws.cardsToCome >= 2 ? 0.04 : 0.02

  const flushProb = Math.min(0.95, draws.flushDraw * outMultiplier)
  const straightProb = Math.min(0.95, draws.straightDraw * outMultiplier)
  const pairProb = Math.min(0.95, draws.overcardDraw * outMultiplier)

  const places = {
    // Current hand strength (normalized 0-1)
    'current_hand': {
      '@type': 'Place',
      initial: [currentHand.rank / 9], // Normalize to 0-1
      x: 100, y: 100
    },
    // High card kicker value
    'kicker_value': {
      '@type': 'Place',
      initial: [currentHand.highCard / 14],
      x: 100, y: 150
    },
    // Draw potential places
    'flush_draw': {
      '@type': 'Place',
      initial: [draws.flushMade ? 1 : flushProb],
      x: 200, y: 100
    },
    'straight_draw': {
      '@type': 'Place',
      initial: [draws.straightMade ? 1 : straightProb],
      x: 200, y: 150
    },
    'pair_draw': {
      '@type': 'Place',
      initial: [pairProb],
      x: 200, y: 200
    },
    // Combined hand strength output
    'hand_strength': {
      '@type': 'Place',
      initial: [0],
      x: 400, y: 150
    },
    // Improvement potential
    'improvement_potential': {
      '@type': 'Place',
      initial: [draws.totalOuts / 20], // Normalize outs
      x: 300, y: 200
    }
  }

  const transitions = {
    'compute_strength': {
      '@type': 'Transition',
      x: 300, y: 150,
      rate: 1.0
    },
    'add_flush_value': {
      '@type': 'Transition',
      x: 350, y: 100,
      rate: draws.flushMade ? 0.55 : flushProb * 0.55 // Flush = ~0.55 hand strength
    },
    'add_straight_value': {
      '@type': 'Transition',
      x: 350, y: 150,
      rate: draws.straightMade ? 0.45 : straightProb * 0.45 // Straight = ~0.45
    },
    'add_pair_value': {
      '@type': 'Transition',
      x: 350, y: 200,
      rate: pairProb * 0.15 // Pair improvement = ~0.15
    }
  }

  const arcs = [
    // Current hand feeds into strength computation
    { '@type': 'Arrow', source: 'current_hand', target: 'compute_strength', weight: [1] },
    { '@type': 'Arrow', source: 'kicker_value', target: 'compute_strength', weight: [0.1] },
    { '@type': 'Arrow', source: 'compute_strength', target: 'hand_strength', weight: [1] },

    // Draw completions add to hand strength
    { '@type': 'Arrow', source: 'flush_draw', target: 'add_flush_value', weight: [1] },
    { '@type': 'Arrow', source: 'add_flush_value', target: 'hand_strength', weight: [1] },

    { '@type': 'Arrow', source: 'straight_draw', target: 'add_straight_value', weight: [1] },
    { '@type': 'Arrow', source: 'add_straight_value', target: 'hand_strength', weight: [1] },

    { '@type': 'Arrow', source: 'pair_draw', target: 'add_pair_value', weight: [1] },
    { '@type': 'Arrow', source: 'add_pair_value', target: 'hand_strength', weight: [1] },

    // Improvement potential modifies output
    { '@type': 'Arrow', source: 'improvement_potential', target: 'compute_strength', weight: [0.5] }
  ]

  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    places,
    transitions,
    arcs,
    // Store metadata for debugging
    _draws: draws,
    _currentHand: currentHand
  }
}

/**
 * Evaluate current made hand (before draws)
 */
function evaluateCurrentHand(holeCards, communityCards) {
  if (!holeCards || holeCards.length < 2) {
    return { rank: 0, highCard: 0 }
  }

  const allCommunity = [
    ...(communityCards.flop || []),
    ...(communityCards.turn || []),
    ...(communityCards.river || [])
  ]

  const allCards = [...holeCards, ...allCommunity].map(parseCard)

  if (allCards.length < 2) {
    return { rank: 0, highCard: 0 }
  }

  // Count ranks and suits
  const rankCounts = {}
  const suitCounts = {}
  const rankValues = []

  for (const card of allCards) {
    const rankVal = '23456789TJQKA'.indexOf(card.rank)
    rankValues.push(rankVal)
    rankCounts[card.rank] = (rankCounts[card.rank] || 0) + 1
    suitCounts[card.suit] = (suitCounts[card.suit] || 0) + 1
  }

  const counts = Object.values(rankCounts).sort((a, b) => b - a)
  const maxSuit = Math.max(...Object.values(suitCounts))
  const highCard = Math.max(...rankValues)

  // Check for flush
  const hasFlush = maxSuit >= 5

  // Check for straight
  const uniqueRanks = [...new Set(rankValues)].sort((a, b) => a - b)
  let hasStraight = false
  for (let i = 0; i <= uniqueRanks.length - 5; i++) {
    if (uniqueRanks[i + 4] - uniqueRanks[i] === 4) {
      hasStraight = true
      break
    }
  }
  // Check wheel (A-2-3-4-5)
  if (uniqueRanks.includes(12) && uniqueRanks.includes(0) &&
      uniqueRanks.includes(1) && uniqueRanks.includes(2) && uniqueRanks.includes(3)) {
    hasStraight = true
  }

  // Determine hand rank
  let rank = HAND_RANKS.HIGH_CARD

  if (hasStraight && hasFlush) {
    rank = highCard === 12 ? HAND_RANKS.ROYAL_FLUSH : HAND_RANKS.STRAIGHT_FLUSH
  } else if (counts[0] === 4) {
    rank = HAND_RANKS.FOUR_OF_A_KIND
  } else if (counts[0] === 3 && counts[1] >= 2) {
    rank = HAND_RANKS.FULL_HOUSE
  } else if (hasFlush) {
    rank = HAND_RANKS.FLUSH
  } else if (hasStraight) {
    rank = HAND_RANKS.STRAIGHT
  } else if (counts[0] === 3) {
    rank = HAND_RANKS.THREE_OF_A_KIND
  } else if (counts[0] === 2 && counts[1] === 2) {
    rank = HAND_RANKS.TWO_PAIR
  } else if (counts[0] === 2) {
    rank = HAND_RANKS.PAIR
  }

  return { rank, highCard }
}

/**
 * Compute hand strength using ODE solver
 */
function computeODEHandStrength(holeCards, communityCards) {
  try {
    const model = buildHandStrengthPetriNet(holeCards, communityCards)

    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = Solver.setRates(net)

    const prob = new Solver.ODEProblem(net, initialState, [0, solverParams.tspan], rates)
    const opts = { dt: solverParams.dt, adaptive: solverParams.adaptive }
    const solution = Solver.solve(prob, Solver.Tsit5(), opts)

    const finalState = solution.u ? solution.u[solution.u.length - 1] : null
    if (!finalState) {
      console.log('ODE solve returned no final state')
      return { strength: 0.5, draws: model._draws, hand: model._currentHand }
    }

    // Combine hand strength with draw potential
    const baseStrength = finalState['hand_strength'] || 0
    const currentHandValue = finalState['current_hand'] || 0
    const improvement = finalState['improvement_potential'] || 0

    // Weighted combination: current hand + improvement potential + draw completions
    const strength = Math.min(1, Math.max(0,
      currentHandValue * 0.6 + baseStrength * 0.3 + improvement * 0.1
    ))

    return {
      strength,
      draws: model._draws,
      hand: model._currentHand,
      finalState
    }
  } catch (err) {
    console.error('ODE hand strength computation failed:', err)
    return { strength: 0.5, draws: null, hand: null }
  }
}

/**
 * Run ODE simulation to compute strategic values for all actions
 */
async function runODESimulation() {
  try {
    showLoading(true)

    const values = {}
    const details = {}

    // Get available actions for current player
    const availableActions = getAvailableActions()

    console.log('Running ODE for actions:', availableActions)

    // Compute hand strength using ODE
    const playerCards = gameState.players[gameState.currentPlayer].cards
    const odeResult = computeODEHandStrength(playerCards, gameState.communityCards)

    console.log('ODE hand strength:', odeResult)

    // Calculate action values based on hand strength
    for (const action of availableActions) {
      let ev = 0
      const callAmount = Math.max(0, gameState.currentBet - gameState.players[gameState.currentPlayer].bet)
      const potSize = gameState.pot

      if (action.type === 'fold') {
        ev = 0 // Folding has 0 EV
      } else if (action.type === 'check') {
        // Check EV = hand strength * pot equity
        ev = odeResult.strength * potSize * 0.2
      } else if (action.type === 'call') {
        // Call EV = (hand strength * pot) - call amount
        const potOdds = callAmount / (potSize + callAmount)
        ev = (odeResult.strength * (potSize + callAmount)) - callAmount
        // Adjust for pot odds
        if (odeResult.strength < potOdds) {
          ev *= 0.5 // Penalize calling with bad odds
        }
      } else if (action.type === 'raise') {
        // Raise EV higher for strong hands
        const raiseAmount = action.amount || gameState.bigBlind * 3
        if (odeResult.strength > 0.6) {
          ev = odeResult.strength * (potSize + raiseAmount) * 1.5
        } else if (odeResult.strength > 0.4) {
          ev = odeResult.strength * potSize * 0.8
        } else {
          ev = -raiseAmount * 0.5 // Bluff value
        }
      }

      values[action.id] = ev
      details[action.id] = { ev, strength: odeResult.strength, draws: odeResult.draws }
    }

    odeValues = { values, details, handStrength: odeResult }
    console.log('ODE values:', odeValues)

    showLoading(false)
    renderActionButtons()
    renderODEAnalysis()

    return odeValues
  } catch (err) {
    console.error('ODE simulation failed:', err)
    showLoading(false)
    return null
  }
}

/**
 * Get available actions for current player
 */
function getAvailableActions() {
  const actions = []
  const player = gameState.currentPlayer

  // Check enabled transitions from API
  const enabledSet = new Set(gameState.enabled || [])

  if (enabledSet.has(`p${player}_fold`)) {
    actions.push({ id: `p${player}_fold`, type: 'fold', label: 'Fold', amount: 0 })
  }
  if (enabledSet.has(`p${player}_check`)) {
    actions.push({ id: `p${player}_check`, type: 'check', label: 'Check', amount: 0 })
  }
  if (enabledSet.has(`p${player}_call`)) {
    actions.push({ id: `p${player}_call`, type: 'call', label: 'Call', amount: 0 })
  }
  if (enabledSet.has(`p${player}_raise`)) {
    actions.push({ id: `p${player}_raise`, type: 'raise', label: 'Raise $50', amount: 50 })
    actions.push({ id: `p${player}_raise_100`, type: 'raise', label: 'Raise $100', amount: 100 })
    actions.push({ id: `p${player}_raise_200`, type: 'raise', label: 'Raise $200', amount: 200 })
  }

  // If it's player 0's turn in an active round and no actions found, show default actions
  if (actions.length === 0 && player === 0 && gameState.currentRound !== 'waiting' && gameState.currentRound !== 'complete') {
    actions.push({ id: `p0_fold`, type: 'fold', label: 'Fold', amount: 0 })
    actions.push({ id: `p0_check`, type: 'check', label: 'Check', amount: 0 })
    actions.push({ id: `p0_call`, type: 'call', label: 'Call', amount: 0 })
    actions.push({ id: `p0_raise`, type: 'raise', label: 'Raise', amount: 50 })
  }

  return actions
}

// ========================================================================
// UI Rendering
// ========================================================================

/**
 * Render the poker table
 */
function renderPokerTable() {
  // Update pot with blinds info
  const blindsInfo = `Blinds: $${gameState.smallBlind}/$${gameState.bigBlind}`
  document.getElementById('pot-display').innerHTML = `Pot: $${gameState.pot}<br><small>${blindsInfo}</small>`

  // Update round indicator
  document.getElementById('round-indicator').textContent = gameState.currentRound.toUpperCase()
  
  // Update community cards
  const communityCardsEl = document.getElementById('community-cards')
  communityCardsEl.innerHTML = renderCommunityCards(gameState.communityCards)
  
  // Update player seats
  gameState.players.forEach((player, i) => {
    const seatEl = document.getElementById(`seat-${i}`)
    
    // Update active/folded class
    if (player.folded) {
      seatEl.classList.add('folded')
    } else {
      seatEl.classList.remove('folded')
    }
    
    // Update active turn
    if (i === gameState.currentPlayer && !player.folded) {
      seatEl.classList.add('active-turn')
    } else {
      seatEl.classList.remove('active-turn')
    }
    
    // Update dealer button
    const dealerBtn = seatEl.querySelector('.dealer-button')
    if (i === gameState.dealer) {
      dealerBtn.style.display = 'inline-flex'
    } else {
      dealerBtn.style.display = 'none'
    }

    // Update blind markers (SB is after dealer, BB is after SB)
    const sbPosition = (gameState.dealer + 1) % 5
    const bbPosition = (gameState.dealer + 2) % 5
    let blindMarker = seatEl.querySelector('.blind-marker')
    if (!blindMarker) {
      blindMarker = document.createElement('span')
      blindMarker.className = 'blind-marker'
      seatEl.querySelector('.player-name').appendChild(blindMarker)
    }

    if (i === sbPosition && gameState.currentRound !== 'waiting') {
      blindMarker.textContent = 'SB'
      blindMarker.className = 'blind-marker sb'
      blindMarker.style.display = 'inline-block'
    } else if (i === bbPosition && gameState.currentRound !== 'waiting') {
      blindMarker.textContent = 'BB'
      blindMarker.className = 'blind-marker bb'
      blindMarker.style.display = 'inline-block'
    } else {
      blindMarker.style.display = 'none'
    }
    
    // Update chips
    seatEl.querySelector('.player-chips').textContent = `$${player.chips}`
    
    // Update cards (only show for player 0)
    const cardsEl = seatEl.querySelector('.player-cards')
    if (i === 0 && player.cards.length > 0) {
      cardsEl.innerHTML = renderCards(player.cards)
    } else if (player.cards.length > 0) {
      cardsEl.innerHTML = renderCards(['??', '??'], true)
    } else {
      cardsEl.innerHTML = ''
    }
    
    // Update bet
    const betEl = seatEl.querySelector('.player-bet')
    if (player.bet > 0) {
      betEl.textContent = `Bet: $${player.bet}`
      betEl.style.display = 'block'
    } else {
      betEl.style.display = 'none'
    }
  })
}

/**
 * Render action buttons with ODE values
 */
function renderActionButtons() {
  const actionsEl = document.getElementById('action-buttons')
  const gridEl = document.getElementById('action-grid')

  // Only show for player 0's turn in active rounds
  if (gameState.currentPlayer !== 0 || gameState.currentRound === 'waiting' || gameState.currentRound === 'complete') {
    actionsEl.style.display = 'none'
    return
  }

  const actions = getAvailableActions()

  if (actions.length === 0) {
    actionsEl.style.display = 'none'
    return
  }

  actionsEl.style.display = 'block'

  const values = odeValues?.values || {}

  // Find best action
  let bestAction = null
  let bestValue = -Infinity
  actions.forEach(action => {
    const value = values[action.id] || 0
    if (value > bestValue) {
      bestValue = value
      bestAction = action.id
    }
  })

  // Render action buttons
  gridEl.innerHTML = actions.map(action => {
    const value = values[action.id] || 0
    const isRecommended = action.id === bestAction && bestValue > 0

    return `
      <button class="action-button ${isRecommended ? 'recommended' : ''}"
              data-action="${action.id}"
              data-ode-value="${value}"
              onclick="window.performAction('${action.id.replace(/_\d+$/, '')}', ${action.amount})">
        <div class="action-label">${action.label}</div>
        <div class="action-ode-value">EV: ${value.toFixed(2)}</div>
      </button>
    `
  }).join('')
}

/**
 * Perform a player action
 */
window.performAction = async function(transitionId, amount = 0) {
  try {
    const data = amount > 0 ? { amount } : {}
    await executeTransition(transitionId, data)

    // Automatically execute dealer actions (deal flop, turn, river, etc.)
    await processGameUntilPlayerTurn()

    // Recalculate ODE
    await runODESimulation()
  } catch (err) {
    console.error('Action failed:', err)
  }
}

/**
 * Process all automatic game actions until Player 0 needs to act
 * This handles AI player decisions and dealer actions (dealing cards, etc.)
 */
async function processGameUntilPlayerTurn() {
  let iterations = 0
  const maxIterations = 50 // Safety limit for full betting rounds

  while (iterations < maxIterations) {
    iterations++

    // Check if game is over or waiting
    if (gameState.currentRound === 'complete' || gameState.currentRound === 'waiting') {
      break
    }

    // If it's Player 0's turn, stop and let human play
    if (gameState.currentPlayer === 0 && hasPlayerAction(0)) {
      renderActionButtons()
      break
    }

    // Try dealer actions first (deal cards, end round, etc.)
    const dealerActed = await checkDealerAction()
    if (dealerActed) {
      await delay(300)
      continue
    }

    // If it's an AI player's turn (P1-P4), have them act
    const aiActed = await processAIPlayerTurn()
    if (aiActed) {
      await delay(200)
      continue
    }

    // No action taken, break to avoid infinite loop
    break
  }
}

/**
 * Check if a player has an available action
 */
function hasPlayerAction(playerIndex) {
  const enabledSet = new Set(gameState.enabled || [])
  const prefix = `p${playerIndex}_`

  for (const action of ['fold', 'check', 'call', 'raise']) {
    if (enabledSet.has(prefix + action)) {
      return true
    }
  }
  return false
}

/**
 * Process AI player turn (P1-P4)
 * Simple AI: check if possible, otherwise call, rarely fold
 */
async function processAIPlayerTurn() {
  const enabledSet = new Set(gameState.enabled || [])

  // Find which AI player can act
  for (let i = 1; i <= 4; i++) {
    const prefix = `p${i}_`

    // Simple AI strategy: check > call > fold
    if (enabledSet.has(prefix + 'check')) {
      showStatus(`Player ${i} checks`, 'info')
      await executeTransition(prefix + 'check', {})
      return true
    }

    if (enabledSet.has(prefix + 'call')) {
      showStatus(`Player ${i} calls`, 'info')
      await executeTransition(prefix + 'call', {})
      return true
    }

    if (enabledSet.has(prefix + 'fold')) {
      // Only fold if no other option (shouldn't happen with check available)
      showStatus(`Player ${i} folds`, 'info')
      await executeTransition(prefix + 'fold', {})
      return true
    }
  }

  return false
}

/**
 * Render game state info
 */
function renderGameState() {
  const infoEl = document.getElementById('game-state-info')
  
  if (!gameState.id) {
    infoEl.innerHTML = '<p>No game in progress</p>'
    return
  }
  
  infoEl.innerHTML = `
    <p><strong>Game ID:</strong> ${gameState.id.slice(0, 8)}...</p>
    <p><strong>Round:</strong> ${gameState.currentRound}</p>
    <p><strong>Pot:</strong> $${gameState.pot}</p>
    <p><strong>Blinds:</strong> $${gameState.smallBlind}/$${gameState.bigBlind} (Level ${gameState.blindLevel + 1})</p>
    <p><strong>Current Bet:</strong> $${gameState.currentBet}</p>
    <p><strong>Current Player:</strong> Player ${gameState.currentPlayer}</p>
    <p><strong>Active Players:</strong> ${gameState.players.filter(p => !p.folded).length}</p>
    <p><strong>Hands Played:</strong> ${gameState.handsPlayed}</p>
  `
}

/**
 * Render event history
 */
function renderEventHistory() {
  const listEl = document.getElementById('event-list')

  if (!gameState.events || gameState.events.length === 0) {
    listEl.innerHTML = '<p style="color: rgba(255,255,255,0.6); font-style: italic;">No events yet</p>'
    return
  }

  listEl.innerHTML = gameState.events.slice(-10).reverse().map(event => {
    // Handle both object events and string events
    const eventType = typeof event === 'string' ? event : (event.type || event.event_type || 'unknown')
    const timestamp = event.timestamp || event.created_at

    // Format the event type for display
    const displayType = eventType.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())

    return `
      <div class="event-item">
        <strong>${displayType}</strong>
        ${timestamp ? `<br><small>${new Date(timestamp).toLocaleTimeString()}</small>` : ''}
      </div>
    `
  }).join('')
}

/**
 * Render ODE analysis
 */
function renderODEAnalysis() {
  const analysisEl = document.getElementById('ode-analysis')

  if (!odeValues || Object.keys(odeValues.values || {}).length === 0) {
    if (gameState.currentRound === 'waiting') {
      analysisEl.innerHTML = '<p style="color: rgba(255,255,255,0.6); font-style: italic;">Start a hand to see analysis</p>'
    } else if (gameState.currentPlayer !== 0) {
      analysisEl.innerHTML = '<p style="color: rgba(255,255,255,0.6); font-style: italic;">Waiting for your turn...</p>'
    } else {
      analysisEl.innerHTML = '<p style="color: rgba(255,255,255,0.6); font-style: italic;">Computing strategic values...</p>'
    }
    return
  }

  const { values } = odeValues

  // Sort by value descending
  const sortedEntries = Object.entries(values).sort((a, b) => b[1] - a[1])

  analysisEl.innerHTML = `
    <p style="margin-bottom: 0.5rem;"><strong>Expected Values:</strong></p>
    ${sortedEntries.map(([action, value]) => {
      const color = value > 0 ? '#10b981' : value < 0 ? '#ef4444' : '#ffd700'
      const label = action.replace(/^p\d_/, '').replace(/_/g, ' ')
      return `<p style="margin: 0.25rem 0;"><span style="color: ${color};">●</span> ${label}: <strong>${value.toFixed(2)}</strong></p>`
    }).join('')}
  `
}

// ========================================================================
// UI Controls
// ========================================================================

/**
 * Show status message
 */
let statusTimeout = null
function showStatus(message, type = 'info') {
  const statusEl = document.getElementById('status-message')
  statusEl.textContent = message
  statusEl.className = `status-message ${type} visible`

  // Clear any existing timeout
  if (statusTimeout) {
    clearTimeout(statusTimeout)
  }

  statusTimeout = setTimeout(() => {
    statusEl.classList.remove('visible')
  }, 3000)
}

/**
 * Show/hide loading indicator
 */
function showLoading(show) {
  const loadingEl = document.getElementById('loading-indicator')
  loadingEl.style.display = show ? 'block' : 'none'
}

/**
 * Toggle heatmap overlay
 */
function toggleHeatmap() {
  const overlayEl = document.getElementById('heatmap-overlay')
  const isVisible = overlayEl.style.display !== 'none'
  
  if (isVisible) {
    overlayEl.style.display = 'none'
  } else {
    renderHeatmap()
    overlayEl.style.display = 'flex'
  }
}

/**
 * Render heatmap
 */
function renderHeatmap() {
  const gridEl = document.getElementById('heatmap-grid')
  
  if (!odeValues) {
    gridEl.innerHTML = '<p>No ODE values available</p>'
    return
  }
  
  const { values } = odeValues
  const entries = Object.entries(values)
  
  gridEl.innerHTML = entries.map(([action, value]) => {
    let className = 'neutral'
    if (value > 0.5) className = 'positive'
    else if (value < -0.5) className = 'negative'
    
    return `
      <div class="heatmap-item ${className}">
        <strong>${action}</strong>
        <div>${value.toFixed(2)}</div>
      </div>
    `
  }).join('')
}

/**
 * Toggle ODE mode
 */
function toggleODEMode() {
  useLocalODE = !useLocalODE
  document.getElementById('ode-mode').textContent = useLocalODE ? 'Local' : 'API'
  console.log(`ODE mode: ${useLocalODE ? 'local' : 'API'}`)
}

// ========================================================================
// Auto-Play System
// ========================================================================

/**
 * Start tournament mode - creates game, starts hand, and runs automatically
 */
async function startAutoPlay() {
  try {
    // Create a new game if none exists
    if (!gameState.id) {
      showStatus('Creating tournament...', 'info')

      const response = await fetch(`${getApiBase()}/api/texasholdem`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
      })

      if (!response.ok) {
        throw new Error(`Failed to create game: ${response.status}`)
      }

      const data = await response.json()
      console.log('Tournament game created:', data)

      // Initialize game state
      gameState.id = data.aggregate_id
      gameState.version = data.version
      gameState.places = data.state
      gameState.enabled = data.enabled_transitions || []

      // Reset local state
      gameState.events = []
      gameState.pot = 0
      gameState.currentRound = 'waiting'
      gameState.currentBet = 0
      gameState.players.forEach(p => {
        p.chips = 1000
        p.cards = []
        p.bet = 0
        p.folded = false
      })
      gameState.communityCards = { flop: [], turn: [], river: [] }
      gameState.blindLevel = 0
      gameState.handsPlayed = 0
      gameState.smallBlind = BLIND_SCHEDULE[0].small
      gameState.bigBlind = BLIND_SCHEDULE[0].big
    }

    // Start the first hand if in waiting state
    if (gameState.currentRound === 'waiting') {
      showStatus('Starting first hand...', 'info')

      const response = await fetch(`${getApiBase()}/api/start_hand`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ aggregate_id: gameState.id, data: {} })
      })

      if (!response.ok) {
        throw new Error(`Failed to start hand: ${response.status}`)
      }

      const data = await response.json()
      updateGameState(data)

      // Post blinds and deal cards
      postBlinds()
      createDeck()
      for (let i = 0; i < 5; i++) {
        gameState.players[i].cards = dealCards(2)
      }

      renderPokerTable()
      renderEventHistory()
    }

    autoPlayActive = true
    document.getElementById('auto-play-btn').style.display = 'none'
    document.getElementById('stop-play-btn').style.display = 'inline-block'
    document.getElementById('speed-control').style.display = 'flex'
    document.getElementById('action-buttons').style.display = 'none'
    document.getElementById('start-hand-btn').style.display = 'none'

    showStatus('🏆 Tournament started!', 'success')
    console.log('Tournament started')

    // Start the auto-play loop
    autoPlayLoop()
  } catch (err) {
    console.error('Failed to start tournament:', err)
    showStatus(`Error: ${err.message}`, 'error')
  }
}

/**
 * Stop auto-play mode
 */
function stopAutoPlay() {
  autoPlayActive = false
  document.getElementById('auto-play-btn').style.display = 'inline-block'
  document.getElementById('stop-play-btn').style.display = 'none'
  document.getElementById('speed-control').style.display = 'none'
  document.getElementById('new-game-btn').style.display = 'inline-block'
  document.getElementById('start-hand-btn').style.display = gameState.currentRound === 'waiting' ? 'inline-block' : 'none'

  showStatus('Tournament paused', 'info')
  console.log('Tournament paused')

  // Re-render action buttons if it's player 0's turn
  renderActionButtons()
}

/**
 * Update auto-play speed from slider
 */
function updateSpeed() {
  const slider = document.getElementById('speed-slider')
  const label = document.getElementById('speed-label')
  const value = parseInt(slider.value)

  // Map 0-100 to 2000ms (slow) to 50ms (fast)
  // Using exponential scale for better feel
  const minSpeed = 50
  const maxSpeed = 2000
  autoPlaySpeed = Math.round(maxSpeed - (value / 100) * (maxSpeed - minSpeed))

  // Calculate multiplier for display (1x = 1000ms)
  const multiplier = (1000 / autoPlaySpeed).toFixed(1)
  label.textContent = `${multiplier}x`

  console.log(`Speed: ${autoPlaySpeed}ms (${multiplier}x)`)
}

/**
 * Main auto-play loop
 */
async function autoPlayLoop() {
  let consecutiveFailures = 0

  while (autoPlayActive && gameState.currentRound !== 'complete' && gameState.currentRound !== 'waiting') {
    await delay(autoPlaySpeed)

    if (!autoPlayActive) break

    // Safety: stop if too many consecutive failures
    if (consecutiveFailures > 10) {
      console.log('Too many failures, stopping auto-play')
      break
    }

    const enabledSet = new Set(gameState.enabled || [])
    console.log('Auto-play enabled transitions:', Array.from(enabledSet))

    // Try dealer actions first (dealing cards, ending rounds)
    let actionTaken = false

    for (const dealerAction of ['deal_flop', 'deal_turn', 'deal_river', 'end_betting_round', 'go_showdown', 'determine_winner', 'end_hand']) {
      if (enabledSet.has(dealerAction)) {
        try {
          showStatus(`Dealer: ${dealerAction}`, 'info')
          await executeTransition(dealerAction, {})

          // Deal community cards when appropriate
          if (dealerAction === 'deal_flop') {
            gameState.communityCards.flop = dealCards(3)
            console.log('Auto-play flop:', gameState.communityCards.flop)
          } else if (dealerAction === 'deal_turn') {
            gameState.communityCards.turn = dealCards(1)
            console.log('Auto-play turn:', gameState.communityCards.turn)
          } else if (dealerAction === 'deal_river') {
            gameState.communityCards.river = dealCards(1)
            console.log('Auto-play river:', gameState.communityCards.river)
          } else if (dealerAction === 'determine_winner') {
            awardPotToWinner()
          } else if (dealerAction === 'end_hand' && gameState.pot > 0) {
            awardPotToWinner()
          }

          renderPokerTable()
          actionTaken = true
          consecutiveFailures = 0
          break
        } catch (err) {
          console.log(`${dealerAction} failed:`, err)
        }
      }
    }

    if (actionTaken) continue

    // Try player actions - use strategic decision making
    for (let p = 0; p < 5; p++) {
      const prefix = `p${p}_`
      const player = gameState.players[p]
      const skipId = `${prefix}skip`

      // Check if this player has any real action available
      const hasAction = ['fold', 'check', 'call', 'raise'].some(a => enabledSet.has(`${prefix}${a}`))

      // If no real actions but skip is available, use skip (player is folded/eliminated/all-in)
      if (!hasAction && enabledSet.has(skipId)) {
        try {
          console.log(`Player ${p} - using skip (no actions available)`)
          showStatus(`Player ${p}: skipped`, 'info')
          await executeTransition(skipId, {})
          actionTaken = true
          consecutiveFailures = 0
          break
        } catch (err) {
          console.log(`${skipId} failed:`, err)
        }
      }

      if (!hasAction) continue

      // Use strategic decision making based on hand strength
      const decision = await simulatePlayerDecision(p)
      if (!decision) continue

      try {
        const callAmount = Math.max(0, gameState.currentBet - player.bet)
        const raiseAmount = gameState.bigBlind * 3
        let betAmount = 0
        let actionLabel = decision.type

        if (decision.type === 'call' && callAmount > 0) {
          betAmount = Math.min(callAmount, player.chips)
          actionLabel = `calls $${betAmount}`
        } else if (decision.type === 'raise') {
          betAmount = callAmount + raiseAmount
          betAmount = Math.min(betAmount, player.chips)
          actionLabel = `raises to $${player.bet + betAmount}`
        } else if (decision.type === 'fold') {
          actionLabel = 'folds'
          player.folded = true
        } else {
          actionLabel = 'checks'
        }

        showStatus(`Player ${p}: ${actionLabel}`, 'info')
        await executeTransition(decision.id, decision.type === 'raise' ? { amount: raiseAmount } : {})

        // Update player's bet and chips
        if (betAmount > 0) {
          player.chips -= betAmount
          player.bet += betAmount
          gameState.pot += betAmount
          if (decision.type === 'raise') {
            gameState.currentBet = player.bet
          }
        }

        actionTaken = true
        consecutiveFailures = 0

        // Add to event history
        gameState.events.push({
          type: `Player ${p} ${actionLabel}`,
          timestamp: new Date().toISOString()
        })
        renderEventHistory()
        renderPokerTable()
        break
      } catch (err) {
        console.log(`${decision.id} failed:`, err)
      }
    }

    if (actionTaken) continue

    // Try advance transitions
    for (let p = 0; p < 5; p++) {
      const advanceId = `advance_to_p${p}`
      if (enabledSet.has(advanceId)) {
        try {
          console.log(`Advancing to player ${p}`)
          await executeTransition(advanceId, {})
          actionTaken = true
          consecutiveFailures = 0
          break
        } catch (err) {
          console.log(`${advanceId} failed:`, err)
        }
      }
    }

    if (!actionTaken) {
      consecutiveFailures++
      console.log(`No action taken, failure count: ${consecutiveFailures}`)
    }
  }

  if (autoPlayActive && gameState.currentRound === 'complete') {
    // Check how many players still have chips
    const playersWithChips = gameState.players.filter(p => p.chips > 0)

    if (playersWithChips.length <= 1) {
      // Tournament over - we have a winner!
      const winner = playersWithChips[0] || gameState.players[0]
      showStatus(`🏆 ${winner.name} wins the tournament with $${winner.chips}!`, 'success')
      stopAutoPlay()
    } else {
      // More players remain - start next hand
      showStatus(`Hand complete! ${playersWithChips.length} players remain. Starting next hand...`, 'info')

      // Reset for next hand
      await delay(1500)

      if (autoPlayActive) {
        await startNextHand()
        // Continue the auto-play loop
        autoPlayLoop()
      }
    }
  }
}

/**
 * Start the next hand (used by tournament for continuous play)
 */
async function startNextHand() {
  try {
    // First, refresh game state from server to get current enabled transitions
    const stateResponse = await fetch(`${getApiBase()}/api/texasholdem/${gameState.id}`)
    if (stateResponse.ok) {
      const stateData = await stateResponse.json()
      updateGameState(stateData)
    }

    // Check if end_hand needs to be fired first (after determine_winner)
    let enabledSet = new Set(gameState.enabled || [])
    if (enabledSet.has('end_hand') && !enabledSet.has('start_hand')) {
      console.log('Firing end_hand to prepare for next hand')
      await executeTransition('end_hand', {})

      // Refresh state after end_hand
      await delay(300)
      const refreshResponse = await fetch(`${getApiBase()}/api/texasholdem/${gameState.id}`)
      if (refreshResponse.ok) {
        const refreshData = await refreshResponse.json()
        updateGameState(refreshData)
      }
      enabledSet = new Set(gameState.enabled || [])
    }

    // Check if start_hand is enabled
    if (!enabledSet.has('start_hand')) {
      console.log('start_hand not enabled, enabled:', gameState.enabled)
      // Wait a bit and try again - the state might need to settle
      await delay(500)
      const retryResponse = await fetch(`${getApiBase()}/api/texasholdem/${gameState.id}`)
      if (retryResponse.ok) {
        const retryData = await retryResponse.json()
        updateGameState(retryData)
      }
    }

    // Reset player states for new hand
    gameState.players.forEach(p => {
      p.cards = []
      p.bet = 0
      p.folded = p.chips <= 0 // Fold out players with no chips
    })
    gameState.communityCards = { flop: [], turn: [], river: [] }
    gameState.pot = 0
    gameState.currentBet = 0

    // Move dealer button
    do {
      gameState.dealer = (gameState.dealer + 1) % 5
    } while (gameState.players[gameState.dealer].chips <= 0)

    // Call start_hand API
    const response = await fetch(`${getApiBase()}/api/start_hand`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: gameState.id, data: {} })
    })

    if (!response.ok) {
      const errorText = await response.text()
      console.error('start_hand failed:', response.status, errorText)
      throw new Error(`Failed to start hand: ${response.status}`)
    }

    const data = await response.json()
    updateGameState(data)

    // Update blinds and post them
    updateBlindLevel()
    postBlinds()

    // Create new deck and deal cards
    createDeck()
    for (let i = 0; i < 5; i++) {
      if (gameState.players[i].chips > 0) {
        gameState.players[i].cards = dealCards(2)
      }
    }

    showStatus(`Hand #${gameState.handsPlayed} - Blinds: $${gameState.smallBlind}/$${gameState.bigBlind}`, 'info')
    renderPokerTable()
    renderEventHistory()

  } catch (err) {
    console.error('Failed to start next hand:', err)
    showStatus(`Error starting hand: ${err.message}`, 'error')
    stopAutoPlay()
  }
}

/**
 * Check if dealer needs to act (deal cards, end round, etc.)
 */
async function checkDealerAction() {
  const enabledSet = new Set(gameState.enabled || [])

  // Check for dealer actions
  if (enabledSet.has('deal_flop')) {
    showStatus('Dealing flop...', 'info')
    await executeTransition('deal_flop', {})
    // Deal 3 cards for the flop
    gameState.communityCards.flop = dealCards(3)
    console.log('Flop:', gameState.communityCards.flop)
    renderPokerTable()
    return true
  }

  if (enabledSet.has('deal_turn')) {
    showStatus('Dealing turn...', 'info')
    await executeTransition('deal_turn', {})
    // Deal 1 card for the turn
    gameState.communityCards.turn = dealCards(1)
    console.log('Turn:', gameState.communityCards.turn)
    renderPokerTable()
    return true
  }

  if (enabledSet.has('deal_river')) {
    showStatus('Dealing river...', 'info')
    await executeTransition('deal_river', {})
    // Deal 1 card for the river
    gameState.communityCards.river = dealCards(1)
    console.log('River:', gameState.communityCards.river)
    renderPokerTable()
    return true
  }

  if (enabledSet.has('end_betting_round')) {
    console.log('Ending betting round')
    await executeTransition('end_betting_round', {})
    return true
  }

  if (enabledSet.has('go_showdown')) {
    showStatus('Going to showdown...', 'info')
    await executeTransition('go_showdown', {})
    return true
  }

  if (enabledSet.has('determine_winner')) {
    showStatus('Determining winner...', 'info')
    await executeTransition('determine_winner', {})

    // Award pot to winner
    awardPotToWinner()
    return true
  }

  if (enabledSet.has('end_hand')) {
    showStatus('Ending hand...', 'info')
    await executeTransition('end_hand', {})

    // If pot wasn't awarded yet (e.g., everyone folded), award it now
    if (gameState.pot > 0) {
      awardPotToWinner()
    }
    return true
  }

  // Check for player advancement transitions
  for (let i = 0; i < 5; i++) {
    if (enabledSet.has(`advance_to_p${i}`)) {
      console.log(`Advancing to player ${i}`)
      try {
        await executeTransition(`advance_to_p${i}`, {})
        return true
      } catch (err) {
        // Transition might have been consumed, continue checking
        console.log(`advance_to_p${i} failed, continuing...`)
      }
    }
  }

  return false
}

/**
 * Simulate a player's decision using ODE analysis
 * No bluffing - play straightforwardly based on hand strength
 */
async function simulatePlayerDecision(playerIndex) {
  const enabledSet = new Set(gameState.enabled || [])
  const prefix = `p${playerIndex}_`
  const player = gameState.players[playerIndex]

  // Calculate call amount (difference between current bet and player's bet)
  const callAmount = Math.max(0, gameState.currentBet - player.bet)

  // Raise amounts based on big blind
  const minRaise = gameState.bigBlind * 2
  const standardRaise = gameState.bigBlind * 3

  // Get available actions for this player
  const availableActions = []

  if (enabledSet.has(`${prefix}fold`)) {
    availableActions.push({ id: `${prefix}fold`, type: 'fold', label: 'Fold', amount: 0 })
  }
  if (enabledSet.has(`${prefix}check`) && callAmount === 0) {
    availableActions.push({ id: `${prefix}check`, type: 'check', label: 'Check', amount: 0 })
  }
  if (enabledSet.has(`${prefix}call`) && callAmount > 0 && player.chips >= callAmount) {
    availableActions.push({ id: `${prefix}call`, type: 'call', label: `Call $${callAmount}`, amount: callAmount })
  }
  if (enabledSet.has(`${prefix}raise`) && player.chips >= callAmount + minRaise) {
    availableActions.push({ id: `${prefix}raise`, type: 'raise', label: `Raise $${standardRaise}`, amount: standardRaise })
  }

  if (availableActions.length === 0) {
    console.log(`Player ${playerIndex} has no available actions`)
    return null
  }

  // Evaluate hand strength using actual dealt cards
  const playerCards = gameState.players[playerIndex].cards
  const handStrength = evaluateHandStrength(playerCards, gameState.communityCards)

  console.log(`Player ${playerIndex} cards: ${playerCards.join(', ')}, strength: ${handStrength.toFixed(2)}, callAmount: $${callAmount}`)

  // Strategy considering pot odds and hand strength:
  // - Strong hand (>0.7): Raise
  // - Medium hand (0.4-0.7): Call if reasonable, check if free
  // - Weak hand (<0.4): Check if free, fold if facing a bet

  let chosenAction = null

  if (handStrength > 0.7) {
    // Strong hand - raise if possible
    chosenAction = availableActions.find(a => a.type === 'raise')
      || availableActions.find(a => a.type === 'call')
      || availableActions.find(a => a.type === 'check')
  } else if (handStrength > 0.4) {
    // Medium hand - call reasonable bets, check if free
    if (callAmount === 0) {
      chosenAction = availableActions.find(a => a.type === 'check')
    } else if (callAmount <= player.chips * 0.2) {
      // Call if it's less than 20% of stack
      chosenAction = availableActions.find(a => a.type === 'call')
    } else {
      chosenAction = availableActions.find(a => a.type === 'fold')
    }
  } else {
    // Weak hand - check if free, otherwise fold
    if (callAmount === 0) {
      chosenAction = availableActions.find(a => a.type === 'check')
    } else {
      chosenAction = availableActions.find(a => a.type === 'fold')
    }
  }

  return chosenAction || availableActions[0]
}

/**
 * Execute a player's action
 */
async function executePlayerAction(action) {
  try {
    const data = action.amount > 0 ? { amount: action.amount } : {}
    await executeTransition(action.id, data)

    // Add to event history
    gameState.events.push({
      type: `Player ${gameState.currentPlayer} ${action.type}s`,
      timestamp: new Date().toISOString()
    })
    renderEventHistory()

  } catch (err) {
    console.error('Action failed:', err)
  }
}

/**
 * Award the pot to the winner
 */
function awardPotToWinner() {
  // Find active players (not folded and have chips)
  const activePlayers = gameState.players
    .map((p, i) => ({ ...p, index: i }))
    .filter(p => !p.folded)

  if (activePlayers.length === 0) {
    console.log('No active players to award pot')
    return
  }

  let winner
  if (activePlayers.length === 1) {
    // Only one player left - they win by default
    winner = activePlayers[0]
  } else {
    // Multiple players - evaluate hands at showdown
    let bestStrength = -1
    for (const player of activePlayers) {
      const strength = evaluateHandStrength(player.cards, gameState.communityCards)
      console.log(`Player ${player.index} showdown strength: ${strength.toFixed(3)}`)
      if (strength > bestStrength) {
        bestStrength = strength
        winner = player
      }
    }
  }

  if (winner) {
    const potAmount = gameState.pot
    gameState.players[winner.index].chips += potAmount
    showStatus(`${winner.name} wins $${potAmount}!`, 'success')

    gameState.events.push({
      type: `${winner.name} wins $${potAmount}`,
      timestamp: new Date().toISOString()
    })

    console.log(`${winner.name} wins pot of $${potAmount}, now has $${gameState.players[winner.index].chips}`)
    gameState.pot = 0

    renderPokerTable()
    renderEventHistory()
  }
}

/**
 * Evaluate hand strength using ODE-based computation
 * Returns a value between 0 and 1
 */
function evaluateHandStrength(holeCards, communityCards) {
  if (!holeCards || holeCards.length < 2) return 0.5

  try {
    // Use ODE computation for hand strength
    const result = computeODEHandStrength(holeCards, communityCards)

    // Add small randomness to prevent predictability
    const noise = (Math.random() - 0.5) * 0.06
    const strength = Math.max(0, Math.min(1, result.strength + noise))

    // Log draw information for debugging
    if (result.draws) {
      const draws = result.draws
      if (draws.flushDraw > 0 || draws.straightDraw > 0) {
        console.log(`  Draws: flush=${draws.flushDraw} outs, straight=${draws.straightDraw} outs, overcards=${draws.overcardDraw}`)
      }
    }

    return strength
  } catch (err) {
    console.error('ODE evaluation failed, using fallback:', err)
    return evaluateHandStrengthFallback(holeCards, communityCards)
  }
}

/**
 * Fallback hand strength evaluation (heuristic-based)
 * Used when ODE computation fails
 */
function evaluateHandStrengthFallback(holeCards, communityCards) {
  if (!holeCards || holeCards.length < 2) return 0.5

  const holeCardsParsed = holeCards.map(parseCard)
  const allCommunity = [
    ...(communityCards.flop || []),
    ...(communityCards.turn || []),
    ...(communityCards.river || [])
  ].map(parseCard)

  const allCards = [...holeCardsParsed, ...allCommunity]
  const holeRanks = holeCardsParsed.map(c => c.rank)
  const holeSuits = holeCardsParsed.map(c => c.suit)
  const holeValues = holeRanks.map(r => '23456789TJQKA'.indexOf(r))

  let strength = 0.2

  // High cards bonus
  strength += holeValues.reduce((sum, v) => sum + (v / 13) * 0.08, 0)

  // Pocket pair bonus
  if (holeRanks[0] === holeRanks[1]) {
    strength += 0.25 + (holeValues[0] / 13) * 0.15
  }

  // Suited bonus
  if (holeSuits[0] === holeSuits[1]) {
    strength += 0.08
  }

  // Connected bonus
  if (Math.abs(holeValues[0] - holeValues[1]) === 1) {
    strength += 0.05
  }

  // Community card analysis
  if (allCommunity.length > 0) {
    const allRanks = allCards.map(c => c.rank)
    const allSuits = allCards.map(c => c.suit)

    // Check for pairs
    for (const holeRank of holeRanks) {
      const matches = allCommunity.filter(c => c.rank === holeRank).length
      if (matches > 0) strength += 0.15 * matches
    }

    // Flush check
    const suitCounts = {}
    for (const suit of allSuits) {
      suitCounts[suit] = (suitCounts[suit] || 0) + 1
    }
    const maxSuit = Math.max(...Object.values(suitCounts))
    if (maxSuit >= 5) strength += 0.35
    else if (maxSuit === 4 && holeSuits[0] === holeSuits[1]) strength += 0.12

    // Hand rank check
    const rankCounts = {}
    for (const rank of allRanks) {
      rankCounts[rank] = (rankCounts[rank] || 0) + 1
    }
    const counts = Object.values(rankCounts).sort((a, b) => b - a)
    if (counts[0] >= 4) strength += 0.45
    else if (counts[0] >= 3 && counts[1] >= 2) strength += 0.4
    else if (counts[0] >= 3) strength += 0.25
    else if (counts[0] >= 2 && counts[1] >= 2) strength += 0.18
  }

  return Math.max(0, Math.min(1, strength + (Math.random() - 0.5) * 0.08))
}

/**
 * Utility delay function
 */
function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

// Export for console testing
window.gameState = gameState
window.runODESimulation = runODESimulation
window.buildHandStrengthPetriNet = buildHandStrengthPetriNet
window.computeODEHandStrength = computeODEHandStrength
window.startAutoPlay = startAutoPlay
window.stopAutoPlay = stopAutoPlay
