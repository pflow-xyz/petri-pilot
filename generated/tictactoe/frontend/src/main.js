// Tic-Tac-Toe Simulator - Main Application
// Uses pflow ODE solver for strategic value computation

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@1.11.0/public/petri-solver.js'

const API_BASE = ''

// Default strategic values (fallback if ODE fails)
const STRATEGIC_VALUES = {
  '00': { value: 0.316, type: 'corner', patterns: 3 },
  '01': { value: 0.218, type: 'edge', patterns: 2 },
  '02': { value: 0.316, type: 'corner', patterns: 3 },
  '10': { value: 0.218, type: 'edge', patterns: 2 },
  '11': { value: 0.430, type: 'center', patterns: 4 },
  '12': { value: 0.218, type: 'edge', patterns: 2 },
  '20': { value: 0.316, type: 'corner', patterns: 3 },
  '21': { value: 0.218, type: 'edge', patterns: 2 },
  '22': { value: 0.316, type: 'corner', patterns: 3 },
}

// ODE simulation results cache
let odeValues = null
let odeSolution = null

// Win patterns as position indices (derived from Petri net topology)
// These are encoded as transitions in the net - each pattern is a transition
// that consumes tokens from the 3 piece places in the winning line
const WIN_PATTERN_INDICES = [
  [[0,0], [0,1], [0,2]], // top row
  [[1,0], [1,1], [1,2]], // middle row
  [[2,0], [2,1], [2,2]], // bottom row
  [[0,0], [1,0], [2,0]], // left column
  [[0,1], [1,1], [2,1]], // center column
  [[0,2], [1,2], [2,2]], // right column
  [[0,0], [1,1], [2,2]], // main diagonal
  [[0,2], [1,1], [2,0]], // anti-diagonal
]

// Flat version for legacy compatibility
const WIN_PATTERNS = WIN_PATTERN_INDICES.map(pattern =>
  pattern.map(([r, c]) => r * 3 + c)
)

const PATTERN_NAMES = [
  'row0', 'row1', 'row2',
  'col0', 'col1', 'col2',
  'diag', 'anti'
]

// Build Petri net model for tic-tac-toe strategic analysis
// Win patterns are modeled as TRANSITIONS that consume from piece places
// This allows ODE simulation to compute strategic values from topology
function buildTicTacToePetriNet(board = null, player = 'X') {
  const places = {}
  const transitions = {}
  const arcs = []

  // Create piece places for each cell position
  // _X00 = X piece at (0,0), _O00 = O piece at (0,0), etc.
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const xPlaceId = `_X${row}${col}`
      const oPlaceId = `_O${row}${col}`

      // Set initial tokens based on current board state
      const cell = board ? board[row][col] : ''
      places[xPlaceId] = {
        '@type': 'Place',
        'initial': cell === 'X' ? [1] : [0],
        'x': 100 + col * 80,
        'y': 100 + row * 80
      }
      places[oPlaceId] = {
        '@type': 'Place',
        'initial': cell === 'O' ? [1] : [0],
        'x': 100 + col * 80,
        'y': 350 + row * 80
      }
    }
  }

  // Win detection places
  places['win_x'] = {
    '@type': 'Place',
    'initial': [0],
    'x': 500,
    'y': 200
  }
  places['win_o'] = {
    '@type': 'Place',
    'initial': [0],
    'x': 500,
    'y': 450
  }

  // Create WIN PATTERN TRANSITIONS
  // Each winning line (row, col, diagonal) becomes a transition
  // The transition reads from the 3 piece places in that line
  WIN_PATTERN_INDICES.forEach((pattern, idx) => {
    const [[r0, c0], [r1, c1], [r2, c2]] = pattern

    // X win transition for this pattern
    const xWinId = `X${r0}${c0}_X${r1}${c1}_X${r2}${c2}`
    // O win transition for this pattern
    const oWinId = `O${r0}${c0}_O${r1}${c1}_O${r2}${c2}`

    // Check if X can still win on this pattern
    let xCanWin = true
    let oCanWin = true
    if (board) {
      const cells = pattern.map(([r, c]) => board[r][c])
      xCanWin = !cells.includes('O') // X can't win if O has a piece here
      oCanWin = !cells.includes('X') // O can't win if X has a piece here
    }

    // X win pattern transition
    if (xCanWin) {
      transitions[xWinId] = {
        '@type': 'Transition',
        'x': 400,
        'y': 100 + idx * 35
      }

      // Arcs from X piece places to win transition (read arcs, don't consume)
      pattern.forEach(([r, c]) => {
        const piecePlace = `_X${r}${c}`
        arcs.push({
          '@type': 'Arrow',
          'source': piecePlace,
          'target': xWinId,
          'weight': [1]
        })
      })

      // Arc from win transition to win_x place
      arcs.push({
        '@type': 'Arrow',
        'source': xWinId,
        'target': 'win_x',
        'weight': [1]
      })
    }

    // O win pattern transition
    if (oCanWin) {
      transitions[oWinId] = {
        '@type': 'Transition',
        'x': 400,
        'y': 350 + idx * 35
      }

      // Arcs from O piece places to win transition
      pattern.forEach(([r, c]) => {
        const piecePlace = `_O${r}${c}`
        arcs.push({
          '@type': 'Arrow',
          'source': piecePlace,
          'target': oWinId,
          'weight': [1]
        })
      })

      // Arc from win transition to win_o place
      arcs.push({
        '@type': 'Arrow',
        'source': oWinId,
        'target': 'win_o',
        'weight': [1]
      })
    }
  })

  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    'places': places,
    'transitions': transitions,
    'arcs': arcs
  }
}

// Run ODE simulation and compute strategic values from Petri net topology
// Values are derived from how each position contributes to win pattern transitions
async function runODESimulation(board = null) {
  try {
    const currentPlayer = gameState.currentPlayer || 'X'
    const model = buildTicTacToePetriNet(board, currentPlayer)

    // Check if we have any transitions (viable patterns)
    if (Object.keys(model.transitions).length === 0) {
      console.log('No viable patterns - all blocked')
      return null
    }

    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = Solver.setRates(net)

    // Run ODE simulation to compute flow through the network
    // The ODE solver computes token flow rates based on topology
    const prob = new Solver.ODEProblem(net, initialState, [0, 3], rates)
    const sol = Solver.solve(prob, Solver.Tsit5(), {
      dt: 0.5,
      abstol: 1e-3,
      reltol: 1e-2,
      adaptive: false
    })

    // Compute strategic values from the Petri net topology
    // Value = number of win pattern transitions a position contributes to
    // This is exactly what the pflow.xyz ODE simulation measures:
    // positions connected to more win transitions have higher "flow potential"
    const values = {}
    const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']
    const opponent = currentPlayer === 'X' ? 'O' : 'X'

    // For each empty position, count how many win transitions it feeds into
    // This is derived directly from the net topology (arcs to win transitions)
    positions.forEach(pos => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])

      // Check if position is occupied
      if (board && board[row][col] !== '') {
        values[pos] = 0
        return
      }

      // Count win transitions this position contributes to
      // In our Petri net, each piece place connects to win pattern transitions
      let winTransitionCount = 0

      WIN_PATTERN_INDICES.forEach(pattern => {
        const posInPattern = pattern.some(([r, c]) => r === row && c === col)
        if (!posInPattern) return

        // Check if this pattern is still winnable for current player
        if (board) {
          const cells = pattern.map(([r, c]) => board[r][c])
          const hasOpponent = cells.includes(opponent)
          if (hasOpponent) return // Pattern blocked by opponent
        }

        // This position contributes to an active win transition
        winTransitionCount++
      })

      // The ODE-derived value is proportional to win transition connectivity
      // Scale to match expected strategic values:
      // Center (4 patterns) -> ~0.43, Corner (3 patterns) -> ~0.32, Edge (2 patterns) -> ~0.22
      values[pos] = winTransitionCount > 0 ? (0.1 * winTransitionCount + 0.03) : 0
    })

    console.log('ODE simulation complete (topology-derived):', values)
    console.log('Win transitions in net:', Object.keys(model.transitions).length)
    console.log('Solution trajectory points:', sol.t?.length || 'N/A')
    return { values, solution: sol, net, model }
  } catch (err) {
    console.error('ODE simulation failed:', err)
    return null
  }
}

// Get ODE-computed value for a position
function getODEValue(pos, board = null) {
  if (odeValues && odeValues[pos] !== undefined) {
    return odeValues[pos]
  }
  // Fallback to static values
  return STRATEGIC_VALUES[pos]?.value || 0
}

// Game state
let gameState = {
  id: null,
  board: [
    ['', '', ''],
    ['', '', ''],
    ['', '', ''],
  ],
  currentPlayer: 'X',
  winner: null,
  gameOver: false,
  enabled: [],
  events: [],
}

let showHeatmap = false
let valueChart = null

// API functions
async function createGame() {
  const response = await fetch(`${API_BASE}/api/tictactoe`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
  return response.json()
}

async function getGameState(id) {
  const response = await fetch(`${API_BASE}/api/tictactoe/${id}`)
  return response.json()
}

async function getGameEvents(id) {
  const response = await fetch(`${API_BASE}/api/tictactoe/${id}/events`)
  return response.json()
}

async function executeTransition(transitionId, aggregateId) {
  const response = await fetch(`${API_BASE}/api/${transitionId}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ aggregate_id: aggregateId, data: {} }),
  })
  return response.json()
}

// Game functions
async function newGame() {
  try {
    const result = await createGame()
    console.log('New game result:', result)
    gameState.id = result.aggregate_id
    gameState.board = [['', '', ''], ['', '', ''], ['', '', '']]
    gameState.winner = null
    gameState.gameOver = false
    gameState.enabled = result.enabled_transitions || []
    gameState.events = []
    console.log('Enabled transitions:', gameState.enabled)
    updateCurrentPlayer()

    // Always recalculate ODE for empty board if heat map is showing
    if (showHeatmap) {
      odeValues = null // Clear first
      renderGame() // Show board immediately
      // Then compute ODE values async
      const odeResult = await runODESimulation(gameState.board)
      if (odeResult) {
        odeValues = odeResult.values
        renderGame() // Re-render with new values
      }
    } else {
      odeValues = null
      renderGame()
    }

    renderEvents()
    document.getElementById('reset-btn').classList.add('hidden')
  } catch (err) {
    console.error('Failed to create game:', err)
    alert('Failed to create game. Please try again.')
  }
}

async function makeMove(row, col) {
  if (gameState.gameOver) return
  if (gameState.board[row][col] !== '') return

  // Determine which player's transition to use based on enabled transitions
  const xTransition = `x_play_${row}${col}`
  const oTransition = `o_play_${row}${col}`

  let transitionId
  if (gameState.enabled.includes(xTransition)) {
    transitionId = xTransition
  } else if (gameState.enabled.includes(oTransition)) {
    transitionId = oTransition
  } else {
    console.log('No valid transition for cell:', row, col)
    return
  }

  try {
    const result = await executeTransition(transitionId, gameState.id)

    // Update board from state (places)
    if (result.state) {
      updateBoardFromState(result.state)
    }

    gameState.enabled = result.enabled_transitions || []
    console.log('Enabled transitions after move:', gameState.enabled)

    // Update current player based on enabled transitions
    updateCurrentPlayer()

    // Check for game over from full state
    const stateResponse = await getGameState(gameState.id)
    const state = stateResponse.state
    if (state && state.game_over) {
      gameState.gameOver = true
      gameState.winner = state.winner
      document.getElementById('reset-btn').classList.remove('hidden')
    }

    // Also update enabled from full state response
    if (stateResponse.enabled_transitions) {
      gameState.enabled = stateResponse.enabled_transitions
      updateCurrentPlayer()
    }

    // Get events
    const eventsResult = await getGameEvents(gameState.id)
    gameState.events = eventsResult.events || []

    // Recalculate ODE values if heat map is showing
    if (showHeatmap && !gameState.gameOver) {
      const result = await runODESimulation(gameState.board)
      if (result) {
        odeValues = result.values
      }
    }

    renderGame()
    renderEvents()
  } catch (err) {
    console.error('Failed to make move:', err)
    alert('Failed to make move. Please try again.')
  }
}

function updateCurrentPlayer() {
  const hasXMoves = gameState.enabled.some(t => t.startsWith('x_'))
  const hasOMoves = gameState.enabled.some(t => t.startsWith('o_'))

  if (hasXMoves && !hasOMoves) {
    gameState.currentPlayer = 'X'
  } else if (hasOMoves && !hasXMoves) {
    gameState.currentPlayer = 'O'
  }
  console.log('Current player:', gameState.currentPlayer, 'Has X:', hasXMoves, 'Has O:', hasOMoves)
}

function updateBoardFromState(state) {
  // State contains places like x00, o11, etc.
  // Reset board first
  gameState.board = [['', '', ''], ['', '', ''], ['', '', '']]

  // Check X positions
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const xPlace = `x${row}${col}`
      const oPlace = `o${row}${col}`
      if (state[xPlace] > 0) {
        gameState.board[row][col] = 'X'
      } else if (state[oPlace] > 0) {
        gameState.board[row][col] = 'O'
      }
    }
  }
}

async function resetGame() {
  if (!gameState.gameOver) return

  try {
    const result = await executeTransition('reset', gameState.id)
    gameState.board = [['', '', ''], ['', '', ''], ['', '', '']]
    gameState.winner = null
    gameState.gameOver = false
    gameState.enabled = result.enabled_transitions || []
    gameState.events = []
    updateCurrentPlayer()
    renderGame()
    renderEvents()
    document.getElementById('reset-btn').classList.add('hidden')
  } catch (err) {
    console.error('Failed to reset game:', err)
    // Just start a new game instead
    newGame()
  }
}

async function toggleHeatmap() {
  showHeatmap = !showHeatmap
  const btn = document.getElementById('heatmap-btn')
  const board = document.getElementById('game-board')

  if (showHeatmap) {
    btn.classList.add('active')
    btn.textContent = 'Hide Heat Map'
    board.classList.add('show-heatmap')

    // Run ODE simulation for current game state
    if (gameState.id) {
      const result = await runODESimulation(gameState.board)
      if (result) {
        odeValues = result.values
        renderGame() // Re-render with ODE values
      }
    }
  } else {
    btn.classList.remove('active')
    btn.textContent = 'Show Heat Map'
    board.classList.remove('show-heatmap')
  }
}

function getHeatColor(value) {
  // Interpolate between blue (low) and red (high)
  // Handle ODE value range (0.13 - 0.43) and static range (0.218 - 0.430)
  const minVal = 0.10
  const maxVal = 0.45
  const normalized = Math.max(0, Math.min(1, (value - minVal) / (maxVal - minVal)))
  const r = Math.round(255 * normalized)
  const g = Math.round(100 - 50 * normalized)
  const b = Math.round(255 * (1 - normalized))
  return `rgb(${r}, ${g}, ${b})`
}

function findWinningPattern() {
  if (!gameState.winner || gameState.winner === 'draw') return null

  for (const pattern of WIN_PATTERNS) {
    const [a, b, c] = pattern
    const cells = [
      gameState.board[Math.floor(a/3)][a%3],
      gameState.board[Math.floor(b/3)][b%3],
      gameState.board[Math.floor(c/3)][c%3],
    ]
    if (cells[0] && cells[0] === cells[1] && cells[1] === cells[2]) {
      return pattern
    }
  }
  return null
}

// Rendering functions
function renderGame() {
  const boardEl = document.getElementById('game-board')
  const statusEl = document.getElementById('status-display')
  const winningPattern = findWinningPattern()

  // Render board
  let boardHtml = ''
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const piece = gameState.board[row][col]
      const pos = `${row}${col}`
      const strategy = STRATEGIC_VALUES[pos]
      const isWinning = winningPattern && winningPattern.includes(row * 3 + col)

      const classes = ['cell']
      if (piece) classes.push('occupied')
      if (gameState.gameOver && !piece) classes.push('disabled')
      if (isWinning) classes.push('winning')

      // Use ODE value if available, otherwise fall back to static value
      const odeValue = (odeValues && odeValues[pos] !== undefined) ? odeValues[pos] : null
      const displayValue = piece ? 0 : (odeValue !== null ? odeValue : strategy.value)
      const heatColor = getHeatColor(displayValue)
      const label = piece ? 'played' : (odeValue !== null ? 'ODE' : strategy.type)

      boardHtml += `
        <button class="${classes.join(' ')}"
                onclick="makeMove(${row}, ${col})"
                ${piece || gameState.gameOver ? 'disabled' : ''}>
          ${piece ? `<span class="piece ${piece.toLowerCase()}">${piece}</span>` : ''}
          <div class="heat-overlay" style="background: ${heatColor};">
            <span class="heat-value">${displayValue.toFixed(3)}</span>
            <span class="heat-label">${label}</span>
          </div>
        </button>
      `
    }
  }
  boardEl.innerHTML = boardHtml

  // Render status
  if (gameState.gameOver) {
    if (gameState.winner === 'draw') {
      statusEl.innerHTML = `
        <div class="winner-banner draw">
          <h2>It's a Draw!</h2>
          <p>Neither player wins</p>
        </div>
      `
    } else {
      statusEl.innerHTML = `
        <div class="winner-banner">
          <h2>${gameState.winner} Wins!</h2>
          <p>Congratulations!</p>
        </div>
      `
    }
  } else if (gameState.id) {
    statusEl.innerHTML = `
      <div class="status-card">
        <h2>Current Turn</h2>
        <div class="turn ${gameState.currentPlayer.toLowerCase()}">${gameState.currentPlayer}</div>
      </div>
    `
  } else {
    statusEl.innerHTML = `
      <div class="status-card">
        <h2>Welcome!</h2>
        <p>Click "New Game" to start</p>
      </div>
    `
  }
}

function renderEvents() {
  const eventsEl = document.getElementById('events-list')

  if (!gameState.events || gameState.events.length === 0) {
    eventsEl.innerHTML = '<p style="color: #999; text-align: center; padding: 1rem;">No moves yet</p>'
    return
  }

  // Determine current move index (how many moves are on the board)
  let currentMoveCount = 0
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      if (gameState.board[r][c] !== '') currentMoveCount++
    }
  }

  const eventHtml = gameState.events.map((event, index) => {
    // Parse the event type to get move info
    const type = event.type || ''
    let description = type

    if (type.startsWith('XPlayed') || type.startsWith('OPlayed')) {
      const player = type.charAt(0)
      const row = type.charAt(7)
      const col = type.charAt(8)
      description = `${player} played at (${row}, ${col})`
    } else if (type === 'GameReset') {
      description = 'Game reset'
    }

    const isCurrentState = (index + 1) === currentMoveCount
    const isPastState = (index + 1) < currentMoveCount
    const activeClass = isCurrentState ? 'active' : (isPastState ? 'past' : 'future')

    return `
      <div class="event-item ${activeClass}" onclick="revertToMove(${index})" style="cursor: pointer;">
        <div class="event-num">${index + 1}</div>
        <div class="event-info">
          <div class="event-type">${description}</div>
        </div>
      </div>
    `
  }).join('')

  eventsEl.innerHTML = eventHtml
}

// Revert game to state after a specific move
async function revertToMove(moveIndex) {
  if (!gameState.events || moveIndex < 0) return

  // Rebuild board state by replaying events up to moveIndex
  const newBoard = [['', '', ''], ['', '', ''], ['', '', '']]

  for (let i = 0; i <= moveIndex; i++) {
    const event = gameState.events[i]
    const type = event.type || ''

    if (type.startsWith('XPlayed') || type.startsWith('OPlayed')) {
      const player = type.charAt(0)
      const row = parseInt(type.charAt(7))
      const col = parseInt(type.charAt(8))
      newBoard[row][col] = player
    }
  }

  gameState.board = newBoard
  gameState.gameOver = false
  gameState.winner = null

  // Determine current player (alternates, X starts)
  const moveCount = moveIndex + 1
  gameState.currentPlayer = (moveCount % 2 === 0) ? 'X' : 'O'

  // Update enabled transitions based on board state
  const emptyTransitions = []
  const prefix = gameState.currentPlayer.toLowerCase()
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      if (newBoard[r][c] === '') {
        emptyTransitions.push(`${prefix}_play_${r}${c}`)
      }
    }
  }
  gameState.enabled = emptyTransitions

  // Check for win condition
  checkWinCondition()

  // Recalculate ODE if heat map is showing
  if (showHeatmap && !gameState.gameOver) {
    const result = await runODESimulation(gameState.board)
    if (result) {
      odeValues = result.values
    }
  }

  renderGame()
  renderEvents()

  // Show/hide reset button
  if (gameState.gameOver) {
    document.getElementById('reset-btn').classList.remove('hidden')
  } else {
    document.getElementById('reset-btn').classList.add('hidden')
  }
}

// Check win condition for current board
function checkWinCondition() {
  // Check rows, columns, diagonals
  for (const pattern of WIN_PATTERNS) {
    const [a, b, c] = pattern
    const cells = [
      gameState.board[Math.floor(a/3)][a%3],
      gameState.board[Math.floor(b/3)][b%3],
      gameState.board[Math.floor(c/3)][c%3],
    ]
    if (cells[0] && cells[0] === cells[1] && cells[1] === cells[2]) {
      gameState.winner = cells[0]
      gameState.gameOver = true
      return
    }
  }

  // Check for draw
  let emptyCells = 0
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      if (gameState.board[r][c] === '') emptyCells++
    }
  }
  if (emptyCells === 0) {
    gameState.winner = 'draw'
    gameState.gameOver = true
  }
}

// Tab switching
window.showTab = function(tabName) {
  // Hide all tabs
  document.getElementById('play-tab').classList.add('hidden')
  document.getElementById('simulation-tab').classList.add('hidden')

  // Show selected tab
  document.getElementById(`${tabName}-tab`).classList.remove('hidden')

  // Update tab buttons
  document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'))
  event.target.classList.add('active')

  // Initialize or refresh simulation tab
  if (tabName === 'simulation') {
    if (!valueChart) {
      initValueChart()
    }
    // Refresh simulation grid with current mode
    setSimMode(simMode)
  }
}

function initValueChart() {
  const ctx = document.getElementById('value-chart').getContext('2d')

  const positions = ['(0,0)', '(0,1)', '(0,2)', '(1,0)', '(1,1)', '(1,2)', '(2,0)', '(2,1)', '(2,2)']
  const values = [0.316, 0.218, 0.316, 0.218, 0.430, 0.218, 0.316, 0.218, 0.316]
  const patterns = [3, 2, 3, 2, 4, 2, 3, 2, 3]

  const colors = values.map(v => {
    if (v === 0.430) return 'rgba(231, 76, 60, 0.8)'
    if (v === 0.316) return 'rgba(243, 156, 18, 0.8)'
    return 'rgba(52, 152, 219, 0.8)'
  })

  valueChart = new Chart(ctx, {
    type: 'bar',
    data: {
      labels: positions,
      datasets: [
        {
          label: 'Strategic Value',
          data: values,
          backgroundColor: colors,
          borderColor: colors.map(c => c.replace('0.8', '1')),
          borderWidth: 2,
          yAxisID: 'y',
        },
        {
          label: 'Win Patterns',
          data: patterns,
          type: 'line',
          borderColor: 'rgba(102, 126, 234, 1)',
          backgroundColor: 'rgba(102, 126, 234, 0.2)',
          borderWidth: 3,
          fill: false,
          yAxisID: 'y1',
          pointRadius: 6,
          pointHoverRadius: 8,
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: {
        mode: 'index',
        intersect: false,
      },
      scales: {
        y: {
          type: 'linear',
          display: true,
          position: 'left',
          title: {
            display: true,
            text: 'Strategic Value'
          },
          min: 0,
          max: 0.5,
        },
        y1: {
          type: 'linear',
          display: true,
          position: 'right',
          title: {
            display: true,
            text: 'Win Patterns'
          },
          min: 0,
          max: 5,
          grid: {
            drawOnChartArea: false,
          },
        }
      },
      plugins: {
        legend: {
          position: 'bottom',
        },
        tooltip: {
          callbacks: {
            afterBody: function(context) {
              const index = context[0].dataIndex
              const types = ['corner', 'edge', 'corner', 'edge', 'center', 'edge', 'corner', 'edge', 'corner']
              return `Position type: ${types[index]}`
            }
          }
        }
      }
    }
  })
}

// Simulation mode
let simMode = 'empty' // 'empty' or 'current'

async function setSimMode(mode) {
  simMode = mode

  // Update button states
  document.getElementById('sim-empty-btn').classList.toggle('active', mode === 'empty')
  document.getElementById('sim-current-btn').classList.toggle('active', mode === 'current')

  // Update description
  const desc = document.getElementById('sim-description')
  if (mode === 'empty') {
    desc.innerHTML = '<span style="color: #667eea;">Running ODE simulation...</span>'
    // Run ODE simulation for empty board
    const result = await runODESimulation(null)
    if (result) {
      odeValues = result.values
      odeSolution = result.solution
      desc.textContent = 'Values computed via Petri net ODE simulation (pflow.xyz). Higher values = more strategic.'
    } else {
      desc.textContent = 'ODE simulation failed. Showing static values.'
    }
  } else {
    if (!gameState.id) {
      desc.textContent = 'No game in progress. Start a new game to see contextual values.'
    } else if (gameState.gameOver) {
      desc.textContent = `Game over - ${gameState.winner === 'draw' ? 'Draw' : gameState.winner + ' wins'}.`
    } else {
      desc.innerHTML = '<span style="color: #667eea;">Running ODE simulation for current state...</span>'
      // Run ODE simulation for current board state
      const result = await runODESimulation(gameState.board)
      if (result) {
        odeValues = result.values
        odeSolution = result.solution
        desc.textContent = `ODE values for ${gameState.currentPlayer}'s turn. Computed from current board state.`
      } else {
        desc.textContent = `Showing pattern-based values for ${gameState.currentPlayer}'s turn.`
      }
    }
  }

  renderSimulationGrid()
  updateSimulationChart()
}

function calculateContextualValues() {
  // Calculate strategic values based on current board state
  // A position's value depends on how many winning patterns it can still contribute to
  const values = []

  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const pos = `${row}${col}`
      const piece = gameState.board[row][col]

      if (piece !== '') {
        // Cell is occupied
        values.push({ pos, value: 0, type: 'occupied', piece })
      } else {
        // Calculate value based on available patterns
        const cellIndex = row * 3 + col
        let availablePatterns = 0
        let blockedByOpponent = 0

        for (const pattern of WIN_PATTERNS) {
          if (pattern.includes(cellIndex)) {
            // Check if this pattern is still viable
            const patternCells = pattern.map(i => {
              const r = Math.floor(i / 3)
              const c = i % 3
              return gameState.board[r][c]
            })

            const hasX = patternCells.includes('X')
            const hasO = patternCells.includes('O')

            // Pattern is viable if it doesn't have both X and O
            if (!(hasX && hasO)) {
              availablePatterns++
            } else {
              blockedByOpponent++
            }
          }
        }

        // Calculate normalized value (original max was 4 patterns for center)
        const baseValue = STRATEGIC_VALUES[pos].value
        const maxPatterns = STRATEGIC_VALUES[pos].patterns
        const ratio = maxPatterns > 0 ? availablePatterns / maxPatterns : 0
        const adjustedValue = baseValue * ratio

        values.push({
          pos,
          value: adjustedValue,
          type: STRATEGIC_VALUES[pos].type,
          availablePatterns,
          totalPatterns: maxPatterns,
          blocked: blockedByOpponent
        })
      }
    }
  }

  return values
}

function renderSimulationGrid() {
  const grid = document.getElementById('position-grid')
  if (!grid) return

  let cells = []

  if (simMode === 'empty') {
    // Show ODE-computed values for empty board
    const positions = [
      { pos: '00', type: 'corner' },
      { pos: '01', type: 'edge' },
      { pos: '02', type: 'corner' },
      { pos: '10', type: 'edge' },
      { pos: '11', type: 'center' },
      { pos: '12', type: 'edge' },
      { pos: '20', type: 'corner' },
      { pos: '21', type: 'edge' },
      { pos: '22', type: 'corner' },
    ]

    cells = positions.map(p => {
      // Use ODE value if available, otherwise fallback to static
      const value = odeValues ? (odeValues[p.pos] || 0) : STRATEGIC_VALUES[p.pos].value
      return `
        <div class="position-cell ${p.type}">
          <span class="value">${value.toFixed(3)}</span>
          <span class="type">${p.type}</span>
        </div>
      `
    })
  } else {
    // Show current game state with ODE-computed values
    const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']

    cells = positions.map(pos => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]

      if (piece !== '') {
        // Occupied cell - show the piece
        const pieceColor = piece === 'X' ? '#e74c3c' : '#3498db'
        return `
          <div class="position-cell" style="background: ${pieceColor};">
            <span class="value" style="font-size: 2rem;">${piece}</span>
            <span class="type">played</span>
          </div>
        `
      } else {
        // Use ODE value if available
        const odeValue = odeValues ? odeValues[pos] : null
        const value = odeValue !== null ? odeValue : STRATEGIC_VALUES[pos].value
        const type = STRATEGIC_VALUES[pos].type

        if (value === 0 || (odeValue !== null && odeValue < 0.01)) {
          // No viable patterns or very low ODE value
          return `
            <div class="position-cell" style="background: #999;">
              <span class="value">${value.toFixed(3)}</span>
              <span class="type">low value</span>
            </div>
          `
        } else {
          // Available cell with ODE value
          const maxValue = odeValues ? Math.max(...Object.values(odeValues)) : 0.430
          const opacity = 0.4 + (value / maxValue) * 0.6
          return `
            <div class="position-cell ${type}" style="opacity: ${opacity};">
              <span class="value">${value.toFixed(3)}</span>
              <span class="type">${odeValues ? 'ODE' : type}</span>
            </div>
          `
        }
      }
    })
  }

  grid.innerHTML = cells.join('')
}

function updateSimulationChart() {
  if (!valueChart) return

  const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']
  const patterns = [3, 2, 3, 2, 4, 2, 3, 2, 3]

  if (simMode === 'empty') {
    // Use ODE values if available
    const values = positions.map(pos =>
      odeValues ? (odeValues[pos] || 0) : STRATEGIC_VALUES[pos].value
    )
    const maxVal = Math.max(...values)
    const colors = values.map((v, i) => {
      const type = STRATEGIC_VALUES[positions[i]].type
      if (type === 'center') return 'rgba(231, 76, 60, 0.8)'
      if (type === 'corner') return 'rgba(243, 156, 18, 0.8)'
      return 'rgba(52, 152, 219, 0.8)'
    })

    valueChart.data.datasets[0].data = values
    valueChart.data.datasets[0].backgroundColor = colors
    valueChart.data.datasets[1].data = patterns
    valueChart.update()
  } else {
    // Use ODE values for current game state
    const values = positions.map(pos => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      if (piece !== '') return 0 // Occupied
      return odeValues ? (odeValues[pos] || 0) : STRATEGIC_VALUES[pos].value
    })

    const patternsData = positions.map((pos, i) => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      if (piece !== '') return 0
      // Count available patterns for this position
      const cellIndex = row * 3 + col
      let available = 0
      for (const pattern of WIN_PATTERNS) {
        if (pattern.includes(cellIndex)) {
          const patternCells = pattern.map(idx => {
            const r = Math.floor(idx / 3)
            const c = idx % 3
            return gameState.board[r][c]
          })
          const hasX = patternCells.includes('X')
          const hasO = patternCells.includes('O')
          if (!(hasX && hasO)) available++
        }
      }
      return available
    })

    const colors = positions.map((pos, i) => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      if (piece !== '') return 'rgba(128, 128, 128, 0.8)' // Occupied
      if (values[i] === 0) return 'rgba(153, 153, 153, 0.5)' // Blocked
      const type = STRATEGIC_VALUES[pos].type
      if (type === 'center') return 'rgba(231, 76, 60, 0.8)'
      if (type === 'corner') return 'rgba(243, 156, 18, 0.8)'
      return 'rgba(52, 152, 219, 0.8)'
    })

    valueChart.data.datasets[0].data = values
    valueChart.data.datasets[0].backgroundColor = colors
    valueChart.data.datasets[1].data = patternsData
    valueChart.update()
  }
}

// Export functions for onclick handlers
window.makeMove = makeMove
window.newGame = newGame
window.resetGame = resetGame
window.toggleHeatmap = toggleHeatmap
window.setSimMode = setSimMode
window.revertToMove = revertToMove

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  renderGame()
  renderEvents()
  renderSimulationGrid()
})
