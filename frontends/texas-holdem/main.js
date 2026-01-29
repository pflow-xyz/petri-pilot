// Texas Hold'em Poker - Main Application
// Uses pflow ODE solver for strategic value computation

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@1.11.0/public/petri-solver.js'
import { renderCard, renderCards, renderCommunityCards, parseCard } from './cards.js'

// Read API_BASE dynamically
function getApiBase() {
  return window.API_BASE || ''
}

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
  currentPlayer: 0
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

// Initialize application
document.addEventListener('DOMContentLoaded', () => {
  console.log('Texas Hold\'em Poker initialized')
  
  // Set up event listeners
  document.getElementById('new-game-btn').addEventListener('click', createNewGame)
  document.getElementById('start-hand-btn').addEventListener('click', startHand)
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
    gameState.players.forEach(p => {
      p.chips = 1000
      p.cards = []
      p.bet = 0
      p.folded = false
    })
    gameState.communityCards = { flop: [], turn: [], river: [] }
    
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
    showStatus('Hand started!', 'success')
    
    // Auto-deal preflop after a short delay
    setTimeout(() => dealPreflop(), 500)
  } catch (err) {
    console.error('Failed to start hand:', err)
    showStatus(`Error: ${err.message}`, 'error')
  }
}

/**
 * Deal preflop cards
 */
async function dealPreflop() {
  try {
    await executeTransition('deal_preflop', {})
    showStatus('Preflop dealt', 'success')
    
    // Simulate dealing cards to players (in real game, this comes from backend)
    gameState.players[0].cards = ['Ah', 'Kh']
    
    renderPokerTable()
    
    // Run ODE analysis
    await runODESimulation()
  } catch (err) {
    console.error('Failed to deal preflop:', err)
  }
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
// ODE Strategic Analysis
// ========================================================================

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
    
    // For each available action, build Petri net and solve ODE
    for (const action of availableActions) {
      const model = buildPokerODEPetriNet(gameState, action)
      const result = solveODE(model)
      
      if (result) {
        values[action.id] = result.expectedValue
        details[action.id] = result
      } else {
        values[action.id] = 0
      }
    }
    
    odeValues = { values, details }
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
 * Build Petri net model for poker state with hypothetical action
 * This is a simplified model focusing on win probability
 */
function buildPokerODEPetriNet(gameState, action) {
  const places = {}
  const transitions = {}
  const arcs = []
  
  // Simple model: track active players and pot
  // Places for each player (active/folded)
  for (let i = 0; i < 5; i++) {
    const isActive = !gameState.players[i].folded
    const willBeActive = !(action.type === 'fold' && i === gameState.currentPlayer)
    
    places[`P${i}_Active`] = {
      '@type': 'Place',
      initial: [isActive && willBeActive ? 1 : 0],
      x: 50 + i * 100,
      y: 50
    }
    
    places[`P${i}_Folded`] = {
      '@type': 'Place',
      initial: [isActive && willBeActive ? 0 : 1],
      x: 50 + i * 100,
      y: 150
    }
  }
  
  // Pot and win places
  places['Pot'] = {
    '@type': 'Place',
    initial: [gameState.pot + (action.amount || 0)],
    x: 300,
    y: 250
  }
  
  places['Win'] = {
    '@type': 'Place',
    initial: [0],
    x: 300,
    y: 350
  }
  
  // Simplified win transitions (player wins if others fold)
  const activePlayers = gameState.players.filter((p, i) => {
    if (i === gameState.currentPlayer && action.type === 'fold') return false
    return !p.folded
  })
  
  if (activePlayers.length === 1) {
    // Only one player left, they win
    const tid = 'WinByFold'
    transitions[tid] = { '@type': 'Transition', x: 300, y: 300 }
    arcs.push({ '@type': 'Arrow', source: 'Pot', target: tid, weight: [1] })
    arcs.push({ '@type': 'Arrow', source: tid, target: 'Win', weight: [1] })
  }
  
  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    places,
    transitions,
    arcs
  }
}

/**
 * Solve ODE and extract expected value
 */
function solveODE(model) {
  try {
    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = Solver.setRates(net)
    
    const prob = new Solver.ODEProblem(net, initialState, [0, solverParams.tspan], rates)
    const opts = { dt: solverParams.dt, adaptive: solverParams.adaptive }
    if (solverParams.adaptive) {
      opts.abstol = solverParams.abstol
      opts.reltol = solverParams.reltol
    }
    const solution = Solver.solve(prob, Solver.Tsit5(), opts)
    
    const finalState = solution.u ? solution.u[solution.u.length - 1] : null
    if (!finalState) return null
    
    // Expected value is the Win place value
    const expectedValue = finalState['Win'] || 0
    
    return {
      expectedValue,
      finalState
    }
  } catch (err) {
    console.error('ODE solve error:', err)
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
  // Update pot
  document.getElementById('pot-display').textContent = `Pot: $${gameState.pot}`
  
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
    
    // Recalculate ODE
    await runODESimulation()
  } catch (err) {
    console.error('Action failed:', err)
  }
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
    <p><strong>Current Player:</strong> Player ${gameState.currentPlayer}</p>
    <p><strong>Active Players:</strong> ${gameState.players.filter(p => !p.folded).length}</p>
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
function showStatus(message, type = 'info') {
  const statusEl = document.getElementById('status-message')
  statusEl.textContent = message
  statusEl.className = `status-message ${type}`
  statusEl.style.display = 'block'
  
  setTimeout(() => {
    statusEl.style.display = 'none'
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

// Export for console testing
window.gameState = gameState
window.runODESimulation = runODESimulation
window.buildPokerODEPetriNet = buildPokerODEPetriNet
window.solveODE = solveODE
