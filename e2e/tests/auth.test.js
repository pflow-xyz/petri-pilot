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
    stopServer(server);
  });

  beforeEach(async () => {
    page = await browser.newPage();
  });

  afterEach(async () => {
    await page.close();
  });

  test('mock login returns session token', async () => {
    const response = await page.goto(`${BASE_URL}/auth/mock/login?user=testuser&roles=admin`);
    const body = await response.json();

    expect(body.token).toBeDefined();
    expect(body.user.login).toBe('testuser');
    expect(body.user.roles).toContain('admin');
  });

  test('authenticated request to /auth/me returns user', async () => {
    // Login first
    const loginResponse = await page.goto(`${BASE_URL}/auth/mock/login?roles=customer`);
    const { token } = await loginResponse.json();

    // Check /auth/me with token
    await page.setExtraHTTPHeaders({ Authorization: `Bearer ${token}` });
    const meResponse = await page.goto(`${BASE_URL}/auth/me`);
    const user = await meResponse.json();

    expect(user.login).toBe('testuser');
    expect(user.roles).toContain('customer');
  });

  test('unauthenticated request to /auth/me returns 401', async () => {
    const response = await page.goto(`${BASE_URL}/auth/me`);
    expect(response.status()).toBe(401);
  });
});
