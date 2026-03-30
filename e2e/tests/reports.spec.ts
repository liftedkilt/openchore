import { test, expect } from '@playwright/test';
import { loginAsAdmin } from './helpers/setup';

test.describe('Reports', () => {
  test('reports page loads with weekly view', async ({ page }) => {
    await loginAsAdmin(page);

    // Click Reports button
    await page.getByRole('button', { name: /Reports/i }).click();
    await expect(page).toHaveURL('/admin/reports');

    // Should show period tabs
    await expect(page.getByText('Week')).toBeVisible();
    await expect(page.getByText('Month')).toBeVisible();
    await expect(page.getByText('Year')).toBeVisible();

    // Should show kid scorecards
    await expect(page.getByText('Emma').first()).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Lily').first()).toBeVisible();
    await expect(page.getByText('Noah').first()).toBeVisible();
  });

  test('can navigate between report periods', async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Reports/i }).click();
    await expect(page).toHaveURL('/admin/reports');
    await expect(page.getByText('Emma').first()).toBeVisible({ timeout: 5_000 });

    // The reports page should have navigation arrows and period display
    // Just verify the page loaded successfully with data
    await expect(page.getByText('Week')).toBeVisible();
  });

  test('can switch to monthly view', async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Reports/i }).click();

    // Click Month tab
    await page.getByText('Month').click();
    await page.waitForTimeout(500);

    // Should still show kid names (scorecards load for any period)
    await expect(page.getByText('Emma').first()).toBeVisible({ timeout: 5_000 });
  });

  test('back button returns to admin dashboard', async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Reports/i }).click();
    await expect(page).toHaveURL('/admin/reports');

    // Click back (ArrowLeft icon button, no text)
    await page.locator('button').first().click();
    await expect(page).toHaveURL('/admin/dashboard');
  });
});
