import { test, expect, type Page } from '@playwright/test';
import { selectUser, authHeaders } from './helpers/setup';

/**
 * Each test gets its own kid with windowless chores scheduled for today, so
 * nothing here depends on the time of day (time-windowed chores lock before
 * they open) or races other specs toggling the seeded kids' chores. Bonus
 * chores wait for every Must do and Every day chore, so the seeded
 * windowless chores (all Bonus for Emma) can't be used for a plain toggle.
 */
async function seedKid(
  page: Page, label: string,
  chores: { title: string; category: 'required' | 'core' | 'bonus'; requires_photo?: boolean }[],
) {
  const stamp = `${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
  const admin = await authHeaders(1);
  const kid = await (await page.request.post('/api/users', {
    headers: admin, data: { name: `${label} ${stamp}`, role: 'child', age: 10 },
  })).json();
  const titles: string[] = [];
  for (const c of chores) {
    const title = `${c.title} ${stamp}`;
    const chore = await (await page.request.post('/api/chores', {
      headers: admin, data: { title, category: c.category, points_value: 5, requires_photo: !!c.requires_photo },
    })).json();
    const resp = await page.request.post(`/api/chores/${chore.id}/schedules`, {
      headers: admin, data: { assigned_to: kid.id, day_of_week: new Date().getDay() },
    });
    expect(resp.ok()).toBeTruthy();
    titles.push(title);
  }
  return { kid, titles };
}

const cardFor = (page: Page, title: string) => page
  .locator(`xpath=//div[contains(@class, "choreCard")][.//text()[contains(., "${title}")]]`)
  .first();

test.describe('Chore Completion', () => {
  test('completing a chore toggles it to the completed state', async ({ page }) => {
    const { kid, titles: [CHORE] } = await seedKid(page, 'Toggler', [{ title: 'Tidy Desk', category: 'core' }]);
    await selectUser(page, kid.name);

    // Wait for dashboard to load with chores
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    const choreCard = cardFor(page, CHORE);
    await expect(choreCard).toBeVisible({ timeout: 10_000 });

    // Click mark complete
    await choreCard.locator('button[aria-label="Mark complete"]').click();

    // Verify the chore shows as completed (button changes to "Mark incomplete")
    await expect(choreCard.locator('button[aria-label="Mark incomplete"]')).toBeVisible({ timeout: 5_000 });

    // Undo, so the state round-trips.
    await choreCard.locator('button[aria-label="Mark incomplete"]').click();
    await expect(choreCard.locator('button[aria-label="Mark complete"]')).toBeVisible({ timeout: 5_000 });
  });

  test('a finished chore celebrates, and the celebration gets out of the way', async ({ page }) => {
    const { kid, titles: [CHORE] } = await seedKid(page, 'Celebrator', [{ title: 'Water Plants', category: 'required' }]);
    await selectUser(page, kid.name);
    const choreCard = cardFor(page, CHORE);
    await choreCard.locator('button[aria-label="Mark complete"]').click();

    const celebration = page.getByRole('heading', { name: 'Nailed it!' });
    await expect(celebration).toBeVisible({ timeout: 5_000 });
    await page.getByRole('button', { name: 'Back to my list' }).click();
    await expect(celebration).toBeHidden();
    await expect(choreCard.locator('button[aria-label="Mark incomplete"]')).toBeVisible();
  });

  test('a photo chore can be finished without a photo, and waits for a grown-up', async ({ page }) => {
    const { kid, titles: [CHORE] } = await seedKid(page, 'NoCamera', [{ title: 'Feed the Fish', category: 'core', requires_photo: true }]);
    await selectUser(page, kid.name);
    const choreCard = cardFor(page, CHORE);
    await choreCard.locator('button[aria-label="Mark complete"]').click();

    // No photo yet: the photo sheet opens (the server answered "photo required").
    const sheet = page.getByRole('dialog', { name: 'Snap a photo' });
    await expect(sheet).toBeVisible({ timeout: 5_000 });
    await sheet.getByRole('button', { name: 'No photo? Finish anyway' }).click();
    await expect(sheet).toBeHidden({ timeout: 5_000 });

    // It went to the approval queue without a photo.
    await expect.poll(async () => {
      const pending = await (await page.request.get('/api/completions/pending', { headers: await authHeaders(1) })).json();
      return pending.find((p: { chore_title: string }) => p.chore_title === CHORE)?.photo_url;
    }, { timeout: 5_000 }).toBe('');
  });

  test('bonus chores open once every Must do and Every day chore is done', async ({ page }) => {
    const { kid, titles: [MUST, DAILY, BONUS] } = await seedKid(page, 'Gatekeeper', [
      { title: 'Sort Socks', category: 'required' },
      { title: 'Stack Chairs', category: 'core' },
      { title: 'Build a Fort', category: 'bonus' },
    ]);
    await selectUser(page, kid.name);

    const bonus = cardFor(page, BONUS);
    await expect(bonus).toContainText('Opens when everything else is done', { timeout: 10_000 });
    await expect(bonus.getByRole('button', { name: 'Locked' })).toBeDisabled();

    for (const title of [MUST, DAILY]) {
      await cardFor(page, title).locator('button[aria-label="Mark complete"]').click();
      await expect(cardFor(page, title).locator('button[aria-label="Mark incomplete"]')).toBeVisible({ timeout: 5_000 });
      await page.keyboard.press('Escape'); // step past the celebration
    }

    await expect(bonus.locator('button[aria-label="Mark complete"]')).toBeEnabled({ timeout: 5_000 });
    await expect(bonus).not.toContainText('Opens when everything else is done');
  });
});
