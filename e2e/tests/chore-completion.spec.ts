import { test, expect } from '@playwright/test';
import { selectUser } from './helpers/setup';

test.describe('Chore Completion', () => {
  // "Practice Piano" is chosen deliberately: it is a daily chore for Emma with
  // no `available_at` / `due_by` window in config.example.yaml, so it is never
  // locked or expired regardless of what time the suite runs. It is also the
  // only chore in this spec, so it does not race with dashboard-views.spec.ts
  // (which toggles "Make Bed" for the same user against the shared database).
  // Do not swap this for a time-windowed chore such as "Brush Teeth (Morning)"
  // — that one is locked before 07:00 local and the complete button is
  // replaced by a countdown, which fails the test on any early-morning run.
  const CHORE = 'Practice Piano';

  test('completing a chore toggles it to the completed state', async ({ page }) => {
    await selectUser(page, 'Emma');

    // Wait for dashboard to load with chores
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    const choreCard = page
      .locator(`xpath=//div[contains(@class, "choreCard")][.//text()[contains(., "${CHORE}")]]`)
      .first();
    await expect(choreCard).toBeVisible({ timeout: 10_000 });

    // The suite shares one database, so reset to a known state first.
    const markIncompleteBtn = choreCard.locator('button[aria-label="Mark incomplete"]');
    if (await markIncompleteBtn.isVisible().catch(() => false)) {
      await markIncompleteBtn.click();
      await expect(choreCard.locator('button[aria-label="Mark complete"]')).toBeVisible({ timeout: 5_000 });
    }

    // Click mark complete
    await choreCard.locator('button[aria-label="Mark complete"]').click();

    // Verify the chore shows as completed (button changes to "Mark incomplete")
    await expect(choreCard.locator('button[aria-label="Mark incomplete"]')).toBeVisible({ timeout: 5_000 });

    // Undo, so re-runs against a reused database start from the same state.
    await choreCard.locator('button[aria-label="Mark incomplete"]').click();
    await expect(choreCard.locator('button[aria-label="Mark complete"]')).toBeVisible({ timeout: 5_000 });
  });
});
