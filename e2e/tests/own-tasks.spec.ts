import { test, expect, type Page } from '@playwright/test';
import { authHeaders, selectUser } from './helpers/setup';

/** A fresh kid, so nothing here races other specs. */
async function freshKid(page: Page, label: string) {
  const stamp = `${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
  const admin = await authHeaders(1);
  return (await page.request.post('/api/users', {
    headers: admin, data: { name: `${label} ${stamp}`, role: 'child', age: 10 },
  })).json();
}

function localDay(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() + offset);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

test.describe('Own tasks', () => {
  test('a kid adds a task for today and one three weeks out', async ({ page }) => {
    const kid = await freshKid(page, 'Planner');
    await selectUser(page, kid.name);

    // Today: add a one-day task.
    await page.getByRole('button', { name: 'Add a task' }).click();
    const sheet = page.getByRole('dialog', { name: 'New task' });
    await expect(sheet).toBeVisible();
    await expect(sheet).toContainText('no points');
    await sheet.getByLabel('What do you want to do?').fill('Practise piano');
    await sheet.getByRole('button', { name: 'Add task' }).click();

    // Back on the list, then close: it shows under Optional today.
    await expect(page.getByRole('dialog', { name: 'My own tasks' })).toContainText('Practise piano');
    await page.getByRole('button', { name: 'Close' }).click();
    const lists = page.getByRole('region', { name: /chores/i }).or(page.locator('main'));
    await expect(lists.getByText('Optional', { exact: false }).first()).toBeVisible();
    await expect(lists.getByText('Practise piano')).toBeVisible();

    // Three weeks out, picked on the calendar.
    const later = localDay(21);
    await page.getByRole('button', { name: 'My own tasks' }).click();
    await page.getByRole('dialog', { name: 'My own tasks' }).getByRole('button', { name: 'Add a task' }).click();
    const form = page.getByRole('dialog', { name: 'New task' });
    await form.getByLabel('What do you want to do?').fill('Pack for camp');
    await form.getByLabel('Which day?').fill(later);
    await form.getByRole('button', { name: 'Add task' }).click();
    const list = page.getByRole('dialog', { name: 'My own tasks' });
    await expect(list).toContainText('Pack for camp');

    // The server has it on that day, worth nothing, and not today.
    const headers = await authHeaders(kid.id);
    const onDay = await (await page.request.get(`/api/users/${kid.id}/chores?view=daily&date=${later}`, { headers })).json();
    expect(onDay.map((c: { title: string }) => c.title)).toEqual(['Pack for camp']);
    expect(onDay[0].category).toBe('optional');
    expect(onDay[0].points_value).toBe(0);

    // Remove it again.
    await list.getByRole('button', { name: 'Change Pack for camp' }).click();
    const edit = page.getByRole('dialog', { name: 'Change task' });
    await edit.getByRole('button', { name: 'Remove this task' }).click();
    await edit.getByRole('button', { name: 'Yes, remove' }).click();
    await expect(page.getByRole('dialog', { name: 'My own tasks' })).not.toContainText('Pack for camp');
  });

  test('a weekly task repeats on the chosen days', async ({ page }) => {
    const kid = await freshKid(page, 'Swimmer');
    await selectUser(page, kid.name);
    await page.getByRole('button', { name: 'Add a task' }).click();
    const form = page.getByRole('dialog', { name: 'New task' });
    await form.getByLabel('What do you want to do?').fill('Swim practice');
    await form.getByRole('radio', { name: 'Every week' }).click();
    await form.getByRole('button', { name: 'Every day' }).click();
    await form.getByRole('button', { name: 'Add task' }).click();
    await expect(page.getByRole('dialog', { name: 'My own tasks' })).toContainText('Every day');

    const headers = await authHeaders(kid.id);
    for (const offset of [0, 3, 10]) {
      const chores = await (await page.request.get(`/api/users/${kid.id}/chores?view=daily&date=${localDay(offset)}`, { headers })).json();
      expect(chores.map((c: { title: string }) => c.title)).toContain('Swim practice');
    }
  });
});
