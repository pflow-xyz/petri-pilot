// Analytics Dashboard for vet-clinic
// Displays financial metrics and visualizations

const API_BASE = ''

// Fetch analytics data
async function fetchAnalytics() {
  try {
    const response = await fetch(`${API_BASE}/api/analytics`)
    if (!response.ok) throw new Error('Failed to fetch analytics')
    return await response.json()
  } catch (error) {
    console.error('Analytics error:', error)
    return null
  }
}

// Create a simple bar chart using CSS
function createBarChart(data, maxValue, color = '#3b82f6') {
  const percentage = maxValue > 0 ? (data / maxValue) * 100 : 0
  return `
    <div class="bar-container">
      <div class="bar" style="width: ${percentage}%; background-color: ${color};"></div>
      <span class="bar-value">${data.toFixed(1)}%</span>
    </div>
  `
}

// Create services vs products comparison
function createServicesProductsChart(services, products, benchmarks) {
  const servicesColor = services >= benchmarks.target_services_pct ? '#22c55e' : '#f59e0b'
  const productsColor = products <= benchmarks.target_products_pct ? '#22c55e' : '#f59e0b'

  return `
    <div class="metric-card">
      <h3>Revenue Mix</h3>
      <div class="comparison-chart">
        <div class="chart-row">
          <span class="chart-label">Services</span>
          ${createBarChart(services, 100, servicesColor)}
          <span class="benchmark">Target: ${benchmarks.target_services_pct}%</span>
        </div>
        <div class="chart-row">
          <span class="chart-label">Products</span>
          ${createBarChart(products, 100, productsColor)}
          <span class="benchmark">Target: ${benchmarks.target_products_pct}%</span>
        </div>
      </div>
      <p class="metric-note">${benchmarks.notes}</p>
    </div>
  `
}

// Create appointment type breakdown
function createAppointmentTypeChart(byType) {
  const types = Object.entries(byType)
  if (types.length === 0) {
    return '<div class="metric-card"><h3>By Appointment Type</h3><p class="no-data">No data yet</p></div>'
  }

  const maxVisits = Math.max(...types.map(([_, data]) => data.visit_count))

  const rows = types.map(([type, data]) => `
    <tr>
      <td class="type-name">${formatTypeName(type)}</td>
      <td class="visit-count">${data.visit_count}</td>
      <td class="services-pct">${data.avg_services_pct.toFixed(1)}%</td>
      <td class="products-pct">${data.avg_products_pct.toFixed(1)}%</td>
    </tr>
  `).join('')

  return `
    <div class="metric-card">
      <h3>By Appointment Type</h3>
      <table class="data-table">
        <thead>
          <tr>
            <th>Type</th>
            <th>Visits</th>
            <th>Services %</th>
            <th>Products %</th>
          </tr>
        </thead>
        <tbody>
          ${rows}
        </tbody>
      </table>
    </div>
  `
}

// Create provider breakdown
function createProviderChart(byProvider) {
  const providers = Object.entries(byProvider)
  if (providers.length === 0) {
    return '<div class="metric-card"><h3>By Provider</h3><p class="no-data">No data yet</p></div>'
  }

  const rows = providers.map(([provider, data]) => `
    <tr>
      <td class="provider-name">${provider}</td>
      <td class="visit-count">${data.visit_count}</td>
      <td class="services-pct">${data.avg_services_pct.toFixed(1)}%</td>
      <td class="products-pct">${data.avg_products_pct.toFixed(1)}%</td>
    </tr>
  `).join('')

  return `
    <div class="metric-card">
      <h3>By Provider</h3>
      <table class="data-table">
        <thead>
          <tr>
            <th>Provider</th>
            <th>Visits</th>
            <th>Services %</th>
            <th>Products %</th>
          </tr>
        </thead>
        <tbody>
          ${rows}
        </tbody>
      </table>
    </div>
  `
}

// Create summary cards
function createSummaryCards(data) {
  const servicesDiff = data.avg_services_pct - data.industry_benchmarks.target_services_pct
  const servicesStatus = servicesDiff >= 0 ? 'positive' : 'negative'
  const servicesIcon = servicesDiff >= 0 ? '&#9650;' : '&#9660;'

  return `
    <div class="summary-cards">
      <div class="summary-card">
        <div class="card-icon">&#128202;</div>
        <div class="card-content">
          <div class="card-value">${data.total_visits}</div>
          <div class="card-label">Total Visits</div>
        </div>
      </div>

      <div class="summary-card">
        <div class="card-icon">&#128176;</div>
        <div class="card-content">
          <div class="card-value">${data.avg_services_pct.toFixed(1)}%</div>
          <div class="card-label">Avg Services</div>
          <div class="card-trend ${servicesStatus}">
            ${servicesIcon} ${Math.abs(servicesDiff).toFixed(1)}% vs target
          </div>
        </div>
      </div>

      <div class="summary-card">
        <div class="card-icon">&#128230;</div>
        <div class="card-content">
          <div class="card-value">${data.avg_products_pct.toFixed(1)}%</div>
          <div class="card-label">Avg Products</div>
        </div>
      </div>

      <div class="summary-card">
        <div class="card-icon">&#128100;</div>
        <div class="card-content">
          <div class="card-value">${Object.keys(data.by_provider).length}</div>
          <div class="card-label">Providers</div>
        </div>
      </div>
    </div>
  `
}

// Format type name for display
function formatTypeName(type) {
  return type.charAt(0).toUpperCase() + type.slice(1)
}

// Render the dashboard
export async function renderDashboard(container) {
  container.innerHTML = `
    <div class="dashboard-loading">
      <div class="spinner"></div>
      <p>Loading analytics...</p>
    </div>
  `

  const data = await fetchAnalytics()

  if (!data) {
    container.innerHTML = `
      <div class="dashboard-error">
        <h2>Unable to load analytics</h2>
        <p>Please check that the server is running and try again.</p>
        <button onclick="window.location.reload()">Retry</button>
      </div>
    `
    return
  }

  if (data.total_visits === 0) {
    container.innerHTML = `
      <div class="dashboard">
        <h1>Financial Analytics Dashboard</h1>
        <div class="dashboard-empty">
          <div class="empty-icon">&#128202;</div>
          <h2>No completed visits yet</h2>
          <p>Complete some patient visits with checkout to see financial analytics.</p>
          <a href="/vet-clinic/new" class="btn btn-primary">Create New Visit</a>
        </div>

        <div class="metric-card">
          <h3>Industry Benchmarks</h3>
          <p>${data.industry_benchmarks.notes}</p>
          <ul>
            <li>Target Services: ${data.industry_benchmarks.target_services_pct}%</li>
            <li>Target Products: ${data.industry_benchmarks.target_products_pct}%</li>
            <li>Target COGS: ${data.industry_benchmarks.target_cogs_pct}%</li>
          </ul>
        </div>
      </div>
    `
    return
  }

  container.innerHTML = `
    <div class="dashboard">
      <h1>Financial Analytics Dashboard</h1>

      ${createSummaryCards(data)}

      <div class="metrics-grid">
        ${createServicesProductsChart(data.avg_services_pct, data.avg_products_pct, data.industry_benchmarks)}
        ${createAppointmentTypeChart(data.by_appointment_type)}
        ${createProviderChart(data.by_provider)}
      </div>

      <div class="dashboard-footer">
        <p>Data refreshed at ${new Date().toLocaleString()}</p>
        <button onclick="window.location.reload()" class="btn btn-secondary">Refresh</button>
      </div>
    </div>
  `
}

// Dashboard styles
export function getDashboardStyles() {
  return `
    .dashboard {
      padding: 20px;
      max-width: 1200px;
      margin: 0 auto;
    }

    .dashboard h1 {
      margin-bottom: 24px;
      color: #1f2937;
    }

    .dashboard-loading {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 400px;
    }

    .spinner {
      width: 40px;
      height: 40px;
      border: 4px solid #e5e7eb;
      border-top-color: #3b82f6;
      border-radius: 50%;
      animation: spin 1s linear infinite;
    }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }

    .dashboard-empty {
      text-align: center;
      padding: 60px 20px;
      background: #f9fafb;
      border-radius: 8px;
      margin-bottom: 24px;
    }

    .empty-icon {
      font-size: 64px;
      margin-bottom: 16px;
    }

    .summary-cards {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 16px;
      margin-bottom: 24px;
    }

    .summary-card {
      background: white;
      border: 1px solid #e5e7eb;
      border-radius: 8px;
      padding: 20px;
      display: flex;
      align-items: center;
      gap: 16px;
    }

    .card-icon {
      font-size: 32px;
    }

    .card-value {
      font-size: 28px;
      font-weight: 700;
      color: #1f2937;
    }

    .card-label {
      font-size: 14px;
      color: #6b7280;
    }

    .card-trend {
      font-size: 12px;
      margin-top: 4px;
    }

    .card-trend.positive {
      color: #22c55e;
    }

    .card-trend.negative {
      color: #ef4444;
    }

    .metrics-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
      gap: 24px;
      margin-bottom: 24px;
    }

    .metric-card {
      background: white;
      border: 1px solid #e5e7eb;
      border-radius: 8px;
      padding: 20px;
    }

    .metric-card h3 {
      margin: 0 0 16px 0;
      color: #374151;
      font-size: 16px;
    }

    .comparison-chart {
      display: flex;
      flex-direction: column;
      gap: 16px;
    }

    .chart-row {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .chart-label {
      width: 80px;
      font-size: 14px;
      color: #4b5563;
    }

    .bar-container {
      flex: 1;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .bar {
      height: 24px;
      border-radius: 4px;
      transition: width 0.5s ease;
    }

    .bar-value {
      font-size: 14px;
      font-weight: 600;
      color: #1f2937;
      min-width: 50px;
    }

    .benchmark {
      font-size: 12px;
      color: #9ca3af;
    }

    .metric-note {
      margin-top: 16px;
      font-size: 12px;
      color: #6b7280;
      font-style: italic;
    }

    .data-table {
      width: 100%;
      border-collapse: collapse;
    }

    .data-table th,
    .data-table td {
      padding: 12px 8px;
      text-align: left;
      border-bottom: 1px solid #e5e7eb;
    }

    .data-table th {
      font-size: 12px;
      font-weight: 600;
      color: #6b7280;
      text-transform: uppercase;
    }

    .data-table td {
      font-size: 14px;
      color: #374151;
    }

    .data-table .visit-count,
    .data-table .services-pct,
    .data-table .products-pct {
      text-align: right;
    }

    .no-data {
      color: #9ca3af;
      font-style: italic;
    }

    .dashboard-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding-top: 16px;
      border-top: 1px solid #e5e7eb;
      color: #6b7280;
      font-size: 14px;
    }

    .btn {
      padding: 8px 16px;
      border-radius: 6px;
      font-size: 14px;
      cursor: pointer;
      border: none;
      text-decoration: none;
      display: inline-block;
    }

    .btn-primary {
      background: #3b82f6;
      color: white;
    }

    .btn-primary:hover {
      background: #2563eb;
    }

    .btn-secondary {
      background: #e5e7eb;
      color: #374151;
    }

    .btn-secondary:hover {
      background: #d1d5db;
    }

    .dashboard-error {
      text-align: center;
      padding: 60px 20px;
    }

    .dashboard-error h2 {
      color: #ef4444;
    }
  `
}
