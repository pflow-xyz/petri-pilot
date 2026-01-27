// ERC-20 Token Frontend
// Wallet-focused UI for token transfers and management

import { PetriGraphQL } from '../shared/graphql-client.js'

const APP_NAME = 'erc20token'
const gql = new PetriGraphQL('/graphql')

// Wallet accounts from model
const ACCOUNTS = [
  { address: '0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266', name: 'Alice (Admin)', roles: ['admin', 'holder'] },
  { address: '0x70997970C51812dc3A010C7d01b50e0d17dc79C8', name: 'Bob (Holder)', roles: ['holder'] },
  { address: '0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC', name: 'Charlie (Holder)', roles: ['holder'] },
  { address: '0x90F79bf6EB2c4f870365E785982E1f101E93b906', name: 'Dave (Observer)', roles: [] }
]

// State
let state = {
  currentWallet: null,
  balances: {},
  allowances: {},
  totalSupply: 0,
  events: [],
  aggregateId: null
}

// API calls using GraphQL
async function fetchState() {
  try {
    // Get aggregate ID first (or create one)
    if (!state.aggregateId) {
      const list = await gql.list(APP_NAME, { page: 1, perPage: 1 })
      if (list.items && list.items.length > 0) {
        state.aggregateId = list.items[0].id
      }
    }

    // If still no aggregate, create one
    if (!state.aggregateId) {
      const created = await gql.create(APP_NAME)
      if (created) {
        state.aggregateId = created.id
      }
    }

    // Get current state
    if (state.aggregateId) {
      const aggState = await gql.getState(APP_NAME, state.aggregateId)
      if (aggState && aggState.state) {
        state.balances = aggState.state.balances || {}
        state.allowances = aggState.state.allowances || {}
        state.totalSupply = aggState.state.total_supply || aggState.state.totalSupply || 0
      }

      // Get events
      try {
        state.events = await gql.getEvents(state.aggregateId)
      } catch {
        // Events may not be enabled
        state.events = []
      }
    }
  } catch (err) {
    console.error('Failed to fetch state:', err)
  }
}

async function executeTransition(transitionId, data) {
  try {
    const result = await gql.execute(APP_NAME, transitionId, state.aggregateId, data)

    // Refresh state after transition
    await fetchState()
    render()

    return result
  } catch (err) {
    console.error('Transaction failed:', err)
    alert('Transaction failed: ' + err.message)
    throw err
  }
}

// Rendering
function render() {
  renderWalletSelector()
  renderWalletCard()
  renderStats()
  renderBalances()
  renderTransactions()
  renderAdminCards()
}

function renderWalletSelector() {
  const selector = document.getElementById('wallet-selector')
  selector.innerHTML = '<option value="">Select Wallet...</option>'

  ACCOUNTS.forEach(account => {
    const option = document.createElement('option')
    option.value = account.address
    const balance = state.balances[account.address] || 0
    option.textContent = `${account.name} (${formatAmount(balance)})`
    if (state.currentWallet === account.address) {
      option.selected = true
    }
    selector.appendChild(option)
  })
}

function renderWalletCard() {
  const addressEl = document.getElementById('current-address')
  const balanceEl = document.getElementById('current-balance')

  if (state.currentWallet) {
    const account = ACCOUNTS.find(a => a.address === state.currentWallet)
    addressEl.textContent = truncateAddress(state.currentWallet)
    const balance = state.balances[state.currentWallet] || 0
    balanceEl.textContent = formatAmount(balance)
  } else {
    addressEl.textContent = 'Select a wallet to begin'
    balanceEl.textContent = '0.00'
  }
}

function renderStats() {
  document.getElementById('total-supply').textContent = formatAmount(state.totalSupply)

  const holders = Object.values(state.balances).filter(b => b > 0).length
  document.getElementById('holder-count').textContent = holders

  document.getElementById('tx-count').textContent = state.events.length
}

function renderBalances() {
  const container = document.getElementById('balances-list')

  const entries = Object.entries(state.balances).filter(([_, balance]) => balance > 0)

  if (entries.length === 0) {
    container.innerHTML = '<div class="empty-state">No balances</div>'
    return
  }

  container.innerHTML = entries.map(([address, balance]) => {
    const account = ACCOUNTS.find(a => a.address === address)
    const name = account ? account.name : ''
    return `
      <div class="balance-row">
        <div>
          <span class="balance-address">${truncateAddress(address)}</span>
          ${name ? `<span class="balance-name">${name}</span>` : ''}
        </div>
        <span class="balance-value">${formatAmount(balance)}</span>
      </div>
    `
  }).join('')
}

function renderTransactions() {
  const container = document.getElementById('transactions-list')

  if (state.events.length === 0) {
    container.innerHTML = '<div class="empty-state">No transactions yet</div>'
    return
  }

  const txHtml = state.events.slice().reverse().map(event => {
    const type = event.type || ''
    let data = event.data || {}
    if (typeof data === 'string') {
      try { data = JSON.parse(data) } catch { data = {} }
    }

    let icon = ''
    let iconClass = ''
    let title = type
    let subtitle = ''
    let amount = ''
    let amountClass = ''

    if (type === 'Transfer' || type === 'transfer_event') {
      const isIncoming = state.currentWallet && data.to === state.currentWallet
      const isOutgoing = state.currentWallet && data.from === state.currentWallet

      if (isIncoming) {
        iconClass = 'transfer-in'
        icon = ''
        amountClass = 'positive'
        amount = '+' + formatAmount(data.amount)
      } else if (isOutgoing) {
        iconClass = 'transfer-out'
        icon = ''
        amountClass = 'negative'
        amount = '-' + formatAmount(data.amount)
      } else {
        iconClass = 'transfer-out'
        icon = ''
        amount = formatAmount(data.amount)
      }

      title = 'Transfer'
      subtitle = `${truncateAddress(data.from)} → ${truncateAddress(data.to)}`
    } else if (type === 'Mint' || type === 'mint_event') {
      iconClass = 'mint'
      icon = ''
      title = 'Mint'
      subtitle = `To: ${truncateAddress(data.to)}`
      amountClass = 'positive'
      amount = '+' + formatAmount(data.amount)
    } else if (type === 'Burn' || type === 'burn_event') {
      iconClass = 'burn'
      icon = ''
      title = 'Burn'
      subtitle = `From: ${truncateAddress(data.from)}`
      amountClass = 'negative'
      amount = '-' + formatAmount(data.amount)
    } else if (type === 'Approval' || type === 'approval_event') {
      iconClass = 'mint'
      icon = ''
      title = 'Approval'
      subtitle = `${truncateAddress(data.owner)} approved ${truncateAddress(data.spender)}`
      amount = formatAmount(data.amount)
    }

    return `
      <div class="tx-item">
        <div class="tx-icon ${iconClass}">${icon}</div>
        <div class="tx-info">
          <div class="tx-type">${title}</div>
          <div class="tx-address">${subtitle}</div>
        </div>
        <div class="tx-amount ${amountClass}">${amount}</div>
      </div>
    `
  }).join('')

  container.innerHTML = txHtml
}

function renderAdminCards() {
  const adminCard = document.getElementById('admin-card')
  const burnCard = document.getElementById('burn-card')

  if (state.currentWallet) {
    const account = ACCOUNTS.find(a => a.address === state.currentWallet)
    if (account && account.roles.includes('admin')) {
      adminCard.style.display = ''
      burnCard.style.display = ''
      return
    }
  }

  adminCard.style.display = 'none'
  burnCard.style.display = 'none'
}

// Helpers
function formatAmount(amount) {
  if (!amount) return '0'
  // Format with commas and 2 decimal places if needed
  const num = parseFloat(amount)
  if (Number.isInteger(num)) {
    return num.toLocaleString()
  }
  return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function truncateAddress(address) {
  if (!address) return ''
  if (address.length <= 12) return address
  return address.slice(0, 6) + '...' + address.slice(-4)
}

// Event handlers
function setupEventHandlers() {
  // Wallet selector
  document.getElementById('wallet-selector').addEventListener('change', (e) => {
    state.currentWallet = e.target.value || null
    localStorage.setItem('erc20-wallet', state.currentWallet || '')
    render()
  })

  // Transfer form
  document.getElementById('transfer-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    if (!state.currentWallet) {
      alert('Please select a wallet first')
      return
    }

    const to = document.getElementById('transfer-to').value
    const amount = parseInt(document.getElementById('transfer-amount').value)

    await executeTransition('transfer', {
      from: state.currentWallet,
      to: to,
      amount: amount
    })

    e.target.reset()
  })

  // Approve form
  document.getElementById('approve-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    if (!state.currentWallet) {
      alert('Please select a wallet first')
      return
    }

    const spender = document.getElementById('approve-spender').value
    const amount = parseInt(document.getElementById('approve-amount').value)

    await executeTransition('approve', {
      owner: state.currentWallet,
      spender: spender,
      amount: amount
    })

    e.target.reset()
  })

  // Mint form
  document.getElementById('mint-form').addEventListener('submit', async (e) => {
    e.preventDefault()

    const to = document.getElementById('mint-to').value
    const amount = parseInt(document.getElementById('mint-amount').value)

    await executeTransition('mint', {
      to: to,
      amount: amount
    })

    e.target.reset()
  })

  // Burn form
  document.getElementById('burn-form').addEventListener('submit', async (e) => {
    e.preventDefault()

    const from = document.getElementById('burn-from').value
    const amount = parseInt(document.getElementById('burn-amount').value)

    await executeTransition('burn', {
      from: from,
      amount: amount
    })

    e.target.reset()
  })
}

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  // Restore wallet selection
  const savedWallet = localStorage.getItem('erc20-wallet')
  if (savedWallet && ACCOUNTS.some(a => a.address === savedWallet)) {
    state.currentWallet = savedWallet
  }

  setupEventHandlers()
  await fetchState()
  render()

  // Auto-refresh every 5 seconds
  setInterval(async () => {
    await fetchState()
    render()
  }, 5000)
})
