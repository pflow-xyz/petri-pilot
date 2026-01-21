/**
 * Test harness for E2E testing generated apps.
 *
 * Provides a complete setup with:
 * - App server management
 * - Browser via Puppeteer
 * - Debug WebSocket client for browser evaluation
 * - Pilot API proxy for user flow testing
 */

const puppeteer = require('puppeteer');
const { AppServer } = require('./app-server');
const { DebugClient } = require('./debug-client');

/**
 * Creates a proxy for window.pilot that allows calling pilot methods from tests.
 * Usage: await harness.pilot.create() or await harness.pilot.action('validate')
 */
function createPilotProxy(harness) {
  return new Proxy({}, {
    get(target, prop) {
      // Return a function that calls window.pilot[prop](...args)
      return async (...args) => {
        const argsJson = JSON.stringify(args);
        const code = `return await window.pilot.${prop}(...${argsJson})`;
        return harness.eval(code);
      };
    }
  });
}

/**
 * TestHarness provides a complete E2E testing environment.
 */
class TestHarness {
  constructor(appName, options = {}) {
    this.appName = appName;
    this.options = options;
    this.server = null;
    this.browser = null;
    this.page = null;
    this.debugClient = null;
    this.sessionId = null;
    this.pilot = createPilotProxy(this);
  }

  /**
   * Set up the test environment.
   */
  async setup() {
    // Start the app server
    this.server = new AppServer(this.appName, this.options);
    await this.server.start();

    // Create debug client
    this.debugClient = new DebugClient(this.server.baseUrl);

    // Launch browser
    this.browser = await puppeteer.launch({
      headless: process.env.HEADLESS !== 'false',
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
      executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || '/usr/bin/chromium',
    });

    // Create page and navigate to app
    this.page = await this.browser.newPage();

    // Enable console logging from browser
    this.page.on('console', (msg) => {
      if (process.env.DEBUG) {
        console.log(`[Browser] ${msg.text()}`);
      }
    });

    // Navigate to the app
    await this.page.goto(this.server.baseUrl, { waitUntil: 'networkidle0' });

    // Wait for debug WebSocket connection
    await this.page.waitForFunction(() => {
      return window.debugSessionId && window.debugSessionId() !== null;
    }, { timeout: 10000 });

    // Get session ID
    this.sessionId = await this.debugClient.waitForSession();

    // Wait for pilot API to be available
    await this.page.waitForFunction(() => {
      return typeof window.pilot === 'object' && window.pilot !== null;
    }, { timeout: 10000 });

    return this;
  }

  /**
   * Tear down the test environment.
   */
  async teardown() {
    if (this.page) {
      await this.page.close();
      this.page = null;
    }
    if (this.browser) {
      await this.browser.close();
      this.browser = null;
    }
    if (this.server) {
      this.server.stop();
      this.server = null;
    }
  }

  /**
   * Execute JavaScript in the browser.
   */
  async eval(code) {
    return this.debugClient.eval(this.sessionId, code);
  }

  /**
   * Create a new workflow instance.
   */
  async createInstance(data = {}) {
    return this.debugClient.createInstance(this.sessionId, data);
  }

  /**
   * Get instance state.
   */
  async getInstance(instanceId) {
    return this.debugClient.getInstance(this.sessionId, instanceId);
  }

  /**
   * Execute a transition.
   */
  async executeTransition(transitionId, aggregateId, data = {}) {
    return this.debugClient.executeTransition(this.sessionId, transitionId, aggregateId, data);
  }

  /**
   * Navigate within the SPA.
   */
  async navigate(path) {
    return this.debugClient.navigate(this.sessionId, path);
  }

  /**
   * Get page state.
   */
  async getPageState() {
    return this.debugClient.getPageState(this.sessionId);
  }

  /**
   * Make an API call directly (bypassing browser).
   */
  async apiCall(method, path, body = null, headers = {}) {
    const url = `${this.server.baseUrl}${path}`;
    const options = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...headers,
      },
    };
    if (body) {
      options.body = JSON.stringify(body);
    }
    const response = await fetch(url, options);
    return response.json();
  }

  /**
   * Login via debug test login endpoint and get token.
   * @param {string|string[]} roles - Single role string or array of roles
   */
  async login(roles = ['admin', 'fulfillment', 'system', 'customer']) {
    // Normalize roles to array if a single string is provided
    const rolesArray = typeof roles === 'string' ? [roles] : roles;

    const response = await fetch(`${this.server.baseUrl}/api/debug/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login: 'test-user', roles: rolesArray }),
    });
    const data = await response.json();
    this.authToken = data.token;

    // Set auth in the browser using the frontend's saveAuth function
    await this.eval(`
      const auth = ${JSON.stringify(data)};
      localStorage.setItem('auth', JSON.stringify(auth));
      if (window.saveAuth) {
        window.saveAuth(auth);
      }
    `);

    // Small delay to let the auth update propagate
    await new Promise(resolve => setTimeout(resolve, 100));

    return data;
  }

  /**
   * Logout current user.
   */
  async logout() {
    await this.eval(`
      localStorage.removeItem('auth');
      if (window.clearAuth) {
        window.clearAuth();
      }
    `);
    this.authToken = null;
  }

  /**
   * Get event history for an aggregate.
   * Uses the events API endpoint to fetch actual events.
   * @param {string} aggregateId - The aggregate ID
   * @returns {Promise<Array>} - Array of events with sequence numbers
   */
  async getEventHistory(aggregateId) {
    // Use the events API endpoint
    // Convert kebab-case to camelcase for API path
    const apiPath = this.appName.replace(/-/g, '');
    const response = await this.apiCall('GET', `/api/${apiPath}/${aggregateId}/events`, null, {
      'Authorization': `Bearer ${this.authToken}`
    });

    // Return the events array with version as sequence
    const events = response.events || [];
    return events.map(e => ({
      ...e,
      sequence: e.version !== undefined ? e.version : e.sequence
    }));
  }

  /**
   * Fire a transition with error handling.
   * @param {string} transitionId - The transition to execute
   * @param {object} data - Additional data for the transition
   * @param {string} aggregateId - The aggregate ID
   * @returns {Promise<object>} - The transition result with success flag and state
   */
  async fireTransition(transitionId, data = {}, aggregateId) {
    try {
      const result = await this.executeTransition(transitionId, aggregateId, data);
      return { success: true, state: result.places || result.state, ...result };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  /**
   * Get state of an aggregate (direct API call).
   * @param {string} aggregateId - The aggregate ID
   * @returns {Promise<object>} - The aggregate state with places
   */
  async getState(aggregateId) {
    const apiPath = this.appName.replace(/-/g, '');
    const result = await this.apiCall('GET', `/api/${apiPath}/${aggregateId}`, null, {
      'Authorization': `Bearer ${this.authToken}`
    });
    return result.places || result.state || result;
  }

  /**
   * Get view data for a specific view and optionally an aggregate instance.
   * @param {string} viewId - The view ID
   * @param {string} aggregateId - Optional aggregate ID for detail views
   * @returns {Promise<object>} - The view data
   */
  async getView(viewId, aggregateId = null) {
    if (aggregateId) {
      return this.debugClient.getView(this.sessionId, viewId, aggregateId);
    } else {
      // For table views - get instances and project event data
      const result = await this.apiCall('GET', '/admin/instances');
      const instances = result.instances || [];

      const rows = [];
      const apiPath = this.appName.replace(/-/g, '');
      for (const instance of instances) {
        const instanceId = instance.id || instance.ID || instance.aggregate_id;
        const events = await this.apiCall('GET', `/api/${apiPath}/${instanceId}/events`);

        const projectedData = { aggregate_id: instanceId };
        if (events.events) {
          for (const evt of events.events) {
            if (evt.data) {
              Object.assign(projectedData, evt.data);
            }
          }
        }
        rows.push(projectedData);
      }

      return { rows };
    }
  }

  /**
   * Restart the server while maintaining browser session.
   */
  async restartServer() {
    const port = this.server ? this.server.port : this.options.port;

    if (this.server) {
      this.server.stop();
    }

    await new Promise(resolve => setTimeout(resolve, 500));

    const { AppServer } = require('./app-server');
    this.server = new AppServer(this.appName, { ...this.options, port });
    await this.server.start();

    await new Promise(resolve => setTimeout(resolve, 1000));
    this.sessionId = await this.debugClient.waitForSession();

    return this;
  }

  /**
   * Get the base URL of the server.
   */
  get baseUrl() {
    return this.server.baseUrl;
  }
}

/**
 * Create a test suite for an app.
 *
 * @param {string} appName - Name of the app (e.g., 'order-processing')
 * @param {Function} testFn - Test function that receives the harness
 */
function createAppTestSuite(appName, testFn) {
  describe(appName, () => {
    let harness;

    beforeAll(async () => {
      harness = new TestHarness(appName);
      await harness.setup();
    }, 120000); // 2 minute timeout for setup

    afterAll(async () => {
      if (harness) {
        await harness.teardown();
      }
    });

    testFn(() => harness);
  });
}

module.exports = { TestHarness, createAppTestSuite };
