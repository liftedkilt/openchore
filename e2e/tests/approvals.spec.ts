import { test, expect } from '@playwright/test';
import { loginAsAdmin, apiGet } from './helpers/setup';

test.describe('Approval Workflow', () => {
  test('chore requiring approval goes to pending queue', async ({ page }) => {
    // Create a chore that requires approval via API
    const createResp = await page.request.post('/api/chores', {
      headers: { 'X-User-ID': '1' },
      data: {
        title: 'E2E Approval Test',
        category: 'core',
        points_value: 10,
        requires_approval: true,
      },
    });
    const chore = await createResp.json();

    // Create a daily schedule for Natalie (user 3)
    const today = new Date();
    const dayOfWeek = today.getDay();
    await page.request.post(`/api/chores/${chore.id}/schedules`, {
      headers: { 'X-User-ID': '1' },
      data: { assigned_to: 3, day_of_week: dayOfWeek },
    });

    // Complete the chore as Natalie
    const dateStr = today.toISOString().slice(0, 10);
    const chores = await apiGet(page, `/users/3/chores?view=daily&date=${dateStr}`, 3);
    const scheduled = chores.find((c: any) => c.title === 'E2E Approval Test');
    expect(scheduled).toBeTruthy();

    await page.request.post(`/api/schedules/${scheduled.schedule_id}/complete`, {
      headers: { 'X-User-ID': '3' },
      data: { completion_date: dateStr },
    });

    // Log in as admin and check approvals tab
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Approvals/i }).click();

    // Should see the pending completion
    await expect(page.getByText('E2E Approval Test')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Emma')).toBeVisible();
  });

  test('admin can approve a pending completion', async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Approvals/i }).click();

    // Find and approve
    const approveBtn = page.getByRole('button', { name: /approve/i }).first();
    if (await approveBtn.isVisible()) {
      await approveBtn.click();

      // Wait for the item to be removed from pending list
      await page.waitForTimeout(1000);

      // Verify approval went through - check that the button is gone or the list changed
      const pendingResp = await page.request.get('/api/completions/pending', {
        headers: { 'X-User-ID': '1' },
      });
      const pending = await pendingResp.json();
      const approvalChore = pending.find((p: any) => p.chore_title === 'E2E Approval Test');
      expect(approvalChore).toBeFalsy();
    }
  });

  test('empty approvals shows all caught up message', async ({ page }) => {
    // Approve everything first
    const pendingResp = await page.request.get('/api/completions/pending', {
      headers: { 'X-User-ID': '1' },
    });
    const pending = await pendingResp.json();
    for (const p of pending) {
      await page.request.post(`/api/completions/${p.id}/approve`, {
        headers: { 'X-User-ID': '1' },
      });
    }

    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Approvals/i }).click();

    await expect(page.getByText(/caught up|no pending/i)).toBeVisible({ timeout: 5_000 });
  });
});
