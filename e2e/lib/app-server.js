/**
 * Generic app server launcher for E2E testing.
 *
 * Uses the unified petri-pilot CLI to start generated services.
 */

const { spawn, execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

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
 * Get the package directory name from an app name (removes hyphens)
 */
function getPkgName(appName) {
  return appName.replace(/-/g, '');
}

/**
 * AppServer manages the lifecycle of a generated app for testing.
 * Uses the unified petri-pilot CLI instead of standalone binaries.
 */
class AppServer {
  constructor(appName, options = {}) {
    this.appName = appName;
    this.port = options.port || 8080 + Math.floor(Math.random() * 1000);
    this.baseUrl = `http://localhost:${this.port}`;
    this.rootDir = options.rootDir || path.resolve(__dirname, '../..');
    this.pkgName = getPkgName(appName);
    // Try original name first (e.g., texas-holdem), then hyphen-stripped (e.g., blogpost)
    const originalDir = path.join(this.rootDir, 'generated', appName);
    const pkgDir = path.join(this.rootDir, 'generated', this.pkgName);
    this.generatedDir = fs.existsSync(originalDir) ? originalDir : pkgDir;
    this.petriPilotBin = path.join(this.rootDir, 'petri-pilot');
    this.server = null;
    this.logs = [];
  }

  /**
   * Build the app if needed (for frontend assets).
   * With the unified CLI, we don't need to build a separate binary,
   * but we may need to build the frontend.
   */
  async build() {
    // Build the frontend if it exists and hasn't been built
    const frontendDir = path.join(this.generatedDir, 'frontend');
    const frontendDistPath = path.join(frontendDir, 'dist', 'index.html');

    if (fs.existsSync(frontendDir) && !fs.existsSync(frontendDistPath)) {
      console.log(`Building ${this.appName} frontend...`);
      execSync('npm install && npm run build', { cwd: frontendDir, stdio: 'inherit', shell: true });
    }
  }

  /**
   * Check if this is a standalone service (has its own go.mod and main.go).
   */
  isStandalone() {
    const goModPath = path.join(this.generatedDir, 'go.mod');
    const mainPath = path.join(this.generatedDir, 'main.go');
    return fs.existsSync(goModPath) && fs.existsSync(mainPath);
  }

  /**
   * Build standalone service binary.
   */
  async buildStandalone() {
    const binaryPath = path.join(this.generatedDir, this.appName);
    // Always rebuild to ensure latest code
    console.log(`Building ${this.appName} standalone binary...`);
    execSync('go build -o ' + this.appName, { cwd: this.generatedDir, stdio: 'inherit', shell: true });
    return binaryPath;
  }

  /**
   * Start the server and wait for it to be healthy.
   */
  async start() {
    // Build frontend if needed
    const frontendDir = path.join(this.generatedDir, 'frontend');
    const frontendDistPath = path.join(frontendDir, 'dist', 'index.html');
    if (fs.existsSync(frontendDir) && !fs.existsSync(frontendDistPath)) {
      await this.build();
    }

    console.log(`Starting ${this.appName} on port ${this.port}...`);

    // Check if this is a standalone service
    if (this.isStandalone()) {
      // Build and run the standalone binary
      const binaryPath = await this.buildStandalone();

      // Run from project root so custom frontends can be found
      this.server = spawn(binaryPath, [], {
        cwd: this.rootDir,
        env: {
          ...process.env,
          PORT: String(this.port),
          MOCK_AUTH: 'true',
        },
        stdio: 'pipe',
      });
    } else {
      // Check if the petri-pilot binary exists
      if (!fs.existsSync(this.petriPilotBin)) {
        throw new Error(`petri-pilot binary not found at ${this.petriPilotBin}. Run 'make build-examples' first.`);
      }

      // Use the unified CLI to start the service - run from project root
      this.server = spawn(this.petriPilotBin, ['serve', '-port', String(this.port), this.appName], {
        cwd: this.rootDir,
        env: {
          ...process.env,
          PORT: String(this.port),
          MOCK_AUTH: 'true',
        },
        stdio: 'pipe',
      });
    }

    // Capture logs
    this.server.stdout.on('data', (data) => {
      const line = data.toString().trim();
      this.logs.push(line);
      if (process.env.DEBUG) {
        console.log(`[${this.appName}] ${line}`);
      }
    });

    this.server.stderr.on('data', (data) => {
      const line = data.toString().trim();
      this.logs.push(line);
      if (process.env.DEBUG) {
        console.error(`[${this.appName}] ${line}`);
      }
    });

    this.server.on('error', (err) => {
      console.error(`Server error for ${this.appName}:`, err);
    });

    this.server.on('exit', (code, signal) => {
      if (code !== null && code !== 0) {
        console.log(`${this.appName} exited with code ${code}`);
      }
    });

    // Wait for server to be healthy
    await waitForHealth(this.baseUrl);
    console.log(`${this.appName} is ready at ${this.baseUrl}`);

    return this;
  }

  /**
   * Stop the server.
   */
  stop() {
    if (this.server && !this.server.killed) {
      this.server.kill('SIGTERM');
      this.server = null;
    }
  }

  /**
   * Get recent logs.
   */
  getLogs(count = 20) {
    return this.logs.slice(-count);
  }
}

module.exports = { AppServer, waitForHealth };
