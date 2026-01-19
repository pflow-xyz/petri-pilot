/**
 * E2E tests for concurrent access patterns and event ordering.
 *
 * Uses window.pilot API to verify:
 * - Concurrent transitions are handled safely with optimistic locking
 * - Event sequence numbers are monotonically increasing
 * - Only one of multiple concurrent transitions succeeds when racing
 */

const { TestHarness } = require('../lib/test-harness');

describe('Concurrent Access', () => {
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

  describe('optimistic locking', () => {
    test('handles concurrent transitions safely', async () => {
      // Create an instance in received state
      const inst = await harness.pilot.create();

      // Two concurrent validate transitions - only one should succeed
      const [result1, result2] = await Promise.all([
        harness.pilot.action('validate').then(r => ({ success: true, ...r })).catch(e => ({ success: false, error: e.message })),
        harness.pilot.action('validate').then(r => ({ success: true, ...r })).catch(e => ({ success: false, error: e.message }))
      ]);

      // Count successes
      const successes = [result1.success, result2.success].filter(Boolean);

      // Exactly one should succeed due to optimistic locking
      expect(successes.length).toBe(1);

      // Verify final state
      await harness.pilot.refresh();
      await harness.pilot.assertState('validated');
    });

    test('prevents double transitions through locking', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      // Try to process payment twice concurrently
      const [result1, result2] = await Promise.all([
        harness.pilot.action('process_payment').then(r => ({ success: true })).catch(e => ({ success: false })),
        harness.pilot.action('process_payment').then(r => ({ success: true })).catch(e => ({ success: false }))
      ]);

      const successes = [result1.success, result2.success].filter(Boolean);
      expect(successes.length).toBe(1);

      // Should be in paid state
      await harness.pilot.refresh();
      await harness.pilot.assertState('paid');
    });

    test('handles race between validate and reject', async () => {
      await harness.pilot.create();

      // Race between validate and reject
      const [result1, result2] = await Promise.all([
        harness.pilot.action('validate').then(r => ({ success: true, action: 'validate' })).catch(e => ({ success: false, action: 'validate' })),
        harness.pilot.action('reject').then(r => ({ success: true, action: 'reject' })).catch(e => ({ success: false, action: 'reject' }))
      ]);

      const successes = [result1, result2].filter(r => r.success);
      expect(successes.length).toBe(1);

      // Should be in either validated or rejected state, but not received
      await harness.pilot.refresh();
      const status = await harness.pilot.getStatus();
      expect(['validated', 'rejected']).toContain(status);
    });
  });

  describe('event ordering', () => {
    test('maintains event ordering under load', async () => {
      // Create multiple instances
      const instances = [];
      for (let i = 0; i < 3; i++) {
        const inst = await harness.pilot.create();
        instances.push(inst.id);
        await harness.pilot.list(); // Navigate away to create fresh instance
      }

      // Execute validate on all instances
      for (const id of instances) {
        await harness.pilot.view(id);
        await harness.pilot.action('validate');
      }

      // Verify each instance has proper event sequence
      for (const id of instances) {
        await harness.pilot.view(id);
        const events = await harness.pilot.getEvents();

        expect(events.length).toBeGreaterThanOrEqual(1);

        // Verify sequence numbers are monotonically increasing
        for (let i = 1; i < events.length; i++) {
          const prev = events[i - 1].version ?? events[i - 1].sequence;
          const curr = events[i].version ?? events[i].sequence;
          expect(curr).toBeGreaterThan(prev);
        }
      }
    });

    test('maintains sequence integrity with rapid transitions', async () => {
      await harness.pilot.create();
      await harness.pilot.action('validate');

      const eventsBefore = await harness.pilot.getEvents();
      const initialCount = eventsBefore.length;

      // Try multiple process_payment attempts (only first should succeed)
      const attempts = 5;
      const promises = Array(attempts).fill().map(() =>
        harness.pilot.action('process_payment').catch(() => null)
      );
      await Promise.all(promises);

      // Verify events after
      const eventsAfter = await harness.pilot.getEvents();

      // Should have exactly one more event
      expect(eventsAfter.length).toBe(initialCount + 1);

      // All sequences should be monotonically increasing
      for (let i = 1; i < eventsAfter.length; i++) {
        const prev = eventsAfter[i - 1].version ?? eventsAfter[i - 1].sequence;
        const curr = eventsAfter[i].version ?? eventsAfter[i].sequence;
        expect(curr).toBeGreaterThan(prev);
      }
    });
  });

  describe('state consistency', () => {
    test('maintains consistent state across concurrent reads and writes', async () => {
      await harness.pilot.create();

      // Concurrent validate and state reads
      const [validateResult, ...stateResults] = await Promise.all([
        harness.pilot.action('validate').then(() => ({ success: true })).catch(e => ({ success: false })),
        harness.pilot.getState(),
        harness.pilot.getState(),
        harness.pilot.getState()
      ]);

      // Validate should succeed
      expect(validateResult.success).toBe(true);

      // All state reads should be consistent (either all received or all validated)
      for (const state of stateResults) {
        if (state) {
          const totalTokens = Object.values(state).reduce((sum, v) => sum + (v || 0), 0);
          // Should have exactly 1 token total (single-token workflow)
          expect(totalTokens).toBe(1);
        }
      }
    });

    test('concurrent creates result in separate instances', async () => {
      // Create multiple instances concurrently
      const creates = await Promise.all([
        harness.pilot.create(),
        harness.pilot.create(),
        harness.pilot.create()
      ]);

      // All should succeed with unique IDs
      const ids = creates.map(c => c.id);
      const uniqueIds = new Set(ids);
      expect(uniqueIds.size).toBe(3);
    });
  });

  describe('workflow isolation', () => {
    test('transitions on one instance dont affect others', async () => {
      // Create two instances
      const inst1 = await harness.pilot.create();
      await harness.pilot.list();
      const inst2 = await harness.pilot.create();

      // Validate first instance
      await harness.pilot.view(inst1.id);
      await harness.pilot.action('validate');
      await harness.pilot.assertState('validated');

      // Second instance should still be in received
      await harness.pilot.view(inst2.id);
      await harness.pilot.assertState('received');
    });

    test('event history is isolated per instance', async () => {
      const inst1 = await harness.pilot.create();
      await harness.pilot.action('validate');
      await harness.pilot.action('process_payment');
      const events1 = await harness.pilot.getEvents();

      await harness.pilot.list();
      const inst2 = await harness.pilot.create();
      const events2 = await harness.pilot.getEvents();

      // inst1 should have more events than inst2
      expect(events1.length).toBeGreaterThan(events2.length);

      // Events should reference correct stream IDs
      expect(events1[0].stream_id).toBe(inst1.id);
      if (events2.length > 0) {
        expect(events2[0].stream_id).toBe(inst2.id);
      }
    });
  });
});
