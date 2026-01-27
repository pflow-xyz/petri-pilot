// Task Manager Frontend
// Kanban-style board with workflow visualization

function getApiBase() {
  return window.API_BASE || ''
}

// State
let state = {
  tasks: [],
  currentTask: null
}

// Status mapping
const STATUS_MAP = {
  pending: 'pending',
  in_progress: 'in_progress',
  review: 'review',
  completed: 'completed'
}

// API calls
async function fetchTasks() {
  try {
    const response = await fetch(`${getApiBase()}/api/taskmanager`)
    if (response.ok) {
      const data = await response.json()
      state.tasks = data.instances || []
    }
  } catch (err) {
    console.error('Failed to fetch tasks:', err)
  }
}

async function createTask(title, description) {
  try {
    const response = await fetch(`${getApiBase()}/api/taskmanager`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        data: { title, description }
      })
    })

    if (!response.ok) {
      throw new Error('Failed to create task')
    }

    return true
  } catch (err) {
    console.error('Failed to create task:', err)
    alert('Failed to create task: ' + err.message)
    return false
  }
}

async function executeTransition(transitionId, aggregateId) {
  try {
    const response = await fetch(`${getApiBase()}/api/${transitionId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: aggregateId,
        data: {}
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
function getTaskStatus(task) {
  const statuses = ['pending', 'in_progress', 'review', 'completed']
  if (task.state) {
    for (const status of statuses) {
      if (task.state[status] > 0) return status
    }
  }
  return 'pending'
}

function getTasksByStatus(status) {
  return state.tasks.filter(task => getTaskStatus(task) === status)
}

function truncateId(id) {
  if (!id) return ''
  return id.substring(0, 8)
}

// Rendering
function render() {
  renderStats()
  renderColumns()
}

function renderStats() {
  const total = state.tasks.length
  const inProgress = getTasksByStatus('in_progress').length
  const inReview = getTasksByStatus('review').length
  const completed = getTasksByStatus('completed').length

  document.getElementById('stat-total').textContent = total
  document.getElementById('stat-progress').textContent = inProgress
  document.getElementById('stat-review').textContent = inReview
  document.getElementById('stat-done').textContent = completed
}

function renderColumns() {
  const columns = ['pending', 'in_progress', 'review', 'completed']

  columns.forEach(status => {
    const tasks = getTasksByStatus(status)
    const container = document.getElementById(`tasks-${status}`)
    const countEl = document.getElementById(`count-${status}`)

    countEl.textContent = tasks.length

    if (tasks.length === 0) {
      container.innerHTML = '<div class="empty-column">No tasks</div>'
      return
    }

    container.innerHTML = tasks.map(task => {
      const title = task.state?.title || 'Untitled Task'
      const description = task.state?.description || ''

      // Determine available actions
      let actions = []
      if (status === 'pending') {
        actions.push({ label: 'Start', transition: 'start' })
      } else if (status === 'in_progress') {
        actions.push({ label: 'Submit', transition: 'submit' })
      } else if (status === 'review') {
        actions.push({ label: 'Approve', transition: 'approve' })
        actions.push({ label: 'Reject', transition: 'reject' })
      }

      return `
        <div class="task-card" data-id="${task.id}">
          <div class="task-title">${escapeHtml(title)}</div>
          <div class="task-meta">
            <span class="task-id">#${truncateId(task.id)}</span>
            <div class="task-actions">
              ${actions.map(a =>
                `<button class="task-action-btn" data-transition="${a.transition}" data-id="${task.id}">${a.label}</button>`
              ).join('')}
            </div>
          </div>
        </div>
      `
    }).join('')
  })
}

function escapeHtml(str) {
  if (!str) return ''
  return str.replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// Modal helpers
function showNewTaskModal() {
  document.getElementById('new-task-modal').classList.remove('hidden')
  document.getElementById('task-title').value = ''
  document.getElementById('task-description').value = ''
  document.getElementById('task-title').focus()
}

function hideNewTaskModal() {
  document.getElementById('new-task-modal').classList.add('hidden')
}

function showTaskDetail(task) {
  state.currentTask = task
  const status = getTaskStatus(task)

  document.getElementById('detail-title').textContent = task.state?.title || 'Untitled Task'
  document.getElementById('detail-description').textContent = task.state?.description || 'No description'

  const statusEl = document.getElementById('detail-status')
  statusEl.textContent = status.replace('_', ' ')
  statusEl.className = `task-detail-status status-${status}`

  // Build action buttons
  let actions = []
  if (status === 'pending') {
    actions.push({ label: 'Start Working', transition: 'start', class: 'btn-primary' })
  } else if (status === 'in_progress') {
    actions.push({ label: 'Submit for Review', transition: 'submit', class: 'btn-primary' })
  } else if (status === 'review') {
    actions.push({ label: 'Approve', transition: 'approve', class: 'btn-primary' })
    actions.push({ label: 'Reject', transition: 'reject', class: 'btn-secondary' })
  }

  const actionsEl = document.getElementById('detail-actions')
  actionsEl.innerHTML = actions.map(a =>
    `<button class="btn ${a.class}" data-transition="${a.transition}">${a.label}</button>`
  ).join('')

  document.getElementById('task-detail-modal').classList.remove('hidden')
}

function hideTaskDetail() {
  document.getElementById('task-detail-modal').classList.add('hidden')
  state.currentTask = null
}

// Event handlers
function setupEventHandlers() {
  // New task button
  document.getElementById('new-task-btn').addEventListener('click', showNewTaskModal)

  // Modal close buttons
  document.getElementById('modal-close').addEventListener('click', hideNewTaskModal)
  document.getElementById('modal-cancel').addEventListener('click', hideNewTaskModal)
  document.getElementById('detail-close').addEventListener('click', hideTaskDetail)

  // Close modals on overlay click
  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) {
        overlay.classList.add('hidden')
      }
    })
  })

  // New task form submit
  document.getElementById('task-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    const title = document.getElementById('task-title').value
    const description = document.getElementById('task-description').value

    if (await createTask(title, description)) {
      hideNewTaskModal()
      await fetchTasks()
      render()
    }
  })

  // Task card clicks (for detail view)
  document.querySelector('.kanban-board').addEventListener('click', async (e) => {
    // Handle action button clicks
    const actionBtn = e.target.closest('.task-action-btn')
    if (actionBtn) {
      e.stopPropagation()
      const transition = actionBtn.dataset.transition
      const id = actionBtn.dataset.id

      if (await executeTransition(transition, id)) {
        await fetchTasks()
        render()
      }
      return
    }

    // Handle card clicks
    const card = e.target.closest('.task-card')
    if (card) {
      const id = card.dataset.id
      const task = state.tasks.find(t => t.id === id)
      if (task) {
        showTaskDetail(task)
      }
    }
  })

  // Detail modal action clicks
  document.getElementById('detail-actions').addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-transition]')
    if (!btn || !state.currentTask) return

    const transition = btn.dataset.transition

    if (await executeTransition(transition, state.currentTask.id)) {
      hideTaskDetail()
      await fetchTasks()
      render()
    }
  })

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      hideNewTaskModal()
      hideTaskDetail()
    }
    if (e.key === 'n' && !e.target.closest('input, textarea')) {
      e.preventDefault()
      showNewTaskModal()
    }
  })
}

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  setupEventHandlers()
  await fetchTasks()
  render()

  // Auto-refresh every 5 seconds
  setInterval(async () => {
    await fetchTasks()
    render()
  }, 5000)
})
