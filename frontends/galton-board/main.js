// Galton Board — 8-row Petri net producing the binomial distribution

const ROWS = 8;
const BINS = 9;
const TOTAL_BALLS = 256;
const PASCAL = [1, 8, 28, 56, 70, 56, 28, 8, 1]; // C(8,k)
const PASCAL_SUM = 256; // 2^8

// Canvas layout — tighter spacing for 8 rows
const BOARD_W = 460;
const BOARD_H = 580;
const CENTER_X = BOARD_W / 2;
const SPACING_X = 42;
const SPACING_Y = 52;
const TOP_Y = 50;
const PEG_R = 5;
const BALL_R = 4;
const BIN_TOP = TOP_Y + ROWS * SPACING_Y + 15;
const BIN_BOTTOM = BOARD_H - 15;

// State
let ballsRemaining = TOTAL_BALLS;
let binCounts = new Array(BINS).fill(0);
let activeBalls = [];
let instance = null;
let autoTimer = null;
let animSpeed = 3;
let lastTime = 0;

const canvas = document.getElementById('board');
const ctx = canvas.getContext('2d');

// Hi-DPI support
const dpr = window.devicePixelRatio || 1;
canvas.width = BOARD_W * dpr;
canvas.height = BOARD_H * dpr;
canvas.style.width = BOARD_W + 'px';
canvas.style.height = BOARD_H + 'px';
ctx.scale(dpr, dpr);

// Position of a node at (row, col) in the triangular grid
function nodePos(row, col) {
  return {
    x: CENTER_X + (col - row / 2) * SPACING_X,
    y: TOP_Y + row * SPACING_Y
  };
}

// Ball class
class Ball {
  constructor(choices) {
    this.choices = choices;
    this.waypoints = this._buildWaypoints();
    this.transitions = this._buildTransitions();
    this.segment = 0;
    this.progress = 0;
    this.done = false;
    this.bin = this._computeBin();
  }

  _buildWaypoints() {
    const pts = [];
    const start = nodePos(0, 0);
    pts.push({ x: start.x, y: start.y - 30 });
    let col = 0;
    for (let row = 0; row < ROWS; row++) {
      pts.push(nodePos(row, col));
      if (this.choices[row]) col++;
    }
    pts.push(nodePos(ROWS, col));
    return pts;
  }

  _buildTransitions() {
    const trans = [];
    let col = 0;
    for (let row = 0; row < ROWS; row++) {
      const dir = this.choices[row] ? 'R' : 'L';
      trans.push(`p${row}${col}${dir}`);
      if (this.choices[row]) col++;
    }
    return trans;
  }

  _computeBin() {
    let col = 0;
    for (let i = 0; i < ROWS; i++) {
      if (this.choices[i]) col++;
    }
    return col;
  }

  update(dt) {
    if (this.done) return;
    this.progress += dt * animSpeed * 2.5;
    if (this.progress >= 1) {
      this.progress = 0;
      this.segment++;
      if (this.segment >= this.waypoints.length - 1) {
        this.done = true;
      }
    }
  }

  getPosition() {
    if (this.done) {
      return this.waypoints[this.waypoints.length - 1];
    }
    const from = this.waypoints[this.segment];
    const to = this.waypoints[this.segment + 1];
    const t = this.progress;
    const ease = t * t;
    return {
      x: from.x + (to.x - from.x) * ease,
      y: from.y + (to.y - from.y) * ease
    };
  }
}

// Drawing
function draw() {
  ctx.clearRect(0, 0, BOARD_W, BOARD_H);

  // Background
  ctx.fillStyle = '#16213e';
  ctx.fillRect(0, 0, BOARD_W, BOARD_H);

  // Draw guide paths
  ctx.strokeStyle = 'rgba(255,255,255,0.05)';
  ctx.lineWidth = 1;
  for (let row = 0; row < ROWS; row++) {
    for (let col = 0; col <= row; col++) {
      const from = nodePos(row, col);
      const toL = nodePos(row + 1, col);
      const toR = nodePos(row + 1, col + 1);
      ctx.beginPath(); ctx.moveTo(from.x, from.y); ctx.lineTo(toL.x, toL.y); ctx.stroke();
      ctx.beginPath(); ctx.moveTo(from.x, from.y); ctx.lineTo(toR.x, toR.y); ctx.stroke();
    }
  }

  // Draw bin dividers
  ctx.strokeStyle = 'rgba(255,255,255,0.2)';
  ctx.lineWidth = 1.5;
  for (let i = 0; i <= BINS; i++) {
    const x = nodePos(ROWS, 0).x + (i - 0.5) * SPACING_X;
    ctx.beginPath();
    ctx.moveTo(x, BIN_TOP - 5);
    ctx.lineTo(x, BIN_BOTTOM + 10);
    ctx.stroke();
  }

  // Draw bin floor
  ctx.strokeStyle = 'rgba(255,255,255,0.3)';
  ctx.lineWidth = 2;
  const leftEdge = nodePos(ROWS, 0).x - SPACING_X / 2;
  const rightEdge = nodePos(ROWS, BINS - 1).x + SPACING_X / 2;
  ctx.beginPath();
  ctx.moveTo(leftEdge, BIN_BOTTOM + 10);
  ctx.lineTo(rightEdge, BIN_BOTTOM + 10);
  ctx.stroke();

  // Draw stacked balls in bins (compact stacking for many balls)
  const maxStack = Math.floor((BIN_BOTTOM - BIN_TOP) / (BALL_R * 2));
  for (let bin = 0; bin < BINS; bin++) {
    const bx = nodePos(ROWS, bin).x;
    const count = binCounts[bin];
    // Stack balls, overflow by shrinking gap
    const gap = count <= maxStack ? BALL_R * 2 + 1 : (BIN_BOTTOM - BIN_TOP - BALL_R) / Math.max(count, 1);
    for (let j = 0; j < count; j++) {
      const by = BIN_BOTTOM - j * gap;
      if (by < BIN_TOP - BALL_R) break; // don't draw above bin area
      drawBall(bx, by, '#ffd700', '#b8860b');
    }
  }

  // Draw pegs
  for (let row = 0; row < ROWS; row++) {
    for (let col = 0; col <= row; col++) {
      const p = nodePos(row, col);
      ctx.beginPath();
      ctx.arc(p.x, p.y, PEG_R, 0, Math.PI * 2);
      const grad = ctx.createRadialGradient(p.x - 1.5, p.y - 1.5, 0.5, p.x, p.y, PEG_R);
      grad.addColorStop(0, '#e0e0e0');
      grad.addColorStop(1, '#808080');
      ctx.fillStyle = grad;
      ctx.fill();
      ctx.strokeStyle = '#555';
      ctx.lineWidth = 0.5;
      ctx.stroke();
    }
  }

  // Draw entry funnel
  const entry = nodePos(0, 0);
  ctx.strokeStyle = 'rgba(255,255,255,0.3)';
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.moveTo(entry.x - 15, entry.y - 35);
  ctx.lineTo(entry.x - 6, entry.y - 15);
  ctx.stroke();
  ctx.beginPath();
  ctx.moveTo(entry.x + 15, entry.y - 35);
  ctx.lineTo(entry.x + 6, entry.y - 15);
  ctx.stroke();

  // Draw animated balls
  for (const ball of activeBalls) {
    if (!ball.done) {
      const p = ball.getPosition();
      drawBall(p.x, p.y, '#ff6b35', '#cc4400');
    }
  }

  // Bin labels
  ctx.font = '11px system-ui, sans-serif';
  ctx.textAlign = 'center';
  for (let bin = 0; bin < BINS; bin++) {
    const p = nodePos(ROWS, bin);
    ctx.fillStyle = 'rgba(255,255,255,0.5)';
    ctx.fillText(binCounts[bin], p.x, BIN_BOTTOM + 24);
  }

  // Remaining indicator
  ctx.fillStyle = 'rgba(255,255,255,0.3)';
  ctx.font = '11px system-ui, sans-serif';
  ctx.textAlign = 'right';
  ctx.fillText(`${ballsRemaining} remaining`, BOARD_W - 12, 18);
}

function drawBall(x, y, color, strokeColor) {
  ctx.beginPath();
  ctx.arc(x, y, BALL_R, 0, Math.PI * 2);
  const grad = ctx.createRadialGradient(x - 1, y - 1, 0.5, x, y, BALL_R);
  grad.addColorStop(0, lighten(color));
  grad.addColorStop(1, color);
  ctx.fillStyle = grad;
  ctx.fill();
  ctx.strokeStyle = strokeColor;
  ctx.lineWidth = 0.5;
  ctx.stroke();
}

function lighten(hex) {
  const r = parseInt(hex.slice(1,3), 16);
  const g = parseInt(hex.slice(3,5), 16);
  const b = parseInt(hex.slice(5,7), 16);
  return `rgb(${Math.min(255, r+60)}, ${Math.min(255, g+60)}, ${Math.min(255, b+60)})`;
}

// Animation loop
function animate(timestamp) {
  const dt = lastTime ? (timestamp - lastTime) / 1000 : 0;
  lastTime = timestamp;

  let settled = false;
  for (const ball of activeBalls) {
    const wasDone = ball.done;
    ball.update(dt);
    if (ball.done && !wasDone) {
      binCounts[ball.bin]++;
      settled = true;
    }
  }

  activeBalls = activeBalls.filter(b => !b.done);

  if (settled) {
    updateDistribution();
    updateStatus();
  }

  draw();
  requestAnimationFrame(animate);
}

// Drop a single ball
async function dropBall() {
  if (ballsRemaining <= 0) return;
  ballsRemaining--;

  const choices = [];
  for (let i = 0; i < ROWS; i++) {
    choices.push(Math.random() < 0.5);
  }

  const ball = new Ball(choices);

  // Try API (fire-and-forget for speed with many balls)
  if (instance) {
    try {
      for (const tid of ball.transitions) {
        fetch(window.API_BASE + '/api/' + tid, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ aggregate_id: instance })
        }).catch(() => {});
      }
    } catch (e) { /* local fallback */ }
  }

  activeBalls.push(ball);
  updateStatus();
}

// Drop all remaining balls with staggered animation
async function dropAll() {
  if (ballsRemaining <= 0) return;
  const count = ballsRemaining;
  const btn = document.getElementById('btn-drop-all');
  btn.disabled = true;
  document.getElementById('btn-drop').disabled = true;

  // Drop in rapid bursts for 256 balls
  const batchSize = Math.max(1, Math.floor(animSpeed));
  for (let i = 0; i < count; i++) {
    await dropBall();
    // Stagger: faster with more balls
    if (i % batchSize === 0) {
      await sleep(60 / animSpeed);
    }
  }

  btn.disabled = false;
  document.getElementById('btn-drop').disabled = false;
}

// Reset
window.resetBoard = function() {
  stopAuto();
  ballsRemaining = TOTAL_BALLS;
  binCounts = new Array(BINS).fill(0);
  activeBalls = [];
  instance = null;
  updateDistribution();
  updateStatus();
  initInstance();
};

// Auto-drop toggle
window.toggleAuto = function() {
  if (document.getElementById('auto-toggle').checked) {
    startAuto();
  } else {
    stopAuto();
  }
};

function startAuto() {
  stopAuto();
  autoTimer = setInterval(async () => {
    if (ballsRemaining > 0 && activeBalls.length < 8) {
      await dropBall();
    } else if (ballsRemaining <= 0 && activeBalls.length === 0) {
      stopAuto();
    }
  }, 200 / animSpeed);
}

function stopAuto() {
  if (autoTimer) {
    clearInterval(autoTimer);
    autoTimer = null;
  }
  const toggle = document.getElementById('auto-toggle');
  if (toggle) toggle.checked = false;
}

window.updateSpeed = function() {
  animSpeed = parseFloat(document.getElementById('speed-slider').value);
  document.getElementById('speed-label').textContent = animSpeed.toFixed(1) + 'x';
  if (autoTimer) startAuto();
};

// Update distribution chart
function updateDistribution() {
  const totalDropped = TOTAL_BALLS - ballsRemaining;
  const maxCount = Math.max(...binCounts, 1);
  const maxExpected = PASCAL[4]; // C(8,4) = 70, the peak

  const chartH = 120;

  for (let i = 0; i < BINS; i++) {
    const obs = document.getElementById('obs-' + i);
    const exp = document.getElementById('exp-' + i);
    const cnt = document.getElementById('count-' + i);
    if (!obs) continue;

    const obsHeight = (binCounts[i] / Math.max(maxCount, maxExpected)) * chartH;
    obs.style.height = obsHeight + 'px';

    const expected = totalDropped > 0 ? (PASCAL[i] / PASCAL_SUM) * totalDropped : 0;
    const expHeight = (expected / Math.max(maxCount, maxExpected)) * chartH;
    exp.style.height = expHeight + 'px';

    cnt.textContent = binCounts[i];
  }
}

function updateStatus() {
  document.getElementById('ball-count').textContent = `Balls remaining: ${ballsRemaining}`;
  document.getElementById('btn-drop').disabled = ballsRemaining <= 0;
  document.getElementById('btn-drop-all').disabled = ballsRemaining <= 0;
}

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

// Instance management
async function initInstance() {
  try {
    const res = await fetch(window.API_BASE + '/api/galtonboard', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    if (res.ok) {
      const data = await res.json();
      instance = data.aggregate_id;
    }
  } catch (e) { /* local mode */ }
  updateDistribution();
  updateStatus();
}

window.dropBall = dropBall;
window.dropAll = dropAll;

initInstance();
requestAnimationFrame(animate);
