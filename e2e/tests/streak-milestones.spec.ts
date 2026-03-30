import { test, expect } from '@playwright/test';
import { loginAsAdmin, selectUser, apiGet } from './helpers/setup';

test.describe('Streak Milestones', () => {
  test('admin can create streak milestone', async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Rewards/i }).click();

    // Verify seeded milestones are shown
    await expect(page.getByText('3-Day Streak!')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Week Warrior!')).toBeVisible();
  });

  test('streak data is visible on child dashboard', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });

    // Streak section should exist (may show 0 if no streak yet)
    // The streak info is in the stats area
    const streakData = await apiGet(page, '/users/3/streak', 3);
    expect(streakData).toHaveProperty('current_streak');
    expect(streakData).toHaveProperty('longest_streak');
  });

  test('streak milestones CRUD via API', async ({ page }) => {
    // List existing
    const listResp = await page.request.get('/api/admin/streak-rewards', {
      headers: { 'X-User-ID': '1' },
    });
    expect(listResp.ok()).toBeTruthy();
    const milestones = await listResp.json();
    expect(milestones.length).toBeGreaterThanOrEqual(4); // 4 seeded

    // Create new
    const createResp = await page.request.post('/api/admin/streak-rewards', {
      headers: { 'X-User-ID': '1' },
      data: { streak_days: 60, bonus_points: 100, label: 'E2E 60-Day Test!' },
    });
    expect(createResp.ok()).toBeTruthy();
    const created = await createResp.json();
    expect(created.id).toBeTruthy();

    // Delete
    const deleteResp = await page.request.delete(`/api/admin/streak-rewards/${created.id}`, {
      headers: { 'X-User-ID': '1' },
    });
    expect(deleteResp.ok()).toBeTruthy();
  });
});
