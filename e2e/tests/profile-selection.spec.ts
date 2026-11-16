import { test, expect } from '@playwright/test';
import { enterPin, profileButton } from './helpers/setup';

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

  test.describe('/u/:slug links', () => {
    test('a kid with no PIN goes straight to their list', async ({ page }) => {
      await page.goto('/u/emma');
      await expect(page).toHaveURL('/');
      await expect(page.locator('body')).not.toContainText("Who's here?");
    });

    test('the slug ignores case', async ({ page }) => {
      await page.goto('/u/EMMA');
      await expect(page).toHaveURL('/');
    });

    test('a PIN-protected profile asks for the PIN first', async ({ page }) => {
      await page.goto('/u/alex');
      await expect(page).toHaveURL('/u/alex');
      await enterPin(page, '1234');
      await expect(page).toHaveURL('/');
    });

    test('an unknown name shows the picker', async ({ page }) => {
      await page.goto('/u/nobody');
      await expect(profileButton(page, 'Emma')).toBeVisible();
    });

    test('signed in as someone else shows the picker instead of switching', async ({ page }) => {
      await page.goto('/u/emma');
      await expect(page).toHaveURL('/');
      await page.goto('/u/lily');
      await expect(profileButton(page, 'Lily')).toBeVisible();
    });
  });
});
