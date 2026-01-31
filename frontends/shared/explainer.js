// Shared Educational Explainer Components for Petri-Pilot
// Provides reusable UI components for explaining go-pflow concepts

// ============================================================================
// Explainer Panel Component
// Usage: <explainer-panel title="Places" icon="📍">...content...</explainer-panel>
// ============================================================================
class ExplainerPanel extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return ['title', 'icon', 'expanded', 'variant']
  }

  connectedCallback() {
    this.render()
  }

  attributeChangedCallback() {
    this.render()
  }

  get expanded() {
    return this.hasAttribute('expanded')
  }

  set expanded(val) {
    if (val) {
      this.setAttribute('expanded', '')
    } else {
      this.removeAttribute('expanded')
    }
  }

  toggle() {
    this.expanded = !this.expanded
    this.dispatchEvent(new CustomEvent('toggle', { detail: { expanded: this.expanded } }))
  }

  render() {
    const title = this.getAttribute('title') || 'Learn More'
    const icon = this.getAttribute('icon') || '🎓'
    const variant = this.getAttribute('variant') || 'default'
    const expanded = this.expanded

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
          margin: 1rem 0;
        }

        .panel {
          background: var(--explainer-bg, rgba(102, 126, 234, 0.1));
          border-radius: 8px;
          border-left: 3px solid var(--explainer-accent, #667eea);
          overflow: hidden;
        }

        .panel.variant-success {
          --explainer-bg: rgba(46, 204, 113, 0.1);
          --explainer-accent: #2ecc71;
        }

        .panel.variant-warning {
          --explainer-bg: rgba(241, 196, 15, 0.1);
          --explainer-accent: #f1c40f;
        }

        .panel.variant-info {
          --explainer-bg: rgba(52, 152, 219, 0.1);
          --explainer-accent: #3498db;
        }

        .header {
          display: flex;
          align-items: center;
          padding: 0.75rem 1rem;
          cursor: pointer;
          user-select: none;
        }

        .header:hover {
          background: rgba(255, 255, 255, 0.05);
        }

        .icon {
          font-size: 1.2rem;
          margin-right: 0.5rem;
        }

        .title {
          flex: 1;
          font-size: 0.95rem;
          font-weight: 600;
          color: var(--explainer-title, #667eea);
          margin: 0;
        }

        .toggle {
          color: var(--explainer-title, #667eea);
          font-size: 0.8rem;
          transition: transform 0.2s ease;
        }

        :host([expanded]) .toggle {
          transform: rotate(90deg);
        }

        .content {
          display: none;
          padding: 0 1rem 1rem 1rem;
          color: var(--explainer-text, #ccc);
          font-size: 0.85rem;
          line-height: 1.6;
        }

        :host([expanded]) .content {
          display: block;
        }

        ::slotted(p) {
          margin: 0 0 0.75rem 0;
        }

        ::slotted(p:last-child) {
          margin-bottom: 0;
        }

        ::slotted(h5) {
          margin: 1rem 0 0.5rem 0;
          color: var(--explainer-title, #667eea);
          font-size: 0.85rem;
        }

        ::slotted(h5:first-child) {
          margin-top: 0;
        }

        ::slotted(code) {
          background: rgba(0, 0, 0, 0.3);
          padding: 0.2em 0.4em;
          border-radius: 4px;
          font-size: 0.9em;
        }

        ::slotted(ul), ::slotted(ol) {
          margin: 0.5rem 0;
          padding-left: 1.25rem;
        }

        ::slotted(li) {
          margin-bottom: 0.25rem;
        }

        ::slotted(strong) {
          color: #fff;
        }

        ::slotted(em) {
          color: #f1c40f;
          font-style: normal;
        }

        ::slotted(.highlight) {
          background: rgba(102, 126, 234, 0.2);
          padding: 0.75rem;
          border-radius: 4px;
          margin: 0.5rem 0;
        }
      </style>
      <div class="panel variant-${variant}">
        <div class="header" onclick="this.getRootNode().host.toggle()">
          <span class="icon">${icon}</span>
          <span class="title">${title}</span>
          <span class="toggle">▶</span>
        </div>
        <div class="content">
          <slot></slot>
        </div>
      </div>
    `
  }
}

// ============================================================================
// Concept Card Component
// Usage: <concept-card concept="Place" icon="📍">A place represents...</concept-card>
// ============================================================================
class ConceptCard extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return ['concept', 'icon', 'link']
  }

  connectedCallback() {
    this.render()
  }

  attributeChangedCallback() {
    this.render()
  }

  render() {
    const concept = this.getAttribute('concept') || 'Concept'
    const icon = this.getAttribute('icon') || '📌'
    const link = this.getAttribute('link')

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .card {
          background: rgba(30, 30, 40, 0.8);
          border: 1px solid rgba(102, 126, 234, 0.3);
          border-radius: 8px;
          padding: 1rem;
          transition: border-color 0.2s ease, transform 0.2s ease;
        }

        .card:hover {
          border-color: rgba(102, 126, 234, 0.6);
          transform: translateY(-2px);
        }

        .header {
          display: flex;
          align-items: center;
          margin-bottom: 0.5rem;
        }

        .icon {
          font-size: 1.5rem;
          margin-right: 0.75rem;
        }

        .concept {
          font-size: 1rem;
          font-weight: 600;
          color: #fff;
          margin: 0;
        }

        .content {
          color: #aaa;
          font-size: 0.85rem;
          line-height: 1.5;
        }

        ::slotted(p) {
          margin: 0 0 0.5rem 0;
        }

        ::slotted(p:last-child) {
          margin-bottom: 0;
        }

        ::slotted(code) {
          background: rgba(0, 0, 0, 0.3);
          padding: 0.15em 0.3em;
          border-radius: 3px;
          font-size: 0.9em;
          color: #f1c40f;
        }

        .link {
          display: inline-block;
          margin-top: 0.75rem;
          color: #667eea;
          font-size: 0.8rem;
          text-decoration: none;
        }

        .link:hover {
          text-decoration: underline;
        }
      </style>
      <div class="card">
        <div class="header">
          <span class="icon">${icon}</span>
          <h4 class="concept">${concept}</h4>
        </div>
        <div class="content">
          <slot></slot>
        </div>
        ${link ? `<a class="link" href="${link}" target="_blank">Learn more →</a>` : ''}
      </div>
    `
  }
}

// ============================================================================
// Go-pflow Concepts Grid
// Usage: <pflow-concepts service="tic-tac-toe"></pflow-concepts>
// ============================================================================
class PflowConcepts extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return ['service', 'concepts']
  }

  connectedCallback() {
    this.render()
  }

  attributeChangedCallback() {
    this.render()
  }

  render() {
    const service = this.getAttribute('service') || 'generic'
    const conceptsAttr = this.getAttribute('concepts')

    // Default concepts for each service type
    const serviceDefaults = {
      'tic-tac-toe': ['places', 'transitions', 'ode', 'events'],
      'coffeeshop': ['places', 'capacity', 'weighted-arcs', 'rates', 'ode'],
      'texas-holdem': ['places', 'transitions', 'roles', 'events', 'guards'],
      'generic': ['places', 'transitions', 'arcs', 'events']
    }

    const concepts = conceptsAttr
      ? conceptsAttr.split(',').map(c => c.trim())
      : serviceDefaults[service] || serviceDefaults.generic

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
          margin: 1.5rem 0;
        }

        .header {
          margin-bottom: 1rem;
        }

        .title {
          font-size: 1.1rem;
          font-weight: 600;
          color: #fff;
          margin: 0 0 0.25rem 0;
          display: flex;
          align-items: center;
          gap: 0.5rem;
        }

        .subtitle {
          color: #888;
          font-size: 0.85rem;
          margin: 0;
        }

        .grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
          gap: 1rem;
        }

        .concept-item {
          background: rgba(30, 30, 40, 0.8);
          border: 1px solid rgba(102, 126, 234, 0.2);
          border-radius: 8px;
          padding: 1rem;
          transition: border-color 0.2s ease;
        }

        .concept-item:hover {
          border-color: rgba(102, 126, 234, 0.5);
        }

        .concept-header {
          display: flex;
          align-items: center;
          margin-bottom: 0.5rem;
        }

        .concept-icon {
          font-size: 1.3rem;
          margin-right: 0.5rem;
        }

        .concept-name {
          font-weight: 600;
          color: #fff;
          font-size: 0.95rem;
        }

        .concept-desc {
          color: #aaa;
          font-size: 0.8rem;
          line-height: 1.5;
          margin: 0;
        }

        .concept-example {
          margin-top: 0.5rem;
          padding: 0.5rem;
          background: rgba(0, 0, 0, 0.3);
          border-radius: 4px;
          font-family: monospace;
          font-size: 0.75rem;
          color: #f1c40f;
          overflow-x: auto;
        }
      </style>
      <div class="header">
        <h3 class="title">
          <span>🔧</span>
          go-pflow Concepts in This Demo
        </h3>
        <p class="subtitle">Click each concept to learn how it's used in the ${service} model</p>
      </div>
      <div class="grid">
        ${concepts.map(c => this.renderConcept(c, service)).join('')}
      </div>
    `

    // Add click handlers
    this.shadowRoot.querySelectorAll('.concept-item').forEach(item => {
      item.addEventListener('click', () => {
        const concept = item.dataset.concept
        this.dispatchEvent(new CustomEvent('concept-click', {
          detail: { concept },
          bubbles: true
        }))
      })
    })
  }

  renderConcept(concept, service) {
    const definitions = {
      places: {
        icon: '📍',
        name: 'Places',
        desc: 'Represent states in the workflow. Tokens in a place indicate the current state.',
        examples: {
          'tic-tac-toe': 'x0, o0, empty0 (cell states)',
          'coffeeshop': 'coffee_beans, milk, cups',
          'texas-holdem': 'preflop, flop, turn, river',
          'generic': 'pending, processing, completed'
        }
      },
      transitions: {
        icon: '⚡',
        name: 'Transitions',
        desc: 'Actions that move tokens between places. Fire when all input places have tokens.',
        examples: {
          'tic-tac-toe': 'x_move_0, o_move_0, win_x',
          'coffeeshop': 'make_espresso, make_latte',
          'texas-holdem': 'deal_flop, p0_raise, p1_fold',
          'generic': 'start, approve, complete'
        }
      },
      arcs: {
        icon: '➡️',
        name: 'Arcs',
        desc: 'Connect places to transitions. Define which tokens are consumed and produced.',
        examples: {
          generic: 'pending → approve → approved'
        }
      },
      'weighted-arcs': {
        icon: '⚖️',
        name: 'Weighted Arcs',
        desc: 'Arcs with weights > 1 consume or produce multiple tokens. Model resource quantities.',
        examples: {
          coffeeshop: 'coffee_beans →(20) make_espresso (20 grams per shot)',
          generic: 'input →(3) process →(2) output'
        }
      },
      capacity: {
        icon: '📦',
        name: 'Place Capacity',
        desc: 'Maximum tokens a place can hold. Models inventory limits and resource constraints.',
        examples: {
          coffeeshop: 'coffee_beans (capacity: 2000g), cups (capacity: 500)',
          generic: 'buffer (capacity: 10)'
        }
      },
      ode: {
        icon: '📈',
        name: 'ODE Simulation',
        desc: 'Continuous simulation using differential equations. Predicts optimal strategies over time.',
        examples: {
          'tic-tac-toe': 'Strategic value computation for each move',
          coffeeshop: 'Predict inventory levels and demand',
          generic: 'Rate-based flow analysis'
        }
      },
      rates: {
        icon: '⏱️',
        name: 'Transition Rates',
        desc: 'Speed at which transitions fire continuously. Used in ODE simulation for flow prediction.',
        examples: {
          coffeeshop: 'make_espresso: 0.5 (rate), customer_arrival: 0.3',
          generic: 'process: 1.0 (items per time unit)'
        }
      },
      events: {
        icon: '📝',
        name: 'Event Sourcing',
        desc: 'Each transition firing creates an immutable event. Enables replay, audit, and undo.',
        examples: {
          'tic-tac-toe': 'XMoved, OWon, GameDrawn',
          'texas-holdem': 'HandStarted, FlopDealt, PlayerRaised',
          generic: 'Created, Updated, Completed'
        }
      },
      roles: {
        icon: '👥',
        name: 'Role-Based Access',
        desc: 'Restrict who can fire transitions. Enables multi-player and permission systems.',
        examples: {
          'texas-holdem': 'dealer: deal_flop, admin: end_hand',
          generic: 'admin: approve, user: submit'
        }
      },
      guards: {
        icon: '🛡️',
        name: 'Guards',
        desc: 'Boolean conditions that must be true for a transition to fire. Add business logic.',
        examples: {
          'texas-holdem': 'p0_raise: "bets[0] >= current_bet"',
          generic: 'approve: "amount < 1000"'
        }
      }
    }

    const def = definitions[concept] || {
      icon: '❓',
      name: concept,
      desc: 'Petri net concept',
      examples: { generic: '' }
    }

    const example = def.examples[service] || def.examples.generic || ''

    return `
      <div class="concept-item" data-concept="${concept}">
        <div class="concept-header">
          <span class="concept-icon">${def.icon}</span>
          <span class="concept-name">${def.name}</span>
        </div>
        <p class="concept-desc">${def.desc}</p>
        ${example ? `<div class="concept-example">${example}</div>` : ''}
      </div>
    `
  }
}

// ============================================================================
// Interactive Petri Net Visualizer Link
// Usage: <pflow-model-link model="tic-tac-toe"></pflow-model-link>
// ============================================================================
class PflowModelLink extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return ['model', 'label']
  }

  connectedCallback() {
    this.render()
  }

  attributeChangedCallback() {
    this.render()
  }

  render() {
    const model = this.getAttribute('model') || ''
    const label = this.getAttribute('label') || 'View Petri Net Model'

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: inline-block;
        }

        .link {
          display: inline-flex;
          align-items: center;
          gap: 0.5rem;
          padding: 0.5rem 1rem;
          background: rgba(102, 126, 234, 0.15);
          border: 1px solid rgba(102, 126, 234, 0.4);
          border-radius: 6px;
          color: #667eea;
          text-decoration: none;
          font-size: 0.9rem;
          transition: all 0.2s ease;
        }

        .link:hover {
          background: rgba(102, 126, 234, 0.25);
          border-color: rgba(102, 126, 234, 0.6);
          transform: translateY(-1px);
        }

        .icon {
          font-size: 1.1rem;
        }
      </style>
      <a class="link" href="/pflow?model=${encodeURIComponent(model)}">
        <span class="icon">🔗</span>
        <span>${label}</span>
      </a>
    `
  }
}

// ============================================================================
// Step-by-Step Explainer (for tutorials)
// Usage: <step-explainer>
//          <step-item step="1" title="Create Instance">...</step-item>
//          <step-item step="2" title="Fire Transition">...</step-item>
//        </step-explainer>
// ============================================================================
class StepExplainer extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
    this.currentStep = 0
  }

  connectedCallback() {
    this.render()
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
          margin: 1rem 0;
        }

        .container {
          background: rgba(30, 30, 40, 0.6);
          border-radius: 8px;
          padding: 1rem;
        }

        .header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          margin-bottom: 1rem;
          padding-bottom: 0.75rem;
          border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }

        .title {
          font-size: 1rem;
          font-weight: 600;
          color: #fff;
          margin: 0;
        }

        .nav {
          display: flex;
          gap: 0.5rem;
        }

        .nav-btn {
          padding: 0.25rem 0.75rem;
          border: 1px solid rgba(102, 126, 234, 0.4);
          border-radius: 4px;
          background: transparent;
          color: #667eea;
          cursor: pointer;
          font-size: 0.8rem;
          transition: all 0.2s ease;
        }

        .nav-btn:hover:not(:disabled) {
          background: rgba(102, 126, 234, 0.2);
        }

        .nav-btn:disabled {
          opacity: 0.4;
          cursor: not-allowed;
        }

        .progress {
          display: flex;
          gap: 0.5rem;
          margin-bottom: 1rem;
        }

        .progress-dot {
          width: 8px;
          height: 8px;
          border-radius: 50%;
          background: rgba(102, 126, 234, 0.3);
          cursor: pointer;
          transition: all 0.2s ease;
        }

        .progress-dot.active {
          background: #667eea;
          transform: scale(1.2);
        }

        .progress-dot.completed {
          background: #2ecc71;
        }

        ::slotted(step-item) {
          display: none;
        }

        ::slotted(step-item.active) {
          display: block;
        }
      </style>
      <div class="container">
        <div class="header">
          <h4 class="title">Step-by-Step Guide</h4>
          <div class="nav">
            <button class="nav-btn" id="prev-btn">← Previous</button>
            <button class="nav-btn" id="next-btn">Next →</button>
          </div>
        </div>
        <div class="progress" id="progress"></div>
        <slot></slot>
      </div>
    `

    this.setupNavigation()
  }

  setupNavigation() {
    const steps = this.querySelectorAll('step-item')
    const progress = this.shadowRoot.getElementById('progress')
    const prevBtn = this.shadowRoot.getElementById('prev-btn')
    const nextBtn = this.shadowRoot.getElementById('next-btn')

    // Create progress dots
    steps.forEach((_, i) => {
      const dot = document.createElement('div')
      dot.className = 'progress-dot'
      dot.addEventListener('click', () => this.goToStep(i))
      progress.appendChild(dot)
    })

    prevBtn.addEventListener('click', () => this.prevStep())
    nextBtn.addEventListener('click', () => this.nextStep())

    this.goToStep(0)
  }

  goToStep(index) {
    const steps = this.querySelectorAll('step-item')
    const dots = this.shadowRoot.querySelectorAll('.progress-dot')

    if (index < 0 || index >= steps.length) return

    steps.forEach((step, i) => {
      step.classList.toggle('active', i === index)
    })

    dots.forEach((dot, i) => {
      dot.classList.toggle('active', i === index)
      dot.classList.toggle('completed', i < index)
    })

    this.currentStep = index

    const prevBtn = this.shadowRoot.getElementById('prev-btn')
    const nextBtn = this.shadowRoot.getElementById('next-btn')
    prevBtn.disabled = index === 0
    nextBtn.disabled = index === steps.length - 1
  }

  prevStep() {
    this.goToStep(this.currentStep - 1)
  }

  nextStep() {
    this.goToStep(this.currentStep + 1)
  }
}

class StepItem extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return ['step', 'title']
  }

  connectedCallback() {
    this.render()
  }

  attributeChangedCallback() {
    this.render()
  }

  render() {
    const step = this.getAttribute('step') || '1'
    const title = this.getAttribute('title') || 'Step'

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .step-header {
          display: flex;
          align-items: center;
          gap: 0.75rem;
          margin-bottom: 0.75rem;
        }

        .step-number {
          width: 28px;
          height: 28px;
          border-radius: 50%;
          background: #667eea;
          color: #fff;
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 0.85rem;
          font-weight: 600;
        }

        .step-title {
          font-size: 1rem;
          font-weight: 600;
          color: #fff;
          margin: 0;
        }

        .step-content {
          padding-left: 2.5rem;
          color: #aaa;
          font-size: 0.9rem;
          line-height: 1.6;
        }

        ::slotted(p) {
          margin: 0 0 0.75rem 0;
        }

        ::slotted(code) {
          background: rgba(0, 0, 0, 0.3);
          padding: 0.2em 0.4em;
          border-radius: 4px;
          font-size: 0.9em;
          color: #f1c40f;
        }
      </style>
      <div class="step-header">
        <div class="step-number">${step}</div>
        <h5 class="step-title">${title}</h5>
      </div>
      <div class="step-content">
        <slot></slot>
      </div>
    `
  }
}

// ============================================================================
// Register all components
// ============================================================================
customElements.define('explainer-panel', ExplainerPanel)
customElements.define('concept-card', ConceptCard)
customElements.define('pflow-concepts', PflowConcepts)
customElements.define('pflow-model-link', PflowModelLink)
customElements.define('step-explainer', StepExplainer)
customElements.define('step-item', StepItem)

// Export for ES modules
export {
  ExplainerPanel,
  ConceptCard,
  PflowConcepts,
  PflowModelLink,
  StepExplainer,
  StepItem
}
