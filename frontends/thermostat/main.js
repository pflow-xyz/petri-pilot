// Thermostat Controller - ODE Prediction with bang-bang control
// Auto-runs simulation on any parameter change

let tempChart = null
let heaterChart = null
let animFrame = null

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
// dT/dt = heatRate * heaterOn - coolRate * T
// Equilibrium (heater always on) = heatRate / coolRate
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

    // Euler step
    const dTemp = (heaterOn ? params.heatRate : 0) - params.coolRate * temp
    temp += dTemp * dt
    temp = Math.max(0, temp)
  }

  return { times, temps, heaterStates }
}

// Extract predictions from simulation results
function computePredictions(result, params) {
  const { times, temps, heaterStates } = result

  // Theoretical equilibrium (heater always on)
  const theoreticalEquil = params.heatRate / params.coolRate

  // Can the heater ever reach the target?
  const canReach = theoreticalEquil > params.target

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

  // Max overshoot above target (only if heating toward target)
  let maxOvershoot = 0
  if (params.initial < params.target) {
    for (const t of temps) {
      maxOvershoot = Math.max(maxOvershoot, t - params.target)
    }
  } else if (params.initial > params.target) {
    for (const t of temps) {
      maxOvershoot = Math.max(maxOvershoot, params.target - t)
    }
  }

  // Duty cycle: % of time heater is on in last 50%
  const lastHalf = Math.floor(heaterStates.length * 0.5)
  const steadyHeater = heaterStates.slice(lastHalf)
  const dutyCycle = (steadyHeater.reduce((a, b) => a + b, 0) / steadyHeater.length) * 100

  return { timeToTarget, equilibrium, maxOvershoot, dutyCycle, canReach, theoreticalEquil }
}

function updatePredictions(preds) {
  const timeEl = document.getElementById('pred-time')
  if (preds.timeToTarget !== null) {
    timeEl.textContent = preds.timeToTarget.toFixed(1) + 's'
  } else if (!preds.canReach) {
    timeEl.textContent = 'never'
    timeEl.title = 'Heater max equilibrium: ' + preds.theoreticalEquil.toFixed(0) + '°'
  } else {
    timeEl.textContent = '>' + document.getElementById('tspan').value + 's'
  }

  document.getElementById('pred-equil').textContent =
    preds.equilibrium.toFixed(1) + '°'

  document.getElementById('pred-overshoot').textContent =
    preds.maxOvershoot > 0.1 ? '+' + preds.maxOvershoot.toFixed(1) + '°' : 'none'

  document.getElementById('pred-duty').textContent =
    preds.dutyCycle.toFixed(0) + '%'
}

// Update thermocouple probe visualization
function updateProbe(temp, target, heaterOn) {
  const maxTemp = 40
  const clampedTemp = Math.max(0, Math.min(maxTemp, temp || 0))
  const clampedTarget = Math.max(0, Math.min(maxTemp, target || 0))

  // Fill height: 0° = 0px, 40° = 270px
  const fillH = (clampedTemp / maxTemp) * 270
  const fillY = 285 - fillH
  const probeFill = document.getElementById('probe-fill')
  if (probeFill) {
    probeFill.setAttribute('height', String(fillH))
    probeFill.setAttribute('y', String(fillY))

    // Color based on temperature relative to target
    if (clampedTemp >= clampedTarget) {
      probeFill.setAttribute('fill', '#e74c3c')
    } else if (clampedTemp >= clampedTarget - 2) {
      probeFill.setAttribute('fill', '#f39c12')
    } else {
      probeFill.setAttribute('fill', '#3498db')
    }
  }

  // Target marker position
  const markerY = 285 - (clampedTarget / maxTemp) * 270
  const marker = document.getElementById('target-marker')
  if (marker) {
    const lines = marker.querySelectorAll('line')
    lines[0].setAttribute('y1', String(markerY))
    lines[0].setAttribute('y2', String(markerY))
    lines[1].setAttribute('y1', String(markerY))
    lines[1].setAttribute('y2', String(markerY))
    marker.querySelector('text').setAttribute('y', String(markerY + 4))
  }

  // Temperature readout
  const readout = document.getElementById('probe-temp')
  if (readout) readout.textContent = clampedTemp.toFixed(1) + '°'

  // Heater status
  const status = document.getElementById('heater-status')
  if (status) {
    if (heaterOn) {
      status.innerHTML = '<span class="heater-indicator on"></span><span>Heater ON</span>'
    } else {
      status.innerHTML = '<span class="heater-indicator off"></span><span>Heater OFF</span>'
    }
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
  if (!result || !result.temps || result.temps.length === 0) return

  const duration = 3000 // 3 second animation
  const startTime = performance.now()

  function step(now) {
    const elapsed = now - startTime
    const progress = Math.min(1, elapsed / duration)
    const idx = Math.min(
      result.temps.length - 1,
      Math.floor(progress * (result.temps.length - 1))
    )
    updateProbe(result.temps[idx], params.target, result.heaterStates[idx] === 1)

    if (progress < 1) {
      animFrame = requestAnimationFrame(step)
    }
  }

  animFrame = requestAnimationFrame(step)
}

function runSimulation() {
  const params = getParams()
  const result = simulate(params)
  const preds = computePredictions(result, params)

  updatePredictions(preds)
  updateCharts(result, params, preds)
  animateProbe(result, params)
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

// Module scripts are deferred, DOM is ready by execution time
initCharts()
setupSliders()
// Set initial probe state before animation
updateProbe(15, 22, false)
runSimulation()
