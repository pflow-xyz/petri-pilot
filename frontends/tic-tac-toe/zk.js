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
        <div class="zk-export-buttons">
          <button class="btn btn-small" onclick="exportProofJSON()">Export JSON</button>
          <button class="btn btn-small" onclick="exportSolidityCalldata()">Solidity Calldata</button>
          <button class="btn btn-small" onclick="downloadVerifier()">Download Verifier</button>
        </div>
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
    ${zkState.proofHistory.length > 0 ? `
    <div class="zk-verify-section">
      <button class="btn btn-verify" onclick="verifyGameHistory()">
        🔍 Verify Game History
      </button>
      <span class="zk-verify-hint">${zkState.proofHistory.length} moves recorded</span>
    </div>
    ` : ''}
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
window.exportProofJSON = function() {
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
    // Solidity-compatible proof points
    a: zkState.lastProof.a,
    b: zkState.lastProof.b,
    c: zkState.lastProof.c,
    raw_proof: zkState.lastProof.raw_proof,
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

// Export as Solidity calldata
window.exportSolidityCalldata = function() {
  if (!zkState.lastProof) {
    alert('No proof to export')
    return
  }

  const proof = zkState.lastProof

  // Format for Groth16 verifier: verifyProof(uint[2] a, uint[2][2] b, uint[2] c, uint[] input)
  const calldata = `// Solidity calldata for ${proof.circuit} circuit verification
// verifyProof(uint[2] memory a, uint[2][2] memory b, uint[2] memory c, uint[] memory input)

// Proof point A
uint[2] memory a = [
    ${proof.a?.[0] || '0x0'},
    ${proof.a?.[1] || '0x0'}
];

// Proof point B (note: order is reversed for Solidity)
uint[2][2] memory b = [
    [${proof.b?.[0]?.[0] || '0x0'}, ${proof.b?.[0]?.[1] || '0x0'}],
    [${proof.b?.[1]?.[0] || '0x0'}, ${proof.b?.[1]?.[1] || '0x0'}]
];

// Proof point C
uint[2] memory c = [
    ${proof.c?.[0] || '0x0'},
    ${proof.c?.[1] || '0x0'}
];

// Public inputs
uint[] memory input = new uint[](${proof.public_inputs?.length || 0});
${(proof.public_inputs || []).map((inp, i) => `input[${i}] = ${inp};`).join('\n')}

// Raw calldata for direct contract call (flat array)
// [a[0], a[1], b[0][0], b[0][1], b[1][0], b[1][1], c[0], c[1], ...inputs]
bytes memory rawCalldata = abi.encode(
    ${(proof.raw_proof || []).join(',\n    ')}${proof.raw_proof?.length ? ',' : ''}
    ${(proof.public_inputs || []).join(',\n    ')}
);
`

  const blob = new Blob([calldata], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `zk-${proof.circuit}-calldata-${Date.now()}.sol`
  a.click()
  URL.revokeObjectURL(url)
}

// Download Solidity verifier contract
window.downloadVerifier = async function() {
  if (!zkState.lastProof) {
    alert('No proof available - play a game first')
    return
  }

  const circuit = zkState.lastProof.circuit

  try {
    const response = await fetch(`${getZkApiBase()}/verifier/${circuit}`)
    if (!response.ok) {
      throw new Error(`Failed to fetch verifier: ${response.status}`)
    }

    const solidity = await response.text()

    const blob = new Blob([solidity], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${circuit}_verifier.sol`
    a.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    console.error('Failed to download verifier:', err)
    alert('Failed to download verifier contract')
  }
}

// Legacy export function (for backwards compatibility)
window.exportProof = window.exportProofJSON

// Verify entire game history (replay verification)
export async function verifyGameHistory() {
  if (!zkState.gameId || zkState.proofHistory.length === 0) {
    alert('No game history to verify')
    return null
  }

  // Build the replay request
  const request = {
    initial_root: zkState.roots[0],
    moves: zkState.proofHistory.map(h => ({
      position: h.move,
      player: h.player,
      pre_root: h.preRoot,
      post_root: h.postRoot,
      proof_verified: h.proof?.verified || false
    })),
    win_proof: zkState.lastProof?.circuit === 'win' ? {
      circuit: zkState.lastProof.circuit,
      verified: zkState.lastProof.verified
    } : null
  }

  const response = await fetch(`${getZkApiBase()}/replay`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request)
  })
  const result = await response.json()

  // Display results
  showReplayResults(result)
  return result
}

// Display replay verification results with interactive step details
function showReplayResults(result) {
  // Create or update results modal
  let modal = document.getElementById('zk-replay-modal')
  if (!modal) {
    modal = document.createElement('div')
    modal.id = 'zk-replay-modal'
    modal.className = 'zk-modal'
    document.body.appendChild(modal)
  }

  const statusIcon = result.valid ? '✅' : '❌'
  const chainIcon = result.chain_valid ? '🔗' : '⛓️‍💥'

  modal.innerHTML = `
    <div class="zk-modal-content">
      <div class="zk-modal-header">
        <h3>${statusIcon} Game Replay Verification</h3>
        <button class="zk-modal-close" onclick="closeReplayModal()">×</button>
      </div>
      <div class="zk-modal-body">
        <div class="zk-replay-summary">
          <div class="zk-replay-stat ${result.valid ? 'valid' : 'invalid'}">
            <span class="label">Overall</span>
            <span class="value">${result.valid ? 'VALID' : 'INVALID'}</span>
          </div>
          <div class="zk-replay-stat ${result.chain_valid ? 'valid' : 'invalid'}">
            <span class="label">${chainIcon} State Chain</span>
            <span class="value">${result.chain_valid ? 'Intact' : 'Broken'}</span>
          </div>
          <div class="zk-replay-stat">
            <span class="label">Moves</span>
            <span class="value">${result.move_count}</span>
          </div>
          ${result.win_verified !== undefined ? `
          <div class="zk-replay-stat ${result.win_verified ? 'valid' : 'invalid'}">
            <span class="label">Win Proof</span>
            <span class="value">${result.win_verified ? '✓ Verified' : '✗ Not Verified'}</span>
          </div>
          ` : ''}
        </div>

        <p class="zk-hint">👆 Click any step below to inspect its ZK proof details</p>

        <div class="zk-replay-container">
          <div class="zk-replay-moves">
            <h4>Move Chain</h4>
            <div class="zk-move-chain">
              <div class="zk-chain-node initial clickable" onclick="showStepDetail(-1)">
                <span class="label">🎯 Initial State</span>
                <code>${result.move_results.length > 0 ? result.move_results[0].pre_root : 'N/A'}</code>
                <span class="click-hint">click to learn</span>
              </div>
              ${result.move_results.map((m, i) => `
                <div class="zk-chain-arrow ${m.chain_valid ? '' : 'broken'} clickable" onclick="showStepDetail(${i})">
                  <span class="move-label">${m.player === 1 ? 'X' : 'O'}→${m.position}</span>
                  <span class="proof-status ${m.proof_verified ? 'verified' : 'unverified'}">
                    ${m.proof_verified ? '✓' : '✗'}
                  </span>
                </div>
                <div class="zk-chain-node ${i === result.move_results.length - 1 ? 'final' : ''} clickable" onclick="showStepDetail(${i})">
                  <span class="label">Move ${m.move}</span>
                  <code>${m.post_root}</code>
                  ${m.error ? `<span class="error">${m.error}</span>` : ''}
                  <span class="click-hint">click to inspect</span>
                </div>
              `).join('')}
            </div>
          </div>

          <div id="zk-step-detail" class="zk-step-detail">
            <div class="zk-detail-placeholder">
              <div class="zk-detail-icon">🔍</div>
              <p>Select a step from the chain to see its ZK proof details</p>
              <p class="zk-detail-hint">Learn how zero-knowledge proofs verify each move!</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  `

  // Store result for detail view
  window._replayResult = result

  modal.style.display = 'flex'
}

// Show detailed view of a specific step
window.showStepDetail = function(stepIndex) {
  const detail = document.getElementById('zk-step-detail')
  if (!detail) return

  // Initial state explanation
  if (stepIndex === -1) {
    const initialRoot = zkState.roots[0]
    detail.innerHTML = `
      <div class="zk-detail-content">
        <h4>🎯 Initial State (Empty Board)</h4>

        <div class="zk-detail-section">
          <div class="zk-detail-label">State Root</div>
          <code class="zk-detail-value">${initialRoot}</code>
        </div>

        <div class="zk-detail-explainer">
          <h5>🎓 What is a State Root?</h5>
          <p>A <strong>state root</strong> is a cryptographic fingerprint of the entire game board.
          It's computed using the <strong>MiMC hash function</strong>, which is efficient to verify inside ZK circuits.</p>

          <p>For an empty board [0,0,0,0,0,0,0,0,0], the MiMC hash produces this unique 256-bit number.</p>

          <h5>🔐 Why does this matter?</h5>
          <p>The state root lets us <strong>prove the board state without revealing it</strong>.
          Anyone can verify that a move was made on a specific board configuration
          just by checking the state root—no need to trust anyone!</p>
        </div>
      </div>
    `
    return
  }

  // Move step explanation
  const move = window._replayResult.move_results[stepIndex]
  const proofData = zkState.proofHistory[stepIndex]
  const player = move.player === 1 ? 'X' : 'O'
  const position = move.position
  const row = Math.floor(position / 3)
  const col = position % 3

  detail.innerHTML = `
    <div class="zk-detail-content">
      <h4>Move ${move.move}: ${player} plays at (${row}, ${col})</h4>

      <div class="zk-detail-grid">
        <div class="zk-detail-section">
          <div class="zk-detail-label">Pre-State Root</div>
          <code class="zk-detail-value small">${proofData?.preRoot || move.pre_root}</code>
        </div>

        <div class="zk-detail-section">
          <div class="zk-detail-label">Post-State Root</div>
          <code class="zk-detail-value small">${proofData?.postRoot || move.post_root}</code>
        </div>
      </div>

      <div class="zk-detail-section">
        <div class="zk-detail-label">Circuit</div>
        <span class="zk-circuit-badge">${proofData?.proof?.circuit || 'move'}</span>
        <span class="zk-verify-badge ${move.proof_verified ? 'verified' : 'unverified'}">
          ${move.proof_verified ? '✓ Verified' : '✗ Not Verified'}
        </span>
      </div>

      ${proofData?.proof ? `
      <div class="zk-detail-section">
        <div class="zk-detail-label">Public Inputs (visible to verifier)</div>
        <div class="zk-public-inputs">
          <div class="zk-input-item">
            <span class="input-label">pre_state_root</span>
            <code>${formatInputShort(proofData.proof.public_inputs?.[0])}</code>
          </div>
          <div class="zk-input-item">
            <span class="input-label">post_state_root</span>
            <code>${formatInputShort(proofData.proof.public_inputs?.[1])}</code>
          </div>
          <div class="zk-input-item">
            <span class="input-label">position</span>
            <code>${position}</code>
          </div>
          <div class="zk-input-item">
            <span class="input-label">player</span>
            <code>${move.player} (${player})</code>
          </div>
        </div>
      </div>

      <div class="zk-detail-section collapsible">
        <div class="zk-detail-label clickable" onclick="toggleProofHex()">
          Proof (click to expand) <span id="proof-toggle">▶</span>
        </div>
        <code id="proof-hex" class="zk-detail-value proof-hex hidden">${proofData.proof.proof_hex}</code>
      </div>
      ` : ''}

      <div class="zk-detail-explainer">
        <h5>🎓 What does this proof verify?</h5>
        <p>This ZK proof mathematically guarantees that:</p>
        <ol>
          <li><strong>Valid Move:</strong> Position ${position} was empty before the move</li>
          <li><strong>Correct Player:</strong> It was ${player}'s turn (turn ${stepIndex + 1})</li>
          <li><strong>State Transition:</strong> The board changed correctly from pre-state to post-state</li>
          <li><strong>No Cheating:</strong> The prover knows the actual board (private input) that hashes to these roots</li>
        </ol>

        <h5>🔐 Zero-Knowledge Property</h5>
        <p>The verifier learns <em>nothing</em> about the board except what's revealed in the public inputs.
        The proof is ~256 bytes regardless of game complexity!</p>

        <h5>🔗 Chain Integrity</h5>
        <p class="${move.chain_valid ? 'valid' : 'invalid'}">
          ${move.chain_valid
            ? '✓ This move\'s pre-state root matches the previous post-state root, maintaining chain integrity.'
            : '✗ Chain broken! Pre-state doesn\'t match previous post-state.'}
        </p>
      </div>
    </div>
  `
}

// Format public input for display
function formatInputShort(input) {
  if (!input) return 'N/A'
  if (input.length > 24) {
    return input.slice(0, 10) + '...' + input.slice(-10)
  }
  return input
}

// Toggle proof hex visibility
window.toggleProofHex = function() {
  const hex = document.getElementById('proof-hex')
  const toggle = document.getElementById('proof-toggle')
  if (hex && toggle) {
    hex.classList.toggle('hidden')
    toggle.textContent = hex.classList.contains('hidden') ? '▶' : '▼'
  }
}

// Close replay modal
window.closeReplayModal = function() {
  const modal = document.getElementById('zk-replay-modal')
  if (modal) {
    modal.style.display = 'none'
  }
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
window.verifyGameHistory = verifyGameHistory
