/**
 * E2E tests for the task-manager app.
 *
 * Workflow: pending → start → in_progress → submit → review → approve → completed
 * Alternative: review → reject → pending
 */

const { TestHarness } = require('../lib/test-harness');

describe('task-manager', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('task-manager');
    await harness.setup();
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('workflow transitions', () => {
    beforeAll(async () => {
      // Login with all roles needed for the workflow
      await harness.login(['admin', 'user', 'reviewer']);
    });

    test('create instance starts in pending state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('pending');
    });

    test('start transition moves to in_progress state', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('start', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('in_progress');
    });

    test('complete task workflow', async () => {
      const instance = await harness.createInstance();

      // Start working
      let result = await harness.executeTransition('start', instance.aggregate_id);
      expect(result).toHaveTokenIn('in_progress');

      // Submit for review
      result = await harness.executeTransition('submit', instance.aggregate_id);
      expect(result).toHaveTokenIn('review');

      // Approve
      result = await harness.executeTransition('approve', instance.aggregate_id);
      expect(result).toHaveTokenIn('completed');
    });

    test('reject workflow returns to pending', async () => {
      const instance = await harness.createInstance();

      // Start and submit
      await harness.executeTransition('start', instance.aggregate_id);
      await harness.executeTransition('submit', instance.aggregate_id);

      // Reject
      const result = await harness.executeTransition('reject', instance.aggregate_id);
      expect(result).toHaveTokenIn('pending');
    });
  });

  describe('debug eval commands', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'user', 'reviewer']);
    });

    test('can get all places', async () => {
      const instance = await harness.createInstance();
      const state = await harness.getInstance(instance.aggregate_id);
      expect(state.places).toBeDefined();
      expect(typeof state.places.pending).toBe('number');
    });

    test('can list enabled transitions', async () => {
      const instance = await harness.createInstance();
      const state = await harness.getInstance(instance.aggregate_id);
      expect(state.enabled_transitions).toBeDefined();
      expect(state.enabled_transitions).toContain('start');
    });
  });
});
