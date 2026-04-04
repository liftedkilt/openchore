import { test, expect } from '@playwright/test';
import { selectUser, loginAsAdmin } from './helpers/setup';

/**
 * Screenshot capture suite — generates wiki screenshots from seeded e2e data.
 * Run: npx playwright test screenshots.spec.ts
 * Output: e2e/screenshots/
 */
test.describe('Screenshots', () => {
  const dir = 'screenshots';

  test('01 - Profile Selection', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto('/login');
    await expect(page.getByText('Emma')).toBeVisible({ timeout: 10_000 });
    await page.screenshot({ path: `${dir}/01-profile-selection.png`, fullPage: false });
  });

  test('02 - Kid Dashboard (Daily)', async ({ page }) => {
    await page.setViewportSize({ width: 430, height: 932 });
    await selectUser(page, 'Emma');
    await expect(page.getByText(/\d+ of \d+ complete/i)).toBeVisible({ timeout: 10_000 });
    // Wait for chore cards to render
    await expect(page.getByText('Make Bed')).toBeVisible();
    await page.screenshot({ path: `${dir}/02-kid-dashboard-daily.png`, fullPage: true });
  });

  test('03 - Kid Dashboard (Weekly)', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });
    await page.getByText('Week').click();
    await expect(page.getByText('Mon').first()).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/03-kid-dashboard-weekly.png`, fullPage: false });
  });

  test('04 - Rewards Store', async ({ page }) => {
    await page.setViewportSize({ width: 430, height: 932 });
    await selectUser(page, 'Emma');
    await expect(page.locator('body')).toContainText('pts', { timeout: 10_000 });
    await page.getByText(/Rewards/i).click();
    await expect(page.getByText('Extra Screen Time')).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/04-rewards-store.png`, fullPage: true });
  });

  test('05 - Admin Dashboard (Chores)', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await loginAsAdmin(page);
    await expect(page.getByText('Make Bed')).toBeVisible({ timeout: 10_000 });
    await page.screenshot({ path: `${dir}/05-admin-chores.png`, fullPage: false });
  });

  test('06 - Admin Dashboard (Rewards)', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Rewards/i }).click();
    await expect(page.getByText('Extra Screen Time')).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/06-admin-rewards.png`, fullPage: false });
  });

  test('07 - Admin Dashboard (Points)', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Points/i }).click();
    await expect(page.getByText('Emma')).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/07-admin-points.png`, fullPage: false });
  });

  test('08 - Admin Dashboard (People)', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /People/i }).click();
    await expect(page.getByText('Emma')).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/08-admin-people.png`, fullPage: false });
  });

  test('09 - Ambient Dashboard', async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 1080 });
    await page.goto('/ambient');
    await expect(page.getByText('Emma').first()).toBeVisible({ timeout: 10_000 });
    // Wait for chart to render
    await page.waitForTimeout(2000);
    await page.screenshot({ path: `${dir}/09-ambient-dashboard.png`, fullPage: false });
  });

  test('10 - Admin PIN Entry', async ({ page }) => {
    await page.setViewportSize({ width: 430, height: 932 });
    await page.goto('/admin');
    await expect(page.getByRole('button', { name: '1', exact: true })).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/10-admin-pin.png`, fullPage: false });
  });

  test('11 - Quick Assign Modal', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await loginAsAdmin(page);
    await expect(page.getByText('Make Bed')).toBeVisible({ timeout: 10_000 });
    // Click the FAB
    await page.locator('button[title="Quick Assign"]').click();
    await expect(page.getByText('Quick Assign')).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/11-quick-assign.png`, fullPage: false });
  });

  test('12 - Chore Creation Wizard', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await loginAsAdmin(page);
    await expect(page.getByText('Make Bed')).toBeVisible({ timeout: 10_000 });
    // Click the Add Chore button
    await page.getByRole('button', { name: /Add Chore/i }).click();
    await expect(page.getByText('New Chore')).toBeVisible({ timeout: 5_000 });
    await page.screenshot({ path: `${dir}/12-chore-wizard.png`, fullPage: false });
  });
});
