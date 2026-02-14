// Producer-Consumer - Bounded Buffer Simulation

let state = {}
let autoRunning = false
let autoTimer = null
let totalProduced = 0
let totalConsumed = 0

function initState() {
  state = {
    producer_idle: 1,
    producing: 0,
    buffer: 0,
    buffer_space: 5,
    consumer_idle: 1,
    consuming: 0,
    consumed: 0
  }
  totalProduced = 0
  totalConsumed = 0
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

  for (const [place, weight] of Object.entries(t.inputs)) {
    state[place] -= weight
  }
  for (const [place, weight] of Object.entries(t.outputs)) {
    state[place] = (state[place] || 0) + weight
  }

  if (transitionId === 'finish_produce') totalProduced++
  if (transitionId === 'finish_consume') totalConsumed++

  addLog(t.label, t.actor)
  updateUI()
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
  const slots = document.querySelectorAll('#buffer-slots .slot')
  const bufferLevel = state.buffer
  slots.forEach((slot, i) => {
    slot.classList.toggle('filled', i < bufferLevel)
  })

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
              onclick="window.fire('${id}')" ${isEn ? '' : 'disabled'}
              style="${isEn ? `border-color: ${color}; color: ${color};` : ''}">
              <span class="trans-label">${t.label}</span>
              <span class="trans-desc">${t.desc}</span>
            </button>`
  }
  container.innerHTML = html
}

function addLog(message, actor) {
  const log = document.getElementById('event-log')
  const entry = document.createElement('div')
  const color = actor === 'producer' ? '#3498db' : '#e67e22'
  entry.className = 'log-entry'
  entry.innerHTML = `<span class="log-dot" style="background:${color};"></span> ${message} <span style="color:${color};font-size:0.8rem;">[${actor}]</span>`
  log.insertBefore(entry, log.firstChild)
  while (log.children.length > 40) log.removeChild(log.lastChild)
}

window.fire = function(id) { fire(id) }

function autoStep() {
  const enabled = getEnabledTransitions()
  if (enabled.length === 0) return
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

window.fillBuffer = async function() {
  if (autoRunning) toggleAuto()
  while (state.buffer < 5) {
    if (!fire('start_produce')) break
    await new Promise(r => setTimeout(r, 200))
    if (!fire('finish_produce')) break
    await new Promise(r => setTimeout(r, 200))
  }
}

window.drainBuffer = async function() {
  if (autoRunning) toggleAuto()
  while (state.buffer > 0) {
    if (!fire('start_consume')) break
    await new Promise(r => setTimeout(r, 200))
    if (!fire('finish_consume')) break
    await new Promise(r => setTimeout(r, 200))
  }
}

window.resetState = function() {
  if (autoRunning) toggleAuto()
  initState()
  updateUI()
  addLog('System reset. Buffer empty.', 'system')
}

window.clearLog = function() {
  document.getElementById('event-log').innerHTML =
    '<div class="log-entry info">Log cleared.</div>'
}

document.getElementById('speed').addEventListener('input', function() {
  document.getElementById('speed-value').textContent = this.value + 'ms'
  if (autoRunning) {
    clearInterval(autoTimer)
    autoTimer = setInterval(autoStep, parseInt(this.value))
  }
})

document.addEventListener('DOMContentLoaded', () => {
  initState()
  updateUI()
})
