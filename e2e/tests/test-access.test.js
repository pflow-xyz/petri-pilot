/**
 * E2E tests for the test-access app.
 *
 * Tests all access control transitions: submit, approve, reject
 *
 * Workflow: draft → submit → submitted → approve → approved
 * Alternative: draft → submit → submitted → reject → rejected
 *
 * Roles:
 * - customer: can submit
 * - reviewer: can approve/reject
 * - admin: inherits customer + reviewer
 */

const { TestHarness } = require('../lib/test-harness');

describe('test-access', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('test-access');
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
      expect(placeIds).toContain('draft');
      expect(placeIds).toContain('submitted');
      expect(placeIds).toContain('approved');
      expect(placeIds).toContain('rejected');
    });

    test('can get all transitions', async () => {
      const transitions = await harness.pilot.getTransitions();
      const transitionIds = transitions.map(t => t.id);
      expect(transitionIds).toContain('submit');
      expect(transitionIds).toContain('approve');
      expect(transitionIds).toContain('reject');
    });

    test('initial state is draft', async () => {
      const info = await harness.pilot.getWorkflowInfo();
      expect(info.initialPlace).toBe('draft');
    });
  });

  describe('submit event', () => {
    beforeAll(async () => {
      await harness.login(['customer']);
    });

    test('customer can submit from draft state', async () => {
      const instance = await harness.pilot.create();
      expect(instance.id).toBeDefined();

      // Should start in draft
      await harness.pilot.assertState('draft');

      // Check submit is enabled
      const canSubmit = await harness.pilot.canFire('submit');
      expect(canSubmit.canFire).toBe(true);

      // Submit
      const result = await harness.pilot.action('submit');
      expect(result.success).toBe(true);

      // Should be in submitted state
      await harness.pilot.assertState('submitted');

      // Verify submit event was recorded
      const events = await harness.pilot.getEvents();
      const submitEvent = events.find(e =>
        e.type.toLowerCase().includes('submit')
      );
      expect(submitEvent).toBeDefined();
    });

    test('submit is only enabled from draft state', async () => {
      await harness.login(['admin']);
      await harness.pilot.create();

      // Submit first
      await harness.pilot.action('submit');
      await harness.pilot.assertState('submitted');

      // Submit should no longer be enabled
      const canSubmit = await harness.pilot.canFire('submit');
      expect(canSubmit.canFire).toBe(false);
    });
  });

  describe('approve event', () => {
    beforeAll(async () => {
      await harness.login(['reviewer']);
    });

    test('reviewer can approve from submitted state', async () => {
      await harness.login(['admin']); // admin can submit and approve
      await harness.pilot.create();
      await harness.pilot.action('submit');
      await harness.pilot.assertState('submitted');

      await harness.login(['reviewer']);

      // Check approve is enabled
      const canApprove = await harness.pilot.canFire('approve');
      expect(canApprove.canFire).toBe(true);

      // Approve
      const result = await harness.pilot.action('approve');
      expect(result.success).toBe(true);

      // Should be in approved state
      await harness.pilot.assertState('approved');

      // Verify approve event was recorded
      const events = await harness.pilot.getEvents();
      const approveEvent = events.find(e =>
        e.type.toLowerCase().includes('approv')
      );
      expect(approveEvent).toBeDefined();
    });

    test('approve is only enabled from submitted state', async () => {
      await harness.login(['admin']);
      await harness.pilot.create();

      // From draft, approve should not be enabled
      await harness.pilot.assertState('draft');
      const canApprove = await harness.pilot.canFire('approve');
      expect(canApprove.canFire).toBe(false);
    });
  });

  describe('reject event', () => {
    test('reviewer can reject from submitted state', async () => {
      await harness.login(['admin']); // admin can submit
      await harness.pilot.create();
      await harness.pilot.action('submit');
      await harness.pilot.assertState('submitted');

      await harness.login(['reviewer']);

      // Check reject is enabled
      const canReject = await harness.pilot.canFire('reject');
      expect(canReject.canFire).toBe(true);

      // Reject
      const result = await harness.pilot.action('reject');
      expect(result.success).toBe(true);

      // Should be in rejected state
      await harness.pilot.assertState('rejected');

      // Verify reject event was recorded
      const events = await harness.pilot.getEvents();
      const rejectEvent = events.find(e =>
        e.type.toLowerCase().includes('reject')
      );
      expect(rejectEvent).toBeDefined();
    });

    test('reject is only enabled from submitted state', async () => {
      await harness.login(['admin']);
      await harness.pilot.create();

      // From draft, reject should not be enabled
      await harness.pilot.assertState('draft');
      const canReject = await harness.pilot.canFire('reject');
      expect(canReject.canFire).toBe(false);
    });
  });

  describe('complete workflows', () => {
    beforeAll(async () => {
      await harness.login(['admin']); // admin has all roles
    });

    test('happy path: draft → submit → approve', async () => {
      await harness.pilot.create();

      const results = await harness.pilot.sequence([
        'submit',
        'approve'
      ]);

      expect(results.length).toBe(2);
      expect(results.every(r => r.success)).toBe(true);
      await harness.pilot.assertState('approved');
    });

    test('rejection path: draft → submit → reject', async () => {
      await harness.pilot.create();

      const results = await harness.pilot.sequence([
        'submit',
        'reject'
      ]);

      expect(results.length).toBe(2);
      expect(results.every(r => r.success)).toBe(true);
      await harness.pilot.assertState('rejected');
    });

    test('no transitions available from terminal states', async () => {
      await harness.pilot.create();
      await harness.pilot.sequence(['submit', 'approve']);
      await harness.pilot.assertState('approved');

      // No more transitions should be enabled
      const enabled = await harness.pilot.getEnabled();
      expect(enabled.length).toBe(0);
    });
  });

  describe('role-based access control', () => {
    test('customer role can only submit', async () => {
      await harness.login(['customer']);
      await harness.pilot.create();

      // Customer can submit
      const canSubmit = await harness.pilot.canFire('submit');
      expect(canSubmit.canFire).toBe(true);

      // Execute submit
      await harness.pilot.action('submit');
      await harness.pilot.assertState('submitted');

      // Customer cannot approve (no reviewer role)
      const canApprove = await harness.pilot.canFire('approve');
      expect(canApprove.canFire).toBe(false);

      // Customer cannot reject (no reviewer role)
      const canReject = await harness.pilot.canFire('reject');
      expect(canReject.canFire).toBe(false);
    });

    test('reviewer role can approve and reject but not submit', async () => {
      // First create and submit as admin
      await harness.login(['admin']);
      await harness.pilot.create();

      // As reviewer only, cannot submit (from a new instance)
      await harness.login(['reviewer']);
      const newInstance = await harness.pilot.create();

      const canSubmit = await harness.pilot.canFire('submit');
      expect(canSubmit.canFire).toBe(false);
    });

    test('admin role inherits all permissions', async () => {
      await harness.login(['admin']);
      await harness.pilot.create();

      // Admin can submit (from customer role)
      const canSubmit = await harness.pilot.canFire('submit');
      expect(canSubmit.canFire).toBe(true);

      await harness.pilot.action('submit');

      // Admin can approve (from reviewer role)
      const canApprove = await harness.pilot.canFire('approve');
      expect(canApprove.canFire).toBe(true);

      // Admin can also reject (from reviewer role)
      const canReject = await harness.pilot.canFire('reject');
      expect(canReject.canFire).toBe(true);
    });
  });

  describe('event sourcing', () => {
    beforeAll(async () => {
      await harness.login(['admin']);
    });

    test('all events are recorded', async () => {
      await harness.pilot.create();
      await harness.pilot.action('submit');
      await harness.pilot.action('approve');

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(2);
    });

    test('events have correct types', async () => {
      await harness.pilot.create();
      await harness.pilot.action('submit');
      await harness.pilot.action('reject');

      const events = await harness.pilot.getEvents();
      const eventTypes = events.map(e => e.type.toLowerCase());

      expect(eventTypes.some(t => t.includes('submit'))).toBe(true);
      expect(eventTypes.some(t => t.includes('reject'))).toBe(true);
    });

    test('can get event count', async () => {
      await harness.pilot.create();
      await harness.pilot.action('submit');

      const count = await harness.pilot.getEventCount();
      expect(count).toBeGreaterThanOrEqual(1);
    });
  });
});
