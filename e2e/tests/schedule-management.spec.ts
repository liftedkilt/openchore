import { test, expect } from '@playwright/test';
import { loginAsAdmin, apiGet } from './helpers/setup';

test.describe('Schedule Management', () => {
  test('deleting a multi-day schedule group removes all days', async ({ page }) => {
    await loginAsAdmin(page);

    // First, create a chore with a multi-day schedule via the wizard
    await page.getByRole('button', { name: /Add Chore/i }).click();
    await page.getByPlaceholder('e.g. Empty the dishwasher').fill('E2E Schedule Delete Test');
    await page.getByRole('button', { name: /Next/i }).click();

    // Assign to Emma, pick Weekdays (Mon-Fri = 5 days)
    await page.getByRole('button', { name: 'Emma' }).click();
    await page.getByRole('button', { name: 'Weekdays' }).click();
    await page.getByRole('button', { name: /Next/i }).click();

    // Create
    await page.getByRole('button', { name: /Create Chore/i }).click();
    await expect(page.getByText('New Chore')).not.toBeVisible({ timeout: 10_000 });

    // Find the chore and verify 5 schedules exist
    const chores = await apiGet(page, '/chores') as Array<{ id: number; title: string }>;
    const created = chores.find(c => c.title === 'E2E Schedule Delete Test');
    expect(created).toBeTruthy();

    const schedulesBefore = await apiGet(page, `/chores/${created!.id}/schedules`) as Array<{ id: number }>;
    expect(schedulesBefore).toHaveLength(5);

    // Expand the chore to see its schedules
    await page.getByText('E2E Schedule Delete Test').click();

    // Find and click the delete (trash) button on the schedule group
    // The schedule should show "Weekdays" as a single grouped row
    await expect(page.getByText('Weekdays')).toBeVisible();
    const scheduleRow = page.getByText('Weekdays').locator('..');
    await scheduleRow.getByRole('button').click();

    // Wait for deletion to complete
    await page.waitForTimeout(2000);

    // Verify all schedules are deleted
    const schedulesAfter = await apiGet(page, `/chores/${created!.id}/schedules`) as Array<{ id: number }>;
    expect(schedulesAfter).toHaveLength(0);
  });
});
