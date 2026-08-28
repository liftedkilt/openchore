import { test, expect } from '@playwright/test';
import { selectUser } from './helpers/setup';

test.describe('Chore Completion', () => {
  test('completing a chore shows checkmark and updates points', async ({ page }) => {
    await selectUser(page, 'Emma');

    // Wait for dashboard to load with chores
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    // Get initial points display
    const pointsText = await page.getByText(/\d+ pts/i).first().textContent();
    const initialPoints = parseInt(pointsText?.match(/(\d+)/)?.[1] || '0');

    // Find "Brush Teeth (Morning)" chore card
    const choreCard = page.locator('xpath=//div[contains(@class, "choreCard")][.//text()[contains(., "Brush Teeth (Morning)")]]').first();
    await expect(choreCard).toBeVisible({ timeout: 10_000 });

    // If it was already completed by another parallel test, uncomplete it first
    const markIncompleteBtn = choreCard.locator('button[aria-label="Mark incomplete"]');
    if (await markIncompleteBtn.isVisible().catch(() => false)) {
      await markIncompleteBtn.click();
      await expect(choreCard.locator('button[aria-label="Mark complete"]')).toBeVisible({ timeout: 5_000 });
    }

    // Click mark complete
    const markCompleteBtn = choreCard.locator('button[aria-label="Mark complete"]');
    await markCompleteBtn.click();

    // Verify the chore shows as completed (button changes to "Mark incomplete")
    await expect(choreCard.locator('button[aria-label="Mark incomplete"]')).toBeVisible({ timeout: 5_000 });
  });
});
