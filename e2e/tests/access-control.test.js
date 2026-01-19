/**
 * E2E tests for Role-Based Access Control.
 *
 * Uses window.pilot API to test role-based access patterns:
 * - Role-based transition restrictions
 * - Role inheritance
 * - Access denial for unauthorized roles
 * - Role switching during workflows
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

  describe('role introspection', () => {
    test('can check current roles after login', async () => {
      await harness.login(['admin', 'fulfillment']);

      const roles = await harness.pilot.getRoles();
      expect(roles).toContain('admin');
      expect(roles).toContain('fulfillment');
    });

    test('hasRole returns correct values', async () => {
      await harness.login('customer');

      expect(await harness.pilot.hasRole('customer')).toBe(true);
      expect(await harness.pilot.hasRole('admin')).toBe(false);
      expect(await harness.pilot.hasRole('fulfillment')).toBe(false);
    });

    test('assertRole throws for missing role', async () => {
      await harness.login('customer');

      await expect(
        harness.pilot.assertRole('admin')
      ).rejects.toThrow(/admin/);
    });
  });

  describe('unauthorized role access', () => {
    test('customer cannot validate orders', async () => {
      // Login as customer (who lacks fulfillment permission)
      await harness.login('customer');

      // Create instance
      await harness.pilot.create();
      await harness.pilot.assertState('received');

      // Try to validate - should fail with permission error
      await expect(
        harness.pilot.action('validate')
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);

      // State should remain unchanged
      await harness.pilot.assertState('received');
    });

    test('customer cannot reject orders', async () => {
      await harness.login('customer');
      await harness.pilot.create();

      await expect(
        harness.pilot.action('reject')
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });

    test('customer cannot ship orders', async () => {
      await harness.login('customer');
      await harness.pilot.create();

      // Ship requires paid state AND fulfillment role
      // Should fail on permissions before state validation
      await expect(
        harness.pilot.action('ship')
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });
  });

  describe('role inheritance', () => {
    test('admin inherits fulfillment permissions', async () => {
      // Admin role inherits from fulfillment
      await harness.login('admin');

      await harness.pilot.create();
      await harness.pilot.assertState('received');

      // Validate requires fulfillment - admin should have it via inheritance
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');
    });

    test('admin can reject orders via inherited permissions', async () => {
      await harness.login('admin');
      await harness.pilot.create();

      // Reject requires fulfillment - admin inherits this
      await harness.pilot.action('reject');
      await harness.pilot.assertState('rejected');
    });

    test('admin cannot process payment without system role', async () => {
      await harness.login('admin');
      await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');

      // process_payment requires system role - admin doesn't inherit this
      await expect(
        harness.pilot.action('process_payment')
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });
  });

  describe('role-specific transitions', () => {
    test('fulfillment role can validate orders', async () => {
      await harness.login('fulfillment');

      await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');
    });

    test('fulfillment cannot process payment', async () => {
      // Setup: create and validate with admin
      await harness.login('admin');
      await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');

      // Switch to fulfillment only
      await harness.login('fulfillment');

      // process_payment requires system role
      await expect(
        harness.pilot.action('process_payment')
      ).rejects.toThrow(/forbidden|unauthorized|insufficient/i);
    });

    test('system role can process payment', async () => {
      // Setup: validate first
      await harness.login(['fulfillment', 'system']);
      await harness.pilot.create();
      await harness.pilot.action('validate');

      // Switch to system only
      await harness.login('system');

      await harness.pilot.action('process_payment');
      await harness.pilot.assertState('paid');
    });
  });

  describe('multi-role workflow', () => {
    test('complete workflow requires role switching', async () => {
      // Start as fulfillment to create and validate
      await harness.login('fulfillment');
      const inst = await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');

      // Switch to system for payment
      await harness.login('system');
      await harness.pilot.view(inst.id);
      await harness.pilot.action('process_payment');
      await harness.pilot.assertState('paid');

      // Switch back to fulfillment for shipping and confirmation
      await harness.login('fulfillment');
      await harness.pilot.view(inst.id);
      await harness.pilot.action('ship');
      await harness.pilot.assertState('shipped');

      await harness.pilot.action('confirm');
      await harness.pilot.assertState('completed');
    });

    test('admin + system can complete entire workflow', async () => {
      // Admin inherits fulfillment, plus system gives payment access
      await harness.login(['admin', 'system']);

      await harness.pilot.create();

      // Full workflow with both roles
      await harness.pilot.sequence([
        'validate',
        'process_payment',
        'ship',
        'confirm'
      ]);

      await harness.pilot.assertState('completed');
    });
  });

  describe('role persistence', () => {
    test('roles persist across navigation', async () => {
      await harness.login(['admin', 'fulfillment']);

      await harness.pilot.list();
      expect(await harness.pilot.hasRole('admin')).toBe(true);

      await harness.pilot.create();
      expect(await harness.pilot.hasRole('admin')).toBe(true);

      await harness.pilot.list();
      expect(await harness.pilot.hasRole('fulfillment')).toBe(true);
    });

    test('logout clears roles', async () => {
      await harness.login(['admin', 'system']);
      expect(await harness.pilot.isAuthenticated()).toBe(true);
      expect(await harness.pilot.hasRole('admin')).toBe(true);

      await harness.logout();
      expect(await harness.pilot.isAuthenticated()).toBe(false);
      expect(await harness.pilot.getRoles()).toEqual([]);
    });

    test('re-login with different roles works', async () => {
      await harness.login('admin');
      expect(await harness.pilot.hasRole('admin')).toBe(true);
      expect(await harness.pilot.hasRole('customer')).toBe(false);

      // Re-login with different role
      await harness.login('customer');
      expect(await harness.pilot.hasRole('customer')).toBe(true);
      expect(await harness.pilot.hasRole('admin')).toBe(false);
    });
  });
});
