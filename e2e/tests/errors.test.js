/**
 * E2E tests for error handling, validation, and recovery scenarios.
 */

const { spawn } = require('child_process');
const path = require('path');

/**
 * Wait for server to be healthy by polling the /health endpoint
 */
async function waitForHealth(url, maxAttempts = 60, intervalMs = 500) {
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const response = await fetch(`${url}/health`);
      if (response.ok) {
        return true;
      }
    } catch (err) {
      // Server not ready yet, continue polling
    }
    await new Promise(resolve => setTimeout(resolve, intervalMs));
  }
  throw new Error(`Server at ${url} did not become healthy within ${maxAttempts * intervalMs}ms`);
}

/**
 * Start the order-processing server
 */
async function startServer(port) {
  const buildDir = path.resolve(__dirname, '../../generated/order-processing');
  const binaryPath = path.join(buildDir, 'order-processing');
  const baseUrl = `http://localhost:${port}`;

  const server = spawn(binaryPath, [], {
    cwd: buildDir,
    env: { ...process.env, MOCK_AUTH: 'true', PORT: String(port) },
    stdio: 'pipe',
  });

  const logs = [];

  // Capture logs
  server.stdout.on('data', (data) => {
    logs.push(data.toString());
  });
  server.stderr.on('data', (data) => {
    logs.push(data.toString());
  });

  server.on('error', (err) => {
    console.error('Server process error:', err);
  });

  // Wait for server to be healthy
  await waitForHealth(baseUrl);

  server.baseUrl = baseUrl;
  server.port = port;
  server.logs = logs;

  return server;
}

/**
 * Stop the server
 */
async function stopServer(server) {
  if (server && !server.killed) {
    return new Promise((resolve) => {
      server.on('exit', resolve);
      server.kill('SIGTERM');
    });
  }
}

describe('Error Handling', () => {
  let server, baseUrl, token;
  const port = 8765;

  beforeAll(async () => {
    server = await startServer(port);
    baseUrl = server.baseUrl;

    // Get auth token via debug login endpoint
    const loginRes = await fetch(`${baseUrl}/api/debug/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login: 'testuser', roles: ['fulfillment', 'system', 'customer', 'admin'] }),
    });
    const loginData = await loginRes.json();
    token = loginData.token;
  }, 120000);

  afterAll(async () => {
    await stopServer(server);
  });

  describe('disabled transition errors', () => {
    test('should return helpful error for disabled transition', async () => {
      // Create instance (starts in 'received' state)
      const createRes = await fetch(`${baseUrl}/api/orderprocessing`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: '{}'
      });
      const instance = await createRes.json();
      expect(instance.aggregate_id).toBeDefined();

      // Try to execute 'ship' transition which requires 'paid' state
      const shipRes = await fetch(`${baseUrl}/api/ship`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          aggregate_id: instance.aggregate_id,
          data: {
            order_id: instance.aggregate_id,
            tracking_number: 'TRACK123',
            carrier: 'UPS'
          }
        })
      });

      const result = await shipRes.json();
      
      // Should return error response
      expect(shipRes.status).toBe(409); // Conflict
      expect(result.code).toBe('TRANSITION_FAILED');
      expect(result.message).toMatch(/ship/i);
      expect(result.message).toMatch(/cannot fire|current state/i);
    });

    test('should provide hint about required state', async () => {
      const createRes = await fetch(`${baseUrl}/api/orderprocessing`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: '{}'
      });
      const instance = await createRes.json();

      // Try to process payment without validating first
      const paymentRes = await fetch(`${baseUrl}/api/process_payment`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          aggregate_id: instance.aggregate_id,
          data: {
            order_id: instance.aggregate_id,
            total: 100,
            payment_method: 'credit_card'
          }
        })
      });

      const result = await paymentRes.json();
      
      // Error should mention process_payment and current state
      expect(paymentRes.status).toBe(409);
      expect(result.code).toBe('TRANSITION_FAILED');
      expect(result.message).toMatch(/process_payment/i);
      expect(result.message).toMatch(/cannot fire|current state/i);
    });
  });

  describe('server restart and recovery', () => {
    // Note: These tests demonstrate the limitation of in-memory storage.
    // The generated server uses NewMemoryStore() which doesn't persist across restarts.
    // For production, you would use NewSQLiteStore() or configure persistent storage.
    
    test('should demonstrate in-memory store limitation on restart', async () => {
      // Create instance and execute a transition
      const createRes = await fetch(`${baseUrl}/api/orderprocessing`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: '{}'
      });
      const instance = await createRes.json();
      const instanceId = instance.aggregate_id;
      
      // Execute validate transition to move to 'validated' state
      const validateRes = await fetch(`${baseUrl}/api/validate`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          aggregate_id: instanceId,
          data: {
            order_id: instanceId,
            customer_name: 'Test Customer',
            total: 100
          }
        })
      });
      const validateResult = await validateRes.json();

      // Verify we're in validated state before restart
      expect(validateResult.state.validated).toBe(1);
      expect(validateResult.state.received).toBe(0);

      // Stop the server
      await stopServer(server);
      
      // Wait for server to stop
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Start a new server on the same port
      server = await startServer(port);
      baseUrl = server.baseUrl;

      // Re-authenticate
      const loginRes = await fetch(`${baseUrl}/api/debug/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ login: 'testuser', roles: ['fulfillment', 'system', 'customer', 'admin'] }),
      });
      const loginData = await loginRes.json();
      token = loginData.token;

      // After restart with in-memory store, the instance reverts to initial state
      // because the events were not persisted
      const stateRes = await fetch(`${baseUrl}/api/orderprocessing/${instanceId}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      const state = await stateRes.json();
      
      // The instance is created with initial state, not the validated state
      // This demonstrates the in-memory store limitation
      expect(state.aggregate_id).toBe(instanceId);
      expect(state.places.received).toBe(1); // Back to initial state
      expect(state.places.validated).toBe(0); // Lost the validated state
    });

    test('should demonstrate graceful error handling for missing instance', async () => {
      // Create instance
      const createRes = await fetch(`${baseUrl}/api/orderprocessing`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: '{}'
      });
      const instance = await createRes.json();
      const instanceId = instance.aggregate_id;

      // Execute multiple transitions
      await fetch(`${baseUrl}/api/validate`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          aggregate_id: instanceId,
          data: {
            order_id: instanceId,
            customer_name: 'Test Customer',
            total: 150
          }
        })
      });

      const paymentRes = await fetch(`${baseUrl}/api/process_payment`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          aggregate_id: instanceId,
          data: {
            order_id: instanceId,
            total: 150,
            payment_method: 'credit_card'
          }
        })
      });
      const paymentResult = await paymentRes.json();

      // Verify state before restart
      expect(paymentResult.state.paid).toBe(1);

      // Restart server
      await stopServer(server);
      await new Promise(resolve => setTimeout(resolve, 1000));
      server = await startServer(port);
      baseUrl = server.baseUrl;

      // Re-authenticate
      const loginRes = await fetch(`${baseUrl}/api/debug/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ login: 'testuser', roles: ['fulfillment', 'system', 'customer', 'admin'] }),
      });
      const loginData = await loginRes.json();
      token = loginData.token;

      // With in-memory store, the instance state is reset to initial
      const stateRes = await fetch(`${baseUrl}/api/orderprocessing/${instanceId}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      const state = await stateRes.json();
      
      // State is reset to initial (received=1), events were lost
      expect(state.places.received).toBe(1);
      expect(state.places.paid).toBe(0);
      
      // Attempting to continue workflow from previous state will fail
      // because we're back in 'received' state
      const shipRes = await fetch(`${baseUrl}/api/ship`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          aggregate_id: instanceId,
          data: {
            order_id: instanceId,
            tracking_number: 'RESTART123',
            carrier: 'FedEx'
          }
        })
      });
      
      // Ship transition requires 'paid' state, but we're in 'received'
      expect(shipRes.status).toBe(409); // Conflict - transition cannot fire
      const error = await shipRes.json();
      expect(error.code).toBe('TRANSITION_FAILED');
      expect(error.message).toMatch(/cannot fire/i);
    });
  });
});
