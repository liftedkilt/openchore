import { type Page, expect } from '@playwright/test';

/** Seeded parent PINs (config/config.example.yaml). Kids have no PIN. */
export const SEEDED_PINS: Record<number, string> = { 1: '1234', 2: '5678' };

/** Enter a PIN on the on-screen PinPad. */
export async function enterPin(page: Page, pin: string) {
  for (const digit of pin) {
    await page.getByRole('button', { name: digit, exact: true }).click();
  }
}

/**
 * Sign in as a parent from the profile picker (tap profile → PIN), then open
 * the Manage screens from their dashboard.
 */
export async function loginAsAdmin(page: Page, pin = '1234', name = 'Alex') {
  await page.goto('/login');
  await page.getByRole('button', { name: `Select profile for ${name}` }).click();
  await enterPin(page, pin);
  await page.waitForURL('/');
  await page.getByRole('button', { name: 'Manage' }).click();
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

const API_ORIGIN = 'http://localhost:8080';
const tokenCache = new Map<string, string>();

/**
 * A session token for userId, obtained by signing in through the API. It is
 * fetched outside the browser so it never replaces the page's own session
 * cookie. Pass it as a Bearer header with page.request.
 */
export async function apiToken(userId: number, pin = SEEDED_PINS[userId]): Promise<string> {
  const key = `${userId}:${pin ?? ''}`;
  const cached = tokenCache.get(key);
  if (cached) return cached;
  const resp = await fetch(`${API_ORIGIN}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(pin ? { user_id: userId, pin } : { user_id: userId }),
  });
  if (!resp.ok) {
    throw new Error(`login as user ${userId} failed: ${resp.status} ${await resp.text()}`);
  }
  const body = await resp.json();
  tokenCache.set(key, body.token);
  return body.token;
}

/** Authorization headers for API calls made as userId (default: parent Alex). */
export async function authHeaders(userId = 1, pin?: string): Promise<Record<string, string>> {
  return { Authorization: `Bearer ${await apiToken(userId, pin ?? SEEDED_PINS[userId])}` };
}

/** Make an authenticated API request using the page's request context. */
export async function apiGet(page: Page, path: string, userId = 1) {
  const resp = await page.request.get(`/api${path}`, {
    headers: await authHeaders(userId),
  });
  expect(resp.ok()).toBeTruthy();
  return resp.json();
}

/**
 * Today's date as YYYY-MM-DD in the *local* timezone.
 *
 * The Go server decides what "today" means using its own local clock, so tests
 * must ask for the same day. `new Date().toISOString().slice(0, 10)` returns
 * the UTC date, which is a different day from local for much of the world —
 * under TZ=Asia/Tokyo it asks the server for yesterday, and lookups for
 * chores scheduled "today" come back empty.
 */
export function localDateStr(d = new Date()) {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
