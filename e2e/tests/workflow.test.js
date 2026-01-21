const { startServer, stopServer } = require('../lib/server');

describe('Workflow State Machine', () => {
  let token, server, baseUrl;

  beforeAll(async () => {
    server = await startServer();
    baseUrl = server.baseUrl;

    // Get auth token via debug login endpoint
    const loginRes = await fetch(`${baseUrl}/api/debug/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login: 'testuser', roles: ['customer', 'reviewer'] }),
    });
    const loginData = await loginRes.json();
    token = loginData.token;
  });

  afterAll(async () => {
    await stopServer(server);
  });

  test('create new aggregate', async () => {
    const res = await fetch(`${baseUrl}/api/testaccess`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      },
      body: '{}'
    });
    const result = await res.json();

    expect(result.aggregate_id).toBeDefined();
    expect(result.places.draft).toBe(1);
  });

  test('execute submit transition', async () => {
    // Create aggregate
    const createRes = await fetch(`${baseUrl}/api/testaccess`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: '{}'
    });
    const createResult = await createRes.json();
    const aggId = createResult.aggregate_id;
    console.log('Created aggregate:', JSON.stringify(createResult, null, 2));

    // Execute submit transition
    const submitRes = await fetch(`${baseUrl}/items/${aggId}/submit`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: aggId })
    });
    const submitResult = await submitRes.json();
    console.log('Submit result:', submitRes.status, JSON.stringify(submitResult, null, 2));

    expect(submitResult.success).toBe(true);
    expect(submitResult.state.submitted).toBe(1);
    expect(submitResult.state.draft).toBe(0);
  });

  test('access control - reviewer can approve', async () => {
    // Create aggregate
    const createRes = await fetch(`${baseUrl}/api/testaccess`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: '{}'
    });
    const createResult = await createRes.json();
    const aggId = createResult.aggregate_id;

    // Submit first
    await fetch(`${baseUrl}/items/${aggId}/submit`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: aggId })
    });

    // Approve (requires reviewer role - we have it)
    const approveRes = await fetch(`${baseUrl}/items/${aggId}/approve`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: aggId })
    });
    const approveResult = await approveRes.json();

    expect(approveRes.status).toBe(200);
    expect(approveResult.state.approved).toBe(1);
  });
});
