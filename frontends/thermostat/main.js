// Thermostat Controller - ODE Prediction with bang-bang control
// Auto-runs simulation on any parameter change

let tempChart = null
let heaterChart = null
let animFrame = null
let simResult = null

function getParams() {
  return {
    heatRate: parseFloat(document.getElementById('heat-rate').value),
    coolRate: parseFloat(document.getElementById('cool-rate').value),
    target: parseInt(document.getElementById('target').value),
    initial: parseInt(document.getElementById('initial').value),
    tspan: parseInt(document.getElementById('tspan').value)
  }
}

// Bang-bang controller simulation with Euler integration
function simulate(params) {
  const dt = 0.05
  const steps = Math.ceil(params.tspan / dt)
  const times = []
  const temps = []
  const heaterStates = []

  let temp = params.initial
  let heaterOn = temp < params.target

  for (let i = 0; i <= steps; i++) {
    const t = i * dt
    times.push(t)
    temps.push(temp)
    heaterStates.push(heaterOn ? 1 : 0)

    // Bang-bang control with 1-degree hysteresis
    if (heaterOn && temp >= params.target) {
      heaterOn = false
    } else if (!heaterOn && temp < params.target - 1) {
      heaterOn = true
    }

    // Euler step: dT/dt = heatRate * heaterOn - coolRate * T
    const dTemp = (heaterOn ? params.heatRate : 0) - params.coolRate * temp
    temp += dTemp * dt
    temp = Math.max(0, temp)
  }

  return { times, temps, heaterStates }
}

// Extract predictions from simulation results
function computePredictions(result, params) {
  const { times, temps, heaterStates } = result

  // Time to first reach target
  let timeToTarget = null
  for (let i = 0; i < temps.length; i++) {
    if (params.initial < params.target && temps[i] >= params.target) {
      timeToTarget = times[i]
      break
    } else if (params.initial > params.target && temps[i] <= params.target) {
      timeToTarget = times[i]
      break
    }
  }

  // Already at or past target
  if (timeToTarget === null && Math.abs(params.initial - params.target) < 1) {
    timeToTarget = 0
  }

  // Equilibrium: average temperature over last 25% of simulation
  const lastQuarter = Math.floor(temps.length * 0.75)
  const eqTemps = temps.slice(lastQuarter)
  const equilibrium = eqTemps.reduce((a, b) => a + b, 0) / eqTemps.length

  // Max overshoot above target
  let maxOvershoot = 0
  if (params.initial < params.target) {
    for (const t of temps) {
      maxOvershoot = Math.max(maxOvershoot, t - params.target)
    }
  } else {
    for (const t of temps) {
      maxOvershoot = Math.max(maxOvershoot, params.target - t)
    }
  }

  // Duty cycle: % of time heater is on in last 50%
  const lastHalf = Math.floor(heaterStates.length * 0.5)
  const steadyHeater = heaterStates.slice(lastHalf)
  const dutyCycle = (steadyHeater.reduce((a, b) => a + b, 0) / steadyHeater.length) * 100

  return { timeToTarget, equilibrium, maxOvershoot, dutyCycle }
}

function updatePredictions(preds) {
  document.getElementById('pred-time').textContent =
    preds.timeToTarget !== null ? preds.timeToTarget.toFixed(1) + 's' : 'N/A'

  document.getElementById('pred-equil').textContent =
    preds.equilibrium.toFixed(1) + '°'

  document.getElementById('pred-overshoot').textContent =
    preds.maxOvershoot > 0.1 ? '+' + preds.maxOvershoot.toFixed(1) + '°' : 'none'

  document.getElementById('pred-duty').textContent =
    preds.dutyCycle.toFixed(0) + '%'
}

// Animate thermocouple probe
function updateProbe(temp, target, heaterOn) {
  // Fill height: 0° = 0px, 40° = 270px (full probe height)
  const maxTemp = 40
  const fillH = Math.min(270, Math.max(0, (temp / maxTemp) * 270))
  const fillY = 285 - fillH
  const probeFill = document.getElementById('probe-fill')
  probeFill.setAttribute('height', fillH)
  probeFill.setAttribute('y', fillY)

  // Color based on temperature relative to target
  if (temp >= target) {
    probeFill.setAttribute('fill', '#e74c3c')
  } else if (temp >= target - 2) {
    probeFill.setAttribute('fill', '#f39c12')
  } else {
    probeFill.setAttribute('fill', '#3498db')
  }

  // Target marker position
  const markerY = 285 - (target / maxTemp) * 270
  const marker = document.getElementById('target-marker')
  marker.querySelector('line').setAttribute('y1', markerY)
  marker.querySelector('line').setAttribute('y2', markerY)
  marker.querySelectorAll('line')[1].setAttribute('y1', markerY)
  marker.querySelectorAll('line')[1].setAttribute('y2', markerY)
  marker.querySelector('text').setAttribute('y', markerY + 4)

  // Temperature readout
  document.getElementById('probe-temp').textContent = temp.toFixed(1) + '°'

  // Heater status
  const status = document.getElementById('heater-status')
  if (heaterOn) {
    status.innerHTML = '<span class="heater-indicator on"></span><span>Heater ON</span>'
  } else {
    status.innerHTML = '<span class="heater-indicator off"></span><span>Heater OFF</span>'
  }
}

function initCharts() {
  const tempCtx = document.getElementById('temp-chart').getContext('2d')
  tempChart = new Chart(tempCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      interaction: { mode: 'index', intersect: false },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Time (s)' }, min: 0 },
        y: { title: { display: true, text: 'Temperature (°)' }, min: 0 }
      },
      plugins: {
        legend: { display: true, position: 'top' },
        annotation: { annotations: {} },
        tooltip: {
          callbacks: {
            label: (ctx) => `${ctx.dataset.label}: ${ctx.parsed.y.toFixed(1)}°`
          }
        }
      }
    }
  })

  const heaterCtx = document.getElementById('heater-chart').getContext('2d')
  heaterChart = new Chart(heaterCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Time (s)' }, min: 0 },
        y: { title: { display: true, text: 'Heater' }, min: -0.1, max: 1.1, ticks: { callback: v => v === 0 ? 'OFF' : v === 1 ? 'ON' : '' } }
      },
      plugins: { legend: { display: false } }
    }
  })
}

function updateCharts(result, params, preds) {
  // Downsample for chart performance
  const skip = Math.max(1, Math.floor(result.times.length / 600))
  const tempData = []
  const heaterData = []
  for (let i = 0; i < result.times.length; i += skip) {
    tempData.push({ x: result.times[i], y: result.temps[i] })
    heaterData.push({ x: result.times[i], y: result.heaterStates[i] })
  }

  const targetData = [{ x: 0, y: params.target }, { x: params.tspan, y: params.target }]

  // Annotations for time-to-target
  const annotations = {}
  if (preds.timeToTarget !== null && preds.timeToTarget > 0) {
    annotations.targetLine = {
      type: 'line',
      xMin: preds.timeToTarget,
      xMax: preds.timeToTarget,
      borderColor: 'rgba(46, 204, 113, 0.7)',
      borderWidth: 2,
      borderDash: [6, 3],
      label: {
        display: true,
        content: 'Target reached: ' + preds.timeToTarget.toFixed(1) + 's',
        position: 'start',
        backgroundColor: 'rgba(46, 204, 113, 0.85)',
        color: 'white',
        font: { size: 11, weight: 'bold' }
      }
    }
  }

  tempChart.data.datasets = [
    {
      label: 'Temperature',
      data: tempData,
      borderColor: '#e74c3c',
      backgroundColor: 'rgba(231, 76, 60, 0.08)',
      borderWidth: 2,
      fill: true,
      tension: 0.1,
      pointRadius: 0
    },
    {
      label: 'Target',
      data: targetData,
      borderColor: '#3498db',
      backgroundColor: 'transparent',
      borderWidth: 2,
      borderDash: [8, 4],
      fill: false,
      pointRadius: 0
    }
  ]
  tempChart.options.plugins.annotation.annotations = annotations
  tempChart.update()

  heaterChart.data.datasets = [
    {
      label: 'Heater',
      data: heaterData,
      borderColor: '#e67e22',
      backgroundColor: 'rgba(230, 126, 34, 0.2)',
      borderWidth: 2,
      fill: true,
      stepped: true,
      pointRadius: 0
    }
  ]
  heaterChart.update()
}

// Animate the thermocouple through simulation time
function animateProbe(result, params) {
  if (animFrame) cancelAnimationFrame(animFrame)

  const duration = 3000 // 3 second animation
  const startTime = performance.now()
  const { times, temps, heaterStates } = result

  function step(now) {
    const elapsed = now - startTime
    const progress = Math.min(1, elapsed / duration)

    // Map animation progress to simulation index
    const idx = Math.floor(progress * (temps.length - 1))
    updateProbe(temps[idx], params.target, heaterStates[idx] === 1)

    if (progress < 1) {
      animFrame = requestAnimationFrame(step)
    }
  }

  animFrame = requestAnimationFrame(step)
}

function runSimulation() {
  const params = getParams()
  simResult = simulate(params)
  const preds = computePredictions(simResult, params)

  updatePredictions(preds)
  updateCharts(simResult, params, preds)
  animateProbe(simResult, params)
}

function setupSliders() {
  const configs = [
    { id: 'target', suffix: '°' },
    { id: 'initial', suffix: '°' },
    { id: 'heat-rate', suffix: '' },
    { id: 'cool-rate', suffix: '' },
    { id: 'tspan', suffix: 's' }
  ]

  configs.forEach(({ id, suffix }) => {
    const slider = document.getElementById(id)
    const display = document.getElementById(id + '-value')
    slider.addEventListener('input', () => {
      display.textContent = slider.value + suffix
      runSimulation()
    })
  })
}

document.addEventListener('DOMContentLoaded', () => {
  initCharts()
  setupSliders()
  runSimulation()
})
