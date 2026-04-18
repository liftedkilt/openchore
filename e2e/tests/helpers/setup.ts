import { type Page, expect } from '@playwright/test';

/** Navigate to /admin, punch in the passcode, wait for admin dashboard. */
export async function loginAsAdmin(page: Page, passcode = '1234') {
  await page.goto('/admin');
  for (const digit of passcode) {
    await page.getByRole('button', { name: digit, exact: true }).click();
  }
  await page.waitForURL('/admin/dashboard');
}

/**
 * Click a top-level admin dashboard tab by its visible label. Use after
 * loginAsAdmin() to navigate tests off the default (Kids) tab to whatever
 * tab is under test.
 */
export async function openAdminTab(page: Page, label: RegExp | string) {
  const matcher = typeof label === 'string' ? new RegExp(`^${label}$`, 'i') : label;
  await page.getByRole('button', { name: matcher }).click();
}

/** Select a user profile by name from the /login screen. */
export async function selectUser(page: Page, name: string) {
  await page.goto('/login');
  await page.getByText(name, { exact: true }).click();
  await page.waitForURL('/');
}

/** Set admin session directly (bypass PIN flow). */
export async function setAdminSession(page: Page) {
  await page.evaluate(() => sessionStorage.setItem('openchore_admin', 'true'));
}

/** Make an authenticated API request using the page's request context. */
export async function apiGet(page: Page, path: string, userId = 1) {
  const resp = await page.request.get(`/api${path}`, {
    headers: { 'X-User-ID': String(userId) },
  });
  expect(resp.ok()).toBeTruthy();
  return resp.json();
}
