/**
 * Animated Coffee Shop Dashboard Components
 * Web Components for real-time visualization
 */

// ============================================================================
// Base Component Class
// ============================================================================
class CoffeeShopComponent extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  emit(eventName, detail = {}) {
    this.dispatchEvent(new CustomEvent(eventName, { 
      detail, 
      bubbles: true, 
      composed: true 
    }));
  }

  $(selector) {
    return this.shadowRoot.querySelector(selector);
  }

  $$(selector) {
    return this.shadowRoot.querySelectorAll(selector);
  }
}

// ============================================================================
// Coffee Shop Scene Component
// ============================================================================
class CoffeeShopScene extends CoffeeShopComponent {
  static get observedAttributes() {
    return ['customers', 'barista-busy', 'ready-drinks', 'brew-progress', 'mood'];
  }

  connectedCallback() {
    this.render();
  }

  attributeChangedCallback() {
    this.updateScene();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
          background: linear-gradient(to bottom, #FFF8E1 0%, #FFECB3 100%);
          border-radius: 12px;
          padding: 2rem;
          position: relative;
          min-height: 340px;
          overflow: hidden;
        }

        .scene-container {
          display: grid;
          grid-template-columns: 1fr 2fr 1fr;
          gap: 2rem;
          align-items: center;
          height: 100%;
          padding-bottom: 3.5rem; /* Make room for inventory bar */
        }

        .queue-area {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 0.5rem;
        }

        .queue-label {
          font-size: 0.9rem;
          font-weight: 600;
          color: #3E2723;
          margin-bottom: 0.5rem;
        }

        .customer {
          font-size: 2rem;
          animation: customer-arrive 0.5s ease-out;
          transform-origin: center;
        }

        @keyframes customer-arrive {
          from {
            opacity: 0;
            transform: translateX(-30px) scale(0.5);
          }
          to {
            opacity: 1;
            transform: translateX(0) scale(1);
          }
        }

        .barista-station {
          display: flex;
          flex-direction: column;
          align-items: center;
          background: #8D6E63;
          border-radius: 16px;
          padding: 2rem;
          box-shadow: 0 4px 12px rgba(0,0,0,0.2);
          position: relative;
        }

        .barista {
          font-size: 3rem;
          margin-bottom: 1rem;
        }

        .barista.busy {
          animation: barista-work 1s ease-in-out infinite;
        }

        @keyframes barista-work {
          0%, 100% { transform: rotate(-5deg); }
          50% { transform: rotate(5deg); }
        }

        .coffee-machine {
          width: 80%;
          height: 80px;
          background: linear-gradient(to bottom, #424242, #212121);
          border-radius: 8px;
          position: relative;
          margin-top: 0.5rem;
          overflow: hidden;
          box-shadow: inset 0 2px 4px rgba(0,0,0,0.3);
        }

        .brew-progress-container {
          position: absolute;
          bottom: 8px;
          left: 8px;
          right: 8px;
          height: 20px;
          background: #1a1a1a;
          border-radius: 4px;
          overflow: hidden;
          border: 1px solid #333;
        }

        .brew-progress-bar {
          height: 100%;
          width: 0%;
          background: linear-gradient(to right, #6D4C41, #8D6E63, #A1887F);
          border-radius: 3px;
          transition: width 0.3s ease;
          position: relative;
        }

        .brew-progress-bar.active {
          animation: brew-pulse 1s ease-in-out infinite;
        }

        @keyframes brew-pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.7; }
        }

        .brew-progress-text {
          position: absolute;
          top: 50%;
          left: 50%;
          transform: translate(-50%, -50%);
          font-size: 10px;
          font-weight: bold;
          color: #FFF8E1;
          text-shadow: 0 1px 2px rgba(0,0,0,0.5);
          z-index: 1;
        }

        .machine-display {
          position: absolute;
          top: 8px;
          left: 50%;
          transform: translateX(-50%);
          background: #1a1a1a;
          padding: 4px 12px;
          border-radius: 4px;
          font-size: 11px;
          font-family: monospace;
          color: #4CAF50;
          border: 1px solid #333;
        }

        .machine-display.brewing {
          color: #FFD54F;
          animation: display-blink 0.5s ease-in-out infinite;
        }

        @keyframes display-blink {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.6; }
        }

        .mood-indicator {
          position: absolute;
          top: 8px;
          right: 8px;
          background: white;
          border-radius: 20px;
          padding: 4px 10px;
          font-size: 0.75rem;
          font-weight: 600;
          box-shadow: 0 2px 8px rgba(0,0,0,0.2);
          display: flex;
          align-items: center;
          gap: 4px;
          transition: all 0.3s ease;
        }

        .mood-indicator.relaxed {
          background: #E8F5E9;
          color: #2E7D32;
        }

        .mood-indicator.normal {
          background: #FFF8E1;
          color: #6D4C41;
        }

        .mood-indicator.busy {
          background: #FFF3E0;
          color: #E65100;
        }

        .mood-indicator.stressed {
          background: #FFEBEE;
          color: #C62828;
          animation: mood-pulse 1s ease-in-out infinite;
        }

        .mood-indicator.overwhelmed {
          background: #F44336;
          color: white;
          animation: mood-shake 0.5s ease-in-out infinite;
        }

        @keyframes mood-pulse {
          0%, 100% { transform: scale(1); }
          50% { transform: scale(1.05); }
        }

        @keyframes mood-shake {
          0%, 100% { transform: translateX(0); }
          25% { transform: translateX(-2px); }
          75% { transform: translateX(2px); }
        }

        .mood-emoji {
          font-size: 1rem;
        }

        .steam {
          position: absolute;
          top: -30px;
          left: 50%;
          transform: translateX(-50%);
          font-size: 1.5rem;
          opacity: 0;
          animation: steam-rise 2s ease-in-out infinite;
        }

        .barista.busy .steam {
          opacity: 1;
        }

        @keyframes steam-rise {
          0% {
            opacity: 0;
            transform: translateX(-50%) translateY(0);
          }
          50% {
            opacity: 0.8;
          }
          100% {
            opacity: 0;
            transform: translateX(-50%) translateY(-20px);
          }
        }

        .serving-counter {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 0.5rem;
        }

        .counter-label {
          font-size: 0.9rem;
          font-weight: 600;
          color: #3E2723;
          margin-bottom: 0.5rem;
        }

        .drink {
          font-size: 2rem;
          animation: drink-ready 0.5s ease-out;
        }

        @keyframes drink-ready {
          from {
            opacity: 0;
            transform: scale(0.5) translateY(-20px);
          }
          to {
            opacity: 1;
            transform: scale(1) translateY(0);
          }
        }

        .inventory-bar {
          position: absolute;
          bottom: 1rem;
          left: 1rem;
          right: 1rem;
          display: flex;
          gap: 1rem;
          background: rgba(255,255,255,0.9);
          padding: 0.75rem;
          border-radius: 8px;
        }

        .inventory-item {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          font-size: 0.85rem;
          color: #3E2723;
        }

        .inventory-icon {
          font-size: 1.2rem;
        }
      </style>

      <div class="scene-container">
        <div class="queue-area">
          <div class="queue-label">Queue</div>
          <div id="customer-queue"></div>
        </div>

        <div class="barista-station">
          <div class="mood-indicator normal" id="mood-indicator">
            <span class="mood-emoji">😊</span>
            <span class="mood-text">Relaxed</span>
          </div>
          <div class="barista" id="barista">👨‍🍳</div>
          <div class="steam">💨</div>
          <div class="coffee-machine">
            <div class="machine-display" id="machine-display">READY</div>
            <div class="brew-progress-container">
              <div class="brew-progress-bar" id="brew-progress"></div>
              <div class="brew-progress-text" id="brew-text">0%</div>
            </div>
          </div>
        </div>

        <div class="serving-counter">
          <div class="counter-label">Ready</div>
          <div id="ready-drinks"></div>
        </div>
      </div>

      <div class="inventory-bar">
        <div class="inventory-item">
          <span class="inventory-icon">☕</span>
          <span id="beans-count">1000g</span>
        </div>
        <div class="inventory-item">
          <span class="inventory-icon">🥛</span>
          <span id="milk-count">500ml</span>
        </div>
        <div class="inventory-item">
          <span class="inventory-icon">🥤</span>
          <span id="cups-count">200</span>
        </div>
      </div>
    `;
  }

  updateScene() {
    const customers = parseInt(this.getAttribute('customers') || '0');
    const baristaBusy = this.getAttribute('barista-busy') === 'true';
    const readyDrinks = parseInt(this.getAttribute('ready-drinks') || '0');

    // Update queue
    const queueEl = this.$('#customer-queue');
    if (queueEl) {
      queueEl.innerHTML = Array(Math.min(customers, 5))
        .fill('👤')
        .map(icon => `<div class="customer">${icon}</div>`)
        .join('');
    }

    // Update barista
    const baristaEl = this.$('#barista');
    if (baristaEl) {
      if (baristaBusy) {
        baristaEl.classList.add('busy');
      } else {
        baristaEl.classList.remove('busy');
      }
    }

    // Update brew progress
    const brewProgress = parseInt(this.getAttribute('brew-progress') || '0');
    const progressBar = this.$('#brew-progress');
    const progressText = this.$('#brew-text');
    const machineDisplay = this.$('#machine-display');

    if (progressBar && progressText && machineDisplay) {
      progressBar.style.width = `${brewProgress}%`;
      progressText.textContent = `${brewProgress}%`;

      if (baristaBusy && brewProgress > 0) {
        progressBar.classList.add('active');
        machineDisplay.classList.add('brewing');
        machineDisplay.textContent = 'BREWING...';
      } else if (brewProgress >= 100) {
        progressBar.classList.remove('active');
        machineDisplay.classList.remove('brewing');
        machineDisplay.textContent = 'DONE!';
      } else {
        progressBar.classList.remove('active');
        machineDisplay.classList.remove('brewing');
        machineDisplay.textContent = 'READY';
      }
    }

    // Update ready drinks
    const drinksEl = this.$('#ready-drinks');
    if (drinksEl) {
      drinksEl.innerHTML = Array(Math.min(readyDrinks, 5))
        .fill('☕')
        .map(icon => `<div class="drink">${icon}</div>`)
        .join('');
    }

    // Update mood indicator
    const mood = this.getAttribute('mood') || 'relaxed';
    const moodIndicator = this.$('#mood-indicator');
    if (moodIndicator) {
      const moods = {
        relaxed: { emoji: '😊', text: 'Relaxed', class: 'relaxed' },
        normal: { emoji: '🙂', text: 'Normal', class: 'normal' },
        busy: { emoji: '😅', text: 'Busy', class: 'busy' },
        stressed: { emoji: '😰', text: 'Stressed', class: 'stressed' },
        overwhelmed: { emoji: '🤯', text: 'Overwhelmed!', class: 'overwhelmed' }
      };
      const moodData = moods[mood] || moods.normal;

      moodIndicator.className = `mood-indicator ${moodData.class}`;
      moodIndicator.innerHTML = `
        <span class="mood-emoji">${moodData.emoji}</span>
        <span class="mood-text">${moodData.text}</span>
      `;
    }
  }

  updateInventory(beans, milk, cups) {
    const beansEl = this.$('#beans-count');
    const milkEl = this.$('#milk-count');
    const cupsEl = this.$('#cups-count');

    if (beansEl) beansEl.textContent = `${beans}g`;
    if (milkEl) milkEl.textContent = `${milk}ml`;
    if (cupsEl) cupsEl.textContent = `${cups}`;
  }
}

// ============================================================================
// Resource Gauges Component
// ============================================================================
class ResourceGauges extends CoffeeShopComponent {
  connectedCallback() {
    this.resources = {
      coffee_beans: { current: 1000, max: 2000, label: 'Coffee Beans', unit: 'g', icon: '☕' },
      milk: { current: 500, max: 1000, label: 'Milk', unit: 'ml', icon: '🥛' },
      cups: { current: 200, max: 500, label: 'Cups', unit: '', icon: '🥤' }
    };
    this.render();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .gauges-container {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
          gap: 1rem;
        }

        .gauge-card {
          background: white;
          border-radius: 12px;
          padding: 1.5rem;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }

        .gauge-header {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          margin-bottom: 1rem;
        }

        .gauge-icon {
          font-size: 1.5rem;
        }

        .gauge-label {
          font-weight: 600;
          color: #3E2723;
        }

        .gauge-bar {
          height: 24px;
          background: #E0E0E0;
          border-radius: 12px;
          overflow: hidden;
          position: relative;
        }

        .gauge-fill {
          height: 100%;
          border-radius: 12px;
          transition: width 0.5s ease-out, background-color 0.5s ease;
          position: relative;
        }

        .gauge-fill.green {
          background: linear-gradient(to right, #4CAF50, #66BB6A);
        }

        .gauge-fill.yellow {
          background: linear-gradient(to right, #FFC107, #FFD54F);
        }

        .gauge-fill.red {
          background: linear-gradient(to right, #F44336, #EF5350);
          animation: pulse-red 1s ease-in-out infinite;
        }

        @keyframes pulse-red {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.7; }
        }

        .gauge-value {
          margin-top: 0.5rem;
          display: flex;
          justify-content: space-between;
          font-size: 0.9rem;
          color: #666;
        }

        .gauge-actions {
          margin-top: 0.75rem;
        }

        .restock-btn {
          width: 100%;
          padding: 0.5rem;
          background: #6D4C41;
          color: white;
          border: none;
          border-radius: 6px;
          cursor: pointer;
          font-weight: 600;
          transition: background 0.2s;
        }

        .restock-btn:hover {
          background: #5D4037;
        }
      </style>

      <div class="gauges-container" id="gauges"></div>
    `;

    this.renderGauges();
  }

  renderGauges() {
    const container = this.$('#gauges');
    if (!container) return;

    container.innerHTML = Object.entries(this.resources).map(([key, res]) => `
      <div class="gauge-card">
        <div class="gauge-header">
          <span class="gauge-icon">${res.icon}</span>
          <span class="gauge-label">${res.label}</span>
        </div>
        <div class="gauge-bar">
          <div class="gauge-fill" id="fill-${key}"></div>
        </div>
        <div class="gauge-value">
          <span id="value-${key}">${res.current}${res.unit}</span>
          <span>${res.max}${res.unit}</span>
        </div>
        <div class="gauge-actions">
          <button class="restock-btn" data-resource="${key}">Restock</button>
        </div>
      </div>
    `).join('');

    // Add event listeners
    this.$$('.restock-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const resource = e.target.dataset.resource;
        this.emit('restock', { resource });
      });
    });

    this.updateAllGauges();
  }

  updateResource(resource, value) {
    if (!this.resources[resource]) return;
    
    this.resources[resource].current = value;
    this.updateGauge(resource);
  }

  updateGauge(resource) {
    const res = this.resources[resource];
    const percentage = (res.current / res.max) * 100;
    const fill = this.$(`#fill-${resource}`);
    const valueEl = this.$(`#value-${resource}`);

    if (fill) {
      fill.style.width = `${Math.max(0, percentage)}%`;
      
      // Color based on percentage
      fill.classList.remove('green', 'yellow', 'red');
      if (percentage > 50) {
        fill.classList.add('green');
      } else if (percentage > 20) {
        fill.classList.add('yellow');
      } else {
        fill.classList.add('red');
      }
    }

    if (valueEl) {
      valueEl.textContent = `${Math.floor(res.current)}${res.unit}`;
    }
  }

  updateAllGauges() {
    Object.keys(this.resources).forEach(key => this.updateGauge(key));
  }
}

// ============================================================================
// Order Flow Board Component
// ============================================================================
class OrderFlowBoard extends CoffeeShopComponent {
  connectedCallback() {
    this.orders = {
      pending: [],
      preparing: [],
      ready: [],
      served: []
    };
    this.render();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .flow-container {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: 1rem;
        }

        .flow-lane {
          background: white;
          border-radius: 12px;
          padding: 1rem;
          min-height: 200px;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }

        .lane-header {
          font-weight: 600;
          color: #3E2723;
          margin-bottom: 1rem;
          padding-bottom: 0.5rem;
          border-bottom: 2px solid #FFECB3;
        }

        .order-card {
          background: #FFF8E1;
          border-radius: 8px;
          padding: 0.75rem;
          margin-bottom: 0.5rem;
          display: flex;
          align-items: center;
          gap: 0.5rem;
          cursor: grab;
        }

        .order-icon {
          font-size: 1.5rem;
        }

        .order-type {
          font-size: 0.85rem;
          font-weight: 600;
          color: #3E2723;
        }

        .order-time {
          font-size: 0.75rem;
          color: #666;
          margin-left: auto;
        }

        .order-card.fresh {
          background: #E8F5E9;
          border-left: 3px solid #4CAF50;
        }

        .order-card.warning {
          background: #FFF3E0;
          border-left: 3px solid #FF9800;
        }

        .order-card.critical {
          background: #FFEBEE;
          border-left: 3px solid #F44336;
        }
      </style>

      <div class="flow-container">
        <div class="flow-lane">
          <div class="lane-header">Pending</div>
          <div id="lane-pending"></div>
        </div>
        <div class="flow-lane">
          <div class="lane-header">Preparing</div>
          <div id="lane-preparing"></div>
        </div>
        <div class="flow-lane">
          <div class="lane-header">Ready</div>
          <div id="lane-ready"></div>
        </div>
        <div class="flow-lane">
          <div class="lane-header">Served</div>
          <div id="lane-served"></div>
        </div>
      </div>
    `;
  }

  addOrder(type, lane = 'pending', simTime = null) {
    const order = {
      id: Date.now() + Math.random(), // Unique ID
      type,
      simTime: simTime, // Simulation time when order was placed
      timestamp: Date.now(), // Real time for display updates
      icon: this.getOrderIcon(type)
    };

    this.orders[lane].push(order);
    this.renderLane(lane);
  }

  moveOrder(orderId, fromLane, toLane) {
    const orderIndex = this.orders[fromLane].findIndex(o => o.id === orderId);
    if (orderIndex === -1) return;

    const [order] = this.orders[fromLane].splice(orderIndex, 1);
    this.orders[toLane].push(order);
    
    this.renderLane(fromLane);
    this.renderLane(toLane);
  }

  getOrderIcon(type) {
    const icons = {
      espresso: '☕',
      latte: '🥛☕',
      cappuccino: '☕🥛'
    };
    return icons[type] || '☕';
  }

  renderLane(lane, currentSimTime = null, patienceWarning = 180, patienceCritical = 240, patienceThreshold = 300) {
    const container = this.$(`#lane-${lane}`);
    if (!container) return;

    const orders = this.orders[lane].slice(-5); // Show max 5
    container.innerHTML = orders.map(order => {
      let waitTime = 0;
      let patienceClass = 'fresh';
      let displayTime = '';

      if (order.simTime !== null && currentSimTime !== null) {
        // Use simulation time for wait calculation
        waitTime = currentSimTime - order.simTime;
        if (waitTime >= patienceCritical) {
          patienceClass = 'critical';
        } else if (waitTime >= patienceWarning) {
          patienceClass = 'warning';
        }
        // Format wait time in minutes:seconds
        const mins = Math.floor(waitTime / 60);
        const secs = Math.floor(waitTime % 60);
        displayTime = mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
      } else {
        // Fallback to real time
        const elapsed = Math.floor((Date.now() - order.timestamp) / 1000);
        displayTime = `${elapsed}s`;
      }

      return `
        <div class="order-card ${patienceClass}" data-id="${order.id}">
          <span class="order-icon">${order.icon}</span>
          <span class="order-type">${order.type}</span>
          <span class="order-time">${displayTime}</span>
        </div>
      `;
    }).join('');
  }

  removeOrder(orderId, lane) {
    const orderIndex = this.orders[lane].findIndex(o => o.id === orderId);
    if (orderIndex === -1) return;

    this.orders[lane].splice(orderIndex, 1);
    this.renderLane(lane);
  }

  updatePatience(currentSimTime, patienceWarning, patienceCritical, patienceThreshold) {
    // Update patience styling in place without re-rendering (avoids animation flicker)
    const container = this.$('#lane-pending');
    if (!container) return;

    const cards = container.querySelectorAll('.order-card');
    const pendingOrders = this.orders.pending.slice(-5); // Match what's displayed

    cards.forEach((card, index) => {
      const order = pendingOrders[index];
      if (!order || order.simTime === null) return;

      const waitTime = currentSimTime - order.simTime;

      // Update patience class
      card.classList.remove('fresh', 'warning', 'critical');
      if (waitTime >= patienceCritical) {
        card.classList.add('critical');
      } else if (waitTime >= patienceWarning) {
        card.classList.add('warning');
      } else {
        card.classList.add('fresh');
      }

      // Update time display
      const timeEl = card.querySelector('.order-time');
      if (timeEl) {
        const mins = Math.floor(waitTime / 60);
        const secs = Math.floor(waitTime % 60);
        timeEl.textContent = mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
      }
    });
  }

  reset() {
    this.orders = {
      pending: [],
      preparing: [],
      ready: [],
      served: []
    };
    ['pending', 'preparing', 'ready', 'served'].forEach(lane => this.renderLane(lane));
  }
}

// ============================================================================
// Rate Configuration Panel
// ============================================================================
class RateConfigPanel extends CoffeeShopComponent {
  connectedCallback() {
    this.rates = {
      order_espresso: 10,
      order_latte: 15,
      order_cappuccino: 8,
      make_espresso: 20,
      make_latte: 12,
      make_cappuccino: 10,
      serve_espresso: 30,
      serve_latte: 30,
      serve_cappuccino: 30
    };
    this.patienceMinutes = 5; // Default 5 minutes before customers leave
    this.render();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .config-container {
          background: white;
          border-radius: 12px;
          padding: 1.5rem;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }

        .config-header {
          font-weight: 600;
          color: #3E2723;
          margin-bottom: 1rem;
          font-size: 1.1rem;
        }

        .rate-group {
          margin-bottom: 1.5rem;
        }

        .group-title {
          font-weight: 600;
          color: #6D4C41;
          margin-bottom: 0.75rem;
          font-size: 0.9rem;
        }

        .rate-slider {
          display: flex;
          align-items: center;
          gap: 1rem;
          margin-bottom: 0.75rem;
        }

        .rate-label {
          flex: 1;
          font-size: 0.85rem;
          color: #333;
        }

        .rate-input {
          width: 150px;
        }

        .rate-value {
          min-width: 60px;
          text-align: right;
          font-weight: 600;
          color: #3E2723;
          font-size: 0.85rem;
        }

        input[type="range"] {
          flex: 1;
          min-width: 100px;
          -webkit-appearance: none;
          appearance: none;
          height: 8px;
          background: linear-gradient(to right, #FFECB3, #A1887F);
          border-radius: 4px;
          outline: none;
          cursor: pointer;
        }

        input[type="range"]::-webkit-slider-thumb {
          -webkit-appearance: none;
          appearance: none;
          width: 20px;
          height: 20px;
          background: linear-gradient(to bottom, #6D4C41, #3E2723);
          border-radius: 50%;
          cursor: pointer;
          box-shadow: 0 2px 4px rgba(62, 39, 35, 0.3);
          border: 2px solid #FFF8E1;
          transition: transform 0.15s ease, box-shadow 0.15s ease;
        }

        input[type="range"]::-webkit-slider-thumb:hover {
          transform: scale(1.1);
          box-shadow: 0 4px 8px rgba(62, 39, 35, 0.4);
        }

        input[type="range"]::-moz-range-track {
          height: 8px;
          background: linear-gradient(to right, #FFECB3, #A1887F);
          border-radius: 4px;
        }

        input[type="range"]::-moz-range-thumb {
          width: 20px;
          height: 20px;
          background: linear-gradient(to bottom, #6D4C41, #3E2723);
          border-radius: 50%;
          cursor: pointer;
          box-shadow: 0 2px 4px rgba(62, 39, 35, 0.3);
          border: 2px solid #FFF8E1;
        }

        .presets {
          display: flex;
          gap: 0.5rem;
          flex-wrap: wrap;
          margin-top: 1rem;
          padding-top: 1rem;
          border-top: 1px solid #E0E0E0;
        }

        .preset-btn {
          padding: 0.5rem 1rem;
          background: #FFECB3;
          border: 1px solid #FFD54F;
          border-radius: 6px;
          cursor: pointer;
          font-weight: 600;
          color: #3E2723;
          transition: all 0.2s;
        }

        .preset-btn:hover {
          background: #FFD54F;
          transform: translateY(-1px);
        }
      </style>

      <div class="config-container">
        <div class="config-header">⚙️ Rate Configuration</div>

        <div class="rate-group">
          <div class="group-title">Customer Arrivals (orders/hour)</div>
          ${this.renderSlider('order_espresso', 'Espresso', 0, 50)}
          ${this.renderSlider('order_latte', 'Latte', 0, 50)}
          ${this.renderSlider('order_cappuccino', 'Cappuccino', 0, 50)}
        </div>

        <div class="rate-group">
          <div class="group-title">Preparation Speed (drinks/hour)</div>
          ${this.renderSlider('make_espresso', 'Espresso', 0, 60)}
          ${this.renderSlider('make_latte', 'Latte', 0, 60)}
          ${this.renderSlider('make_cappuccino', 'Cappuccino', 0, 60)}
        </div>

        <div class="rate-group">
          <div class="group-title">Serving Speed (drinks/hour)</div>
          ${this.renderSlider('serve_espresso', 'Espresso', 0, 60)}
          ${this.renderSlider('serve_latte', 'Latte', 0, 60)}
          ${this.renderSlider('serve_cappuccino', 'Cappuccino', 0, 60)}
        </div>

        <div class="rate-group">
          <div class="group-title">🚶 Customer Patience</div>
          <div class="rate-slider">
            <label class="rate-label">Walk out after</label>
            <input type="range"
              class="rate-input"
              id="patience"
              min="1"
              max="15"
              value="${this.patienceMinutes}"
              step="1">
            <span class="rate-value" id="patience-value">${this.patienceMinutes} min</span>
          </div>
        </div>

        <div class="presets">
          <button class="preset-btn" data-preset="slow">☕ Slow Day</button>
          <button class="preset-btn" data-preset="normal">📊 Normal</button>
          <button class="preset-btn" data-preset="rush">🔥 Rush Hour</button>
          <button class="preset-btn" data-preset="stressed">😰 Stressed</button>
        </div>
      </div>
    `;

    this.attachEventListeners();
  }

  renderSlider(id, label, min, max) {
    const value = this.rates[id];
    return `
      <div class="rate-slider">
        <label class="rate-label">${label}</label>
        <input type="range" 
          class="rate-input" 
          id="${id}" 
          min="${min}" 
          max="${max}" 
          value="${value}"
          step="1">
        <span class="rate-value" id="${id}-value">${value}</span>
      </div>
    `;
  }

  attachEventListeners() {
    // Sliders
    Object.keys(this.rates).forEach(key => {
      const slider = this.$(`#${key}`);
      if (slider) {
        slider.addEventListener('input', (e) => {
          const value = parseInt(e.target.value);
          this.rates[key] = value;
          const valueEl = this.$(`#${key}-value`);
          if (valueEl) valueEl.textContent = value;
          this.emit('rate-change', { transition: key, rate: value });
        });
      }
    });

    // Patience slider
    const patienceSlider = this.$('#patience');
    if (patienceSlider) {
      patienceSlider.addEventListener('input', (e) => {
        const value = parseInt(e.target.value);
        this.patienceMinutes = value;
        const valueEl = this.$('#patience-value');
        if (valueEl) valueEl.textContent = `${value} min`;
        this.emit('patience-change', { minutes: value });
      });
    }

    // Presets
    this.$$('.preset-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const preset = e.target.dataset.preset;
        this.applyPreset(preset);
      });
    });
  }

  applyPreset(preset) {
    const presets = {
      slow: {
        order_espresso: 5, order_latte: 7, order_cappuccino: 3,
        make_espresso: 20, make_latte: 12, make_cappuccino: 10,
        serve_espresso: 30, serve_latte: 30, serve_cappuccino: 30
      },
      normal: {
        order_espresso: 10, order_latte: 15, order_cappuccino: 8,
        make_espresso: 20, make_latte: 12, make_cappuccino: 10,
        serve_espresso: 30, serve_latte: 30, serve_cappuccino: 30
      },
      rush: {
        order_espresso: 25, order_latte: 30, order_cappuccino: 20,
        make_espresso: 30, make_latte: 20, make_cappuccino: 18,
        serve_espresso: 40, serve_latte: 40, serve_cappuccino: 40
      },
      stressed: {
        order_espresso: 40, order_latte: 45, order_cappuccino: 35,
        make_espresso: 25, make_latte: 15, make_cappuccino: 12,
        serve_espresso: 30, serve_latte: 30, serve_cappuccino: 30
      }
    };

    const rates = presets[preset];
    if (!rates) return;

    Object.entries(rates).forEach(([key, value]) => {
      this.rates[key] = value;
      const slider = this.$(`#${key}`);
      const valueEl = this.$(`#${key}-value`);
      if (slider) slider.value = value;
      if (valueEl) valueEl.textContent = value;
    });

    this.emit('preset-applied', { preset, rates });
  }

  getRates() {
    return { ...this.rates };
  }

  getPatienceMinutes() {
    return this.patienceMinutes;
  }
}

// ============================================================================
// Simulation Controls
// ============================================================================
class SimulationControls extends CoffeeShopComponent {
  connectedCallback() {
    this.isPlaying = false;
    this.speed = 1;
    this.render();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .controls-container {
          background: white;
          border-radius: 12px;
          padding: 1.5rem;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1);
          display: flex;
          align-items: center;
          justify-content: space-between;
          flex-wrap: wrap;
          gap: 1rem;
        }

        .control-group {
          display: flex;
          align-items: center;
          gap: 0.5rem;
        }

        .control-btn {
          width: 40px;
          height: 40px;
          border: none;
          border-radius: 50%;
          background: #6D4C41;
          color: white;
          font-size: 1.2rem;
          cursor: pointer;
          transition: all 0.2s;
          display: flex;
          align-items: center;
          justify-content: center;
        }

        .control-btn:hover {
          background: #5D4037;
          transform: scale(1.1);
        }

        .control-btn.primary {
          width: 50px;
          height: 50px;
          background: #D84315;
        }

        .control-btn.primary:hover {
          background: #BF360C;
        }

        .speed-controls {
          display: flex;
          gap: 0.25rem;
        }

        .speed-btn {
          padding: 0.5rem 0.75rem;
          border: 1px solid #6D4C41;
          background: white;
          color: #6D4C41;
          border-radius: 6px;
          cursor: pointer;
          font-weight: 600;
          font-size: 0.85rem;
          transition: all 0.2s;
        }

        .speed-btn:hover {
          background: #FFF8E1;
        }

        .speed-btn.active {
          background: #6D4C41;
          color: white;
        }

        .time-display {
          font-size: 1.2rem;
          font-weight: 600;
          color: #3E2723;
          font-family: monospace;
        }

        .jump-btn {
          padding: 0.5rem 1rem;
          background: #FF6F00;
          color: white;
          border: none;
          border-radius: 6px;
          cursor: pointer;
          font-weight: 600;
          transition: background 0.2s;
        }

        .report-btn {
          background: #2E7D32;
          color: white;
          border: none;
          padding: 0.5rem 1rem;
          border-radius: 6px;
          cursor: pointer;
          font-weight: 600;
          transition: background 0.2s;
        }

        .report-btn:hover {
          background: #1B5E20;
        }
      </style>

      <div class="controls-container">
        <div class="control-group">
          <button class="control-btn primary" id="play-pause">▶️</button>
          <button class="control-btn" id="reset">🔄</button>
        </div>

        <div class="time-display" id="time-display">00:00:00</div>

        <div class="speed-controls">
          <button class="speed-btn active" data-speed="1">1x</button>
          <button class="speed-btn" data-speed="2">2x</button>
          <button class="speed-btn" data-speed="5">5x</button>
          <button class="speed-btn" data-speed="10">10x</button>
        </div>

        <button class="report-btn" id="download-report">📊 Download Report</button>
      </div>
    `;

    this.attachEventListeners();
  }

  attachEventListeners() {
    const playPauseBtn = this.$('#play-pause');
    if (playPauseBtn) {
      playPauseBtn.addEventListener('click', () => {
        this.isPlaying = !this.isPlaying;
        playPauseBtn.textContent = this.isPlaying ? '⏸️' : '▶️';
        this.emit('play-pause', { playing: this.isPlaying });
      });
    }

    const resetBtn = this.$('#reset');
    if (resetBtn) {
      resetBtn.addEventListener('click', () => {
        this.isPlaying = false;
        if (playPauseBtn) playPauseBtn.textContent = '▶️';
        this.emit('reset');
      });
    }

    this.$$('.speed-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        this.$$('.speed-btn').forEach(b => b.classList.remove('active'));
        e.target.classList.add('active');
        this.speed = parseInt(e.target.dataset.speed);
        this.emit('speed-change', { speed: this.speed });
      });
    });

    const reportBtn = this.$('#download-report');
    if (reportBtn) {
      reportBtn.addEventListener('click', () => {
        this.emit('download-report');
      });
    }
  }

  updateTime(seconds) {
    const timeEl = this.$('#time-display');
    if (!timeEl) return;

    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);

    timeEl.textContent = `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }
}

// ============================================================================
// Stress Indicator Component
// ============================================================================
class StressIndicator extends CoffeeShopComponent {
  connectedCallback() {
    this.alerts = [];
    this.render();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .alert-container {
          position: fixed;
          top: 80px;
          right: 20px;
          z-index: 1000;
          display: flex;
          flex-direction: column;
          gap: 0.5rem;
          max-width: 400px;
        }

        .alert {
          background: white;
          border-left: 4px solid #FFC107;
          border-radius: 8px;
          padding: 1rem;
          box-shadow: 0 4px 12px rgba(0,0,0,0.15);
          animation: alert-slide 0.3s ease-out;
        }

        .alert.warning {
          border-left-color: #FF9800;
        }

        .alert.critical {
          border-left-color: #F44336;
          animation: alert-pulse 1s ease-in-out infinite;
        }

        @keyframes alert-slide {
          from {
            opacity: 0;
            transform: translateX(100%);
          }
          to {
            opacity: 1;
            transform: translateX(0);
          }
        }

        @keyframes alert-pulse {
          0%, 100% {
            box-shadow: 0 4px 12px rgba(244, 67, 54, 0.3);
          }
          50% {
            box-shadow: 0 4px 20px rgba(244, 67, 54, 0.6);
          }
        }

        .alert-header {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          font-weight: 600;
          color: #3E2723;
          margin-bottom: 0.5rem;
        }

        .alert-icon {
          font-size: 1.2rem;
        }

        .alert-message {
          font-size: 0.9rem;
          color: #666;
        }

        .alert-close {
          margin-left: auto;
          background: none;
          border: none;
          font-size: 1.2rem;
          cursor: pointer;
          opacity: 0.6;
          transition: opacity 0.2s;
        }

        .alert-close:hover {
          opacity: 1;
        }
      </style>

      <div class="alert-container" id="alerts"></div>
    `;
  }

  addAlert(message, type = 'info') {
    const alert = {
      id: Date.now(),
      message,
      type,
      timestamp: Date.now()
    };

    this.alerts.push(alert);
    this.renderAlerts();

    // Auto-dismiss after 5 seconds
    setTimeout(() => {
      this.removeAlert(alert.id);
    }, 5000);
  }

  removeAlert(id) {
    this.alerts = this.alerts.filter(a => a.id !== id);
    this.renderAlerts();
  }

  renderAlerts() {
    const container = this.$('#alerts');
    if (!container) return;

    const icons = {
      info: 'ℹ️',
      warning: '⚠️',
      critical: '🚨'
    };

    container.innerHTML = this.alerts.map(alert => `
      <div class="alert ${alert.type}">
        <div class="alert-header">
          <span class="alert-icon">${icons[alert.type] || 'ℹ️'}</span>
          <span>${alert.type.toUpperCase()}</span>
          <button class="alert-close" data-id="${alert.id}">✕</button>
        </div>
        <div class="alert-message">${alert.message}</div>
      </div>
    `).join('');

    // Attach close handlers
    this.$$('.alert-close').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const id = parseInt(e.target.dataset.id);
        this.removeAlert(id);
      });
    });
  }
}

// ============================================================================
// Statistics Dashboard
// ============================================================================
class StatsDashboard extends CoffeeShopComponent {
  connectedCallback() {
    this.stats = {
      ordersPerHour: { espresso: 0, latte: 0, cappuccino: 0 },
      averageWaitTime: 0,
      drinksServed: 0,
      resourceEfficiency: 100,
      waitTimeSampleCount: 0
    };
    this.selectedWindow = 'all';
    this.render();
    this.attachListeners();
  }

  attachListeners() {
    const select = this.$('#wait-time-window');
    if (select) {
      select.addEventListener('change', (e) => {
        this.selectedWindow = e.target.value;
        this.dispatchEvent(new CustomEvent('wait-time-window-change', {
          detail: { window: this.selectedWindow }
        }));
      });
    }
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .stats-container {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
          gap: 1rem;
        }

        .stat-card {
          background: white;
          border-radius: 12px;
          padding: 1.5rem;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1);
          text-align: center;
        }

        .stat-value {
          font-size: 2rem;
          font-weight: 700;
          color: #3E2723;
          margin-bottom: 0.25rem;
        }

        .stat-label {
          font-size: 0.85rem;
          color: #666;
          font-weight: 600;
        }

        .stat-trend {
          margin-top: 0.5rem;
          font-size: 0.75rem;
        }

        .trend-up {
          color: #4CAF50;
        }

        .trend-down {
          color: #F44336;
        }

        .mini-chart {
          height: 40px;
          margin-top: 0.5rem;
          display: flex;
          align-items: flex-end;
          gap: 2px;
          justify-content: center;
        }

        .chart-bar {
          width: 8px;
          background: #6D4C41;
          border-radius: 2px 2px 0 0;
          transition: height 0.3s ease;
        }

        .abandoned-card .stat-value {
          color: #F44336;
        }

        .stat-subtext {
          font-size: 0.75rem;
          color: #999;
          margin-top: 0.25rem;
        }

        .window-select {
          margin-top: 0.5rem;
          padding: 0.25rem 0.5rem;
          border: 1px solid #A1887F;
          border-radius: 4px;
          background: #FFF8E1;
          color: #5D4037;
          font-size: 0.75rem;
          cursor: pointer;
        }

        .window-select:focus {
          outline: none;
          border-color: #6D4C41;
        }

        .sample-count {
          font-size: 0.7rem;
          color: #999;
          margin-top: 0.25rem;
        }
      </style>

      <div class="stats-container">
        <div class="stat-card">
          <div class="stat-value" id="drinks-served">0</div>
          <div class="stat-label">Drinks Served</div>
          <div class="mini-chart" id="served-chart"></div>
        </div>

        <div class="stat-card">
          <div class="stat-value" id="orders-hour">0</div>
          <div class="stat-label">Orders/Hour</div>
        </div>

        <div class="stat-card">
          <div class="stat-value" id="wait-time">0s</div>
          <div class="stat-label">Avg Wait Time</div>
          <select class="window-select" id="wait-time-window">
            <option value="all">All Time</option>
            <option value="1h">Last Hour</option>
            <option value="30m">Last 30 Min</option>
            <option value="10m">Last 10 Min</option>
            <option value="5m">Last 5 Min</option>
          </select>
          <div class="sample-count" id="sample-count">0 samples</div>
        </div>

        <div class="stat-card">
          <div class="stat-value" id="efficiency">100%</div>
          <div class="stat-label">Efficiency</div>
        </div>

        <div class="stat-card abandoned-card">
          <div class="stat-value" id="abandoned-count">0</div>
          <div class="stat-label">Abandoned</div>
          <div class="stat-subtext" id="abandonment-rate">0% rate</div>
        </div>
      </div>
    `;

    this.initCharts();
  }

  initCharts() {
    const chart = this.$('#served-chart');
    if (!chart) return;

    // Initialize with 10 bars
    for (let i = 0; i < 10; i++) {
      const bar = document.createElement('div');
      bar.className = 'chart-bar';
      bar.style.height = '0%';
      chart.appendChild(bar);
    }
  }

  updateStats(stats) {
    if (stats.drinksServed !== undefined) {
      const el = this.$('#drinks-served');
      if (el) el.textContent = stats.drinksServed;
      this.updateChart(stats.drinksServed);
    }

    if (stats.ordersPerHour !== undefined) {
      const el = this.$('#orders-hour');
      if (el) el.textContent = Math.round(stats.ordersPerHour);
    }

    if (stats.averageWaitTime !== undefined) {
      const el = this.$('#wait-time');
      if (el) {
        const seconds = Math.round(stats.averageWaitTime);
        if (seconds >= 60) {
          const mins = Math.floor(seconds / 60);
          const secs = seconds % 60;
          el.textContent = `${mins}m ${secs}s`;
        } else {
          el.textContent = `${seconds}s`;
        }
      }
    }

    if (stats.waitTimeSampleCount !== undefined) {
      const el = this.$('#sample-count');
      if (el) {
        el.textContent = `${stats.waitTimeSampleCount} sample${stats.waitTimeSampleCount !== 1 ? 's' : ''}`;
      }
    }

    if (stats.resourceEfficiency !== undefined) {
      const el = this.$('#efficiency');
      if (el) el.textContent = `${Math.round(stats.resourceEfficiency)}%`;
    }

    if (stats.abandonedCount !== undefined) {
      const countEl = this.$('#abandoned-count');
      if (countEl) countEl.textContent = stats.abandonedCount;
    }

    if (stats.abandonmentRate !== undefined) {
      const rateEl = this.$('#abandonment-rate');
      if (rateEl) rateEl.textContent = `${stats.abandonmentRate.toFixed(1)}% rate`;
    }
  }

  updateChart(value) {
    const chart = this.$('#served-chart');
    if (!chart) return;

    const bars = chart.querySelectorAll('.chart-bar');
    
    // Shift bars left
    for (let i = 0; i < bars.length - 1; i++) {
      bars[i].style.height = bars[i + 1].style.height;
    }

    // Add new value
    const maxValue = 100;
    const percentage = Math.min((value % maxValue) / maxValue * 100, 100);
    bars[bars.length - 1].style.height = `${percentage}%`;
  }
}

// ============================================================================
// Simulation Charts Component
// ============================================================================
class SimulationCharts extends CoffeeShopComponent {
  connectedCallback() {
    this.queueChart = null;
    this.waitTimeChart = null;
    this.render();
    // Defer chart initialization until Chart.js is available
    this.initChartsWhenReady();
  }

  disconnectedCallback() {
    if (this.queueChart) {
      this.queueChart.destroy();
      this.queueChart = null;
    }
    if (this.waitTimeChart) {
      this.waitTimeChart.destroy();
      this.waitTimeChart = null;
    }
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .charts-container {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
          gap: 1rem;
        }

        .chart-card {
          background: white;
          border-radius: 12px;
          padding: 1.5rem;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }

        .chart-header {
          font-weight: 600;
          color: #3E2723;
          margin-bottom: 1rem;
          font-size: 1rem;
          display: flex;
          align-items: center;
          gap: 0.5rem;
        }

        .chart-wrapper {
          position: relative;
          height: 200px;
        }

        canvas {
          width: 100% !important;
          height: 100% !important;
        }

        .no-data {
          display: flex;
          align-items: center;
          justify-content: center;
          height: 200px;
          color: #999;
          font-size: 0.9rem;
        }
      </style>

      <div class="charts-container">
        <div class="chart-card">
          <div class="chart-header">
            <span>📊</span>
            <span>Queue Length Over Time</span>
          </div>
          <div class="chart-wrapper">
            <canvas id="queue-chart"></canvas>
          </div>
        </div>

        <div class="chart-card">
          <div class="chart-header">
            <span>⏱️</span>
            <span>Average Wait Time</span>
          </div>
          <div class="chart-wrapper">
            <canvas id="wait-chart"></canvas>
          </div>
        </div>
      </div>
    `;
  }

  initChartsWhenReady() {
    // Wait for Chart.js to be available
    const checkChart = () => {
      if (typeof Chart !== 'undefined') {
        this.initCharts();
      } else {
        setTimeout(checkChart, 100);
      }
    };
    checkChart();
  }

  initCharts() {
    const queueCanvas = this.$('#queue-chart');
    const waitCanvas = this.$('#wait-chart');

    if (!queueCanvas || !waitCanvas) return;

    // Store timestamps and abandonment times for drawing markers
    this.timestamps = [];
    this.abandonmentTimes = [];

    // Vertical line crosshair plugin
    const crosshairPlugin = {
      id: 'crosshair',
      afterDraw: (chart) => {
        if (chart.tooltip._active && chart.tooltip._active.length) {
          const ctx = chart.ctx;
          const activePoint = chart.tooltip._active[0];
          const x = activePoint.element.x;
          const topY = chart.scales.y.top;
          const bottomY = chart.scales.y.bottom;

          ctx.save();
          ctx.beginPath();
          ctx.moveTo(x, topY);
          ctx.lineTo(x, bottomY);
          ctx.lineWidth = 1;
          ctx.strokeStyle = 'rgba(0, 0, 0, 0.3)';
          ctx.setLineDash([4, 4]);
          ctx.stroke();
          ctx.restore();
        }
      }
    };

    // Abandonment marker plugin - draws red vertical lines when customers leave
    const self = this;
    const abandonmentPlugin = {
      id: 'abandonmentMarkers',
      afterDraw: (chart) => {
        if (!self.abandonmentTimes || self.abandonmentTimes.length === 0) return;
        if (!self.timestamps || self.timestamps.length < 2) return;

        const ctx = chart.ctx;
        const xScale = chart.scales.x;
        const yScale = chart.scales.y;

        const minTime = self.timestamps[0];
        const maxTime = self.timestamps[self.timestamps.length - 1];
        const timeRange = maxTime - minTime;

        if (timeRange <= 0) return;

        self.abandonmentTimes.forEach(simTime => {
          // Only draw if within the visible range
          if (simTime < minTime || simTime > maxTime) return;

          // Calculate position as fraction of the x-axis
          const fraction = (simTime - minTime) / timeRange;
          const labelCount = self.timestamps.length - 1;
          const x = xScale.getPixelForValue(fraction * labelCount);

          const topY = yScale.top;
          const bottomY = yScale.bottom;

          ctx.save();
          ctx.beginPath();
          ctx.moveTo(x, topY);
          ctx.lineTo(x, bottomY);
          ctx.lineWidth = 2;
          ctx.strokeStyle = 'rgba(244, 67, 54, 0.7)';
          ctx.stroke();

          // Draw small triangle marker at top
          ctx.beginPath();
          ctx.moveTo(x - 5, topY);
          ctx.lineTo(x + 5, topY);
          ctx.lineTo(x, topY + 8);
          ctx.closePath();
          ctx.fillStyle = 'rgba(244, 67, 54, 0.9)';
          ctx.fill();
          ctx.restore();
        });
      }
    };

    const commonOptions = {
      responsive: true,
      maintainAspectRatio: false,
      animation: {
        duration: 300
      },
      interaction: {
        mode: 'index',
        intersect: false
      },
      scales: {
        x: {
          display: true,
          title: {
            display: true,
            text: 'Time',
            color: '#666'
          },
          ticks: {
            color: '#666',
            maxTicksLimit: 8
          },
          grid: {
            color: 'rgba(0,0,0,0.05)'
          }
        },
        y: {
          display: true,
          beginAtZero: true,
          ticks: {
            color: '#666'
          },
          grid: {
            color: 'rgba(0,0,0,0.05)'
          }
        }
      },
      plugins: {
        legend: {
          display: false
        },
        tooltip: {
          enabled: true,
          backgroundColor: 'rgba(62, 39, 35, 0.9)',
          titleColor: '#FFF8E1',
          bodyColor: '#FFF8E1',
          borderColor: '#8D6E63',
          borderWidth: 1,
          cornerRadius: 8,
          displayColors: false,
          callbacks: {
            title: (items) => `Time: ${items[0].label}`,
            label: (item) => `${item.dataset.label}: ${item.parsed.y}`
          }
        }
      }
    };

    // Queue Length Chart
    this.queueChart = new Chart(queueCanvas.getContext('2d'), {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          label: 'Queue Length',
          data: [],
          borderColor: '#2196F3',
          backgroundColor: 'rgba(33, 150, 243, 0.1)',
          fill: true,
          tension: 0.4,
          pointRadius: 2,
          pointHoverRadius: 4
        }]
      },
      options: {
        ...commonOptions,
        scales: {
          ...commonOptions.scales,
          y: {
            ...commonOptions.scales.y,
            title: {
              display: true,
              text: 'Orders',
              color: '#666'
            }
          }
        }
      },
      plugins: [crosshairPlugin]
    });

    // Wait Time Chart (with abandonment markers)
    this.waitTimeChart = new Chart(waitCanvas.getContext('2d'), {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          label: 'Avg Wait Time',
          data: [],
          borderColor: '#FF9800',
          backgroundColor: 'rgba(255, 152, 0, 0.1)',
          fill: true,
          tension: 0.4,
          pointRadius: 2,
          pointHoverRadius: 4
        }]
      },
      options: {
        ...commonOptions,
        scales: {
          ...commonOptions.scales,
          y: {
            ...commonOptions.scales.y,
            title: {
              display: true,
              text: 'Seconds',
              color: '#666'
            }
          }
        }
      },
      plugins: [crosshairPlugin, abandonmentPlugin]
    });
  }

  formatTime(simSeconds) {
    const mins = Math.floor(simSeconds / 60);
    const secs = Math.floor(simSeconds % 60);
    return `${mins}:${String(secs).padStart(2, '0')}`;
  }

  updateData(historicalData) {
    if (!this.queueChart || !this.waitTimeChart) return;

    const labels = historicalData.timestamps.map(t => this.formatTime(t));

    // Store raw timestamps and abandonment times for the marker plugin
    this.timestamps = historicalData.timestamps || [];
    this.abandonmentTimes = historicalData.abandonmentTimes || [];

    // Update Queue Chart
    this.queueChart.data.labels = labels;
    this.queueChart.data.datasets[0].data = historicalData.queueLength;
    this.queueChart.update('none');

    // Update Wait Time Chart
    this.waitTimeChart.data.labels = labels;
    this.waitTimeChart.data.datasets[0].data = historicalData.waitTime;
    this.waitTimeChart.update('none');
  }

  reset() {
    this.timestamps = [];
    this.abandonmentTimes = [];
    if (this.queueChart) {
      this.queueChart.data.labels = [];
      this.queueChart.data.datasets[0].data = [];
      this.queueChart.update();
    }
    if (this.waitTimeChart) {
      this.waitTimeChart.data.labels = [];
      this.waitTimeChart.data.datasets[0].data = [];
      this.waitTimeChart.update();
    }
  }
}

// ============================================================================
// Recipe Display Component - Shows Petri Net Resource Requirements
// ============================================================================
class RecipeDisplay extends CoffeeShopComponent {
  constructor() {
    super();
    // Recipe data derived from Petri net model arcs:
    // - make_espresso: coffee_beans (20g), cups (1)
    // - make_latte: coffee_beans (15g), milk (50ml), cups (1)
    // - make_cappuccino: coffee_beans (15g), milk (30ml), cups (1)
    this.recipes = [
      {
        id: 'espresso',
        name: 'Espresso',
        icon: '☕',
        rate: 20, // transitions rate from model
        ingredients: [
          { resource: 'coffee_beans', amount: 20, unit: 'g' },
          { resource: 'cups', amount: 1, unit: '' }
        ]
      },
      {
        id: 'latte',
        name: 'Latte',
        icon: '🥛',
        rate: 12,
        ingredients: [
          { resource: 'coffee_beans', amount: 15, unit: 'g' },
          { resource: 'milk', amount: 50, unit: 'ml' },
          { resource: 'cups', amount: 1, unit: '' }
        ]
      },
      {
        id: 'cappuccino',
        name: 'Cappuccino',
        icon: '☕',
        rate: 10,
        ingredients: [
          { resource: 'coffee_beans', amount: 15, unit: 'g' },
          { resource: 'milk', amount: 30, unit: 'ml' },
          { resource: 'cups', amount: 1, unit: '' }
        ]
      }
    ];
  }

  connectedCallback() {
    this.render();
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }
        .recipes-container {
          background: var(--milk-white, #FFFEF7);
          border-radius: 12px;
          padding: 1.5rem;
          box-shadow: 0 4px 8px rgba(62, 39, 35, 0.15);
        }
        .recipes-header {
          display: flex;
          align-items: center;
          gap: 0.75rem;
          margin-bottom: 1rem;
          padding-bottom: 0.75rem;
          border-bottom: 2px solid #FFECB3;
        }
        .recipes-icon {
          font-size: 1.5rem;
        }
        .recipes-title {
          font-size: 1.2rem;
          font-weight: 600;
          color: #3E2723;
        }
        .recipes-subtitle {
          font-size: 0.85rem;
          color: #8D6E63;
          margin-left: auto;
        }
        .recipes-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
          gap: 1rem;
        }
        .recipe-card {
          background: linear-gradient(135deg, #FFF8E1 0%, #FFECB3 100%);
          border-radius: 10px;
          padding: 1rem;
          border: 1px solid #A1887F;
        }
        .recipe-header {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          margin-bottom: 0.75rem;
        }
        .recipe-icon {
          font-size: 1.5rem;
        }
        .recipe-name {
          font-weight: 600;
          color: #3E2723;
          font-size: 1.1rem;
        }
        .recipe-rate {
          margin-left: auto;
          font-size: 0.75rem;
          background: #6D4C41;
          color: white;
          padding: 0.2rem 0.5rem;
          border-radius: 10px;
        }
        .ingredients-list {
          display: flex;
          flex-direction: column;
          gap: 0.4rem;
        }
        .ingredient {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          font-size: 0.9rem;
          color: #5D4037;
        }
        .ingredient-icon {
          width: 20px;
          text-align: center;
        }
        .ingredient-amount {
          font-weight: 600;
          color: #3E2723;
        }
        .ingredient-name {
          color: #6D4C41;
        }
        .petri-note {
          margin-top: 1rem;
          padding-top: 0.75rem;
          border-top: 1px dashed #A1887F;
          font-size: 0.8rem;
          color: #8D6E63;
          text-align: center;
        }
        .petri-note code {
          background: #EFEBE9;
          padding: 0.1rem 0.3rem;
          border-radius: 3px;
          font-family: monospace;
        }
      </style>
      <div class="recipes-container">
        <div class="recipes-header">
          <span class="recipes-icon">📋</span>
          <span class="recipes-title">Drink Recipes</span>
          <span class="recipes-subtitle">Resource requirements from Petri net</span>
        </div>
        <div class="recipes-grid">
          ${this.recipes.map(recipe => this.renderRecipe(recipe)).join('')}
        </div>
        <div class="petri-note">
          Derived from <code>coffeeshop.json</code> transitions:
          <code>make_espresso</code>, <code>make_latte</code>, <code>make_cappuccino</code>
        </div>
      </div>
    `;
  }

  renderRecipe(recipe) {
    const resourceIcons = {
      coffee_beans: '☕',
      milk: '🥛',
      cups: '🥤'
    };
    const resourceNames = {
      coffee_beans: 'Coffee Beans',
      milk: 'Milk',
      cups: 'Cup'
    };

    return `
      <div class="recipe-card">
        <div class="recipe-header">
          <span class="recipe-icon">${recipe.icon}</span>
          <span class="recipe-name">${recipe.name}</span>
          <span class="recipe-rate">${recipe.rate}/hr</span>
        </div>
        <div class="ingredients-list">
          ${recipe.ingredients.map(ing => `
            <div class="ingredient">
              <span class="ingredient-icon">${resourceIcons[ing.resource]}</span>
              <span class="ingredient-amount">${ing.amount}${ing.unit}</span>
              <span class="ingredient-name">${resourceNames[ing.resource]}</span>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  }
}

// ============================================================================
// Register Custom Elements
// ============================================================================
customElements.define('coffee-shop-scene', CoffeeShopScene);
customElements.define('resource-gauges', ResourceGauges);
customElements.define('order-flow-board', OrderFlowBoard);
customElements.define('rate-config-panel', RateConfigPanel);
customElements.define('simulation-controls', SimulationControls);
customElements.define('stress-indicator', StressIndicator);
customElements.define('stats-dashboard', StatsDashboard);
customElements.define('simulation-charts', SimulationCharts);
customElements.define('recipe-display', RecipeDisplay);

export {
  CoffeeShopScene,
  ResourceGauges,
  OrderFlowBoard,
  RateConfigPanel,
  SimulationControls,
  StressIndicator,
  StatsDashboard,
  SimulationCharts,
  RecipeDisplay
};
