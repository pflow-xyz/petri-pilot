/**
 * Coffee Counter - Animated Coffee Shop Simulation
 * Real-time simulation visualization with Web Components
 */

import '../custom/components.js';

// API base path for when service is mounted at a prefix
const API_BASE = window.API_BASE || '';

// Configuration constants
const RESOURCE_MAX_VALUES = {
  coffee_beans: 2000,
  milk: 1000,
  cups: 500
};

// Dashboard active flag for cleanup
let isDashboardActive = false;

// Alert tracking for debouncing
let shownAlerts = new Map();
const ALERT_DEBOUNCE_MS = 30000; // 30 seconds between same alert

// Wait time tracking - store with timestamps for time window filtering
let completedOrderWaitTimes = []; // Array of {waitTime, simTime}
const MAX_WAIT_TIME_SAMPLES = 500; // Keep more samples for time window filtering

// Time window options (in simulation seconds)
const WAIT_TIME_WINDOWS = {
  'all': Infinity,
  '1h': 3600,      // 1 hour
  '30m': 1800,     // 30 minutes
  '10m': 600,      // 10 minutes
  '5m': 300        // 5 minutes
};
let selectedWaitTimeWindow = 'all';

// Historical data for charts
let historicalData = {
  timestamps: [],       // Simulation time points
  queueLength: [],      // Queue counts at each point
  waitTime: [],         // Avg wait time at each point
  abandonedCount: 0,    // Total customers who gave up
  abandonmentTimes: []  // Simulation times when customers abandoned
};
const MAX_HISTORY_POINTS = 100;
let lastHistorySample = 0;
const HISTORY_SAMPLE_INTERVAL = 30; // Sample every 30 sim seconds

// Patience configuration (in simulation seconds) - can be adjusted via UI
let patienceThreshold = 300;  // 5 min - customer leaves (adjustable)
const PATIENCE_WARNING_RATIO = 0.6;   // 60% of threshold - turn yellow
const PATIENCE_CRITICAL_RATIO = 0.8;  // 80% of threshold - turn red

// Session tracking for accurate rate calculation
let sessionStartDrinks = 0;  // Drinks count when session started

// State management
let simulationState = {
  isRunning: false,
  speed: 1,
  time: 0,
  instanceId: null,
  currentState: {
    coffee_beans: 1000,
    milk: 500,
    cups: 200,
    orders_pending: 0,
    espresso_ready: 0,
    latte_ready: 0,
    cappuccino_ready: 0,
    orders_complete: 0
  },
  stats: {
    drinksServed: 0,
    ordersPerHour: 0,
    averageWaitTime: 0,
    resourceEfficiency: 100,
    waitTimeSampleCount: 0
  }
};

let simulationInterval = null;
let webSocket = null;
let brewProgress = 0;
let brewCycleTime = 3000; // 3 seconds per brew cycle

// Component references
let sceneComponent = null;
let gaugesComponent = null;
let flowComponent = null;
let rateComponent = null;
let controlsComponent = null;
let stressComponent = null;
let statsComponent = null;
let chartsComponent = null;

/**
 * Render the dashboard page
 */
export function renderDashboard() {
  return `
    <div class="dashboard-container">
      <div class="dashboard-header">
        <h1 class="dashboard-title coffee-heading">☕ Bean Counter</h1>
        <p class="dashboard-subtitle">Real-time simulation and resource management</p>
      </div>

      <div class="dashboard-grid">
        <!-- Simulation Controls -->
        <div class="controls-section">
          <simulation-controls></simulation-controls>
        </div>

        <!-- Coffee Shop Scene -->
        <div class="scene-section">
          <coffee-shop-scene 
            customers="0" 
            barista-busy="false" 
            ready-drinks="0">
          </coffee-shop-scene>
        </div>

        <!-- Resource Gauges -->
        <div class="gauges-section">
          <resource-gauges></resource-gauges>
        </div>

        <!-- Time-Series Charts -->
        <div class="charts-section">
          <simulation-charts></simulation-charts>
        </div>

        <!-- Order Flow Board -->
        <div class="flow-section">
          <order-flow-board></order-flow-board>
        </div>

        <!-- Configuration and Stats -->
        <div class="config-section">
          <rate-config-panel></rate-config-panel>
          <stats-dashboard></stats-dashboard>
        </div>

        <!-- Recipe Display -->
        <div class="recipes-section">
          <recipe-display></recipe-display>
        </div>
      </div>

      <!-- Stress Alerts (fixed position overlay) -->
      <stress-indicator></stress-indicator>
    </div>
  `;
}

/**
 * Initialize dashboard after DOM is ready
 */
export function initDashboard() {
  // Set dashboard as active
  isDashboardActive = true;
  
  // Clear previous alert tracking
  shownAlerts.clear();
  
  // Get component references
  sceneComponent = document.querySelector('coffee-shop-scene');
  gaugesComponent = document.querySelector('resource-gauges');
  flowComponent = document.querySelector('order-flow-board');
  rateComponent = document.querySelector('rate-config-panel');
  controlsComponent = document.querySelector('simulation-controls');
  stressComponent = document.querySelector('stress-indicator');
  statsComponent = document.querySelector('stats-dashboard');
  chartsComponent = document.querySelector('simulation-charts');

  // Attach event listeners
  attachEventListeners();

  // Initialize instance
  initializeInstance();

  // Connect WebSocket for real-time updates
  connectWebSocket();

  // Load initial state
  loadCurrentState();
}

/**
 * Attach event listeners to components
 */
function attachEventListeners() {
  if (controlsComponent) {
    controlsComponent.addEventListener('play-pause', handlePlayPause);
    controlsComponent.addEventListener('reset', handleReset);
    controlsComponent.addEventListener('speed-change', handleSpeedChange);
    controlsComponent.addEventListener('download-report', handleDownloadReport);
  }

  if (rateComponent) {
    rateComponent.addEventListener('rate-change', handleRateChange);
    rateComponent.addEventListener('preset-applied', handlePresetApplied);
    rateComponent.addEventListener('patience-change', handlePatienceChange);
  }

  if (gaugesComponent) {
    gaugesComponent.addEventListener('restock', handleRestock);
  }

  if (statsComponent) {
    statsComponent.addEventListener('wait-time-window-change', handleWaitTimeWindowChange);
  }
}

/**
 * Handle wait time window change
 */
function handleWaitTimeWindowChange(event) {
  selectedWaitTimeWindow = event.detail.window;
  // Recalculate stats with new window
  updateStats();
  // Update UI immediately
  if (statsComponent) {
    statsComponent.updateStats(simulationState.stats);
  }
}

/**
 * Handle patience/walkout time change
 */
function handlePatienceChange(event) {
  const minutes = event.detail.minutes;
  patienceThreshold = minutes * 60; // Convert to seconds
}

/**
 * Initialize or get existing instance
 */
async function initializeInstance() {
  try {
    // Try to get existing instance from localStorage
    const savedInstanceId = localStorage.getItem('coffeeshop_instance_id');
    
    if (savedInstanceId) {
      // Check if instance still exists
      const response = await fetch(`${API_BASE}/api/coffeeshop/${savedInstanceId}`);
      if (response.ok) {
        simulationState.instanceId = savedInstanceId;
        return;
      }
    }

    // Create new instance
    const response = await fetch(`${API_BASE}/api/coffeeshop`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    });

    if (response.ok) {
      const data = await response.json();
      // API returns aggregate_id, not id
      const instanceId = data.aggregate_id || data.id;
      simulationState.instanceId = instanceId;
      localStorage.setItem('coffeeshop_instance_id', instanceId);
    }
  } catch (error) {
    console.error('Failed to initialize instance:', error);
    if (stressComponent) {
      stressComponent.addAlert('Failed to initialize coffee shop', 'critical');
    }
  }
}

/**
 * Connect WebSocket for real-time updates
 */
function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}${API_BASE}/ws`;
  
  webSocket = new WebSocket(wsUrl);
  
  webSocket.onopen = () => {
    console.log('WebSocket connected');
  };
  
  webSocket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      handleWebSocketMessage(data);
    } catch (error) {
      console.error('WebSocket message error:', error);
    }
  };
  
  webSocket.onerror = (error) => {
    console.error('WebSocket error:', error);
  };
  
  webSocket.onclose = () => {
    console.log('WebSocket disconnected');
    // Only attempt to reconnect if dashboard is still active
    if (isDashboardActive) {
      setTimeout(connectWebSocket, 5000);
    }
  };
}

/**
 * Handle WebSocket messages
 */
function handleWebSocketMessage(data) {
  if (data.type === 'state_update') {
    updateStateFromEvent(data);
  }
}

/**
 * Load current state from API
 */
async function loadCurrentState() {
  if (!simulationState.instanceId) return;

  try {
    const response = await fetch(`${API_BASE}/api/coffeeshop/${simulationState.instanceId}`);
    if (response.ok) {
      const data = await response.json();
      simulationState.currentState = data.state || simulationState.currentState;
      updateUI();
    }
  } catch (error) {
    console.error('Failed to load state:', error);
  }
}

/**
 * Update state from event
 */
function updateStateFromEvent(event) {
  if (event.state) {
    simulationState.currentState = event.state;
    updateUI();
    
    // Add order to flow board based on event type
    if (flowComponent) {
      if (event.eventType && event.eventType.includes('Order')) {
        const drinkType = extractDrinkType(event.eventType);
        flowComponent.addOrder(drinkType, 'pending', simulationState.time);
      }
    }
  }
}

/**
 * Extract drink type from event
 */
function extractDrinkType(eventType) {
  if (eventType.includes('Espresso')) return 'espresso';
  if (eventType.includes('Latte')) return 'latte';
  if (eventType.includes('Cappuccino')) return 'cappuccino';
  return 'espresso';
}

/**
 * Calculate barista mood based on simulation state and rate balance
 */
function calculateMood(state) {
  const queueLength = state.orders_pending || 0;
  const coffeePercent = (state.coffee_beans || 0) / RESOURCE_MAX_VALUES.coffee_beans * 100;
  const milkPercent = (state.milk || 0) / RESOURCE_MAX_VALUES.milk * 100;
  const cupsPercent = (state.cups || 0) / RESOURCE_MAX_VALUES.cups * 100;
  const minResource = Math.min(coffeePercent, milkPercent, cupsPercent);

  // Get current rates to see if we're keeping up
  let rateBalance = 0; // positive = making faster than ordering
  if (rateComponent) {
    const rates = rateComponent.getRates();
    const totalOrders = (rates.order_espresso || 0) + (rates.order_latte || 0) + (rates.order_cappuccino || 0);
    const totalMake = (rates.make_espresso || 0) + (rates.make_latte || 0) + (rates.make_cappuccino || 0);
    rateBalance = totalMake - totalOrders;
  }

  // Critical resources always triggers overwhelmed
  if (minResource < 10) {
    return 'overwhelmed';
  }

  // Huge queue with no hope of catching up
  if (queueLength > 8 && rateBalance <= 0) {
    return 'overwhelmed';
  }

  // Large queue but catching up, or low resources
  if (queueLength > 5 && rateBalance <= 0) {
    return 'stressed';
  }
  if (minResource < 20) {
    return 'stressed';
  }

  // Moderate queue
  if (queueLength > 3) {
    return rateBalance > 0 ? 'busy' : 'stressed';
  }

  // Small queue - mood depends on whether we're keeping up
  if (queueLength > 0) {
    return rateBalance > 10 ? 'normal' : 'busy';
  }

  // No queue, good resources
  return 'relaxed';
}

/**
 * Update all UI components
 */
function updateUI() {
  const state = simulationState.currentState;

  // Update scene
  if (sceneComponent) {
    const isBusy = state.orders_pending > 0;
    sceneComponent.setAttribute('customers', state.orders_pending || 0);
    sceneComponent.setAttribute('ready-drinks',
      (state.espresso_ready || 0) + (state.latte_ready || 0) + (state.cappuccino_ready || 0)
    );
    sceneComponent.setAttribute('barista-busy', isBusy ? 'true' : 'false');
    sceneComponent.setAttribute('brew-progress', Math.round(brewProgress));

    // Calculate mood based on conditions
    const mood = calculateMood(state);
    sceneComponent.setAttribute('mood', mood);

    sceneComponent.updateInventory(
      state.coffee_beans || 0,
      state.milk || 0,
      state.cups || 0
    );

    // Update brew progress animation
    if (isBusy && simulationState.isRunning) {
      brewProgress += (100 / (brewCycleTime / 100)) * simulationState.speed;
      if (brewProgress >= 100) {
        brewProgress = 0; // Reset for next drink
      }
    } else {
      brewProgress = 0;
    }
  }

  // Update gauges
  if (gaugesComponent) {
    gaugesComponent.updateResource('coffee_beans', state.coffee_beans || 0);
    gaugesComponent.updateResource('milk', state.milk || 0);
    gaugesComponent.updateResource('cups', state.cups || 0);
  }

  // Check for stress conditions
  checkStressConditions(state);
}

/**
 * Show debounced alert - prevents spamming same alert
 */
function showDebouncedAlert(key, message, type) {
  if (!stressComponent) return;
  
  const now = Date.now();
  const lastShown = shownAlerts.get(key);
  
  // Only show if not shown recently
  if (!lastShown || (now - lastShown) > ALERT_DEBOUNCE_MS) {
    stressComponent.addAlert(message, type);
    shownAlerts.set(key, now);
  }
}

/**
 * Check for stress conditions
 */
function checkStressConditions(state) {
  if (!stressComponent) return;

  // Check resource levels with debouncing
  if (state.coffee_beans < 200) {
    showDebouncedAlert('coffee_beans_critical', 'Coffee beans running low! Restock soon.', 'critical');
  } else if (state.coffee_beans < 400) {
    showDebouncedAlert('coffee_beans_warning', 'Coffee beans getting low.', 'warning');
  }

  if (state.milk < 100) {
    showDebouncedAlert('milk_critical', 'Milk running low! Restock soon.', 'critical');
  } else if (state.milk < 200) {
    showDebouncedAlert('milk_warning', 'Milk getting low.', 'warning');
  }

  if (state.cups < 40) {
    showDebouncedAlert('cups_critical', 'Cups running low! Restock soon.', 'critical');
  } else if (state.cups < 80) {
    showDebouncedAlert('cups_warning', 'Cups getting low.', 'warning');
  }

  // Check queue buildup
  if (state.orders_pending > 10) {
    showDebouncedAlert('queue_buildup', 'Order queue is building up!', 'warning');
  }
}

/**
 * Handle play/pause
 */
function handlePlayPause(event) {
  simulationState.isRunning = event.detail.playing;

  if (simulationState.isRunning) {
    // Track starting drinks count for accurate rate calculation (only on fresh start)
    if (simulationState.time === 0) {
      sessionStartDrinks = simulationState.currentState.orders_complete || 0;
    }
    startSimulation();
  } else {
    stopSimulation();
  }
}

/**
 * Handle reset
 */
async function handleReset() {
  stopSimulation();
  simulationState.time = 0;

  // Reset all stats
  simulationState.stats = {
    drinksServed: 0,
    ordersPerHour: 0,
    averageWaitTime: 0,
    resourceEfficiency: 100,
    waitTimeSampleCount: 0
  };

  // Reset session tracking
  sessionStartDrinks = 0;

  // Reset wait time tracking
  completedOrderWaitTimes.length = 0;

  // Reset historical data
  historicalData = {
    timestamps: [],
    queueLength: [],
    waitTime: [],
    abandonedCount: 0,
    abandonmentTimes: []
  };
  lastHistorySample = 0;

  // Reset charts
  if (chartsComponent) {
    chartsComponent.reset();
  }

  // Reset stats display
  if (statsComponent) {
    statsComponent.updateStats({
      ...simulationState.stats,
      abandonedCount: 0,
      abandonmentRate: 0
    });
  }

  // Reset flow board
  if (flowComponent) {
    flowComponent.reset();
  }

  if (controlsComponent) {
    controlsComponent.updateTime(0);
  }

  // Reset instance
  await initializeInstance();
  await loadCurrentState();

  if (stressComponent) {
    stressComponent.addAlert('Simulation reset', 'info');
  }
}

/**
 * Handle speed change
 */
function handleSpeedChange(event) {
  simulationState.speed = event.detail.speed;
  
  // Restart simulation with new speed if running
  if (simulationState.isRunning) {
    stopSimulation();
    startSimulation();
  }
}

/**
 * Handle download report - generates an SVG report with stats and charts
 */
function handleDownloadReport() {
  const stats = simulationState.stats;
  const state = simulationState.currentState;
  const simTime = simulationState.time;

  // Format time
  const hours = Math.floor(simTime / 3600);
  const minutes = Math.floor((simTime % 3600) / 60);
  const timeStr = `${hours}h ${minutes}m`;

  // Format wait time
  const waitSecs = Math.round(stats.averageWaitTime);
  const waitStr = waitSecs >= 60
    ? `${Math.floor(waitSecs / 60)}m ${waitSecs % 60}s`
    : `${waitSecs}s`;

  // Get chart data for mini sparklines
  const queueData = historicalData.queueLength.slice(-50);
  const waitData = historicalData.waitTime.slice(-50);

  // Generate sparkline path
  const generateSparkline = (data, width, height, yOffset) => {
    if (data.length < 2) return '';
    const max = Math.max(...data, 1);
    const xStep = width / (data.length - 1);
    const points = data.map((v, i) => `${i * xStep},${yOffset + height - (v / max) * height}`);
    return `M${points.join(' L')}`;
  };

  const svgWidth = 600;
  const svgHeight = 500;

  const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${svgWidth}" height="${svgHeight}" viewBox="0 0 ${svgWidth} ${svgHeight}">
  <defs>
    <linearGradient id="headerGrad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" style="stop-color:#3E2723"/>
      <stop offset="100%" style="stop-color:#6D4C41"/>
    </linearGradient>
    <linearGradient id="cardGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" style="stop-color:#FFFEF7"/>
      <stop offset="100%" style="stop-color:#FFF8E1"/>
    </linearGradient>
  </defs>

  <!-- Background -->
  <rect width="100%" height="100%" fill="#FFF8E1"/>

  <!-- Header -->
  <rect width="100%" height="70" fill="url(#headerGrad)"/>
  <text x="30" y="45" font-family="Georgia, serif" font-size="28" font-weight="bold" fill="white">☕ Bean Counter Report</text>
  <text x="${svgWidth - 30}" y="45" font-family="Arial, sans-serif" font-size="14" fill="rgba(255,255,255,0.8)" text-anchor="end">Simulation Time: ${timeStr}</text>

  <!-- Stats Cards Row -->
  <g transform="translate(20, 90)">
    <!-- Drinks Served -->
    <rect x="0" y="0" width="130" height="80" rx="8" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="65" y="35" font-family="Arial, sans-serif" font-size="24" font-weight="bold" fill="#3E2723" text-anchor="middle">${stats.drinksServed}</text>
    <text x="65" y="55" font-family="Arial, sans-serif" font-size="11" fill="#6D4C41" text-anchor="middle">Drinks Served</text>

    <!-- Orders/Hour -->
    <rect x="145" y="0" width="130" height="80" rx="8" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="210" y="35" font-family="Arial, sans-serif" font-size="24" font-weight="bold" fill="#3E2723" text-anchor="middle">${Math.round(stats.ordersPerHour)}</text>
    <text x="210" y="55" font-family="Arial, sans-serif" font-size="11" fill="#6D4C41" text-anchor="middle">Orders/Hour</text>

    <!-- Avg Wait Time -->
    <rect x="290" y="0" width="130" height="80" rx="8" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="355" y="35" font-family="Arial, sans-serif" font-size="24" font-weight="bold" fill="#3E2723" text-anchor="middle">${waitStr}</text>
    <text x="355" y="55" font-family="Arial, sans-serif" font-size="11" fill="#6D4C41" text-anchor="middle">Avg Wait Time</text>

    <!-- Abandoned -->
    <rect x="435" y="0" width="130" height="80" rx="8" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="500" y="35" font-family="Arial, sans-serif" font-size="24" font-weight="bold" fill="#F44336" text-anchor="middle">${historicalData.abandonedCount}</text>
    <text x="500" y="55" font-family="Arial, sans-serif" font-size="11" fill="#6D4C41" text-anchor="middle">Abandoned</text>
  </g>

  <!-- Resources Section -->
  <g transform="translate(20, 190)">
    <text x="0" y="0" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#3E2723">Resources</text>

    <!-- Coffee Beans -->
    <rect x="0" y="15" width="180" height="50" rx="6" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="15" y="45" font-family="Arial, sans-serif" font-size="12" fill="#5D4037">☕ Coffee Beans</text>
    <text x="165" y="45" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#3E2723" text-anchor="end">${state.coffee_beans}g</text>
    <rect x="15" y="52" width="150" height="6" rx="3" fill="#EFEBE9"/>
    <rect x="15" y="52" width="${(state.coffee_beans / 2000) * 150}" height="6" rx="3" fill="#6D4C41"/>

    <!-- Milk -->
    <rect x="195" y="15" width="180" height="50" rx="6" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="210" y="45" font-family="Arial, sans-serif" font-size="12" fill="#5D4037">🥛 Milk</text>
    <text x="360" y="45" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#3E2723" text-anchor="end">${state.milk}ml</text>
    <rect x="210" y="52" width="150" height="6" rx="3" fill="#EFEBE9"/>
    <rect x="210" y="52" width="${(state.milk / 1000) * 150}" height="6" rx="3" fill="#2196F3"/>

    <!-- Cups -->
    <rect x="390" y="15" width="180" height="50" rx="6" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <text x="405" y="45" font-family="Arial, sans-serif" font-size="12" fill="#5D4037">🥤 Cups</text>
    <text x="555" y="45" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#3E2723" text-anchor="end">${state.cups}</text>
    <rect x="405" y="52" width="150" height="6" rx="3" fill="#EFEBE9"/>
    <rect x="405" y="52" width="${(state.cups / 500) * 150}" height="6" rx="3" fill="#4CAF50"/>
  </g>

  <!-- Charts Section -->
  <g transform="translate(20, 290)">
    <text x="0" y="0" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#3E2723">Queue Length Over Time</text>
    <rect x="0" y="10" width="555" height="80" rx="6" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <path d="${generateSparkline(queueData, 535, 60, 20)}" fill="none" stroke="#2196F3" stroke-width="2" transform="translate(10, 0)"/>
  </g>

  <g transform="translate(20, 390)">
    <text x="0" y="0" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#3E2723">Average Wait Time Over Time</text>
    <rect x="0" y="10" width="555" height="80" rx="6" fill="url(#cardGrad)" stroke="#A1887F" stroke-width="1"/>
    <path d="${generateSparkline(waitData, 535, 60, 20)}" fill="none" stroke="#FF9800" stroke-width="2" transform="translate(10, 0)"/>
    ${historicalData.abandonmentTimes.length > 0 ? historicalData.abandonmentTimes.map(t => {
      const idx = historicalData.timestamps.findIndex(ts => ts >= t);
      if (idx === -1 || historicalData.timestamps.length < 2) return '';
      const x = 10 + (idx / (historicalData.timestamps.length - 1)) * 535;
      return `<line x1="${x}" y1="15" x2="${x}" y2="85" stroke="#F44336" stroke-width="2" opacity="0.7"/>`;
    }).join('\n    ') : ''}
  </g>

  <!-- Footer -->
  <text x="${svgWidth / 2}" y="${svgHeight - 15}" font-family="Arial, sans-serif" font-size="10" fill="#8D6E63" text-anchor="middle">Generated by Bean Counter • ${new Date().toLocaleString()}</text>
</svg>`;

  // Create download
  const blob = new Blob([svg], { type: 'image/svg+xml' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `bean-counter-report-${Date.now()}.svg`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);

  if (stressComponent) {
    stressComponent.addAlert('Report downloaded!', 'info');
  }
}

/**
 * Run prediction
 */
async function runPrediction(hours = 8) {
  if (!simulationState.instanceId) return;

  try {
    // Build URL with aggregate_id and current rates
    const params = new URLSearchParams();
    params.set('hours', hours.toString());
    params.set('aggregate_id', simulationState.instanceId);

    // Add current rates from the rate config panel
    if (rateComponent) {
      const rates = rateComponent.getRates();
      for (const [key, value] of Object.entries(rates)) {
        params.set(key, value.toString());
      }
    }

    const response = await fetch(`${API_BASE}/api/coffeeshop/predict?${params.toString()}`);
    if (response.ok) {
      const data = await response.json();

      // Show predicted end state
      if (data.resources) {
        const lastIndex = data.timePoints.length - 1;
        const endState = {};

        Object.entries(data.resources).forEach(([key, values]) => {
          endState[key] = Math.max(0, Math.floor(values[lastIndex] || 0));
        });

        // Update simulation state with predicted values
        simulationState.currentState = {
          ...simulationState.currentState,
          ...endState
        };

        // Update UI
        updateUI();
      }
    }
  } catch (error) {
    console.error('Prediction failed:', error);
  }
}

/**
 * Handle rate change
 */
function handleRateChange(event) {
  // Rate changes would be applied to backend configuration
  console.log('Rate changed:', event.detail);
}

/**
 * Handle preset applied
 */
function handlePresetApplied(event) {
  if (stressComponent) {
    stressComponent.addAlert(`Applied ${event.detail.preset} preset`, 'info');
  }
}

/**
 * Handle restock
 */
async function handleRestock(event) {
  const resource = event.detail.resource;

  if (!simulationState.instanceId) {
    if (stressComponent) {
      stressComponent.addAlert('No active instance to restock', 'warning');
    }
    return;
  }

  // Map resource name to transition
  const transitionMap = {
    'coffee_beans': 'restock_coffee_beans',
    'milk': 'restock_milk',
    'cups': 'restock_cups'
  };

  const transition = transitionMap[resource];
  if (!transition) {
    console.error('Unknown resource:', resource);
    return;
  }

  if (stressComponent) {
    stressComponent.addAlert(`Restocking ${resource}...`, 'info');
  }

  try {
    const response = await fetch(`${API_BASE}/api/${transition}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: simulationState.instanceId
      })
    });

    if (response.ok) {
      const data = await response.json();
      if (data.state) {
        simulationState.currentState = data.state;
        updateUI();
      }
      if (stressComponent) {
        stressComponent.addAlert(`${resource} restocked!`, 'info');
      }
    } else {
      const error = await response.json();
      if (stressComponent) {
        stressComponent.addAlert(`Restock failed: ${error.message || 'Unknown error'}`, 'warning');
      }
    }
  } catch (error) {
    console.error('Restock failed:', error);
    if (stressComponent) {
      stressComponent.addAlert('Restock failed', 'warning');
    }
  }
}

/**
 * Start simulation loop
 */
function startSimulation() {
  // Prevent multiple intervals from running simultaneously
  if (simulationInterval) {
    stopSimulation();
  }

  // Run loop faster at higher speeds, but add 1 sim second per tick
  // This gives us speed-x acceleration without double-counting
  const intervalMs = 1000 / simulationState.speed;

  simulationInterval = setInterval(() => {
    simulationState.time += 1;

    if (controlsComponent) {
      controlsComponent.updateTime(simulationState.time);
    }

    // Simulate random events based on rates
    simulateRandomEvents();

    // Check for impatient customers
    checkCustomerPatience();

    // Update stats
    updateStats();
  }, intervalMs);
}

/**
 * Stop simulation loop
 */
function stopSimulation() {
  if (simulationInterval) {
    clearInterval(simulationInterval);
    simulationInterval = null;
  }
}

/**
 * Check for customers who have waited too long and remove them
 */
function checkCustomerPatience() {
  if (!flowComponent) return;

  const pendingOrders = flowComponent.orders.pending;
  const ordersToAbandon = [];

  pendingOrders.forEach(order => {
    if (order.simTime !== null) {
      const waitTime = simulationState.time - order.simTime;
      if (waitTime >= patienceThreshold) {
        ordersToAbandon.push(order);
      }
    }
  });

  // Remove abandoned orders
  ordersToAbandon.forEach(order => {
    flowComponent.removeOrder(order.id, 'pending');
    historicalData.abandonedCount++;
    historicalData.abandonmentTimes.push(simulationState.time);

    if (stressComponent) {
      const waitMinutes = Math.floor((simulationState.time - order.simTime) / 60);
      stressComponent.addAlert(`Customer left after waiting ${waitMinutes}m!`, 'warning');
    }
  });

  // Update patience visual indicators on order cards
  if (flowComponent) {
    const patienceWarning = patienceThreshold * PATIENCE_WARNING_RATIO;
    const patienceCritical = patienceThreshold * PATIENCE_CRITICAL_RATIO;
    flowComponent.updatePatience(simulationState.time, patienceWarning, patienceCritical, patienceThreshold);
  }
}

/**
 * Simulate random events
 */
function simulateRandomEvents() {
  if (!rateComponent || !simulationState.instanceId) return;

  const rates = rateComponent.getRates();

  // Convert rates from per-hour to per-tick probability
  // Note: speed is already accounted for by running the loop faster (intervalMs = 1000/speed)
  const scaleFactor = 1 / 3600;

  // Order events
  ['order_espresso', 'order_latte', 'order_cappuccino'].forEach((transition) => {
    const probability = rates[transition] * scaleFactor;
    if (Math.random() < probability) {
      const drinkType = transition.replace('order_', '');
      executeTransition(transition);
      // Add to flow board with simulation time
      if (flowComponent) {
        flowComponent.addOrder(drinkType, 'pending', simulationState.time);
      }
    }
  });

  // Make events - moves order from pending -> preparing -> ready
  const state = simulationState.currentState;
  if (state.orders_pending > 0) {
    ['make_espresso', 'make_latte', 'make_cappuccino'].forEach((transition) => {
      const probability = rates[transition] * scaleFactor;
      if (Math.random() < probability) {
        executeTransition(transition);
        // Move from pending to preparing, then to ready (drink is made)
        if (flowComponent) {
          moveOldestOrder('pending', 'preparing');
          // After a short delay, move to ready (simulates prep time)
          setTimeout(() => {
            moveOldestOrder('preparing', 'ready');
          }, 500 / simulationState.speed);
        }
      }
    });
  }

  // Serve events
  const serveTransitions = [
    { transition: 'serve_espresso', ready: state.espresso_ready },
    { transition: 'serve_latte', ready: state.latte_ready },
    { transition: 'serve_cappuccino', ready: state.cappuccino_ready }
  ];

  serveTransitions.forEach(({ transition, ready }) => {
    if (ready > 0 && Math.random() < rates[transition] * scaleFactor) {
      executeTransition(transition);
      // Move from ready to served in flow board
      if (flowComponent) {
        moveOldestOrder('ready', 'served');
      }
    }
  });
}

/**
 * Move oldest order between flow board lanes
 */
function moveOldestOrder(fromLane, toLane) {
  if (!flowComponent) return;

  const orders = flowComponent.orders[fromLane];
  if (orders && orders.length > 0) {
    const oldestOrder = orders[0];

    // Track wait time when order is served (in simulation seconds)
    if (toLane === 'served' && oldestOrder.simTime !== null) {
      const waitTime = simulationState.time - oldestOrder.simTime; // simulation seconds
      completedOrderWaitTimes.push({
        waitTime: waitTime,
        simTime: simulationState.time  // when the order was completed
      });

      // Keep only the last N samples
      if (completedOrderWaitTimes.length > MAX_WAIT_TIME_SAMPLES) {
        completedOrderWaitTimes.shift();
      }
    }

    flowComponent.moveOrder(oldestOrder.id, fromLane, toLane);
  }
}

/**
 * Execute transition
 */
async function executeTransition(transition) {
  try {
    const response = await fetch(`${API_BASE}/api/${transition}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        aggregate_id: simulationState.instanceId
      })
    });

    if (response.ok) {
      const data = await response.json();
      if (data.state) {
        simulationState.currentState = data.state;
        updateUI();
      }
    }
    // 409 Conflict is expected when transition preconditions aren't met
    // (e.g., trying to make espresso when no orders pending)
    // Silently ignore these as part of normal simulation
  } catch (error) {
    // Only log actual network errors, not expected conflicts
    if (error.name !== 'TypeError') {
      console.error('Transition failed:', transition, error);
    }
  }
}

/**
 * Update statistics
 */
function updateStats() {
  const state = simulationState.currentState;

  // Calculate drinks served (session only, not cumulative from backend)
  const totalDrinks = state.orders_complete || 0;
  simulationState.stats.drinksServed = totalDrinks - sessionStartDrinks;

  // Calculate orders per hour based on session drinks
  const hours = simulationState.time / 3600;
  if (hours > 0) {
    simulationState.stats.ordersPerHour = simulationState.stats.drinksServed / hours;
  }

  // Calculate average wait time from tracked samples (filtered by time window)
  if (completedOrderWaitTimes.length > 0) {
    const windowSeconds = WAIT_TIME_WINDOWS[selectedWaitTimeWindow];
    const cutoffTime = simulationState.time - windowSeconds;

    // Filter samples within the time window
    const filteredSamples = completedOrderWaitTimes.filter(
      sample => sample.simTime >= cutoffTime
    );

    if (filteredSamples.length > 0) {
      const sum = filteredSamples.reduce((a, b) => a + b.waitTime, 0);
      simulationState.stats.averageWaitTime = sum / filteredSamples.length;
      simulationState.stats.waitTimeSampleCount = filteredSamples.length;
    } else {
      simulationState.stats.averageWaitTime = 0;
      simulationState.stats.waitTimeSampleCount = 0;
    }
  }

  // Calculate resource efficiency
  const totalResources = (state.coffee_beans || 0) + (state.milk || 0) + (state.cups || 0);
  const maxResources = RESOURCE_MAX_VALUES.coffee_beans + RESOURCE_MAX_VALUES.milk + RESOURCE_MAX_VALUES.cups;
  simulationState.stats.resourceEfficiency = (totalResources / maxResources) * 100;

  // Sample historical data for charts
  if (simulationState.time - lastHistorySample >= HISTORY_SAMPLE_INTERVAL) {
    lastHistorySample = simulationState.time;
    historicalData.timestamps.push(simulationState.time);
    historicalData.queueLength.push(state.orders_pending || 0);
    historicalData.waitTime.push(simulationState.stats.averageWaitTime);

    // Trim to max points
    if (historicalData.timestamps.length > MAX_HISTORY_POINTS) {
      historicalData.timestamps.shift();
      historicalData.queueLength.shift();
      historicalData.waitTime.shift();
    }

    // Update charts
    if (chartsComponent) {
      chartsComponent.updateData(historicalData);
    }
  }

  // Update stats component with abandoned count
  if (statsComponent) {
    const totalOrders = (state.orders_complete || 0) + historicalData.abandonedCount;
    const abandonmentRate = totalOrders > 0 ? (historicalData.abandonedCount / totalOrders) * 100 : 0;
    statsComponent.updateStats({
      ...simulationState.stats,
      abandonedCount: historicalData.abandonedCount,
      abandonmentRate: abandonmentRate
    });
  }
}

/**
 * Cleanup on page unload
 */
export function cleanupDashboard() {
  // Mark dashboard as inactive to prevent WebSocket reconnection
  isDashboardActive = false;
  
  // Stop simulation
  stopSimulation();
  
  // Close WebSocket
  if (webSocket) {
    webSocket.close();
    webSocket = null;
  }
  
  // Clear alert tracking
  shownAlerts.clear();
}
