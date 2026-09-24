import { test, expect } from '@playwright/test';
import { authHeaders, loginAsAdmin } from './helpers/setup';

// Runs in its own project after the main suite: it changes a seeded parent's
// PIN, which other specs rely on.
test.describe('Parent PIN change', () => {
  test('can change a parent PIN and sign in with it', async ({ page }) => {
    const headers = await authHeaders(1);
    const changeResp = await page.request.put('/api/users/1/pin', {
      headers,
      data: { current_pin: '1234', new_pin: '4680' },
    });
    expect(changeResp.ok()).toBeTruthy();

    // The old PIN no longer works.
    const oldResp = await page.request.post('/api/auth/login', { data: { user_id: 1, pin: '1234' } });
    expect(oldResp.status()).toBe(401);

    // The new PIN works via the UI.
    await loginAsAdmin(page, '4680');
    await expect(page).toHaveURL('/admin/dashboard');

    // Restore the original PIN.
    const restoreResp = await page.request.put('/api/users/1/pin', {
      headers,
      data: { current_pin: '4680', new_pin: '1234' },
    });
    expect(restoreResp.ok()).toBeTruthy();
  });
});
