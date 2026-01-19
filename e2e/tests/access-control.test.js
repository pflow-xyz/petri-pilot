/**
 * E2E tests for Role-Based Access Control.
 *
 * Tests role-based access control including:
 * - Role-based transition restrictions
 * - Role inheritance
 * - Access denial for unauthorized roles
 */

const { TestHarness } = require('../lib/test-harness');

describe('Role-Based Access Control', () => {
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

  describe('unauthorized role access', () => {
    test('should deny transition to unauthorized role', async () => {
      // Login as customer (who doesn't have permission to validate orders)
      await harness.login('customer');

      // Create an instance
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();

      // Try to execute validate transition (requires fulfillment role)
      await expect(
        harness.executeTransition('validate', instance.aggregate_id, {})
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });

    test('should deny reject transition to customer role', async () => {
      // Login as customer
      await harness.login('customer');

      // Create an instance
      const instance = await harness.createInstance();
      
      // Try to reject (requires fulfillment role)
      await expect(
        harness.executeTransition('reject', instance.aggregate_id, {})
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });

    test('should deny ship transition to customer role', async () => {
      // Login as customer
      await harness.login('customer');

      // Create an instance
      const instance = await harness.createInstance();
      
      // Try to ship directly from received state
      // This should fail due to permissions (fulfillment required)
      // before any state validation occurs
      await expect(
        harness.executeTransition('ship', instance.aggregate_id, {})
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });
  });

  describe('role inheritance', () => {
    test('should allow inherited role permissions', async () => {
      // Login as admin (admin inherits from fulfillment)
      await harness.login('admin');

      // Create an instance
      const instance = await harness.createInstance();
      expect(instance).toHaveTokenIn('received');

      // Execute validate transition (requires fulfillment role, which admin inherits)
      const result = await harness.executeTransition('validate', instance.aggregate_id, {});
      
      // Should succeed
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('validated');
    });

    test('should allow admin to perform fulfillment actions', async () => {
      // Login as admin
      await harness.login('admin');

      // Create instance
      const instance = await harness.createInstance();
      
      // Validate (fulfillment action that admin has via inheritance)
      let result = await harness.executeTransition('validate', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('validated');

      // Process payment requires system role
      // Admin does NOT inherit system role (only inherits from fulfillment)
      // Therefore we must add system role explicitly for this step
      await harness.login(['admin', 'system']);
      result = await harness.executeTransition('process_payment', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('paid');

      // Ship (fulfillment action - admin has this through inheritance)
      await harness.login('admin');
      result = await harness.executeTransition('ship', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('shipped');

      // Confirm (fulfillment action)
      result = await harness.executeTransition('confirm', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('completed');
    });

    test('should allow reject transition with inherited permissions', async () => {
      // Login as admin
      await harness.login('admin');

      // Create instance
      const instance = await harness.createInstance();
      expect(instance).toHaveTokenIn('received');

      // Reject (requires fulfillment, admin should inherit this)
      const result = await harness.executeTransition('reject', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('rejected');
    });
  });

  describe('role-specific transitions', () => {
    test('should allow fulfillment role to validate', async () => {
      // Login as fulfillment
      await harness.login('fulfillment');

      // Create instance
      const instance = await harness.createInstance();
      
      // Validate (fulfillment action)
      const result = await harness.executeTransition('validate', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('validated');
    });

    test('should deny system-only transition to fulfillment role', async () => {
      // First, setup an instance in validated state using admin role
      await harness.login('admin');
      
      const instance = await harness.createInstance();
      await harness.executeTransition('validate', instance.aggregate_id, {});
      
      // Now switch to fulfillment-only role
      await harness.login('fulfillment');
      
      // Try to process payment (requires system role, not inherited by fulfillment)
      await expect(
        harness.executeTransition('process_payment', instance.aggregate_id, {})
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });

    test('should allow system role to process payment', async () => {
      // Setup instance in validated state
      await harness.login(['fulfillment', 'system']);
      
      const instance = await harness.createInstance();
      await harness.executeTransition('validate', instance.aggregate_id, {});
      
      // Login as system only
      await harness.login('system');
      
      // Process payment (system action)
      const result = await harness.executeTransition('process_payment', instance.aggregate_id, {});
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('paid');
    });
  });

  describe('fireTransition helper', () => {
    test('should work with fireTransition shorthand', async () => {
      // Login as admin
      await harness.login('admin');

      // Use fireTransition (creates instance automatically)
      const result = await harness.fireTransition('validate', {});
      
      // Should succeed
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('validated');
    });

    test('should deny unauthorized access via fireTransition', async () => {
      // Login as customer
      await harness.login('customer');

      // Try to validate via fireTransition
      await expect(
        harness.fireTransition('validate', {})
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });
  });

  // NOTE: Guard expression tests are not included because the order-processing
  // example does not define any guard expressions in its schema. Guard expressions
  // are runtime conditions that check data values (e.g., user.id == assignee_id).
  // To test guard expressions, use an example that includes them, such as:
  // - task-manager-app.json (has guards like "user.id == assignee_id")
  // - order-system.json (has guards like "inventory[product_id] >= quantity")
  // - token-ledger.json (has guards like "balances[from] >= amount")
});
