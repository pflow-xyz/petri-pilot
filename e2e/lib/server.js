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
 * Start a service using the unified petri-pilot CLI and wait for it to be healthy.
 *
 * @param {string} serviceName - The name of the service to start (e.g., 'test-access', 'blog-post')
 * @param {string} workingDir - Optional working directory for the service (for frontend assets)
 * @returns {Object} - Object with server process and baseUrl
 */
async function startServer(serviceName = 'test-access', workingDir = null) {
  const projectRoot = path.resolve(__dirname, '../..');
  const petriPilotBin = path.join(projectRoot, 'petri-pilot');
  const port = getRandomPort();
  const baseUrl = `http://localhost:${port}`;

  // Determine working directory - use the generated service directory if not specified
  // This ensures frontend assets can be found
  let cwd = workingDir;
  if (!cwd) {
    // Map service name to package directory name (remove hyphens)
    const pkgName = serviceName.replace(/-/g, '');
    cwd = path.join(projectRoot, 'generated', pkgName);
  }

  console.log(`Starting service ${serviceName} on port ${port} (cwd: ${cwd})`);

  const server = spawn(petriPilotBin, ['serve', '-port', String(port), serviceName], {
    cwd: cwd,
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
