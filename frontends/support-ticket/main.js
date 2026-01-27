// Support Ticket Frontend
// Professional ticketing system with priority and status management

function getApiBase() {
  return window.API_BASE || ''
}

// State
let state = {
  tickets: [],
  currentTicket: null,
  filter: 'all'
}

// Status configuration
const STATUSES = ['new', 'assigned', 'in_progress', 'escalated', 'pending_customer', 'resolved', 'closed']

const STATUS_LABELS = {
  new: 'New',
  assigned: 'Assigned',
  in_progress: 'In Progress',
  escalated: 'Escalated',
  pending_customer: 'Pending Customer',
  resolved: 'Resolved',
  closed: 'Closed'
}

const PRIORITY_LABELS = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  urgent: 'Urgent'
}

// Transitions available for each status
const STATUS_ACTIONS = {
  new: [
    { label: 'Assign', transition: 'assign', class: 'btn-primary' }
  ],
  assigned: [
    { label: 'Start Work', transition: 'start_work', class: 'btn-primary' }
  ],
  in_progress: [
    { label: 'Request Info', transition: 'request_info', class: 'btn-secondary' },
    { label: 'Escalate', transition: 'escalate', class: 'btn-warning' },
    { label: 'Resolve', transition: 'resolve', class: 'btn-success' }
  ],
  escalated: [
    { label: 'Resolve', transition: 'resolve_escalated', class: 'btn-success' }
  ],
  pending_customer: [
    { label: 'Customer Replied', transition: 'customer_reply', class: 'btn-primary' }
  ],
  resolved: [
    { label: 'Close', transition: 'close', class: 'btn-secondary' }
  ],
  closed: [
    { label: 'Reopen', transition: 'reopen', class: 'btn-warning' }
  ]
}

// API calls
async function fetchTickets() {
  try {
    const response = await fetch(`${getApiBase()}/api/supportticket`)
    if (response.ok) {
      const data = await response.json()
      state.tickets = data.instances || []
    }
  } catch (err) {
    console.error('Failed to fetch tickets:', err)
  }
}

async function createTicket(subject, description, priority, customerName, customerEmail) {
  try {
    const response = await fetch(`${getApiBase()}/api/supportticket`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        data: {
          subject,
          description,
          priority,
          customer_name: customerName,
          customer_email: customerEmail
        }
      })
    })

    if (!response.ok) {
      throw new Error('Failed to create ticket')
    }

    return true
  } catch (err) {
    console.error('Failed to create ticket:', err)
    alert('Failed to create ticket: ' + err.message)
    return false
  }
}

async function executeTransition(transitionId, aggregateId, data = {}) {
  try {
    const response = await fetch(`${getApiBase()}/api/${transitionId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: aggregateId,
        data: data
      })
    })

    if (!response.ok) {
      const error = await response.text()
      throw new Error(error)
    }

    return true
  } catch (err) {
    console.error('Transition failed:', err)
    alert('Action failed: ' + err.message)
    return false
  }
}

// Helpers
function getTicketStatus(ticket) {
  if (ticket.state) {
    for (const status of STATUSES) {
      if (ticket.state[status] > 0) return status
    }
  }
  return 'new'
}

function getTicketsByStatus(status) {
  return state.tickets.filter(ticket => getTicketStatus(ticket) === status)
}

function formatTicketId(id) {
  if (!id) return 'TKT-???'
  return 'TKT-' + id.substring(0, 6).toUpperCase()
}

function formatDate(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  })
}

function escapeHtml(str) {
  if (!str) return ''
  return str.replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// Rendering
function render() {
  renderStats()
  renderTicketsList()
}

function renderStats() {
  const newAndAssigned = getTicketsByStatus('new').length + getTicketsByStatus('assigned').length
  const inProgress = getTicketsByStatus('in_progress').length + getTicketsByStatus('pending_customer').length
  const escalated = getTicketsByStatus('escalated').length
  const resolved = getTicketsByStatus('resolved').length + getTicketsByStatus('closed').length

  document.getElementById('stat-open').textContent = newAndAssigned
  document.getElementById('stat-progress').textContent = inProgress
  document.getElementById('stat-escalated').textContent = escalated
  document.getElementById('stat-resolved').textContent = resolved
}

function renderTicketsList() {
  const container = document.getElementById('tickets-list')

  let filteredTickets = state.tickets
  if (state.filter !== 'all') {
    filteredTickets = getTicketsByStatus(state.filter)
  }

  if (filteredTickets.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <h3>No tickets${state.filter !== 'all' ? ` with status "${STATUS_LABELS[state.filter]}"` : ''}</h3>
        <p>Create a new ticket to get started</p>
      </div>
    `
    return
  }

  container.innerHTML = filteredTickets.map(ticket => {
    const status = getTicketStatus(ticket)
    const subject = ticket.state?.subject || 'No Subject'
    const description = ticket.state?.description || ''
    const priority = ticket.state?.priority || 'medium'
    const customerName = ticket.state?.customer_name || 'Unknown'
    const assignedTo = ticket.state?.assigned_to || '-'

    return `
      <div class="ticket-row" data-id="${ticket.id}">
        <span class="ticket-id">${formatTicketId(ticket.id)}</span>
        <div class="ticket-subject">
          ${escapeHtml(subject)}
          <small>${escapeHtml(description.substring(0, 60))}${description.length > 60 ? '...' : ''}</small>
        </div>
        <span class="priority-badge priority-${priority}">${PRIORITY_LABELS[priority]}</span>
        <span class="status-badge status-${status}">${STATUS_LABELS[status]}</span>
        <span class="ticket-date">${formatDate(ticket.created_at)}</span>
        <span>${escapeHtml(assignedTo)}</span>
      </div>
    `
  }).join('')
}

function renderTicketDetail() {
  const ticket = state.currentTicket
  if (!ticket) return

  const status = getTicketStatus(ticket)
  const subject = ticket.state?.subject || 'No Subject'
  const description = ticket.state?.description || 'No description provided.'
  const priority = ticket.state?.priority || 'medium'
  const customerName = ticket.state?.customer_name || 'Unknown'
  const customerEmail = ticket.state?.customer_email || '-'

  document.getElementById('detail-subject').textContent = subject
  document.getElementById('detail-id').textContent = formatTicketId(ticket.id)
  document.getElementById('detail-description').textContent = description
  document.getElementById('detail-customer').textContent = customerName
  document.getElementById('detail-email').textContent = customerEmail

  // Status badge
  const statusEl = document.getElementById('detail-status')
  statusEl.textContent = STATUS_LABELS[status]
  statusEl.className = `status-badge status-${status}`

  // Priority badge
  const priorityEl = document.getElementById('detail-priority')
  priorityEl.textContent = PRIORITY_LABELS[priority]
  priorityEl.className = `priority-badge priority-${priority}`

  // Actions
  const actions = STATUS_ACTIONS[status] || []
  const actionsEl = document.getElementById('detail-actions')
  actionsEl.innerHTML = actions.map(a =>
    `<button class="btn ${a.class}" data-transition="${a.transition}">${a.label}</button>`
  ).join('')

  if (actions.length === 0) {
    actionsEl.innerHTML = '<p style="color: #64748b; font-size: 0.875rem;">No actions available</p>'
  }
}

// Modal helpers
function showNewTicketModal() {
  document.getElementById('new-ticket-modal').classList.remove('hidden')
  document.getElementById('ticket-subject').value = ''
  document.getElementById('ticket-description').value = ''
  document.getElementById('ticket-priority').value = 'medium'
  document.getElementById('ticket-name').value = ''
  document.getElementById('ticket-email').value = ''
  document.getElementById('ticket-subject').focus()
}

function hideNewTicketModal() {
  document.getElementById('new-ticket-modal').classList.add('hidden')
}

function showTicketDetail(ticket) {
  state.currentTicket = ticket
  renderTicketDetail()
  document.getElementById('ticket-detail-modal').classList.remove('hidden')
}

function hideTicketDetail() {
  document.getElementById('ticket-detail-modal').classList.add('hidden')
  state.currentTicket = null
}

// Event handlers
function setupEventHandlers() {
  // New ticket button
  document.getElementById('new-ticket-btn').addEventListener('click', showNewTicketModal)

  // Modal close buttons
  document.getElementById('modal-close').addEventListener('click', hideNewTicketModal)
  document.getElementById('modal-cancel').addEventListener('click', hideNewTicketModal)
  document.getElementById('detail-close').addEventListener('click', hideTicketDetail)

  // Close modals on overlay click
  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) {
        overlay.classList.add('hidden')
      }
    })
  })

  // Filter buttons
  document.querySelectorAll('.filter-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'))
      btn.classList.add('active')
      state.filter = btn.dataset.filter
      render()
    })
  })

  // New ticket form
  document.getElementById('ticket-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    const subject = document.getElementById('ticket-subject').value
    const description = document.getElementById('ticket-description').value
    const priority = document.getElementById('ticket-priority').value
    const name = document.getElementById('ticket-name').value
    const email = document.getElementById('ticket-email').value

    if (await createTicket(subject, description, priority, name, email)) {
      hideNewTicketModal()
      await fetchTickets()
      render()
    }
  })

  // Ticket row clicks
  document.getElementById('tickets-list').addEventListener('click', (e) => {
    const row = e.target.closest('.ticket-row')
    if (row) {
      const id = row.dataset.id
      const ticket = state.tickets.find(t => t.id === id)
      if (ticket) {
        showTicketDetail(ticket)
      }
    }
  })

  // Detail action clicks
  document.getElementById('detail-actions').addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-transition]')
    if (!btn || !state.currentTicket) return

    const transition = btn.dataset.transition

    if (await executeTransition(transition, state.currentTicket.id)) {
      // Refresh ticket data
      await fetchTickets()
      const updatedTicket = state.tickets.find(t => t.id === state.currentTicket.id)
      if (updatedTicket) {
        state.currentTicket = updatedTicket
        renderTicketDetail()
      }
      render()
    }
  })

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      hideNewTicketModal()
      hideTicketDetail()
    }
  })
}

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  setupEventHandlers()
  await fetchTickets()
  render()

  // Auto-refresh every 10 seconds
  setInterval(async () => {
    await fetchTickets()
    render()
  }, 10000)
})
