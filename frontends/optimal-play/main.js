// Provably Optimal Play — interactive board + ODE heatmap + ZK proofs

const EMPTY = '';
const X = 'X';
const O = 'O';

const WIN_LINES = [
  [[0,0],[0,1],[0,2]], [[1,0],[1,1],[1,2]], [[2,0],[2,1],[2,2]], // rows
  [[0,0],[1,0],[2,0]], [[0,1],[1,1],[2,1]], [[0,2],[1,2],[2,2]], // cols
  [[0,0],[1,1],[2,2]], [[0,2],[1,1],[2,0]],                      // diags
];

// TTT contract addresses on Base Sepolia (updated after deployment)
const TTT_CONTRACT = {
  zkOde: '0x5B96db6164EC6d5c8F99c650B3979EF931771Dd8',
  verifier: '0x6c8f6dC588f0f3f89aF581338d2196B06F3Fd989',
  adapter: '0x1DDfa68Ac8578aEF0D33948622a87d3614A9B462',
};
const BASE_SEPOLIA_RPC = 'https://sepolia.base.org';
const BASESCAN_URL = 'https://sepolia.basescan.org';

let board = Array.from({ length: 3 }, () => Array(3).fill(EMPTY));
let currentPlayer = X;
let gameOver = false;

const $board = document.getElementById('board');
const $status = document.getElementById('status-text');
const cells = $board.querySelectorAll('.cell');

// Initialize board
function resetGame() {
  board = Array.from({ length: 3 }, () => Array(3).fill(EMPTY));
  currentPlayer = X;
  gameOver = false;
  $status.textContent = "X\u2019s turn \u2014 click a cell to play";
  clearPipelineHighlights();
  hideProofSection();

  cells.forEach(cell => {
    cell.innerHTML = '';
    cell.className = 'cell';
  });

  evaluate();
}

// Check for winner
function checkWinner() {
  for (const line of WIN_LINES) {
    const [a, b, c] = line;
    const va = board[a[0]][a[1]];
    const vb = board[b[0]][b[1]];
    const vc = board[c[0]][c[1]];
    if (va && va === vb && vb === vc) {
      return { winner: va, line };
    }
  }
  const full = board.every(row => row.every(c => c !== EMPTY));
  if (full) return { winner: 'draw', line: null };
  return null;
}

// Highlight winning cells
function highlightWin(line) {
  if (!line) return;
  for (const [r, c] of line) {
    const cell = $board.querySelector(`[data-r="${r}"][data-c="${c}"]`);
    if (cell) cell.classList.add('winning');
  }
}

// Handle cell click
function onCellClick(e) {
  const cell = e.currentTarget;
  const r = parseInt(cell.dataset.r);
  const c = parseInt(cell.dataset.c);

  if (board[r][c] !== EMPTY || gameOver) return;

  // Capture board state BEFORE the move for the proof request
  const boardBeforeMove = board.map(row => [...row]);
  const playerForProof = currentPlayer;

  board[r][c] = currentPlayer;
  cell.innerHTML = `<span class="piece ${currentPlayer.toLowerCase()}">${currentPlayer}</span>`;
  cell.classList.add('occupied');

  // Request proof for the move (async, non-blocking)
  requestProof(boardBeforeMove, playerForProof, r, c);

  const result = checkWinner();
  if (result) {
    gameOver = true;
    cells.forEach(c => c.classList.add('game-over'));
    clearHeatmap();

    if (result.winner === 'draw') {
      $status.textContent = "Draw!";
    } else {
      $status.textContent = `${result.winner} wins!`;
      highlightWin(result.line);
    }
    return;
  }

  currentPlayer = currentPlayer === X ? O : X;
  $status.textContent = `${currentPlayer}\u2019s turn \u2014 click a cell to play`;
  evaluate();
}

cells.forEach(cell => cell.addEventListener('click', onCellClick));

// Clear heatmap overlays
function clearHeatmap() {
  cells.forEach(cell => {
    const overlay = cell.querySelector('.heat-overlay');
    const marker = cell.querySelector('.optimal-marker');
    if (overlay) overlay.remove();
    if (marker) marker.remove();
    cell.style.background = '';
  });
}

// Color interpolation: score in [0,1] -> red(0) -> yellow(0.5) -> green(1)
function scoreColor(t) {
  t = Math.max(0, Math.min(1, t));
  if (t < 0.5) {
    const p = t / 0.5;
    const r = Math.round(239 + (245 - 239) * p);
    const g = Math.round(68 + (158 - 68) * p);
    const b = Math.round(68 + (11 - 68) * p);
    return `rgb(${r},${g},${b})`;
  } else {
    const p = (t - 0.5) / 0.5;
    const r = Math.round(245 + (52 - 245) * p);
    const g = Math.round(158 + (211 - 158) * p);
    const b = Math.round(11 + (153 - 11) * p);
    return `rgb(${r},${g},${b})`;
  }
}

// Call /zk-ode/api/evaluate and render heatmap
async function evaluate() {
  clearHeatmap();

  const hasEmpty = board.some(row => row.some(c => c === EMPTY));
  if (!hasEmpty || gameOver) return;

  highlightPipelineStage('stage-ode');

  try {
    const resp = await fetch('/zk-ode/api/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ board, player: currentPlayer }),
    });

    if (!resp.ok) {
      console.warn('Evaluate API error:', resp.status);
      clearPipelineHighlights();
      return;
    }

    const data = await resp.json();
    highlightPipelineStage('stage-heatmap');
    renderHeatmap(data.values || {}, data.optimal);
  } catch (err) {
    console.warn('Evaluate API unavailable:', err.message);
    clearPipelineHighlights();
  }
}

// Render heatmap overlay on empty cells
function renderHeatmap(values, optimalKey) {
  const scores = Object.values(values);
  if (scores.length === 0) return;

  const min = Math.min(...scores);
  const max = Math.max(...scores);
  const range = max - min || 1;

  cells.forEach(cell => {
    const r = cell.dataset.r;
    const c = cell.dataset.c;
    const key = `${r}${c}`;

    if (board[r][c] !== EMPTY) return;

    const score = values[key];
    if (score === undefined) return;

    const t = (score - min) / range;
    const color = scoreColor(t);

    cell.style.background = `${color}22`;
    cell.style.borderColor = color;

    const overlay = document.createElement('div');
    overlay.className = 'heat-overlay';
    overlay.style.background = `${color}33`;
    overlay.innerHTML = `<span class="heat-value">${score.toFixed(2)}</span>`;
    cell.appendChild(overlay);

    if (key === optimalKey) {
      const marker = document.createElement('span');
      marker.className = 'optimal-marker';
      marker.textContent = '\u2605';
      cell.appendChild(marker);
      cell.style.borderColor = '#fbbf24';
      cell.style.boxShadow = '0 0 10px rgba(251, 191, 36, 0.25)';
    }
  });
}

// Pipeline stage highlighting
function highlightPipelineStage(id) {
  clearPipelineHighlights();
  const el = document.getElementById(id);
  if (el) el.classList.add('active');
}

function clearPipelineHighlights() {
  document.querySelectorAll('.pipeline-stage').forEach(s => s.classList.remove('active'));
}

// --- ZK Proof Section ---

function showProofSection(msg) {
  const section = document.getElementById('proof-section');
  const status = document.getElementById('proof-status');
  section.style.display = '';
  status.textContent = msg;
  document.getElementById('proof-details').innerHTML = '';
}

function hideProofSection() {
  document.getElementById('proof-section').style.display = 'none';
}

async function requestProof(boardBeforeMove, player, row, col) {
  showProofSection(`Generating ZK proof for ${player} at (${row},${col})...`);
  highlightPipelineStage('stage-zk');

  try {
    const resp = await fetch('/zk-ode/api/prove-optimal', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        board: boardBeforeMove,
        player: player,
        move: [row, col],
      }),
    });

    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}));
      showProofSection(`Proof failed: ${err.error || resp.status}`);
      return;
    }

    const data = await resp.json();
    highlightPipelineStage('stage-contract');
    renderProofResult(data, player, row, col);
  } catch (err) {
    showProofSection(`Proof unavailable: ${err.message}`);
  }
}

function renderProofResult(data, player, row, col) {
  const status = document.getElementById('proof-status');
  const details = document.getElementById('proof-details');

  const circuit = data.circuit || 'tsit5_step';
  const isOptimal = data.is_optimal;
  const optimalLabel = isOptimal ? 'Optimal' : 'Suboptimal';
  const optimalClass = isOptimal ? 'proof-optimal' : 'proof-suboptimal';

  status.textContent = `Proof generated for ${player} at (${row},${col})`;

  let html = `
    <div class="proof-grid">
      <div class="proof-item">
        <span class="proof-label">Circuit</span>
        <span class="proof-value">${circuit}</span>
      </div>
      <div class="proof-item">
        <span class="proof-label">Move</span>
        <span class="proof-value ${optimalClass}">${optimalLabel}</span>
      </div>
      <div class="proof-item">
        <span class="proof-label">Score</span>
        <span class="proof-value">${(data.objective_value || 0).toFixed(4)}</span>
      </div>`;

  if (data.pre_state_root) {
    const root = data.pre_state_root;
    const short = root.slice(0, 10) + '...' + root.slice(-8);
    html += `
      <div class="proof-item">
        <span class="proof-label">State Root</span>
        <span class="proof-value mono">${short}</span>
      </div>`;
  }

  if (data.transition_index !== undefined) {
    html += `
      <div class="proof-item">
        <span class="proof-label">Transition</span>
        <span class="proof-value">#${data.transition_index}</span>
      </div>`;
  }

  html += '</div>';
  details.innerHTML = html;
}

// --- On-Chain Status ---

async function fetchOnChainStatus() {
  if (!TTT_CONTRACT.zkOde) {
    document.getElementById('ttt-contract').textContent = 'Awaiting deployment';
    return;
  }

  const addr = TTT_CONTRACT.zkOde;
  const short = addr.slice(0, 6) + '...' + addr.slice(-4);
  const link = `<a href="${BASESCAN_URL}/address/${addr}" target="_blank" class="onchain-link">${short}</a>`;
  document.getElementById('ttt-contract').innerHTML = link;

  try {
    // currentStateRoot()
    const rootData = '0x53f3a866'; // keccak256("currentStateRoot()")[:4]
    const rootResp = await ethCall(addr, rootData);
    if (rootResp && rootResp !== '0x') {
      const rootHex = '0x' + rootResp.slice(2, 18) + '...';
      document.getElementById('ttt-state-root').textContent = rootHex;
    }

    // enforceOptimal()
    const enforceData = '0x6c63a6a8'; // keccak256("enforceOptimal()")[:4]
    const enforceResp = await ethCall(addr, enforceData);
    if (enforceResp) {
      const enforced = parseInt(enforceResp, 16) !== 0;
      document.getElementById('ttt-enforce').textContent = enforced ? 'Yes' : 'No';
    }

    // stepCount()
    const stepsData = '0xc4b55e77'; // keccak256("stepCount()")[:4]
    const stepsResp = await ethCall(addr, stepsData);
    if (stepsResp) {
      document.getElementById('ttt-steps').textContent = parseInt(stepsResp, 16).toString();
    }
  } catch (err) {
    console.warn('On-chain fetch error:', err.message);
  }
}

async function ethCall(to, data) {
  const resp = await fetch(BASE_SEPOLIA_RPC, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'eth_call',
      params: [{ to, data }, 'latest'],
    }),
  });
  const json = await resp.json();
  return json.result;
}

// New game button
document.getElementById('btn-new').addEventListener('click', resetGame);

// Initial evaluation + on-chain status
evaluate();
fetchOnChainStatus();
