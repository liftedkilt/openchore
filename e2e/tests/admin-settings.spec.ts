import { test, expect } from '@playwright/test';
import { loginAsAdmin } from './helpers/setup';

test.describe('Admin Settings', () => {
  test('can update system base URL via API', async ({ page }) => {
    const resp = await page.request.put('/api/admin/settings/base_url', {
      headers: { 'X-User-ID': '1' },
      data: { value: 'https://e2e-test.example.com' },
    });
    expect(resp.ok()).toBeTruthy();

    // Verify
    const getResp = await page.request.get('/api/admin/settings/base_url', {
      headers: { 'X-User-ID': '1' },
    });
    const data = await getResp.json();
    expect(data.value).toBe('https://e2e-test.example.com');
  });

  test('settings tab loads in admin dashboard', async ({ page }) => {
    await loginAsAdmin(page);

    // Click settings tab (gear icon, last tab)
    const tabs = page.locator('nav button');
    await tabs.last().click();

    // Verify settings page content
    await expect(page.getByText('System Base URL')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Discord Notifications')).toBeVisible();
    await expect(page.getByText('Change Admin PIN')).toBeVisible();
  });

  test('can export configuration', async ({ page }) => {
    const resp = await page.request.get('/api/admin/export-config', {
      headers: { 'X-User-ID': '1' },
    });
    expect(resp.ok()).toBeTruthy();
    const text = await resp.text();
    expect(text).toContain('users:');
    expect(text).toContain('chores:');
  });
});
