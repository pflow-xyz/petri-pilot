// Petri-Pilot GraphQL Client
// Shared client for all frontends to communicate via unified GraphQL endpoint

/**
 * PetriGraphQL - Client for the unified GraphQL endpoint
 *
 * Usage:
 *   const gql = new PetriGraphQL('/graphql')
 *
 *   // Create a new instance
 *   const instance = await gql.create('erc20token')
 *
 *   // Get state
 *   const state = await gql.getState('erc20token', 'instance-id')
 *
 *   // Execute a transition
 *   const result = await gql.execute('erc20token', 'transfer', 'instance-id', { from: '0x...', to: '0x...', amount: 100 })
 *
 *   // List instances
 *   const list = await gql.list('erc20token', { page: 1, perPage: 10 })
 */
class PetriGraphQL {
  constructor(endpoint = '/graphql') {
    this.endpoint = endpoint
  }

  /**
   * Execute a raw GraphQL query
   * @param {string} query - GraphQL query string
   * @param {Object} variables - Query variables
   * @returns {Promise<Object>} - Query result
   */
  async query(query, variables = {}) {
    const response = await fetch(this.endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ query, variables }),
    })

    if (!response.ok) {
      throw new Error(`GraphQL request failed: ${response.statusText}`)
    }

    const result = await response.json()

    if (result.errors && result.errors.length > 0) {
      throw new Error(result.errors.map(e => e.message).join(', '))
    }

    return result.data
  }

  /**
   * Create a new aggregate instance
   * @param {string} appName - Application name (e.g., 'erc20token', 'blogpost')
   * @returns {Promise<Object>} - Created instance state
   */
  async create(appName) {
    // Schema uses createBlogpost, createErc20token, etc. (capitalize package name)
    const mutationName = `create${capitalize(appName)}`
    const query = `
      mutation {
        ${mutationName} {
          id
          version
          state
          places
          enabledTransitions
        }
      }
    `
    const data = await this.query(query)
    return data[mutationName]
  }

  /**
   * Get the current state of an aggregate
   * @param {string} appName - Application name
   * @param {string} id - Aggregate ID
   * @returns {Promise<Object>} - Aggregate state
   */
  async getState(appName, id) {
    const query = `
      query GetState($id: ID!) {
        ${appName}(id: $id) {
          id
          version
          state
          places
          enabledTransitions
        }
      }
    `
    const data = await this.query(query, { id })
    return data[appName]
  }

  /**
   * List aggregate instances
   * @param {string} appName - Application name
   * @param {Object} options - List options
   * @param {string} [options.place] - Filter by place
   * @param {number} [options.page] - Page number (default: 1)
   * @param {number} [options.perPage] - Items per page (default: 50)
   * @returns {Promise<Object>} - List result with items, total, page, perPage
   */
  async list(appName, options = {}) {
    const queryName = `${appName}List`
    const query = `
      query ListInstances($place: String, $page: Int, $perPage: Int) {
        ${queryName}(place: $place, page: $page, perPage: $perPage) {
          items {
            id
            version
            state
            places
            enabledTransitions
          }
          total
          page
          perPage
        }
      }
    `
    const data = await this.query(query, options)
    return data[queryName]
  }

  /**
   * Execute a transition on an aggregate
   * @param {string} appName - Application name
   * @param {string} transition - Transition name (e.g., 'transfer', 'mint', 'create_post')
   * @param {string} aggregateId - Aggregate ID
   * @param {Object} [inputData] - Transition input data
   * @returns {Promise<Object>} - Transition result
   */
  async execute(appName, transition, aggregateId, inputData = {}) {
    // Convert snake_case to camelCase for mutation name (e.g., create_post -> createPost)
    const mutationName = snakeToCamel(transition)
    // Input type is PascalCase (e.g., CreatePostInput)
    const inputTypeName = `${snakeToPascal(transition)}Input`

    // Build the input object
    const input = {
      aggregateId,
      ...inputData
    }

    const query = `
      mutation ExecuteTransition($input: ${inputTypeName}!) {
        ${mutationName}(input: $input) {
          success
          aggregateId
          version
          state
          enabledTransitions
          error
        }
      }
    `

    const data = await this.query(query, { input })
    const result = data[mutationName]

    if (!result.success) {
      throw new Error(result.error || 'Transition failed')
    }

    return result
  }

  /**
   * Get events for an aggregate (if event sourcing enabled)
   * @param {string} appName - Application name (e.g., 'blogpost')
   * @param {string} aggregateId - Aggregate ID
   * @param {number} [from] - Start from version
   * @returns {Promise<Array>} - Events array
   */
  async getEvents(appName, aggregateId, from = 0) {
    // Use namespaced query name for unified endpoint (e.g., blogpostEvents)
    const eventsField = `${appName}Events`
    const query = `
      query GetEvents($aggregateId: ID!, $from: Int) {
        ${eventsField}(aggregateId: $aggregateId, from: $from) {
          id
          streamId
          type
          version
          timestamp
          data
        }
      }
    `
    const data = await this.query(query, { aggregateId, from })
    return data[eventsField] || []
  }

  /**
   * Get admin statistics (if admin enabled)
   * @returns {Promise<Object>} - Admin stats
   */
  async getAdminStats() {
    // Use namespaced query name for unified endpoint (e.g., blogpostAdminStats)
    const statsField = `${this.appName}AdminStats`
    const query = `
      query {
        ${statsField} {
          totalInstances
          byPlace {
            place
            count
          }
        }
      }
    `
    const data = await this.query(query)
    return data[statsField]
  }
}

// Helper function to capitalize first letter
function capitalize(str) {
  if (!str) return ''
  return str.charAt(0).toUpperCase() + str.slice(1)
}

// Convert snake_case to camelCase (e.g., create_post -> createPost)
function snakeToCamel(str) {
  if (!str) return ''
  return str.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())
}

// Convert snake_case to PascalCase (e.g., create_post -> CreatePost)
function snakeToPascal(str) {
  if (!str) return ''
  const camel = snakeToCamel(str)
  return camel.charAt(0).toUpperCase() + camel.slice(1)
}

// Export for ES modules
export { PetriGraphQL, capitalize }

// Also attach to window for non-module usage
if (typeof window !== 'undefined') {
  window.PetriGraphQL = PetriGraphQL
}
