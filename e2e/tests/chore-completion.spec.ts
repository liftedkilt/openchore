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

    // Find "Make Bed" — a daily required chore assigned to Natalie
    const makeBed = page.getByText('Make Bed').first();
    await expect(makeBed).toBeVisible();

    // Click the complete button (the circle/checkmark next to the chore)
    const choreCard = makeBed.locator('xpath=ancestor::div[contains(@class, "choreCard")]');
    const completeBtn = choreCard.locator('button[aria-label="Mark complete"]');
    await completeBtn.click();

    // Verify the chore shows as completed (button changes to "Mark incomplete")
    await expect(choreCard.locator('button[aria-label="Mark incomplete"]')).toBeVisible({ timeout: 5_000 });
  });
});
