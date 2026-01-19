/**
 * Generic app server launcher for E2E testing.
 *
 * Handles starting/stopping generated apps and waiting for them to be ready.
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
 * Get the binary name for an app (converts hyphens to hyphens, same as package name)
 */
function getBinaryName(appName) {
  // The binary name matches the package name which is the kebab-case app name
  return appName;
}

/**
 * AppServer manages the lifecycle of a generated app for testing.
 */
class AppServer {
  constructor(appName, options = {}) {
    this.appName = appName;
    this.port = options.port || 8080 + Math.floor(Math.random() * 1000);
    this.baseUrl = `http://localhost:${this.port}`;
    this.rootDir = options.rootDir || path.resolve(__dirname, '../..');
    this.generatedDir = path.join(this.rootDir, 'generated', appName);
    this.binaryPath = path.join(this.generatedDir, getBinaryName(appName));
    this.server = null;
    this.logs = [];
  }

  /**
   * Build the app if the binary doesn't exist or is outdated.
   */
  async build() {
    // Check if we need to regenerate
    const modelPath = path.join(this.rootDir, 'examples', `${this.appName}.json`);

    if (!fs.existsSync(modelPath)) {
      throw new Error(`Model file not found: ${modelPath}`);
    }

    // Generate the code
    console.log(`Generating ${this.appName}...`);
    execSync(
      `go run ./cmd/petri-pilot/... codegen -o ./generated/${this.appName} --frontend examples/${this.appName}.json`,
      { cwd: this.rootDir, stdio: 'inherit' }
    );

    // Add replace directive for local development
    const goModPath = path.join(this.generatedDir, 'go.mod');
    let goMod = fs.readFileSync(goModPath, 'utf8');
    // Check for an actual (non-commented) replace directive
    const hasReplaceDirective = /^replace\s+github\.com\/pflow-xyz\/petri-pilot\s+=>/m.test(goMod);
    if (!hasReplaceDirective) {
      goMod += `\nreplace github.com/pflow-xyz/petri-pilot => ${this.rootDir}\n`;
      fs.writeFileSync(goModPath, goMod);
    }

    // Build the binary
    console.log(`Building ${this.appName} backend...`);
    execSync('GOWORK=off go mod tidy && GOWORK=off go build .', { cwd: this.generatedDir, stdio: 'inherit', shell: true });

    // Build the frontend if it exists
    const frontendDir = path.join(this.generatedDir, 'frontend');
    if (fs.existsSync(frontendDir)) {
      console.log(`Building ${this.appName} frontend...`);
      execSync('npm install && npm run build', { cwd: frontendDir, stdio: 'inherit', shell: true });
    }
  }

  /**
   * Start the server and wait for it to be healthy.
   */
  async start() {
    // Check if we need to build (binary missing or frontend not built)
    const frontendDistPath = path.join(this.generatedDir, 'frontend', 'dist', 'index.html');
    if (!fs.existsSync(this.binaryPath) || !fs.existsSync(frontendDistPath)) {
      await this.build();
    }

    console.log(`Starting ${this.appName} on port ${this.port}...`);

    this.server = spawn(this.binaryPath, [], {
      cwd: this.generatedDir,
      env: {
        ...process.env,
        PORT: String(this.port),
        MOCK_AUTH: 'true',
      },
      stdio: 'pipe',
    });

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
