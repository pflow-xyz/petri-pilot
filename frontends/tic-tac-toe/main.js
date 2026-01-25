// Tic-Tac-Toe Simulator - Main Application
// Uses pflow ODE solver for strategic value computation

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@1.11.0/public/petri-solver.js'

// Read API_BASE dynamically to avoid race condition with inline script
function getApiBase() {
  return window.API_BASE || ''
}

// ODE simulation results cache
let odeValues = null
let odeSolution = null

// Configurable ODE solver parameters
let solverParams = {
  tspan: 2.0,
  dt: 0.2,
  adaptive: false,
  abstol: 1e-4,
  reltol: 1e-3
}

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
// Matches go-pflow structure: https://github.com/pflow-xyz/go-pflow/blob/main/examples/tictactoe/metamodel/tictactoe.go
// - Board positions P00-P22 (initial=1 if empty)
// - Move history X00-X22, O00-O22 (initial=0, set to 1 when played)
// - Turn control: Next (0=X's turn, 1=O's turn)
// - Win detection: WinX, WinO
function buildTicTacToePetriNet(board = null, player = 'X') {
  const places = {}
  const transitions = {}
  const arcs = []

  // Determine current turn from board state
  let xCount = 0, oCount = 0
  if (board) {
    for (let r = 0; r < 3; r++) {
      for (let c = 0; c < 3; c++) {
        if (board[r][c] === 'X') xCount++
        if (board[r][c] === 'O') oCount++
      }
    }
  }
  const isXTurn = xCount === oCount // X goes first, so equal means X's turn

  // Board position places (P00-P22): 1 if empty, 0 if taken
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const posId = `P${row}${col}`
      const cell = board ? board[row][col] : ''
      places[posId] = {
        '@type': 'Place',
        'initial': cell === '' ? [1] : [0],
        'x': 50 + col * 60,
        'y': 50 + row * 60
      }
    }
  }

  // X move history places (X00-X22): 1 if X played here
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const xId = `X${row}${col}`
      const cell = board ? board[row][col] : ''
      places[xId] = {
        '@type': 'Place',
        'initial': cell === 'X' ? [1] : [0],
        'x': 200 + col * 60,
        'y': 50 + row * 60
      }
    }
  }

  // O move history places (O00-O22): 1 if O played here
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const oId = `O${row}${col}`
      const cell = board ? board[row][col] : ''
      places[oId] = {
        '@type': 'Place',
        'initial': cell === 'O' ? [1] : [0],
        'x': 350 + col * 60,
        'y': 50 + row * 60
      }
    }
  }

  // Turn control: Next (0=X's turn, 1=O's turn)
  places['Next'] = {
    '@type': 'Place',
    'initial': isXTurn ? [0] : [1],
    'x': 250,
    'y': 250
  }

  // Win detection places
  places['WinX'] = {
    '@type': 'Place',
    'initial': [0],
    'x': 500,
    'y': 100
  }
  places['WinO'] = {
    '@type': 'Place',
    'initial': [0],
    'x': 500,
    'y': 200
  }

  // X move transitions: Position -> PlayX -> History + Next
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const posId = `P${row}${col}`
      const xHistId = `X${row}${col}`
      const playXId = `PlayX${row}${col}`

      // Only add if position is empty (or building full model)
      const cell = board ? board[row][col] : ''
      if (cell === '') {
        transitions[playXId] = {
          '@type': 'Transition',
          'x': 120 + col * 60,
          'y': 50 + row * 60
        }
        // Position -> PlayX
        arcs.push({ '@type': 'Arrow', 'source': posId, 'target': playXId, 'weight': [1] })
        // PlayX -> History
        arcs.push({ '@type': 'Arrow', 'source': playXId, 'target': xHistId, 'weight': [1] })
        // PlayX -> Next (produces turn token for O)
        arcs.push({ '@type': 'Arrow', 'source': playXId, 'target': 'Next', 'weight': [1] })
      }
    }
  }

  // O move transitions: Next + Position -> PlayO -> History
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const posId = `P${row}${col}`
      const oHistId = `O${row}${col}`
      const playOId = `PlayO${row}${col}`

      const cell = board ? board[row][col] : ''
      if (cell === '') {
        transitions[playOId] = {
          '@type': 'Transition',
          'x': 270 + col * 60,
          'y': 50 + row * 60
        }
        // Next -> PlayO (consumes turn token)
        arcs.push({ '@type': 'Arrow', 'source': 'Next', 'target': playOId, 'weight': [1] })
        // Position -> PlayO
        arcs.push({ '@type': 'Arrow', 'source': posId, 'target': playOId, 'weight': [1] })
        // PlayO -> History
        arcs.push({ '@type': 'Arrow', 'source': playOId, 'target': oHistId, 'weight': [1] })
      }
    }
  }

  // X win detection transitions: 3-in-a-row -> WinX
  const winPatternNames = ['Row0', 'Row1', 'Row2', 'Col0', 'Col1', 'Col2', 'Dg0', 'Dg1']
  WIN_PATTERN_INDICES.forEach((pattern, idx) => {
    const xWinId = `X${winPatternNames[idx]}`
    const oWinId = `O${winPatternNames[idx]}`

    // Check if patterns are still possible
    let xCanWin = true, oCanWin = true
    if (board) {
      const cells = pattern.map(([r, c]) => board[r][c])
      xCanWin = !cells.includes('O')
      oCanWin = !cells.includes('X')
    }

    if (xCanWin) {
      transitions[xWinId] = {
        '@type': 'Transition',
        'x': 450,
        'y': 50 + idx * 25
      }
      pattern.forEach(([r, c]) => {
        arcs.push({ '@type': 'Arrow', 'source': `X${r}${c}`, 'target': xWinId, 'weight': [1] })
      })
      arcs.push({ '@type': 'Arrow', 'source': xWinId, 'target': 'WinX', 'weight': [1] })
    }

    if (oCanWin) {
      transitions[oWinId] = {
        '@type': 'Transition',
        'x': 450,
        'y': 250 + idx * 25
      }
      pattern.forEach(([r, c]) => {
        arcs.push({ '@type': 'Arrow', 'source': `O${r}${c}`, 'target': oWinId, 'weight': [1] })
      })
      arcs.push({ '@type': 'Arrow', 'source': oWinId, 'target': 'WinO', 'weight': [1] })
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

// Compute strategic values using Go backend ODE API
// Calls /api/heatmap endpoint which uses go-pflow ODE solver
// Use local JS ODE by default, set to false to use Go backend API
let useLocalODE = true

async function runODESimulation(board = null) {
  if (useLocalODE) {
    return runLocalODESimulation(board)
  } else {
    return runAPIHeatmap(board)
  }
}

// Local JavaScript ODE computation using pflow.xyz solver
// Mirrors the Go backend logic in ode.go
// Note: Solver has shared state, so we run sequentially
function runLocalODESimulation(board = null) {
  try {
    const values = {}
    const details = {}
    const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']

    // Determine current player
    let xCount = 0, oCount = 0
    if (board) {
      for (let r = 0; r < 3; r++) {
        for (let c = 0; c < 3; c++) {
          if (board[r][c] === 'X') xCount++
          if (board[r][c] === 'O') oCount++
        }
      }
    }
    const currentPlayer = xCount > oCount ? 'O' : 'X'

    console.log(`Local ODE: currentPlayer=${currentPlayer}, board=`, board)

    // Run ODE simulations sequentially (Solver has shared state)
    for (const pos of positions) {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])

      // Skip occupied positions
      if (board && board[row][col] !== '') {
        values[pos] = 0
        continue
      }

      // Build Petri net with hypothetical move
      const model = buildODEPetriNet(board, currentPlayer, row, col)

      // Run ODE simulation
      const result = solveODE(model)

      if (result) {
        const { winX, winO } = result
        // Score from current player's perspective
        const score = currentPlayer === 'X' ? (winX - winO) : (winO - winX)
        values[pos] = score
        details[pos] = { WinX: winX, WinO: winO, score }
      } else {
        values[pos] = 0
      }
    }

    console.log('Local ODE values:', values)
    return { values, details, player: currentPlayer, solution: null, model: null }
  } catch (err) {
    console.error('Local ODE failed:', err)
    return null
  }
}

// Ensure ODE values are computed (called before rendering if needed)
function ensureODEValues(board = null) {
  if (odeValues === null) {
    const result = runLocalODESimulation(board)
    if (result) {
      odeValues = result.values
    }
  }
  return odeValues
}

// Build Petri net for ODE with hypothetical move applied
// Matches go-pflow structure: 30 places, 34 transitions
function buildODEPetriNet(board, currentPlayer, hypRow, hypCol) {
  const places = {}
  const transitions = {}
  const arcs = []

  // Board position places P00-P22
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const id = `P${r}${c}`
      let initial = 1 // empty
      if (board && board[r][c] !== '') initial = 0
      // Apply hypothetical move
      if (r === hypRow && c === hypCol) initial = 0
      places[id] = { '@type': 'Place', initial: [initial], x: 50 + c * 60, y: 50 + r * 60 }
    }
  }

  // X history places X00-X22
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const id = `X${r}${c}`
      let initial = 0
      if (board && board[r][c] === 'X') initial = 1
      // Apply hypothetical move for X
      if (currentPlayer === 'X' && r === hypRow && c === hypCol) initial = 1
      places[id] = { '@type': 'Place', initial: [initial], x: 200 + c * 60, y: 50 + r * 60 }
    }
  }

  // O history places O00-O22
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const id = `O${r}${c}`
      let initial = 0
      if (board && board[r][c] === 'O') initial = 1
      // Apply hypothetical move for O
      if (currentPlayer === 'O' && r === hypRow && c === hypCol) initial = 1
      places[id] = { '@type': 'Place', initial: [initial], x: 350 + c * 60, y: 50 + r * 60 }
    }
  }

  // Turn control: Next (0=X's turn, 1=O's turn)
  // After hypothetical move, it's opponent's turn
  const nextTurn = currentPlayer === 'X' ? 1 : 0
  places['Next'] = { '@type': 'Place', initial: [nextTurn], x: 250, y: 250 }

  // Win detection places
  places['WinX'] = { '@type': 'Place', initial: [0], x: 500, y: 100 }
  places['WinO'] = { '@type': 'Place', initial: [0], x: 500, y: 200 }

  // X move transitions: P -> PlayX -> X + Next
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const tid = `PlayX${r}${c}`
      transitions[tid] = { '@type': 'Transition', x: 120 + c * 60, y: 50 + r * 60 }
      arcs.push({ '@type': 'Arrow', source: `P${r}${c}`, target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: `X${r}${c}`, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: 'Next', weight: [1] })
    }
  }

  // O move transitions: Next + P -> PlayO -> O
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const tid = `PlayO${r}${c}`
      transitions[tid] = { '@type': 'Transition', x: 270 + c * 60, y: 50 + r * 60 }
      arcs.push({ '@type': 'Arrow', source: 'Next', target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: `P${r}${c}`, target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: `O${r}${c}`, weight: [1] })
    }
  }

  // Win pattern transitions
  const winPatterns = [
    [0, 1, 2], [3, 4, 5], [6, 7, 8], // rows
    [0, 3, 6], [1, 4, 7], [2, 5, 8], // cols
    [0, 4, 8], [2, 4, 6]             // diags
  ]
  const patternNames = ['Row0', 'Row1', 'Row2', 'Col0', 'Col1', 'Col2', 'Dg0', 'Dg1']

  // X win transitions (use read arcs: consume and produce back to preserve pieces)
  winPatterns.forEach((pattern, idx) => {
    const tid = `X${patternNames[idx]}`
    transitions[tid] = { '@type': 'Transition', x: 450, y: 50 + idx * 25 }
    pattern.forEach(cellIdx => {
      const r = Math.floor(cellIdx / 3)
      const c = cellIdx % 3
      // Consume from X place
      arcs.push({ '@type': 'Arrow', source: `X${r}${c}`, target: tid, weight: [1] })
      // Produce back to X place (read arc pattern)
      arcs.push({ '@type': 'Arrow', source: tid, target: `X${r}${c}`, weight: [1] })
    })
    arcs.push({ '@type': 'Arrow', source: tid, target: 'WinX', weight: [1] })
  })

  // O win transitions (use read arcs: consume and produce back to preserve pieces)
  winPatterns.forEach((pattern, idx) => {
    const tid = `O${patternNames[idx]}`
    transitions[tid] = { '@type': 'Transition', x: 450, y: 250 + idx * 25 }
    pattern.forEach(cellIdx => {
      const r = Math.floor(cellIdx / 3)
      const c = cellIdx % 3
      // Consume from O place
      arcs.push({ '@type': 'Arrow', source: `O${r}${c}`, target: tid, weight: [1] })
      // Produce back to O place (read arc pattern)
      arcs.push({ '@type': 'Arrow', source: tid, target: `O${r}${c}`, weight: [1] })
    })
    arcs.push({ '@type': 'Arrow', source: tid, target: 'WinO', weight: [1] })
  })

  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    places,
    transitions,
    arcs
  }
}

// Run ODE solver and extract WinX/WinO values
function solveODE(model) {
  try {
    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = Solver.setRates(net)

    // Use configurable solver parameters
    const prob = new Solver.ODEProblem(net, initialState, [0, solverParams.tspan], rates)
    const opts = { dt: solverParams.dt, adaptive: solverParams.adaptive }
    if (solverParams.adaptive) {
      opts.abstol = solverParams.abstol
      opts.reltol = solverParams.reltol
    }
    const solution = Solver.solve(prob, Solver.Tsit5(), opts)

    const finalState = solution.u ? solution.u[solution.u.length - 1] : null
    if (!finalState) return null

    // finalState is a dictionary with place names as keys
    return {
      winX: finalState['WinX'] || 0,
      winO: finalState['WinO'] || 0
    }
  } catch (err) {
    console.error('ODE solve error:', err)
    return null
  }
}

// API-based heatmap (Go backend) - for demo/testing
async function runAPIHeatmap(board = null) {
  try {
    const apiBoard = board ? board.map(row => [...row]) : [['','',''],['','',''],['','','']]

    const response = await fetch(`${getApiBase()}/api/heatmap`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ board: apiBoard })
    })

    if (!response.ok) {
      throw new Error(`Heatmap API error: ${response.status}`)
    }

    const data = await response.json()
    console.log('ODE heatmap from backend API:', data)

    return {
      values: data.values,
      details: data.details,
      player: data.current_player,
      solution: null,
      model: null
    }
  } catch (err) {
    console.error('Heatmap API failed, falling back to local:', err)
    return runLocalODESimulation(board)
  }
}

// Toggle between local and API ODE computation
function setODEMode(useLocal) {
  useLocalODE = useLocal
  console.log(`ODE mode: ${useLocal ? 'local (JS)' : 'API (Go backend)'}`)
}

// Export for console testing
window.setODEMode = setODEMode
window.runAPIHeatmap = runAPIHeatmap
window.runLocalODESimulation = runLocalODESimulation

// Get ODE-computed value for a position
function getODEValue(pos, board = null) {
  if (odeValues === null) {
    ensureODEValues(board)
  }
  return (odeValues && odeValues[pos] !== undefined) ? odeValues[pos] : 0
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
  revertedToIndex: null, // Track if we've reverted to an earlier state
  preferredBest: null, // Preferred cell for tie-breaking (e.g., "22" for position 2,2)
}

let showHeatmap = false
let valueChart = null

// API functions
async function createGame() {
  const response = await fetch(`${getApiBase()}/api/tictactoe`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
  return response.json()
}

async function getGameState(id) {
  const response = await fetch(`${getApiBase()}/api/tictactoe/${id}`)
  return response.json()
}

async function getGameEvents(id) {
  const response = await fetch(`${getApiBase()}/api/tictactoe/${id}/events`)
  return response.json()
}

async function executeTransition(transitionId, aggregateId) {
  const response = await fetch(`${getApiBase()}/api/${transitionId}`, {
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
    // If we've reverted to an earlier state, create a new game and replay moves
    if (gameState.revertedToIndex !== null) {
      const eventsToReplay = gameState.events.slice(0, gameState.revertedToIndex + 1)

      // Create new game
      const newGame = await createGame()
      gameState.id = newGame.aggregate_id

      // Replay events up to the revert point
      for (const event of eventsToReplay) {
        const type = event.type || ''
        if (type.startsWith('XPlayed') || type.startsWith('OPlayed')) {
          const player = type.charAt(0).toLowerCase()
          const eventRow = parseInt(type.charAt(7))
          const eventCol = parseInt(type.charAt(8))
          await executeTransition(`${player}_play_${eventRow}${eventCol}`, gameState.id)
        }
      }

      // Clear revert state and truncate local events
      gameState.events = eventsToReplay
      gameState.revertedToIndex = null
    }

    const result = await executeTransition(transitionId, gameState.id)

    // Update board from state (places)
    if (result.state) {
      updateBoardFromState(result.state)
    }

    gameState.enabled = result.enabled_transitions || []
    console.log('Enabled transitions after move:', gameState.enabled)

    // Update current player based on enabled transitions
    updateCurrentPlayer()

    // Check for win condition locally (backend may not track this)
    checkWinCondition()

    if (gameState.gameOver) {
      document.getElementById('reset-btn').classList.remove('hidden')
    }

    // Also check for game over from full state (if backend supports it)
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

    // Auto-start a new game if none exists
    if (!gameState.id) {
      await newGame() // newGame() handles ODE computation when showHeatmap is true
    } else {
      // Run ODE simulation for current game state
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

function getHeatColor(value, minVal = null, maxVal = null) {
  // Auto-scale based on all available ODE values (including negative)
  if (minVal === null || maxVal === null) {
    if (odeValues) {
      // Get all non-zero values (empty positions only)
      const vals = Object.values(odeValues).filter(v => v !== 0)
      if (vals.length > 0) {
        minVal = Math.min(...vals)
        maxVal = Math.max(...vals)
      }
    }
    // Fallback defaults
    if (minVal === null) minVal = 0
    if (maxVal === null) maxVal = 1
  }

  // For occupied cells (value=0), return gray
  if (value === 0 && odeValues) {
    return 'rgb(180, 180, 180)'
  }

  // Normalize: best move (highest) = red, worst move (lowest) = blue
  const range = maxVal - minVal
  const normalized = range > 0 ? Math.max(0, Math.min(1, (value - minVal) / range)) : 0.5

  // Interpolate between blue (low/worst) and red (high/best)
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

  // Ensure ODE values are computed for heatmap display
  if (showHeatmap && !gameState.gameOver && odeValues === null) {
    ensureODEValues(gameState.id ? gameState.board : null)
  }

  // Find recommended move (highest ODE value among empty cells)
  // If there's a tie and preferredBest is set, use that position
  let recommendedPos = null
  if (showHeatmap && odeValues && !gameState.gameOver) {
    let maxValue = -Infinity
    let tiedPositions = []

    for (let r = 0; r < 3; r++) {
      for (let c = 0; c < 3; c++) {
        if (gameState.board[r][c] === '') {
          const pos = `${r}${c}`
          const val = odeValues[pos] || 0
          if (val > maxValue) {
            maxValue = val
            tiedPositions = [pos]
          } else if (Math.abs(val - maxValue) < 0.001) {
            tiedPositions.push(pos)
          }
        }
      }
    }

    if (gameState.preferredBest && tiedPositions.length > 1 && tiedPositions.includes(gameState.preferredBest)) {
      recommendedPos = gameState.preferredBest
    } else {
      recommendedPos = tiedPositions[0] || null
    }
  }

  // Render board
  let boardHtml = ''
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const piece = gameState.board[row][col]
      const pos = `${row}${col}`
      const isWinning = winningPattern && winningPattern.includes(row * 3 + col)
      const isRecommended = recommendedPos === pos

      const classes = ['cell']
      if (piece) classes.push('occupied')
      if (gameState.gameOver && !piece) classes.push('disabled')
      if (isWinning) classes.push('winning')

      // Always use ODE computed values
      const odeValue = (odeValues && odeValues[pos] !== undefined) ? odeValues[pos] : 0
      const displayValue = piece ? 0 : odeValue
      const heatColor = getHeatColor(displayValue)
      const recommendedStyle = isRecommended ? 'border: 3px dashed #333; border-radius: 8px;' : ''

      boardHtml += `
        <button class="${classes.join(' ')}"
                onclick="makeMove(${row}, ${col})"
                style="${recommendedStyle}"
                ${piece || gameState.gameOver ? 'disabled' : ''}>
          ${piece ? `<span class="piece ${piece.toLowerCase()}">${piece}</span>` : ''}
          <div class="heat-overlay" style="background: ${heatColor};">
            <span class="heat-value">${displayValue.toFixed(3)}</span>
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

    // Event types are "XPlayed{row}{col}" or "OPlayed{row}{col}"
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

    // Event types are "XPlayed{row}{col}" or "OPlayed{row}{col}"
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
  gameState.revertedToIndex = moveIndex // Track that we've reverted

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

// Custom plugin to draw "marked" separator line
const markedLinePlugin = {
  id: 'markedLine',
  afterDraw: (chart) => {
    if (chart.markedLevel === null || chart.markedLevel === undefined) return

    const ctx = chart.ctx
    const yAxis = chart.scales.y
    const xAxis = chart.scales.x

    // Draw line slightly above marked level
    const lineY = yAxis.getPixelForValue(chart.markedLevel + 0.02)

    ctx.save()
    ctx.beginPath()
    ctx.setLineDash([5, 5])
    ctx.strokeStyle = 'rgba(150, 150, 150, 0.7)'
    ctx.lineWidth = 1
    ctx.moveTo(xAxis.left, lineY)
    ctx.lineTo(xAxis.right, lineY)
    ctx.stroke()

    // Draw "marked" label
    ctx.fillStyle = 'rgba(150, 150, 150, 0.9)'
    ctx.font = '11px sans-serif'
    ctx.textAlign = 'left'
    ctx.fillText('marked', xAxis.left + 5, lineY - 4)
    ctx.restore()
  }
}

function initValueChart() {
  const ctx = document.getElementById('value-chart').getContext('2d')

  const positions = ['(0,0)', '(0,1)', '(0,2)', '(1,0)', '(1,1)', '(1,2)', '(2,0)', '(2,1)', '(2,2)']
  const values = [0.316, 0.218, 0.316, 0.218, 0.430, 0.218, 0.316, 0.218, 0.316]

  // Position type colors: center=red, corner=orange, edge=blue
  const positionColors = [
    'rgba(243, 156, 18, 1)',  // (0,0) corner
    'rgba(52, 152, 219, 1)',   // (0,1) edge
    'rgba(243, 156, 18, 1)',  // (0,2) corner
    'rgba(52, 152, 219, 1)',   // (1,0) edge
    'rgba(231, 76, 60, 1)',    // (1,1) center
    'rgba(52, 152, 219, 1)',   // (1,2) edge
    'rgba(243, 156, 18, 1)',  // (2,0) corner
    'rgba(52, 152, 219, 1)',   // (2,1) edge
    'rgba(243, 156, 18, 1)',  // (2,2) corner
  ]

  valueChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: positions,
      datasets: [
        {
          label: 'ODE Strategic Value',
          data: values,
          borderColor: 'rgba(150, 150, 150, 0.3)',
          borderWidth: 1,
          pointBackgroundColor: positionColors,
          pointBorderColor: positionColors,
          pointRadius: 12,
          pointHoverRadius: 16,
          fill: false,
          tension: 0,
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
          ticks: {
            callback: function(value) {
              return value.toFixed(2)
            }
          }
        }
      },
      plugins: {
        legend: {
          display: true,
          position: 'bottom',
          labels: {
            generateLabels: function(chart) {
              return [
                { text: 'Center', fillStyle: 'rgba(231, 76, 60, 1)', strokeStyle: 'rgba(231, 76, 60, 1)', pointStyle: 'circle', lineWidth: 0 },
                { text: 'Corner', fillStyle: 'rgba(243, 156, 18, 1)', strokeStyle: 'rgba(243, 156, 18, 1)', pointStyle: 'circle', lineWidth: 0 },
                { text: 'Edge', fillStyle: 'rgba(52, 152, 219, 1)', strokeStyle: 'rgba(52, 152, 219, 1)', pointStyle: 'circle', lineWidth: 0 },
                { text: 'X Played', fillStyle: 'rgba(231, 76, 60, 1)', strokeStyle: 'rgba(231, 76, 60, 1)', pointStyle: 'rectRot', lineWidth: 0 },
                { text: 'O Played', fillStyle: 'rgba(52, 152, 219, 1)', strokeStyle: 'rgba(52, 152, 219, 1)', pointStyle: 'rectRot', lineWidth: 0 },
              ]
            },
            usePointStyle: true,
            pointStyle: 'circle',
            padding: 15,
            boxWidth: 12,
            boxHeight: 12,
          }
        },
        tooltip: {
          callbacks: {
            label: function(context) {
              const index = context.dataIndex
              const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']
              const pos = positions[index]
              const row = parseInt(pos[0])
              const col = parseInt(pos[1])
              const piece = gameState.board[row][col]

              if (piece !== '') {
                return `Played: ${piece}`
              }
              return `Value: ${context.parsed.y.toFixed(3)}`
            },
            afterBody: function(context) {
              const index = context[0].dataIndex
              const types = ['corner', 'edge', 'corner', 'edge', 'center', 'edge', 'corner', 'edge', 'corner']
              return `Position type: ${types[index]}`
            }
          }
        }
      }
    },
    plugins: [markedLinePlugin]
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
      odeValues = null // Clear stale values
      desc.textContent = 'No game in progress. Start a new game to see contextual values.'
    } else if (gameState.gameOver) {
      odeValues = null // Clear - no strategic values for completed game
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

function renderSimulationGrid() {
  const grid = document.getElementById('position-grid')
  if (!grid) return

  // Ensure ODE values are computed
  if (odeValues === null) {
    const board = simMode === 'current' ? gameState.board : null
    ensureODEValues(board)
  }

  const positionTypes = {
    '00': 'corner', '01': 'edge', '02': 'corner',
    '10': 'edge', '11': 'center', '12': 'edge',
    '20': 'corner', '21': 'edge', '22': 'corner'
  }

  let cells = []
  const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']

  if (simMode === 'empty') {
    cells = positions.map(pos => {
      const value = odeValues ? (odeValues[pos] || 0) : 0
      const type = positionTypes[pos]
      return `
        <div class="position-cell ${type}">
          <span class="value">${value.toFixed(3)}</span>
          <span class="type">ODE</span>
        </div>
      `
    })
  } else {
    cells = positions.map(pos => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      const type = positionTypes[pos]

      if (piece !== '') {
        const pieceColor = piece === 'X' ? '#e74c3c' : '#3498db'
        return `
          <div class="position-cell" style="background: ${pieceColor};">
            <span class="value" style="font-size: 2rem;">${piece}</span>
            <span class="type">played</span>
          </div>
        `
      } else {
        const value = odeValues ? (odeValues[pos] || 0) : 0
        const maxValue = odeValues ? Math.max(...Object.values(odeValues).map(Math.abs)) : 1
        const opacity = 0.4 + (Math.abs(value) / maxValue) * 0.6

        return `
          <div class="position-cell ${type}" style="opacity: ${opacity};">
            <span class="value">${value.toFixed(3)}</span>
            <span class="type">ODE</span>
          </div>
        `
      }
    })
  }

  grid.innerHTML = cells.join('')
}

function updateSimulationChart() {
  if (!valueChart) return

  const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']
  const positionTypes = {
    '00': 'corner', '01': 'edge', '02': 'corner',
    '10': 'edge', '11': 'center', '12': 'edge',
    '20': 'corner', '21': 'edge', '22': 'corner'
  }

  // Ensure ODE values are computed
  if (odeValues === null) {
    const board = simMode === 'current' ? gameState.board : null
    ensureODEValues(board)
  }

  // Helper to update y-axis scale based on values
  function updateYAxisScale(values, markedLevel = null) {
    const nonZeroValues = values.filter(v => v !== 0)
    if (nonZeroValues.length === 0) {
      valueChart.options.scales.y.min = markedLevel !== null ? markedLevel - 0.05 : -0.5
      valueChart.options.scales.y.max = 0.5
    } else {
      const minVal = Math.min(...nonZeroValues)
      const maxVal = Math.max(...nonZeroValues)
      // Add 10% padding and round to nice numbers
      const range = maxVal - minVal || 0.1
      const padding = range * 0.15
      let scaleMin = Math.floor((minVal - padding) * 10) / 10
      // Extend to include marked level if present
      if (markedLevel !== null) {
        scaleMin = Math.min(scaleMin, markedLevel - 0.02)
      }
      valueChart.options.scales.y.min = scaleMin
      valueChart.options.scales.y.max = Math.ceil((maxVal + padding) * 10) / 10
    }
  }

  // Position type colors: center=red, corner=orange, edge=blue
  function getPositionColor(pos, isOccupied = false) {
    if (isOccupied) return 'rgba(128, 128, 128, 0.5)'
    const type = positionTypes[pos]
    if (type === 'center') return 'rgba(231, 76, 60, 1)'
    if (type === 'corner') return 'rgba(243, 156, 18, 1)'
    return 'rgba(52, 152, 219, 1)'
  }

  if (simMode === 'empty') {
    const values = positions.map(pos => odeValues ? (odeValues[pos] || 0) : 0)
    const colors = positions.map(pos => getPositionColor(pos))

    valueChart.data.datasets[0].data = values
    valueChart.data.datasets[0].pointBackgroundColor = colors
    valueChart.data.datasets[0].pointBorderColor = colors
    valueChart.data.datasets[0].pointStyle = 'circle'  // Reset to circles
    valueChart.markedLevel = null  // No marked level for empty board
    updateYAxisScale(values)
    valueChart.update()
  } else {
    // First pass: collect open position values and track occupied positions
    const openValues = []
    const occupiedIndices = []
    positions.forEach((pos, i) => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      if (piece === '') {
        const val = odeValues ? (odeValues[pos] || 0) : 0
        openValues.push(val)
      } else {
        occupiedIndices.push(i)
      }
    })

    // Calculate scale bounds first
    let scaleMin = -0.5
    let markedLevel = -0.6  // Below the data, for played pieces
    if (openValues.length > 0) {
      const minVal = Math.min(...openValues)
      const maxVal = Math.max(...openValues)
      const range = maxVal - minVal || 0.1
      const padding = range * 0.15
      scaleMin = Math.floor((minVal - padding) * 10) / 10
      markedLevel = scaleMin - (range * 0.15)  // Place below scale min
    }

    // Build display values (occupied cells go at "marked" level)
    const values = positions.map(pos => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      if (piece !== '') return markedLevel  // Place at "marked" level
      return odeValues ? (odeValues[pos] || 0) : 0
    })

    // Store marked level for axis label
    valueChart.markedLevel = occupiedIndices.length > 0 ? markedLevel : null

    // Colors: X=red, O=blue, open positions by type
    const colors = positions.map((pos, i) => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      if (piece === 'X') return 'rgba(231, 76, 60, 1)'  // X is red
      if (piece === 'O') return 'rgba(52, 152, 219, 1)' // O is blue
      return getPositionColor(pos, false)
    })

    // Custom point styles: diamond for played pieces
    const pointStyles = positions.map(pos => {
      const row = parseInt(pos[0])
      const col = parseInt(pos[1])
      const piece = gameState.board[row][col]
      return piece !== '' ? 'rectRot' : 'circle'
    })

    valueChart.data.datasets[0].data = values
    valueChart.data.datasets[0].pointBackgroundColor = colors
    valueChart.data.datasets[0].pointBorderColor = colors
    valueChart.data.datasets[0].pointStyle = pointStyles
    updateYAxisScale(openValues, occupiedIndices.length > 0 ? markedLevel : null)
    valueChart.update()
  }
}

// Generate SVG snapshot of current game state
function downloadSnapshot() {
  const cellSize = 60
  const boardSize = cellSize * 3
  const padding = 12
  const historyWidth = gameState.events && gameState.events.length > 0 ? 100 : 0
  const svgWidth = boardSize + padding * 2 + (historyWidth > 0 ? historyWidth + 16 : 0)
  const svgHeight = boardSize + padding * 2
  const boardX = padding
  const boardY = padding

  // Helper to get heat color
  function getHeatColorForSVG(value) {
    if (!odeValues || value === 0) return '#e0e0e0'
    const vals = Object.values(odeValues).filter(v => v !== 0)
    if (vals.length === 0) return '#e0e0e0'
    const minVal = Math.min(...vals)
    const maxVal = Math.max(...vals)
    const range = maxVal - minVal
    const normalized = range > 0 ? Math.max(0, Math.min(1, (value - minVal) / range)) : 0.5
    const r = Math.round(255 * normalized)
    const g = Math.round(100 - 50 * normalized)
    const b = Math.round(255 * (1 - normalized))
    return `rgb(${r}, ${g}, ${b})`
  }

  // Build grid lines
  let gridLines = ''
  const lineColor = '#333'
  const lineWidth = 3
  // Vertical lines
  gridLines += `<line x1="${boardX + cellSize}" y1="${boardY}" x2="${boardX + cellSize}" y2="${boardY + boardSize}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`
  gridLines += `<line x1="${boardX + cellSize * 2}" y1="${boardY}" x2="${boardX + cellSize * 2}" y2="${boardY + boardSize}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`
  // Horizontal lines
  gridLines += `<line x1="${boardX}" y1="${boardY + cellSize}" x2="${boardX + boardSize}" y2="${boardY + cellSize}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`
  gridLines += `<line x1="${boardX}" y1="${boardY + cellSize * 2}" x2="${boardX + boardSize}" y2="${boardY + cellSize * 2}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`

  // Find recommended move (highest ODE value among empty cells)
  // If there's a tie and preferredBest is set, use that position
  let recommendedPos = null
  if (showHeatmap && odeValues && !gameState.gameOver) {
    let maxValue = -Infinity
    let tiedPositions = []

    // First pass: find max value and all positions with that value
    for (let r = 0; r < 3; r++) {
      for (let c = 0; c < 3; c++) {
        if (gameState.board[r][c] === '') {
          const pos = `${r}${c}`
          const val = odeValues[pos] || 0
          if (val > maxValue) {
            maxValue = val
            tiedPositions = [{ row: r, col: c, pos }]
          } else if (Math.abs(val - maxValue) < 0.001) { // Within tolerance = tie
            tiedPositions.push({ row: r, col: c, pos })
          }
        }
      }
    }

    // If there's a preferred position and it's among the tied best, use it
    if (gameState.preferredBest && tiedPositions.length > 1) {
      const preferred = tiedPositions.find(p => p.pos === gameState.preferredBest)
      if (preferred) {
        recommendedPos = { row: preferred.row, col: preferred.col }
      } else {
        recommendedPos = tiedPositions[0] ? { row: tiedPositions[0].row, col: tiedPositions[0].col } : null
      }
    } else {
      recommendedPos = tiedPositions[0] ? { row: tiedPositions[0].row, col: tiedPositions[0].col } : null
    }
  }

  // Build board pieces and heat values
  let boardContent = ''
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const piece = gameState.board[row][col]
      const pos = `${row}${col}`
      const centerX = boardX + col * cellSize + cellSize / 2
      const centerY = boardY + row * cellSize + cellSize / 2
      const isWinningCell = gameState.winner && gameState.winner !== 'draw' &&
        WIN_PATTERNS.some(pattern => {
          const cells = pattern.map(i => gameState.board[Math.floor(i/3)][i%3])
          return cells[0] && cells[0] === cells[1] && cells[1] === cells[2] && pattern.includes(row * 3 + col)
        })

      // Winning cell highlight
      if (isWinningCell) {
        boardContent += `<circle cx="${centerX}" cy="${centerY}" r="24" fill="#90EE90" opacity="0.6"/>`
      }

      if (piece) {
        const color = piece === 'X' ? '#e74c3c' : '#3498db'
        boardContent += `<text x="${centerX}" y="${centerY + 10}" font-family="Arial" font-size="36" font-weight="bold" fill="${color}" text-anchor="middle">${piece}</text>`
      } else if (showHeatmap && odeValues && odeValues[pos] !== undefined) {
        const heatColor = getHeatColorForSVG(odeValues[pos] || 0)
        boardContent += `<circle cx="${centerX}" cy="${centerY}" r="20" fill="${heatColor}" opacity="0.8"/>`
        boardContent += `<text x="${centerX}" y="${centerY + 4}" font-family="Arial" font-size="11" font-weight="bold" fill="white" text-anchor="middle">${odeValues[pos].toFixed(2)}</text>`

        // Dotted box for recommended move
        if (recommendedPos && recommendedPos.row === row && recommendedPos.col === col) {
          const boxX = boardX + col * cellSize + 4
          const boxY = boardY + row * cellSize + 4
          const boxSize = cellSize - 8
          boardContent += `<rect x="${boxX}" y="${boxY}" width="${boxSize}" height="${boxSize}" rx="6" fill="none" stroke="#2d2d2d" stroke-width="2" stroke-dasharray="4,3"/>`
        }
      }
    }
  }

  // Build move history (compact)
  let historyItems = ''
  if (gameState.events && gameState.events.length > 0) {
    const historyX = boardX + boardSize + 16
    gameState.events.forEach((event, index) => {
      const type = event.type || ''
      let player = ''
      let pos = ''

      if (type.startsWith('XPlayed') || type.startsWith('OPlayed')) {
        player = type.charAt(0)
        pos = `${type.charAt(7)},${type.charAt(8)}`
      }

      const itemY = boardY + index * 20
      const color = player === 'X' ? '#e74c3c' : '#3498db'

      historyItems += `
        <circle cx="${historyX + 8}" cy="${itemY + 8}" r="8" fill="${color}"/>
        <text x="${historyX + 8}" y="${itemY + 12}" font-family="Arial" font-size="10" font-weight="bold" fill="white" text-anchor="middle">${index + 1}</text>
        <text x="${historyX + 22}" y="${itemY + 12}" font-family="Arial" font-size="11" fill="#333">${player} ${pos}</text>
      `
    })
  }

  const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${svgWidth}" height="${svgHeight}" viewBox="0 0 ${svgWidth} ${svgHeight}">
  <rect width="100%" height="100%" rx="12" fill="#f8f9fa"/>
  ${gridLines}
  ${boardContent}
  ${historyItems}
</svg>`

  // Download
  const blob = new Blob([svg], { type: 'image/svg+xml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `tictactoe-snapshot-${Date.now()}.svg`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// Export functions for onclick handlers
window.makeMove = makeMove
window.newGame = newGame
window.resetGame = resetGame
window.toggleHeatmap = toggleHeatmap
window.setSimMode = setSimMode
window.revertToMove = revertToMove
window.downloadSnapshot = downloadSnapshot

// Parse URL parameters to recreate board state
// Format: ?moves=X11,O00,X22&heatmap=true&snapshot=true
// Moves are comma-separated: player + row + col (e.g., X11 = X plays at row 1, col 1)
// Use ?moves=&heatmap=true for empty board with heatmap
async function loadFromURL() {
  const params = new URLSearchParams(window.location.search)
  const movesParam = params.get('moves')
  const heatmapParam = params.get('heatmap')
  const snapshotParam = params.get('snapshot')
  const delayParam = params.get('delay') // ms to wait before snapshot
  const bestParam = params.get('best') // preferred cell for tie-breaking (e.g., "22")

  // Return false only if no URL params at all (not even heatmap)
  if (movesParam === null && heatmapParam === null) return false

  // Parse moves (comma-separated, e.g., "X11,O00,X22")
  const moveStrs = movesParam ? movesParam.toUpperCase().split(',').filter(m => m.length > 0) : []

  // Validate and apply moves
  const newBoard = [['', '', ''], ['', '', ''], ['', '', '']]
  gameState.events = []
  let expectedPlayer = 'X' // X always goes first

  for (const moveStr of moveStrs) {
    // Parse move: player + row + col (e.g., "X11")
    const match = moveStr.match(/^([XO])(\d)(\d)$/)
    if (!match) {
      console.error(`Invalid move format: ${moveStr}. Expected XRC or ORC (e.g., X11)`)
      continue
    }

    const player = match[1]
    const row = parseInt(match[2])
    const col = parseInt(match[3])

    // Validate position
    if (row < 0 || row > 2 || col < 0 || col > 2) {
      console.error(`Invalid position: ${row},${col}`)
      continue
    }

    // Validate turn order
    if (player !== expectedPlayer) {
      console.error(`Invalid turn order: expected ${expectedPlayer}, got ${player}`)
      continue
    }

    // Validate position is empty
    if (newBoard[row][col] !== '') {
      console.error(`Position ${row},${col} already occupied`)
      continue
    }

    // Apply move
    newBoard[row][col] = player
    gameState.events.push({ type: `${player}Played${row}${col}` })

    // Alternate player
    expectedPlayer = player === 'X' ? 'O' : 'X'
  }

  // Update game state (local only, no backend)
  gameState.board = newBoard
  gameState.id = 'url-loaded' // Mark as URL-loaded
  gameState.currentPlayer = expectedPlayer
  gameState.gameOver = false
  gameState.winner = null
  gameState.preferredBest = bestParam ? bestParam.replace(/[^0-2]/g, '').slice(0, 2) : null

  // Check for win
  checkWinCondition()

  // Enable heatmap if requested
  if (heatmapParam === 'true' || heatmapParam === '1') {
    showHeatmap = true
    const btn = document.getElementById('heatmap-btn')
    const board = document.getElementById('game-board')
    if (btn) {
      btn.classList.add('active')
      btn.textContent = 'Hide Heat Map'
    }
    if (board) board.classList.add('show-heatmap')

    // Compute ODE values
    const result = await runODESimulation(gameState.board)
    if (result) {
      odeValues = result.values
    }
  }

  // Render
  renderGame()
  renderEvents()

  // Auto-download snapshot if requested
  if (snapshotParam === 'true' || snapshotParam === '1') {
    const delay = parseInt(delayParam) || 500
    setTimeout(() => {
      downloadSnapshot()
    }, delay)
  }

  return true
}

// Generate URL for current game state with moves in order
function getBoardURL() {
  // Build moves string from events (e.g., "X11,O00,X22")
  const moves = gameState.events
    .filter(e => e.type && (e.type.startsWith('XPlayed') || e.type.startsWith('OPlayed')))
    .map(e => {
      const player = e.type.charAt(0)
      const row = e.type.charAt(7)
      const col = e.type.charAt(8)
      return `${player}${row}${col}`
    })
    .join(',')

  const params = new URLSearchParams()
  if (moves) params.set('moves', moves)
  if (showHeatmap) params.set('heatmap', 'true')
  return `${window.location.pathname}?${params.toString()}`
}

// Copy current board URL to clipboard
function copyBoardURL() {
  const url = window.location.origin + getBoardURL()
  navigator.clipboard.writeText(url).then(() => {
    console.log('Board URL copied:', url)
    alert('Board URL copied to clipboard!')
  }).catch(err => {
    console.error('Failed to copy URL:', err)
    prompt('Copy this URL:', url)
  })
}

window.copyBoardURL = copyBoardURL
window.getBoardURL = getBoardURL
window.generateSVGSnapshot = generateSVGSnapshot

// Generate SVG snapshot and return as string (for programmatic use)
function generateSVGSnapshot() {
  const cellSize = 60
  const boardSize = cellSize * 3
  const padding = 12
  const historyWidth = gameState.events && gameState.events.length > 0 ? 100 : 0
  const svgWidth = boardSize + padding * 2 + (historyWidth > 0 ? historyWidth + 16 : 0)
  const svgHeight = boardSize + padding * 2
  const boardX = padding
  const boardY = padding

  // Helper to get heat color
  function getHeatColorForSVG(value) {
    if (!odeValues || value === 0) return '#e0e0e0'
    const vals = Object.values(odeValues).filter(v => v !== 0)
    if (vals.length === 0) return '#e0e0e0'
    const minVal = Math.min(...vals)
    const maxVal = Math.max(...vals)
    const range = maxVal - minVal
    const normalized = range > 0 ? Math.max(0, Math.min(1, (value - minVal) / range)) : 0.5
    const r = Math.round(255 * normalized)
    const g = Math.round(100 - 50 * normalized)
    const b = Math.round(255 * (1 - normalized))
    return `rgb(${r}, ${g}, ${b})`
  }

  // Build grid lines
  let gridLines = ''
  const lineColor = '#333'
  const lineWidth = 3
  gridLines += `<line x1="${boardX + cellSize}" y1="${boardY}" x2="${boardX + cellSize}" y2="${boardY + boardSize}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`
  gridLines += `<line x1="${boardX + cellSize * 2}" y1="${boardY}" x2="${boardX + cellSize * 2}" y2="${boardY + boardSize}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`
  gridLines += `<line x1="${boardX}" y1="${boardY + cellSize}" x2="${boardX + boardSize}" y2="${boardY + cellSize}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`
  gridLines += `<line x1="${boardX}" y1="${boardY + cellSize * 2}" x2="${boardX + boardSize}" y2="${boardY + cellSize * 2}" stroke="${lineColor}" stroke-width="${lineWidth}" stroke-linecap="round"/>`

  // Find recommended move
  let recommendedPos = null
  if (showHeatmap && odeValues && !gameState.gameOver) {
    let maxValue = -Infinity
    for (let r = 0; r < 3; r++) {
      for (let c = 0; c < 3; c++) {
        if (gameState.board[r][c] === '') {
          const pos = `${r}${c}`
          const val = odeValues[pos] || 0
          if (val > maxValue) {
            maxValue = val
            recommendedPos = { row: r, col: c }
          }
        }
      }
    }
  }

  // Build board content
  let boardContent = ''
  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const piece = gameState.board[row][col]
      const pos = `${row}${col}`
      const centerX = boardX + col * cellSize + cellSize / 2
      const centerY = boardY + row * cellSize + cellSize / 2
      const isWinningCell = gameState.winner && gameState.winner !== 'draw' &&
        WIN_PATTERNS.some(pattern => {
          const cells = pattern.map(i => gameState.board[Math.floor(i/3)][i%3])
          return cells[0] && cells[0] === cells[1] && cells[1] === cells[2] && pattern.includes(row * 3 + col)
        })

      if (isWinningCell) {
        boardContent += `<circle cx="${centerX}" cy="${centerY}" r="24" fill="#90EE90" opacity="0.6"/>`
      }

      if (piece) {
        const color = piece === 'X' ? '#e74c3c' : '#3498db'
        boardContent += `<text x="${centerX}" y="${centerY + 10}" font-family="Arial" font-size="36" font-weight="bold" fill="${color}" text-anchor="middle">${piece}</text>`
      } else if (showHeatmap && odeValues && odeValues[pos] !== undefined) {
        const heatColor = getHeatColorForSVG(odeValues[pos] || 0)
        boardContent += `<circle cx="${centerX}" cy="${centerY}" r="20" fill="${heatColor}" opacity="0.8"/>`
        boardContent += `<text x="${centerX}" y="${centerY + 4}" font-family="Arial" font-size="11" font-weight="bold" fill="white" text-anchor="middle">${odeValues[pos].toFixed(2)}</text>`

        if (recommendedPos && recommendedPos.row === row && recommendedPos.col === col) {
          const boxX = boardX + col * cellSize + 4
          const boxY = boardY + row * cellSize + 4
          const boxSize = cellSize - 8
          boardContent += `<rect x="${boxX}" y="${boxY}" width="${boxSize}" height="${boxSize}" rx="6" fill="none" stroke="#2d2d2d" stroke-width="2" stroke-dasharray="4,3"/>`
        }
      }
    }
  }

  // Build move history (compact)
  let historyItems = ''
  if (gameState.events && gameState.events.length > 0) {
    const historyX = boardX + boardSize + 16
    gameState.events.forEach((event, index) => {
      const type = event.type || ''
      let player = ''
      let pos = ''

      if (type.startsWith('XPlayed') || type.startsWith('OPlayed')) {
        player = type.charAt(0)
        pos = `${type.charAt(7)},${type.charAt(8)}`
      }

      const itemY = boardY + index * 20
      const color = player === 'X' ? '#e74c3c' : '#3498db'

      historyItems += `
        <circle cx="${historyX + 8}" cy="${itemY + 8}" r="8" fill="${color}"/>
        <text x="${historyX + 8}" y="${itemY + 12}" font-family="Arial" font-size="10" font-weight="bold" fill="white" text-anchor="middle">${index + 1}</text>
        <text x="${historyX + 22}" y="${itemY + 12}" font-family="Arial" font-size="11" fill="#333">${player} ${pos}</text>
      `
    })
  }

  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${svgWidth}" height="${svgHeight}" viewBox="0 0 ${svgWidth} ${svgHeight}">
  <rect width="100%" height="100%" rx="12" fill="#f8f9fa"/>
  ${gridLines}
  ${boardContent}
  ${historyItems}
</svg>`
}

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  // Check for URL parameters first
  const loadedFromURL = await loadFromURL()

  if (!loadedFromURL) {
    renderGame()
    renderEvents()
  }
  renderSimulationGrid()
})
