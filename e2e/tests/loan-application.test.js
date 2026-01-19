/**
 * E2E tests for the loan-application app.
 *
 * Workflow: submitted → run_credit_check → credit_check → auto_approve/flag_for_review/auto_deny →
 *           auto_approved/manual_review/denied → finalize_approval/underwriter_approve/underwriter_deny →
 *           approved/denied → disburse → disbursed → start_repayment → repaying →
 *           make_payment/complete/mark_default → repaying/paid_off/defaulted
 */

const { TestHarness } = require('../lib/test-harness');

describe('loan-application', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('loan-application');
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
      await harness.login(['admin', 'system', 'underwriter', 'applicant']);
    });

    test('create instance starts in submitted state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('submitted');
    });

    test('run_credit_check transition moves to credit_check', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('run_credit_check', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('credit_check');
    });

    test('auto approval workflow', async () => {
      const instance = await harness.createInstance();

      // Credit check
      await harness.executeTransition('run_credit_check', instance.aggregate_id);

      // Auto approve
      let result = await harness.executeTransition('auto_approve', instance.aggregate_id);
      expect(result).toHaveTokenIn('auto_approved');

      // Finalize approval
      result = await harness.executeTransition('finalize_approval', instance.aggregate_id);
      expect(result).toHaveTokenIn('approved');
    });

    test('manual review workflow - approve', async () => {
      const instance = await harness.createInstance();

      // Credit check
      await harness.executeTransition('run_credit_check', instance.aggregate_id);

      // Flag for manual review
      let result = await harness.executeTransition('flag_for_review', instance.aggregate_id);
      expect(result).toHaveTokenIn('manual_review');

      // Underwriter approves
      result = await harness.executeTransition('underwriter_approve', instance.aggregate_id);
      expect(result).toHaveTokenIn('approved');
    });

    test('manual review workflow - deny', async () => {
      const instance = await harness.createInstance();

      // Credit check
      await harness.executeTransition('run_credit_check', instance.aggregate_id);

      // Flag for manual review
      await harness.executeTransition('flag_for_review', instance.aggregate_id);

      // Underwriter denies
      const result = await harness.executeTransition('underwriter_deny', instance.aggregate_id);
      expect(result).toHaveTokenIn('denied');
    });

    test('disbursement and payment workflow', async () => {
      const instance = await harness.createInstance();

      // Get to approved
      await harness.executeTransition('run_credit_check', instance.aggregate_id);
      await harness.executeTransition('auto_approve', instance.aggregate_id);
      await harness.executeTransition('finalize_approval', instance.aggregate_id);

      // Disburse funds
      let result = await harness.executeTransition('disburse', instance.aggregate_id);
      expect(result).toHaveTokenIn('disbursed');

      // Start repayment
      result = await harness.executeTransition('start_repayment', instance.aggregate_id);
      expect(result).toHaveTokenIn('repaying');

      // Make a payment (stays in repaying state per model)
      result = await harness.executeTransition('make_payment', instance.aggregate_id);
      expect(result).toHaveTokenIn('repaying');

      // Complete loan
      result = await harness.executeTransition('complete', instance.aggregate_id);
      expect(result).toHaveTokenIn('paid_off');
    });
  });

  describe('API health', () => {
    test('debug sessions endpoint works', async () => {
      const sessions = await harness.apiCall('GET', '/api/debug/sessions');
      expect(sessions.sessions).toBeDefined();
      expect(sessions.sessions.length).toBeGreaterThan(0);
    });
  });
});
