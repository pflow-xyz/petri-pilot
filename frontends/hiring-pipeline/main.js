// Hiring Pipeline - Multi-stage workflow with roles and fork-join

let currentRole = 'recruiter'
let state = {}

const ROLE_DESCS = {
  recruiter: 'Screens candidates and manages initial pipeline',
  engineer: 'Conducts technical interviews and assessments',
  manager: 'Conducts culture interviews and extends offers',
  candidate: 'Accepts or declines job offers'
}

const ROLE_COLORS = {
  recruiter: '#3498db',
  engineer: '#e67e22',
  manager: '#8e44ad',
  candidate: '#27ae60'
}

function initState() {
  state = {
    applied: 1,
    phone_screen: 0,
    technical_interview: 0,
    culture_interview: 0,
    tech_passed: 0,
    culture_passed: 0,
    ready_for_offer: 0,
    offer_extended: 0,
    hired: 0,
    rejected: 0
  }
}

function getTransitions() {
  return {
    screen_candidate: {
      label: 'Screen Candidate',
      desc: 'Conduct initial phone screen',
      role: 'recruiter',
      inputs: { applied: 1 },
      outputs: { phone_screen: 1 }
    },
    pass_screen: {
      label: 'Pass Screen',
      desc: 'Advance to parallel interviews',
      role: 'recruiter',
      inputs: { phone_screen: 1 },
      outputs: { technical_interview: 1, culture_interview: 1 }
    },
    fail_screen: {
      label: 'Fail Screen',
      desc: 'Reject after phone screen',
      role: 'recruiter',
      inputs: { phone_screen: 1 },
      outputs: { rejected: 1 }
    },
    pass_technical: {
      label: 'Pass Technical',
      desc: 'Candidate passes technical assessment',
      role: 'engineer',
      inputs: { technical_interview: 1 },
      outputs: { tech_passed: 1 }
    },
    fail_technical: {
      label: 'Fail Technical',
      desc: 'Reject after technical interview',
      role: 'engineer',
      inputs: { technical_interview: 1 },
      outputs: { rejected: 1 }
    },
    pass_culture: {
      label: 'Pass Culture',
      desc: 'Candidate passes culture interview',
      role: 'manager',
      inputs: { culture_interview: 1 },
      outputs: { culture_passed: 1 }
    },
    fail_culture: {
      label: 'Fail Culture',
      desc: 'Reject after culture interview',
      role: 'manager',
      inputs: { culture_interview: 1 },
      outputs: { rejected: 1 }
    },
    merge_interviews: {
      label: 'Merge Results',
      desc: 'Both interviews passed - synchronization join',
      role: null,
      inputs: { tech_passed: 1, culture_passed: 1 },
      outputs: { ready_for_offer: 1 }
    },
    extend_offer: {
      label: 'Extend Offer',
      desc: 'Send formal job offer to candidate',
      role: 'manager',
      inputs: { ready_for_offer: 1 },
      outputs: { offer_extended: 1 }
    },
    accept_offer: {
      label: 'Accept Offer',
      desc: 'Candidate accepts the position',
      role: 'candidate',
      inputs: { offer_extended: 1 },
      outputs: { hired: 1 }
    },
    reject_offer: {
      label: 'Decline Offer',
      desc: 'Candidate declines the position',
      role: 'candidate',
      inputs: { offer_extended: 1 },
      outputs: { rejected: 1 }
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

  const role = t.role ? ` [${t.role}]` : ' [auto]'
  addLog(`${t.label}${role}`, t.role || 'system')
  updateUI()

  // Auto-fire merge if both interviews passed
  const merge = transitions.merge_interviews
  if (isEnabled(merge)) {
    setTimeout(() => fire('merge_interviews'), 300)
  }

  return true
}

function updateKanban() {
  // Map places to kanban columns
  const columns = {
    applied: ['applied'],
    screening: ['phone_screen'],
    interviews: ['technical_interview', 'culture_interview', 'tech_passed', 'culture_passed'],
    offer: ['ready_for_offer', 'offer_extended'],
    result: ['hired', 'rejected']
  }

  const labels = {
    applied: 'Candidate',
    phone_screen: 'In Phone Screen',
    technical_interview: 'Technical Interview',
    culture_interview: 'Culture Interview',
    tech_passed: 'Tech Passed',
    culture_passed: 'Culture Passed',
    ready_for_offer: 'Ready for Offer',
    offer_extended: 'Offer Extended',
    hired: 'HIRED',
    rejected: 'REJECTED'
  }

  const cardColors = {
    applied: '#3498db',
    phone_screen: '#3498db',
    technical_interview: '#e67e22',
    culture_interview: '#8e44ad',
    tech_passed: '#27ae60',
    culture_passed: '#27ae60',
    ready_for_offer: '#f39c12',
    offer_extended: '#f39c12',
    hired: '#27ae60',
    rejected: '#e74c3c'
  }

  for (const [colId, places] of Object.entries(columns)) {
    const container = document.getElementById(`cards-${colId}`)
    let html = ''
    for (const place of places) {
      if (state[place] > 0) {
        const color = cardColors[place]
        const isResult = place === 'hired' || place === 'rejected'
        html += `<div class="kanban-card${isResult ? ' result' : ''}" style="border-left-color: ${color};">
          <span class="card-label">${labels[place]}</span>
        </div>`
      }
    }
    container.innerHTML = html
  }

  // Highlight active column
  document.querySelectorAll('.kanban-column').forEach(col => col.classList.remove('active'))
  for (const [colId, places] of Object.entries(columns)) {
    const hasTokens = places.some(p => state[p] > 0)
    if (hasTokens) {
      document.getElementById(`col-${colId}`).classList.add('active')
    }
  }
}

function renderActions() {
  const container = document.getElementById('actions-container')
  const transitions = getTransitions()
  let html = ''

  for (const [id, t] of Object.entries(transitions)) {
    if (t.role === null) continue
    if (t.role !== currentRole) continue

    const enabled = isEnabled(t)
    const color = ROLE_COLORS[t.role]

    html += `
      <div class="action-card ${enabled ? 'enabled' : 'disabled'}">
        <div class="action-header">
          <span class="action-name">${t.label}</span>
          <span class="action-role" style="color: ${color};">${t.role}</span>
        </div>
        <div class="action-desc">${t.desc}</div>
        <button class="btn btn-action" onclick="window.fire('${id}')"
          ${enabled ? '' : 'disabled'}
          style="${enabled ? `background: ${color};` : ''}">
          ${enabled ? 'Execute' : 'Not Available'}
        </button>
      </div>`
  }

  if (!html) {
    html = '<div class="no-actions">No actions available for this role in the current state.</div>'
  }
  container.innerHTML = html
}

function updateUI() {
  updateKanban()
  renderActions()
}

window.selectRole = function(role) {
  currentRole = role
  document.querySelectorAll('.role-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.role === role)
  })
  document.getElementById('role-desc').textContent = ROLE_DESCS[role]
  renderActions()
}

window.fire = function(id) { fire(id) }

window.resetState = function() {
  initState()
  updateUI()
  addLog('Pipeline reset. New candidate applied.', 'system')
}

function addLog(message, role) {
  const log = document.getElementById('event-log')
  const entry = document.createElement('div')
  const color = ROLE_COLORS[role] || '#666'
  entry.className = 'log-entry'
  entry.innerHTML = `<span class="log-dot" style="background:${color};"></span> ${message}`
  log.insertBefore(entry, log.firstChild)
  while (log.children.length > 30) log.removeChild(log.lastChild)
}

window.clearLog = function() {
  document.getElementById('event-log').innerHTML =
    '<div class="log-entry info">Log cleared.</div>'
}

document.addEventListener('DOMContentLoaded', () => {
  initState()
  updateUI()
})
