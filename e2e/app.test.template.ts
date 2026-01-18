// E2E Tests Template for {{APP_NAME}}
// Uses Playwright for browser automation

import { test, expect } from '@playwright/test';

const BASE_URL = process.env.APP_URL || 'http://localhost:8080';

test.describe('{{APP_NAME}}', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(BASE_URL);
  });

  test('displays navigation menu', async ({ page }) => {
    // Copilot: Update selectors based on generated UI
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.getByRole('link', { name: /dashboard/i })).toBeVisible();
  });

  test('creates new instance', async ({ page }) => {
    // Click create button
    await page.getByRole('button', { name: /create|new/i }).click();

    // Verify instance created
    await expect(page.locator('[data-testid="aggregate-id"]')).toBeVisible();
  });

  test('executes workflow transitions', async ({ page }) => {
    // Create new instance
    await page.getByRole('button', { name: /create|new/i }).click();

    // Copilot: Generate steps for each transition in the happy path
    // {{#WORKFLOW_STEPS}}
    // Step: {{TRANSITION_ID}}
    await page.getByRole('button', { name: /{{TRANSITION_LABEL}}/i }).click();
    await expect(page.locator('[data-state="{{NEXT_STATE}}"]')).toBeVisible();
    // {{/WORKFLOW_STEPS}}
  });

  test('shows enabled transitions only', async ({ page }) => {
    await page.getByRole('button', { name: /create|new/i }).click();

    // Initial state should show first transition enabled
    await expect(page.getByRole('button', { name: /{{FIRST_TRANSITION}}/i })).toBeEnabled();

    // Later transitions should be disabled or hidden
    // Copilot: Add assertions for disabled transitions
  });

  test.describe('with authentication', () => {
    test.use({
      storageState: 'e2e/.auth/user.json' // Copilot: Set up auth state
    });

    test('shows user-specific navigation items', async ({ page }) => {
      await page.goto(BASE_URL);

      // Copilot: Check role-based nav items
      await expect(page.getByRole('link', { name: /admin/i })).toBeVisible();
    });
  });

  test.describe('admin dashboard', () => {
    test('lists all instances', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin`);

      await expect(page.getByRole('heading', { name: /instances/i })).toBeVisible();
      await expect(page.locator('table')).toBeVisible();
    });

    test('shows instance detail', async ({ page }) => {
      // Create an instance first
      await page.goto(BASE_URL);
      await page.getByRole('button', { name: /create|new/i }).click();

      // Get the ID and navigate to admin detail
      const id = await page.locator('[data-testid="aggregate-id"]').textContent();
      await page.goto(`${BASE_URL}/admin/instances/${id}`);

      await expect(page.getByText(/event history/i)).toBeVisible();
    });
  });

  test.describe('event history', () => {
    test('displays event timeline', async ({ page }) => {
      // Create instance and execute some transitions
      await page.goto(BASE_URL);
      await page.getByRole('button', { name: /create|new/i }).click();
      await page.getByRole('button', { name: /{{FIRST_TRANSITION}}/i }).click();

      // Navigate to events
      await page.getByRole('link', { name: /history|events/i }).click();

      // Verify events shown
      await expect(page.locator('[data-testid="event-timeline"]')).toBeVisible();
      await expect(page.getByText(/{{FIRST_TRANSITION}}/i)).toBeVisible();
    });
  });
});
