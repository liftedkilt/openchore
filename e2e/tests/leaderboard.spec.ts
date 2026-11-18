import { test, expect } from '@playwright/test';
import { authHeaders, loginAsAdmin, selectUser } from './helpers/setup';

// The weekly leaderboard is off by default. These turn it on, check what a
// parent and a kid see, and turn it off again so other specs see the usual
// Today screen.
test.describe('Leaderboard', () => {
  test.afterAll(async ({ request }) => {
    await request.put('/api/admin/friends', { headers: await authHeaders(1), data: { leaderboard_enabled: false } });
  });

  test('is hidden from kids until a parent turns it on', async ({ page }) => {
    await page.request.put('/api/admin/friends', { headers: await authHeaders(1), data: { leaderboard_enabled: false } });
    await selectUser(page, 'Emma');
    await expect(page.getByRole('button', { name: /Add a task/ })).toBeVisible();
    await expect(page.getByRole('button', { name: 'See everyone' })).toHaveCount(0);
  });

  test('a parent turns it on in Settings and names the family', async ({ page }) => {
    await loginAsAdmin(page);
    await page.locator('nav button').last().click();
    const section = page.locator('section', { has: page.getByRole('heading', { name: 'Leaderboard & friend families' }) });
    await expect(section).toBeVisible({ timeout: 5_000 });

    const toggle = section.getByLabel('Show the weekly leaderboard');
    await toggle.click(); // saved to the server first, then shown as ticked
    await expect(toggle).toBeChecked();

    // Linking needs a family name first.
    await expect(section.getByText('Give your family a name before linking.')).toBeVisible();
    await section.getByPlaceholder('e.g. The Smiths').fill('The Testers');
    await section.getByRole('button', { name: 'Save' }).first().click();
    await expect(section.getByText('Give your family a name before linking.')).toHaveCount(0);

    const cfg = await (await page.request.get('/api/admin/friends', { headers: await authHeaders(1) })).json();
    expect(cfg.leaderboard_enabled).toBe(true);
    expect(cfg.family_name).toBe('The Testers');
    expect(cfg.links).toEqual([]);
  });

  test('a kid sees the card on Today and the full leaderboard', async ({ page }) => {
    await page.request.put('/api/admin/friends', { headers: await authHeaders(1), data: { leaderboard_enabled: true } });
    await selectUser(page, 'Emma');

    await page.getByRole('button', { name: 'See everyone' }).click();
    const sheet = page.getByRole('dialog', { name: 'Leaderboard' });
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole('tab', { name: 'This week' })).toHaveAttribute('aria-selected', 'true');
    // Everyone in the family is listed, and Emma's own row is marked.
    for (const name of ['Emma', 'Lily', 'Noah']) {
      await expect(sheet.getByRole('listitem').filter({ hasText: name })).toBeVisible();
    }
    await expect(sheet.getByText('You', { exact: true })).toBeVisible();
    await expect(sheet.getByText(/Your score is out of 100/)).toBeVisible();

    await sheet.getByRole('tab', { name: 'Last week' }).click();
    await expect(sheet.getByRole('tab', { name: 'Last week' })).toHaveAttribute('aria-selected', 'true');
  });

  test('the leaderboard needs a signed-in person', async ({ request }) => {
    expect((await request.get('/api/leaderboard')).status()).toBe(401);
    expect((await request.get('/api/friends/farms')).status()).toBe(401);
  });
});
