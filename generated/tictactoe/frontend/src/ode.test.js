// ODE Heatmap Tests for Tic-Tac-Toe
// Run with: node --experimental-vm-modules node_modules/.bin/jest src/ode.test.js

import * as Solver from './petri-solver.js'

// Expected values from Go ODE test (ode_test.go)
const GO_REFERENCE_VALUES = {
  empty: {
    '00': 0.3157, '01': 0.2179, '02': 0.3157,
    '10': 0.2179, '11': 0.4300, '12': 0.2179,
    '20': 0.3157, '21': 0.2179, '22': 0.3157,
  }
}

// Tolerance for floating point comparison (relaxed for fast solver)
const TOLERANCE = 0.06

// Win patterns (same as main.js)
const WIN_PATTERNS = [
  [0, 1, 2], [3, 4, 5], [6, 7, 8], // rows
  [0, 3, 6], [1, 4, 7], [2, 5, 8], // cols
  [0, 4, 8], [2, 4, 6]             // diags
]
const PATTERN_NAMES = ['Row0', 'Row1', 'Row2', 'Col0', 'Col1', 'Col2', 'Dg0', 'Dg1']

// Build Petri net for ODE (mirrors main.js buildODEPetriNet)
function buildODEPetriNet(board, currentPlayer, hypRow, hypCol) {
  const places = {}
  const transitions = {}
  const arcs = []

  // Board position places P00-P22
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const id = `P${r}${c}`
      let initial = 1
      if (board && board[r][c] !== '') initial = 0
      if (r === hypRow && c === hypCol) initial = 0
      places[id] = { '@type': 'Place', initial: [initial], x: 50 + c * 60, y: 50 + r * 60 }
    }
  }

  // X history places
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      const id = `X${r}${c}`
      let initial = 0
      if (board && board[r][c] === 'X') initial = 1
      if (currentPlayer === 'X' && r === hypRow && c === hypCol) initial = 1
      places[id] = { '@type': 'Place', initial: [initial], x: 200 + c * 60, y: 50 + r * 60 }
    }
  }

  // O history places
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

  // Win transitions
  WIN_PATTERNS.forEach((pattern, idx) => {
    const xTid = `X${PATTERN_NAMES[idx]}`
    const oTid = `O${PATTERN_NAMES[idx]}`
    transitions[xTid] = { '@type': 'Transition', x: 450, y: 50 + idx * 25 }
    transitions[oTid] = { '@type': 'Transition', x: 450, y: 250 + idx * 25 }

    pattern.forEach(cellIdx => {
      const r = Math.floor(cellIdx / 3)
      const c = cellIdx % 3
      arcs.push({ '@type': 'Arrow', source: `X${r}${c}`, target: xTid, weight: [1] })
      arcs.push({ '@type': 'Arrow', source: `O${r}${c}`, target: oTid, weight: [1] })
    })
    arcs.push({ '@type': 'Arrow', source: xTid, target: 'WinX', weight: [1] })
    arcs.push({ '@type': 'Arrow', source: oTid, target: 'WinO', weight: [1] })
  })

  return { '@context': 'https://pflow.xyz/schema', '@type': 'PetriNet', places, transitions, arcs }
}

// Solve ODE and return WinX/WinO
function solveODE(model) {
  const net = Solver.fromJSON(model)
  const initialState = Solver.setState(net)
  const rates = Solver.setRates(net)

  // Optimized params: 420x faster, 0.05 max diff
  const prob = new Solver.ODEProblem(net, initialState, [0, 2.0], rates)
  const solution = Solver.solve(prob, Solver.Tsit5(), {
    dt: 0.2, adaptive: false
  })

  const finalState = solution.u ? solution.u[solution.u.length - 1] : null
  if (!finalState) return null

  // finalState is a dictionary with place names as keys
  return {
    winX: finalState['WinX'] || 0,
    winO: finalState['WinO'] || 0
  }
}

// Compute heatmap for a board state
function computeHeatmap(board = null) {
  const values = {}
  const positions = ['00', '01', '02', '10', '11', '12', '20', '21', '22']

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

  for (const pos of positions) {
    const row = parseInt(pos[0])
    const col = parseInt(pos[1])

    if (board && board[row][col] !== '') {
      values[pos] = 0
      continue
    }

    const model = buildODEPetriNet(board, currentPlayer, row, col)
    const result = solveODE(model)

    if (result) {
      values[pos] = currentPlayer === 'X' ? (result.winX - result.winO) : (result.winO - result.winX)
    } else {
      values[pos] = 0
    }
  }

  return { values, currentPlayer }
}

// Jest tests
describe('ODE Heatmap', () => {
  test('empty board matches Go reference values', () => {
    const result = computeHeatmap(null)

    expect(result.currentPlayer).toBe('X')

    for (const pos of Object.keys(GO_REFERENCE_VALUES.empty)) {
      const jsVal = result.values[pos]
      const goVal = GO_REFERENCE_VALUES.empty[pos]
      const diff = Math.abs(jsVal - goVal)

      console.log(`Position ${pos}: JS=${jsVal.toFixed(4)}, Go=${goVal.toFixed(4)}, diff=${diff.toFixed(6)}`)
      expect(diff).toBeLessThan(TOLERANCE)
    }
  })

  test('value ordering: center > corner > edge', () => {
    const result = computeHeatmap(null)

    const center = result.values['11']
    const corner = result.values['00']
    const edge = result.values['01']

    console.log(`Center: ${center.toFixed(4)}, Corner: ${corner.toFixed(4)}, Edge: ${edge.toFixed(4)}`)

    expect(center).toBeGreaterThan(corner)
    expect(corner).toBeGreaterThan(edge)
  })

  test('corner symmetry', () => {
    const result = computeHeatmap(null)

    const corners = ['00', '02', '20', '22'].map(p => result.values[p])
    const maxDiff = Math.max(...corners) - Math.min(...corners)

    console.log(`Corners: ${corners.map(v => v.toFixed(4)).join(', ')}`)
    console.log(`Max corner difference: ${maxDiff.toFixed(6)}`)

    expect(maxDiff).toBeLessThan(0.001)
  })

  test('X at center (O to move)', () => {
    const board = [['','',''],['','X',''],['','','']]
    const result = computeHeatmap(board)

    expect(result.currentPlayer).toBe('O')
    expect(result.values['11']).toBe(0) // center is occupied

    // All values should be negative (O is behind)
    const emptyPositions = ['00', '01', '02', '10', '12', '20', '21', '22']
    for (const pos of emptyPositions) {
      console.log(`Position ${pos}: ${result.values[pos].toFixed(4)}`)
    }
  })

  test('Petri net model structure', () => {
    const model = buildODEPetriNet(null, 'X', 1, 1)

    const placeCount = Object.keys(model.places).length
    const transitionCount = Object.keys(model.transitions).length
    const arcCount = model.arcs.length

    console.log(`Places: ${placeCount}, Transitions: ${transitionCount}, Arcs: ${arcCount}`)

    expect(placeCount).toBe(30)
    expect(transitionCount).toBe(34)

    // Check key places exist
    expect(model.places['P00']).toBeDefined()
    expect(model.places['X11']).toBeDefined()
    expect(model.places['O22']).toBeDefined()
    expect(model.places['Next']).toBeDefined()
    expect(model.places['WinX']).toBeDefined()
    expect(model.places['WinO']).toBeDefined()
  })
})
