import { test, expect } from '@playwright/test';
import { selectUser } from './helpers/setup';

test.describe('Dashboard Views', () => {
  test('daily view shows time-grouped chores', async ({ page }) => {
    await selectUser(page, 'Emma');

    // Should be on Today view by default
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    // Should show progress bar
    await expect(page.getByText(/\d+ of \d+ complete|All done/i)).toBeVisible();

    // Should show at least one chore
    await expect(page.getByText('Make Bed')).toBeVisible();
  });

  test('weekly view shows 7-day calendar', async ({ page }) => {
    await selectUser(page, 'Emma');

    // Switch to Week view
    await page.getByText('Week').click();

    // Should show day abbreviations
    await expect(page.getByText('Sun').first()).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Mon').first()).toBeVisible();
    await expect(page.getByText('Sat').first()).toBeVisible();
  });

  test('rewards view shows available rewards', async ({ page }) => {
    await selectUser(page, 'Emma');

    // Switch to Rewards view
    await page.getByText(/Rewards/i).click();

    // Should show seeded rewards
    await expect(page.getByText('Extra Screen Time')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Ice Cream Trip')).toBeVisible();
  });

  test('progress updates when chore is completed', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    // Get initial progress text
    const progressBefore = await page.getByText(/\d+ of \d+ complete/i).textContent();

    // Complete a chore
    const choreCard = page.getByText('Make Bed').first()
      .locator('xpath=ancestor::div[contains(@class, "choreCard")]');
    const btn = choreCard.locator('button[aria-label="Mark complete"], button[aria-label="Mark incomplete"]');
    const label = await btn.getAttribute('aria-label');

    if (label === 'Mark complete') {
      await btn.click();
      await expect(choreCard.locator('button[aria-label="Mark incomplete"]')).toBeVisible({ timeout: 5_000 });

      // Progress should have changed
      const progressAfter = await page.getByText(/\d+ of \d+ complete|All done/i).textContent();
      expect(progressAfter).not.toBe(progressBefore);
    }
  });

  test('undo completion restores chore state', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    // Find a completed chore (Make Bed might be completed from previous test)
    const choreCard = page.getByText('Make Bed').first()
      .locator('xpath=ancestor::div[contains(@class, "choreCard")]');
    const btn = choreCard.locator('button[aria-label="Mark complete"], button[aria-label="Mark incomplete"]');
    const label = await btn.getAttribute('aria-label');

    if (label === 'Mark incomplete') {
      // Undo it
      await btn.click();
      await expect(choreCard.locator('button[aria-label="Mark complete"]')).toBeVisible({ timeout: 5_000 });
    }
  });
});
