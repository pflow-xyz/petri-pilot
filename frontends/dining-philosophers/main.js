// Dining Philosophers - Interactive Petri Net Simulation

const NUM = 5
const NAMES = ['Aristotle', 'Bentham', 'Confucius', 'Descartes', 'Epicurus']
const COLORS = { thinking: '#3498db', has_left: '#f39c12', eating: '#27ae60', idle: '#95a5a6' }

// State: tokens in each place
let state = {}
let autoRunning = false
let autoTimer = null

function initState() {
  state = {}
  for (let i = 0; i < NUM; i++) {
    state[`thinking_${i}`] = 1
    state[`fork_${i}`] = 1
    state[`has_left_${i}`] = 0
    state[`eating_${i}`] = 0
  }
}

// Transition definitions: { inputs: {place: weight}, outputs: {place: weight} }
function getTransitions() {
  const transitions = {}
  for (let i = 0; i < NUM; i++) {
    const leftFork = `fork_${(i + NUM - 1) % NUM}`
    const rightFork = `fork_${i}`

    transitions[`pickup_left_${i}`] = {
      label: `${NAMES[i]}: pick up left`,
      philosopher: i,
      inputs: { [`thinking_${i}`]: 1, [leftFork]: 1 },
      outputs: { [`has_left_${i}`]: 1 }
    }
    transitions[`pickup_right_${i}`] = {
      label: `${NAMES[i]}: pick up right`,
      philosopher: i,
      inputs: { [`has_left_${i}`]: 1, [rightFork]: 1 },
      outputs: { [`eating_${i}`]: 1 }
    }
    transitions[`release_${i}`] = {
      label: `${NAMES[i]}: release & think`,
      philosopher: i,
      inputs: { [`eating_${i}`]: 1 },
      outputs: { [`thinking_${i}`]: 1, [leftFork]: 1, [rightFork]: 1 }
    }
  }
  return transitions
}

function isEnabled(transition) {
  for (const [place, weight] of Object.entries(transition.inputs)) {
    if ((state[place] || 0) < weight) return false
  }
  return true
}

function fire(transitionId) {
  const transitions = getTransitions()
  const t = transitions[transitionId]
  if (!t || !isEnabled(t)) return false

  for (const [place, weight] of Object.entries(t.inputs)) {
    state[place] = (state[place] || 0) - weight
  }
  for (const [place, weight] of Object.entries(t.outputs)) {
    state[place] = (state[place] || 0) + weight
  }

  addLog(t.label, transitionId)
  updateUI()
  return true
}

function getPhilosopherState(i) {
  if (state[`eating_${i}`] > 0) return 'eating'
  if (state[`has_left_${i}`] > 0) return 'has_left'
  if (state[`thinking_${i}`] > 0) return 'thinking'
  return 'idle'
}

function isDeadlocked() {
  for (let i = 0; i < NUM; i++) {
    if (state[`has_left_${i}`] !== 1) return false
  }
  return true
}

function getEnabledTransitions() {
  const transitions = getTransitions()
  const enabled = []
  for (const [id, t] of Object.entries(transitions)) {
    if (isEnabled(t)) enabled.push(id)
  }
  return enabled
}

// SVG rendering
function renderTable() {
  const svg = document.getElementById('table-svg')
  const cx = 250, cy = 250, r = 170, forkR = 135

  // Clear dynamic elements
  svg.querySelectorAll('.dynamic').forEach(el => el.remove())

  for (let i = 0; i < NUM; i++) {
    const angle = (i * 2 * Math.PI / NUM) - Math.PI / 2
    const px = cx + r * Math.cos(angle)
    const py = cy + r * Math.sin(angle)

    const pState = getPhilosopherState(i)
    const color = COLORS[pState]

    // Philosopher circle
    const circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle')
    circle.setAttribute('cx', px)
    circle.setAttribute('cy', py)
    circle.setAttribute('r', 30)
    circle.setAttribute('fill', color)
    circle.setAttribute('stroke', 'white')
    circle.setAttribute('stroke-width', '2')
    circle.setAttribute('class', 'dynamic philosopher')
    if (pState === 'eating') {
      circle.setAttribute('filter', 'url(#glow)')
    }
    svg.appendChild(circle)

    // Philosopher label
    const text = document.createElementNS('http://www.w3.org/2000/svg', 'text')
    text.setAttribute('x', px)
    text.setAttribute('y', py - 4)
    text.setAttribute('text-anchor', 'middle')
    text.setAttribute('fill', 'white')
    text.setAttribute('font-size', '10')
    text.setAttribute('font-weight', 'bold')
    text.setAttribute('class', 'dynamic')
    text.textContent = NAMES[i].charAt(0)
    svg.appendChild(text)

    // State emoji
    const emoji = document.createElementNS('http://www.w3.org/2000/svg', 'text')
    emoji.setAttribute('x', px)
    emoji.setAttribute('y', py + 14)
    emoji.setAttribute('text-anchor', 'middle')
    emoji.setAttribute('font-size', '14')
    emoji.setAttribute('class', 'dynamic')
    emoji.textContent = pState === 'eating' ? '🍝' : pState === 'has_left' ? '🍴' : '💭'
    svg.appendChild(emoji)

    // Fork between philosopher i and i+1
    const forkAngle = ((i + 0.5) * 2 * Math.PI / NUM) - Math.PI / 2
    const fx = cx + forkR * Math.cos(forkAngle)
    const fy = cy + forkR * Math.sin(forkAngle)
    const forkAvailable = state[`fork_${i}`] > 0

    const fork = document.createElementNS('http://www.w3.org/2000/svg', 'text')
    fork.setAttribute('x', fx)
    fork.setAttribute('y', fy + 5)
    fork.setAttribute('text-anchor', 'middle')
    fork.setAttribute('font-size', forkAvailable ? '24' : '16')
    fork.setAttribute('opacity', forkAvailable ? '1' : '0.3')
    fork.setAttribute('class', 'dynamic')
    fork.textContent = '🍴'
    svg.appendChild(fork)
  }

  // Add glow filter if not present
  if (!svg.querySelector('#glow')) {
    const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs')
    defs.innerHTML = `
      <filter id="glow">
        <feGaussianBlur stdDeviation="3" result="coloredBlur"/>
        <feMerge>
          <feMergeNode in="coloredBlur"/>
          <feMergeNode in="SourceGraphic"/>
        </feMerge>
      </filter>`
    defs.setAttribute('class', 'dynamic')
    svg.appendChild(defs)
  }
}

function renderStateGrid() {
  const grid = document.getElementById('state-grid')
  let html = ''
  for (let i = 0; i < NUM; i++) {
    const pState = getPhilosopherState(i)
    const color = COLORS[pState]
    const leftFork = `fork_${(i + NUM - 1) % NUM}`
    const rightFork = `fork_${i}`
    const hasLeft = !state[leftFork] || state[`has_left_${i}`] || state[`eating_${i}`]
    const hasRight = !state[rightFork] || state[`eating_${i}`]

    html += `
      <div class="state-card" style="border-left: 4px solid ${color};">
        <div class="state-name">${NAMES[i]}</div>
        <div class="state-status" style="color: ${color};">${pState.replace('_', ' ')}</div>
        <div class="state-forks">
          <span class="${hasLeft ? 'fork-held' : 'fork-free'}">L${hasLeft ? '✓' : '○'}</span>
          <span class="${hasRight ? 'fork-held' : 'fork-free'}">R${hasRight ? '✓' : '○'}</span>
        </div>
      </div>`
  }
  grid.innerHTML = html
}

function renderTransitions() {
  const grid = document.getElementById('transitions-grid')
  const transitions = getTransitions()
  const enabled = getEnabledTransitions()
  let html = ''

  for (let i = 0; i < NUM; i++) {
    const ids = [`pickup_left_${i}`, `pickup_right_${i}`, `release_${i}`]
    html += `<div class="philosopher-transitions">`
    html += `<div class="pt-name">${NAMES[i]}</div>`
    html += `<div class="pt-buttons">`
    for (const id of ids) {
      const t = transitions[id]
      const isEn = enabled.includes(id)
      const shortLabel = id.startsWith('pickup_left') ? '⬅ Left' :
                         id.startsWith('pickup_right') ? '➡ Right' : '🔄 Release'
      html += `<button class="btn-transition ${isEn ? 'enabled' : 'disabled'}"
                onclick="window.fire('${id}')" ${isEn ? '' : 'disabled'}>
                ${shortLabel}
              </button>`
    }
    html += `</div></div>`
  }
  grid.innerHTML = html
}

function updateUI() {
  renderTable()
  renderStateGrid()
  renderTransitions()

  const deadlocked = isDeadlocked()
  const alert = document.getElementById('deadlock-alert')
  alert.classList.toggle('hidden', !deadlocked)

  if (deadlocked && autoRunning) {
    toggleAuto()
    addLog('DEADLOCK! All philosophers stuck.', 'deadlock')
  }
}

// Event log
function addLog(message, type) {
  const log = document.getElementById('event-log')
  const entry = document.createElement('div')
  entry.className = `log-entry ${type.includes('deadlock') ? 'danger' : 'action'}`
  const time = new Date().toLocaleTimeString()
  entry.textContent = `[${time}] ${message}`
  log.insertBefore(entry, log.firstChild)

  // Keep max 50 entries
  while (log.children.length > 50) {
    log.removeChild(log.lastChild)
  }
}

window.clearLog = function() {
  document.getElementById('event-log').innerHTML =
    '<div class="log-entry info">Log cleared.</div>'
}

// Auto-run: randomly fire an enabled transition
function autoStep() {
  const enabled = getEnabledTransitions()
  if (enabled.length === 0) {
    if (autoRunning) toggleAuto()
    return
  }
  const pick = enabled[Math.floor(Math.random() * enabled.length)]
  fire(pick)
}

window.toggleAuto = function() {
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

window.forceDeadlock = function() {
  if (autoRunning) toggleAuto()

  // Reset to clean state
  initState()

  // Fire all pickup_left transitions in order
  for (let i = 0; i < NUM; i++) {
    fire(`pickup_left_${i}`)
  }

  addLog('Forced deadlock: all philosophers hold left fork', 'deadlock')
}

window.resetState = function() {
  if (autoRunning) toggleAuto()
  initState()
  updateUI()
  addLog('State reset. All philosophers thinking.', 'info')
}

window.fire = function(transitionId) {
  fire(transitionId)
}

// Speed slider
document.addEventListener('DOMContentLoaded', () => {
  const speedSlider = document.getElementById('speed')
  const speedValue = document.getElementById('speed-value')
  speedSlider.addEventListener('input', () => {
    speedValue.textContent = speedSlider.value + 'ms'
    if (autoRunning) {
      clearInterval(autoTimer)
      autoTimer = setInterval(autoStep, parseInt(speedSlider.value))
    }
  })

  initState()
  updateUI()
})
