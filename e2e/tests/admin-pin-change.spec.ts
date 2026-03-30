import { test, expect } from '@playwright/test';

test.describe('Admin PIN Change', () => {
  test('can change admin PIN and verify', async ({ page }) => {
    // Change PIN via API
    const changeResp = await page.request.put('/api/admin/passcode', {
      headers: { 'X-User-ID': '1' },
      data: { old_passcode: '1234', new_passcode: '5678' },
    });
    expect(changeResp.ok()).toBeTruthy();

    // Verify new PIN works
    const verifyResp = await page.request.post('/api/admin/verify', {
      data: { passcode: '5678' },
    });
    expect(verifyResp.ok()).toBeTruthy();

    // Verify new PIN works via UI
    await page.goto('/admin');
    for (const digit of ['5', '6', '7', '8']) {
      await page.getByRole('button', { name: digit, exact: true }).click();
    }
    await expect(page).toHaveURL('/admin/dashboard');

    // Restore original PIN
    const restoreResp = await page.request.put('/api/admin/passcode', {
      headers: { 'X-User-ID': '1' },
      data: { old_passcode: '5678', new_passcode: '1234' },
    });
    expect(restoreResp.ok()).toBeTruthy();
  });
});
