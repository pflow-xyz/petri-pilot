// Producer-Consumer - Bounded Buffer Simulation with Charts

let state = {}
let capacity = 5
let autoRunning = false
let autoTimer = null
let totalProduced = 0
let totalConsumed = 0
let stepCount = 0
let startTime = null

// Statistics tracking
let bufferHistory = []
let producedHistory = []
let consumedHistory = []
let prodBlockedSteps = 0
let consBlockedSteps = 0

let bufferChart = null
let throughputChart = null

function initState() {
  state = {
    producer_idle: 1,
    producing: 0,
    buffer: 0,
    buffer_space: capacity,
    consumer_idle: 1,
    consuming: 0,
    consumed: 0
  }
  totalProduced = 0
  totalConsumed = 0
  stepCount = 0
  startTime = null
  bufferHistory = [{ step: 0, level: 0 }]
  producedHistory = [{ step: 0, count: 0 }]
  consumedHistory = [{ step: 0, count: 0 }]
  prodBlockedSteps = 0
  consBlockedSteps = 0
}

function getTransitions() {
  return {
    start_produce: {
      label: 'Start Produce',
      desc: 'Producer begins creating item (needs buffer space)',
      actor: 'producer',
      inputs: { producer_idle: 1, buffer_space: 1 },
      outputs: { producing: 1 }
    },
    finish_produce: {
      label: 'Finish Produce',
      desc: 'Item placed in buffer',
      actor: 'producer',
      inputs: { producing: 1 },
      outputs: { producer_idle: 1, buffer: 1 }
    },
    start_consume: {
      label: 'Start Consume',
      desc: 'Consumer takes item from buffer',
      actor: 'consumer',
      inputs: { consumer_idle: 1, buffer: 1 },
      outputs: { consuming: 1 }
    },
    finish_consume: {
      label: 'Finish Consume',
      desc: 'Item processed, space freed',
      actor: 'consumer',
      inputs: { consuming: 1 },
      outputs: { consumer_idle: 1, buffer_space: 1, consumed: 1 }
    }
  }
}

function isEnabled(t) {
  for (const [place, weight] of Object.entries(t.inputs)) {
    if ((state[place] || 0) < weight) return false
  }
  return true
}

function fire(transitionId) {
  const transitions = getTransitions()
  const t = transitions[transitionId]
  if (!t || !isEnabled(t)) return false

  // Track blocking before firing
  const prodBlocked = state.buffer_space === 0 && state.producer_idle === 1
  const consBlocked = state.buffer === 0 && state.consumer_idle === 1
  if (prodBlocked) prodBlockedSteps++
  if (consBlocked) consBlockedSteps++

  for (const [place, weight] of Object.entries(t.inputs)) {
    state[place] -= weight
  }
  for (const [place, weight] of Object.entries(t.outputs)) {
    state[place] = (state[place] || 0) + weight
  }

  if (transitionId === 'finish_produce') totalProduced++
  if (transitionId === 'finish_consume') totalConsumed++

  stepCount++
  if (!startTime) startTime = performance.now()

  // Record history
  bufferHistory.push({ step: stepCount, level: state.buffer })
  producedHistory.push({ step: stepCount, count: totalProduced })
  consumedHistory.push({ step: stepCount, count: totalConsumed })

  // Cap history length
  if (bufferHistory.length > 500) {
    bufferHistory = bufferHistory.slice(-400)
    producedHistory = producedHistory.slice(-400)
    consumedHistory = consumedHistory.slice(-400)
  }

  updateUI()
  updateCharts()
  updatePredictions()
  return true
}

function getEnabledTransitions() {
  const transitions = getTransitions()
  return Object.entries(transitions)
    .filter(([id, t]) => isEnabled(t))
    .map(([id]) => id)
}

function updateUI() {
  // Buffer slots
  const slotsContainer = document.getElementById('buffer-slots')
  let slotsHtml = ''
  for (let i = 0; i < capacity; i++) {
    slotsHtml += `<div class="slot${i < state.buffer ? ' filled' : ''}"></div>`
  }
  slotsContainer.innerHTML = slotsHtml

  // Actor states
  const producerState = state.producing ? 'Working' : (state.buffer_space === 0 && state.producer_idle ? 'Blocked' : 'Idle')
  const consumerState = state.consuming ? 'Working' : (state.buffer === 0 && state.consumer_idle ? 'Blocked' : 'Idle')

  document.getElementById('producer-state').textContent = producerState
  document.getElementById('consumer-state').textContent = consumerState

  document.getElementById('producer-icon').className = 'actor-icon' + (state.producing ? ' working' : (producerState === 'Blocked' ? ' blocked' : ''))
  document.getElementById('consumer-icon').className = 'actor-icon' + (state.consuming ? ' working' : (consumerState === 'Blocked' ? ' blocked' : ''))

  // Counts
  document.getElementById('buffer-count').textContent = state.buffer
  document.getElementById('produced-count').textContent = totalProduced
  document.getElementById('consumed-count').textContent = totalConsumed
  document.getElementById('capacity-display').textContent = capacity

  // Transition buttons
  renderTransitions()
}

function renderTransitions() {
  const container = document.getElementById('transitions-container')
  const transitions = getTransitions()
  const enabled = getEnabledTransitions()
  let html = ''

  for (const [id, t] of Object.entries(transitions)) {
    const isEn = enabled.includes(id)
    const color = t.actor === 'producer' ? '#3498db' : '#e67e22'
    html += `<button class="btn-transition ${isEn ? 'enabled' : 'disabled'}"
              data-transition="${id}" ${isEn ? '' : 'disabled'}
              style="${isEn ? `border-color: ${color}; color: ${color};` : ''}">
              <span class="trans-label">${t.label}</span>
              <span class="trans-desc">${t.desc}</span>
            </button>`
  }
  container.innerHTML = html

  // Attach click handlers
  container.querySelectorAll('.btn-transition.enabled').forEach(btn => {
    btn.addEventListener('click', () => fire(btn.dataset.transition))
  })
}

function updatePredictions() {
  // Throughput
  const throughputEl = document.getElementById('pred-throughput')
  if (startTime && stepCount > 0) {
    const elapsed = (performance.now() - startTime) / 1000
    const throughput = elapsed > 0 ? (totalConsumed / elapsed) : 0
    throughputEl.textContent = throughput.toFixed(1)
  } else {
    throughputEl.textContent = '--'
  }

  // Buffer utilization
  const utilEl = document.getElementById('pred-utilization')
  if (bufferHistory.length > 1) {
    const avg = bufferHistory.reduce((sum, h) => sum + h.level, 0) / bufferHistory.length
    utilEl.textContent = Math.round((avg / capacity) * 100) + '%'
  } else {
    utilEl.textContent = '--'
  }

  // Blocking percentages
  const prodBlockedEl = document.getElementById('pred-prod-blocked')
  const consBlockedEl = document.getElementById('pred-cons-blocked')
  if (stepCount > 0) {
    prodBlockedEl.textContent = Math.round((prodBlockedSteps / stepCount) * 100) + '%'
    consBlockedEl.textContent = Math.round((consBlockedSteps / stepCount) * 100) + '%'
  } else {
    prodBlockedEl.textContent = '--'
    consBlockedEl.textContent = '--'
  }
}

function initCharts() {
  const bufCtx = document.getElementById('buffer-chart').getContext('2d')
  bufferChart = new Chart(bufCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Step' }, min: 0 },
        y: {
          title: { display: true, text: 'Buffer Level' },
          min: 0,
          max: capacity + 0.5,
          ticks: { stepSize: 1 }
        }
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) => `Buffer: ${ctx.parsed.y}/${capacity}`
          }
        }
      }
    }
  })

  const tpCtx = document.getElementById('throughput-chart').getContext('2d')
  throughputChart = new Chart(tpCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Step' }, min: 0 },
        y: { title: { display: true, text: 'Items' }, min: 0 }
      },
      plugins: {
        legend: { display: true, position: 'top' }
      }
    }
  })
}

function updateCharts() {
  // Buffer level chart
  const bufData = bufferHistory.map(h => ({ x: h.step, y: h.level }))

  bufferChart.data.datasets = [
    {
      label: 'Buffer Level',
      data: bufData,
      borderColor: '#2ecc71',
      backgroundColor: 'rgba(46, 204, 113, 0.15)',
      borderWidth: 2,
      fill: true,
      stepped: true,
      pointRadius: 0
    }
  ]
  bufferChart.options.scales.y.max = capacity + 0.5
  bufferChart.update()

  // Throughput chart
  const prodData = producedHistory.map(h => ({ x: h.step, y: h.count }))
  const consData = consumedHistory.map(h => ({ x: h.step, y: h.count }))

  throughputChart.data.datasets = [
    {
      label: 'Produced',
      data: prodData,
      borderColor: '#3498db',
      backgroundColor: 'rgba(52, 152, 219, 0.08)',
      borderWidth: 2,
      fill: true,
      pointRadius: 0,
      tension: 0.1
    },
    {
      label: 'Consumed',
      data: consData,
      borderColor: '#e67e22',
      backgroundColor: 'rgba(230, 126, 34, 0.08)',
      borderWidth: 2,
      fill: true,
      pointRadius: 0,
      tension: 0.1
    }
  ]
  throughputChart.update()
}

function autoStep() {
  const enabled = getEnabledTransitions()
  if (enabled.length === 0) return
  const pick = enabled[Math.floor(Math.random() * enabled.length)]
  fire(pick)
}

function toggleAuto() {
  autoRunning = !autoRunning
  const btn = document.getElementById('auto-btn')
  if (autoRunning) {
    btn.textContent = 'Stop'
    btn.classList.remove('btn-primary')
    btn.classList.add('btn-warning')
    const speed = parseInt(document.getElementById('speed').value)
    autoTimer = setInterval(autoStep, speed)
  } else {
    btn.textContent = 'Auto Run'
    btn.classList.remove('btn-warning')
    btn.classList.add('btn-primary')
    clearInterval(autoTimer)
    autoTimer = null
  }
}

async function fillBuffer() {
  if (autoRunning) toggleAuto()
  while (state.buffer < capacity) {
    if (!fire('start_produce')) break
    await new Promise(r => setTimeout(r, 150))
    if (!fire('finish_produce')) break
    await new Promise(r => setTimeout(r, 150))
  }
}

async function drainBuffer() {
  if (autoRunning) toggleAuto()
  while (state.buffer > 0) {
    if (!fire('start_consume')) break
    await new Promise(r => setTimeout(r, 150))
    if (!fire('finish_consume')) break
    await new Promise(r => setTimeout(r, 150))
  }
}

function resetState() {
  if (autoRunning) toggleAuto()
  initState()
  updateUI()
  updateCharts()
  updatePredictions()
}

function updateCapacity(newCapacity) {
  if (autoRunning) toggleAuto()
  capacity = newCapacity
  initState()
  updateUI()
  updateCharts()
  updatePredictions()
  document.getElementById('capacity-label').textContent = capacity
  document.getElementById('capacity-display').textContent = capacity
}

// Setup event listeners
document.getElementById('auto-btn').addEventListener('click', toggleAuto)
document.getElementById('reset-btn').addEventListener('click', resetState)
document.getElementById('fill-btn').addEventListener('click', fillBuffer)
document.getElementById('drain-btn').addEventListener('click', drainBuffer)

document.getElementById('speed').addEventListener('input', function() {
  document.getElementById('speed-value').textContent = this.value + 'ms'
  if (autoRunning) {
    clearInterval(autoTimer)
    autoTimer = setInterval(autoStep, parseInt(this.value))
  }
})

document.getElementById('capacity').addEventListener('input', function() {
  document.getElementById('capacity-value').textContent = this.value
  updateCapacity(parseInt(this.value))
})

// Module scripts are deferred, DOM is ready
initState()
initCharts()
updateUI()
updateCharts()
updatePredictions()
