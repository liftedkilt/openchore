import { test, expect } from '@playwright/test';

test.describe('Admin Login', () => {
  test('correct passcode navigates to admin dashboard', async ({ page }) => {
    await page.goto('/admin');
    await expect(page.getByText('Parent Access')).toBeVisible();

    // Enter PIN: 1234
    for (const digit of ['1', '2', '3', '4']) {
      await page.getByRole('button', { name: digit, exact: true }).click();
    }

    await expect(page).toHaveURL('/admin/dashboard');
  });

  test('wrong passcode shows error and clears', async ({ page }) => {
    await page.goto('/admin');

    // Enter wrong PIN: 0000
    for (const digit of ['0', '0', '0', '0']) {
      await page.getByRole('button', { name: digit, exact: true }).click();
    }

    await expect(page.getByText('Incorrect passcode')).toBeVisible();
  });

  test('admin dashboard loads chores tab', async ({ page }) => {
    await page.goto('/admin');
    for (const digit of ['1', '2', '3', '4']) {
      await page.getByRole('button', { name: digit, exact: true }).click();
    }
    await expect(page).toHaveURL('/admin/dashboard');
    // Kids is the default tab; switch to Chores to verify seeded data renders.
    await page.getByRole('button', { name: /^Chores$/i }).click();
    await expect(page.getByText('Make Bed')).toBeVisible();
  });
});
