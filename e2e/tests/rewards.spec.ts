import { test, expect } from '@playwright/test';
import { loginAsAdmin, selectUser, apiGet } from './helpers/setup';

test.describe('Rewards', () => {
  test('admin can create a reward via API', async ({ page }) => {
    const createResp = await page.request.post('/api/rewards', {
      headers: { 'X-User-ID': '1' },
      data: { name: 'E2E Test Reward', icon: '🎯', cost: 25 },
    });
    expect(createResp.ok()).toBeTruthy();
    const reward = await createResp.json();
    expect(reward.name).toBe('E2E Test Reward');

    // Verify in admin UI
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Rewards/i }).click();
    await expect(page.getByText('E2E Test Reward')).toBeVisible({ timeout: 5_000 });
  });

  test('child can see and redeem a reward', async ({ page }) => {
    // Give Emma points
    await page.request.post('/api/points/adjust', {
      headers: { 'X-User-ID': '1' },
      data: { user_id: 3, amount: 200, note: 'E2E test points' },
    });

    await selectUser(page, 'Emma');
    await page.getByText(/Rewards/i).click();
    await expect(page.getByText('Extra Screen Time')).toBeVisible({ timeout: 5_000 });

    // Click first redeem button
    const redeemBtn = page.getByRole('button', { name: /Redeem/i }).first();
    await redeemBtn.click();

    // Verify redemption succeeded (look for the Redeemed! button specifically)
    await expect(page.getByRole('button', { name: 'Redeemed!' })).toBeVisible({ timeout: 5_000 });
  });

  test('child cannot redeem reward they cannot afford', async ({ page }) => {
    // Noah has no points
    await selectUser(page, 'Noah');
    await page.getByText(/Rewards/i).click();
    await expect(page.getByText('Extra Screen Time')).toBeVisible({ timeout: 5_000 });

    // All redeem buttons should show "Need X more" (disabled)
    const disabledBtns = page.locator('button[disabled]').filter({ hasText: /Need|pts/i });
    await expect(disabledBtns.first()).toBeVisible({ timeout: 5_000 });
  });

  test('admin can delete a reward via API', async ({ page }) => {
    // Create then delete
    const createResp = await page.request.post('/api/rewards', {
      headers: { 'X-User-ID': '1' },
      data: { name: 'E2E Delete Me Reward', icon: '🗑️', cost: 10 },
    });
    const reward = await createResp.json();

    const deleteResp = await page.request.delete(`/api/rewards/${reward.id}`, {
      headers: { 'X-User-ID': '1' },
    });
    expect(deleteResp.ok()).toBeTruthy();

    // Verify gone
    const rewards = await page.request.get('/api/rewards/all', {
      headers: { 'X-User-ID': '1' },
    });
    const list = await rewards.json();
    const found = list.find((r: any) => r.name === 'E2E Delete Me Reward');
    expect(found).toBeFalsy();
  });
});
