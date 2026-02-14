// TCP Handshake - Interactive Petri Net Simulation

let state = {}
let packetHistory = []

const CLIENT_STATES = ['client_closed', 'client_syn_sent', 'client_established', 'client_fin_wait']
const SERVER_STATES = ['server_listen', 'server_syn_received', 'server_established', 'server_closed']

const STATE_LABELS = {
  client_closed: 'CLOSED',
  client_syn_sent: 'SYN_SENT',
  client_established: 'ESTABLISHED',
  client_fin_wait: 'FIN_WAIT',
  server_listen: 'LISTEN',
  server_syn_received: 'SYN_RECEIVED',
  server_established: 'ESTABLISHED',
  server_closed: 'CLOSED'
}

const TRANSITION_INFO = {
  send_syn: { label: 'SYN', desc: 'Client sends SYN', direction: 'right', color: '#3498db', phase: 'handshake' },
  send_syn_ack: { label: 'SYN-ACK', desc: 'Server sends SYN-ACK', direction: 'left', color: '#e67e22', phase: 'handshake' },
  send_ack: { label: 'ACK', desc: 'Client sends ACK', direction: 'right', color: '#27ae60', phase: 'handshake' },
  send_fin: { label: 'FIN', desc: 'Client sends FIN', direction: 'right', color: '#e74c3c', phase: 'teardown' },
  send_fin_ack: { label: 'FIN-ACK', desc: 'Server sends FIN-ACK', direction: 'left', color: '#9b59b6', phase: 'teardown' },
  close: { label: 'CLOSE', desc: 'Connection closed', direction: 'both', color: '#7f8c8d', phase: 'teardown' }
}

function getTransitions() {
  return {
    send_syn: {
      inputs: { client_closed: 1, server_listen: 1 },
      outputs: { client_syn_sent: 1 }
    },
    send_syn_ack: {
      inputs: { client_syn_sent: 1 },
      outputs: { server_syn_received: 1 }
    },
    send_ack: {
      inputs: { server_syn_received: 1 },
      outputs: { client_established: 1, server_established: 1 }
    },
    send_fin: {
      inputs: { client_established: 1 },
      outputs: { client_fin_wait: 1 }
    },
    send_fin_ack: {
      inputs: { server_established: 1, client_fin_wait: 1 },
      outputs: { server_closed: 1 }
    },
    close: {
      inputs: { server_closed: 1 },
      outputs: { client_closed: 1, server_listen: 1 }
    }
  }
}

function initState() {
  state = {}
  const allPlaces = [...CLIENT_STATES, ...SERVER_STATES]
  allPlaces.forEach(p => state[p] = 0)
  state.client_closed = 1
  state.server_listen = 1
  packetHistory = []
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

  const info = TRANSITION_INFO[transitionId]
  packetHistory.push({ id: transitionId, ...info })
  addSequenceEntry(transitionId)
  updateUI()
  return true
}

function getActiveState(states) {
  for (const s of states) {
    if (state[s] > 0) return s
  }
  return null
}

function updateUI() {
  // Update endpoint state displays
  const clientState = getActiveState(CLIENT_STATES)
  const serverState = getActiveState(SERVER_STATES)

  const clientEl = document.getElementById('client-state')
  const serverEl = document.getElementById('server-state')

  clientEl.textContent = clientState ? STATE_LABELS[clientState] : '-'
  serverEl.textContent = serverState ? STATE_LABELS[serverState] : '-'

  clientEl.className = 'endpoint-state ' + (clientState || '')
  serverEl.className = 'endpoint-state ' + (serverState || '')

  // Update state machine displays
  renderStateMachine('client-states', CLIENT_STATES)
  renderStateMachine('server-states', SERVER_STATES)

  // Update transition buttons
  renderTransitions()

  // Update packet visualization
  renderPackets()
}

function renderStateMachine(containerId, states) {
  const container = document.getElementById(containerId)
  let html = ''
  for (const s of states) {
    const active = state[s] > 0
    const label = STATE_LABELS[s]
    html += `<div class="sm-state ${active ? 'active' : ''}">${label}</div>`
  }
  container.innerHTML = html
}

function renderTransitions() {
  const container = document.getElementById('transitions-container')
  const transitions = getTransitions()
  let html = ''

  for (const [id, t] of Object.entries(transitions)) {
    const info = TRANSITION_INFO[id]
    const enabled = isEnabled(t)
    html += `<button class="btn-transition ${enabled ? 'enabled' : 'disabled'} ${info.phase}"
              onclick="window.fire('${id}')" ${enabled ? '' : 'disabled'}
              style="${enabled ? `border-color: ${info.color}; color: ${info.color};` : ''}">
              <span class="trans-label">${info.label}</span>
              <span class="trans-desc">${info.desc}</span>
            </button>`
  }
  container.innerHTML = html
}

function renderPackets() {
  const svg = document.getElementById('packet-svg')
  // Clear old packets
  svg.querySelectorAll('.packet').forEach(el => el.remove())

  const maxShow = 6
  const recent = packetHistory.slice(-maxShow)
  const yStep = 100 / (maxShow + 1)

  recent.forEach((pkt, i) => {
    const y = 15 + (i + 1) * yStep
    const info = TRANSITION_INFO[pkt.id]

    if (info.direction === 'right') {
      // Client -> Server arrow
      const line = document.createElementNS('http://www.w3.org/2000/svg', 'line')
      line.setAttribute('x1', '30')
      line.setAttribute('y1', y)
      line.setAttribute('x2', '270')
      line.setAttribute('y2', y)
      line.setAttribute('stroke', info.color)
      line.setAttribute('stroke-width', '2')
      line.setAttribute('marker-end', 'url(#arrowhead)')
      line.setAttribute('class', 'packet')
      svg.appendChild(line)
    } else if (info.direction === 'left') {
      const line = document.createElementNS('http://www.w3.org/2000/svg', 'line')
      line.setAttribute('x1', '270')
      line.setAttribute('y1', y)
      line.setAttribute('x2', '30')
      line.setAttribute('y2', y)
      line.setAttribute('stroke', info.color)
      line.setAttribute('stroke-width', '2')
      line.setAttribute('marker-end', 'url(#arrowhead)')
      line.setAttribute('class', 'packet')
      svg.appendChild(line)
    } else {
      // both - two short lines
      const line1 = document.createElementNS('http://www.w3.org/2000/svg', 'line')
      line1.setAttribute('x1', '150')
      line1.setAttribute('y1', y)
      line1.setAttribute('x2', '30')
      line1.setAttribute('y2', y)
      line1.setAttribute('stroke', info.color)
      line1.setAttribute('stroke-width', '2')
      line1.setAttribute('marker-end', 'url(#arrowhead)')
      line1.setAttribute('class', 'packet')
      svg.appendChild(line1)

      const line2 = document.createElementNS('http://www.w3.org/2000/svg', 'line')
      line2.setAttribute('x1', '150')
      line2.setAttribute('y1', y)
      line2.setAttribute('x2', '270')
      line2.setAttribute('y2', y)
      line2.setAttribute('stroke', info.color)
      line2.setAttribute('stroke-width', '2')
      line2.setAttribute('marker-end', 'url(#arrowhead)')
      line2.setAttribute('class', 'packet')
      svg.appendChild(line2)
    }

    // Label
    const text = document.createElementNS('http://www.w3.org/2000/svg', 'text')
    text.setAttribute('x', '150')
    text.setAttribute('y', y - 5)
    text.setAttribute('text-anchor', 'middle')
    text.setAttribute('fill', info.color)
    text.setAttribute('font-size', '11')
    text.setAttribute('font-weight', 'bold')
    text.setAttribute('class', 'packet')
    text.textContent = info.label
    svg.appendChild(text)
  })

  // Add arrowhead marker if not present
  if (!svg.querySelector('#arrowhead')) {
    const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs')
    defs.innerHTML = `<marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
      <polygon points="0 0, 10 3.5, 0 7" fill="currentColor"/>
    </marker>`
    svg.insertBefore(defs, svg.firstChild)
  }
}

function addSequenceEntry(transitionId) {
  const log = document.getElementById('sequence-log')
  const info = TRANSITION_INFO[transitionId]
  const entry = document.createElement('div')
  entry.className = 'seq-entry'

  let arrow = ''
  if (info.direction === 'right') arrow = 'Client --->'
  else if (info.direction === 'left') arrow = '<--- Server'
  else arrow = '<-- CLOSE -->'

  entry.innerHTML = `<span class="seq-arrow" style="color: ${info.color};">${arrow}</span> <strong style="color: ${info.color};">${info.label}</strong> ${info.desc}`
  log.insertBefore(entry, log.firstChild)
  while (log.children.length > 20) log.removeChild(log.lastChild)
}

window.fire = function(id) { fire(id) }

window.autoHandshake = async function() {
  const steps = ['send_syn', 'send_syn_ack', 'send_ack']
  for (const step of steps) {
    if (!fire(step)) break
    await new Promise(r => setTimeout(r, 600))
  }
}

window.autoTeardown = async function() {
  const steps = ['send_fin', 'send_fin_ack', 'close']
  for (const step of steps) {
    if (!fire(step)) break
    await new Promise(r => setTimeout(r, 600))
  }
}

window.resetState = function() {
  initState()
  updateUI()
  document.getElementById('sequence-log').innerHTML =
    '<div class="seq-entry info">Connection reset. Ready for new handshake.</div>'
}

document.addEventListener('DOMContentLoaded', () => {
  initState()
  updateUI()
})
