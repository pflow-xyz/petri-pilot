/**
 * E2E tests for the job-application app.
 *
 * Workflow: applied → start_screening → screening → schedule_phone_screen/start_background_check →
 *           phone_screen_pending/background_check_pending → complete_phone_screen/complete_background_check →
 *           phone_screen_complete/background_check_complete → advance_to_interview → ready_for_interview →
 *           conduct_interview → interviewing → extend_offer → offer_extended → accept_offer → hired
 * Alternatives:
 *   - screening → reject_after_screen → rejected
 *   - interviewing → reject_after_interview → rejected
 *   - offer_extended → decline_offer → rejected
 */

const { TestHarness } = require('../lib/test-harness');

describe('job-application', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('job-application');
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
      await harness.login(['admin', 'candidate', 'recruiter', 'hiring_manager']);
    });

    test('create instance starts in applied state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('applied');
    });

    test('start_screening transition moves to screening', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('start_screening', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('screening');
    });

    test('reject application workflow', async () => {
      const instance = await harness.createInstance();

      // Screen
      await harness.executeTransition('start_screening', instance.aggregate_id);

      // Reject
      const result = await harness.executeTransition('reject_after_screen', instance.aggregate_id);
      expect(result).toHaveTokenIn('rejected');
    });

    test('phone screen workflow', async () => {
      const instance = await harness.createInstance();

      // Screen
      await harness.executeTransition('start_screening', instance.aggregate_id);

      // Schedule phone screen
      let result = await harness.executeTransition('schedule_phone_screen', instance.aggregate_id);
      expect(result).toHaveTokenIn('phone_screen_pending');

      // Complete phone screen
      result = await harness.executeTransition('complete_phone_screen', instance.aggregate_id);
      expect(result).toHaveTokenIn('phone_screen_complete');
    });

    test('background check workflow', async () => {
      const instance = await harness.createInstance();

      // Screen
      await harness.executeTransition('start_screening', instance.aggregate_id);

      // Start background check
      let result = await harness.executeTransition('start_background_check', instance.aggregate_id);
      expect(result).toHaveTokenIn('background_check_pending');

      // Complete background check
      result = await harness.executeTransition('complete_background_check', instance.aggregate_id);
      expect(result).toHaveTokenIn('background_check_complete');
    });
  });

  describe('enabled transitions', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'candidate', 'recruiter', 'hiring_manager']);
    });

    test('new instance has start_screening transition enabled', async () => {
      const instance = await harness.createInstance();
      const state = await harness.getInstance(instance.aggregate_id);
      expect(state).toHaveTransitionEnabled('start_screening');
    });
  });
});
