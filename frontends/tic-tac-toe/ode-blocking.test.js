/**
 * ODE Blocking Bug Test
 *
 * This test documents a bug where the ODE-based strategic value computation
 * does not properly prioritize blocking an opponent's immediate winning threat.
 *
 * Board state being tested:
 *   Row 0: -  -  -    (empty)
 *   Row 1: O  X  -    (O at 1,0 - X at 1,1)
 *   Row 2: -  X  -    (X at 2,1)
 *
 * X has pieces at (1,1) and (2,1) - threatening to win via center column.
 * If X plays (0,1), X wins immediately.
 *
 * It's O's turn. O MUST block at (0,1) to prevent immediate loss.
 *
 * BUG: The ODE simulation currently gives position (2,0) a higher score
 * than position (0,1), failing to recognize the blocking urgency.
 *
 * ROOT CAUSE: The Petri net win detection transitions use "read arcs"
 * (consume and produce back) but don't consume the 'Next' turn token.
 * This means the game doesn't "end" when a win is detected - play continues
 * in the ODE simulation, diluting the importance of blocking.
 *
 * FIX: Win detection transitions should consume the 'Next' token to
 * terminate game flow, making blocking moves much more valuable.
 *
 * Run with: node --experimental-vm-modules ode-blocking.test.js
 * Or load in browser console on the tic-tac-toe page.
 */

// Board state: O at (1,0), X at (1,1) and (2,1)
// It's O's turn - O must block at (0,1)
const BLOCKING_TEST_BOARD = [
  ['', '', ''],      // Row 0: empty
  ['O', 'X', ''],    // Row 1: O, X, empty
  ['', 'X', ''],     // Row 2: empty, X, empty
]

// Expected: Position (0,1) should have the HIGHEST score for O
// because it blocks X's immediate win via center column.
//
// Current bug: Position (2,0) or other corners get higher scores.

/**
 * Build the Petri net model for ODE simulation with a hypothetical move.
 * This is extracted from main.js buildODEPetriNet function.
 */
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

  // Win pattern transitions - CURRENT IMPLEMENTATION (buggy)
  // Win transitions use read arcs but do NOT consume the Next token
  // This means the game continues after a win is "detected"
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

    // BUG: Missing arc to consume 'Next' token - game doesn't end!
    // FIX would be: arcs.push({ '@type': 'Arrow', source: 'Next', target: tid, weight: [1] })
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

    // BUG: Missing arc to consume 'Next' token - game doesn't end!
  })

  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    places,
    transitions,
    arcs
  }
}

/**
 * Build Petri net with the FIX applied:
 * Win transitions consume the 'Next' token, ending the game.
 */
function buildODEPetriNetFixed(board, currentPlayer, hypRow, hypCol) {
  const places = {}
  const transitions = {}
  const arcs = []

  // Board position places P00-P22
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const id = `P${r}${c}`
      let initial = 1 // empty
      if (board && board[r][c] !== '') initial = 0
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
      if (currentPlayer === 'O' && r === hypRow && c === hypCol) initial = 1
      places[id] = { '@type': 'Place', initial: [initial], x: 350 + c * 60, y: 50 + r * 60 }
    }
  }

  // Turn control
  const nextTurn = currentPlayer === 'X' ? 1 : 0
  places['Next'] = { '@type': 'Place', initial: [nextTurn], x: 250, y: 250 }

  // Win detection places
  places['WinX'] = { '@type': 'Place', initial: [0], x: 500, y: 100 }
  places['WinO'] = { '@type': 'Place', initial: [0], x: 500, y: 200 }

  // X move transitions
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const tid = `PlayX${r}${c}`
      transitions[tid] = { '@type': 'Transition', x: 120 + c * 60, y: 50 + r * 60 }
      arcs.push({ '@type': 'Arrow', source: `P${r}${c}`, target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: `X${r}${c}`, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: 'Next', weight: [1] })
    }
  }

  // O move transitions
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const tid = `PlayO${r}${c}`
      transitions[tid] = { '@type': 'Transition', x: 270 + c * 60, y: 50 + r * 60 }
      arcs.push({ '@type': 'Arrow', source: 'Next', target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: `P${r}${c}`, target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: `O${r}${c}`, weight: [1] })
    }
  }

  // Win patterns
  const winPatterns = [
    [0, 1, 2], [3, 4, 5], [6, 7, 8], // rows
    [0, 3, 6], [1, 4, 7], [2, 5, 8], // cols
    [0, 4, 8], [2, 4, 6]             // diags
  ]
  const patternNames = ['Row0', 'Row1', 'Row2', 'Col0', 'Col1', 'Col2', 'Dg0', 'Dg1']

  // X win transitions - FIXED: consume Next token to end game
  winPatterns.forEach((pattern, idx) => {
    const tid = `X${patternNames[idx]}`
    transitions[tid] = { '@type': 'Transition', x: 450, y: 50 + idx * 25 }
    pattern.forEach(cellIdx => {
      const r = Math.floor(cellIdx / 3)
      const c = cellIdx % 3
      arcs.push({ '@type': 'Arrow', source: `X${r}${c}`, target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: `X${r}${c}`, weight: [1] })
    })
    arcs.push({ '@type': 'Arrow', source: tid, target: 'WinX', weight: [1] })
    // FIX: Consume Next token - this ends the game!
    arcs.push({ '@type': 'Arrow', source: 'Next', target: tid, weight: [1] })
  })

  // O win transitions - FIXED: consume Next token to end game
  winPatterns.forEach((pattern, idx) => {
    const tid = `O${patternNames[idx]}`
    transitions[tid] = { '@type': 'Transition', x: 450, y: 250 + idx * 25 }
    pattern.forEach(cellIdx => {
      const r = Math.floor(cellIdx / 3)
      const c = cellIdx % 3
      arcs.push({ '@type': 'Arrow', source: `O${r}${c}`, target: tid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: tid, target: `O${r}${c}`, weight: [1] })
    })
    arcs.push({ '@type': 'Arrow', source: tid, target: 'WinO', weight: [1] })
    // FIX: Consume Next token - this ends the game!
    arcs.push({ '@type': 'Arrow', source: 'Next', target: tid, weight: [1] })
  })

  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    places,
    transitions,
    arcs
  }
}

/**
 * Test runner - prints model JSON for verification.
 * To fully test, load in browser and use the Solver.
 */
function runTest() {
  console.log('='.repeat(70))
  console.log('ODE BLOCKING BUG TEST')
  console.log('='.repeat(70))
  console.log('')
  console.log('Board state:')
  console.log('  Row 0: -  -  -    (all empty)')
  console.log('  Row 1: O  X  -    (O at 1,0 / X at 1,1)')
  console.log('  Row 2: -  X  -    (X at 2,1)')
  console.log('')
  console.log("X threatens center column win. O's turn - MUST block at (0,1).")
  console.log('')

  // Build models for key positions
  const currentPlayer = 'O'

  const blockingPos = { row: 0, col: 1 }  // Blocking move
  const cornerPos = { row: 2, col: 0 }    // Corner that bug prefers

  console.log('Building models for comparison...')
  console.log('')

  // Current (buggy) model
  const buggyBlockModel = buildODEPetriNet(BLOCKING_TEST_BOARD, currentPlayer, blockingPos.row, blockingPos.col)
  const buggyCornerModel = buildODEPetriNet(BLOCKING_TEST_BOARD, currentPlayer, cornerPos.row, cornerPos.col)

  // Fixed model
  const fixedBlockModel = buildODEPetriNetFixed(BLOCKING_TEST_BOARD, currentPlayer, blockingPos.row, blockingPos.col)
  const fixedCornerModel = buildODEPetriNetFixed(BLOCKING_TEST_BOARD, currentPlayer, cornerPos.row, cornerPos.col)

  console.log('CURRENT (BUGGY) BEHAVIOR:')
  console.log('-'.repeat(40))
  console.log(`Position (0,1) blocking move - arcs: ${buggyBlockModel.arcs.length}`)
  console.log(`Position (2,0) corner move - arcs: ${buggyCornerModel.arcs.length}`)
  console.log('')
  console.log('Bug: Win transitions do NOT consume Next token.')
  console.log('     Game continues after win, diluting blocking importance.')
  console.log('')

  console.log('FIXED BEHAVIOR:')
  console.log('-'.repeat(40))
  console.log(`Position (0,1) blocking move - arcs: ${fixedBlockModel.arcs.length}`)
  console.log(`Position (2,0) corner move - arcs: ${fixedCornerModel.arcs.length}`)
  console.log('')
  console.log('Fix: Win transitions consume Next token.')
  console.log('     Game ends on win, making blocking critical.')
  console.log('')

  // Count Next->Win arcs in each model
  const buggyNextArcs = buggyBlockModel.arcs.filter(a =>
    a.source === 'Next' && (a.target.startsWith('X') || a.target.startsWith('O')) &&
    !a.target.startsWith('PlayX') && !a.target.startsWith('PlayO')
  )
  const fixedNextArcs = fixedBlockModel.arcs.filter(a =>
    a.source === 'Next' && (a.target.startsWith('X') || a.target.startsWith('O')) &&
    !a.target.startsWith('PlayX') && !a.target.startsWith('PlayO')
  )

  console.log('Verification:')
  console.log(`  Buggy model: ${buggyNextArcs.length} arcs from Next to Win transitions`)
  console.log(`  Fixed model: ${fixedNextArcs.length} arcs from Next to Win transitions`)
  console.log('')

  if (buggyNextArcs.length === 0 && fixedNextArcs.length === 16) {
    console.log('✓ Models built correctly.')
    console.log('')
    console.log('To verify ODE values, run in browser:')
    console.log('  1. Open tic-tac-toe page')
    console.log('  2. Enter URL: ?moves=X11,O10,X21&heatmap=true')
    console.log('  3. Check console for ODE values')
    console.log('  4. BUG: Position 20 should NOT be higher than 01')
  } else {
    console.log('✗ Model construction error')
  }

  console.log('')
  console.log('='.repeat(70))

  return {
    buggyBlockModel,
    buggyCornerModel,
    fixedBlockModel,
    fixedCornerModel,
    testBoard: BLOCKING_TEST_BOARD
  }
}

// Export for use in browser or Node
if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    BLOCKING_TEST_BOARD,
    buildODEPetriNet,
    buildODEPetriNetFixed,
    runTest
  }
}

// Run if executed directly
if (typeof window === 'undefined') {
  runTest()
}
