/**
 * E2E tests for the blog-post app.
 *
 * Workflow: draft → submit → in_review → approve → published → unpublish → archived → restore → draft
 * Alternative: in_review → reject → draft
 */

const { TestHarness } = require('../lib/test-harness');

describe('blog-post', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('blog-post');
    await harness.setup();
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('workflow transitions', () => {
    beforeAll(async () => {
      // Login with all roles needed for the workflow
      await harness.login(['admin', 'author', 'editor']);
    });

    test('create instance starts in draft state', async () => {
      const instance = await harness.createInstance();
      expect(instance.aggregate_id).toBeDefined();
      expect(instance).toHaveTokenIn('draft');
    });

    test('submit transition moves to in_review state', async () => {
      const instance = await harness.createInstance();

      const result = await harness.executeTransition('submit', instance.aggregate_id);
      expect(result.success).toBe(true);
      expect(result).toHaveTokenIn('in_review');
    });

    test('complete publish workflow', async () => {
      // Create instance (starts in draft)
      const instance = await harness.createInstance();
      expect(instance).toHaveTokenIn('draft');

      // Submit for review
      let result = await harness.executeTransition('submit', instance.aggregate_id);
      expect(result).toHaveTokenIn('in_review');

      // Approve
      result = await harness.executeTransition('approve', instance.aggregate_id);
      expect(result).toHaveTokenIn('published');
    });

    test('reject workflow returns to draft', async () => {
      const instance = await harness.createInstance();

      // Submit
      let result = await harness.executeTransition('submit', instance.aggregate_id);
      expect(result).toHaveTokenIn('in_review');

      // Reject
      result = await harness.executeTransition('reject', instance.aggregate_id);
      expect(result).toHaveTokenIn('draft');
    });

    test('complete archive and restore cycle', async () => {
      const instance = await harness.createInstance();

      // Publish
      await harness.executeTransition('submit', instance.aggregate_id);
      await harness.executeTransition('approve', instance.aggregate_id);

      // Unpublish
      let result = await harness.executeTransition('unpublish', instance.aggregate_id);
      expect(result).toHaveTokenIn('archived');

      // Restore
      result = await harness.executeTransition('restore', instance.aggregate_id);
      expect(result).toHaveTokenIn('draft');
    });
  });

  describe('debug eval', () => {
    test('can check document title', async () => {
      const title = await harness.eval('return document.title');
      expect(typeof title).toBe('string');
    });

    test('can access api client', async () => {
      const hasApi = await harness.eval('return window.api !== undefined');
      expect(hasApi).toBe(true);
    });
  });
});
