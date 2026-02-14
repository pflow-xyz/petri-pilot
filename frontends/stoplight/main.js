// Stoplight — cyclic state machine frontend

const PLACES = ['red', 'green', 'yellow'];
const TRANSITIONS = [
  { id: 'go',   label: 'Go',   from: 'red',    to: 'green',  desc: 'red → green' },
  { id: 'slow', label: 'Slow', from: 'green',  to: 'yellow', desc: 'green → yellow' },
  { id: 'stop', label: 'Stop', from: 'yellow', to: 'red',    desc: 'yellow → red' },
];

// Token positions for the SVG Petri net diagram
const TOKEN_POS = {
  red:    { cx: 100, cy: 40 },
  green:  { cx: 200, cy: 160 },
  yellow: { cx: 100, cy: 160 },
};

let state = { red: 1, green: 0, yellow: 0 };
let instance = null;
let autoTimer = null;
let cycleStep = 0;

// Determine which transition is enabled
function enabledTransition() {
  for (const t of TRANSITIONS) {
    if (state[t.from] > 0) return t;
  }
  return null;
}

function render() {
  // Traffic light
  for (const p of PLACES) {
    const el = document.getElementById('light-' + p);
    el.classList.toggle('active', state[p] > 0);
  }

  // Petri net token position
  const active = PLACES.find(p => state[p] > 0) || 'red';
  const pos = TOKEN_POS[active];
  const token = document.getElementById('token');
  token.setAttribute('cx', pos.cx);
  token.setAttribute('cy', pos.cy);

  // Token count labels
  for (const p of PLACES) {
    document.getElementById('t-' + p).textContent = state[p] > 0 ? '1' : '';
  }

  // Highlight active place in SVG
  for (const p of PLACES) {
    const circle = document.getElementById('p-' + p);
    circle.setAttribute('stroke-width', state[p] > 0 ? '3' : '2');
  }

  // Transition buttons
  const enabled = enabledTransition();
  const container = document.getElementById('transitions-container');
  container.innerHTML = '';
  for (const t of TRANSITIONS) {
    const btn = document.createElement('button');
    btn.className = 'trans-btn' + (enabled && enabled.id === t.id ? ' enabled' : '');
    btn.disabled = !(enabled && enabled.id === t.id);
    btn.textContent = t.label;
    btn.title = t.desc;
    btn.onclick = () => fireTransition(t.id);
    container.appendChild(btn);
  }
}

async function fireTransition(transitionId) {
  const t = TRANSITIONS.find(tr => tr.id === transitionId);
  if (!t || state[t.from] <= 0) return;

  // Try API if we have an instance
  if (instance) {
    try {
      const res = await fetch(window.API_BASE + '/api/' + transitionId, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ aggregate_id: instance })
      });
      if (res.ok) {
        const data = await res.json();
        if (data.success) {
          state = readState(data);
          addLog(t);
          render();
          return;
        }
      }
    } catch (e) {
      // Fall through to local simulation
    }
  }

  // Local simulation fallback
  state[t.from] = 0;
  state[t.to] = 1;
  addLog(t);
  render();
}

function addLog(t) {
  const log = document.getElementById('event-log');
  const entry = document.createElement('div');
  entry.className = 'log-entry ' + t.id;
  const ts = new Date().toLocaleTimeString();
  entry.textContent = `[${ts}] ${t.id}: ${t.desc}`;
  log.appendChild(entry);
  log.scrollTop = log.scrollHeight;

  // Keep log manageable
  while (log.children.length > 50) {
    log.removeChild(log.firstChild);
  }
}

function readState(data) {
  // Create returns token counts in "places", transitions return them in "state"
  const s = data.places || data.state || {};
  return { red: s.red || 0, green: s.green || 0, yellow: s.yellow || 0 };
}

async function initInstance() {
  try {
    const res = await fetch(window.API_BASE + '/api/stoplight', { method: 'POST', headers: { 'Content-Type': 'application/json' } });
    if (res.ok) {
      const data = await res.json();
      instance = data.aggregate_id;
      state = readState(data);
    }
  } catch (e) {
    // Local mode
  }
  render();
}

// Full cycle: go → slow → stop
const CYCLE_ORDER = ['go', 'slow', 'stop'];

async function autoCycle() {
  const btn = document.getElementById('btn-cycle');
  btn.disabled = true;
  for (const tid of CYCLE_ORDER) {
    await fireTransition(tid);
    await sleep(400);
  }
  btn.disabled = false;
}

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

window.resetState = function() {
  stopAuto();
  state = { red: 1, green: 0, yellow: 0 };
  instance = null;
  document.getElementById('event-log').innerHTML = '<div class="log-entry info">Reset. Ready.</div>';
  initInstance();
};

window.autoCycle = autoCycle;

// Auto-cycle toggle
window.toggleAuto = function() {
  const on = document.getElementById('auto-toggle').checked;
  if (on) {
    startAuto();
  } else {
    stopAuto();
  }
};

function startAuto() {
  stopAuto();
  const speed = parseInt(document.getElementById('speed-slider').value);
  autoTimer = setInterval(async () => {
    const t = enabledTransition();
    if (t) await fireTransition(t.id);
  }, speed);
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
  const val = parseInt(document.getElementById('speed-slider').value);
  document.getElementById('speed-label').textContent = (val / 1000).toFixed(1) + 's';
  if (autoTimer) startAuto(); // restart with new speed
};

// Initialize
initInstance();
