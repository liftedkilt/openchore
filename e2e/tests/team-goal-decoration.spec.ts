import { execFileSync, spawn, type ChildProcess } from 'child_process';
import fs from 'fs';
import net from 'net';
import os from 'os';
import path from 'path';
import { test, expect } from '@playwright/test';
import { authHeaders, localDateStr, selectUser } from './helpers/setup';

const REPO_ROOT = path.resolve(__dirname, '../..');
const EMMA = 3;

function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.unref();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const addr = srv.address();
      const port = typeof addr === 'object' && addr ? addr.port : 0;
      srv.close(() => resolve(port));
    });
  });
}

async function waitForHttp(url: string, timeoutMs = 15_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(url);
      if (resp.status > 0) return;
    } catch {
      // keep polling until listener is up
    }
    await new Promise(r => setTimeout(r, 150));
  }
  throw new Error(`Timed out waiting for ${url}`);
}

async function peerFetch(peerBase: string, endpoint: string, init: RequestInit = {}) {
  const resp = await fetch(`${peerBase}${endpoint}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) },
  });
  const text = await resp.text();
  if (!resp.ok) {
    throw new Error(`peer ${init.method ?? 'GET'} ${endpoint} failed (${resp.status}): ${text}`);
  }
  return text ? JSON.parse(text) : null;
}

async function peerAuthHeaders(peerBase: string, userId: number, pin?: string): Promise<Record<string, string>> {
  const body = await peerFetch(peerBase, '/api/auth/login', {
    method: 'POST',
    body: JSON.stringify(pin ? { user_id: userId, pin } : { user_id: userId }),
  });
  return { Authorization: `Bearer ${body.token}` };
}

test.describe('Team goal farm decoration', () => {
  test.describe.configure({ mode: 'serial' });

  let tmpDir = '';
  let hubProc: ChildProcess | null = null;
  let peerProc: ChildProcess | null = null;
  let peerBase = '';
  let peerAdmin: Record<string, string> = {};
  let peerEmma: Record<string, string> = {};
  let aLinkId = 0;
  let bLinkId = 0;
  let aMakeBedScheduleId = 0;

  test.beforeAll(async ({ request }) => {
    test.setTimeout(60_000);
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'openchore-e2e-goal-'));
    const hubBin = path.join(tmpDir, 'hub');
    const serverBin = path.join(tmpDir, 'server');

    // Compile direct binaries into tmpDir so SIGTERM cleanly stops the exact processes.
    execFileSync('go', ['build', '-o', hubBin, 'cmd/hub/main.go'], { cwd: REPO_ROOT, stdio: 'pipe' });
    execFileSync('go', ['build', '-o', serverBin, 'cmd/server/main.go'], { cwd: REPO_ROOT, stdio: 'pipe' });

    const hubPort = await getFreePort();
    const peerPort = await getFreePort();
    const hubURL = `http://127.0.0.1:${hubPort}`;
    peerBase = `http://127.0.0.1:${peerPort}`;

    hubProc = spawn(hubBin, [], {
      cwd: tmpDir,
      env: { ...process.env, PORT: String(hubPort), HUB_DB: path.join(tmpDir, 'hub.db'), HUB_OPEN: 'true' },
      stdio: 'ignore',
    });
    peerProc = spawn(serverBin, [], {
      cwd: REPO_ROOT,
      env: {
        ...process.env,
        PORT: String(peerPort),
        DB_PATH: path.join(tmpDir, 'peer.db'),
        CONFIG_PATH: path.join(REPO_ROOT, 'config/config.example.yaml'),
        OPENCHORE_HUB_ALLOW_PRIVATE: 'true',
      },
      stdio: 'ignore',
    });

    await waitForHttp(`${hubURL}/v1/links`);
    await waitForHttp(`${peerBase}/api/users`);

    const aAdmin = await authHeaders(1);
    const aEmma = await authHeaders(EMMA);
    peerAdmin = await peerAuthHeaders(peerBase, 1, '1234');
    peerEmma = await peerAuthHeaders(peerBase, EMMA);

    // Enable pets & leaderboard on both families and adopt animals for Emma on both.
    await request.put('/api/admin/pets', { headers: aAdmin, data: { enabled: true } });
    await request.delete(`/api/admin/users/${EMMA}/pet`, { headers: aAdmin });
    const aAdopt = await request.post('/api/me/pet', { headers: aEmma, data: { name: 'Biscuit', species: 'dog' } });
    expect(aAdopt.ok()).toBeTruthy();
    await request.put('/api/admin/friends', {
      headers: aAdmin,
      data: { leaderboard_enabled: true, family_name: 'The Home Team', hub_url: hubURL },
    });

    await peerFetch(peerBase, '/api/admin/pets', { method: 'PUT', headers: peerAdmin, body: JSON.stringify({ enabled: true }) });
    await peerFetch(peerBase, '/api/me/pet', { method: 'POST', headers: peerEmma, body: JSON.stringify({ name: 'Clover', species: 'horse' }) });
    await peerFetch(peerBase, '/api/admin/friends', {
      method: 'PUT',
      headers: peerAdmin,
      body: JSON.stringify({ leaderboard_enabled: true, family_name: 'The Neighbors' }),
    });

    // Link A <-> B through the real hub.
    const invResp = await request.post('/api/admin/friends/invites', { headers: aAdmin });
    expect(invResp.status()).toBe(201);
    const inv = await invResp.json();
    aLinkId = inv.id;

    const joined = await peerFetch(peerBase, '/api/admin/friends/join', {
      method: 'POST',
      headers: peerAdmin,
      body: JSON.stringify({ invite: inv.invite }),
    });
    bLinkId = joined.id;

    // Inviting parent syncs and confirms the joining family before configuring shares.
    await request.post(`/api/admin/friends/${aLinkId}/sync`, { headers: aAdmin });
    const confirmResp = await request.post(`/api/admin/friends/${aLinkId}/confirm`, { headers: aAdmin });
    expect(confirmResp.ok()).toBeTruthy();

    // Share Emma from both sides, sync, then mutually accept Emma and sync twice.
    await request.put(`/api/admin/friends/${aLinkId}`, { headers: aAdmin, data: { share_user_ids: [EMMA] } });
    await peerFetch(peerBase, `/api/admin/friends/${bLinkId}`, {
      method: 'PUT',
      headers: peerAdmin,
      body: JSON.stringify({ share_user_ids: [EMMA] }),
    });
    for (let i = 0; i < 2; i++) {
      await request.post(`/api/admin/friends/${aLinkId}/sync`, { headers: aAdmin });
      await peerFetch(peerBase, `/api/admin/friends/${bLinkId}/sync`, { method: 'POST', headers: peerAdmin });
    }
    await request.put(`/api/admin/friends/${aLinkId}`, { headers: aAdmin, data: { accept_member_ids: [String(EMMA)] } });
    await peerFetch(peerBase, `/api/admin/friends/${bLinkId}`, {
      method: 'PUT',
      headers: peerAdmin,
      body: JSON.stringify({ accept_member_ids: [String(EMMA)] }),
    });
    for (let i = 0; i < 2; i++) {
      await request.post(`/api/admin/friends/${aLinkId}/sync`, { headers: aAdmin });
      await peerFetch(peerBase, `/api/admin/friends/${bLinkId}/sync`, { method: 'POST', headers: peerAdmin });
    }
  });

  test.afterAll(async ({ request }) => {
    const aAdmin = await authHeaders(1);
    const aEmma = await authHeaders(EMMA);
    const today = localDateStr();
    if (aMakeBedScheduleId > 0) {
      await request.delete(`/api/schedules/${aMakeBedScheduleId}/complete?date=${today}`, { headers: aEmma }).catch(() => {});
    }
    if (aLinkId > 0) {
      await request.delete(`/api/admin/friends/${aLinkId}`, { headers: aAdmin }).catch(() => {});
    }
    await request.delete(`/api/admin/users/${EMMA}/pet`, { headers: aAdmin }).catch(() => {});
    await request.put('/api/admin/pets', { headers: aAdmin, data: { enabled: false } }).catch(() => {});
    await request.put('/api/admin/friends', { headers: aAdmin, data: { leaderboard_enabled: false, hub_url: '' } }).catch(() => {});

    for (const proc of [peerProc, hubProc]) {
      if (proc && !proc.killed) {
        proc.kill('SIGTERM');
      }
    }
    if (tmpDir) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  test('stays hidden on both farms before a team goal is agreed and reached', async ({ page }) => {
    await selectUser(page, 'Emma');
    await expect(page.getByRole('button', { name: 'Visit Biscuit' })).toBeVisible();
    await expect(page.getByTestId('farm-goal-decoration')).toHaveCount(0);

    // Open full farm sheet: still hidden.
    await page.getByRole('button', { name: 'Visit Biscuit' }).click();
    const farmDialog = page.getByRole('dialog', { name: 'The farm' });
    await expect(farmDialog).toBeVisible();
    await expect(farmDialog.getByTestId('farm-goal-decoration')).toHaveCount(0);
    await farmDialog.getByRole('button', { name: 'Close' }).click();

    // Visit friend family's farm via Leaderboard: still hidden.
    await page.getByRole('button', { name: 'See everyone' }).click();
    const board = page.getByRole('dialog', { name: 'Leaderboard' });
    await expect(board).toBeVisible();
    await board.getByRole('button', { name: /The Neighbors/ }).click();
    const friendDialog = page.getByRole('dialog', { name: /The Neighbors/ });
    await expect(friendDialog).toBeVisible();
    await expect(friendDialog.getByTestId('farm-goal-decoration')).toHaveCount(0);
  });

  test('appears on both own farm and visited friend farm when agreed team goal is reached, works in winter, and stays off /ambient', async ({ page, request }) => {
    const aAdmin = await authHeaders(1);
    const aEmma = await authHeaders(EMMA);
    const today = localDateStr();

    // Agree on weekly team goal 15 on both sides (Emma has 5-6 daily core chores in
    // config.example.yaml, so 0 done = 0% < 15 and completing Make Bed = 16-20% >= 15
    // on every day of the week).
    await request.put(`/api/admin/friends/${aLinkId}`, { headers: aAdmin, data: { goal_target: 15 } });
    await peerFetch(peerBase, `/api/admin/friends/${bLinkId}`, {
      method: 'PUT',
      headers: peerAdmin,
      body: JSON.stringify({ goal_target: 15 }),
    });

    // Complete "Make Bed" (no available_at restriction) for Emma on both families -> combined score >= 15.
    // Earlier specs in the shared-state run may already have completed Make Bed for Emma today.
    const aChores = await (await request.get(`/api/users/${EMMA}/chores?view=daily&date=${today}`, { headers: aEmma })).json();
    const aMakeBed = aChores.find((c: { title: string; completed?: boolean; schedule_id: number }) => c.title === 'Make Bed');
    expect(aMakeBed).toBeTruthy();
    if (!aMakeBed.completed) {
      aMakeBedScheduleId = aMakeBed.schedule_id;
      const aComp = await request.post(`/api/schedules/${aMakeBedScheduleId}/complete`, {
        headers: aEmma,
        data: { date: today },
      });
      expect(aComp.ok()).toBeTruthy();
    }

    const bChores = await peerFetch(peerBase, `/api/users/${EMMA}/chores?view=daily&date=${today}`, { headers: peerEmma });
    const bMakeBed = bChores.find((c: { title: string }) => c.title === 'Make Bed');
    expect(bMakeBed).toBeTruthy();
    await peerFetch(peerBase, `/api/schedules/${bMakeBed.schedule_id}/complete`, {
      method: 'POST',
      headers: peerEmma,
      body: JSON.stringify({ date: today }),
    });

    // Sync both sides twice so both households carry the agreed goal and combined team score.
    for (let i = 0; i < 2; i++) {
      await request.post(`/api/admin/friends/${aLinkId}/sync`, { headers: aAdmin });
      await peerFetch(peerBase, `/api/admin/friends/${bLinkId}/sync`, { method: 'POST', headers: peerAdmin });
    }

    // Also verify the peer household's own API reports the celebration for both farms.
    const bVisitors = await peerFetch(peerBase, '/api/friends/visitors', { headers: peerEmma });
    expect(bVisitors.goal_celebrations).toHaveLength(1);
    expect(bVisitors.goal_celebrations[0].family).toBe('The Home Team');
    const bFarms = await peerFetch(peerBase, '/api/friends/farms', { headers: peerEmma });
    expect(bFarms.farms[0].goal_celebration?.target).toBe(15);

    // 1. Today compact farm shows celebratory decoration.
    await selectUser(page, 'Emma');
    const compactPill = page.getByTestId('farm-goal-decoration');
    await expect(compactPill).toBeVisible();
    await expect(compactPill).toContainText('Team goal!');

    // 2. Full own farm sheet shows celebratory decoration with friend family & target.
    await page.getByRole('button', { name: 'Visit Biscuit' }).click();
    const farmDialog = page.getByRole('dialog', { name: 'The farm' });
    await expect(farmDialog).toBeVisible();
    const ownPill = farmDialog.getByTestId('farm-goal-decoration');
    await expect(ownPill).toBeVisible();
    await expect(ownPill).toContainText('Team goal with The Neighbors: 15!');
    await farmDialog.getByRole('button', { name: 'Close' }).click();

    // 3. Visited friend farm shows celebratory decoration too.
    await page.getByRole('button', { name: 'See everyone' }).click();
    const board = page.getByRole('dialog', { name: 'Leaderboard' });
    await expect(board).toBeVisible();
    await board.getByRole('button', { name: /The Neighbors/ }).click();
    const friendDialog = page.getByRole('dialog', { name: /The Neighbors/ });
    await expect(friendDialog).toBeVisible();
    const friendPill = friendDialog.getByTestId('farm-goal-decoration');
    await expect(friendPill).toBeVisible();
    await expect(friendPill).toContainText('Team goal reached: 15!');
    await friendDialog.getByRole('button', { name: 'Close' }).click();

    // 4. Christmas winter backdrop renders cleanly alongside the decoration.
    await page.goto('/?season=christmas');
    await expect(page.locator('[data-season="christmas"]')).toBeVisible();
    await expect(page.getByTestId('farm-goal-decoration')).toBeVisible();

    // 5. Public wall display (/ambient) NEVER exposes friend team goal decorations.
    await page.goto('/ambient');
    await expect(page.getByRole('group', { name: 'The farm' })).toBeVisible();
    await expect(page.getByTestId('farm-goal-decoration')).toHaveCount(0);
  });
});
