// ZK Tic-Tac-Toe Integration
// Provides zero-knowledge proof functionality for game moves

// ZK API base - uses the zk-tic-tac-toe service
function getZkApiBase() {
  // In production, ZK endpoints are at /zk-tic-tac-toe/zk/
  // Detect if we're on the zk-tic-tac-toe service directly or need full path
  const path = window.location.pathname
  if (path.startsWith('/zk-tic-tac-toe')) {
    return '/zk-tic-tac-toe/zk'
  }
  // When accessed from /tic-tac-toe/, redirect to zk service
  return '/zk-tic-tac-toe/zk'
}

// ZK game state
export const zkState = {
  enabled: false,
  gameId: null,
  stateRoot: null,
  roots: [],
  lastProof: null,
  proofHistory: []
}

// Enable/disable ZK mode
export function setZkMode(enabled) {
  zkState.enabled = enabled
  console.log(`ZK mode: ${enabled ? 'enabled' : 'disabled'}`)

  // Update UI
  const zkToggle = document.getElementById('zk-toggle')
  if (zkToggle) {
    zkToggle.classList.toggle('active', enabled)
    zkToggle.textContent = enabled ? 'ZK Mode: ON' : 'ZK Mode: OFF'
  }

  // Show/hide ZK panel
  const zkPanel = document.getElementById('zk-panel')
  if (zkPanel) {
    zkPanel.classList.toggle('hidden', !enabled)
  }

  return enabled
}

// Toggle ZK mode
export function toggleZkMode() {
  return setZkMode(!zkState.enabled)
}

// Create a new ZK game
export async function createZkGame() {
  const response = await fetch(`${getZkApiBase()}/game`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' }
  })
  const data = await response.json()

  zkState.gameId = data.id
  zkState.stateRoot = data.state_root
  zkState.roots = [data.state_root]
  zkState.lastProof = null
  zkState.proofHistory = []

  renderZkState()
  return data
}

// Get ZK game state
export async function getZkGame(id) {
  const response = await fetch(`${getZkApiBase()}/game/${id}`)
  const data = await response.json()

  zkState.stateRoot = data.state_root
  zkState.roots = data.roots || []

  renderZkState()
  return data
}

// Make a ZK move
export async function makeZkMove(position) {
  if (!zkState.gameId) {
    throw new Error('No ZK game created')
  }

  const response = await fetch(`${getZkApiBase()}/game/${zkState.gameId}/move`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ position })
  })
  const data = await response.json()

  if (data.success) {
    zkState.stateRoot = data.post_state_root
    zkState.roots.push(data.post_state_root)
    zkState.lastProof = data.proof
    zkState.proofHistory.push({
      move: position,
      player: data.player,
      proof: data.proof,
      preRoot: data.pre_state_root,
      postRoot: data.post_state_root
    })

    renderZkState()
  }

  return data
}

// Check for win with ZK proof
export async function checkZkWin() {
  if (!zkState.gameId) {
    throw new Error('No ZK game created')
  }

  const response = await fetch(`${getZkApiBase()}/game/${zkState.gameId}/check-win`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' }
  })
  const data = await response.json()

  if (data.has_winner && data.proof) {
    zkState.lastProof = data.proof
  }

  renderZkState()
  return data
}

// Get available circuits
export async function getZkCircuits() {
  const response = await fetch(`${getZkApiBase()}/circuits`)
  return response.json()
}

// Check ZK health
export async function checkZkHealth() {
  try {
    const response = await fetch(`${getZkApiBase()}/health`)
    return response.json()
  } catch (e) {
    return { status: 'error', error: e.message }
  }
}

// Format state root for display (truncated)
function formatRoot(root) {
  if (!root) return 'N/A'
  const str = String(root)
  if (str.length > 20) {
    return str.slice(0, 10) + '...' + str.slice(-8)
  }
  return str
}

// Format proof hex for display
function formatProofHex(hex) {
  if (!hex) return 'N/A'
  if (hex.length > 32) {
    return hex.slice(0, 16) + '...'
  }
  return hex
}

// Render ZK state panel
export function renderZkState() {
  const panel = document.getElementById('zk-state-content')
  if (!panel) return

  if (!zkState.enabled) {
    panel.innerHTML = '<p class="zk-disabled">ZK mode disabled</p>'
    return
  }

  if (!zkState.gameId) {
    panel.innerHTML = '<p class="zk-no-game">Start a new game to see ZK state</p>'
    return
  }

  const proofInfo = zkState.lastProof ? `
    <div class="zk-proof-info">
      <div class="zk-proof-header" onclick="toggleProofDetails()">
        <span class="zk-proof-circuit">${zkState.lastProof.circuit}</span>
        <span class="zk-proof-status ${zkState.lastProof.verified ? 'verified' : 'unverified'}">
          ${zkState.lastProof.verified ? '✓ Verified' : '✗ Unverified'}
        </span>
      </div>
      <div id="zk-proof-details" class="zk-proof-details hidden">
        <div class="zk-proof-row">
          <span class="zk-label">Proof:</span>
          <code class="zk-value">${formatProofHex(zkState.lastProof.proof_hex)}</code>
        </div>
        <div class="zk-proof-row">
          <span class="zk-label">Public Inputs:</span>
          <div class="zk-inputs">
            ${(zkState.lastProof.public_inputs || []).map(input =>
              `<code class="zk-input">${formatRoot(input)}</code>`
            ).join('')}
          </div>
        </div>
        <button class="btn btn-small" onclick="exportProof()">Export Proof</button>
      </div>
    </div>
  ` : ''

  panel.innerHTML = `
    <div class="zk-game-info">
      <div class="zk-row">
        <span class="zk-label">Game ID:</span>
        <code class="zk-value">${zkState.gameId}</code>
      </div>
      <div class="zk-row">
        <span class="zk-label">State Root:</span>
        <code class="zk-value zk-root">${formatRoot(zkState.stateRoot)}</code>
      </div>
      <div class="zk-row">
        <span class="zk-label">Moves:</span>
        <span class="zk-value">${zkState.roots.length - 1}</span>
      </div>
    </div>
    ${proofInfo}
    <div class="zk-roots">
      <div class="zk-roots-header">State Root History</div>
      <div class="zk-roots-list">
        ${zkState.roots.map((root, i) => `
          <div class="zk-root-item ${i === zkState.roots.length - 1 ? 'current' : ''}">
            <span class="zk-root-num">${i}</span>
            <code class="zk-root-hash">${formatRoot(root)}</code>
          </div>
        `).join('')}
      </div>
    </div>
  `
}

// Toggle proof details visibility
window.toggleProofDetails = function() {
  const details = document.getElementById('zk-proof-details')
  if (details) {
    details.classList.toggle('hidden')
  }
}

// Export proof as JSON
window.exportProof = function() {
  if (!zkState.lastProof) {
    alert('No proof to export')
    return
  }

  const data = {
    game_id: zkState.gameId,
    circuit: zkState.lastProof.circuit,
    proof_hex: zkState.lastProof.proof_hex,
    public_inputs: zkState.lastProof.public_inputs,
    verified: zkState.lastProof.verified,
    state_root: zkState.stateRoot,
    exported_at: new Date().toISOString()
  }

  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `zk-proof-${zkState.gameId}-${Date.now()}.json`
  a.click()
  URL.revokeObjectURL(url)
}

// Export for console testing
window.zkState = zkState
window.setZkMode = setZkMode
window.toggleZkMode = toggleZkMode
window.createZkGame = createZkGame
window.getZkGame = getZkGame
window.makeZkMove = makeZkMove
window.checkZkWin = checkZkWin
window.checkZkHealth = checkZkHealth
