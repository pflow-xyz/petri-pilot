/**
 * Animated Coffee Shop Dashboard
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
    resourceEfficiency: 100
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
    controlsComponent.addEventListener('jump-runout', handleJumpToRunout);
  }

  if (rateComponent) {
    rateComponent.addEventListener('rate-change', handleRateChange);
    rateComponent.addEventListener('preset-applied', handlePresetApplied);
  }

  if (gaugesComponent) {
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
    const response = await fetch(`${API_BASE}/api/coffeeshop/runout`);
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
    const response = await fetch(`${API_BASE}/api/coffeeshop/predict?hours=${hours}`);
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
 * Handle restock
 */
async function handleRestock(event) {
  const resource = event.detail.resource;
  
  // For simplicity, we'll just reload the instance
  // In a real app, you'd have a restock API endpoint
  if (stressComponent) {
    stressComponent.addAlert(`Restocking ${resource}...`, 'info');
  }
  
  // Reset to initial values
  await handleReset();
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
