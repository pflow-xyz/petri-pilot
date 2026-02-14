// Predator-Prey (Lotka-Volterra) ODE Simulation
// Uses pflow ODE solver for continuous dynamics

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@latest/public/petri-solver.js'

let populationChart = null
let phaseChart = null

// Build Petri net model from current parameters
function buildModel(alpha, beta, gamma, prey0, pred0) {
  return {
    '@context': 'https://pflow.xyz/schema',
    '@type': 'PetriNet',
    'places': {
      'prey': { '@type': 'Place', 'initial': [prey0], 'x': 200, 'y': 200 },
      'predator': { '@type': 'Place', 'initial': [pred0], 'x': 600, 'y': 200 }
    },
    'transitions': {
      'prey_reproduce': { '@type': 'Transition', 'rate': alpha, 'x': 200, 'y': 50 },
      'predation': { '@type': 'Transition', 'rate': beta, 'x': 400, 'y': 200 },
      'predator_death': { '@type': 'Transition', 'rate': gamma, 'x': 600, 'y': 50 }
    },
    'arcs': [
      { '@type': 'Arrow', 'source': 'prey', 'target': 'prey_reproduce', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'prey_reproduce', 'target': 'prey', 'weight': [2] },
      { '@type': 'Arrow', 'source': 'prey', 'target': 'predation', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'predator', 'target': 'predation', 'weight': [1] },
      { '@type': 'Arrow', 'source': 'predation', 'target': 'predator', 'weight': [2] },
      { '@type': 'Arrow', 'source': 'predator', 'target': 'predator_death', 'weight': [1] }
    ]
  }
}

function getParams() {
  return {
    alpha: parseFloat(document.getElementById('alpha').value),
    beta: parseFloat(document.getElementById('beta').value),
    gamma: parseFloat(document.getElementById('gamma').value),
    prey0: parseInt(document.getElementById('prey0').value),
    pred0: parseInt(document.getElementById('pred0').value),
    tspan: parseInt(document.getElementById('tspan').value)
  }
}

function runODE(params) {
  const model = buildModel(params.alpha, params.beta, params.gamma, params.prey0, params.pred0)
  try {
    const net = Solver.fromJSON(model)
    const initialState = Solver.setState(net)
    // Rates must be passed explicitly - fromJSON doesn't preserve them
    const rates = {
      'prey_reproduce': params.alpha,
      'predation': params.beta,
      'predator_death': params.gamma
    }
    const prob = new Solver.ODEProblem(net, initialState, [0, params.tspan], rates)
    const solution = Solver.solve(prob, Solver.Tsit5(), { dt: 0.05, adaptive: false })
    return solution
  } catch (err) {
    console.error('ODE solve error:', err)
    return null
  }
}

function createChartOptions(xLabel, yLabel, xIsLinear) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 0 },
    interaction: { mode: 'index', intersect: false },
    scales: {
      x: {
        type: 'linear',
        title: { display: true, text: xLabel },
        min: 0
      },
      y: {
        title: { display: true, text: yLabel },
        min: 0,
        beginAtZero: true
      }
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
}

function initCharts() {
  const popCtx = document.getElementById('population-chart').getContext('2d')
  populationChart = new Chart(popCtx, {
    type: 'line',
    data: { datasets: [] },
    options: createChartOptions('Time', 'Population')
  })

  const phaseCtx = document.getElementById('phase-chart').getContext('2d')
  phaseChart = new Chart(phaseCtx, {
    type: 'scatter',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      scales: {
        x: { title: { display: true, text: 'Prey' }, min: 0 },
        y: { title: { display: true, text: 'Predators' }, min: 0 }
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) => `Prey: ${ctx.parsed.x.toFixed(1)}, Pred: ${ctx.parsed.y.toFixed(1)}`
          }
        }
      }
    }
  })
}

function updateCharts(solution, params) {
  if (!solution || !solution.t || !solution.u) return

  const times = solution.t
  const preyData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['prey'] || 0) }))
  const predData = times.map((t, i) => ({ x: t, y: Math.max(0, solution.u[i]['predator'] || 0) }))

  // Population chart
  populationChart.data.datasets = [
    {
      label: 'Prey',
      data: preyData,
      borderColor: '#27ae60',
      backgroundColor: 'rgba(39, 174, 96, 0.1)',
      borderWidth: 2,
      fill: true,
      tension: 0.3,
      pointRadius: 0
    },
    {
      label: 'Predators',
      data: predData,
      borderColor: '#e74c3c',
      backgroundColor: 'rgba(231, 76, 60, 0.1)',
      borderWidth: 2,
      fill: true,
      tension: 0.3,
      pointRadius: 0
    }
  ]
  populationChart.update()

  // Phase space chart
  const phaseData = times.map((t, i) => ({
    x: Math.max(0, solution.u[i]['prey'] || 0),
    y: Math.max(0, solution.u[i]['predator'] || 0)
  }))

  // Equilibrium point
  const eqPrey = params.gamma / params.beta
  const eqPred = params.alpha / params.beta

  phaseChart.data.datasets = [
    {
      label: 'Trajectory',
      data: phaseData,
      borderColor: '#8e44ad',
      backgroundColor: 'rgba(142, 68, 173, 0.05)',
      showLine: true,
      borderWidth: 2,
      pointRadius: 0,
      tension: 0.1,
      fill: false
    },
    {
      label: 'Start',
      data: [{ x: params.prey0, y: params.pred0 }],
      borderColor: '#2ecc71',
      backgroundColor: '#2ecc71',
      pointRadius: 8,
      pointStyle: 'circle',
      showLine: false
    },
    {
      label: 'Equilibrium',
      data: [{ x: eqPrey, y: eqPred }],
      borderColor: '#e67e22',
      backgroundColor: '#e67e22',
      pointRadius: 8,
      pointStyle: 'crossRot',
      showLine: false
    }
  ]
  phaseChart.update()

  // Update population display with final values
  const lastState = solution.u[solution.u.length - 1]
  document.getElementById('prey-count').textContent = Math.round(Math.max(0, lastState['prey'] || 0))
  document.getElementById('pred-count').textContent = Math.round(Math.max(0, lastState['predator'] || 0))
}

function updateEquilibrium(params) {
  const eqPrey = params.gamma / params.beta
  const eqPred = params.alpha / params.beta
  document.getElementById('eq-prey').textContent = Math.round(eqPrey)
  document.getElementById('eq-pred').textContent = Math.round(eqPred)
}

// Attach slider listeners
function setupSliders() {
  const sliders = ['alpha', 'beta', 'gamma', 'prey0', 'pred0', 'tspan']
  sliders.forEach(id => {
    const slider = document.getElementById(id)
    const display = document.getElementById(id.replace('prey0', 'prey0').replace('pred0', 'pred0') + '-value')
    slider.addEventListener('input', () => {
      display.textContent = slider.value
      const params = getParams()
      updateEquilibrium(params)
    })
  })
}

window.runSimulation = function() {
  const params = getParams()
  const solution = runODE(params)
  if (solution) {
    updateCharts(solution, params)
    updateEquilibrium(params)
  }
}

window.resetParams = function() {
  document.getElementById('alpha').value = 1.0
  document.getElementById('beta').value = 0.01
  document.getElementById('gamma').value = 0.5
  document.getElementById('prey0').value = 100
  document.getElementById('pred0').value = 20
  document.getElementById('tspan').value = 30

  document.getElementById('alpha-value').textContent = '1.0'
  document.getElementById('beta-value').textContent = '0.01'
  document.getElementById('gamma-value').textContent = '0.5'
  document.getElementById('prey0-value').textContent = '100'
  document.getElementById('pred0-value').textContent = '20'
  document.getElementById('tspan-value').textContent = '30'

  runSimulation()
}

document.addEventListener('DOMContentLoaded', () => {
  initCharts()
  setupSliders()
  runSimulation()
})
