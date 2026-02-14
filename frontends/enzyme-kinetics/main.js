// Enzyme Kinetics - Michaelis-Menten ODE Simulation

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@latest/public/petri-solver.js'

let concChart = null
let rateChart = null

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
    // Rates must be passed explicitly
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
        legend: { display: false }
      }
    }
  })
}

function updateCharts(solution, params) {
  if (!solution || !solution.t || !solution.u) return

  const times = solution.t
  const subData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['substrate'] || 0) }))
  const enzData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['enzyme'] || 0) }))
  const cplxData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['complex'] || 0) }))
  const prodData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['product'] || 0) }))

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
  concChart.update()

  // Compute reaction rate: d[P]/dt using finite differences
  const rateData = []
  for (let i = 1; i < times.length; i++) {
    const dt = times[i] - times[i-1]
    if (dt === 0) continue
    const dP = (solution.u[i]['product'] || 0) - (solution.u[i-1]['product'] || 0)
    rateData.push({ x: times[i], y: Math.max(0, dP / dt) })
  }

  // Also show Vmax line
  const Vmax = params.kcat * params.e0
  const vmaxLine = [{ x: 0, y: Vmax }, { x: params.tspan, y: Vmax }]

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
  rateChart.update()

  // Update molecule displays
  const lastState = solution.u[solution.u.length - 1]
  document.getElementById('mol-substrate').textContent = Math.round(Math.max(0, lastState['substrate'] || 0))
  document.getElementById('mol-enzyme').textContent = Math.round(Math.max(0, lastState['enzyme'] || 0))
  document.getElementById('mol-complex').textContent = Math.round(Math.max(0, lastState['complex'] || 0))
  document.getElementById('mol-product').textContent = Math.round(Math.max(0, lastState['product'] || 0))
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
      updateMM(getParams())
    })
  })
}

window.runSimulation = function() {
  const params = getParams()
  const solution = runODE(params)
  if (solution) {
    updateCharts(solution, params)
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

document.addEventListener('DOMContentLoaded', () => {
  initCharts()
  setupSliders()
  runSimulation()
})
