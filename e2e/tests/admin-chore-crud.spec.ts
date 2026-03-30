import { test, expect } from '@playwright/test';
import { loginAsAdmin, apiGet } from './helpers/setup';

test.describe('Admin Chore CRUD', () => {
  test('admin can edit a chore via API', async ({ page }) => {
    // Update Make Bed points via API
    const resp = await page.request.put('/api/chores/3', {
      headers: { 'X-User-ID': '1' },
      data: { title: 'Make Bed', category: 'required', points_value: 10, requires_approval: false, requires_photo: false },
    });
    expect(resp.ok()).toBeTruthy();
    const updated = await resp.json();
    expect(updated.points_value).toBe(10);

    // Verify in admin UI
    await loginAsAdmin(page);
    await expect(page.getByText('10 pts').first()).toBeVisible({ timeout: 5_000 });
  });

  test('admin can delete a chore via API', async ({ page }) => {
    // Create a chore to delete
    const createResp = await page.request.post('/api/chores', {
      headers: { 'X-User-ID': '1' },
      data: { title: 'E2E Delete Chore', category: 'bonus', points_value: 5 },
    });
    const chore = await createResp.json();

    // Delete it
    const deleteResp = await page.request.delete(`/api/chores/${chore.id}`, {
      headers: { 'X-User-ID': '1' },
    });
    expect(deleteResp.ok()).toBeTruthy();

    // Verify gone
    const chores = await apiGet(page, '/chores');
    const found = chores.find((c: any) => c.title === 'E2E Delete Chore');
    expect(found).toBeFalsy();
  });

  test('admin can create chore with interval schedule', async ({ page }) => {
    await loginAsAdmin(page);

    await page.getByRole('button', { name: /Add Chore/i }).click();
    await page.getByPlaceholder('e.g. Empty the dishwasher').fill('E2E Interval Chore');
    await page.getByRole('button', { name: /Next/i }).click();

    // Select interval schedule type
    await page.getByRole('button', { name: 'Emma' }).click();
    const schedTypeSelect = page.locator('select').filter({ has: page.getByText('Weekly') });
    await schedTypeSelect.selectOption('interval');

    // Set interval to 3 days
    await page.locator('input[type="number"][min="1"]').fill('3');

    await page.getByRole('button', { name: /Next/i }).click();
    await page.getByRole('button', { name: /Create Chore/i }).click();

    await expect(page.getByText('New Chore')).not.toBeVisible({ timeout: 10_000 });

    // Verify via API
    const chores = await apiGet(page, '/chores');
    const created = chores.find((c: any) => c.title === 'E2E Interval Chore');
    expect(created).toBeTruthy();

    const schedules = await apiGet(page, `/chores/${created.id}/schedules`);
    expect(schedules).toHaveLength(1);
    expect(schedules[0].recurrence_interval).toBe(3);
  });

  test('chore list shows all seeded chores', async ({ page }) => {
    await loginAsAdmin(page);

    const expectedChores = ['Feed Cats (Morning)', 'Feed Cats (Evening)', 'Make Bed',
      'Brush Teeth (Morning)', 'Brush Teeth (Evening)', 'Empty Dishwasher',
      'Clean Room', 'Read 20 Minutes'];

    for (const chore of expectedChores) {
      await expect(page.getByText(chore)).toBeVisible();
    }
  });
});
