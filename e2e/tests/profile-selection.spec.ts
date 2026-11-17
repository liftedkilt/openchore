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

    test('signed in as someone else on a shared device switches to them', async ({ page }) => {
      await page.goto('/u/emma');
      await expect(page).toHaveURL('/');
      await page.goto('/u/lily');
      await expect(page).toHaveURL('/');
      const me = await page.request.get('/api/auth/me');
      expect((await me.json()).user.name).toBe('Lily');
    });

    test('switching to a PIN-protected profile still asks for the PIN', async ({ page }) => {
      await page.goto('/u/emma');
      await expect(page).toHaveURL('/');
      await page.goto('/u/alex');
      await expect(page).toHaveURL('/u/alex');
      await enterPin(page, '1234');
      await expect(page).toHaveURL('/');
      const me = await page.request.get('/api/auth/me');
      expect((await me.json()).user.name).toBe('Alex');
    });

    test('on a personal device it shows the picker instead of switching', async ({ page }) => {
      await page.goto('/u/emma');
      await expect(page).toHaveURL('/');
      // Only linked-account (OIDC) sessions are persistent; the suite has no
      // IdP, so mark Emma's session as one in what the app is told.
      await page.route('**/api/auth/me', async route => {
        const res = await route.fetch();
        const body = await res.json();
        body.session = { ...body.session, persistent: true };
        await route.fulfill({ response: res, json: body });
      });
      await page.goto('/u/lily');
      // The picker paints before the link is handled; let any sign-in settle.
      await page.waitForLoadState('networkidle');
      await expect(page).toHaveURL('/u/lily');
      await expect(profileButton(page, 'Lily')).toBeVisible();
      await page.unroute('**/api/auth/me');
      const me = await page.request.get('/api/auth/me');
      expect((await me.json()).user.name).toBe('Emma');
    });
  });
});
