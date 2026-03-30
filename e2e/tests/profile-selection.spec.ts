import { test, expect } from '@playwright/test';

test.describe('Profile Selection', () => {
  test('redirects to /login when no user selected', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL('/login');
  });

  test('shows all seeded users', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByText('Emma')).toBeVisible();
    await expect(page.getByText('Lily')).toBeVisible();
    await expect(page.getByText('Noah')).toBeVisible();
    await expect(page.getByText('Alex')).toBeVisible();
    await expect(page.getByText('Jamie')).toBeVisible();
  });

  test('selecting a child navigates to dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.getByText('Emma').click();
    await expect(page).toHaveURL('/');
    // Dashboard should show chore content
    await expect(page.locator('body')).not.toContainText('Who\'s doing chores today?');
  });
});
