/**
 * E2E tests for the support-ticket app.
 *
 * Workflow: new → assign → assigned → start_work → in_progress → resolve → resolved → close → closed
 * Alternatives:
 *   - in_progress → escalate → escalated → resolve → resolved
 *   - in_progress → request_info → pending_customer → customer_reply → in_progress
 *   - closed → reopen → new
 */

const { TestHarness } = require('../lib/test-harness');

describe('support-ticket', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('support-ticket');
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
      await harness.login(['admin', 'customer', 'agent', 'supervisor']);
    });

    test('create instance starts in new state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('new');
    });

    test('assign transition moves to assigned state', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('assign', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('assigned');
    });

    test('complete standard resolution workflow', async () => {
      const instance = await harness.createInstance();

      // Assign
      let result = await harness.executeTransition('assign', instance.aggregate_id);
      expect(result).toHaveTokenIn('assigned');

      // Start work
      result = await harness.executeTransition('start_work', instance.aggregate_id);
      expect(result).toHaveTokenIn('in_progress');

      // Resolve
      result = await harness.executeTransition('resolve', instance.aggregate_id);
      expect(result).toHaveTokenIn('resolved');

      // Close
      result = await harness.executeTransition('close', instance.aggregate_id);
      expect(result).toHaveTokenIn('closed');
    });

    test('escalation workflow', async () => {
      const instance = await harness.createInstance();

      // Assign and start
      await harness.executeTransition('assign', instance.aggregate_id);
      await harness.executeTransition('start_work', instance.aggregate_id);

      // Escalate
      let result = await harness.executeTransition('escalate', instance.aggregate_id);
      expect(result).toHaveTokenIn('escalated');

      // Resolve escalated ticket (uses separate transition for escalated tickets)
      result = await harness.executeTransition('resolve_escalated', instance.aggregate_id);
      expect(result).toHaveTokenIn('resolved');
    });

    test('customer info request workflow', async () => {
      const instance = await harness.createInstance();

      // Assign and start
      await harness.executeTransition('assign', instance.aggregate_id);
      await harness.executeTransition('start_work', instance.aggregate_id);

      // Request info
      let result = await harness.executeTransition('request_info', instance.aggregate_id);
      expect(result).toHaveTokenIn('pending_customer');

      // Customer reply
      result = await harness.executeTransition('customer_reply', instance.aggregate_id);
      expect(result).toHaveTokenIn('in_progress');
    });

    test('reopen workflow', async () => {
      const instance = await harness.createInstance();

      // Go through full workflow to closed
      await harness.executeTransition('assign', instance.aggregate_id);
      await harness.executeTransition('start_work', instance.aggregate_id);
      await harness.executeTransition('resolve', instance.aggregate_id);
      await harness.executeTransition('close', instance.aggregate_id);

      // Reopen
      const result = await harness.executeTransition('reopen', instance.aggregate_id);
      expect(result).toHaveTokenIn('new');
    });
  });
});
