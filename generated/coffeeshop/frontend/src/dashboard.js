/**
 * Animated Coffee Shop Dashboard
 * Real-time simulation visualization with Web Components
 */

import '/custom/components.js';

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

// Health state tracking (mirrors go-pflow/examples/coffeeshop)
const HEALTH_STATES = {
  HEALTHY: { key: 'healthy', emoji: '💚', label: 'Healthy', description: 'Operating smoothly' },
  BUSY: { key: 'busy', emoji: '💛', label: 'Busy', description: 'High traffic, managing well' },
  STRESSED: { key: 'stressed', emoji: '🟠', label: 'Stressed', description: 'Falling behind, queues growing' },
  SLA_CRISIS: { key: 'sla_crisis', emoji: '🔴', label: 'SLA Crisis', description: 'SLA targets being missed' },
  INVENTORY_CRISIS: { key: 'inventory_crisis', emoji: '📦', label: 'Inventory Crisis', description: 'Running low on ingredients' },
  CRITICAL: { key: 'critical', emoji: '🚨', label: 'Critical', description: 'Immediate action needed' }
};

// Map backend health keys to HEALTH_STATES
const HEALTH_KEY_MAP = {
  'healthy': HEALTH_STATES.HEALTHY,
  'busy': HEALTH_STATES.BUSY,
  'stressed': HEALTH_STATES.STRESSED,
  'sla_crisis': HEALTH_STATES.SLA_CRISIS,
  'inventory_crisis': HEALTH_STATES.INVENTORY_CRISIS,
  'critical': HEALTH_STATES.CRITICAL
};

let currentHealthState = HEALTH_STATES.HEALTHY;
let previousHealthState = HEALTH_STATES.HEALTHY;
let queueHistory = [];
const QUEUE_HISTORY_SIZE = 10;

// State management
let simulationState = {
  isRunning: false,
  isPlaying: false,
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
    resourceEfficiency: 100
  },
  rates: {
    order_espresso: 10,
    order_latte: 15,
    order_cappuccino: 8,
    make_espresso: 20,
    make_latte: 15,
    make_cappuccino: 12,
    serve_espresso: 25,
    serve_latte: 20,
    serve_cappuccino: 18
  }
};

let simulationInterval = null;
let webSocket = null;

// Component references
let sceneComponent = null;
let gaugesComponent = null;
let flowComponent = null;
let rateComponent = null;
let controlsComponent = null;
let stressComponent = null;
let statsComponent = null;

/**
 * Render the dashboard page
 */
export function renderDashboard() {
  return `
    <div class="dashboard-container">
      <div class="dashboard-header">
        <h1 class="dashboard-title coffee-heading">☕ Coffee Shop Dashboard</h1>
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

        <!-- Order Flow Board -->
        <div class="flow-section">
          <order-flow-board></order-flow-board>
        </div>

        <!-- Configuration and Stats -->
        <div class="config-section">
          <rate-config-panel></rate-config-panel>
          <stats-dashboard></stats-dashboard>
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
export async function initDashboard() {
  // Set dashboard as active
  isDashboardActive = true;

  // Clear previous alert tracking
  shownAlerts.clear();

  // Wait for custom elements to be defined
  await Promise.all([
    customElements.whenDefined('coffee-shop-scene'),
    customElements.whenDefined('resource-gauges'),
    customElements.whenDefined('order-flow-board'),
    customElements.whenDefined('rate-config-panel'),
    customElements.whenDefined('simulation-controls'),
    customElements.whenDefined('stress-indicator'),
    customElements.whenDefined('stats-dashboard'),
  ]).catch(() => {
    console.warn('Some custom elements not defined, continuing anyway');
  });

  // Get component references
  sceneComponent = document.querySelector('coffee-shop-scene');
  gaugesComponent = document.querySelector('resource-gauges');
  flowComponent = document.querySelector('order-flow-board');
  rateComponent = document.querySelector('rate-config-panel');
  controlsComponent = document.querySelector('simulation-controls');
  stressComponent = document.querySelector('stress-indicator');
  statsComponent = document.querySelector('stats-dashboard');

  // Attach event listeners
  attachEventListeners();

  // Initialize instance and load state
  await initializeInstance();

  // Connect WebSocket for real-time updates
  connectWebSocket();

  // Load initial state and update UI
  await loadCurrentState();

  // Show initial state in UI even without instance
  updateUI();
}

/**
 * Attach event listeners to components
 */
function attachEventListeners() {
  console.log('[Dashboard] attachEventListeners called', {
    controls: !!controlsComponent,
    rate: !!rateComponent,
    gauges: !!gaugesComponent
  });

  if (controlsComponent) {
    controlsComponent.addEventListener('play-pause', handlePlayPause);
    controlsComponent.addEventListener('reset', handleReset);
    controlsComponent.addEventListener('speed-change', handleSpeedChange);
    controlsComponent.addEventListener('jump-runout', handleJumpToRunout);
  }

  if (rateComponent) {
    rateComponent.addEventListener('rate-change', handleRateChange);
    rateComponent.addEventListener('preset-applied', handlePresetApplied);
    rateComponent.addEventListener('test-state-applied', handleTestStateApplied);
  }

  if (gaugesComponent) {
    console.log('[Dashboard] Adding restock listener to gauges');
    gaugesComponent.addEventListener('restock', handleRestock);
  }
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
      const response = await fetch(`/api/coffeeshop/${savedInstanceId}`);
      if (response.ok) {
        simulationState.instanceId = savedInstanceId;
        return;
      }
    }

    // Create new instance
    const response = await fetch('/api/coffeeshop', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    });

    if (response.ok) {
      const data = await response.json();
      simulationState.instanceId = data.id;
      localStorage.setItem('coffeeshop_instance_id', data.id);
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
  const wsUrl = `${protocol}//${window.location.host}/ws`;
  
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
    const response = await fetch(`/api/coffeeshop/${simulationState.instanceId}`);
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
        flowComponent.addOrder(drinkType, 'pending');
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
 * Update all UI components
 */
function updateUI() {
  const state = simulationState.currentState;

  // Update scene
  if (sceneComponent) {
    sceneComponent.setAttribute('customers', state.orders_pending || 0);
    sceneComponent.setAttribute('ready-drinks', 
      (state.espresso_ready || 0) + (state.latte_ready || 0) + (state.cappuccino_ready || 0)
    );
    sceneComponent.setAttribute('barista-busy', state.orders_pending > 0 ? 'true' : 'false');
    sceneComponent.updateInventory(
      state.coffee_beans || 0,
      state.milk || 0,
      state.cups || 0
    );
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
 * Calculate inventory health score (0.0 - 1.0)
 * Returns the minimum health across all key ingredients
 */
function calculateInventoryHealth(state) {
  const levels = [
    (state.coffee_beans || 0) / RESOURCE_MAX_VALUES.coffee_beans,
    (state.milk || 0) / RESOURCE_MAX_VALUES.milk,
    (state.cups || 0) / RESOURCE_MAX_VALUES.cups
  ];
  return Math.min(...levels);
}

/**
 * Calculate queue trend (positive = growing, negative = shrinking)
 */
function calculateQueueTrend() {
  if (queueHistory.length < 3) return 0;
  const recent = queueHistory.slice(-3);
  return (recent[2] - recent[0]) / 3.0;
}

/**
 * Classify current health state based on metrics
 * Mirrors go-pflow/examples/coffeeshop/simulator.go classifyHealth()
 */
function classifyHealthState(state) {
  const queueLength = state.orders_pending || 0;
  const inventoryHealth = calculateInventoryHealth(state);
  const queueTrend = calculateQueueTrend();

  // Calculate SLA breach rate (simplified - based on queue buildup)
  // In a real system, track actual wait times vs SLA target
  const completedOrders = state.orders_complete || 0;
  const estimatedBreachRate = completedOrders > 10
    ? Math.min(queueLength / (completedOrders + queueLength), 1.0)
    : 0;

  // Critical: Any ingredient depleted (menu would be empty)
  if (inventoryHealth <= 0) {
    return HEALTH_STATES.CRITICAL;
  }

  // Inventory Crisis: Any ingredient below 10%
  if (inventoryHealth < 0.10) {
    return HEALTH_STATES.INVENTORY_CRISIS;
  }

  // SLA Crisis: More than 30% breach rate
  if (estimatedBreachRate > 0.30) {
    return HEALTH_STATES.SLA_CRISIS;
  }

  // Stressed: Queue > 10 or growing fast, or 15-30% breach rate
  if (queueLength > 10 || queueTrend > 2.0 || estimatedBreachRate > 0.15) {
    return HEALTH_STATES.STRESSED;
  }

  // Busy: Queue > 5 or moderate growth, or 5-15% breach rate
  if (queueLength > 5 || queueTrend > 1.0 || estimatedBreachRate > 0.05) {
    return HEALTH_STATES.BUSY;
  }

  // Healthy: Everything under control
  return HEALTH_STATES.HEALTHY;
}

/**
 * Check for stress conditions and update health state
 */
function checkStressConditions(state) {
  // Update queue history for trend analysis
  queueHistory.push(state.orders_pending || 0);
  if (queueHistory.length > QUEUE_HISTORY_SIZE) {
    queueHistory.shift();
  }

  // Classify current health state
  previousHealthState = currentHealthState;
  currentHealthState = classifyHealthState(state);

  // Alert on health state changes
  if (currentHealthState.key !== previousHealthState.key) {
    const isWorsening = getHealthSeverity(currentHealthState) > getHealthSeverity(previousHealthState);
    const alertType = isWorsening ? 'warning' : 'info';
    showDebouncedAlert(
      `health_${currentHealthState.key}`,
      `${currentHealthState.emoji} ${currentHealthState.label}: ${currentHealthState.description}`,
      alertType
    );
  }

  // Specific resource warnings
  const inventoryHealth = calculateInventoryHealth(state);

  if (state.coffee_beans < 200) {
    showDebouncedAlert('coffee_beans_critical', '☕ Coffee beans critical! Restock immediately.', 'critical');
  } else if (state.coffee_beans < 400) {
    showDebouncedAlert('coffee_beans_warning', '☕ Coffee beans getting low.', 'warning');
  }

  if (state.milk < 100) {
    showDebouncedAlert('milk_critical', '🥛 Milk critical! Restock immediately.', 'critical');
  } else if (state.milk < 200) {
    showDebouncedAlert('milk_warning', '🥛 Milk getting low.', 'warning');
  }

  if (state.cups < 40) {
    showDebouncedAlert('cups_critical', '🥤 Cups critical! Restock immediately.', 'critical');
  } else if (state.cups < 80) {
    showDebouncedAlert('cups_warning', '🥤 Cups getting low.', 'warning');
  }

  // Queue warnings
  if (state.orders_pending > 15) {
    showDebouncedAlert('queue_critical', '📋 Queue critical! Orders backing up severely.', 'critical');
  } else if (state.orders_pending > 10) {
    showDebouncedAlert('queue_warning', '📋 Queue building up - consider increasing prep speed.', 'warning');
  }
}

/**
 * Get numeric severity for health state comparison
 */
function getHealthSeverity(healthState) {
  const severities = {
    healthy: 0,
    busy: 1,
    stressed: 2,
    sla_crisis: 3,
    inventory_crisis: 3,
    critical: 4
  };
  return severities[healthState.key] || 0;
}

/**
 * Get current health state (for external access)
 */
export function getCurrentHealthState() {
  return currentHealthState;
}

/**
 * Fetch health state from backend API
 * Provides parity - same calculation available on both frontend and backend
 */
export async function fetchHealthFromBackend(aggregateId = null) {
  try {
    // Use the predict endpoint with health=true query param (workaround for router limitations)
    let url = '/api/coffeeshop/predict?health=true';
    if (aggregateId) {
      url += `&id=${aggregateId}`;
    }

    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Health API error: ${response.status}`);
    }

    const data = await response.json();

    // Update local state to match backend
    if (data.health && HEALTH_KEY_MAP[data.health]) {
      currentHealthState = HEALTH_KEY_MAP[data.health];
    }

    return data;
  } catch (error) {
    console.error('Failed to fetch health from backend:', error);
    // Fall back to local calculation
    return {
      health: currentHealthState.key,
      info: currentHealthState,
      metrics: {
        queueHistory: queueHistory,
        inventoryHealth: calculateInventoryHealth(simulationState.currentState),
        currentQueueLength: simulationState.currentState.orders_pending || 0
      },
      state: simulationState.currentState
    };
  }
}

/**
 * Sync health state with backend
 * Called periodically or on demand to ensure parity
 */
export async function syncHealthWithBackend() {
  if (!simulationState.instanceId) return;

  const backendHealth = await fetchHealthFromBackend(simulationState.instanceId);

  // Compare local vs backend
  if (backendHealth.health !== currentHealthState.key) {
    console.log(`Health sync: local=${currentHealthState.key}, backend=${backendHealth.health}`);
    // Backend is authoritative when available
    if (HEALTH_KEY_MAP[backendHealth.health]) {
      previousHealthState = currentHealthState;
      currentHealthState = HEALTH_KEY_MAP[backendHealth.health];
    }
  }

  return backendHealth;
}

/**
 * Handle play/pause
 */
function handlePlayPause(event) {
  simulationState.isRunning = event.detail.playing;

  if (simulationState.isRunning) {
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
 * Handle jump to runout
 */
async function handleJumpToRunout() {
  try {
    const response = await fetch('/api/coffeeshop/runout');
    if (response.ok) {
      const runoutData = await response.json();
      const runoutTimes = Object.values(runoutData);
      
      if (runoutTimes.length > 0) {
        const minRunout = Math.min(...runoutTimes.filter(t => t !== null));
        const hours = minRunout / 60; // Convert minutes to hours
        
        if (stressComponent) {
          stressComponent.addAlert(`Fastest runout predicted at ${hours.toFixed(1)} hours`, 'warning');
        }
        
        // Run prediction to that point
        runPrediction(hours);
      }
    }
  } catch (error) {
    console.error('Failed to get runout prediction:', error);
  }
}

/**
 * Run prediction
 */
async function runPrediction(hours = 8) {
  try {
    const response = await fetch(`/api/coffeeshop/predict?hours=${hours}`);
    if (response.ok) {
      const data = await response.json();
      
      // Show predicted end state
      if (data.resources) {
        const lastIndex = data.timePoints.length - 1;
        const endState = {};
        
        Object.entries(data.resources).forEach(([key, values]) => {
          endState[key] = Math.floor(values[lastIndex] || 0);
        });
        
        // Update gauges with predictions
        if (gaugesComponent) {
          Object.entries(endState).forEach(([key, value]) => {
            gaugesComponent.updateResource(key, value);
          });
        }
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
 * Handle test state applied - directly sets state values to induce health conditions
 */
function handleTestStateApplied(event) {
  const { testState, state } = event.detail;

  // Update simulation state directly
  simulationState.currentState = { ...state };

  // Recalculate and display health state
  const health = classifyHealthState(state);
  updateHealthDisplay(health);

  // Update scene component
  if (sceneComponent) {
    sceneComponent.setAttribute('customers', state.orders_pending || 0);
    sceneComponent.setAttribute('ready-drinks',
      (state.espresso_ready || 0) + (state.latte_ready || 0) + (state.cappuccino_ready || 0)
    );
    sceneComponent.setAttribute('barista-busy', state.orders_pending > 0 ? 'true' : 'false');
    sceneComponent.updateInventory(
      state.coffee_beans || 0,
      state.milk || 0,
      state.cups || 0
    );
  }

  // Update gauges component
  if (gaugesComponent) {
    gaugesComponent.updateResource('coffee_beans', state.coffee_beans || 0);
    gaugesComponent.updateResource('milk', state.milk || 0);
    gaugesComponent.updateResource('cups', state.cups || 0);
  }

  // Update order flow
  if (flowComponent) {
    flowComponent.updateOrders(state);
  }

  // Show alert
  if (stressComponent) {
    const healthInfo = HEALTH_STATES[health.key.toUpperCase()] || health;
    stressComponent.addAlert(
      `Test state applied: ${healthInfo.emoji} ${healthInfo.label}`,
      health.severity >= 3 ? 'warning' : 'info'
    );
  }

  console.log('Test state applied:', testState, state, 'Health:', health);
}

/**
 * Handle restock - adds inventory without resetting simulation
 */
async function handleRestock(event) {
  const resource = event.detail.resource;

  // Define restock amounts (back to max values)
  const restockAmounts = {
    coffee_beans: 2000,
    milk: 1000,
    cups: 500
  };

  const maxAmount = restockAmounts[resource];
  if (!maxAmount) return;

  // Update simulation state directly
  simulationState.currentState[resource] = maxAmount;

  // Recalculate health
  const health = classifyHealthState(simulationState.currentState);
  updateHealthDisplay(health);

  // Update UI components
  if (sceneComponent) {
    sceneComponent.updateInventory(
      simulationState.currentState.coffee_beans || 0,
      simulationState.currentState.milk || 0,
      simulationState.currentState.cups || 0
    );
  }

  if (gaugesComponent) {
    gaugesComponent.updateResource(resource, maxAmount);
  }

  if (stressComponent) {
    stressComponent.addAlert(`Restocked ${resource} to ${maxAmount}`, 'info');
  }
}

/**
 * Start simulation loop
 */
function startSimulation() {
  const intervalMs = 1000 / simulationState.speed; // Adjust based on speed
  
  simulationInterval = setInterval(() => {
    simulationState.time += 1 * simulationState.speed;
    
    if (controlsComponent) {
      controlsComponent.updateTime(simulationState.time);
    }

    // Simulate random events based on rates
    simulateRandomEvents();

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
 * Simulate random events
 */
function simulateRandomEvents() {
  if (!rateComponent || !simulationState.instanceId) return;

  const rates = rateComponent.getRates();
  
  // Convert rates from per-hour to per-second probability
  const scaleFactor = 1 / 3600 * simulationState.speed;

  // Order events
  ['order_espresso', 'order_latte', 'order_cappuccino'].forEach((transition) => {
    const probability = rates[transition] * scaleFactor;
    if (Math.random() < probability) {
      executeTransition(transition);
    }
  });

  // Make events
  const state = simulationState.currentState;
  if (state.orders_pending > 0) {
    ['make_espresso', 'make_latte', 'make_cappuccino'].forEach((transition) => {
      const probability = rates[transition] * scaleFactor;
      if (Math.random() < probability) {
        executeTransition(transition);
      }
    });
  }

  // Serve events
  if (state.espresso_ready > 0 && Math.random() < rates['serve_espresso'] * scaleFactor) {
    executeTransition('serve_espresso');
  }
  if (state.latte_ready > 0 && Math.random() < rates['serve_latte'] * scaleFactor) {
    executeTransition('serve_latte');
  }
  if (state.cappuccino_ready > 0 && Math.random() < rates['serve_cappuccino'] * scaleFactor) {
    executeTransition('serve_cappuccino');
  }
}

/**
 * Execute transition
 */
async function executeTransition(transition) {
  try {
    const response = await fetch(`/api/${transition}`, {
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
  } catch (error) {
    console.error('Transition failed:', transition, error);
  }
}

/**
 * Update statistics
 */
function updateStats() {
  const state = simulationState.currentState;
  
  // Calculate drinks served
  simulationState.stats.drinksServed = state.orders_complete || 0;
  
  // Calculate orders per hour (simplified)
  const hours = simulationState.time / 3600;
  if (hours > 0) {
    simulationState.stats.ordersPerHour = simulationState.stats.drinksServed / hours;
  }
  
  // Calculate resource efficiency
  const totalResources = (state.coffee_beans || 0) + (state.milk || 0) + (state.cups || 0);
  const maxResources = RESOURCE_MAX_VALUES.coffee_beans + RESOURCE_MAX_VALUES.milk + RESOURCE_MAX_VALUES.cups;
  simulationState.stats.resourceEfficiency = (totalResources / maxResources) * 100;
  
  // Update stats component
  if (statsComponent) {
    statsComponent.updateStats(simulationState.stats);
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

// ============================================================================
// Dashboard Pilot - Debug/Test Driving API for Dashboard
// ============================================================================

/**
 * window.dashboardPilot provides a testing API for the dashboard.
 * Use this for E2E tests to verify dashboard behavior.
 */
window.dashboardPilot = {
  // --- State Access ---

  /** Get current simulation state */
  getSimulationState() {
    return {
      time: simulationState.time,
      speed: simulationState.speed,
      isPlaying: simulationState.isPlaying,
      instanceId: simulationState.instanceId,
      currentState: { ...simulationState.currentState },
      stats: { ...simulationState.stats }
    };
  },

  /** Get current resource levels */
  getResources() {
    return {
      coffee_beans: simulationState.currentState.coffee_beans || 0,
      milk: simulationState.currentState.milk || 0,
      cups: simulationState.currentState.cups || 0
    };
  },

  /** Get current order state */
  getOrders() {
    return {
      orders_pending: simulationState.currentState.orders_pending || 0,
      espresso_ready: simulationState.currentState.espresso_ready || 0,
      latte_ready: simulationState.currentState.latte_ready || 0,
      cappuccino_ready: simulationState.currentState.cappuccino_ready || 0,
      orders_complete: simulationState.currentState.orders_complete || 0
    };
  },

  /** Get current health state classification with severity */
  getHealth() {
    const health = classifyHealthState(simulationState.currentState);
    return {
      ...health,
      severity: getHealthSeverity(health)
    };
  },

  /** Get all stats */
  getStats() {
    return { ...simulationState.stats };
  },

  // --- Simulation Controls ---

  /** Start the simulation */
  play() {
    if (!simulationState.isPlaying) {
      simulationState.isPlaying = true;
      startSimulation();
      // Update component via attribute if it exists
      if (controlsComponent && controlsComponent.setAttribute) {
        controlsComponent.setAttribute('playing', 'true');
      }
    }
    return { playing: true };
  },

  /** Pause the simulation */
  pause() {
    if (simulationState.isPlaying) {
      simulationState.isPlaying = false;
      stopSimulation();
      // Update component via attribute if it exists
      if (controlsComponent && controlsComponent.setAttribute) {
        controlsComponent.setAttribute('playing', 'false');
      }
    }
    return { playing: false };
  },

  /** Check if simulation is playing */
  isPlaying() {
    return simulationState.isPlaying === true;
  },

  /** Set simulation speed (1, 2, 5, 10) */
  setSpeed(speed) {
    simulationState.speed = speed;
    if (simulationState.isPlaying) {
      stopSimulation();
      startSimulation();
    }
    // Update component via attribute if it exists
    if (controlsComponent && controlsComponent.setAttribute) {
      controlsComponent.setAttribute('speed', String(speed));
    }
    return { speed };
  },

  /** Reset simulation - creates new instance with full resources */
  async reset() {
    // Clear queue history to reset trend calculations
    queueHistory.length = 0;

    // Reset to initial state with max resources
    simulationState.currentState = {
      coffee_beans: RESOURCE_MAX_VALUES.coffee_beans,
      milk: RESOURCE_MAX_VALUES.milk,
      cups: RESOURCE_MAX_VALUES.cups,
      orders_pending: 0,
      espresso_ready: 0,
      latte_ready: 0,
      cappuccino_ready: 0,
      orders_complete: 0
    };
    simulationState.time = 0;
    simulationState.stats = {
      drinksServed: 0,
      ordersPerHour: 0,
      resourceEfficiency: 100
    };

    updateUI();
    return this.getSimulationState();
  },

  // --- State Manipulation ---

  /** Set state directly for testing (also clears queue history) */
  setState(newState) {
    // Clear queue history to ensure clean trend calculation
    queueHistory.length = 0;
    Object.assign(simulationState.currentState, newState);
    updateUI();
    return this.getSimulationState();
  },

  /** Restock a specific resource to max */
  restock(resource) {
    const maxValues = {
      coffee_beans: RESOURCE_MAX_VALUES.coffee_beans,
      milk: RESOURCE_MAX_VALUES.milk,
      cups: RESOURCE_MAX_VALUES.cups
    };
    if (maxValues[resource]) {
      simulationState.currentState[resource] = maxValues[resource];
      updateUI();
    }
    return this.getResources();
  },

  /** Restock all resources to max */
  restockAll() {
    simulationState.currentState.coffee_beans = RESOURCE_MAX_VALUES.coffee_beans;
    simulationState.currentState.milk = RESOURCE_MAX_VALUES.milk;
    simulationState.currentState.cups = RESOURCE_MAX_VALUES.cups;
    updateUI();
    return this.getResources();
  },

  /** Apply a test state for health testing (clears queue history for accurate classification) */
  applyTestState(testState) {
    const testStates = {
      healthy: {
        coffee_beans: 1000,
        milk: 500,
        cups: 200,
        orders_pending: 0,
        orders_complete: 50
      },
      busy: {
        coffee_beans: 800,
        milk: 400,
        cups: 150,
        orders_pending: 7,  // > 5 but low breach rate
        orders_complete: 500  // Low breach rate: 7/(500+7) = 1.4%
      },
      stressed: {
        coffee_beans: 600,
        milk: 300,
        cups: 100,
        orders_pending: 12,  // > 10
        orders_complete: 200
      },
      sla_crisis: {
        coffee_beans: 500,
        milk: 250,
        cups: 80,
        orders_pending: 20,
        orders_complete: 40  // 20/(40+20) = 33% breach
      },
      inventory_crisis: {
        coffee_beans: 150,  // 7.5% of 2000
        milk: 80,  // 8% of 1000
        cups: 40,  // 8% of 500
        orders_pending: 0,
        orders_complete: 100
      },
      critical: {
        coffee_beans: 0,
        milk: 50,
        cups: 20,
        orders_pending: 5,
        orders_complete: 50
      }
    };

    const state = testStates[testState];
    if (state) {
      // Clear queue history to ensure accurate health classification
      queueHistory.length = 0;
      Object.assign(simulationState.currentState, state);
      updateUI();
      return {
        testState,
        state: this.getSimulationState().currentState,
        health: this.getHealth()
      };
    }
    return { error: `Unknown test state: ${testState}` };
  },

  // --- Event Rates ---

  /** Get current event rates */
  getRates() {
    if (rateComponent) {
      return rateComponent.getRates();
    }
    return simulationState.rates;
  },

  /** Set event rates (updates internal state, component reads on next getRates) */
  setRates(rates) {
    // Update simulation state rates
    Object.assign(simulationState.rates, rates);
    // If component exists, update its internal rates
    if (rateComponent && rateComponent.rates) {
      Object.assign(rateComponent.rates, rates);
    }
    return this.getRates();
  },

  /** Apply a preset */
  applyPreset(preset) {
    if (rateComponent && rateComponent.applyPreset) {
      rateComponent.applyPreset(preset);
    }
    return this.getRates();
  },

  // --- UI Component Access ---

  /** Get component presence (for initialization tests) */
  getComponents() {
    return {
      scene: !!sceneComponent,
      gauges: !!gaugesComponent,
      flow: !!flowComponent,
      rate: !!rateComponent,
      controls: !!controlsComponent,
      stress: !!stressComponent,
      stats: !!statsComponent
    };
  },

  /** Check if dashboard is initialized */
  isInitialized() {
    return isDashboardActive && simulationState.instanceId !== null;
  },

  /** Wait for dashboard to be initialized */
  async waitForInit(timeoutMs = 10000) {
    const start = Date.now();
    while (Date.now() - start < timeoutMs) {
      if (this.isInitialized()) {
        return true;
      }
      await new Promise(r => setTimeout(r, 100));
    }
    return false;
  },

  // --- Direct Transitions ---

  /** Execute a transition directly */
  async executeTransition(transition) {
    await executeTransition(transition);
    return this.getSimulationState();
  }
};
