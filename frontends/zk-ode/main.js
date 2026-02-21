// ZK-ODE Frontend: Tsit5 solver simulation + proof generation

const SCALE = 1000000000000000000n; // 10^18

// On-chain contract addresses (Base Sepolia)
const ZKODE_ADDRESS = '0x2084d59f9797d96ddAA3BaE2E38745D2a5D0f6F8';
const VERIFIER_ADDRESS = '0xA675a162C5097e5eBa2968C918D4D0530b7005Ae';
const RPC_URL = 'https://sepolia.base.org';
const BASESCAN = 'https://sepolia.basescan.org';

// Tsit5 Butcher tableau (scaled to fixed-point integers for display accuracy).
// We use float64 for the browser simulation and compare with the ZK prover.
const B = [
  0.09646076681806523, 0.01, 0.4798896504144996,
  1.379008574103742, -3.290069515436081, 2.324710524099774, 0,
];
const A = [
  [],
  [0.161],
  [-0.008480655492356924, 0.335480655492357],
  [2.8971530571054935, -6.359448489975075, 4.362295432869581],
  [5.325864828439257, -11.748883564062828, 7.4955393428898365, -0.09249506636175525],
  [5.86145544294642, -12.92096931784711, 8.159367898576159, -0.071584973281401, -0.028269050394068383],
  [0.09646076681806523, 0.01, 0.4798896504144996, 1.379008574103742, -3.290069515436081, 2.324710524099774],
];

// Stoichiometry: S[place][transition]
const S = [
  [-1,  0], // A
  [+1, -1], // B
  [ 0, +1], // C
];
const INPUT_PLACES = [0, 1]; // t0 depends on A, t1 depends on B

// ODE derivative: f(y, rates) = S * v(y)
function derivative(y, rates) {
  const massRates = INPUT_PLACES.map((p, t) => rates[t] * y[p]);
  return S.map(row => row.reduce((sum, s, t) => sum + s * massRates[t], 0));
}

// One Tsit5 step
function tsit5Step(y, h, rates) {
  const nPlaces = y.length;
  const k = Array.from({ length: 7 }, () => new Float64Array(nPlaces));

  for (let stage = 0; stage < 7; stage++) {
    const yStage = [...y];
    for (let j = 0; j < A[stage].length; j++) {
      const hA = h * A[stage][j];
      for (let p = 0; p < nPlaces; p++) {
        yStage[p] += hA * k[j][p];
      }
    }
    const dy = derivative(yStage, rates);
    for (let p = 0; p < nPlaces; p++) {
      k[stage][p] = dy[p];
    }
  }

  const yNext = [...y];
  for (let j = 0; j < 7; j++) {
    if (B[j] === 0) continue;
    const hB = h * B[j];
    for (let p = 0; p < nPlaces; p++) {
      yNext[p] += hB * k[j][p];
    }
  }
  return yNext;
}

// Simple hash display (not real MiMC, just for visualization)
function simpleHash(values) {
  let hash = 0n;
  for (const v of values) {
    const scaled = BigInt(Math.round(v * Number(SCALE)));
    hash = hash ^ (scaled * 0x100000001B3n); // FNV-like
    hash = ((hash >> 128n) ^ hash) & ((1n << 256n) - 1n);
  }
  return '0x' + hash.toString(16).padStart(8, '0').slice(0, 16) + '...';
}

// State
let history = [];
let currentState = [1, 0, 0];
let currentStep = 0;

// DOM elements
const $tokensA = document.getElementById('tokens-a');
const $tokensB = document.getElementById('tokens-b');
const $tokensC = document.getElementById('tokens-c');
const $stepNumber = document.getElementById('step-number');
const $timeValue = document.getElementById('time-value');
const $conservation = document.getElementById('conservation');
const $stateRoot = document.getElementById('state-root');
const $chainContainer = document.getElementById('chain-container');
const $proofSection = document.getElementById('proof-section');
const $proofStatus = document.getElementById('proof-status');
const $proofData = document.getElementById('proof-data');
const $chart = document.getElementById('chart');
const ctx = $chart.getContext('2d');

function updateDisplay(y, step, h) {
  $tokensA.textContent = y[0].toFixed(3);
  $tokensB.textContent = y[1].toFixed(3);
  $tokensC.textContent = y[2].toFixed(3);
  $stepNumber.textContent = step;
  $timeValue.textContent = (step * h).toFixed(3);
  $conservation.textContent = (y[0] + y[1] + y[2]).toFixed(6);
  $stateRoot.textContent = simpleHash(y);

  // Update circle opacity based on token count
  document.getElementById('place-a').querySelector('.place-circle').style.opacity = 0.3 + 0.7 * y[0];
  document.getElementById('place-b').querySelector('.place-circle').style.opacity = 0.3 + 0.7 * Math.min(y[1] * 3, 1);
  document.getElementById('place-c').querySelector('.place-circle').style.opacity = 0.3 + 0.7 * y[2];
}

function drawChart() {
  const W = $chart.width = $chart.clientWidth * (window.devicePixelRatio || 1);
  const H = $chart.height = 300 * (window.devicePixelRatio || 1);
  const pad = { top: 20, right: 20, bottom: 30, left: 50 };
  const plotW = W - pad.left - pad.right;
  const plotH = H - pad.top - pad.bottom;

  ctx.clearRect(0, 0, W, H);
  ctx.save();
  ctx.scale(window.devicePixelRatio || 1, window.devicePixelRatio || 1);

  const w = $chart.clientWidth;
  const h = 300;
  const pW = w - pad.left - pad.right;
  const pH = h - pad.top - pad.bottom;

  // Axes
  ctx.strokeStyle = '#2d2d4a';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(pad.left, pad.top);
  ctx.lineTo(pad.left, pad.top + pH);
  ctx.lineTo(pad.left + pW, pad.top + pH);
  ctx.stroke();

  // Labels
  ctx.fillStyle = '#6b7280';
  ctx.font = '11px -apple-system, sans-serif';
  ctx.textAlign = 'center';
  ctx.fillText('Time', pad.left + pW / 2, h - 5);
  ctx.save();
  ctx.translate(12, pad.top + pH / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText('Tokens', 0, 0);
  ctx.restore();

  if (history.length < 2) {
    ctx.restore();
    return;
  }

  const maxT = history.length - 1;
  const colors = ['#818cf8', '#34d399', '#f59e0b']; // A, B, C
  const labels = ['A', 'B', 'C'];

  for (let p = 0; p < 3; p++) {
    ctx.strokeStyle = colors[p];
    ctx.lineWidth = 2;
    ctx.beginPath();
    for (let i = 0; i < history.length; i++) {
      const x = pad.left + (i / maxT) * pW;
      const y = pad.top + (1 - history[i][p]) * pH;
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.stroke();

    // Legend
    const lx = pad.left + pW - 60;
    const ly = pad.top + 15 + p * 18;
    ctx.fillStyle = colors[p];
    ctx.fillRect(lx, ly - 4, 12, 3);
    ctx.fillStyle = '#9ca3af';
    ctx.font = '11px -apple-system, sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText(labels[p], lx + 16, ly);
  }

  // Tick marks
  ctx.fillStyle = '#6b7280';
  ctx.textAlign = 'center';
  ctx.font = '10px -apple-system, sans-serif';
  const stepH = parseFloat(document.getElementById('step-size').value);
  for (let i = 0; i <= 5; i++) {
    const t = (maxT * stepH * i / 5).toFixed(1);
    const x = pad.left + (i / 5) * pW;
    ctx.fillText(t, x, pad.top + pH + 15);
  }
  ctx.textAlign = 'right';
  for (let i = 0; i <= 4; i++) {
    const v = (i / 4).toFixed(1);
    const y = pad.top + (1 - i / 4) * pH;
    ctx.fillText(v, pad.left - 8, y + 4);
  }

  ctx.restore();
}

function updateChain() {
  $chainContainer.innerHTML = '';
  const maxShow = 12;
  const start = Math.max(0, history.length - maxShow);

  for (let i = start; i < history.length; i++) {
    if (i > start) {
      const arrow = document.createElement('div');
      arrow.className = 'chain-arrow';
      arrow.textContent = '\u2192';
      $chainContainer.appendChild(arrow);
    }

    const block = document.createElement('div');
    block.className = 'chain-block';
    block.innerHTML = `
      <div class="step-num">Step ${i}</div>
      <div class="hash-val">${simpleHash(history[i])}</div>
    `;
    $chainContainer.appendChild(block);
  }

  $chainContainer.scrollLeft = $chainContainer.scrollWidth;
}

// Run simulation
document.getElementById('btn-run').addEventListener('click', () => {
  const h = parseFloat(document.getElementById('step-size').value);
  const k0 = parseFloat(document.getElementById('rate-0').value);
  const k1 = parseFloat(document.getElementById('rate-1').value);
  const nSteps = parseInt(document.getElementById('num-steps').value);
  const rates = [k0, k1];

  // Reset
  currentState = [1, 0, 0];
  currentStep = 0;
  history = [[...currentState]];

  // Animate steps
  const stepsPerFrame = Math.max(1, Math.floor(nSteps / 200));
  let step = 0;

  function animate() {
    for (let i = 0; i < stepsPerFrame && step < nSteps; i++, step++) {
      currentState = tsit5Step(currentState, h, rates);
      currentStep++;
      history.push([...currentState]);
    }

    updateDisplay(currentState, currentStep, h);
    drawChart();
    updateChain();

    if (step < nSteps) {
      requestAnimationFrame(animate);
    } else {
      document.getElementById('btn-prove').disabled = false;
    }
  }

  animate();
});

// Generate proof (calls backend API)
document.getElementById('btn-prove').addEventListener('click', async () => {
  $proofSection.style.display = '';
  $proofStatus.textContent = 'Generating proof via ZK prover...';
  $proofData.textContent = '';

  try {
    const h = parseFloat(document.getElementById('step-size').value);
    const k0 = parseFloat(document.getElementById('rate-0').value);
    const k1 = parseFloat(document.getElementById('rate-1').value);

    const resp = await fetch('api/prove', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        step_size: h,
        rates: [k0, k1],
        initial_marking: [1, 0, 0],
        num_steps: 1,
      }),
    });

    const contentType = resp.headers.get('content-type') || '';
    if (!contentType.includes('application/json')) {
      throw new Error('ZK prover backend is not running');
    }

    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || resp.statusText);
    }

    $proofStatus.textContent = 'Proof generated and verified!';
    $proofData.textContent = JSON.stringify(data, null, 2);
    fetchOnchainState(); // Refresh on-chain state after proof
  } catch (err) {
    $proofStatus.textContent = 'ZK prover backend is not running';
    $proofData.textContent = 'Proof generation requires the ZK prover backend.\n\n' +
      'The simulation above runs locally in the browser.\n' +
      'To generate real Groth16 proofs, start the prover service:\n\n' +
      '  go run ./cmd/zk-ode/...';
  }
});

// On-chain state via raw JSON-RPC eth_call
async function ethCall(to, selector) {
  const resp = await fetch(RPC_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'eth_call',
      params: [{ to, data: selector }, 'latest'],
    }),
  });
  const json = await resp.json();
  if (json.error) throw new Error(json.error.message);
  return json.result;
}

async function fetchOnchainState() {
  const $card = document.getElementById('onchain-card');
  const $status = document.getElementById('onchain-status');
  const $stepCount = document.getElementById('onchain-step-count');
  const $enforce = document.getElementById('onchain-enforce');
  const $root = document.getElementById('onchain-root');

  if (!$card) return;

  try {
    $status.textContent = 'Fetching...';
    $status.className = 'onchain-status loading';

    const [rootHex, stepHex, enforceHex] = await Promise.all([
      ethCall(ZKODE_ADDRESS, '0xac2eba98'), // currentStateRoot()
      ethCall(ZKODE_ADDRESS, '0x415deffa'), // stepCount()
      ethCall(ZKODE_ADDRESS, '0x51fef09f'), // enforceOptimal()
    ]);

    const stepCount = parseInt(stepHex, 16);
    const enforceOptimal = parseInt(enforceHex, 16) === 1;
    const rootShort = rootHex.slice(0, 18) + '...' + rootHex.slice(-8);

    $stepCount.textContent = stepCount;
    $enforce.textContent = enforceOptimal ? 'Yes' : 'No';
    $root.textContent = rootShort;
    $root.title = rootHex;
    $status.textContent = 'Connected';
    $status.className = 'onchain-status connected';
  } catch (err) {
    $status.textContent = 'Unable to connect';
    $status.className = 'onchain-status error';
    $stepCount.textContent = '-';
    $enforce.textContent = '-';
    $root.textContent = 'RPC unavailable';
  }
}

// Fetch on-chain state on page load
fetchOnchainState();

// Initial display
updateDisplay(currentState, 0, 0.01);
drawChart();
