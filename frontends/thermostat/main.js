// Thermostat Controller - ODE Simulation with bang-bang control
// Uses custom simulation since bang-bang control requires discrete switching

let tempChart = null
let heaterChart = null

function getParams() {
  return {
    heatRate: parseFloat(document.getElementById('heat-rate').value),
    coolRate: parseFloat(document.getElementById('cool-rate').value),
    target: parseInt(document.getElementById('target').value),
    initial: parseInt(document.getElementById('initial').value),
    tspan: parseInt(document.getElementById('tspan').value)
  }
}

// Custom simulation: bang-bang controller with Euler integration
// (The ODE solver can't handle the discrete switching, so we simulate directly)
function simulate(params) {
  const dt = 0.1
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

    // Bang-bang control: switch heater based on temperature vs target
    if (heaterOn && temp >= params.target) {
      heaterOn = false
    } else if (!heaterOn && temp < params.target - 1) {
      // 1-degree hysteresis to prevent rapid cycling
      heaterOn = true
    }

    // Euler step
    const dTemp = (heaterOn ? params.heatRate : 0) - params.coolRate * temp
    temp += dTemp * dt
    temp = Math.max(0, temp)
  }

  return { times, temps, heaterStates }
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
        x: { type: 'linear', title: { display: true, text: 'Time' }, min: 0 },
        y: { title: { display: true, text: 'Temperature' }, min: 0 }
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

  const heaterCtx = document.getElementById('heater-chart').getContext('2d')
  heaterChart = new Chart(heaterCtx, {
    type: 'line',
    data: { datasets: [] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      scales: {
        x: { type: 'linear', title: { display: true, text: 'Time' }, min: 0 },
        y: { title: { display: true, text: 'Heater' }, min: -0.1, max: 1.1, ticks: { callback: v => v === 0 ? 'OFF' : v === 1 ? 'ON' : '' } }
      },
      plugins: { legend: { display: false } }
    }
  })
}

function updateCharts(result, params) {
  if (!result) return

  const tempData = result.times.map((t, i) => ({ x: t, y: result.temps[i] }))
  const targetData = [{ x: 0, y: params.target }, { x: params.tspan, y: params.target }]
  const heaterData = result.times.map((t, i) => ({ x: t, y: result.heaterStates[i] }))

  tempChart.data.datasets = [
    {
      label: 'Temperature',
      data: tempData,
      borderColor: '#e74c3c',
      backgroundColor: 'rgba(231, 76, 60, 0.1)',
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

  // Update gauge
  const finalTemp = result.temps[result.temps.length - 1]
  const finalHeater = result.heaterStates[result.heaterStates.length - 1]
  updateGauge(finalTemp, params.target, finalHeater)
}

function updateGauge(currentTemp, targetTemp, heaterOn) {
  document.getElementById('current-temp').textContent = currentTemp.toFixed(1)
  document.getElementById('target-temp').textContent = targetTemp

  // Thermometer fill (0 to 40 range)
  const fillPct = Math.min(100, Math.max(0, (currentTemp / 40) * 100))
  const targetPct = Math.min(100, Math.max(0, (targetTemp / 40) * 100))
  document.getElementById('thermo-fill').style.height = fillPct + '%'
  document.getElementById('thermo-target').style.bottom = targetPct + '%'

  const status = document.getElementById('heater-status')
  if (heaterOn) {
    status.innerHTML = '<span class="heater-indicator on"></span><span>Heater ON</span>'
  } else {
    status.innerHTML = '<span class="heater-indicator off"></span><span>Heater OFF</span>'
  }
}

function setupSliders() {
  const sliders = ['heat-rate', 'cool-rate', 'target', 'initial', 'tspan']
  sliders.forEach(id => {
    const slider = document.getElementById(id)
    const display = document.getElementById(id + '-value')
    slider.addEventListener('input', () => {
      display.textContent = slider.value
    })
  })
}

window.runSimulation = function() {
  const params = getParams()
  const result = simulate(params)
  updateCharts(result, params)
}

window.resetParams = function() {
  document.getElementById('heat-rate').value = 0.5
  document.getElementById('cool-rate').value = 0.1
  document.getElementById('target').value = 22
  document.getElementById('initial').value = 15
  document.getElementById('tspan').value = 60

  document.getElementById('heat-rate-value').textContent = '0.5'
  document.getElementById('cool-rate-value').textContent = '0.1'
  document.getElementById('target-value').textContent = '22'
  document.getElementById('initial-value').textContent = '15'
  document.getElementById('tspan-value').textContent = '60'

  runSimulation()
}

document.addEventListener('DOMContentLoaded', () => {
  initCharts()
  setupSliders()
  runSimulation()
})
