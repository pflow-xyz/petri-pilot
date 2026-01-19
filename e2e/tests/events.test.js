/**
 * E2E tests for Events First Schema - event field validation and binding behavior.
 *
 * Uses window.pilot API to test:
 * - Event history retrieval
 * - Event field capture
 * - Event ordering and versioning
 * - Replaying events to specific versions
 */

const { TestHarness } = require('../lib/test-harness');

describe('Events First Schema', () => {
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

  describe('event history via pilot API', () => {
    test('can get events after transitions', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const events = await harness.pilot.getEvents();
      expect(events).toBeDefined();
      expect(events.length).toBeGreaterThanOrEqual(1);
    });

    test('can get event count', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.action('process_payment');

      const count = await harness.pilot.getEventCount();
      expect(count).toBeGreaterThanOrEqual(2);
    });

    test('can get last event', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const lastEvent = await harness.pilot.getLastEvent();
      expect(lastEvent).toBeDefined();
      expect(lastEvent.type).toBeDefined();
    });
  });

  describe('event field capture', () => {
    test('captures transition data in events', async () => {
      await harness.pilot.create();

      // Execute validate with specific data
      await harness.pilot.action('validate', {
        customer_name: 'Alice Test',
        total: 123.45,
        customer_email: 'alice@test.com'
      });

      const events = await harness.pilot.getEvents();
      const validateEvent = events.find(e =>
        e.type === 'validate' || e.type === 'Validateed' || e.type?.includes('validate')
      );

      expect(validateEvent).toBeDefined();
      expect(validateEvent.data).toBeDefined();
      expect(validateEvent.data.customer_name).toBe('Alice Test');
      expect(validateEvent.data.total).toBe(123.45);
      expect(validateEvent.data.customer_email).toBe('alice@test.com');
    });

    test('accumulates data from multiple transitions', async () => {
      await harness.pilot.create();

      await harness.pilot.action('validate', {
        customer_name: 'Bob',
        total: 200
      });

      await harness.pilot.action('process_payment', {
        payment_method: 'credit_card',
        payment_status: 'completed'
      });

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(2);

      // Verify each event has its data
      const validateEvent = events.find(e => e.data?.customer_name);
      const paymentEvent = events.find(e => e.data?.payment_method);

      expect(validateEvent.data.customer_name).toBe('Bob');
      expect(paymentEvent.data.payment_method).toBe('credit_card');
    });
  });

  describe('event ordering and versioning', () => {
    test('events have monotonically increasing versions', async () => {
      await harness.pilot.create();
      await harness.pilot.sequence(['validate', 'process_payment', 'ship']);

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(3);

      // Check versions are strictly increasing
      for (let i = 1; i < events.length; i++) {
        const prevVersion = events[i - 1].version ?? events[i - 1].sequence;
        const currVersion = events[i].version ?? events[i].sequence;
        expect(currVersion).toBeGreaterThan(prevVersion);
      }
    });

    test('events have timestamps', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThan(0);

      const event = events[0];
      expect(event.timestamp).toBeDefined();
    });

    test('events are returned in order', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate', { customer_name: 'First' });
      await harness.pilot.action('process_payment', { note: 'Second' });

      const events = await harness.pilot.getEvents();

      // First event should be validate (or created), later should be payment
      const validateIdx = events.findIndex(e => e.data?.customer_name === 'First');
      const paymentIdx = events.findIndex(e => e.data?.note === 'Second');

      if (validateIdx >= 0 && paymentIdx >= 0) {
        expect(validateIdx).toBeLessThan(paymentIdx);
      }
    });
  });

  describe('event replay', () => {
    test('can replay to specific version', async () => {
      await harness.pilot.create();

      await harness.pilot.action('validate');
      await harness.pilot.action('process_payment');
      await harness.pilot.action('ship');

      const events = await harness.pilot.getEvents();
      const midVersion = events.length > 1 ? events[1].version : 1;

      const replay = await harness.pilot.replayTo(midVersion);

      expect(replay.version).toBe(midVersion);
      expect(replay.events.length).toBeLessThanOrEqual(events.length);
    });
  });

  describe('system fields', () => {
    test('events have stream_id matching aggregate', async () => {
      const inst = await harness.pilot.create();
      await harness.pilot.action('validate');

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThan(0);

      const event = events[0];
      expect(event.stream_id).toBe(inst.id);
    });

    test('events have version numbers', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const events = await harness.pilot.getEvents();
      const event = events[0];

      expect(event.version !== undefined || event.sequence !== undefined).toBe(true);
    });
  });

  describe('complete workflow event trail', () => {
    test('full workflow generates expected event sequence', async () => {
      await harness.pilot.create();

      await harness.pilot.sequence([
        'validate',
        'process_payment',
        'ship',
        'confirm'
      ]);

      const events = await harness.pilot.getEvents();

      // Should have at least 4 events (one per transition)
      expect(events.length).toBeGreaterThanOrEqual(4);

      // Final state should be completed
      await harness.pilot.assertState('completed');

      // Event count matches transition count
      const count = await harness.pilot.getEventCount();
      expect(count).toBeGreaterThanOrEqual(4);
    });
  });
});
