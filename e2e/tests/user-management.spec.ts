import { test, expect } from '@playwright/test';
import { loginAsAdmin, apiGet } from './helpers/setup';

test.describe('User Management', () => {
  test('admin can create a new child user via API', async ({ page }) => {
    const createResp = await page.request.post('/api/users', {
      headers: { 'X-User-ID': '1' },
      data: { name: 'E2E Test Child', role: 'child' },
    });
    expect(createResp.ok()).toBeTruthy();
    const created = await createResp.json();
    expect(created.name).toBe('E2E Test Child');
    expect(created.role).toBe('child');

    // Verify in user list
    const users = await apiGet(page, '/users');
    const found = users.find((u: any) => u.name === 'E2E Test Child');
    expect(found).toBeTruthy();

    // Verify shows on profile selection
    await page.goto('/login');
    await expect(page.getByText('E2E Test Child')).toBeVisible({ timeout: 5_000 });
  });

  test('admin can pause and unpause a user', async ({ page }) => {
    // Pause Lily (user 4)
    const pauseResp = await page.request.put('/api/users/4/pause', {
      headers: { 'X-User-ID': '1' },
    });
    expect(pauseResp.ok()).toBeTruthy();

    const users = await apiGet(page, '/users');
    const zoe = users.find((u: any) => u.name === 'Lily');
    expect(zoe.paused).toBe(true);

    // Unpause
    const unpauseResp = await page.request.put('/api/users/4/unpause', {
      headers: { 'X-User-ID': '1' },
    });
    expect(unpauseResp.ok()).toBeTruthy();

    const usersAfter = await apiGet(page, '/users');
    const zoeAfter = usersAfter.find((u: any) => u.name === 'Lily');
    expect(zoeAfter.paused).toBe(false);
  });

  test('admin can delete a user via API', async ({ page }) => {
    // Create a user to delete
    const createResp = await page.request.post('/api/users', {
      headers: { 'X-User-ID': '1' },
      data: { name: 'E2E Delete Me', role: 'child' },
    });
    const created = await createResp.json();

    // Delete
    const deleteResp = await page.request.delete(`/api/users/${created.id}`, {
      headers: { 'X-User-ID': '1' },
    });
    expect(deleteResp.ok()).toBeTruthy();

    // Verify gone
    const users = await apiGet(page, '/users');
    const found = users.find((u: any) => u.name === 'E2E Delete Me');
    expect(found).toBeFalsy();
  });

  test('people tab shows all seeded users', async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /People/i }).click();

    await expect(page.getByText('Emma')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Lily')).toBeVisible();
    await expect(page.getByText('Noah')).toBeVisible();
    await expect(page.getByText('Alex')).toBeVisible();
    await expect(page.getByText('Jamie')).toBeVisible();
  });
});
