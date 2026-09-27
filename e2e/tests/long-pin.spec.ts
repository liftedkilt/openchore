import { test, expect } from '@playwright/test';
import { authHeaders, enterPin, profileButton } from './helpers/setup';

// PINs can be 4-8 digits. The PIN pad used to stop at four, so anyone with a
// longer PIN couldn't sign in from the UI (issue #99).
test.describe('Longer PINs', () => {
  test('a 6-digit PIN signs in from the PIN pad', async ({ page }) => {
    const headers = await authHeaders(1);
    const created = await page.request.post('/api/users', {
      headers,
      data: { name: 'Sixdigit', role: 'child', pin: '246813' },
    });
    expect(created.ok()).toBeTruthy();
    const user = await created.json();
    expect(user.pin_length).toBe(6);

    try {
      await page.goto('/login');
      await profileButton(page, 'Sixdigit').click();
      await expect(page.getByText('Enter your PIN')).toBeVisible();
      await enterPin(page, '246813');
      await expect(page).toHaveURL('/');
    } finally {
      await page.request.delete(`/api/users/${user.id}`, { headers });
    }
  });
});
