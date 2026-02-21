// Provably Optimal Play — interactive board + ODE heatmap

const EMPTY = '';
const X = 'X';
const O = 'O';

const WIN_LINES = [
  [[0,0],[0,1],[0,2]], [[1,0],[1,1],[1,2]], [[2,0],[2,1],[2,2]], // rows
  [[0,0],[1,0],[2,0]], [[0,1],[1,1],[2,1]], [[0,2],[1,2],[2,2]], // cols
  [[0,0],[1,1],[2,2]], [[0,2],[1,1],[2,0]],                      // diags
];

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
  // Check draw
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

  board[r][c] = currentPlayer;
  cell.innerHTML = `<span class="piece ${currentPlayer.toLowerCase()}">${currentPlayer}</span>`;
  cell.classList.add('occupied');

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

  // Check if there are any empty cells
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
  // Collect all scores to normalize
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

    // Background tint
    cell.style.background = `${color}22`;
    cell.style.borderColor = color;

    // Overlay with score value
    const overlay = document.createElement('div');
    overlay.className = 'heat-overlay';
    overlay.style.background = `${color}33`;
    overlay.innerHTML = `<span class="heat-value">${score.toFixed(2)}</span>`;
    cell.appendChild(overlay);

    // Mark optimal
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

// New game button
document.getElementById('btn-new').addEventListener('click', resetGame);

// Initial evaluation
evaluate();
