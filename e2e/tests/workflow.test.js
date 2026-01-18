const { BASE_URL, startServer, stopServer } = require('../lib/server');

describe('Workflow State Machine', () => {
  let token, server;

  beforeAll(async () => {
    server = await startServer();

    // Get auth token via mock login
    const loginRes = await fetch(`${BASE_URL}/auth/mock/login?roles=customer,reviewer`);
    const loginData = await loginRes.json();
    token = loginData.token;
  });

  afterAll(async () => {
    stopServer(server);
  });

  test('create new aggregate', async () => {
    const res = await fetch(`${BASE_URL}/api/accesstest`, {
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
    const createRes = await fetch(`${BASE_URL}/api/accesstest`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: '{}'
    });
    const createResult = await createRes.json();
    const aggId = createResult.aggregate_id;
    console.log('Created aggregate:', JSON.stringify(createResult, null, 2));

    // Execute submit transition
    const submitRes = await fetch(`${BASE_URL}/items/${aggId}/submit`, {
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
    const createRes = await fetch(`${BASE_URL}/api/accesstest`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: '{}'
    });
    const createResult = await createRes.json();
    const aggId = createResult.aggregate_id;

    // Submit first
    await fetch(`${BASE_URL}/items/${aggId}/submit`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: aggId })
    });

    // Approve (requires reviewer role - we have it)
    const approveRes = await fetch(`${BASE_URL}/items/${aggId}/approve`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ aggregate_id: aggId })
    });
    const approveResult = await approveRes.json();

    expect(approveRes.status).toBe(200);
    expect(approveResult.state.approved).toBe(1);
  });
});
