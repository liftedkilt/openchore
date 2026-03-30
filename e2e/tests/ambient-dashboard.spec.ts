import { test, expect } from '@playwright/test';

test.describe('Ambient Dashboard', () => {
  test('ambient view loads from login page', async ({ page }) => {
    await page.goto('/login');
    await page.getByText('Wall Display').click();
    await expect(page).toHaveURL('/ambient');
  });

  test('shows all children with progress', async ({ page }) => {
    await page.goto('/ambient');

    // Should show kid names (may take time to load)
    await expect(page.getByText('Emma').first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Lily').first()).toBeVisible();
    await expect(page.getByText('Noah').first()).toBeVisible();
  });

  test('shows current time', async ({ page }) => {
    await page.goto('/ambient');

    // Should show a time display (HH:MM format)
    await expect(page.getByText(/\d{1,2}:\d{2}/)).toBeVisible({ timeout: 5_000 });
  });

  test('clicking anywhere navigates to login', async ({ page }) => {
    await page.goto('/ambient');
    await expect(page.getByText('Emma').first()).toBeVisible({ timeout: 10_000 });

    // Click on the page body area
    await page.locator('body').click({ position: { x: 100, y: 100 } });
    await expect(page).toHaveURL('/login', { timeout: 5_000 });
  });
});
