import { test, expect } from '@playwright/test';
import { authHeaders, localDateStr, loginAsAdmin, selectUser } from './helpers/setup';

test.describe('Parents acting for kids', () => {
  // "Pick Up Toys" is a daily core chore for Noah with no time window. Core
  // points only count once required chores are done, and nothing else in the
  // suite completes Noah's required chores, so toggling it never changes his
  // balance (rewards.spec.ts relies on Noah having no points).
  const CHORE = 'Pick Up Toys';
  const KID = 'Noah';

  test('a parent can mark a kid\'s chore done and undo it from the Kids tab', async ({ page }) => {
    await loginAsAdmin(page);

    // The card button's name starts with the person's name ("Noah …"); the
    // avatar initial is decorative (aria-hidden).
    await page.getByRole('button', { name: new RegExp(`^${KID}\\b`) }).click();
    const markDone = page.getByRole('button', { name: `Mark "${CHORE}" done for ${KID}` });
    const undo = page.getByRole('button', { name: `Undo "${CHORE}" for ${KID}` });

    // Reset to a known state if a previous run left it completed.
    if (await undo.isVisible().catch(() => false)) {
      page.once('dialog', d => d.accept());
      await undo.click();
    }

    await markDone.click();
    await expect(undo).toBeVisible({ timeout: 5_000 });

    // The completion belongs to the kid.
    const users = await (await page.request.get('/api/users')).json();
    const noah = users.find((u: { name: string }) => u.name === KID);
    const chores = await (await page.request.get(
      `/api/users/${noah.id}/chores?view=daily&date=${localDateStr()}`,
      { headers: await authHeaders(1) },
    )).json();
    const chore = chores.find((c: { title: string }) => c.title === CHORE);
    expect(chore.completed).toBeTruthy();

    page.once('dialog', d => d.accept());
    await undo.click();
    await expect(markDone).toBeVisible({ timeout: 5_000 });
  });

  test('kids cannot complete a sibling\'s chore', async ({ page }) => {
    const users = await (await page.request.get('/api/users')).json();
    const emma = users.find((u: { name: string }) => u.name === 'Emma');
    const noah = users.find((u: { name: string }) => u.name === KID);
    const chores = await (await page.request.get(
      `/api/users/${noah.id}/chores?view=daily&date=${localDateStr()}`,
    )).json();
    const chore = chores.find((c: { title: string }) => c.title === CHORE);

    const resp = await page.request.post(`/api/schedules/${chore.schedule_id}/complete`, {
      headers: await authHeaders(emma.id),
      data: { completion_date: localDateStr() },
    });
    expect(resp.status()).toBe(403);
  });
});

test.describe('Linked accounts', () => {
  test('profile menu explains when no sign-in providers are configured', async ({ page }) => {
    await selectUser(page, 'Emma');
    // Linked accounts live in the "Me" sheet, opened from the avatar.
    await page.getByRole('button', { name: 'Emma: settings' }).click();
    await page.getByRole('button', { name: 'Linked accounts' }).click();
    await expect(page.getByText('No sign-in providers are configured on this server.')).toBeVisible();
  });

  test('the API no longer trusts a bare X-User-ID header', async ({ page }) => {
    const resp = await page.request.get('/api/admin/tokens', { headers: { 'X-User-ID': '1' } });
    expect(resp.status()).toBe(401);
  });
});
