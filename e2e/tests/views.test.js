/**
 * E2E tests for views and data projection.
 *
 * Uses window.pilot API to test that view definitions correctly
 * filter fields and that event data is properly projected.
 */

const { TestHarness } = require('../lib/test-harness');

describe('Views and Data Projection', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('order-processing');
    await harness.setup();
    await harness.login(['admin', 'fulfillment', 'system', 'customer']);
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('list page view', () => {
    test('can navigate to list and see instances', async () => {
      // Create a couple of instances
      await harness.pilot.create();
      await harness.pilot.create();

      // Navigate to list
      await harness.pilot.list();

      // Check we're on list page
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing');

      // Should have instances displayed
      const instances = await harness.pilot.getInstances();
      expect(instances.length).toBeGreaterThanOrEqual(2);
    });

    test('list shows instance status', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      await harness.pilot.list();

      // Page should render status badges
      const hasCard = await harness.pilot.exists('.entity-card');
      expect(hasCard).toBe(true);
    });
  });

  describe('detail page view', () => {
    test('shows current state information', async () => {
      await harness.pilot.create();
      await harness.pilot.assertState('received');

      // Should be on detail page
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing/:id');

      // Should show status
      const status = await harness.pilot.getStatus();
      expect(status).toBe('received');
    });

    test('shows enabled transitions as buttons', async () => {
      await harness.pilot.create();

      const buttons = await harness.pilot.getButtons();
      const buttonTexts = buttons.map(b => b.text.toLowerCase());

      // Should have validate and reject buttons
      expect(buttonTexts.some(t => t.includes('validate'))).toBe(true);
      expect(buttonTexts.some(t => t.includes('reject'))).toBe(true);
    });

    test('disabled transitions show as disabled buttons', async () => {
      await harness.pilot.create();

      const buttons = await harness.pilot.getButtons();

      // Ship should be disabled in received state
      const shipButton = buttons.find(b => b.text.toLowerCase().includes('ship'));
      if (shipButton) {
        expect(shipButton.disabled).toBe(true);
      }
    });

    test('buttons update after transition', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const buttons = await harness.pilot.getButtons();
      const buttonTexts = buttons.map(b => b.text.toLowerCase());

      // After validate, process_payment should be enabled
      const paymentButton = buttons.find(b =>
        b.text.toLowerCase().includes('payment') ||
        b.text.toLowerCase().includes('process')
      );
      if (paymentButton) {
        expect(paymentButton.disabled).toBe(false);
      }
    });
  });

  describe('data projection from events', () => {
    test('detail view accumulates data from events', async () => {
      await harness.pilot.create();

      // Add data via validate
      await harness.pilot.action('validate', {
        customer_name: 'Alice',
        total: 100,
        customer_email: 'alice@test.com'
      });

      // Add more data via payment
      await harness.pilot.action('process_payment', {
        payment_method: 'credit_card',
        payment_status: 'completed'
      });

      // Events should contain all the data
      const events = await harness.pilot.getEvents();
      const allData = {};
      for (const e of events) {
        if (e.data) Object.assign(allData, e.data);
      }

      expect(allData.customer_name).toBe('Alice');
      expect(allData.payment_method).toBe('credit_card');
    });
  });

  describe('UI elements', () => {
    test('page has required structure', async () => {
      await harness.pilot.create();

      // Should have page wrapper
      await harness.pilot.assertExists('.page');

      // Should have card for content
      await harness.pilot.assertExists('.card');
    });

    test('can get text from elements', async () => {
      await harness.pilot.create();

      // Should have a heading
      const headingText = await harness.pilot.getText('h1');
      expect(headingText).toBeTruthy();
    });

    test('back button navigates to list', async () => {
      await harness.pilot.create();

      // Click back button
      await harness.pilot.clickButton('← Back to List');

      // Should be on list page
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing');
    });
  });

  describe('admin view', () => {
    test('can navigate to admin dashboard', async () => {
      await harness.pilot.admin();

      const route = await harness.pilot.getRoute();
      expect(route.path).toMatch(/\/admin/);
    });

    test('admin shows statistics', async () => {
      // Create some instances first
      await harness.pilot.create();
      await harness.pilot.create();

      await harness.pilot.admin();

      // Should have stats card
      const hasStats = await harness.pilot.exists('#admin-stats');
      expect(hasStats).toBe(true);
    });

    test('admin shows recent instances', async () => {
      await harness.pilot.create();
      await harness.pilot.admin();

      // Should have instances section
      const hasInstances = await harness.pilot.exists('#admin-instances');
      expect(hasInstances).toBe(true);
    });
  });

  describe('form view', () => {
    test('new form page has submit button', async () => {
      await harness.pilot.newForm();

      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing/new');

      const buttons = await harness.pilot.getButtons();
      const hasCreate = buttons.some(b =>
        b.text.toLowerCase().includes('create')
      );
      expect(hasCreate).toBe(true);
    });

    test('can submit form to create instance', async () => {
      await harness.pilot.newForm();

      // Submit the form
      await harness.pilot.submit();

      // Should navigate to detail page
      const route = await harness.pilot.getRoute();
      expect(route.path).toBe('/order-processing/:id');

      // Should be in initial state
      const status = await harness.pilot.getStatus();
      expect(status).toBe('received');
    });
  });
});
