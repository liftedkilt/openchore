import { test, expect, type Page } from '@playwright/test';
import { selectUser } from './helpers/setup';

/**
 * Regression cover for the goal-savings decay loophole.
 *
 * Points committed to a savings goal sit in the ledger as `commit_to_goal`
 * debits, so they're excluded from the spendable balance. Decay used to clamp
 * its debit against exactly that balance, which meant a kid who parked every
 * point in an expensive goal they never redeemed decayed by zero — do the fun
 * chores, skip the rest, lose nothing. Decay must now reach into the goal.
 *
 * The decay worker only ticks every 15 minutes in production, so the e2e
 * server is started with POINTS_DECAY_INTERVAL set (see playwright.config.ts).
 * If this spec times out waiting for a decay, check that override first.
 */

const ADMIN = { 'X-User-ID': '1' };

/** Yesterday's day-of-week index, matching the server's local clock. */
function yesterdayDayOfWeek() {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  return d.getDay();
}

/** Yesterday as YYYY-MM-DD in local time. */
function yesterdayDateStr() {
  const d = new Date();
  d.setDate(d.getDate() - 1);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

async function postJSON(page: Page, path: string, data: unknown, headers = ADMIN) {
  const resp = await page.request.post(path, { headers, data });
  expect(resp.ok(), `${path} -> ${resp.status()}`).toBeTruthy();
  return resp.status() === 204 ? null : resp.json();
}

/** Create a child, a goal they've banked every point into, and a chore due yesterday. */
async function seedSaver(page: Page, label: string, opts: { chorePoints?: number } = {}) {
  const stamp = `${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
  const name = `${label} ${stamp}`;

  const kid = await postJSON(page, '/api/users', { name, role: 'child' });
  const reward = await postJSON(page, '/api/rewards', {
    name: `Dream Console ${stamp}`,
    icon: '🎮',
    cost: 5000,
  });

  // The kid commits and banks everything, leaving a spendable balance of 0.
  const commitment = await postJSON(
    page,
    `/api/rewards/${reward.id}/commit`,
    { auto_contribute_percent: 0 },
    { 'X-User-ID': String(kid.id) },
  );
  await postJSON(page, '/api/points/adjust', { user_id: kid.id, amount: 50, note: 'e2e allowance' });
  await postJSON(
    page,
    `/api/commitments/${commitment.id}/contribute`,
    { amount: 50 },
    { 'X-User-ID': String(kid.id) },
  );

  const chore = await postJSON(page, '/api/chores', {
    title: `Tidy Up ${stamp}`,
    category: 'required',
    points_value: opts.chorePoints ?? 10,
  });
  const schedule = await postJSON(page, `/api/chores/${chore.id}/schedules`, {
    assigned_to: kid.id,
    day_of_week: yesterdayDayOfWeek(),
  });

  return { kid, reward, commitment, chore, schedule, name };
}

async function enableDecay(page: Page, kidId: number, rate: number) {
  const resp = await page.request.put(`/api/admin/users/${kidId}/decay`, {
    headers: ADMIN,
    data: { enabled: true, decay_rate: rate, decay_interval_hours: 24 },
  });
  expect(resp.ok()).toBeTruthy();
}

async function getPoints(page: Page, kidId: number) {
  const resp = await page.request.get(`/api/users/${kidId}/points`, {
    headers: { 'X-User-ID': String(kidId) },
  });
  expect(resp.ok()).toBeTruthy();
  return resp.json();
}

test.describe('Points decay vs. goal savings', () => {
  // The worker tick plus the poll interval; generous because the whole e2e
  // suite shares one API process.
  test.setTimeout(60_000);

  test('a kid who banks everything in a goal still loses points for a missed chore', async ({ page }) => {
    const saver = await seedSaver(page, 'Decay Saver');

    // Nothing spendable, everything in the goal: the exact shape of the bug.
    const before = await getPoints(page, saver.kid.id);
    expect(before.balance).toBe(0);
    expect(before.committed).toBe(50);

    await enableDecay(page, saver.kid.id, 7);

    // Wait for the background worker to run a pass.
    await expect
      .poll(async () => (await getPoints(page, saver.kid.id)).committed, {
        timeout: 30_000,
        intervals: [500],
        message:
          'goal savings never decayed — is POINTS_DECAY_INTERVAL set on the e2e API server?',
      })
      .toBe(43);

    const after = await getPoints(page, saver.kid.id);
    // The debit came out of the goal, not out of thin air.
    expect(after.balance).toBe(0);

    const decayRows = after.transactions.filter((t: any) => t.reason === 'points_decay');
    expect(decayRows).toHaveLength(1);
    expect(decayRows[0].amount).toBe(-7);

    const reclaimRows = after.transactions.filter((t: any) => t.reason === 'goal_break');
    expect(reclaimRows).toHaveLength(1);
    expect(reclaimRows[0].amount).toBe(7);
    expect(reclaimRows[0].reference_id).toBe(saver.commitment.id);

    // The goal survives, just lighter — it is not cancelled out from under them.
    const commitments = await (
      await page.request.get(`/api/users/${saver.kid.id}/commitments`, {
        headers: { 'X-User-ID': String(saver.kid.id) },
      })
    ).json();
    const active = commitments.find((c: any) => c.id === saver.commitment.id);
    expect(active.status).toBe('active');
    expect(active.amount_saved).toBe(43);
  });

  test("the kid's goal card shows the reduced savings", async ({ page }) => {
    const saver = await seedSaver(page, 'Decay Card');
    await enableDecay(page, saver.kid.id, 7);

    await expect
      .poll(async () => (await getPoints(page, saver.kid.id)).committed, {
        timeout: 30_000,
        intervals: [500],
        message:
          'goal savings never decayed — is POINTS_DECAY_INTERVAL set on the e2e API server?',
      })
      .toBe(43);

    await selectUser(page, saver.name);
    await expect(page.getByText(saver.reward.name)).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('43 / 5000 pts')).toBeVisible({ timeout: 10_000 });
  });

  test('a kid who finished yesterday keeps their savings on the same pass', async ({ page }) => {
    // Two savers, identical but for one thing: the control did the chore.
    const slacker = await seedSaver(page, 'Decay Slacker');
    const control = await seedSaver(page, 'Decay Control');

    await postJSON(page, `/api/schedules/${control.schedule.id}/complete`, {
      completed_by: control.kid.id,
      completion_date: yesterdayDateStr(),
    });

    await enableDecay(page, slacker.kid.id, 7);
    await enableDecay(page, control.kid.id, 7);

    // When the slacker's decay lands, the same worker pass has already
    // considered the control — so this is a decision, not a race.
    await expect
      .poll(async () => (await getPoints(page, slacker.kid.id)).committed, {
        timeout: 30_000,
        intervals: [500],
        message:
          'goal savings never decayed — is POINTS_DECAY_INTERVAL set on the e2e API server?',
      })
      .toBe(43);

    const controlPoints = await getPoints(page, control.kid.id);
    expect(controlPoints.committed).toBe(50);
    expect(
      controlPoints.transactions.filter((t: any) => t.reason === 'points_decay'),
    ).toHaveLength(0);
  });
});
