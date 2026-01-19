/**
 * Debug WebSocket client for E2E testing.
 *
 * This module provides utilities for:
 * - Connecting to the debug WebSocket endpoint
 * - Executing JavaScript in the browser via the eval endpoint
 * - Managing debug sessions
 */

const WebSocket = require('ws');

/**
 * DebugClient connects to a server's debug WebSocket and provides
 * methods for evaluating code in connected browser sessions.
 */
class DebugClient {
  constructor(baseUrl) {
    this.baseUrl = baseUrl;
    this.wsUrl = baseUrl.replace('http://', 'ws://').replace('https://', 'wss://');
  }

  /**
   * List all active debug sessions.
   * @returns {Promise<Array<{id: string, created_at: string}>>}
   */
  async listSessions() {
    const response = await fetch(`${this.baseUrl}/api/debug/sessions`);
    if (!response.ok) {
      throw new Error(`Failed to list sessions: ${response.status}`);
    }
    const data = await response.json();
    return data.sessions || [];
  }

  /**
   * Wait for at least one debug session to be available.
   * @param {number} timeoutMs - Maximum time to wait
   * @returns {Promise<string>} - The session ID
   */
  async waitForSession(timeoutMs = 10000) {
    const startTime = Date.now();
    while (Date.now() - startTime < timeoutMs) {
      const sessions = await this.listSessions();
      if (sessions.length > 0) {
        return sessions[0].id;
      }
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    throw new Error(`No debug session available after ${timeoutMs}ms`);
  }

  /**
   * Execute JavaScript code in a browser session.
   * @param {string} sessionId - The session ID
   * @param {string} code - JavaScript code to execute
   * @returns {Promise<any>} - The result of the evaluation
   */
  async eval(sessionId, code) {
    const response = await fetch(`${this.baseUrl}/api/debug/sessions/${sessionId}/eval`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code }),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(`Eval failed: ${error.message || response.status}`);
    }

    const result = await response.json();
    if (process.env.DEBUG) {
      console.log(`[DebugClient] Eval response:`, JSON.stringify(result));
    }
    if (result.error) {
      throw new Error(`Browser error: ${result.error}`);
    }
    return result.result;
  }

  /**
   * Execute an API call from the browser and return the result.
   * This is useful for testing the frontend's API client.
   * @param {string} sessionId - The session ID
   * @param {string} method - HTTP method
   * @param {string} path - API path
   * @param {object} body - Request body (optional)
   * @returns {Promise<any>} - The API response
   */
  async browserFetch(sessionId, method, path, body = null) {
    const fetchCode = body
      ? `
        const response = await fetch('${path}', {
          method: '${method}',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(${JSON.stringify(body)})
        });
        return await response.json();
      `
      : `
        const response = await fetch('${path}');
        return await response.json();
      `;
    return this.eval(sessionId, fetchCode);
  }

  /**
   * Get the current page state from the browser.
   * @param {string} sessionId - The session ID
   * @returns {Promise<object>} - Page state including URL, title, etc.
   */
  async getPageState(sessionId) {
    return this.eval(sessionId, `
      return {
        url: window.location.href,
        title: document.title,
        currentInstance: window.currentInstance || null,
        debugSessionId: window.debugSessionId ? window.debugSessionId() : null
      };
    `);
  }

  /**
   * Navigate to a path in the browser's SPA router.
   * @param {string} sessionId - The session ID
   * @param {string} path - Path to navigate to
   */
  async navigate(sessionId, path) {
    return this.eval(sessionId, `
      if (window.navigate) {
        window.navigate('${path}');
      } else {
        window.location.href = '${path}';
      }
      return window.location.pathname;
    `);
  }

  /**
   * Execute a workflow transition via the browser's API client.
   * @param {string} sessionId - The session ID
   * @param {string} transitionId - The transition to execute
   * @param {string} aggregateId - The aggregate ID
   * @param {object} data - Additional data for the transition
   * @returns {Promise<object>} - The transition result
   */
  async executeTransition(sessionId, transitionId, aggregateId, data = {}) {
    return this.eval(sessionId, `
      if (window.api && window.api.executeTransition) {
        return await window.api.executeTransition('${transitionId}', '${aggregateId}', ${JSON.stringify(data)});
      } else {
        throw new Error('API client not available');
      }
    `);
  }

  /**
   * Create a new workflow instance via the browser's API client.
   * @param {string} sessionId - The session ID
   * @param {object} data - Initial data
   * @returns {Promise<object>} - The created instance
   */
  async createInstance(sessionId, data = {}) {
    return this.eval(sessionId, `
      if (window.api && window.api.createInstance) {
        return await window.api.createInstance(${JSON.stringify(data)});
      } else {
        throw new Error('API client not available');
      }
    `);
  }

  /**
   * Get instance state via the browser's API client.
   * @param {string} sessionId - The session ID
   * @param {string} instanceId - The instance ID
   * @returns {Promise<object>} - The instance state
   */
  async getInstance(sessionId, instanceId) {
    return this.eval(sessionId, `
      if (window.api && window.api.getInstance) {
        return await window.api.getInstance('${instanceId}');
      } else {
        throw new Error('API client not available');
      }
    `);
  }
}

module.exports = { DebugClient };
