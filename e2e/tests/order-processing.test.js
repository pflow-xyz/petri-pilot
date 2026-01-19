/**
 * E2E tests for the order-processing app.
 *
 * Uses window.pilot API to test complete user flows through the UI.
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

  describe('pilot API availability', () => {
    test('window.pilot is available', async () => {
      const hasPilot = await harness.eval('return typeof window.pilot === "object"');
      expect(hasPilot).toBe(true);
    });

    test('pilot has all expected methods', async () => {
      const methods = await harness.eval(`
        return Object.keys(window.pilot).filter(k => typeof window.pilot[k] === 'function')
      `);
      expect(methods).toContain('create');
      expect(methods).toContain('action');
      expect(methods).toContain('getState');
      expect(methods).toContain('loginAs');
      expect(methods).toContain('sequence');
      expect(methods).toContain('assertState');
    });
  });

  describe('workflow introspection', () => {
    test('can get workflow info', async () => {
      const info = await harness.pilot.getWorkflowInfo();
      expect(info.places.length).toBeGreaterThan(0);
      expect(info.transitions.length).toBeGreaterThan(0);
      expect(info.initialPlace).toBe('received');
    });

    test('can get all places', async () => {
      const places = await harness.pilot.getPlaces();
      const placeIds = places.map(p => p.id);
      expect(placeIds).toContain('received');
      expect(placeIds).toContain('validated');
      expect(placeIds).toContain('paid');
      expect(placeIds).toContain('shipped');
      expect(placeIds).toContain('completed');
      expect(placeIds).toContain('rejected');
    });

    test('can get all transitions', async () => {
      const transitions = await harness.pilot.getTransitions();
      const transitionIds = transitions.map(t => t.id);
      expect(transitionIds).toContain('validate');
      expect(transitionIds).toContain('reject');
      expect(transitionIds).toContain('process_payment');
      expect(transitionIds).toContain('ship');
      expect(transitionIds).toContain('confirm');
    });
  });

  describe('user flow: create and view instance', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('can navigate to list page', async () => {
      await harness.pilot.list();
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing');
    });

    test('can create instance via UI flow', async () => {
      // Navigate to new form
      await harness.pilot.newForm();
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing/new');

      // Submit form to create instance
      const instance = await harness.pilot.submit();

      // Should navigate to detail page
      const newRoute = await harness.pilot.getRoute();
      expect(newRoute.path).toBe('/order-processing/:id');
    });

    test('can create instance directly and view it', async () => {
      const result = await harness.pilot.create();
      expect(result.id).toBeDefined();

      // Should be on detail page
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing/:id');

      // Should be in received state
      const status = await harness.pilot.getStatus();
      expect(status).toBe('received');
    });
  });

  describe('user flow: happy path workflow', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('complete workflow using pilot.action()', async () => {
      // Create instance
      const instance = await harness.pilot.create();
      expect(instance.id).toBeDefined();

      // Check initial state
      await harness.pilot.assertState('received');

      // Check which transitions are enabled
      let enabled = await harness.pilot.getEnabled();
      expect(enabled).toContain('validate');
      expect(enabled).toContain('reject');
      expect(enabled).not.toContain('ship');

      // Validate
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');

      // Process payment
      await harness.pilot.action('process_payment');
      await harness.pilot.assertState('paid');

      // Ship
      await harness.pilot.action('ship');
      await harness.pilot.assertState('shipped');

      // Confirm
      await harness.pilot.action('confirm');
      await harness.pilot.assertState('completed');

      // No more transitions should be enabled
      enabled = await harness.pilot.getEnabled();
      expect(enabled.length).toBe(0);
    });

    test('complete workflow using pilot.sequence()', async () => {
      // Create instance
      await harness.pilot.create();

      // Execute full workflow as sequence
      const results = await harness.pilot.sequence([
        'validate',
        'process_payment',
        'ship',
        'confirm'
      ]);

      // All should succeed
      expect(results.length).toBe(4);
      expect(results.every(r => r.success)).toBe(true);

      // Should be in completed state
      await harness.pilot.assertState('completed');
    });
  });

  describe('user flow: rejection path', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('can reject order from received state', async () => {
      await harness.pilot.create();
      await harness.pilot.assertState('received');

      // Check canFire before action
      const check = await harness.pilot.canFire('reject');
      expect(check.canFire).toBe(true);

      await harness.pilot.action('reject');
      await harness.pilot.assertState('rejected');

      // No transitions should be enabled from rejected
      const enabled = await harness.pilot.getEnabled();
      expect(enabled.length).toBe(0);
    });
  });

  describe('user flow: disabled transitions', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('cannot skip workflow steps', async () => {
      await harness.pilot.create();
      await harness.pilot.assertState('received');

      // Check that ship is not enabled
      await harness.pilot.assertDisabled('ship');
      await harness.pilot.assertDisabled('confirm');

      // canFire should explain why
      const check = await harness.pilot.canFire('ship');
      expect(check.canFire).toBe(false);
      expect(check.reason).toContain('not enabled');
      expect(check.currentState).toBe('received');
    });

    test('sequence fails on disabled transition', async () => {
      await harness.pilot.create();

      // Try to execute invalid sequence
      await expect(
        harness.pilot.sequence(['ship', 'confirm'])
      ).rejects.toThrow(/not enabled/);
    });
  });

  describe('event sourcing', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('can get event history after transitions', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.action('process_payment');

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(2);
    });

    test('can get event count', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const count = await harness.pilot.getEventCount();
      expect(count).toBeGreaterThanOrEqual(1);
    });

    test('can get last event', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const lastEvent = await harness.pilot.getLastEvent();
      expect(lastEvent).toBeDefined();
      expect(lastEvent.type).toBeDefined();
    });
  });

  describe('UI inspection', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('can get buttons on detail page', async () => {
      await harness.pilot.create();

      const buttons = await harness.pilot.getButtons();
      expect(buttons.length).toBeGreaterThan(0);

      // Should have action buttons
      const buttonTexts = buttons.map(b => b.text);
      expect(buttonTexts.some(t => t.toLowerCase().includes('validate'))).toBe(true);
    });

    test('can check element existence', async () => {
      await harness.pilot.create();

      // Should have page content
      const hasPage = await harness.pilot.exists('.page');
      expect(hasPage).toBe(true);

      // Should have card
      const hasCard = await harness.pilot.exists('.card');
      expect(hasCard).toBe(true);
    });

    test('can click button by text', async () => {
      await harness.pilot.create();
      await harness.pilot.assertState('received');

      // Click the Validate button
      await harness.pilot.clickButton('Validate');

      // Should have transitioned
      await harness.pilot.assertState('validated');
    });
  });

  describe('authentication', () => {
    test('can check authentication status', async () => {
      await harness.login('admin');

      const isAuth = await harness.pilot.isAuthenticated();
      expect(isAuth).toBe(true);

      const user = await harness.pilot.getUser();
      expect(user).toBeDefined();
    });

    test('can check user roles', async () => {
      await harness.login(['admin', 'fulfillment']);

      const roles = await harness.pilot.getRoles();
      expect(roles).toContain('admin');
      expect(roles).toContain('fulfillment');

      const hasAdmin = await harness.pilot.hasRole('admin');
      expect(hasAdmin).toBe(true);
    });

    test('can logout', async () => {
      await harness.login('admin');
      expect(await harness.pilot.isAuthenticated()).toBe(true);

      await harness.logout();
      expect(await harness.pilot.isAuthenticated()).toBe(false);
    });
  });

  describe('debug helpers', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'fulfillment', 'system', 'customer']);
    });

    test('debug() returns useful info', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const debug = await harness.pilot.debug();
      expect(debug.route).toBeDefined();
      expect(debug.instance).toBeDefined();
      expect(debug.instance.places.validated).toBe(1);
    });
  });
});
