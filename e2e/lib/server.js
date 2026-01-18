const { spawn } = require('child_process');
const path = require('path');

const BASE_URL = 'http://localhost:8081';

/**
 * Wait for server to be healthy by polling the /health endpoint
 */
async function waitForHealth(url, maxAttempts = 60, intervalMs = 500) {
  console.log(`Waiting for server at ${url} to be healthy...`);
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const response = await fetch(`${url}/health`);
      if (response.ok) {
        console.log(`Server is healthy after ${(i + 1) * intervalMs}ms`);
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
 * Start the test-access server and wait for it to be healthy
 */
async function startServer() {
  const buildDir = path.resolve(__dirname, '../../generated/test-access');
  const binaryPath = path.join(buildDir, 'access-test');

  console.log(`Starting server from ${binaryPath}`);

  const server = spawn(binaryPath, [], {
    cwd: buildDir,
    env: { ...process.env, MOCK_AUTH: 'true', PORT: '8081' },
    stdio: 'pipe',
  });

  // Log server output
  server.stdout.on('data', (data) => {
    console.log(`[server] ${data.toString().trim()}`);
  });
  server.stderr.on('data', (data) => {
    console.log(`[server] ${data.toString().trim()}`);
  });

  // Handle server errors
  server.on('error', (err) => {
    console.error('Server process error:', err);
  });

  server.on('exit', (code, signal) => {
    if (code !== null) {
      console.log(`Server exited with code ${code}`);
    } else if (signal) {
      console.log(`Server killed with signal ${signal}`);
    }
  });

  // Wait for server to be healthy
  await waitForHealth(BASE_URL);

  return server;
}

/**
 * Stop the server process
 */
function stopServer(server) {
  if (server && !server.killed) {
    console.log('Stopping server...');
    server.kill('SIGTERM');
  }
}

module.exports = {
  BASE_URL,
  startServer,
  stopServer,
  waitForHealth,
};
