import { test, expect } from '@playwright/test';

test.describe('Webhooks', () => {
  test('webhook CRUD via API', async ({ page }) => {
    // Create
    const createResp = await page.request.post('/api/admin/webhooks', {
      headers: { 'X-User-ID': '1' },
      data: {
        url: 'https://example.com/e2e-webhook',
        events: 'chore.completed,reward.redeemed',
        active: true,
      },
    });
    expect(createResp.ok()).toBeTruthy();
    const webhook = await createResp.json();
    expect(webhook.id).toBeTruthy();

    // Read
    const listResp = await page.request.get('/api/admin/webhooks', {
      headers: { 'X-User-ID': '1' },
    });
    const webhooks = await listResp.json();
    const found = webhooks.find((w: any) => w.url === 'https://example.com/e2e-webhook');
    expect(found).toBeTruthy();

    // Update (disable)
    const updateResp = await page.request.put(`/api/admin/webhooks/${webhook.id}`, {
      headers: { 'X-User-ID': '1' },
      data: { ...webhook, active: false },
    });
    expect(updateResp.ok()).toBeTruthy();
    const updated = await updateResp.json();
    expect(updated.active).toBe(false);

    // Delete
    const deleteResp = await page.request.delete(`/api/admin/webhooks/${webhook.id}`, {
      headers: { 'X-User-ID': '1' },
    });
    expect(deleteResp.ok()).toBeTruthy();
  });
});
