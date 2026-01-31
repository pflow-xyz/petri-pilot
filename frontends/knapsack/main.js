// Knapsack Optimizer - ODE Simulation Frontend
// Uses pflow ODE solver for continuous optimization

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@1.11.0/public/petri-solver.js'

// Item definitions
const ITEMS = [
  { id: 0, name: 'Item A', icon: '📦', weight: 2, value: 10, efficiency: 5.0, color: '#e74c3c' },
  { id: 1, name: 'Item B', icon: '💎', weight: 5, value: 15, efficiency: 3.0, color: '#3498db' },
  { id: 2, name: 'Item C', icon: '📚', weight: 6, value: 12, efficiency: 2.0, color: '#f39c12' },
  { id: 3, name: 'Item D', icon: '🏆', weight: 8, value: 13, efficiency: 1.625, color: '#9b59b6' },
]

const MAX_CAPACITY = 15
const MAX_EFFICIENCY = Math.max(...ITEMS.map(i => i.efficiency))

// State
let selectedItems = new Set()
let odeChart = null
let odeSolution = null
let odeValues = {} // Final ODE values for each item
let baselineSeries = null // Baseline ODE with all items competing

// Build Petri net model for knapsack
function buildKnapsackPetriNet(excludeItems = []) {
  const places = {}
  const transitions = {}
  const arcs = []

  // Item availability places
  ITEMS.forEach(item => {
    const isExcluded = excludeItems.includes(item.id)
    places[`item${item.id}`] = {
      '@type': 'Place',
      'initial': [isExcluded ? 0 : 1],
      'x': 100 + item.id * 150,
      'y': 100
    }
    places[`item${item.id}_taken`] = {
      '@type': 'Place',
      'initial': [isExcluded ? 1 : 0],
      'x': 100 + item.id * 150,
      'y': 300
    }
  })

  // Capacity place - reduced by weight of already-selected items
  const usedCapacity = excludeItems.reduce((sum, id) => {
    const item = ITEMS.find(i => i.id === id)
    return sum + (item ? item.weight : 0)
  }, 0)
  places['capacity'] = {
    '@type': 'Place',
    'initial': [MAX_CAPACITY - usedCapacity],
    'x': 300,
    'y': 200
  }

  // Total value and weight places
  places['total_value'] = {
    '@type': 'Place',
    'initial': [0],
    'x': 500,
    'y': 400
  }
  places['total_weight'] = {
    '@type': 'Place',
    'initial': [0],
    'x': 600,
    'y': 400
  }

  // Take item transitions
  ITEMS.forEach(item => {
    const isExcluded = excludeItems.includes(item.id)
    if (!isExcluded) {
      const tid = `take_item${item.id}`
      transitions[tid] = {
        '@type': 'Transition',
        'rate': item.efficiency, // Rate based on efficiency
        'x': 100 + item.id * 150,
        'y': 200
      }

      // Input arcs
      arcs.push({ '@type': 'Arrow', 'source': `item${item.id}`, 'target': tid, 'weight': [1] })
      arcs.push({ '@type': 'Arrow', 'source': 'capacity', 'target': tid, 'weight': [item.weight] })

      // Output arcs
      arcs.push({ '@type': 'Arrow', 'source': tid, 'target': `item${item.id}_taken`, 'weight': [1] })
      arcs.push({ '@type': 'Arrow', 'source': tid, 'target': 'total_value', 'weight': [item.value] })
      arcs.push({ '@type': 'Arrow', 'source': tid, 'target': 'total_weight', 'weight': [item.weight] })
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

// Run ODE simulation
function runODE(model, tspan = 2.0, dt = 0.05) {
  try {
    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = Solver.setRates(net)

    const prob = new Solver.ODEProblem(net, initialState, [0, tspan], rates)
    const solution = Solver.solve(prob, Solver.Tsit5(), { dt: dt, adaptive: false })

    return solution
  } catch (err) {
    console.error('ODE solve error:', err)
    return null
  }
}

// Extract time series for places
function extractTimeSeries(solution, placeNames) {
  if (!solution || !solution.t || !solution.u) return null

  const series = {}
  placeNames.forEach(name => {
    series[name] = solution.u.map(state => state[name] || 0)
  })
  series.t = solution.t

  return series
}

// Compute expected total weight from ODE (weighted sum of item weights by their taken probability)
function computeExpectedWeight(solution) {
  if (!solution || !solution.t || !solution.u) return null

  return solution.t.map((t, i) => {
    const state = solution.u[i]
    let weight = 0
    ITEMS.forEach(item => {
      const takenVal = state[`item${item.id}_taken`] || 0
      weight += takenVal * item.weight
    })
    return weight
  })
}

// Compute expected total value from ODE
function computeExpectedValue(solution) {
  if (!solution || !solution.t || !solution.u) return null

  return solution.t.map((t, i) => {
    const state = solution.u[i]
    let value = 0
    ITEMS.forEach(item => {
      const takenVal = state[`item${item.id}_taken`] || 0
      value += takenVal * item.value
    })
    return value
  })
}

// Initialize chart
function initChart() {
  const ctx = document.getElementById('ode-chart').getContext('2d')

  odeChart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: [],
      datasets: []
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: {
        duration: 0
      },
      interaction: {
        mode: 'index',
        intersect: false,
      },
      scales: {
        x: {
          type: 'linear',
          title: {
            display: true,
            text: 'Time'
          },
          ticks: {
            callback: (value) => value.toFixed(1)
          }
        },
        y: {
          title: {
            display: true,
            text: 'Tokens'
          },
          min: 0,
          beginAtZero: true,
          suggestedMin: 0
        }
      },
      plugins: {
        legend: {
          display: false
        },
        tooltip: {
          callbacks: {
            label: function(context) {
              return `${context.dataset.label}: ${context.parsed.y.toFixed(3)}`
            }
          }
        }
      }
    }
  })
}

// Compute baseline ODE (all items competing, no exclusions)
function computeBaseline() {
  const model = buildKnapsackPetriNet([]) // No exclusions
  const solution = runODE(model, 3.0, 0.05)
  if (solution) {
    const placeNames = [
      ...ITEMS.map(i => `item${i.id}_taken`),
      'capacity'
    ]
    baselineSeries = extractTimeSeries(solution, placeNames)
    // Add expected weight and value to baseline
    baselineSeries.expected_weight = computeExpectedWeight(solution)
    baselineSeries.expected_value = computeExpectedValue(solution)
  }
}

// Update chart with ODE solution
function updateChart(series, currentSolution) {
  if (!odeChart || !series) return

  const datasets = []

  // Baseline Weight (all items) - dashed green
  if (baselineSeries && baselineSeries.expected_weight) {
    datasets.push({
      label: 'Weight (all)',
      data: baselineSeries.t.map((t, i) => ({ x: t, y: baselineSeries.expected_weight[i] })),
      borderColor: '#2ecc71',
      backgroundColor: 'transparent',
      borderWidth: 2,
      borderDash: [6, 3],
      fill: false,
      tension: 0.3,
      pointRadius: 0,
    })
  }

  // Baseline Value (all items) - dashed blue
  if (baselineSeries && baselineSeries.expected_value) {
    datasets.push({
      label: 'Value (all)',
      data: baselineSeries.t.map((t, i) => ({ x: t, y: baselineSeries.expected_value[i] })),
      borderColor: '#3498db',
      backgroundColor: 'transparent',
      borderWidth: 2,
      borderDash: [6, 3],
      fill: false,
      tension: 0.3,
      pointRadius: 0,
    })
  }

  // Current Weight (selected) - solid green
  // Note: selected items already have item_taken=1 in the model, so ODE includes them
  if (currentSolution) {
    const currentWeight = computeExpectedWeight(currentSolution)
    if (currentWeight) {
      datasets.push({
        label: 'Weight (selected)',
        data: currentSolution.t.map((t, i) => ({ x: t, y: currentWeight[i] })),
        borderColor: '#27ae60',
        backgroundColor: '#27ae6033',
        borderWidth: 3,
        fill: false,
        tension: 0.3,
        pointRadius: 0,
      })
    }
  }

  // Current Value (selected) - solid blue
  // Note: selected items already have item_taken=1 in the model, so ODE includes them
  if (currentSolution) {
    const currentValue = computeExpectedValue(currentSolution)
    if (currentValue) {
      datasets.push({
        label: 'Value (selected)',
        data: currentSolution.t.map((t, i) => ({ x: t, y: currentValue[i] })),
        borderColor: '#2980b9',
        backgroundColor: '#2980b933',
        borderWidth: 3,
        fill: false,
        tension: 0.3,
        pointRadius: 0,
      })
    }
  }

  odeChart.data.datasets = datasets
  odeChart.update()

  // Update legend
  updateLegend()
}

function updateLegend() {
  const legendEl = document.getElementById('chart-legend')
  let html = ''

  // Weight (selected) - solid green
  html += `
    <div class="legend-item">
      <div class="legend-color" style="background: #27ae60;"></div>
      <span>Weight (selected)</span>
    </div>
  `

  // Weight (all) - dashed green
  html += `
    <div class="legend-item">
      <div class="legend-color" style="background: transparent; border: 2px dashed #2ecc71;"></div>
      <span>Weight (all)</span>
    </div>
  `

  // Value (selected) - solid blue
  html += `
    <div class="legend-item">
      <div class="legend-color" style="background: #2980b9;"></div>
      <span>Value (selected)</span>
    </div>
  `

  // Value (all) - dashed blue
  html += `
    <div class="legend-item">
      <div class="legend-color" style="background: transparent; border: 2px dashed #3498db;"></div>
      <span>Value (all)</span>
    </div>
  `

  legendEl.innerHTML = html
}

// Compute ODE values for recommendation
function computeODEValues() {
  const model = buildKnapsackPetriNet(Array.from(selectedItems))
  const solution = runODE(model, 3.0, 0.1)

  if (!solution || !solution.u) return {}

  const finalState = solution.u[solution.u.length - 1]
  const values = {}

  ITEMS.forEach(item => {
    if (!selectedItems.has(item.id)) {
      // Value is how much of the item gets taken in the ODE
      const takenKey = `item${item.id}_taken`
      values[item.id] = finalState[takenKey] || 0
    }
  })

  return values
}

// Get heat color based on ODE value
function getHeatColor(value, minVal, maxVal) {
  if (maxVal <= minVal) return 'rgb(150, 150, 150)'

  const normalized = Math.max(0, Math.min(1, (value - minVal) / (maxVal - minVal)))

  // Blue (low) to Red (high)
  const r = Math.round(255 * normalized)
  const g = Math.round(100 - 50 * normalized)
  const b = Math.round(255 * (1 - normalized))

  return `rgb(${r}, ${g}, ${b})`
}

// Render items grid
function renderItems() {
  const grid = document.getElementById('items-grid')

  // Compute ODE values for recommendations
  odeValues = computeODEValues()
  const odeValuesArray = Object.values(odeValues)
  const minODE = odeValuesArray.length > 0 ? Math.min(...odeValuesArray) : 0
  const maxODE = odeValuesArray.length > 0 ? Math.max(...odeValuesArray) : 1

  // Find recommended item using efficiency-based scoring
  // When capacity is tight, prefer items that maximize value AND fit well
  const remainingCapacity = MAX_CAPACITY - getCurrentWeight()
  let recommendedId = null
  let maxScore = -1
  ITEMS.forEach(item => {
    if (!selectedItems.has(item.id) && canFitItem(item)) {
      // Base score is efficiency (matches ODE rate)
      let score = item.efficiency
      // When capacity is tight (<= 10), prioritize filling capacity with best value
      if (remainingCapacity <= 10) {
        const utilizationRatio = item.weight / remainingCapacity
        // Strong bonus for items that fill remaining capacity well
        // This can override efficiency for the final pick
        score = item.value / item.weight  // Base: value efficiency
        score += utilizationRatio * 2  // Big bonus for capacity fit (up to 2.0)
        score += item.value / 50  // Bonus for absolute value
      }
      if (score > maxScore) {
        maxScore = score
        recommendedId = item.id
      }
    }
  })

  let html = ''
  ITEMS.forEach(item => {
    const isSelected = selectedItems.has(item.id)
    const canFit = canFitItem(item)
    const isRecommended = recommendedId === item.id && !isSelected

    let cardClass = 'item-card'
    if (isSelected) cardClass += ' selected'
    if (!isSelected && !canFit) cardClass += ' disabled'
    if (isRecommended) cardClass += ' recommended'

    const odeVal = odeValues[item.id] || 0
    const heatColor = getHeatColor(odeVal, minODE, maxODE)

    html += `
      <div class="${cardClass}" onclick="toggleItem(${item.id})" data-item="${item.id}">
        ${isSelected ? '<span class="item-status in-bag">In Bag</span>' : ''}
        <div class="item-heat-overlay" style="background: ${heatColor};">
          <span class="item-heat-value">${odeVal.toFixed(2)}</span>
        </div>
        <div class="item-header">
          <span class="item-icon">${item.icon}</span>
          <span class="item-name">${item.name}</span>
        </div>
        <div class="item-stats">
          <div class="item-stat">
            <div class="item-stat-label">Weight</div>
            <div class="item-stat-value weight">${item.weight}</div>
          </div>
          <div class="item-stat">
            <div class="item-stat-label">Value</div>
            <div class="item-stat-value value">${item.value}</div>
          </div>
          <div class="item-stat">
            <div class="item-stat-label">Eff.</div>
            <div class="item-stat-value efficiency">${item.efficiency.toFixed(1)}</div>
          </div>
        </div>
      </div>
    `
  })

  grid.innerHTML = html
}

// Check if item can fit
function canFitItem(item) {
  const currentWeight = getCurrentWeight()
  return currentWeight + item.weight <= MAX_CAPACITY
}

// Get current weight
function getCurrentWeight() {
  let weight = 0
  selectedItems.forEach(id => {
    const item = ITEMS.find(i => i.id === id)
    if (item) weight += item.weight
  })
  return weight
}

// Get current value
function getCurrentValue() {
  let value = 0
  selectedItems.forEach(id => {
    const item = ITEMS.find(i => i.id === id)
    if (item) value += item.value
  })
  return value
}

// Toggle item selection
window.toggleItem = function(itemId) {
  const item = ITEMS.find(i => i.id === itemId)
  if (!item) return

  if (selectedItems.has(itemId)) {
    selectedItems.delete(itemId)
  } else if (canFitItem(item)) {
    selectedItems.add(itemId)
  }

  updateUI()
  runSimulation()
}

// Reset selection
window.resetSelection = function() {
  selectedItems.clear()
  updateUI()
  runSimulation()
}

// Run simulation
window.runSimulation = function() {
  const model = buildKnapsackPetriNet(Array.from(selectedItems))
  odeSolution = runODE(model, 3.0, 0.05)

  if (odeSolution) {
    const placeNames = [
      ...ITEMS.filter(i => !selectedItems.has(i.id)).map(i => `item${i.id}_taken`),
      'capacity',
      'total_value',
      'total_weight'
    ]
    const series = extractTimeSeries(odeSolution, placeNames)
    updateChart(series, odeSolution)
    updateAnalysis(series)
  }
}

// Update UI
function updateUI() {
  renderItems()

  const currentWeight = getCurrentWeight()
  const currentValue = getCurrentValue()
  const usedPercent = (currentWeight / MAX_CAPACITY) * 100

  document.getElementById('capacity-display').textContent = `${currentWeight} / ${MAX_CAPACITY}`
  document.getElementById('total-value').textContent = currentValue
  document.getElementById('total-weight').textContent = currentWeight

  const fillEl = document.getElementById('capacity-fill')
  fillEl.style.width = `${usedPercent}%`
  fillEl.className = 'capacity-fill'
  if (usedPercent >= 100) fillEl.classList.add('full')
  else if (usedPercent >= 80) fillEl.classList.add('warning')
}

// Update analysis panel
function updateAnalysis(series) {
  const analysisEl = document.getElementById('analysis-content')

  if (!series || !odeSolution) {
    analysisEl.innerHTML = '<p class="placeholder">Run ODE simulation to see analysis</p>'
    return
  }

  const finalState = odeSolution.u[odeSolution.u.length - 1]

  // Calculate expected weight and value from ODE (continuous/fractional)
  let expectedWeight = getCurrentWeight()
  let expectedValue = getCurrentValue()

  ITEMS.forEach(item => {
    if (!selectedItems.has(item.id)) {
      const takenVal = finalState[`item${item.id}_taken`] || 0
      expectedWeight += takenVal * item.weight
      expectedValue += takenVal * item.value
    }
  })

  // Get baseline expected values (all items competing)
  let baselineWeight = 0
  let baselineValue = 0
  if (baselineSeries && baselineSeries.expected_weight) {
    const finalIdx = baselineSeries.expected_weight.length - 1
    baselineWeight = baselineSeries.expected_weight[finalIdx]
    // Compute baseline value too
    const baselineFinalState = baselineSeries
    ITEMS.forEach(item => {
      const takenVal = baselineFinalState[`item${item.id}_taken`]
      if (takenVal) {
        baselineValue += takenVal[finalIdx] * item.value
      }
    })
  }

  const optimalValue = 38
  const efficiency = ((expectedValue / optimalValue) * 100).toFixed(1)

  let html = ''

  // Expected totals from ODE (continuous approximation)
  html += `
    <div class="analysis-item">
      <div class="analysis-icon">📊</div>
      <div class="analysis-text">
        Expected (ODE): <strong>Value ${expectedValue.toFixed(1)}</strong>, Weight ${expectedWeight.toFixed(1)}
      </div>
      <div class="analysis-value ${expectedValue >= optimalValue ? 'positive' : ''}">${efficiency}%</div>
    </div>
  `

  // Baseline comparison
  html += `
    <div class="analysis-item">
      <div class="analysis-icon">📈</div>
      <div class="analysis-text">
        Baseline (all items): Value ${baselineValue.toFixed(1)}, Weight <strong>${baselineWeight.toFixed(1)}</strong>
      </div>
    </div>
  `

  // Optimal reference
  html += `
    <div class="analysis-item">
      <div class="analysis-icon">🎯</div>
      <div class="analysis-text">
        Optimal: A+B+D = Value <strong>38</strong>, Weight 15
      </div>
    </div>
  `

  // Efficiency ranking
  html += `
    <div class="efficiency-bars">
      <div style="font-size: 0.85rem; color: #666; margin-bottom: 0.5rem;">Item Flow Rates (efficiency-based)</div>
  `

  ITEMS.forEach(item => {
    const percent = (item.efficiency / MAX_EFFICIENCY) * 100
    const isSelected = selectedItems.has(item.id)
    const opacity = isSelected ? '0.4' : '1'

    html += `
      <div class="efficiency-bar-item" style="opacity: ${opacity};">
        <div class="efficiency-bar-label">${item.icon} ${item.name}</div>
        <div class="efficiency-bar-track">
          <div class="efficiency-bar-fill" style="width: ${percent}%; background: ${item.color};"></div>
        </div>
        <div class="efficiency-bar-value">${item.efficiency.toFixed(1)}</div>
      </div>
    `
  })

  html += '</div>'

  analysisEl.innerHTML = html
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  initChart()

  // Compute baseline (all items competing) for comparison
  computeBaseline()

  updateUI()
  updateLegend()

  // Run initial simulation
  runSimulation()
})
