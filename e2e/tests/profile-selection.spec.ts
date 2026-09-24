import { test, expect } from '@playwright/test';
import { profileButton } from './helpers/setup';

test.describe('Profile Selection', () => {
  test('redirects to /login when no user selected', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL('/login');
  });

  test('shows all seeded users', async ({ page }) => {
    await page.goto('/login');
    await expect(profileButton(page, 'Emma')).toBeVisible();
    await expect(profileButton(page, 'Lily')).toBeVisible();
    await expect(profileButton(page, 'Noah')).toBeVisible();
    await expect(profileButton(page, 'Alex')).toBeVisible();
    await expect(profileButton(page, 'Jamie')).toBeVisible();
  });

  test('selecting a child navigates to dashboard', async ({ page }) => {
    await page.goto('/login');
    await profileButton(page, 'Emma').click();
    await expect(page).toHaveURL('/');
    // Dashboard should show chore content
    await expect(page.locator('body')).not.toContainText('Who\'s here?');
  });

  test('doors show each kid\'s day; grown-ups share one door', async ({ page }) => {
    await page.goto('/login');
    // Progress comes from the same public endpoints the wall display uses.
    await expect(page.getByText(/\d+ to go|All done/).first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('heading', { name: 'The family today' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Grown-ups' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Select profile for Alex' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Select profile for Jamie' })).toBeVisible();
  });
});
