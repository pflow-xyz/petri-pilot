const puppeteer = require('puppeteer');
const { BASE_URL, startServer, stopServer } = require('../lib/server');

describe('Authentication', () => {
  let browser, page, server;

  beforeAll(async () => {
    server = await startServer();
    browser = await puppeteer.launch({ headless: process.env.HEADLESS !== 'false' });
  });

  afterAll(async () => {
    await browser.close();
    await stopServer(server);
  });

  beforeEach(async () => {
    page = await browser.newPage();
    // Navigate to the app so fetch requests work in browser context
    await page.goto(BASE_URL);
  });

  afterEach(async () => {
    await page.close();
  });

  test('debug login returns session token', async () => {
    const response = await page.evaluate(async (baseUrl) => {
      const res = await fetch(`${baseUrl}/api/debug/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ login: 'testuser', roles: ['admin'] }),
      });
      return res.json();
    }, BASE_URL);

    expect(response.token).toBeDefined();
    expect(response.user.login).toBe('testuser');
    expect(response.user.roles).toContain('admin');
  });

  test('health endpoint returns ok', async () => {
    const response = await page.goto(`${BASE_URL}/health`);
    const body = await response.json();
    expect(body.status).toBe('ok');
  });

  test('protected transition returns 401 without auth', async () => {
    // Try to call a protected transition without auth
    // The server should check authentication before validating the instance
    const response = await page.evaluate(async (baseUrl) => {
      const res = await fetch(`${baseUrl}/items/test-id/submit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ data: {} }),
      });
      const body = await res.text();
      return { status: res.status, body };
    }, BASE_URL);

    expect(response.status).toBe(401);
    expect(response.body).toContain('unauthorized');
  });
});
