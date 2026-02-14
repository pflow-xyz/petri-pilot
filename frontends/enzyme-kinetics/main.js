// Enzyme Kinetics - Michaelis-Menten ODE Simulation
// Auto-runs simulation on any parameter change

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@latest/public/petri-solver.js'

let concChart = null
let rateChart = null
let animFrame = null

function buildModel(k1, km1, kcat, s0, e0) {
  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    'places': {
      'substrate': { '@type': 'Place', 'initial': [s0], 'x': 100, 'y': 200 },
      'enzyme': { '@type': 'Place', 'initial': [e0], 'x': 300, 'y': 50 },
      'complex': { '@type': 'Place', 'initial': [0], 'x': 500, 'y': 200 },
      'product': { '@type': 'Place', 'initial': [0], 'x': 700, 'y': 200 }
    },
    'transitions': {
      'bind': { '@type': 'Transition', 'rate': k1, 'x': 300, 'y': 200 },
      'unbind': { '@type': 'Transition', 'rate': km1, 'x': 400, 'y': 50 },
      'catalyze': { '@type': 'Transition', 'rate': kcat, 'x': 600, 'y': 200 }
    },
    'arcs': [
      { '@type': 'Arrow', 'source': 'substrate', 'target': 'bind', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'enzyme', 'target': 'bind', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'bind', 'target': 'complex', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'complex', 'target': 'unbind', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'unbind', 'target': 'substrate', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'unbind', 'target': 'enzyme', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'complex', 'target': 'catalyze', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'catalyze', 'target': 'product', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'catalyze', 'target': 'enzyme', 'weight': [1] }
    ]
  }
}

function getParams() {
  return {
    k1: parseFloat(document.getElementById('k1').value),
    km1: parseFloat(document.getElementById('km1').value),
    kcat: parseFloat(document.getElementById('kcat').value),
    s0: parseInt(document.getElementById('s0').value),
    e0: parseInt(document.getElementById('e0').value),
    tspan: parseInt(document.getElementById('tspan').value)
  }
}

function runODE(params) {
  const model = buildModel(params.k1, params.km1, params.kcat, params.s0, params.e0)
  try {
    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    const rates = {
      'bind': params.k1,
      'unbind': params.km1,
      'catalyze': params.kcat
    }
    const prob = new Solver.ODEProblem(net, initialState, [0, params.tspan], rates)
    const solution = Solver.solve(prob, Solver.Tsit5(), { dt: 0.05, adaptive: false })
    return solution
  } catch (err) {
    console.error('ODE solve error:', err)
    return null
  }
}

// Extract predictions from simulation results
function computePredictions(solution, params) {
  if (!solution || !solution.t || !solution.u) {
    return { halfTime: null, peakRate: 0, peakRateTime: 0, finalYield: 0, Km: 0, Vmax: 0 }
  }

  const times = solution.t
  const halfTarget = params.s0 * 0.5

  // Time to 50% substrate conversion
  let halfTime = null
  for (let i = 0; i < times.length; i++) {
    const s = solution.u[i]['substrate'] || 0
    if (s <= halfTarget) {
      halfTime = times[i]
      break
    }
  }

  // Peak reaction rate (d[P]/dt)
  let peakRate = 0
  let peakRateTime = 0
  for (let i = 1; i < times.length; i++) {
    const dt = times[i] - times[i - 1]
    if (dt === 0) continue
    const dP = (solution.u[i]['product'] || 0) - (solution.u[i - 1]['product'] || 0)
    const rate = Math.max(0, dP / dt)
    if (rate > peakRate) {
      peakRate = rate
      peakRateTime = times[i]
    }
  }

  // Final yield
  const lastState = solution.u[solution.u.length - 1]
  const finalProduct = Math.max(0, lastState['product'] || 0)
  const finalYield = params.s0 > 0 ? (finalProduct / params.s0) * 100 : 0

  // Michaelis-Menten constants
  const Km = (params.km1 + params.kcat) / params.k1
  const Vmax = params.kcat * params.e0

  return { halfTime, peakRate, peakRateTime, finalYield, Km, Vmax }
}

function updatePredictions(preds) {
  const halfEl = document.getElementById('pred-half')
  if (preds.halfTime !== null) {
    halfEl.textContent = preds.halfTime.toFixed(1) + 's'
  } else {
    halfEl.textContent = '>' + document.getElementById('tspan').value + 's'
  }

  document.getElementById('pred-peak-rate').textContent = preds.peakRate.toFixed(2)
  document.getElementById('pred-yield').textContent = preds.finalYield.toFixed(0) + '%'
  document.getElementById('pred-km-vmax').textContent =
    preds.Km.toFixed(0) + ' / ' + preds.Vmax.toFixed(1)
}

function initCharts() {
  const concCtx = document.getElementById('conc-chart').getContext('2d')
  concChart = new Chart(concCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      interaction: { mode: 'index', intersect: false },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Time' }, min: 0 },
        y: { title: { display: true, text: 'Concentration' }, min: 0 }
      },
      plugins: {
        legend: { display: false },
        annotation: { annotations: {} },
        tooltip: {
          callbacks: {
            label: (ctx) => `${ctx.dataset.label}: ${ctx.parsed.y.toFixed(1)}`
          }
        }
      }
    }
  })

  const rateCtx = document.getElementById('rate-chart').getContext('2d')
  rateChart = new Chart(rateCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Time' }, min: 0 },
        y: { title: { display: true, text: 'Reaction Rate (d[P]/dt)' }, min: 0 }
      },
      plugins: {
        legend: { display: false },
        annotation: { annotations: {} }
      }
    }
  })
}

function updateCharts(solution, params, preds) {
  if (!solution || !solution.t || !solution.u) return

  const times = solution.t
  const subData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['substrate'] || 0) }))
  const enzData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['enzyme'] || 0) }))
  const cplxData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['complex'] || 0) }))
  const prodData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['product'] || 0) }))

  // Annotation for 50% conversion
  const annotations = {}
  if (preds.halfTime !== null) {
    annotations.halfLine = {
      type: 'line',
      xMin: preds.halfTime,
      xMax: preds.halfTime,
      borderColor: 'rgba(46, 204, 113, 0.7)',
      borderWidth: 2,
      borderDash: [6, 3],
      label: {
        display: true,
        content: '50% at ' + preds.halfTime.toFixed(1) + 's',
        position: 'start',
        backgroundColor: 'rgba(46, 204, 113, 0.85)',
        color: 'white',
        font: { size: 11, weight: 'bold' }
      }
    }
  }

  concChart.data.datasets = [
    {
      label: 'Substrate [S]',
      data: subData,
      borderColor: '#3498db',
      backgroundColor: 'rgba(52, 152, 219, 0.1)',
      borderWidth: 2, fill: false, tension: 0.3, pointRadius: 0
    },
    {
      label: 'Enzyme [E]',
      data: enzData,
      borderColor: '#27ae60',
      backgroundColor: 'rgba(39, 174, 96, 0.1)',
      borderWidth: 2, fill: false, tension: 0.3, pointRadius: 0
    },
    {
      label: 'Complex [ES]',
      data: cplxData,
      borderColor: '#e67e22',
      backgroundColor: 'rgba(230, 126, 34, 0.1)',
      borderWidth: 2, fill: false, tension: 0.3, pointRadius: 0
    },
    {
      label: 'Product [P]',
      data: prodData,
      borderColor: '#e74c3c',
      backgroundColor: 'rgba(231, 76, 60, 0.1)',
      borderWidth: 2, fill: true, tension: 0.3, pointRadius: 0
    }
  ]
  concChart.options.plugins.annotation.annotations = annotations
  concChart.update()

  // Compute reaction rate: d[P]/dt using finite differences
  const rateData = []
  for (let i = 1; i < times.length; i++) {
    const dt = times[i] - times[i - 1]
    if (dt === 0) continue
    const dP = (solution.u[i]['product'] || 0) - (solution.u[i - 1]['product'] || 0)
    rateData.push({ x: times[i], y: Math.max(0, dP / dt) })
  }

  // Vmax line
  const Vmax = params.kcat * params.e0
  const vmaxLine = [{ x: 0, y: Vmax }, { x: params.tspan, y: Vmax }]

  // Annotation for peak rate
  const rateAnnotations = {}
  if (preds.peakRateTime > 0) {
    rateAnnotations.peakLine = {
      type: 'line',
      xMin: preds.peakRateTime,
      xMax: preds.peakRateTime,
      borderColor: 'rgba(142, 68, 173, 0.6)',
      borderWidth: 2,
      borderDash: [6, 3],
      label: {
        display: true,
        content: 'Peak: ' + preds.peakRate.toFixed(2),
        position: 'start',
        backgroundColor: 'rgba(142, 68, 173, 0.85)',
        color: 'white',
        font: { size: 11, weight: 'bold' }
      }
    }
  }

  rateChart.data.datasets = [
    {
      label: 'Reaction Rate',
      data: rateData,
      borderColor: '#8e44ad',
      backgroundColor: 'rgba(142, 68, 173, 0.1)',
      borderWidth: 2, fill: true, tension: 0.3, pointRadius: 0
    },
    {
      label: 'Vmax',
      data: vmaxLine,
      borderColor: '#e74c3c',
      backgroundColor: 'transparent',
      borderWidth: 1.5, borderDash: [6, 3], fill: false, pointRadius: 0
    }
  ]
  rateChart.options.plugins.annotation.annotations = rateAnnotations
  rateChart.update()
}

// Animate molecule boxes through simulation time
function animateMolecules(solution, params) {
  if (animFrame) cancelAnimationFrame(animFrame)
  if (!solution || !solution.t || !solution.u || solution.u.length === 0) return

  const duration = 3000 // 3 second animation
  const startTime = performance.now()

  const molSub = document.getElementById('mol-substrate')
  const molEnz = document.getElementById('mol-enzyme')
  const molCplx = document.getElementById('mol-complex')
  const molProd = document.getElementById('mol-product')

  // Compute bar fill percentages (relative to max possible = s0 + e0)
  const maxConc = params.s0

  function step(now) {
    const elapsed = now - startTime
    const progress = Math.min(1, elapsed / duration)
    const idx = Math.min(
      solution.u.length - 1,
      Math.floor(progress * (solution.u.length - 1))
    )

    const state = solution.u[idx]
    const s = Math.max(0, state['substrate'] || 0)
    const e = Math.max(0, state['enzyme'] || 0)
    const c = Math.max(0, state['complex'] || 0)
    const p = Math.max(0, state['product'] || 0)

    molSub.textContent = Math.round(s)
    molEnz.textContent = Math.round(e)
    molCplx.textContent = Math.round(c)
    molProd.textContent = Math.round(p)

    // Update fill bars
    updateMolFill('mol-box-substrate', s / maxConc)
    updateMolFill('mol-box-enzyme', e / params.e0)
    updateMolFill('mol-box-complex', c / params.e0)
    updateMolFill('mol-box-product', p / maxConc)

    if (progress < 1) {
      animFrame = requestAnimationFrame(step)
    }
  }

  animFrame = requestAnimationFrame(step)
}

function updateMolFill(id, fraction) {
  const el = document.getElementById(id)
  if (!el) return
  const pct = Math.max(0, Math.min(100, fraction * 100))
  el.style.setProperty('--fill-pct', pct + '%')
}

function updateMM(params) {
  const Km = (params.km1 + params.kcat) / params.k1
  const Vmax = params.kcat * params.e0
  document.getElementById('km-value').textContent = Km.toFixed(1)
  document.getElementById('vmax-value').textContent = Vmax.toFixed(1)
}

function setupSliders() {
  const sliders = ['k1', 'km1', 'kcat', 's0', 'e0', 'tspan']
  sliders.forEach(id => {
    const slider = document.getElementById(id)
    const display = document.getElementById(id + '-value')
    slider.addEventListener('input', () => {
      display.textContent = slider.value
      runSimulation()
    })
  })
}

window.runSimulation = function() {
  const params = getParams()
  const solution = runODE(params)
  if (solution) {
    const preds = computePredictions(solution, params)
    updatePredictions(preds)
    updateCharts(solution, params, preds)
    animateMolecules(solution, params)
    updateMM(params)
  }
}

window.resetParams = function() {
  document.getElementById('k1').value = 0.01
  document.getElementById('km1').value = 0.1
  document.getElementById('kcat').value = 0.5
  document.getElementById('s0').value = 100
  document.getElementById('e0').value = 10
  document.getElementById('tspan').value = 30

  document.getElementById('k1-value').textContent = '0.01'
  document.getElementById('km1-value').textContent = '0.1'
  document.getElementById('kcat-value').textContent = '0.5'
  document.getElementById('s0-value').textContent = '100'
  document.getElementById('e0-value').textContent = '10'
  document.getElementById('tspan-value').textContent = '30'

  runSimulation()
}

// Module scripts are deferred, DOM is ready
initCharts()
setupSliders()
runSimulation()
