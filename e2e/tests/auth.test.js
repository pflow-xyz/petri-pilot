const puppeteer = require('puppeteer');
const { startServer, stopServer } = require('../lib/server');

describe('Authentication', () => {
  let browser, page, server, baseUrl;

  beforeAll(async () => {
    server = await startServer();
    baseUrl = server.baseUrl;
    browser = await puppeteer.launch({ headless: process.env.HEADLESS !== 'false' });
  });

  afterAll(async () => {
    await browser.close();
    await stopServer(server);
  });

  beforeEach(async () => {
    page = await browser.newPage();
    // Navigate to the app so fetch requests work in browser context
    await page.goto(baseUrl);
  });

  afterEach(async () => {
    await page.close();
  });

  test('debug login returns session token', async () => {
    const response = await page.evaluate(async (url) => {
      const res = await fetch(`${url}/api/debug/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ login: 'testuser', roles: ['admin'] }),
      });
      return res.json();
    }, baseUrl);

    expect(response.token).toBeDefined();
    expect(response.user.login).toBe('testuser');
    expect(response.user.roles).toContain('admin');
  });

  test('health endpoint returns ok', async () => {
    const response = await page.goto(`${baseUrl}/health`);
    const body = await response.json();
    expect(body.status).toBe('ok');
  });

  test('protected transition returns 401 without auth', async () => {
    // Try to call a protected transition without auth
    // The server should check authentication before validating the instance
    const response = await page.evaluate(async (url) => {
      const res = await fetch(`${url}/items/test-id/submit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ data: {} }),
      });
      const body = await res.text();
      return { status: res.status, body };
    }, baseUrl);

    expect(response.status).toBe(401);
    expect(response.body).toContain('unauthorized');
  });
});
