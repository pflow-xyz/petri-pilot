/**
 * Test harness for E2E testing generated apps.
 *
 * Provides a complete setup with:
 * - App server management
 * - Browser via Puppeteer
 * - Debug WebSocket client for browser evaluation
 */

const puppeteer = require('puppeteer');
const { AppServer } = require('./app-server');
const { DebugClient } = require('./debug-client');

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
   */
  async login(roles = ['admin', 'fulfillment', 'system', 'customer']) {
    const response = await fetch(`${this.server.baseUrl}/api/debug/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login: 'test-user', roles }),
    });
    const data = await response.json();
    this.authToken = data.token;

    // Set auth in the browser using the frontend's saveAuth function
    // which properly sets the module-level authToken variable
    await this.eval(`
      const auth = ${JSON.stringify(data)};
      localStorage.setItem('auth', JSON.stringify(auth));
      // Trigger a reload of auth from localStorage
      if (window.saveAuth) {
        window.saveAuth(auth);
      } else {
        // Fallback: reload the page to pick up auth from localStorage
        location.reload();
      }
    `);

    // Small delay to let the auth update propagate
    await new Promise(resolve => setTimeout(resolve, 100));

    return data;
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
