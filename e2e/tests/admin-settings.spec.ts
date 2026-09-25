import { test, expect } from '@playwright/test';
import { loginAsAdmin, authHeaders } from './helpers/setup';

test.describe('Admin Settings', () => {
  test('can update system base URL via API', async ({ page }) => {
    const resp = await page.request.put('/api/admin/settings/base_url', {
      headers: await authHeaders(1),
      data: { value: 'https://e2e-test.example.com' },
    });
    expect(resp.ok()).toBeTruthy();

    // Verify
    const getResp = await page.request.get('/api/admin/settings/base_url', {
      headers: await authHeaders(1),
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
    await expect(page.getByRole('heading', { name: 'Sign-in' })).toBeVisible();
  });

  test('shows the AI connection form', async ({ page }) => {
    await loginAsAdmin(page);
    await page.locator('nav button').last().click();

    const card = page.locator('form', { has: page.getByRole('heading', { name: 'AI connections' }) });
    await expect(card).toBeVisible({ timeout: 5_000 });
    await expect(card.getByText('AI model')).toBeVisible();
    await expect(card.getByText('Speech service')).toBeVisible();
    await expect(card.getByRole('button', { name: 'Save connections' })).toBeVisible();
  });

  test('can add, edit and remove a sign-in provider', async ({ page }) => {
    await loginAsAdmin(page);
    await page.locator('nav button').last().click();

    const card = page.locator('section', { has: page.getByRole('heading', { name: 'Sign-in', exact: true }) });
    await card.getByRole('button', { name: 'Add provider' }).click();

    const form = card.getByRole('form', { name: 'Add a sign-in provider' });
    await form.getByLabel('Button label').fill('E2E Family ID');
    await expect(form.getByRole('textbox', { name: /^ID\b/ })).toHaveValue('e2e-family-id');
    await expect(form.getByText('/api/auth/oidc/e2e-family-id/callback')).toBeVisible();
    await form.getByLabel('Issuer URL').fill('https://id.e2e.invalid');
    await form.getByLabel('Client ID').fill('openchore');
    await form.getByLabel('Client secret').fill('s3cret');
    await form.getByRole('button', { name: 'Add provider' }).click();

    await expect(card.getByText('E2E Family ID saved')).toBeVisible();
    await expect(card.getByText('https://id.e2e.invalid')).toBeVisible();

    // The login screen can offer it straight away.
    const providers = await (await page.request.get('/api/auth/providers')).json();
    expect(providers).toContainEqual({ id: 'e2e-family-id', name: 'E2E Family ID' });

    // Edit the label; the secret is kept.
    await card.getByRole('button', { name: 'Edit' }).click();
    const edit = card.getByRole('form', { name: 'Edit E2E Family ID' });
    await expect(edit.getByLabel('Client secret')).toHaveAttribute('placeholder', /Saved/);
    await edit.getByLabel('Button label').fill('E2E Home ID');
    await edit.getByRole('button', { name: 'Save' }).click();
    await expect(card.getByText('E2E Home ID saved')).toBeVisible();

    page.once('dialog', d => d.accept());
    await card.getByRole('button', { name: 'Remove E2E Home ID' }).click();
    await expect(card.getByText('E2E Home ID removed')).toBeVisible();
    const after = await (await page.request.get('/api/auth/providers')).json();
    expect(after.map((p: { id: string }) => p.id)).not.toContain('e2e-family-id');
  });

  test('can export configuration', async ({ page }) => {
    const resp = await page.request.get('/api/admin/export-config', {
      headers: await authHeaders(1),
    });
    expect(resp.ok()).toBeTruthy();
    const text = await resp.text();
    expect(text).toContain('users:');
    expect(text).toContain('chores:');
  });
});
