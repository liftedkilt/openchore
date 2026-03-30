import { test, expect } from '@playwright/test';
import { loginAsAdmin, apiGet } from './helpers/setup';

test.describe('Chore Creation', () => {
  test.describe.serial(() => {
    test('create chore with Every Day schedule assigns all 7 days', async ({ page }) => {
      await loginAsAdmin(page);

      // Open create wizard
      await page.getByRole('button', { name: /Add Chore/i }).click();
      await expect(page.getByText('New Chore')).toBeVisible();

      // Step 1: Fill in chore details
      await page.getByPlaceholder('e.g. Empty the dishwasher').fill('E2E Test Chore Daily');
      await page.getByRole('button', { name: /Next/i }).click();

      // Step 2: Schedule - assign to first child, pick Every Day
      await page.getByRole('button', { name: 'Emma' }).click();
      await page.getByRole('button', { name: 'Every day' }).click();
      await page.getByRole('button', { name: /Next/i }).click();

      // Step 3: Review and create
      await expect(page.getByText('E2E Test Chore Daily')).toBeVisible();
      await page.getByRole('button', { name: /Create Chore/i }).click();

      // Wait for wizard to close
      await expect(page.getByText('New Chore')).not.toBeVisible({ timeout: 10_000 });

      // Verify via API that all 7 schedules were created
      const chores = await apiGet(page, '/chores') as Array<{ id: number; title: string }>;
      const created = chores.find(c => c.title === 'E2E Test Chore Daily');
      expect(created).toBeTruthy();

      const schedules = await apiGet(page, `/chores/${created!.id}/schedules`) as Array<{ day_of_week: number }>;
      const days = schedules.map(s => s.day_of_week).sort();
      expect(days).toEqual([0, 1, 2, 3, 4, 5, 6]);
    });

    test('photo_source persists through create and update', async ({ page }) => {
      await loginAsAdmin(page);

      // Create a chore with photo required + external source via the wizard
      await page.getByRole('button', { name: /Add Chore/i }).click();
      await page.getByPlaceholder('e.g. Empty the dishwasher').fill('E2E Photo Source Test');

      // Enable photo proof
      await page.getByText('Requires photo proof').locator('..').locator('input[type="checkbox"]').check();

      // Select "External with manual fallback"
      await page.locator('select').filter({ has: page.getByText('Child uploads photo') }).selectOption('both');

      // Skip schedule
      await page.getByRole('button', { name: /Next/i }).click();
      await page.getByRole('button', { name: /Skip/i }).click();

      // Create
      await page.getByRole('button', { name: /Create Chore/i }).click();
      await expect(page.getByText('New Chore')).not.toBeVisible({ timeout: 10_000 });

      // Verify photo_source was saved via API
      const chores = await apiGet(page, '/chores') as Array<{ id: number; title: string; photo_source: string; requires_photo: boolean }>;
      const created = chores.find(c => c.title === 'E2E Photo Source Test');
      expect(created).toBeTruthy();
      expect(created!.requires_photo).toBe(true);
      expect(created!.photo_source).toBe('both');

      // Update the chore via API (simulates what the edit modal does) and verify photo_source persists
      const resp = await page.request.put(`/api/chores/${created!.id}`, {
        headers: { 'X-User-ID': '1' },
        data: {
          title: 'E2E Photo Source Test Updated',
          description: '',
          category: 'core',
          points_value: 5,
          requires_approval: false,
          requires_photo: true,
          photo_source: 'both',
        },
      });
      expect(resp.ok()).toBeTruthy();
      const updated = await resp.json() as { photo_source: string };
      expect(updated.photo_source).toBe('both');
    });
  });
});
