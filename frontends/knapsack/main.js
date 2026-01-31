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
let weightChart = null
let valueChart = null
let odeSolution = null
let odeValues = {} // Final ODE values for each item
let baselineSolution = null // Full baseline ODE solution

// Build Petri net with uniform rates (rate=1 for all items)
// Used for baseline comparison showing equal competition
function buildUniformRatePetriNet() {
  const places = {}
  const transitions = {}
  const arcs = []

  // All items start as available
  ITEMS.forEach(item => {
    places[`item${item.id}`] = {
      '@type': 'Place',
      'initial': [1],
      'x': 100 + item.id * 150,
      'y': 100
    }
    places[`item${item.id}_taken`] = {
      '@type': 'Place',
      'initial': [0],
      'x': 100 + item.id * 150,
      'y': 300
    }
  })

  // Full capacity
  places['capacity'] = {
    '@type': 'Place',
    'initial': [MAX_CAPACITY],
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

  // Create transitions for ALL items with rate=1
  ITEMS.forEach(item => {
    const tid = `take_item${item.id}`
    transitions[tid] = {
      '@type': 'Transition',
      'rate': 1, // Uniform rate for baseline
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
  })

  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    'places': places,
    'transitions': transitions,
    'arcs': arcs
  }
}

// Compute rates based on mode:
// - 'baseline': all rates = 1.0 (all items competing equally)
// - 'selected': rate = 1.0 for selected items, 0 for non-selected
// - 'efficiency': rate = item.efficiency for non-selected items (for recommendations)
function getRates(net, mode = 'baseline') {
    const rates = {}
    if (net.transitions instanceof Map) {
        net.transitions.forEach((t, tid) => {
            // Extract item ID from transition ID (e.g., "take_item0" -> 0)
            const match = tid.match(/take_item(\d+)/)
            if (!match) {
                rates[tid] = 1.0
                return
            }
            const itemId = parseInt(match[1], 10)
            const item = ITEMS.find(i => i.id === itemId)

            if (mode === 'baseline') {
                // All items compete equally
                rates[tid] = 1.0
            } else if (mode === 'selected') {
                // Only selected items have non-zero rates
                if (selectedItems.size === 0) {
                    rates[tid] = 1.0 // Fall back to baseline if nothing selected
                } else {
                    rates[tid] = selectedItems.has(itemId) ? 1.0 : 0
                }
            } else if (mode === 'efficiency') {
                // Use efficiency-based rates for non-selected items (for recommendations)
                if (selectedItems.has(itemId)) {
                    rates[tid] = 0 // Already selected, exclude from competition
                } else {
                    rates[tid] = item ? item.efficiency : 1.0
                }
            }
        })
    }
    return rates
}

// Run ODE simulation
// mode: 'baseline', 'selected', or 'efficiency'
function runODE(model, tspan = 2.0, dt = 0.05, mode = 'baseline') {
  try {
    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = getRates(net, mode)
    const prob = new Solver.ODEProblem(net, initialState, [0, tspan], rates)
    const solution = Solver.solve(prob, Solver.Tsit5(), { dt: dt, adaptive: false })

    return solution
  } catch (err) {
    console.error('ODE solve error:', err)
    return null
  }
}

// Compute weight time series from solution - read directly from total_weight place
function computeWeightSeries(solution) {
  if (!solution || !solution.t || !solution.u) return null

  return solution.t.map((t, i) => {
    const state = solution.u[i]
    return state['total_weight'] || 0
  })
}

// Compute value time series from solution - read directly from total_value place
function computeValueSeries(solution) {
  if (!solution || !solution.t || !solution.u) return null

  return solution.t.map((t, i) => {
    const state = solution.u[i]
    return state['total_value'] || 0
  })
}

// Vertical line plugin for hover crosshair
const verticalLinePlugin = {
  id: 'verticalLine',
  afterDraw: (chart) => {
    if (chart.tooltip._active && chart.tooltip._active.length) {
      const ctx = chart.ctx
      const activePoint = chart.tooltip._active[0]
      const x = activePoint.element.x
      const topY = chart.scales.y.top
      const bottomY = chart.scales.y.bottom

      ctx.save()
      ctx.beginPath()
      ctx.moveTo(x, topY)
      ctx.lineTo(x, bottomY)
      ctx.lineWidth = 1
      ctx.strokeStyle = 'rgba(0, 0, 0, 0.4)'
      ctx.setLineDash([4, 4])
      ctx.stroke()
      ctx.restore()
    }
  }
}

// Create chart options
function createChartOptions(yAxisLabel) {
  return {
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
          text: yAxisLabel
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
}

// Initialize charts
function initCharts() {
  const weightCtx = document.getElementById('weight-chart').getContext('2d')
  const valueCtx = document.getElementById('value-chart').getContext('2d')

  weightChart = new Chart(weightCtx, {
    type: 'line',
    data: { labels: [], datasets: [] },
    options: createChartOptions('Weight'),
    plugins: [verticalLinePlugin]
  })

  valueChart = new Chart(valueCtx, {
    type: 'line',
    data: { labels: [], datasets: [] },
    options: createChartOptions('Value'),
    plugins: [verticalLinePlugin]
  })
}

// Compute baseline ODE (all items with rate=1)
function computeBaseline() {
  const model = buildUniformRatePetriNet()
  baselineSolution = runODE(model, 10.0, 0.05, 'baseline')
}

// Update charts with ODE solution
function updateCharts() {
  if (!weightChart || !valueChart || !baselineSolution) return

  const currentSelectedWeight = getCurrentWeight()
  const isOverCapacity = currentSelectedWeight > MAX_CAPACITY
  const selectedIds = Array.from(selectedItems)
  const hasSelection = selectedIds.length > 0

  // Compute baseline series (all items competing)
  const allWeight = computeWeightSeries(baselineSolution)
  const allValue = computeValueSeries(baselineSolution)
  const timeData = baselineSolution.t

  // Run separate ODE for selected items only (rate=0 for non-selected)
  let selectedWeight = null
  let selectedValue = null
  let selectedTimeData = null
  if (hasSelection) {
    const selectedModel = buildUniformRatePetriNet()
    const selectedSolution = runODE(selectedModel, 10.0, 0.05, 'selected')
    if (selectedSolution) {
      selectedWeight = computeWeightSeries(selectedSolution)
      selectedValue = computeValueSeries(selectedSolution)
      selectedTimeData = selectedSolution.t
    }
  }

  // Weight chart datasets
  const weightDatasets = []

  // All items weight - dashed green
  weightDatasets.push({
    label: 'Weight (all)',
    data: timeData.map((t, i) => ({ x: t, y: allWeight[i] })),
    borderColor: '#2ecc71',
    backgroundColor: 'transparent',
    borderWidth: 2,
    borderDash: [6, 3],
    fill: false,
    tension: 0.3,
    pointRadius: 0,
  })

  // Selected items weight - solid green (from separate ODE with rate=0 for non-selected)
  if (hasSelection && selectedWeight && selectedTimeData) {
    weightDatasets.push({
      label: 'Weight (selected)',
      data: selectedTimeData.map((t, i) => ({ x: t, y: selectedWeight[i] })),
      borderColor: isOverCapacity ? '#dc3545' : '#27ae60',
      backgroundColor: isOverCapacity ? '#dc354533' : '#27ae6033',
      borderWidth: 3,
      fill: false,
      tension: 0.3,
      pointRadius: 0,
    })
  }

  // Capacity limit line
  weightDatasets.push({
    label: 'MAX',
    data: timeData.map(t => ({ x: t, y: MAX_CAPACITY })),
    borderColor: isOverCapacity ? '#dc3545' : '#000000',
    backgroundColor: 'transparent',
    borderWidth: isOverCapacity ? 2.5 : 1.5,
    borderDash: [4, 4],
    fill: false,
    tension: 0,
    pointRadius: 0,
  })

  weightChart.data.datasets = weightDatasets
  weightChart.update()

  // Value chart datasets
  const valueDatasets = []

  // All items value - dashed blue
  valueDatasets.push({
    label: 'Value (all)',
    data: timeData.map((t, i) => ({ x: t, y: allValue[i] })),
    borderColor: '#3498db',
    backgroundColor: 'transparent',
    borderWidth: 2,
    borderDash: [6, 3],
    fill: false,
    tension: 0.3,
    pointRadius: 0,
  })

  // Selected items value - solid blue (from separate ODE with rate=0 for non-selected)
  if (hasSelection && selectedValue && selectedTimeData) {
    valueDatasets.push({
      label: 'Value (selected)',
      data: selectedTimeData.map((t, i) => ({ x: t, y: selectedValue[i] })),
      borderColor: '#2980b9',
      backgroundColor: '#2980b933',
      borderWidth: 3,
      fill: false,
      tension: 0.3,
      pointRadius: 0,
    })
  }

  // Max value limit line
  valueDatasets.push({
    label: 'MAX',
    data: timeData.map(t => ({ x: t, y: 50 })),
    borderColor: '#000000',
    backgroundColor: 'transparent',
    borderWidth: 1.5,
    borderDash: [4, 4],
    fill: false,
    tension: 0,
    pointRadius: 0,
  })

  valueChart.data.datasets = valueDatasets
  valueChart.update()

  // Update legends
  updateLegends(hasSelection, isOverCapacity)
}

function updateLegends(hasSelection, isOverCapacity) {
  // Weight legend
  const weightLegendEl = document.getElementById('weight-legend')
  let weightHtml = ''

  if (hasSelection) {
    const color = isOverCapacity ? '#dc3545' : '#27ae60'
    weightHtml += `
      <div class="legend-item">
        <div class="legend-color" style="background: ${color};"></div>
        <span>Weight (selected)</span>
      </div>
    `
  }

  weightHtml += `
    <div class="legend-item">
      <div class="legend-line" style="border-top: 2px dashed #2ecc71;"></div>
      <span>Weight (all)</span>
    </div>
    <div class="legend-item">
      <div class="legend-line" style="border-top: 2px dotted ${isOverCapacity ? '#dc3545' : '#000'};"></div>
      <span>MAX</span>
    </div>
  `

  weightLegendEl.innerHTML = weightHtml

  // Value legend
  const valueLegendEl = document.getElementById('value-legend')
  let valueHtml = ''

  if (hasSelection) {
    valueHtml += `
      <div class="legend-item">
        <div class="legend-color" style="background: #2980b9;"></div>
        <span>Value (selected)</span>
      </div>
    `
  }

  valueHtml += `
    <div class="legend-item">
      <div class="legend-line" style="border-top: 2px dashed #3498db;"></div>
      <span>Value (all)</span>
    </div>
    <div class="legend-item">
      <div class="legend-line" style="border-top: 2px dotted #000;"></div>
      <span>MAX</span>
    </div>
  `

  valueLegendEl.innerHTML = valueHtml
}

// Compute ODE values for recommendation
function computeODEValues() {
  const model = buildUniformRatePetriNet()
  const solution = runODE(model, 10.0, 0.05, 'efficiency')

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
  const remainingCapacity = MAX_CAPACITY - getCurrentWeight()
  let recommendedId = null
  let maxScore = -1
  ITEMS.forEach(item => {
    if (!selectedItems.has(item.id) && canFitItem(item)) {
      let score = item.efficiency
      if (remainingCapacity <= 10) {
        const utilizationRatio = item.weight / remainingCapacity
        score = item.value / item.weight
        score += utilizationRatio * 2
        score += item.value / 50
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
  } else {
    selectedItems.add(itemId)
  }

  updateUI()
  updateCharts()
  updateAnalysis()
}

// Reset selection
window.resetSelection = function() {
  selectedItems.clear()
  updateUI()
  updateCharts()
  updateAnalysis()
}

// Run simulation (kept for button compatibility)
window.runSimulation = function() {
  updateCharts()
  updateAnalysis()
}

// Update UI
function updateUI() {
  renderItems()

  const currentWeight = getCurrentWeight()
  const currentValue = getCurrentValue()
  const usedPercent = (currentWeight / MAX_CAPACITY) * 100
  const isOverCapacity = currentWeight > MAX_CAPACITY

  document.getElementById('capacity-display').textContent = `${currentWeight} / ${MAX_CAPACITY}`
  document.getElementById('total-value').textContent = currentValue
  document.getElementById('total-weight').textContent = currentWeight

  const fillEl = document.getElementById('capacity-fill')
  fillEl.style.width = `${Math.min(usedPercent, 100)}%`
  fillEl.className = 'capacity-fill'
  if (isOverCapacity) fillEl.classList.add('over')
  else if (usedPercent >= 100) fillEl.classList.add('full')
  else if (usedPercent >= 80) fillEl.classList.add('warning')

  // Show/hide over-capacity warning
  const warningEl = document.getElementById('over-capacity-warning')
  if (warningEl) {
    warningEl.style.display = isOverCapacity ? 'block' : 'none'
    if (isOverCapacity) {
      const overBy = currentWeight - MAX_CAPACITY
      warningEl.textContent = `⚠️ Over capacity by ${overBy} weight units! Remove items to fit.`
    }
  }
}

// Update analysis panel
function updateAnalysis() {
  const analysisEl = document.getElementById('analysis-content')

  if (!baselineSolution) {
    analysisEl.innerHTML = '<p class="placeholder">Run ODE simulation to see analysis</p>'
    return
  }

  const finalState = baselineSolution.u[baselineSolution.u.length - 1]

  // Get baseline expected values (all items competing)
  let baselineWeight = 0
  let baselineValue = 0
  ITEMS.forEach(item => {
    const takenVal = finalState[`item${item.id}_taken`] || 0
    baselineWeight += takenVal * item.weight
    baselineValue += takenVal * item.value
  })

  // Run separate ODE for selected items only (rate=0 for non-selected)
  let selectedWeight = 0
  let selectedValue = 0
  if (selectedItems.size > 0) {
    const selectedModel = buildUniformRatePetriNet()
    const selectedSolution = runODE(selectedModel, 10.0, 0.05, 'selected')
    if (selectedSolution) {
      const selectedFinalState = selectedSolution.u[selectedSolution.u.length - 1]
      ITEMS.forEach(item => {
        const takenVal = selectedFinalState[`item${item.id}_taken`] || 0
        selectedWeight += takenVal * item.weight
        selectedValue += takenVal * item.value
      })
    }
  }

  const optimalValue = 38
  const efficiency = ((selectedValue / optimalValue) * 100).toFixed(1)

  let html = ''

  // Selected items contribution from ODE
  if (selectedItems.size > 0) {
    html += `
      <div class="analysis-item">
        <div class="analysis-icon">📊</div>
        <div class="analysis-text">
          Selected (ODE): <strong>Value ${selectedValue.toFixed(1)}</strong>, Weight ${selectedWeight.toFixed(1)}
        </div>
        <div class="analysis-value ${selectedValue >= optimalValue ? 'positive' : ''}">${efficiency}%</div>
      </div>
    `
  }

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
  initCharts()

  // Compute baseline (all items competing) for comparison
  computeBaseline()

  updateUI()
  updateCharts()
  updateAnalysis()
})
