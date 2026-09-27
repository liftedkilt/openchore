import { test, expect } from '@playwright/test';
import { authHeaders, loginAsAdmin, openAdminTab } from './helpers/setup';

// Runs in the project after the main suite: the start page is a
// household-wide setting other specs rely on. Signs in as Jamie because the
// PIN-change spec in the same project changes Alex's PIN.
test.describe('Sign-in options', () => {
  test.afterEach(async ({ page }) => {
    await page.request.put('/api/admin/auth/options', {
      headers: await authHeaders(2),
      data: { start_page: 'picker', pin_sign_in: true },
    });
  });

  test('the start page can send signed-out visitors to the wall display', async ({ page, browser }) => {
    await loginAsAdmin(page, '5678', 'Jamie');
    await openAdminTab(page, 'Settings');
    const start = page.getByLabel('When someone opens OpenChore signed out');
    await expect(start).toHaveValue('picker');
    await start.selectOption('wall');
    const card = page.locator('form').filter({ has: start });
    await card.getByRole('button', { name: 'Save' }).click();
    await expect(card.getByText('Saved.')).toBeVisible();

    const visitor = await browser.newPage();
    try {
      await visitor.goto('/');
      await expect(visitor).toHaveURL('/ambient');
      // The family picker is still one address away for a shared tablet.
      await visitor.goto('/login');
      await expect(visitor.getByRole('button', { name: 'Select profile for Emma', exact: true })).toBeVisible();
    } finally {
      await visitor.close();
    }
  });
});
