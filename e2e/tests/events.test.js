/**
 * E2E tests for Events First Schema - event field validation and binding behavior.
 *
 * Tests the order-processing app to validate:
 * - Required event field validation
 * - Optional event field capture
 * - Auto-populated system fields
 */

const { TestHarness } = require('../lib/test-harness');

describe('Events First Schema', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('order-processing');
    await harness.setup();
    // Login with all roles needed for transitions
    await harness.login(['admin', 'fulfillment', 'system', 'customer']);
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('Event field validation', () => {
    test('should validate required event fields', async () => {
      // Create a new instance
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();

      // Attempt transition without required binding (customer_name is required)
      // The validate transition requires customer_name and total
      await expect(
        harness.executeTransition('validate', instance.aggregate_id, {
          // missing required 'customer_name'
          total: 100
        })
      ).rejects.toThrow();
    });

    test('should capture all event fields including optional', async () => {
      // Create a new instance
      const instance = await harness.createInstance();

      // Execute validate transition with all fields including optional ones
      await harness.executeTransition('validate', instance.aggregate_id, {
        customer_name: 'Alice',
        total: 100,
        customer_email: 'alice@example.com', // optional field
        shipping_address: '123 Main St', // optional field
        order_id: 'ORDER-001'
      });

      // Get event history
      const events = await harness.getEventHistory(instance.aggregate_id);
      
      expect(events).toBeDefined();
      expect(events.length).toBeGreaterThan(0);
      
      // Find the order_validated event
      const validatedEvent = events.find(e => e.event_type === 'order_validated');
      expect(validatedEvent).toBeDefined();
      expect(validatedEvent.data.customer_name).toBe('Alice');
      expect(validatedEvent.data.total).toBe(100);
      expect(validatedEvent.data.customer_email).toBe('alice@example.com');
      expect(validatedEvent.data.shipping_address).toBe('123 Main St');
    });

    test('should auto-populate system fields', async () => {
      // Create a new instance
      const instance = await harness.createInstance();

      // Execute transition with minimal required fields
      await harness.executeTransition('validate', instance.aggregate_id, {
        customer_name: 'Bob',
        total: 50,
        order_id: 'ORDER-002'
      });

      // Get event history
      const events = await harness.getEventHistory(instance.aggregate_id);
      
      expect(events).toBeDefined();
      expect(events.length).toBeGreaterThan(0);
      
      // Check the first event has system fields
      const event = events[0];
      expect(event.aggregate_id).toBeDefined();
      expect(event.aggregate_id).toBe(instance.aggregate_id);
      
      // Events should have version number
      expect(event.version).toBeDefined();
      expect(typeof event.version).toBe('number');
      
      // Events should have timestamp
      expect(event.timestamp).toBeDefined();
    });
  });

  describe('Event sourcing behavior', () => {
    test('should maintain event order in history', async () => {
      // Create a new instance
      const instance = await harness.createInstance();

      // Execute multiple transitions
      await harness.executeTransition('validate', instance.aggregate_id, {
        customer_name: 'Charlie',
        total: 150,
        order_id: 'ORDER-003'
      });

      await harness.executeTransition('process_payment', instance.aggregate_id, {
        order_id: 'ORDER-003',
        total: 150,
        payment_method: 'credit_card'
      });

      // Get event history
      const events = await harness.getEventHistory(instance.aggregate_id);
      
      expect(events.length).toBeGreaterThanOrEqual(2);
      
      // Events should be in order by version
      for (let i = 1; i < events.length; i++) {
        expect(events[i].version).toBeGreaterThan(events[i - 1].version);
      }
    });

    test('should allow querying events from specific version', async () => {
      // Create a new instance
      const instance = await harness.createInstance();

      // Execute transitions to create multiple events
      await harness.executeTransition('validate', instance.aggregate_id, {
        customer_name: 'Diana',
        total: 200,
        order_id: 'ORDER-004'
      });

      await harness.executeTransition('process_payment', instance.aggregate_id, {
        order_id: 'ORDER-004',
        total: 200,
        payment_method: 'debit_card'
      });

      // Get all events
      const allEvents = await harness.getEventHistory(instance.aggregate_id, 0);
      
      // Get events from version 2 onwards
      const laterEvents = await harness.getEventHistory(instance.aggregate_id, 2);
      
      expect(laterEvents.length).toBeLessThanOrEqual(allEvents.length);
      if (laterEvents.length > 0) {
        expect(laterEvents[0].version).toBeGreaterThanOrEqual(2);
      }
    });
  });
});
