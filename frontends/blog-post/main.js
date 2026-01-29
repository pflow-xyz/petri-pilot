// Blog Post Frontend
// Clean article-focused UI with workflow states

import { PetriGraphQL } from '../shared/graphql-client.js'

const APP_NAME = 'blogpost'
const gql = new PetriGraphQL('/graphql')

// State
let state = {
  posts: [],
  postData: {}, // Cache of post data extracted from events: { postId: { title, content, author_name, tags } }
  currentPost: null,
  filter: 'all',
  view: 'list' // list, editor, detail
}

// API calls using GraphQL
async function fetchPosts() {
  try {
    const list = await gql.list(APP_NAME, { page: 1, perPage: 100 })
    state.posts = list.items || []

    // Fetch events for all posts in parallel to get their data
    const eventPromises = state.posts.map(post =>
      gql.getEvents(post.id).then(events => ({ id: post.id, events }))
    )
    const results = await Promise.all(eventPromises)

    // Cache the extracted data
    for (const { id, events } of results) {
      state.postData[id] = getPostData(events)
    }
  } catch (err) {
    console.error('Failed to fetch posts:', err)
  }
}

async function fetchPost(id) {
  try {
    const aggState = await gql.getState(APP_NAME, id)
    return aggState
  } catch (err) {
    console.error('Failed to fetch post:', err)
  }
  return null
}

async function fetchEvents(id) {
  try {
    return await gql.getEvents(id)
  } catch (err) {
    console.error('Failed to fetch events:', err)
  }
  return []
}

async function createPost(title, content, tags) {
  try {
    // First create the aggregate
    const created = await gql.create(APP_NAME)
    if (!created) throw new Error('Failed to create post')

    const aggregateId = created.id

    // Then fire the create_post transition with content
    // Convert tags array to comma-separated string (GraphQL schema expects String)
    const tagsStr = Array.isArray(tags) ? tags.join(', ') : (tags || '')
    await gql.execute(APP_NAME, 'create_post', aggregateId, {
      title: title,
      content: content,
      tags: tagsStr,
      author_id: getCurrentUser()?.id || 'anonymous',
      author_name: getCurrentUser()?.name || 'Anonymous'
    })

    return aggregateId
  } catch (err) {
    console.error('Failed to create post:', err)
    alert('Failed to create post: ' + err.message)
    return null
  }
}

async function updatePost(id, title, content, tags) {
  try {
    // Convert tags array to comma-separated string (GraphQL schema expects String)
    const tagsStr = Array.isArray(tags) ? tags.join(', ') : (tags || '')
    await gql.execute(APP_NAME, 'update', id, { title, content, tags: tagsStr })
    return true
  } catch (err) {
    console.error('Failed to update post:', err)
    alert('Failed to update post: ' + err.message)
    return false
  }
}

async function executeTransition(transitionId, aggregateId, data = {}) {
  try {
    await gql.execute(APP_NAME, transitionId, aggregateId, data)
    return true
  } catch (err) {
    console.error('Transition failed:', err)
    alert('Action failed: ' + err.message)
    return false
  }
}

// Helpers
function getCurrentUser() {
  try {
    const auth = localStorage.getItem('auth')
    if (auth) {
      const data = JSON.parse(auth)
      if (data.user) return data.user
    }
  } catch (e) {}
  return null
}

function getPostStatus(post) {
  const statuses = ['draft', 'in_review', 'published', 'archived']
  if (post.state) {
    for (const status of statuses) {
      if (post.state[status] > 0) return status
    }
  }
  return 'draft'
}

function getPostData(events) {
  const data = {}
  for (const event of events) {
    if (event.data) {
      Object.assign(data, event.data)
    }
  }
  return data
}

function formatDate(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

function parseTags(tagsStr) {
  if (!tagsStr) return []
  if (Array.isArray(tagsStr)) return tagsStr
  return tagsStr.split(',').map(t => t.trim()).filter(t => t)
}

// Views
function showView(viewName) {
  state.view = viewName
  document.getElementById('list-view').classList.toggle('hidden', viewName !== 'list')
  document.getElementById('editor-view').classList.toggle('hidden', viewName !== 'editor')
  document.getElementById('detail-view').classList.toggle('hidden', viewName !== 'detail')
}

// Rendering
async function render() {
  if (state.view === 'list') {
    renderPostsList()
  } else if (state.view === 'detail' && state.currentPost) {
    await renderPostDetail()
  }
}

function renderPostsList() {
  const container = document.getElementById('posts-list')

  let filteredPosts = state.posts
  if (state.filter !== 'all') {
    filteredPosts = state.posts.filter(post => getPostStatus(post) === state.filter)
  }

  if (filteredPosts.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <h3>No posts${state.filter !== 'all' ? ` in "${state.filter}"` : ''}</h3>
        <p>Create your first post to get started</p>
      </div>
    `
    return
  }

  container.innerHTML = filteredPosts.map(post => {
    const status = getPostStatus(post)
    // Get data from cached events or use placeholders
    const data = state.postData[post.id] || {}
    const title = data.title || 'Untitled Post'
    const content = data.content || ''
    const excerpt = content.substring(0, 150) + (content.length > 150 ? '...' : '')
    const tags = parseTags(data.tags)
    const authorName = data.author_name || 'Unknown'

    return `
      <div class="post-card" data-id="${post.id}">
        <span class="post-status ${status}">${status.replace('_', ' ')}</span>
        <h2 class="post-title">${escapeHtml(title)}</h2>
        <p class="post-excerpt">${escapeHtml(excerpt)}</p>
        <div class="post-meta">
          <span>By ${escapeHtml(authorName)}</span>
          <span>${formatDate(post.created_at)}</span>
        </div>
        ${tags.length > 0 ? `
          <div class="post-tags">
            ${tags.map(tag => `<span class="tag">${escapeHtml(tag)}</span>`).join('')}
          </div>
        ` : ''}
      </div>
    `
  }).join('')
}

async function renderPostDetail() {
  const post = state.currentPost
  const status = getPostStatus(post)

  // Get accumulated data from events
  const events = await fetchEvents(post.id)
  const data = getPostData(events)

  // Status
  const statusEl = document.getElementById('detail-status')
  statusEl.textContent = status.replace('_', ' ')
  statusEl.className = `post-status ${status}`

  // Content
  document.getElementById('detail-title').textContent = data.title || 'Untitled'
  document.getElementById('detail-author').textContent = `By ${data.author_name || 'Unknown'}`
  document.getElementById('detail-date').textContent = formatDate(post.created_at)
  document.getElementById('detail-content').textContent = data.content || ''

  // Tags
  const tagsEl = document.getElementById('detail-tags')
  const tags = parseTags(data.tags)
  if (tags.length > 0) {
    tagsEl.innerHTML = tags.map(tag => `<span class="tag">${escapeHtml(tag)}</span>`).join('')
  } else {
    tagsEl.innerHTML = ''
  }

  // Actions based on status
  const actionsEl = document.getElementById('detail-actions')
  let actions = []

  if (status === 'draft') {
    actions.push({ label: 'Edit', action: 'edit', class: 'btn-secondary' })
    actions.push({ label: 'Submit for Review', action: 'submit', class: 'btn-primary' })
  } else if (status === 'in_review') {
    actions.push({ label: 'Approve', action: 'approve', class: 'btn-success' })
    actions.push({ label: 'Reject', action: 'reject', class: 'btn-danger' })
  } else if (status === 'published') {
    actions.push({ label: 'Unpublish', action: 'unpublish', class: 'btn-danger' })
  } else if (status === 'archived') {
    actions.push({ label: 'Restore', action: 'restore', class: 'btn-secondary' })
  }

  actionsEl.innerHTML = actions.map(a =>
    `<button class="btn ${a.class}" data-action="${a.action}">${a.label}</button>`
  ).join('')
}

function escapeHtml(str) {
  if (!str) return ''
  return str.replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// Event handlers
function setupEventHandlers() {
  // Home link
  document.getElementById('home-link').addEventListener('click', (e) => {
    e.preventDefault()
    state.currentPost = null
    showView('list')
    render()
  })

  // New post link
  document.getElementById('new-post-link').addEventListener('click', (e) => {
    e.preventDefault()
    state.currentPost = null
    document.getElementById('editor-title').textContent = 'New Post'
    document.getElementById('post-title-input').value = ''
    document.getElementById('post-content-input').value = ''
    document.getElementById('post-tags-input').value = ''
    document.getElementById('save-btn').textContent = 'Save Draft'
    showView('editor')
  })

  // Editor back
  document.getElementById('editor-back').addEventListener('click', (e) => {
    e.preventDefault()
    showView('list')
    render()
  })

  // Detail back
  document.getElementById('detail-back').addEventListener('click', (e) => {
    e.preventDefault()
    state.currentPost = null
    showView('list')
    render()
  })

  // Tab buttons
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'))
      btn.classList.add('active')
      state.filter = btn.dataset.filter
      render()
    })
  })

  // Post cards click
  document.getElementById('posts-list').addEventListener('click', async (e) => {
    const card = e.target.closest('.post-card')
    if (card) {
      const id = card.dataset.id
      const post = await fetchPost(id)
      if (post) {
        state.currentPost = { id, ...post }
        showView('detail')
        render()
      }
    }
  })

  // Post form submit
  document.getElementById('post-form').addEventListener('submit', async (e) => {
    e.preventDefault()
    const title = document.getElementById('post-title-input').value
    const content = document.getElementById('post-content-input').value
    const tags = document.getElementById('post-tags-input').value

    if (state.currentPost) {
      // Update existing
      const success = await updatePost(state.currentPost.id, title, content, parseTags(tags))
      if (success) {
        await fetchPosts()
        showView('list')
        render()
      }
    } else {
      // Create new
      const id = await createPost(title, content, parseTags(tags))
      if (id) {
        await fetchPosts()
        showView('list')
        render()
      }
    }
  })

  // Cancel button
  document.getElementById('cancel-btn').addEventListener('click', () => {
    showView('list')
    render()
  })

  // Detail actions
  document.getElementById('detail-actions').addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-action]')
    if (!btn || !state.currentPost) return

    const action = btn.dataset.action
    const id = state.currentPost.id

    if (action === 'edit') {
      const events = await fetchEvents(id)
      const data = getPostData(events)
      document.getElementById('editor-title').textContent = 'Edit Post'
      document.getElementById('post-title-input').value = data.title || ''
      document.getElementById('post-content-input').value = data.content || ''
      document.getElementById('post-tags-input').value = (parseTags(data.tags)).join(', ')
      document.getElementById('save-btn').textContent = 'Update Post'
      showView('editor')
    } else if (action === 'submit') {
      if (await executeTransition('submit', id)) {
        const post = await fetchPost(id)
        state.currentPost = { id, ...post }
        render()
      }
    } else if (action === 'approve') {
      const user = getCurrentUser()
      if (await executeTransition('approve', id, { approved_by: user?.name || 'Editor' })) {
        const post = await fetchPost(id)
        state.currentPost = { id, ...post }
        render()
      }
    } else if (action === 'reject') {
      const reason = prompt('Reason for rejection:')
      const user = getCurrentUser()
      if (await executeTransition('reject', id, { rejected_by: user?.name || 'Editor', reason })) {
        const post = await fetchPost(id)
        state.currentPost = { id, ...post }
        render()
      }
    } else if (action === 'unpublish') {
      if (await executeTransition('unpublish', id)) {
        const post = await fetchPost(id)
        state.currentPost = { id, ...post }
        render()
      }
    } else if (action === 'restore') {
      if (await executeTransition('restore', id)) {
        const post = await fetchPost(id)
        state.currentPost = { id, ...post }
        render()
      }
    }
  })
}

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  setupEventHandlers()
  await fetchPosts()
  render()

  // Auto-refresh every 10 seconds
  setInterval(async () => {
    if (state.view === 'list') {
      await fetchPosts()
      render()
    }
  }, 10000)
})
