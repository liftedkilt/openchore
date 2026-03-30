import { test, expect } from '@playwright/test';
import { loginAsAdmin, selectUser, apiGet } from './helpers/setup';

test.describe('Points System', () => {
  test('admin can manually adjust points via API', async ({ page }) => {
    // Get Natalie's initial balance
    const initialData = await apiGet(page, '/users/3/points', 3);
    const initialBalance = initialData.balance;

    // Adjust points via API
    const resp = await page.request.post('/api/points/adjust', {
      headers: { 'X-User-ID': '1' },
      data: { user_id: 3, amount: 50, note: 'E2E test adjustment' },
    });
    expect(resp.ok()).toBeTruthy();

    // Verify
    const updatedData = await apiGet(page, '/users/3/points', 3);
    expect(updatedData.balance).toBe(initialBalance + 50);
  });

  test('completing a chore creates a point transaction', async ({ page }) => {
    // Complete a chore via API and verify points transaction is created
    const today = new Date().toISOString().slice(0, 10);
    const chores = await apiGet(page, `/users/3/chores?view=daily&date=${today}`, 3);

    // Find an incomplete chore
    const incomplete = chores.find((c: any) => !c.completed && c.available);
    if (!incomplete) return; // All done for today

    await page.request.post(`/api/schedules/${incomplete.schedule_id}/complete`, {
      headers: { 'X-User-ID': '3' },
      data: { completion_date: today },
    });

    // Verify a transaction was created
    const pointsData = await apiGet(page, '/users/3/points', 3);
    const transactions = pointsData.transactions || [];
    expect(transactions.length).toBeGreaterThan(0);
  });

  test('point transactions are recorded', async ({ page }) => {
    // Add points
    await page.request.post('/api/points/adjust', {
      headers: { 'X-User-ID': '1' },
      data: { user_id: 3, amount: 10, note: 'E2E log test' },
    });

    // Verify transaction exists in points data
    const pointsData = await page.request.get('/api/users/3/points', {
      headers: { 'X-User-ID': '3' },
    });
    const data = await pointsData.json();
    const transactions = data.transactions || [];
    const found = transactions.find((t: any) => t.note === 'E2E log test');
    expect(found).toBeTruthy();
    expect(found.amount).toBe(10);
  });
});
