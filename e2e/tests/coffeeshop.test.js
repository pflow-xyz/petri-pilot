/**
 * E2E tests for the coffeeshop app.
 *
 * Tests all coffee shop operations: ordering, making, and serving drinks.
 *
 * Workflow for each drink type:
 * - Order: order_espresso/latte/cappuccino → creates pending order
 * - Make: make_espresso/latte/cappuccino → consumes resources, creates ready drink
 * - Serve: serve_espresso/latte/cappuccino → completes the order
 *
 * Resources: coffee_beans, milk, cups
 */

const { TestHarness } = require('../lib/test-harness');

describe('coffeeshop', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('coffeeshop');
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
  });

  describe('workflow introspection', () => {
    test('can get all places', async () => {
      const places = await harness.pilot.getPlaces();
      const placeIds = places.map(p => p.id);
      // Resources
      expect(placeIds).toContain('coffee_beans');
      expect(placeIds).toContain('milk');
      expect(placeIds).toContain('cups');
      // Workflow states
      expect(placeIds).toContain('orders_pending');
      expect(placeIds).toContain('espresso_ready');
      expect(placeIds).toContain('latte_ready');
      expect(placeIds).toContain('cappuccino_ready');
      expect(placeIds).toContain('orders_complete');
    });

    test('can get all transitions', async () => {
      const transitions = await harness.pilot.getTransitions();
      const transitionIds = transitions.map(t => t.id);
      // Order transitions
      expect(transitionIds).toContain('order_espresso');
      expect(transitionIds).toContain('order_latte');
      expect(transitionIds).toContain('order_cappuccino');
      // Make transitions
      expect(transitionIds).toContain('make_espresso');
      expect(transitionIds).toContain('make_latte');
      expect(transitionIds).toContain('make_cappuccino');
      // Serve transitions
      expect(transitionIds).toContain('serve_espresso');
      expect(transitionIds).toContain('serve_latte');
      expect(transitionIds).toContain('serve_cappuccino');
    });
  });

  describe('espresso workflow', () => {
    test('order_espresso event - customer orders espresso', async () => {
      const instance = await harness.pilot.create();
      expect(instance.id).toBeDefined();

      const result = await harness.pilot.action('order_espresso');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const orderEvent = events.find(e =>
        e.type.toLowerCase().includes('order') && e.type.toLowerCase().includes('espresso')
      );
      expect(orderEvent).toBeDefined();
    });

    test('make_espresso event - barista makes espresso', async () => {
      await harness.pilot.create();

      // First order
      await harness.pilot.action('order_espresso');

      // Then make
      const result = await harness.pilot.action('make_espresso');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const makeEvent = events.find(e =>
        e.type.toLowerCase().includes('make') && e.type.toLowerCase().includes('espresso')
      );
      expect(makeEvent).toBeDefined();
    });

    test('serve_espresso event - serve to customer', async () => {
      await harness.pilot.create();

      // Order -> Make -> Serve
      await harness.pilot.action('order_espresso');
      await harness.pilot.action('make_espresso');
      const result = await harness.pilot.action('serve_espresso');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const serveEvent = events.find(e =>
        e.type.toLowerCase().includes('serve') && e.type.toLowerCase().includes('espresso')
      );
      expect(serveEvent).toBeDefined();
    });

    test('complete espresso workflow', async () => {
      await harness.pilot.create();

      const results = await harness.pilot.sequence([
        'order_espresso',
        'make_espresso',
        'serve_espresso'
      ]);

      expect(results.length).toBe(3);
      expect(results.every(r => r.success)).toBe(true);
    });
  });

  describe('latte workflow', () => {
    test('order_latte event - customer orders latte', async () => {
      await harness.pilot.create();

      const result = await harness.pilot.action('order_latte');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const orderEvent = events.find(e =>
        e.type.toLowerCase().includes('order') && e.type.toLowerCase().includes('latte')
      );
      expect(orderEvent).toBeDefined();
    });

    test('make_latte event - barista makes latte (uses milk)', async () => {
      await harness.pilot.create();
      await harness.pilot.action('order_latte');

      const result = await harness.pilot.action('make_latte');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const makeEvent = events.find(e =>
        e.type.toLowerCase().includes('make') && e.type.toLowerCase().includes('latte')
      );
      expect(makeEvent).toBeDefined();
    });

    test('serve_latte event - serve to customer', async () => {
      await harness.pilot.create();
      await harness.pilot.action('order_latte');
      await harness.pilot.action('make_latte');

      const result = await harness.pilot.action('serve_latte');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const serveEvent = events.find(e =>
        e.type.toLowerCase().includes('serve') && e.type.toLowerCase().includes('latte')
      );
      expect(serveEvent).toBeDefined();
    });

    test('complete latte workflow', async () => {
      await harness.pilot.create();

      const results = await harness.pilot.sequence([
        'order_latte',
        'make_latte',
        'serve_latte'
      ]);

      expect(results.length).toBe(3);
      expect(results.every(r => r.success)).toBe(true);
    });
  });

  describe('cappuccino workflow', () => {
    test('order_cappuccino event - customer orders cappuccino', async () => {
      await harness.pilot.create();

      const result = await harness.pilot.action('order_cappuccino');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const orderEvent = events.find(e =>
        e.type.toLowerCase().includes('order') && e.type.toLowerCase().includes('cappuccino')
      );
      expect(orderEvent).toBeDefined();
    });

    test('make_cappuccino event - barista makes cappuccino (uses milk)', async () => {
      await harness.pilot.create();
      await harness.pilot.action('order_cappuccino');

      const result = await harness.pilot.action('make_cappuccino');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const makeEvent = events.find(e =>
        e.type.toLowerCase().includes('make') && e.type.toLowerCase().includes('cappuccino')
      );
      expect(makeEvent).toBeDefined();
    });

    test('serve_cappuccino event - serve to customer', async () => {
      await harness.pilot.create();
      await harness.pilot.action('order_cappuccino');
      await harness.pilot.action('make_cappuccino');

      const result = await harness.pilot.action('serve_cappuccino');
      expect(result.success).toBe(true);

      const events = await harness.pilot.getEvents();
      const serveEvent = events.find(e =>
        e.type.toLowerCase().includes('serve') && e.type.toLowerCase().includes('cappuccino')
      );
      expect(serveEvent).toBeDefined();
    });

    test('complete cappuccino workflow', async () => {
      await harness.pilot.create();

      const results = await harness.pilot.sequence([
        'order_cappuccino',
        'make_cappuccino',
        'serve_cappuccino'
      ]);

      expect(results.length).toBe(3);
      expect(results.every(r => r.success)).toBe(true);
    });
  });

  describe('mixed orders', () => {
    test('can process multiple different drink types', async () => {
      await harness.pilot.create();

      // Order all three types
      await harness.pilot.action('order_espresso');
      await harness.pilot.action('order_latte');
      await harness.pilot.action('order_cappuccino');

      // Make all three
      await harness.pilot.action('make_espresso');
      await harness.pilot.action('make_latte');
      await harness.pilot.action('make_cappuccino');

      // Serve all three
      await harness.pilot.action('serve_espresso');
      await harness.pilot.action('serve_latte');
      await harness.pilot.action('serve_cappuccino');

      // Verify we have 9 events (3 orders + 3 makes + 3 serves)
      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(9);
    });

    test('all nine event types are recorded', async () => {
      await harness.pilot.create();

      // Execute all transitions
      await harness.pilot.sequence([
        'order_espresso', 'order_latte', 'order_cappuccino',
        'make_espresso', 'make_latte', 'make_cappuccino',
        'serve_espresso', 'serve_latte', 'serve_cappuccino'
      ]);

      const events = await harness.pilot.getEvents();
      const eventTypes = events.map(e => e.type.toLowerCase());

      // Verify all event types are present
      expect(eventTypes.some(t => t.includes('order') && t.includes('espresso'))).toBe(true);
      expect(eventTypes.some(t => t.includes('order') && t.includes('latte'))).toBe(true);
      expect(eventTypes.some(t => t.includes('order') && t.includes('cappuccino'))).toBe(true);
      expect(eventTypes.some(t => t.includes('make') && t.includes('espresso'))).toBe(true);
      expect(eventTypes.some(t => t.includes('make') && t.includes('latte'))).toBe(true);
      expect(eventTypes.some(t => t.includes('make') && t.includes('cappuccino'))).toBe(true);
      expect(eventTypes.some(t => t.includes('serve') && t.includes('espresso'))).toBe(true);
      expect(eventTypes.some(t => t.includes('serve') && t.includes('latte'))).toBe(true);
      expect(eventTypes.some(t => t.includes('serve') && t.includes('cappuccino'))).toBe(true);
    });
  });

  describe('event sourcing', () => {
    test('events are recorded in order', async () => {
      await harness.pilot.create();

      await harness.pilot.action('order_espresso');
      await harness.pilot.action('make_espresso');
      await harness.pilot.action('serve_espresso');

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(3);

      // Events should have increasing versions
      for (let i = 1; i < events.length; i++) {
        expect(events[i].version).toBeGreaterThan(events[i - 1].version);
      }
    });

    test('can get event count', async () => {
      await harness.pilot.create();
      await harness.pilot.action('order_latte');
      await harness.pilot.action('make_latte');

      const count = await harness.pilot.getEventCount();
      expect(count).toBeGreaterThanOrEqual(2);
    });
  });
});
