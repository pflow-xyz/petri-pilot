// Verifiable Computation — interactive board + heatmap scoring + ZK proofs
// Scoring matches the on-chain ZK heatmap verifier exactly (no ODE solver needed)

const EMPTY = '';
const X = 'X';
const O = 'O';

const WIN_LINES = [
  [[0,0],[0,1],[0,2]], [[1,0],[1,1],[1,2]], [[2,0],[2,1],[2,2]], // rows
  [[0,0],[1,0],[2,0]], [[0,1],[1,1],[2,1]], [[0,2],[1,2],[2,2]], // cols
  [[0,0],[1,1],[2,2]], [[0,2],[1,1],[2,0]],                      // diags
];

// Win lines as flat cell indices (0-8) for heatmap scoring
const WIN_LINES_FLAT = [
  [0, 1, 2], [3, 4, 5], [6, 7, 8], // rows
  [0, 3, 6], [1, 4, 7], [2, 5, 8], // cols
  [0, 4, 8], [2, 4, 6],             // diags
];

// Position weights from Petri net rate constants (number of win lines through each cell)
const POSITION_WEIGHTS = [
  3, 2, 3, // corner, edge, corner
  2, 4, 2, // edge, center, edge
  3, 2, 3, // corner, edge, corner
];

const WIN_BONUS = 10.0;
const BLOCK_PENALTY = 1.5;

// Compute heatmap scores matching the ZK circuit exactly
// score[i] = position_weight + 10*win_flag - 1.5*block_flag*(1-win_flag)
function computeHeatmap(board, player) {
  const opponent = player === X ? O : X;
  const values = {};
  let optimal = null;
  let bestScore = -Infinity;

  // Build flat cell arrays (indexed 0-8)
  const currentPiece = [];
  const opponentPiece = [];
  const cellEmpty = [];
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      currentPiece.push(board[r][c] === player ? 1 : 0);
      opponentPiece.push(board[r][c] === opponent ? 1 : 0);
      cellEmpty.push(board[r][c] === EMPTY ? 1 : 0);
    }
  }

  for (let i = 0; i < 9; i++) {
    if (!cellEmpty[i]) continue;

    let score = POSITION_WEIGHTS[i];

    // Win flag: does placing here complete 3-in-a-row?
    const win = heatmapWinFlag(i, currentPiece);
    // Block flag: after placing here, does opponent have an unblocked threat?
    const block = heatmapBlockFlag(i, opponentPiece, cellEmpty);

    if (win) {
      score += WIN_BONUS;
    } else if (block) {
      score -= BLOCK_PENALTY;
    }

    const r = Math.floor(i / 3);
    const c = i % 3;
    const key = `${r}${c}`;
    values[key] = score;

    if (score > bestScore) {
      bestScore = score;
      optimal = key;
    }
  }

  return { values, optimal, player };
}

// Check if placing at cell completes a 3-in-a-row for current player
function heatmapWinFlag(cell, currentPiece) {
  for (const line of WIN_LINES_FLAT) {
    if (!line.includes(cell)) continue;
    let allOwned = true;
    for (const c of line) {
      if (c === cell) continue;
      if (!currentPiece[c]) { allOwned = false; break; }
    }
    if (allOwned) return true;
  }
  return false;
}

// Check if after placing at cell, opponent has an unblocked winning threat
function heatmapBlockFlag(cell, opponentPiece, cellEmpty) {
  for (const line of WIN_LINES_FLAT) {
    let oppCount = 0;
    let missingCell = -1;
    for (const c of line) {
      if (opponentPiece[c]) oppCount++;
      else if (cellEmpty[c]) missingCell = c;
    }
    if (oppCount === 2 && missingCell >= 0 && missingCell !== cell) return true;
  }
  return false;
}

// TTT contract addresses on Base Sepolia (updated after deployment)
const TTT_CONTRACT = {
  zkOde: '0xF5d9cB0247698361D561faA2E30dDA7855fC25Db',
  verifier: '0x97a6Bb8FBBbBb81BF36456829A6a41e29030f351',
  adapter: '0x3211ac2a941d357819EdC2b4ce0D0888953b950E',
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

// --- API Explorer ---
const $apiRequest = document.getElementById('api-request');
const $apiResponse = document.getElementById('api-response');
const $apiStatus = document.getElementById('api-status');
const $apiDetails = document.getElementById('api-details');

function updateApiRequest() {
  const payload = { board, player: currentPlayer };
  $apiRequest.textContent = JSON.stringify(payload, null, 2);
}

function updateApiResponse(data, status) {
  if (status === 'ok') {
    $apiStatus.textContent = '200 OK';
    $apiStatus.className = 'api-status ok';
    $apiResponse.textContent = JSON.stringify(data, null, 2);

    // Show details
    $apiDetails.style.display = '';
    const optKey = data.optimal;
    const optScore = optKey ? data.values[optKey] : null;
    document.getElementById('api-optimal').textContent = optKey ? `(${optKey[0]},${optKey[1]})` : '--';
    document.getElementById('api-optimal').className = 'api-detail-value' + (optKey ? ' optimal' : '');
    document.getElementById('api-optimal-score').textContent = optScore != null ? optScore.toFixed(4) : '--';

    const scores = Object.values(data.values || {});
    document.getElementById('api-empty-count').textContent = scores.length;
    if (scores.length > 0) {
      const min = Math.min(...scores).toFixed(2);
      const max = Math.max(...scores).toFixed(2);
      document.getElementById('api-score-range').textContent = `${min} .. ${max}`;
    } else {
      document.getElementById('api-score-range').textContent = '--';
    }
  } else if (status === 'error') {
    $apiStatus.textContent = 'Error';
    $apiStatus.className = 'api-status error';
    $apiResponse.textContent = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    $apiDetails.style.display = 'none';
  } else {
    $apiStatus.textContent = 'Loading...';
    $apiStatus.className = 'api-status loading';
  }
}

function buildCurlCommand() {
  const payload = JSON.stringify({ board, player: currentPlayer });
  return `# Scores computed locally (same algorithm as ZK circuit)
# Server endpoint for comparison:
curl -s -X POST https://pilot.pflow.xyz/zk-ode/api/evaluate \\
  -H "Content-Type: application/json" \\
  -d '${payload}' | jq .`;
}

document.getElementById('btn-copy-curl').addEventListener('click', () => {
  navigator.clipboard.writeText(buildCurlCommand()).then(() => {
    const btn = document.getElementById('btn-copy-curl');
    btn.textContent = 'Copied!';
    setTimeout(() => { btn.textContent = 'Copy curl'; }, 1500);
  });
});

document.getElementById('btn-eval').addEventListener('click', () => {
  evaluate();
});

// Compute heatmap locally (matches ZK circuit exactly, no server needed)
function evaluate() {
  clearHeatmap();

  const hasEmpty = board.some(row => row.some(c => c === EMPTY));
  if (!hasEmpty || gameOver) {
    updateApiRequest();
    return;
  }

  highlightPipelineStage('stage-ode');
  updateApiRequest();

  const data = computeHeatmap(board, currentPlayer);

  highlightPipelineStage('stage-heatmap');
  updateApiResponse(data, 'ok');
  renderHeatmap(data.values, data.optimal);
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
    const rootData = '0xac2eba98'; // cast sig "currentStateRoot()"
    const rootResp = await ethCall(addr, rootData);
    if (rootResp && rootResp !== '0x') {
      const rootHex = '0x' + rootResp.slice(2, 18) + '...';
      document.getElementById('ttt-state-root').textContent = rootHex;
    }

    // enforceOptimal()
    const enforceData = '0x51fef09f'; // cast sig "enforceOptimal()"
    const enforceResp = await ethCall(addr, enforceData);
    if (enforceResp) {
      const enforced = parseInt(enforceResp, 16) !== 0;
      document.getElementById('ttt-enforce').textContent = enforced ? 'Yes' : 'No';
    }

    // stepCount()
    const stepsData = '0x415deffa'; // cast sig "stepCount()"
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
