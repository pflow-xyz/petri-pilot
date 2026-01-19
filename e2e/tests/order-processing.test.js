/**
 * E2E tests for the order-processing app.
 *
 * Workflow: received → validate → validated → process_payment → paid → ship → shipped → confirm → completed
 * Alternative: received → reject → rejected
 */

const { TestHarness } = require('../lib/test-harness');

describe('order-processing', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('order-processing');
    await harness.setup();
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('API endpoints', () => {
    test('health check returns ok', async () => {
      const result = await harness.apiCall('GET', '/health');
      expect(result.status).toBe('ok');
    });

    test('can list instances', async () => {
      const result = await harness.apiCall('GET', '/admin/instances');
      expect(result.instances).toBeDefined();
      expect(Array.isArray(result.instances)).toBe(true);
    });
  });

  describe('debug WebSocket', () => {
    test('browser has debug session', async () => {
      const state = await harness.getPageState();
      expect(state.debugSessionId).toBeDefined();
      expect(state.debugSessionId).toBe(harness.sessionId);
    });

    test('can eval in browser', async () => {
      const result = await harness.eval('return 1 + 1');
      expect(result).toBe(2);
    });

    test('can access window object', async () => {
      const result = await harness.eval('return typeof window.api');
      expect(result).toBe('object');
    });
  });

  describe('workflow transitions', () => {
    beforeAll(async () => {
      // Login with all roles needed for transitions
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('create instance starts in received state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('received');
    });

    test('validate transition moves to validated state', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('validate', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('validated');
    });

    test('reject transition moves to rejected state', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('reject', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('rejected');
    });

    test('complete happy path workflow', async () => {
      // Create instance (starts in received)
      const instance = await harness.createInstance();
      expect(instance).toHaveTokenIn('received');

      // Validate
      let result = await harness.executeTransition('validate', instance.aggregate_id);
      expect(result).toHaveTokenIn('validated');

      // Process payment
      result = await harness.executeTransition('process_payment', instance.aggregate_id);
      expect(result).toHaveTokenIn('paid');

      // Ship
      result = await harness.executeTransition('ship', instance.aggregate_id);
      expect(result).toHaveTokenIn('shipped');

      // Confirm
      result = await harness.executeTransition('confirm', instance.aggregate_id);
      expect(result).toHaveTokenIn('completed');
    });

    test('cannot skip steps in workflow', async () => {
      const instance = await harness.createInstance();

      // Try to ship without validating and paying first
      await expect(
        harness.executeTransition('ship', instance.aggregate_id)
      ).rejects.toThrow();
    });
  });

  describe('browser navigation', () => {
    test('can navigate to list page', async () => {
      await harness.navigate('/order-processing');
      const state = await harness.getPageState();
      expect(state.url).toContain('/order-processing');
    });
  });
});
