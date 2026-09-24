import { test, expect, type Page } from '@playwright/test';
import { selectUser, authHeaders } from './helpers/setup';

/** Open the "Me" sheet from the avatar at the top of a person's screen. */
async function openMe(page: Page, name: string) {
  await page.getByRole('button', { name: `${name}: settings` }).click();
  await expect(page.getByRole('dialog', { name: 'Me' })).toBeVisible();
}

test.describe('Theme and Profile', () => {
  test('child can switch themes', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });
    const skinRoot = page.locator('[data-theme][data-person]').first();
    await expect(skinRoot).toHaveAttribute('data-theme', 'sunroom');

    await openMe(page, 'Emma');
    await page.getByRole('radio', { name: 'Tint' }).click();
    await expect(page.getByRole('radio', { name: 'Tint' })).toHaveAttribute('aria-checked', 'true');
    // The whole screen re-skins at once.
    await expect(skinRoot).toHaveAttribute('data-theme', 'tint');

    // Verify theme is persisted via API
    await expect.poll(async () => {
      const resp = await page.request.get('/api/users/3', { headers: await authHeaders(3) });
      return (await resp.json()).theme;
    }).toBe('tint');

    // Put Emma back in her own skin for the rest of the suite.
    await page.getByRole('radio', { name: 'Sunroom' }).click();
    await expect(skinRoot).toHaveAttribute('data-theme', 'sunroom');
    await expect.poll(async () => {
      const resp = await page.request.get('/api/users/3', { headers: await authHeaders(3) });
      return (await resp.json()).theme;
    }).toBe('sunroom');
  });

  test('child can pick their colour', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });
    await openMe(page, 'Emma');

    await page.getByRole('radio', { name: 'Lilac' }).click();
    await expect(page.locator('[data-theme][data-person]').first()).toHaveAttribute('data-person', 'lilac');
    await expect.poll(async () => {
      const resp = await page.request.get('/api/users/3', { headers: await authHeaders(3) });
      return (await resp.json()).color;
    }).toBe('lilac');

    // Restore the seeded colour.
    await page.getByRole('radio', { name: 'Coral' }).click();
    await expect(page.locator('[data-theme][data-person]').first()).toHaveAttribute('data-person', 'coral');
  });

  test('TTS toggle persists preference', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    // Emma is 11, so TTS defaults to off.
    await openMe(page, 'Emma');
    const toggle = page.getByRole('switch', { name: /read chores aloud/i });
    const before = await toggle.getAttribute('aria-checked');
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-checked', before === 'true' ? 'false' : 'true');

    // Verify localStorage was set
    const ttsValue = await page.evaluate((userId) => localStorage.getItem(`openchore_tts_${userId}`), 3);
    expect(ttsValue).toBe(before === 'true' ? '0' : '1');

    // Put it back.
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-checked', before ?? 'false');
  });

  test('logout returns to profile selection', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    await openMe(page, 'Emma');
    await page.getByRole('button', { name: /sign out/i }).click();
    await expect(page).toHaveURL('/login');
  });
});
