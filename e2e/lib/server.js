const { spawn } = require('child_process');
const path = require('path');

/**
 * Generate a random port between min and max (inclusive)
 */
function getRandomPort(min = 9100, max = 9999) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

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
 * Returns an object with server process and baseUrl
 */
async function startServer() {
  const buildDir = path.resolve(__dirname, '../../generated/test-access');
  const binaryPath = path.join(buildDir, 'access-test');
  const port = getRandomPort();
  const baseUrl = `http://localhost:${port}`;

  console.log(`Starting server from ${binaryPath} on port ${port}`);

  const server = spawn(binaryPath, [], {
    cwd: buildDir,
    env: { ...process.env, MOCK_AUTH: 'true', PORT: String(port) },
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
  await waitForHealth(baseUrl);

  // Attach baseUrl to server object for easy access
  server.baseUrl = baseUrl;

  return server;
}

/**
 * Stop the server process and wait for it to exit
 */
async function stopServer(server) {
  if (server && !server.killed) {
    console.log('Stopping server...');
    return new Promise((resolve) => {
      server.on('exit', resolve);
      server.kill('SIGTERM');
    });
  }
}

module.exports = {
  startServer,
  stopServer,
  waitForHealth,
};
