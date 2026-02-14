// Predator-Prey (Lotka-Volterra) ODE Simulation
// Auto-runs on any parameter change

import * as Solver from 'https://cdn.jsdelivr.net/gh/pflow-xyz/pflow-xyz@latest/public/petri-solver.js'

let populationChart = null
let phaseChart = null
let animFrame = null

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

// Extract predictions from ODE solution
function computePredictions(solution, params) {
  if (!solution || !solution.t || !solution.u) {
    return { period: null, peakPrey: 0, peakPred: 0, amplitude: 0 }
  }

  const times = solution.t
  const preyVals = times.map((_, i) => Math.max(0, solution.u[i]['prey'] || 0))
  const predVals = times.map((_, i) => Math.max(0, solution.u[i]['predator'] || 0))

  // Peak populations
  const peakPrey = Math.max(...preyVals)
  const peakPred = Math.max(...predVals)

  // Equilibrium values
  const eqPrey = params.gamma / params.beta
  const eqPred = params.alpha / params.beta

  // Amplitude: max deviation of prey from equilibrium
  const amplitude = peakPrey - eqPrey

  // Oscillation period: find successive prey peaks
  const peaks = []
  for (let i = 2; i < preyVals.length - 2; i++) {
    if (preyVals[i] > preyVals[i - 1] && preyVals[i] > preyVals[i - 2] &&
        preyVals[i] > preyVals[i + 1] && preyVals[i] > preyVals[i + 2]) {
      // Avoid detecting the same peak twice
      if (peaks.length === 0 || (times[i] - peaks[peaks.length - 1]) > 1) {
        peaks.push(times[i])
      }
    }
  }

  let period = null
  if (peaks.length >= 2) {
    // Average period across detected peaks
    let totalPeriod = 0
    for (let i = 1; i < peaks.length; i++) {
      totalPeriod += peaks[i] - peaks[i - 1]
    }
    period = totalPeriod / (peaks.length - 1)
  }

  return { period, peakPrey, peakPred, amplitude, eqPrey, eqPred }
}

function updatePredictions(preds) {
  const periodEl = document.getElementById('pred-period')
  if (preds.period !== null) {
    periodEl.textContent = preds.period.toFixed(1)
  } else {
    periodEl.textContent = '< 2 cycles'
    periodEl.title = 'Increase simulation time to detect oscillation period'
  }

  document.getElementById('pred-peak-prey').textContent = Math.round(preds.peakPrey)
  document.getElementById('pred-peak-pred').textContent = Math.round(preds.peakPred)

  const ampEl = document.getElementById('pred-amplitude')
  if (preds.amplitude > 0.5) {
    ampEl.textContent = '\u00B1' + Math.round(preds.amplitude)
  } else {
    ampEl.textContent = 'stable'
  }

  // Update equilibrium in explainer
  if (preds.eqPrey !== undefined) {
    document.getElementById('eq-prey').textContent = Math.round(preds.eqPrey)
    document.getElementById('eq-pred').textContent = Math.round(preds.eqPred)
  }
}

// Render animals in the ecosystem SVG
function renderEcosystem(preyCount, predCount) {
  const maxAnimals = 12
  const prey = Math.min(maxAnimals, Math.max(0, Math.round(preyCount / 20)))
  const pred = Math.min(6, Math.max(0, Math.round(predCount / 10)))

  // Prey (rabbits) - scattered on the ground
  const preyGroup = document.getElementById('prey-group')
  if (preyGroup) {
    let html = ''
    for (let i = 0; i < prey; i++) {
      const x = 30 + (i % 6) * 38 + (Math.sin(i * 2.3) * 10)
      const y = 120 + Math.floor(i / 6) * 18 + (Math.cos(i * 1.7) * 6)
      html += `<text x="${x}" y="${y}" font-size="16" opacity="0.9">&#x1F407;</text>`
    }
    preyGroup.innerHTML = html
  }

  // Predators (foxes) - fewer, spread out
  const predGroup = document.getElementById('pred-group')
  if (predGroup) {
    let html = ''
    for (let i = 0; i < pred; i++) {
      const x = 50 + i * 45 + (Math.sin(i * 3.1) * 15)
      const y = 108 + (Math.cos(i * 2.1) * 10)
      html += `<text x="${x}" y="${y}" font-size="18">&#x1F43A;</text>`
    }
    predGroup.innerHTML = html
  }

  // Population labels
  const preyLabel = document.getElementById('prey-label')
  const predLabel = document.getElementById('pred-label')
  if (preyLabel) preyLabel.textContent = Math.round(preyCount) + ' prey'
  if (predLabel) predLabel.textContent = Math.round(predCount) + ' predators'

  // Phase indicator
  const phaseLabel = document.getElementById('phase-label')
  if (phaseLabel) {
    const eqPrey = parseFloat(document.getElementById('gamma').value) / parseFloat(document.getElementById('beta').value)
    const eqPred = parseFloat(document.getElementById('alpha').value) / parseFloat(document.getElementById('beta').value)
    if (preyCount > eqPrey * 1.1 && predCount < eqPred) {
      phaseLabel.textContent = 'prey boom \u2192 predators rising'
    } else if (predCount > eqPred * 1.1 && preyCount > eqPrey) {
      phaseLabel.textContent = 'predators booming \u2192 prey declining'
    } else if (preyCount < eqPrey * 0.9 && predCount > eqPred) {
      phaseLabel.textContent = 'prey scarce \u2192 predators starving'
    } else if (predCount < eqPred * 0.9 && preyCount < eqPrey) {
      phaseLabel.textContent = 'predators scarce \u2192 prey recovering'
    } else {
      phaseLabel.textContent = 'near equilibrium'
    }
  }

  // Counter display
  document.getElementById('prey-count').textContent = Math.round(preyCount)
  document.getElementById('pred-count').textContent = Math.round(predCount)
}

// Animate ecosystem through simulation time
function animateEcosystem(solution, params) {
  if (animFrame) cancelAnimationFrame(animFrame)
  if (!solution || !solution.t || !solution.u || solution.t.length === 0) return

  const duration = 4000
  const startTime = performance.now()

  function step(now) {
    const elapsed = now - startTime
    const progress = Math.min(1, elapsed / duration)
    const idx = Math.min(
      solution.t.length - 1,
      Math.floor(progress * (solution.t.length - 1))
    )
    const prey = Math.max(0, solution.u[idx]['prey'] || 0)
    const pred = Math.max(0, solution.u[idx]['predator'] || 0)
    renderEcosystem(prey, pred)

    if (progress < 1) {
      animFrame = requestAnimationFrame(step)
    }
  }

  animFrame = requestAnimationFrame(step)
}

function initCharts() {
  const popCtx = document.getElementById('population-chart').getContext('2d')
  populationChart = new Chart(popCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      interaction: { mode: 'index', intersect: false },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Time' }, min: 0 },
        y: { title: { display: true, text: 'Population' }, min: 0 }
      },
      plugins: {
        legend: { display: true, position: 'top' },
        annotation: { annotations: {} },
        tooltip: {
          callbacks: {
            label: (ctx) => `${ctx.dataset.label}: ${ctx.parsed.y.toFixed(1)}`
          }
        }
      }
    }
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
        legend: { display: true, position: 'top' },
        tooltip: {
          callbacks: {
            label: (ctx) => `Prey: ${ctx.parsed.x.toFixed(0)}, Pred: ${ctx.parsed.y.toFixed(0)}`
          }
        }
      }
    }
  })
}

function updateCharts(solution, params, preds) {
  if (!solution || !solution.t || !solution.u) return

  const times = solution.t
  const skip = Math.max(1, Math.floor(times.length / 600))
  const preyData = []
  const predData = []
  const phaseData = []

  for (let i = 0; i < times.length; i += skip) {
    const prey = Math.max(0, solution.u[i]['prey'] || 0)
    const pred = Math.max(0, solution.u[i]['predator'] || 0)
    preyData.push({ x: times[i], y: prey })
    predData.push({ x: times[i], y: pred })
    phaseData.push({ x: prey, y: pred })
  }

  // Equilibrium annotation lines
  const eqPrey = params.gamma / params.beta
  const eqPred = params.alpha / params.beta
  const annotations = {
    eqPreyLine: {
      type: 'line',
      yMin: eqPrey, yMax: eqPrey,
      borderColor: 'rgba(39, 174, 96, 0.4)',
      borderWidth: 1,
      borderDash: [6, 3],
      label: {
        display: true,
        content: 'prey eq: ' + Math.round(eqPrey),
        position: 'end',
        backgroundColor: 'rgba(39, 174, 96, 0.7)',
        color: 'white',
        font: { size: 10 }
      }
    },
    eqPredLine: {
      type: 'line',
      yMin: eqPred, yMax: eqPred,
      borderColor: 'rgba(231, 76, 60, 0.4)',
      borderWidth: 1,
      borderDash: [6, 3],
      label: {
        display: true,
        content: 'pred eq: ' + Math.round(eqPred),
        position: 'start',
        backgroundColor: 'rgba(231, 76, 60, 0.7)',
        color: 'white',
        font: { size: 10 }
      }
    }
  }

  populationChart.data.datasets = [
    {
      label: 'Prey',
      data: preyData,
      borderColor: '#27ae60',
      backgroundColor: 'rgba(39, 174, 96, 0.08)',
      borderWidth: 2,
      fill: true,
      tension: 0.3,
      pointRadius: 0
    },
    {
      label: 'Predators',
      data: predData,
      borderColor: '#e74c3c',
      backgroundColor: 'rgba(231, 76, 60, 0.08)',
      borderWidth: 2,
      fill: true,
      tension: 0.3,
      pointRadius: 0
    }
  ]
  populationChart.options.plugins.annotation.annotations = annotations
  populationChart.update()

  // Phase space
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
}

function runSimulation() {
  const params = getParams()
  const solution = runODE(params)
  if (solution) {
    const preds = computePredictions(solution, params)
    updatePredictions(preds)
    updateCharts(solution, params, preds)
    animateEcosystem(solution, params)
  }
}

function setupSliders() {
  const configs = [
    { id: 'alpha', suffix: '', decimals: 1 },
    { id: 'beta', suffix: '', decimals: 3 },
    { id: 'gamma', suffix: '', decimals: 1 },
    { id: 'prey0', suffix: '', decimals: 0 },
    { id: 'pred0', suffix: '', decimals: 0 },
    { id: 'tspan', suffix: '', decimals: 0 }
  ]

  configs.forEach(({ id, suffix, decimals }) => {
    const slider = document.getElementById(id)
    const display = document.getElementById(id + '-value')
    slider.addEventListener('input', () => {
      display.textContent = parseFloat(slider.value).toFixed(decimals) + suffix
      runSimulation()
    })
  })
}

// Module scripts are deferred, DOM is ready
initCharts()
setupSliders()
renderEcosystem(100, 20)
runSimulation()
