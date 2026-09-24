import { test, expect } from '@playwright/test';
import { enterPin, loginAsAdmin } from './helpers/setup';

test.describe('Parent sign-in', () => {
  test('parent PIN opens their dashboard with a Manage button', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('button', { name: 'Select profile for Alex' }).click();
    await expect(page.getByText('Enter your PIN')).toBeVisible();
    await enterPin(page, '1234');

    await expect(page).toHaveURL('/');
    await page.getByRole('button', { name: 'Manage' }).click();
    await expect(page).toHaveURL('/admin/dashboard');
  });

  test('wrong PIN shows an error', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('button', { name: 'Select profile for Jamie' }).click();
    await enterPin(page, '0000');
    await expect(page.getByText('Incorrect PIN')).toBeVisible();
    await expect(page).toHaveURL('/login');
  });

  test('kids have no Manage button and cannot open admin pages', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('button', { name: 'Select profile for Emma' }).click();
    await expect(page).toHaveURL('/');
    await expect(page.getByRole('button', { name: 'Manage' })).toHaveCount(0);

    await page.goto('/admin/dashboard');
    await expect(page).toHaveURL('/');
  });

  test('admin pages require signing in', async ({ page }) => {
    await page.goto('/admin/dashboard');
    await expect(page).toHaveURL('/login');
  });

  test('the old household passcode screen is gone', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByRole('button', { name: /^Manage$/ })).toHaveCount(0);
  });

  test('admin dashboard loads chores tab', async ({ page }) => {
    await loginAsAdmin(page);
    // Kids is the default tab; switch to Chores to verify seeded data renders.
    await page.getByRole('button', { name: /^Chores$/i }).click();
    await expect(page.getByText('Make Bed')).toBeVisible();
  });
});
