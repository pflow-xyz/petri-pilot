/**
 * E2E tests for the ecommerce-checkout app.
 *
 * Workflow: cart → start_checkout → checkout_started → enter_payment → payment_pending →
 *           process_payment → payment_processing → payment_success → paid → fulfill → fulfilled
 * Alternatives:
 *   - payment_processing → payment_fail_1 → retry_1 → retry_payment_1 → payment_processing
 *   - payment_processing → payment_fail_2 → retry_2 → retry_payment_2 → payment_processing
 *   - payment_processing → payment_fail_3 → retry_3 → cancel_order → cancelled
 */

const { TestHarness } = require('../lib/test-harness');

describe('ecommerce-checkout', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('ecommerce-checkout');
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
      await harness.login(['admin', 'customer', 'system', 'fulfillment']);
    });

    test('create instance starts in cart state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('cart');
    });

    test('start_checkout transition moves to checkout_started', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('start_checkout', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('checkout_started');
    });

    test('complete successful checkout workflow', async () => {
      const instance = await harness.createInstance();

      // Start checkout
      let result = await harness.executeTransition('start_checkout', instance.aggregate_id);
      expect(result).toHaveTokenIn('checkout_started');

      // Enter payment
      result = await harness.executeTransition('enter_payment', instance.aggregate_id);
      expect(result).toHaveTokenIn('payment_pending');

      // Process payment
      result = await harness.executeTransition('process_payment', instance.aggregate_id);
      expect(result).toHaveTokenIn('payment_processing');

      // Payment success
      result = await harness.executeTransition('payment_success', instance.aggregate_id);
      expect(result).toHaveTokenIn('paid');
    });

    test('payment failure with retry', async () => {
      const instance = await harness.createInstance();

      // Go through checkout to payment processing
      await harness.executeTransition('start_checkout', instance.aggregate_id);
      await harness.executeTransition('enter_payment', instance.aggregate_id);
      await harness.executeTransition('process_payment', instance.aggregate_id);

      // Payment fails (first attempt)
      let result = await harness.executeTransition('payment_fail_1', instance.aggregate_id);
      expect(result).toHaveTokenIn('retry_1');

      // Retry payment
      result = await harness.executeTransition('retry_payment_1', instance.aggregate_id);
      expect(result).toHaveTokenIn('payment_processing');

      // Complete payment on retry
      result = await harness.executeTransition('payment_success', instance.aggregate_id);
      expect(result).toHaveTokenIn('paid');
    });

    test('fulfillment workflow', async () => {
      const instance = await harness.createInstance();

      // Complete checkout
      await harness.executeTransition('start_checkout', instance.aggregate_id);
      await harness.executeTransition('enter_payment', instance.aggregate_id);
      await harness.executeTransition('process_payment', instance.aggregate_id);
      await harness.executeTransition('payment_success', instance.aggregate_id);

      // Fulfill
      const result = await harness.executeTransition('fulfill', instance.aggregate_id);
      expect(result).toHaveTokenIn('fulfilled');
    });
  });
});
