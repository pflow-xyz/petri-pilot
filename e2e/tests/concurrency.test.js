/**
 * E2E tests for concurrent access patterns and event ordering.
 * 
 * Tests verify:
 * - Concurrent transitions are handled safely with optimistic locking
 * - Event sequence numbers are monotonically increasing
 * - Only one of multiple concurrent transitions succeeds when racing
 * 
 * Note: These tests use the 'order-processing' app because it has a consistent
 * naming convention (filename matches package name). The test-access app has
 * a mismatch between its filename (test-access.json) and package name (access-test).
 */

const { TestHarness } = require('../lib/test-harness');

describe('Concurrent Access', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('order-processing');
    await harness.setup();
    // Login with all necessary roles
    await harness.login(['admin', 'fulfillment', 'system', 'customer']);
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('Optimistic Locking', () => {
    it('should handle concurrent transitions safely', async () => {
      // Create an instance in received state
      const instance = await harness.createInstance({});
      const id = instance.aggregate_id;

      // Two concurrent validate transitions (only one should succeed since we only have 1 token in received)
      const [result1, result2] = await Promise.all([
        harness.fireTransition('validate', {}, id),
        harness.fireTransition('validate', {}, id)
      ]);

      // Count successes
      const successes = [result1.success, result2.success].filter(Boolean);
      
      // Exactly one should succeed due to optimistic locking
      expect(successes.length).toBe(1);

      // Verify final state has moved from received to validated
      const state = await harness.getState(id);
      expect(state.validated).toBe(1);
      expect(state.received).toBe(0);
    });

    it('should prevent double transitions through optimistic locking', async () => {
      // Create instance and validate it
      const instance = await harness.createInstance({});
      const id = instance.aggregate_id;
      
      await harness.fireTransition('validate', {}, id);

      // Try to process payment twice concurrently
      const [result1, result2] = await Promise.all([
        harness.fireTransition('process_payment', {}, id),
        harness.fireTransition('process_payment', {}, id)
      ]);

      // Only one should succeed
      const successes = [result1.success, result2.success].filter(Boolean);
      expect(successes.length).toBe(1);

      // Verify we're in paid state
      const state = await harness.getState(id);
      expect(state.paid).toBe(1);
      expect(state.validated).toBe(0);
    });

    it('should handle race between validate and reject', async () => {
      // Create instance
      const instance = await harness.createInstance({});
      const id = instance.aggregate_id;

      // Race between validate and reject (only one should win)
      const [result1, result2] = await Promise.all([
        harness.fireTransition('validate', {}, id),
        harness.fireTransition('reject', {}, id)
      ]);

      // Exactly one should succeed
      const successes = [result1.success, result2.success].filter(Boolean);
      expect(successes.length).toBe(1);

      // Verify we're in either validated or rejected state, but not both
      const state = await harness.getState(id);
      const inFinalState = (state.validated === 1 && state.rejected === 0) || 
                           (state.validated === 0 && state.rejected === 1);
      expect(inFinalState).toBe(true);
      expect(state.received).toBe(0);
    });
  });

  describe('Event Ordering', () => {
    it('should maintain event ordering under load', async () => {
      // Create multiple instances and execute transitions rapidly
      const instances = [];
      
      // Create 5 instances
      for (let i = 0; i < 5; i++) {
        const instance = await harness.createInstance({});
        instances.push(instance.aggregate_id);
      }

      // Execute transitions on all instances concurrently
      const promises = instances.map(id => 
        harness.fireTransition('validate', {}, id)
      );
      await Promise.all(promises);

      // Verify each instance has proper event sequence
      for (const id of instances) {
        const events = await harness.getEventHistory(id);
        
        // Should have at least the validate event (version should be >= 1 after transition)
        // Note: version starts at 0, but after one transition it should be >= 1
        expect(events.length).toBeGreaterThanOrEqual(1);
        
        // Verify sequence numbers are monotonically increasing
        for (let i = 1; i < events.length; i++) {
          expect(events[i].sequence).toBeGreaterThan(events[i-1].sequence);
        }
      }
    });

    it('should maintain sequence integrity with rapid transitions', async () => {
      // Create an instance
      const instance = await harness.createInstance({});
      const id = instance.aggregate_id;

      // Validate it
      await harness.fireTransition('validate', {}, id);
      
      // Get initial event count
      const eventsBefore = await harness.getEventHistory(id);
      const initialCount = eventsBefore.length;

      // Try multiple process_payment attempts (only first should succeed)
      const attempts = 10;
      const promises = Array(attempts).fill().map(() =>
        harness.fireTransition('process_payment', {}, id)
      );
      await Promise.all(promises);

      // Verify events after
      const eventsAfter = await harness.getEventHistory(id);
      
      // Should have exactly one more event (the successful payment processing)
      expect(eventsAfter.length).toBe(initialCount + 1);
      
      // All sequences should be monotonically increasing
      for (let i = 1; i < eventsAfter.length; i++) {
        expect(eventsAfter[i].sequence).toBeGreaterThan(eventsAfter[i-1].sequence);
      }
    });
  });

  describe('State Consistency', () => {
    it('should maintain consistent state across concurrent reads and writes', async () => {
      const instance = await harness.createInstance({});
      const id = instance.aggregate_id;

      // Concurrent validate and state reads
      const promises = [
        harness.fireTransition('validate', {}, id),
        harness.getState(id),
        harness.getState(id),
        harness.getState(id)
      ];

      const results = await Promise.all(promises);
      const [validateResult, ...stateResults] = results;

      // Validate should succeed
      expect(validateResult.success).toBe(true);

      // All state reads should be consistent (either all received or all validated)
      // They might see different states due to timing, but each should be valid
      for (const state of stateResults) {
        const totalTokens = (state.received || 0) + (state.validated || 0) + 
                           (state.rejected || 0) + (state.paid || 0) +
                           (state.shipped || 0) + (state.completed || 0);
        // Should have exactly 1 token total
        expect(totalTokens).toBe(1);
      }
    });
  });
});
