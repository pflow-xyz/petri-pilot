// Shared Navigation Components for Petri-Pilot
// Provides consistent header and footer across all demos

// ============================================================================
// Site Header Component
// Usage: <site-header current="texas-holdem"></site-header>
// ============================================================================
class SiteHeader extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  static get observedAttributes() {
    return ['current']
  }

  connectedCallback() {
    this.render()
    this.checkAuth()
  }

  attributeChangedCallback() {
    this.render()
  }

  async checkAuth() {
    try {
      const authData = localStorage.getItem('auth')
      if (authData) {
        const auth = JSON.parse(authData)
        if (auth.token && auth.expires_at > Date.now()) {
          this.renderUserInfo(auth)
          return
        }
      }
      // Check with server
      const res = await fetch('/auth/status')
      if (res.ok) {
        const data = await res.json()
        if (data.authenticated && data.user) {
          this.renderUserInfo({ user: data.user })
        }
      }
    } catch (e) {
      // Not authenticated
    }
  }

  renderUserInfo(auth) {
    const userArea = this.shadowRoot.getElementById('user-area')
    if (!userArea || !auth.user) return

    userArea.innerHTML = `
      <span class="user-name">${auth.user.name || auth.user.login}</span>
      <button class="logout-btn" id="logout-btn">Logout</button>
    `

    this.shadowRoot.getElementById('logout-btn').addEventListener('click', async () => {
      localStorage.removeItem('auth')
      try {
        await fetch('/auth/logout', { method: 'POST' })
      } catch (e) {}
      window.location.reload()
    })
  }

  render() {
    const current = this.getAttribute('current') || ''

    const demos = [
      { id: 'tic-tac-toe', name: 'Tic-Tac-Toe', icon: '⭕' },
      { id: 'zk-tic-tac-toe', name: 'ZK TTT', icon: '🔐' },
      { id: 'texas-holdem', name: 'Poker', icon: '🃏' },
      { id: 'coffeeshop', name: 'Coffee Shop', icon: '☕' },
      { id: 'knapsack', name: 'Knapsack', icon: '🎒' },
    ]

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 0.75rem 1.5rem;
          background: rgba(0, 0, 0, 0.4);
          backdrop-filter: blur(10px);
          border-bottom: 1px solid rgba(255, 255, 255, 0.1);
          position: sticky;
          top: 0;
          z-index: 1000;
        }

        .brand {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          text-decoration: none;
          color: #fff;
          font-weight: 600;
          font-size: 1.1rem;
        }

        .brand:hover {
          opacity: 0.9;
        }

        .brand-icon {
          font-size: 1.3rem;
        }

        .nav-links {
          display: flex;
          align-items: center;
          gap: 0.25rem;
        }

        .nav-link {
          display: flex;
          align-items: center;
          gap: 0.35rem;
          padding: 0.5rem 0.75rem;
          border-radius: 6px;
          text-decoration: none;
          color: rgba(255, 255, 255, 0.7);
          font-size: 0.9rem;
          transition: all 0.2s ease;
        }

        .nav-link:hover {
          background: rgba(255, 255, 255, 0.1);
          color: #fff;
        }

        .nav-link.active {
          background: rgba(102, 126, 234, 0.3);
          color: #fff;
        }

        .nav-link .icon {
          font-size: 1rem;
        }

        .user-area {
          display: flex;
          align-items: center;
          gap: 0.75rem;
        }

        .login-btn {
          display: flex;
          align-items: center;
          gap: 0.4rem;
          padding: 0.5rem 1rem;
          background: rgba(255, 255, 255, 0.1);
          border: 1px solid rgba(255, 255, 255, 0.2);
          border-radius: 6px;
          color: #fff;
          font-size: 0.85rem;
          cursor: pointer;
          transition: all 0.2s ease;
          text-decoration: none;
        }

        .login-btn:hover {
          background: rgba(255, 255, 255, 0.15);
          border-color: rgba(255, 255, 255, 0.3);
        }

        .user-name {
          color: rgba(255, 255, 255, 0.9);
          font-size: 0.9rem;
        }

        .logout-btn {
          padding: 0.4rem 0.75rem;
          background: transparent;
          border: 1px solid rgba(255, 255, 255, 0.2);
          border-radius: 4px;
          color: rgba(255, 255, 255, 0.7);
          font-size: 0.8rem;
          cursor: pointer;
          transition: all 0.2s ease;
        }

        .logout-btn:hover {
          background: rgba(255, 255, 255, 0.1);
          color: #fff;
        }

        .ops-divider {
          width: 1px;
          height: 20px;
          background: rgba(255, 255, 255, 0.2);
          margin: 0 0.5rem;
        }

        .ops-dropdown {
          position: relative;
        }

        .ops-trigger {
          display: flex;
          align-items: center;
          gap: 0.35rem;
          padding: 0.5rem 0.75rem;
          border-radius: 6px;
          text-decoration: none;
          color: rgba(255, 255, 255, 0.7);
          font-size: 0.9rem;
          cursor: pointer;
          transition: all 0.2s ease;
          background: none;
          border: none;
        }

        .ops-trigger:hover {
          background: rgba(255, 255, 255, 0.1);
          color: #fff;
        }

        .ops-trigger .chevron {
          font-size: 0.7rem;
          transition: transform 0.2s;
        }

        .ops-dropdown:hover .chevron {
          transform: rotate(180deg);
        }

        .ops-menu {
          position: absolute;
          top: 100%;
          right: 0;
          margin-top: 0.25rem;
          background: rgba(30, 30, 40, 0.98);
          backdrop-filter: blur(10px);
          border: 1px solid rgba(255, 255, 255, 0.15);
          border-radius: 8px;
          padding: 0.5rem;
          min-width: 240px;
          opacity: 0;
          visibility: hidden;
          transform: translateY(-8px);
          transition: all 0.2s ease;
          box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
        }

        .ops-dropdown:hover .ops-menu {
          opacity: 1;
          visibility: visible;
          transform: translateY(0);
        }

        .ops-item {
          display: block;
          padding: 0.6rem 0.75rem;
          border-radius: 6px;
          text-decoration: none;
          color: rgba(255, 255, 255, 0.85);
          transition: all 0.15s ease;
        }

        .ops-item:hover {
          background: rgba(102, 126, 234, 0.25);
          color: #fff;
        }

        .ops-item-title {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          font-size: 0.9rem;
          font-weight: 500;
        }

        .ops-item-icon {
          font-size: 1rem;
        }

        .ops-item-desc {
          font-size: 0.75rem;
          color: rgba(255, 255, 255, 0.5);
          margin-top: 0.15rem;
          margin-left: 1.5rem;
        }

        .ops-section-title {
          font-size: 0.7rem;
          text-transform: uppercase;
          letter-spacing: 0.05em;
          color: rgba(255, 255, 255, 0.4);
          padding: 0.5rem 0.75rem 0.25rem;
          margin-top: 0.25rem;
        }

        .ops-section-title:first-child {
          margin-top: 0;
        }

        @media (max-width: 768px) {
          .header {
            padding: 0.5rem 1rem;
            flex-wrap: wrap;
            gap: 0.5rem;
          }

          .nav-links {
            order: 3;
            width: 100%;
            justify-content: center;
            flex-wrap: wrap;
          }

          .nav-link span:not(.icon) {
            display: none;
          }

          .nav-link {
            padding: 0.5rem;
          }

          .ops-divider {
            display: none;
          }

          .ops-menu {
            right: auto;
            left: 50%;
            transform: translateX(-50%) translateY(-8px);
          }

          .ops-dropdown:hover .ops-menu {
            transform: translateX(-50%) translateY(0);
          }
        }
      </style>

      <header class="header">
        <a href="/" class="brand">
          <span class="brand-icon">🔷</span>
          <span>Petri Pilot</span>
        </a>

        <nav class="nav-links">
          ${demos.map(d => `
            <a href="/${d.id}/" class="nav-link ${current === d.id ? 'active' : ''}">
              <span class="icon">${d.icon}</span>
              <span>${d.name}</span>
            </a>
          `).join('')}
          <div class="ops-divider"></div>
          <div class="ops-dropdown">
            <button class="ops-trigger">
              <span>Ops</span>
              <span class="chevron">▼</span>
            </button>
            <div class="ops-menu">
              <div class="ops-section-title">Petri Net Models</div>
              <a href="/pflow" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">🔷</span>
                  Model Viewer
                </div>
                <div class="ops-item-desc">Visual Petri net editor & simulator</div>
              </a>
              <a href="/models" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">📋</span>
                  Model List
                </div>
                <div class="ops-item-desc">All available models as JSON</div>
              </a>
              <div class="ops-section-title">GraphQL API</div>
              <a href="/graphql/i" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">▶️</span>
                  Playground
                </div>
                <div class="ops-item-desc">Interactive query explorer</div>
              </a>
              <a href="/graphql" class="ops-item" id="schema-link">
                <div class="ops-item-title">
                  <span class="ops-item-icon">📐</span>
                  Schema
                </div>
                <div class="ops-item-desc">Introspect queries, mutations & types</div>
              </a>
              <div class="ops-section-title">Introspection Queries</div>
              <a href="/graphql/i?query=${encodeURIComponent('{ __schema { queryType { fields { name description } } } }')}" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">🔍</span>
                  List Queries
                </div>
                <div class="ops-item-desc">All available query operations</div>
              </a>
              <a href="/graphql/i?query=${encodeURIComponent('{ __schema { mutationType { fields { name description } } } }')}" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">✏️</span>
                  List Mutations
                </div>
                <div class="ops-item-desc">All available mutation operations</div>
              </a>
              <a href="/graphql/i?query=${encodeURIComponent('{ __schema { types { name kind description } } }')}" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">📦</span>
                  List Types
                </div>
                <div class="ops-item-desc">All types in the schema</div>
              </a>
              <a href="/graphql/i?query=${encodeURIComponent('{ __type(name: \"CoffeeshopPlaces\") { name fields { name type { name } } } }')}" class="ops-item">
                <div class="ops-item-title">
                  <span class="ops-item-icon">🔬</span>
                  Inspect Type
                </div>
                <div class="ops-item-desc">Example: CoffeeshopPlaces fields</div>
              </a>
            </div>
          </div>
        </nav>

        <div class="user-area" id="user-area">
          <a href="/auth/login" class="login-btn">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
              <path fill-rule="evenodd" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
            </svg>
            Login
          </a>
        </div>
      </header>
    `
  }
}

// ============================================================================
// Site Footer Component
// Usage: <site-footer></site-footer>
// ============================================================================
class SiteFooter extends HTMLElement {
  constructor() {
    super()
    this.attachShadow({ mode: 'open' })
  }

  connectedCallback() {
    this.render()
  }

  render() {
    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
        }

        .footer {
          padding: 1.5rem 2rem;
          background: rgba(0, 0, 0, 0.3);
          border-top: 1px solid rgba(255, 255, 255, 0.1);
          margin-top: auto;
        }

        .footer-content {
          max-width: 1200px;
          margin: 0 auto;
          display: flex;
          justify-content: space-between;
          align-items: center;
          flex-wrap: wrap;
          gap: 1rem;
        }

        .footer-brand {
          display: flex;
          align-items: center;
          gap: 0.5rem;
          color: rgba(255, 255, 255, 0.6);
          font-size: 0.9rem;
        }

        .footer-brand a {
          color: rgba(255, 255, 255, 0.8);
          text-decoration: none;
        }

        .footer-brand a:hover {
          color: #fff;
        }

        .footer-links {
          display: flex;
          gap: 1.5rem;
          flex-wrap: wrap;
        }

        .footer-link {
          color: rgba(255, 255, 255, 0.5);
          text-decoration: none;
          font-size: 0.85rem;
          transition: color 0.2s ease;
        }

        .footer-link:hover {
          color: rgba(255, 255, 255, 0.8);
        }

        @media (max-width: 600px) {
          .footer-content {
            flex-direction: column;
            text-align: center;
          }
        }
      </style>

      <footer class="footer">
        <div class="footer-content">
          <div class="footer-brand">
            <span>Built with</span>
            <a href="https://github.com/pflow-xyz/go-pflow" target="_blank">go-pflow</a>
            <span>•</span>
            <a href="https://pflow.xyz" target="_blank">pflow.xyz</a>
          </div>

          <nav class="footer-links">
            <a href="https://github.com/pflow-xyz/petri-pilot" target="_blank" class="footer-link">GitHub</a>
            <a href="/pflow" class="footer-link">Models</a>
            <a href="/graphql/i" class="footer-link">GraphQL</a>
            <a href="/graphql" class="footer-link">Schema</a>
            <a href="https://blog.stackdump.com" target="_blank" class="footer-link">Blog</a>
          </nav>
        </div>
      </footer>
    `
  }
}

// Register components
customElements.define('site-header', SiteHeader)
customElements.define('site-footer', SiteFooter)

// Export for ES modules
export { SiteHeader, SiteFooter }
