// Simulation Control Panel for vet-clinic
// Allows running simulated patient visits through the workflow

const API_BASE = ''

// Simulation state
let pollInterval = null

// Fetch simulation status
async function fetchStatus() {
  try {
    const response = await fetch(`${API_BASE}/api/simulation/status`)
    if (!response.ok) throw new Error('Failed to fetch status')
    return await response.json()
  } catch (error) {
    console.error('Status fetch error:', error)
    return null
  }
}

// Start simulation
async function startSimulation(config) {
  try {
    const response = await fetch(`${API_BASE}/api/simulation/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
    if (!response.ok) {
      const err = await response.json()
      throw new Error(err.message || 'Failed to start simulation')
    }
    return await response.json()
  } catch (error) {
    console.error('Start simulation error:', error)
    throw error
  }
}

// Stop simulation
async function stopSimulation() {
  try {
    const response = await fetch(`${API_BASE}/api/simulation/stop`, {
      method: 'POST'
    })
    if (!response.ok) {
      const err = await response.json()
      throw new Error(err.message || 'Failed to stop simulation')
    }
    return await response.json()
  } catch (error) {
    console.error('Stop simulation error:', error)
    throw error
  }
}

// Update the status display
function updateStatusDisplay(status) {
  const statusEl = document.getElementById('sim-status')
  if (!statusEl) return

  if (!status) {
    statusEl.innerHTML = '<p class="error">Unable to fetch status</p>'
    return
  }

  const progressPct = status.total_planned > 0
    ? ((status.completed + status.failed) / status.total_planned * 100).toFixed(1)
    : 0

  statusEl.innerHTML = `
    <div class="status-grid">
      <div class="status-item">
        <span class="status-label">Status</span>
        <span class="status-value ${status.running ? 'running' : 'stopped'}">
          ${status.running ? 'Running' : 'Stopped'}
        </span>
      </div>
      <div class="status-item">
        <span class="status-label">Progress</span>
        <span class="status-value">${status.completed + status.failed} / ${status.total_planned}</span>
      </div>
      <div class="status-item">
        <span class="status-label">Completed</span>
        <span class="status-value success">${status.completed}</span>
      </div>
      <div class="status-item">
        <span class="status-label">Failed</span>
        <span class="status-value ${status.failed > 0 ? 'error' : ''}">${status.failed}</span>
      </div>
      ${status.current_patient ? `
        <div class="status-item full-width">
          <span class="status-label">Current Patient</span>
          <span class="status-value">${status.current_patient}</span>
        </div>
      ` : ''}
      <div class="status-item full-width">
        <span class="status-label">Message</span>
        <span class="status-value message">${status.message || '-'}</span>
      </div>
    </div>

    <div class="progress-bar">
      <div class="progress-fill" style="width: ${progressPct}%"></div>
    </div>
    <div class="progress-label">${progressPct}% complete</div>
  `

  // Update button states
  const startBtn = document.getElementById('sim-start-btn')
  const stopBtn = document.getElementById('sim-stop-btn')
  if (startBtn) startBtn.disabled = status.running
  if (stopBtn) stopBtn.disabled = !status.running
}

// Start polling for status
function startPolling() {
  if (pollInterval) return
  pollInterval = setInterval(async () => {
    const status = await fetchStatus()
    updateStatusDisplay(status)

    // Stop polling if simulation is not running
    if (status && !status.running) {
      stopPolling()
    }
  }, 500)
}

// Stop polling
function stopPolling() {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = null
  }
}

// Handle start button click
async function handleStart() {
  const visitsInput = document.getElementById('sim-visits')
  const delayInput = document.getElementById('sim-delay')

  const config = {
    visits_to_generate: parseInt(visitsInput?.value || '10'),
    delay_ms: parseInt(delayInput?.value || '500'),
    auto_complete: true
  }

  try {
    await startSimulation(config)
    startPolling()
  } catch (error) {
    alert('Failed to start simulation: ' + error.message)
  }
}

// Handle stop button click
async function handleStop() {
  try {
    await stopSimulation()
    // Keep polling to see final status
  } catch (error) {
    alert('Failed to stop simulation: ' + error.message)
  }
}

// Render the simulation control panel
export async function renderSimulation(container) {
  const status = await fetchStatus()

  container.innerHTML = `
    <div class="simulation">
      <h1>Simulation Mode</h1>
      <p class="subtitle">Generate and process simulated patient visits to test the workflow</p>

      <div class="sim-controls">
        <div class="control-group">
          <label for="sim-visits">Number of Visits</label>
          <input type="number" id="sim-visits" min="1" max="100" value="10">
        </div>
        <div class="control-group">
          <label for="sim-delay">Delay (ms)</label>
          <input type="number" id="sim-delay" min="100" max="5000" value="500" step="100">
        </div>
        <div class="control-buttons">
          <button id="sim-start-btn" class="btn btn-primary" ${status?.running ? 'disabled' : ''}>
            Start Simulation
          </button>
          <button id="sim-stop-btn" class="btn btn-danger" ${!status?.running ? 'disabled' : ''}>
            Stop
          </button>
        </div>
      </div>

      <div class="sim-status-panel">
        <h2>Status</h2>
        <div id="sim-status">
          <p class="loading">Loading status...</p>
        </div>
      </div>

      <div class="sim-info">
        <h2>About Simulation</h2>
        <p>The simulation generates realistic patient visits with:</p>
        <ul>
          <li><strong>Weighted appointment types:</strong> Wellness (40%), Sick (30%), Dental (10%), Surgery (10%), Vaccination (10%)</li>
          <li><strong>Appropriate provider assignment:</strong> DVMs for most visits, RVTs for vaccinations</li>
          <li><strong>Realistic financial mix:</strong> Services percentage varies by appointment type</li>
          <li><strong>Full workflow execution:</strong> Schedule &rarr; Check-in &rarr; Exam &rarr; Complete &rarr; Checkout</li>
        </ul>
        <p>After running simulations, visit the <a href="/dashboard" onclick="event.preventDefault(); window.handleNavClick && handleNavClick(event, '/dashboard')">Dashboard</a> to see aggregated analytics.</p>
      </div>
    </div>
  `

  // Attach event handlers
  document.getElementById('sim-start-btn')?.addEventListener('click', handleStart)
  document.getElementById('sim-stop-btn')?.addEventListener('click', handleStop)

  // Initial status update
  updateStatusDisplay(status)

  // Start polling if simulation is running
  if (status?.running) {
    startPolling()
  }
}

// For backwards compatibility with generated router
export function renderSimulationPage() {
  return `<div id="simulation-container"></div>`
}

export function initSimulation() {
  const container = document.getElementById('simulation-container')
  if (container) {
    renderSimulation(container)
  }
}

// Simulation styles
export function getSimulationStyles() {
  return `
    .simulation {
      padding: 20px;
      max-width: 800px;
      margin: 0 auto;
    }

    .simulation h1 {
      margin-bottom: 8px;
      color: #1f2937;
    }

    .simulation .subtitle {
      color: #6b7280;
      margin-bottom: 24px;
    }

    .sim-controls {
      background: white;
      border: 1px solid #e5e7eb;
      border-radius: 8px;
      padding: 20px;
      display: flex;
      gap: 20px;
      align-items: flex-end;
      flex-wrap: wrap;
      margin-bottom: 24px;
    }

    .control-group {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    .control-group label {
      font-size: 14px;
      font-weight: 500;
      color: #374151;
    }

    .control-group input {
      padding: 8px 12px;
      border: 1px solid #d1d5db;
      border-radius: 6px;
      font-size: 14px;
      width: 120px;
    }

    .control-group input:focus {
      outline: none;
      border-color: #3b82f6;
      box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
    }

    .control-buttons {
      display: flex;
      gap: 12px;
      margin-left: auto;
    }

    .btn {
      padding: 10px 20px;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      border: none;
      transition: all 0.2s;
    }

    .btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .btn-primary {
      background: #3b82f6;
      color: white;
    }

    .btn-primary:hover:not(:disabled) {
      background: #2563eb;
    }

    .btn-danger {
      background: #ef4444;
      color: white;
    }

    .btn-danger:hover:not(:disabled) {
      background: #dc2626;
    }

    .sim-status-panel {
      background: white;
      border: 1px solid #e5e7eb;
      border-radius: 8px;
      padding: 20px;
      margin-bottom: 24px;
    }

    .sim-status-panel h2 {
      margin: 0 0 16px 0;
      font-size: 16px;
      color: #374151;
    }

    .status-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 16px;
      margin-bottom: 16px;
    }

    .status-item {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .status-item.full-width {
      grid-column: 1 / -1;
    }

    .status-label {
      font-size: 12px;
      font-weight: 500;
      color: #6b7280;
      text-transform: uppercase;
    }

    .status-value {
      font-size: 18px;
      font-weight: 600;
      color: #1f2937;
    }

    .status-value.running {
      color: #3b82f6;
    }

    .status-value.stopped {
      color: #6b7280;
    }

    .status-value.success {
      color: #22c55e;
    }

    .status-value.error {
      color: #ef4444;
    }

    .status-value.message {
      font-size: 14px;
      font-weight: 400;
    }

    .progress-bar {
      height: 8px;
      background: #e5e7eb;
      border-radius: 4px;
      overflow: hidden;
    }

    .progress-fill {
      height: 100%;
      background: linear-gradient(90deg, #3b82f6, #22c55e);
      transition: width 0.3s ease;
    }

    .progress-label {
      text-align: center;
      font-size: 12px;
      color: #6b7280;
      margin-top: 8px;
    }

    .sim-info {
      background: #f9fafb;
      border: 1px solid #e5e7eb;
      border-radius: 8px;
      padding: 20px;
    }

    .sim-info h2 {
      margin: 0 0 12px 0;
      font-size: 16px;
      color: #374151;
    }

    .sim-info p {
      color: #4b5563;
      margin: 0 0 12px 0;
    }

    .sim-info ul {
      margin: 0 0 12px 0;
      padding-left: 20px;
    }

    .sim-info li {
      color: #4b5563;
      margin-bottom: 8px;
    }

    .sim-info a {
      color: #3b82f6;
      text-decoration: none;
    }

    .sim-info a:hover {
      text-decoration: underline;
    }

    .loading {
      color: #6b7280;
      font-style: italic;
    }

    .error {
      color: #ef4444;
    }
  `
}

// Cleanup when leaving page
export function cleanupSimulation() {
  stopPolling()
}

// Export for router integration
export const simulationRenderers = {
  'SimulationPage': renderSimulationPage,
}
