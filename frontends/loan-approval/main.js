// Loan Approval Pipeline - Interactive Petri Net Simulation

let currentRole = 'processor'
let state = {}

const ROLE_DESCS = {
  processor: 'Initiates the loan review process',
  credit_analyst: 'Reviews applicant credit history and score',
  hr_analyst: 'Verifies employment and income',
  underwriter: 'Makes the final approval or rejection decision'
}

const ROLE_COLORS = {
  processor: '#3498db',
  credit_analyst: '#e67e22',
  hr_analyst: '#27ae60',
  underwriter: '#8e44ad'
}

function initState() {
  state = {
    submitted: 1,
    credit_review: 0,
    employment_review: 0,
    credit_passed: 0,
    credit_failed: 0,
    employment_passed: 0,
    employment_failed: 0,
    ready_for_decision: 0,
    under_review: 0,
    approved: 0,
    rejected: 0
  }
}

function getTransitions() {
  return {
    start_reviews: {
      label: 'Start Reviews',
      desc: 'Begin parallel credit and employment reviews',
      role: 'processor',
      inputs: { submitted: 1 },
      outputs: { credit_review: 1, employment_review: 1 }
    },
    approve_credit: {
      label: 'Approve Credit',
      desc: 'Credit check passed (score >= 680)',
      role: 'credit_analyst',
      inputs: { credit_review: 1 },
      outputs: { credit_passed: 1 }
    },
    reject_credit: {
      label: 'Reject Credit',
      desc: 'Credit check failed (score < 680)',
      role: 'credit_analyst',
      inputs: { credit_review: 1 },
      outputs: { credit_failed: 1 }
    },
    approve_employment: {
      label: 'Approve Employment',
      desc: 'Employment verified and income sufficient',
      role: 'hr_analyst',
      inputs: { employment_review: 1 },
      outputs: { employment_passed: 1 }
    },
    reject_employment: {
      label: 'Reject Employment',
      desc: 'Employment or income verification failed',
      role: 'hr_analyst',
      inputs: { employment_review: 1 },
      outputs: { employment_failed: 1 }
    },
    merge_reviews: {
      label: 'Merge Reviews',
      desc: 'Both reviews passed - synchronization join',
      role: null,
      inputs: { credit_passed: 1, employment_passed: 1 },
      outputs: { ready_for_decision: 1 }
    },
    begin_underwriting: {
      label: 'Begin Underwriting',
      desc: 'Start final review of the application',
      role: 'underwriter',
      inputs: { ready_for_decision: 1 },
      outputs: { under_review: 1 }
    },
    approve_loan: {
      label: 'Approve Loan',
      desc: 'Approve the loan application',
      role: 'underwriter',
      inputs: { under_review: 1 },
      outputs: { approved: 1 }
    },
    reject_loan: {
      label: 'Reject Loan',
      desc: 'Reject the loan application',
      role: 'underwriter',
      inputs: { under_review: 1 },
      outputs: { rejected: 1 }
    },
    reject_on_credit: {
      label: 'Reject (Credit Failure)',
      desc: 'Automatic rejection due to failed credit check',
      role: null,
      inputs: { credit_failed: 1 },
      outputs: { rejected: 1 }
    },
    reject_on_employment: {
      label: 'Reject (Employment Failure)',
      desc: 'Automatic rejection due to failed employment verification',
      role: null,
      inputs: { employment_failed: 1 },
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

  // Auto-fire merge_reviews if both passed
  const merge = transitions.merge_reviews
  if (isEnabled(merge)) {
    setTimeout(() => {
      fire('merge_reviews')
    }, 300)
  }

  // Auto-fire rejection transitions
  const autoRejectCredit = transitions.reject_on_credit
  if (isEnabled(autoRejectCredit)) {
    setTimeout(() => { fire('reject_on_credit') }, 300)
  }

  const autoRejectEmployment = transitions.reject_on_employment
  if (isEnabled(autoRejectEmployment)) {
    setTimeout(() => { fire('reject_on_employment') }, 300)
  }

  return true
}

function getActiveStages() {
  const active = []
  for (const [place, tokens] of Object.entries(state)) {
    if (tokens > 0) active.push(place)
  }
  return active
}

function updatePipeline() {
  const stages = [
    'submitted', 'credit_review', 'employment_review',
    'credit_passed', 'employment_passed',
    'ready_for_decision', 'under_review', 'approved'
  ]

  const activeStages = getActiveStages()

  stages.forEach(id => {
    const el = document.getElementById(`stage-${id}`)
    if (!el) return
    el.classList.remove('active', 'completed')

    if (activeStages.includes(id)) {
      el.classList.add('active')
    }
  })

  // Check for completed stages (tokens have passed through)
  const completedMap = {
    submitted: state.credit_review > 0 || state.credit_passed > 0 || state.credit_failed > 0 || state.ready_for_decision > 0 || state.under_review > 0 || state.approved > 0 || state.rejected > 0,
    credit_review: state.credit_passed > 0 || state.credit_failed > 0,
    employment_review: state.employment_passed > 0 || state.employment_failed > 0,
    credit_passed: state.ready_for_decision > 0 || state.under_review > 0 || state.approved > 0 || state.rejected > 0,
    employment_passed: state.ready_for_decision > 0 || state.under_review > 0 || state.approved > 0 || state.rejected > 0,
    ready_for_decision: state.under_review > 0 || state.approved > 0 || state.rejected > 0,
    under_review: state.approved > 0 || state.rejected > 0
  }

  for (const [id, completed] of Object.entries(completedMap)) {
    const el = document.getElementById(`stage-${id}`)
    if (el && completed && !activeStages.includes(id)) {
      el.classList.add('completed')
    }
  }

  // Show/hide rejection and approval badges
  document.getElementById('badge-credit_failed').classList.toggle('hidden', state.credit_failed === 0)
  document.getElementById('badge-employment_failed').classList.toggle('hidden', state.employment_failed === 0)
  document.getElementById('badge-rejected').classList.toggle('hidden', state.rejected === 0)
  document.getElementById('badge-approved').classList.toggle('hidden', state.approved === 0)
}

function renderActions() {
  const container = document.getElementById('actions-container')
  const transitions = getTransitions()
  let html = ''

  for (const [id, t] of Object.entries(transitions)) {
    // Skip auto-fire transitions and transitions for other roles
    if (t.role === null) continue
    if (t.role !== currentRole) continue

    const enabled = isEnabled(t)
    const color = ROLE_COLORS[t.role]

    html += `
      <div class="action-card ${enabled ? 'enabled' : 'disabled'}">
        <div class="action-header">
          <span class="action-name">${t.label}</span>
          <span class="action-role" style="color: ${color};">${t.role.replace('_', ' ')}</span>
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
  updatePipeline()
  renderActions()
}

// Role selector
window.selectRole = function(role) {
  currentRole = role
  document.querySelectorAll('.role-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.role === role)
  })
  document.getElementById('role-desc').textContent = ROLE_DESCS[role]
  renderActions()
}

window.fire = function(id) {
  fire(id)
}

window.resetState = function() {
  initState()
  updateUI()
  addLog('Pipeline reset. Application resubmitted.', 'system')
}

// Event log
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
