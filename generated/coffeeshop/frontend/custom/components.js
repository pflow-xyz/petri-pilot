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
    return ['customers', 'barista-busy', 'ready-drinks'];
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
          min-height: 300px;
          overflow: hidden;
        }

        .scene-container {
          display: grid;
          grid-template-columns: 1fr 2fr 1fr;
          gap: 2rem;
          align-items: center;
          height: 100%;
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
          height: 60px;
          background: linear-gradient(to bottom, #424242, #212121);
          border-radius: 8px;
          position: relative;
          margin-top: 0.5rem;
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
          <div class="barista" id="barista">👨‍🍳</div>
          <div class="steam">💨</div>
          <div class="coffee-machine"></div>
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

    // Update ready drinks
    const drinksEl = this.$('#ready-drinks');
    if (drinksEl) {
      drinksEl.innerHTML = Array(Math.min(readyDrinks, 5))
        .fill('☕')
        .map(icon => `<div class="drink">${icon}</div>`)
        .join('');
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
          animation: order-slide 0.3s ease-out;
          cursor: grab;
        }

        @keyframes order-slide {
          from {
            opacity: 0;
            transform: translateY(-10px);
          }
          to {
            opacity: 1;
            transform: translateY(0);
          }
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

  addOrder(type, lane = 'pending') {
    const order = {
      id: Date.now(),
      type,
      timestamp: Date.now(),
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

  renderLane(lane) {
    const container = this.$(`#lane-${lane}`);
    if (!container) return;

    const orders = this.orders[lane].slice(-5); // Show max 5
    container.innerHTML = orders.map(order => {
      const elapsed = Math.floor((Date.now() - order.timestamp) / 1000);
      return `
        <div class="order-card" data-id="${order.id}">
          <span class="order-icon">${order.icon}</span>
          <span class="order-type">${order.type}</span>
          <span class="order-time">${elapsed}s</span>
        </div>
      `;
    }).join('');
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

        <div class="presets">
          <button class="preset-btn" data-preset="healthy">💚 Healthy</button>
          <button class="preset-btn" data-preset="busy">💛 Busy</button>
          <button class="preset-btn" data-preset="stressed">🟠 Stressed</button>
          <button class="preset-btn" data-preset="sla_crisis">🔴 SLA Crisis</button>
          <button class="preset-btn" data-preset="inventory_crisis">📦 Inventory</button>
          <button class="preset-btn" data-preset="critical">🚨 Critical</button>
        </div>

        <div class="rate-group" style="margin-top: 1rem;">
          <div class="group-title">🧪 Test States (directly set state values)</div>
          <div class="presets">
            <button class="preset-btn test-state-btn" data-test-state="healthy">💚 Test Healthy</button>
            <button class="preset-btn test-state-btn" data-test-state="busy">💛 Test Busy</button>
            <button class="preset-btn test-state-btn" data-test-state="stressed">🟠 Test Stressed</button>
            <button class="preset-btn test-state-btn" data-test-state="sla_crisis">🔴 Test SLA</button>
            <button class="preset-btn test-state-btn" data-test-state="inventory_crisis">📦 Test Inventory</button>
            <button class="preset-btn test-state-btn" data-test-state="critical">🚨 Test Critical</button>
          </div>
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

    // Rate Presets
    this.$$('.preset-btn:not(.test-state-btn)').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const preset = e.target.dataset.preset;
        if (preset) this.applyPreset(preset);
      });
    });

    // Test State Presets (directly set state values)
    this.$$('.test-state-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        const testState = e.target.dataset.testState;
        if (testState) this.applyTestState(testState);
      });
    });
  }

  applyPreset(preset) {
    // Presets mirror go-pflow/examples/coffeeshop health states
    const presets = {
      // 💚 Healthy: Capacity exceeds demand, ~90% SLA compliance
      // Orders: 33/hr total, Capacity: ~42/hr = comfortable headroom
      healthy: {
        order_espresso: 10, order_latte: 15, order_cappuccino: 8,
        make_espresso: 20, make_latte: 14, make_cappuccino: 12,
        serve_espresso: 30, serve_latte: 30, serve_cappuccino: 30
      },
      // 💛 Busy: High traffic but managing, queue 5-10
      // Orders: 60/hr total, Capacity: ~54/hr = slight pressure
      busy: {
        order_espresso: 18, order_latte: 25, order_cappuccino: 17,
        make_espresso: 22, make_latte: 18, make_cappuccino: 14,
        serve_espresso: 35, serve_latte: 35, serve_cappuccino: 35
      },
      // 🟠 Stressed: Falling behind, queue > 10, growing fast
      // Orders: 90/hr total, Capacity: ~52/hr = queue grows ~38/hr
      stressed: {
        order_espresso: 28, order_latte: 38, order_cappuccino: 24,
        make_espresso: 22, make_latte: 18, make_cappuccino: 12,
        serve_espresso: 30, serve_latte: 30, serve_cappuccino: 30
      },
      // 🔴 SLA Crisis: >30% breach rate, slow baristas + high demand
      // Orders: 75/hr total, Capacity: ~30/hr = guaranteed SLA breaches
      sla_crisis: {
        order_espresso: 25, order_latte: 30, order_cappuccino: 20,
        make_espresso: 12, make_latte: 10, make_cappuccino: 8,
        serve_espresso: 20, serve_latte: 20, serve_cappuccino: 20
      },
      // 📦 Inventory Crisis: Very high throughput to drain resources
      // Fast prep + high orders = rapid inventory depletion
      inventory_crisis: {
        order_espresso: 35, order_latte: 45, order_cappuccino: 30,
        make_espresso: 40, make_latte: 35, make_cappuccino: 30,
        serve_espresso: 50, serve_latte: 50, serve_cappuccino: 50
      },
      // 🚨 Critical: Impossible scenario - everything overwhelmed
      // Orders: 150/hr, Capacity: ~24/hr = complete breakdown
      critical: {
        order_espresso: 50, order_latte: 60, order_cappuccino: 40,
        make_espresso: 10, make_latte: 8, make_cappuccino: 6,
        serve_espresso: 15, serve_latte: 15, serve_cappuccino: 15
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

  applyTestState(testState) {
    // Test states directly set inventory/queue values to induce each health condition
    // These match the thresholds in health.go and dashboard.js
    const testStates = {
      // 💚 Healthy: Full inventory, no queue
      healthy: {
        coffee_beans: 1000,
        milk: 500,
        cups: 200,
        orders_pending: 0,
        orders_complete: 50
      },
      // 💛 Busy: Queue > 5 (triggers busy threshold)
      busy: {
        coffee_beans: 800,
        milk: 400,
        cups: 150,
        orders_pending: 8,
        orders_complete: 200  // 8/(200+8) = 4% breach rate
      },
      // 🟠 Stressed: Queue > 10 (triggers stressed threshold)
      stressed: {
        coffee_beans: 600,
        milk: 300,
        cups: 100,
        orders_pending: 15,
        orders_complete: 200  // 15/(200+15) = 7% breach rate
      },
      // 🔴 SLA Crisis: >30% breach rate
      sla_crisis: {
        coffee_beans: 500,
        milk: 250,
        cups: 80,
        orders_pending: 20,
        orders_complete: 40  // 20/(40+20) = 33% breach rate
      },
      // 📦 Inventory Crisis: Any resource < 10%
      inventory_crisis: {
        coffee_beans: 150,   // 7.5% of 2000 max
        milk: 80,            // 8% of 1000 max
        cups: 40,            // 8% of 500 max
        orders_pending: 5,
        orders_complete: 100
      },
      // 🚨 Critical: Any resource depleted
      critical: {
        coffee_beans: 0,     // Depleted!
        milk: 50,
        cups: 20,
        orders_pending: 25,
        orders_complete: 50
      }
    };

    const state = testStates[testState];
    if (!state) return;

    this.emit('test-state-applied', { testState, state });
  }

  getRates() {
    return { ...this.rates };
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

        .jump-btn:hover {
          background: #E65100;
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

        <button class="jump-btn" id="jump-runout">⏩ Jump to Runout</button>
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

    const jumpBtn = this.$('#jump-runout');
    if (jumpBtn) {
      jumpBtn.addEventListener('click', () => {
        this.emit('jump-runout');
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
      resourceEfficiency: 100
    };
    this.render();
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
        </div>

        <div class="stat-card">
          <div class="stat-value" id="efficiency">100%</div>
          <div class="stat-label">Efficiency</div>
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
      if (el) el.textContent = `${Math.round(stats.averageWaitTime)}s`;
    }

    if (stats.resourceEfficiency !== undefined) {
      const el = this.$('#efficiency');
      if (el) el.textContent = `${Math.round(stats.resourceEfficiency)}%`;
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
// Register Custom Elements
// ============================================================================
customElements.define('coffee-shop-scene', CoffeeShopScene);
customElements.define('resource-gauges', ResourceGauges);
customElements.define('order-flow-board', OrderFlowBoard);
customElements.define('rate-config-panel', RateConfigPanel);
customElements.define('simulation-controls', SimulationControls);
customElements.define('stress-indicator', StressIndicator);
customElements.define('stats-dashboard', StatsDashboard);

export {
  CoffeeShopScene,
  ResourceGauges,
  OrderFlowBoard,
  RateConfigPanel,
  SimulationControls,
  StressIndicator,
  StatsDashboard
};
