import { test, expect } from '@playwright/test';
import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { authHeaders, loginAsAdmin, selectUser } from './helpers/setup';

// The weekly leaderboard is off by default. These turn it on, check what a
// parent and a kid see, and turn it off again so other specs see the usual
// Today screen.
test.describe('Leaderboard', () => {
  test.afterAll(async ({ request }) => {
    await request.put('/api/admin/friends', { headers: await authHeaders(1), data: { leaderboard_enabled: false } });
  });

  test('is hidden from kids until a parent turns it on', async ({ page }) => {
    await page.request.put('/api/admin/friends', { headers: await authHeaders(1), data: { leaderboard_enabled: false } });
    await selectUser(page, 'Emma');
    await expect(page.getByRole('button', { name: /Add a task/ })).toBeVisible();
    await expect(page.getByRole('button', { name: 'See everyone' })).toHaveCount(0);
  });

  test('a parent turns it on in Settings and names the family', async ({ page }) => {
    await loginAsAdmin(page);
    await page.locator('nav button').last().click();
    const section = page.locator('section', { has: page.getByRole('heading', { name: 'Leaderboard & friend families' }) });
    await expect(section).toBeVisible({ timeout: 5_000 });

    const toggle = section.getByLabel('Show the weekly leaderboard');
    await toggle.click(); // saved to the server first, then shown as ticked
    await expect(toggle).toBeChecked();

    // Linking needs a family name first.
    await expect(section.getByText('Give your family a name before linking.')).toBeVisible();
    await section.getByPlaceholder('e.g. The Smiths').fill('The Testers');
    await section.getByRole('button', { name: 'Save' }).first().click();
    await expect(section.getByText('Give your family a name before linking.')).toHaveCount(0);

    const cfg = await (await page.request.get('/api/admin/friends', { headers: await authHeaders(1) })).json();
    expect(cfg.leaderboard_enabled).toBe(true);
    expect(cfg.family_name).toBe('The Testers');
    expect(cfg.links).toEqual([]);
  });

  test('a kid sees the card on Today and the full leaderboard', async ({ page }) => {
    await page.request.put('/api/admin/friends', { headers: await authHeaders(1), data: { leaderboard_enabled: true } });
    await selectUser(page, 'Emma');

    await page.getByRole('button', { name: 'See everyone' }).click();
    const sheet = page.getByRole('dialog', { name: 'Leaderboard' });
    await expect(sheet).toBeVisible();
    await expect(sheet.getByRole('tab', { name: 'This week' })).toHaveAttribute('aria-selected', 'true');
    // Everyone in the family is listed, and Emma's own row is marked.
    for (const name of ['Emma', 'Lily', 'Noah']) {
      await expect(sheet.getByRole('listitem').filter({ hasText: name })).toBeVisible();
    }
    await expect(sheet.getByText('You', { exact: true })).toBeVisible();
    await expect(sheet.getByText(/Your score is out of 100/)).toBeVisible();

    await sheet.getByRole('tab', { name: 'Last week' }).click();
    await expect(sheet.getByRole('tab', { name: 'Last week' })).toHaveAttribute('aria-selected', 'true');
  });

  test('the leaderboard needs a signed-in person', async ({ request }) => {
    expect((await request.get('/api/leaderboard')).status()).toBe(401);
    expect((await request.get('/api/friends/farms')).status()).toBe(401);
  });
});

async function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.unref();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const addr = srv.address();
      if (!addr || typeof addr === 'string') return reject(new Error('no address'));
      const { port } = addr;
      srv.close(() => resolve(port));
    });
  });
}

async function waitForHealth(url: string, timeoutMs = 10_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      // keep polling
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`hub did not become healthy at ${url}`);
}

interface ParsedInvite {
  h: string;
  l: string;
  c: string;
  k: string;
}

function decodeInvite(code: string): ParsedInvite {
  const rest = code.replace(/\s+/g, '').replace(/^ocf1\./, '');
  return JSON.parse(Buffer.from(rest, 'base64url').toString('utf8')) as ParsedInvite;
}

function sealForSideB(inv: ParsedInvite, peerName: string): string {
  const key = Buffer.from(inv.k, 'base64url');
  const nonce = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv('aes-256-gcm', key, nonce);
  cipher.setAAD(Buffer.from(`${inv.l}:b`));
  const plain = Buffer.from(
    JSON.stringify({
      version: 1,
      family: peerName,
      generated_at: new Date().toISOString(),
      today: new Date().toISOString().slice(0, 10),
      members: [{ id: '99', name: 'PeerKid', weeks: [] }],
      accepted: [],
    }),
    'utf8',
  );
  const encrypted = Buffer.concat([cipher.update(plain), cipher.final()]);
  return Buffer.concat([nonce, encrypted, cipher.getAuthTag()]).toString('base64');
}

async function peerJoinAndExchange(inviteCode: string, peerName: string): Promise<{ inv: ParsedInvite; token: string }> {
  const inv = decodeInvite(inviteCode);
  const joinRes = await fetch(`${inv.h}/v1/links/${encodeURIComponent(inv.l)}/join`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code: inv.c }),
  });
  expect(joinRes.status).toBe(200);
  const { token } = (await joinRes.json()) as { token: string };

  const exRes = await fetch(`${inv.h}/v1/links/${encodeURIComponent(inv.l)}/exchange`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ data: sealForSideB(inv, peerName) }),
  });
  expect(exRes.status).toBe(200);
  return { inv, token };
}

test.describe('Friend invite confirmation', () => {
  test.describe.configure({ mode: 'serial' });

  let hubProc: ChildProcess | null = null;
  let hubTmpDir = '';
  let hubUrl = '';

  test.beforeAll(async () => {
    hubTmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'openchore-e2e-hub-'));
    const hubBin = path.join(hubTmpDir, 'openchore-hub');
    execFileSync('go', ['build', '-o', hubBin, './cmd/hub'], { cwd: path.resolve(__dirname, '../..'), stdio: 'pipe' });
    const port = await freePort();
    hubUrl = `http://127.0.0.1:${port}`;
    hubProc = spawn(hubBin, [], {
      env: {
        ...process.env,
        PORT: String(port),
        HUB_DB: path.join(hubTmpDir, 'hub.db'),
        HUB_KEYS: 'e2ekey',
      },
      stdio: 'ignore',
    });
    await waitForHealth(`${hubUrl}/healthz`);
  });

  test.afterAll(async ({ request }) => {
    const hdrs = await authHeaders(1);
    const cfgRes = await request.get('/api/admin/friends', { headers: hdrs });
    if (cfgRes.ok()) {
      const cfg = (await cfgRes.json()) as { links?: { id: number }[] };
      for (const link of cfg.links ?? []) {
        await request.delete(`/api/admin/friends/${link.id}`, { headers: hdrs });
      }
    }
    await request.put('/api/admin/friends', {
      headers: hdrs,
      data: { leaderboard_enabled: false, family_name: '', hub_url: '' },
    });
    if (hubProc) {
      hubProc.kill('SIGTERM');
      await new Promise((r) => hubProc?.once('exit', r));
      hubProc = null;
    }
    if (hubTmpDir) {
      fs.rmSync(hubTmpDir, { recursive: true, force: true });
    }
  });

  test('invite expires in ~24h, joining peer needs parent confirmation, and Decline cancels with 410 Gone', async ({ page }) => {
    await loginAsAdmin(page);
    await page.locator('nav button').last().click();
    const section = page.locator('section', { has: page.getByRole('heading', { name: 'Leaderboard & friend families' }) });
    await expect(section).toBeVisible({ timeout: 5_000 });

    // Configure family name (distinct from earlier test's 'The Testers' so Save is enabled) and local real hub.
    await section.getByPlaceholder('e.g. The Smiths').fill('The Home Team');
    await section.getByRole('button', { name: 'Save' }).first().click();
    await section.getByLabel('Coordination server').fill(hubUrl);
    await section.getByLabel('Key').fill('e2ekey');
    await section.getByRole('button', { name: 'Save' }).nth(1).click();
    await expect(section.getByRole('button', { name: 'Make an invite' })).toBeEnabled();

    // Create invite and verify ~24h expiry.
    await section.getByRole('button', { name: 'Make an invite' }).click();
    await expect(section.getByText('Waiting for them')).toBeVisible();
    const inviteInput = section.getByLabel('Invite', { exact: true });
    const inviteCode = await inviteInput.inputValue();
    expect(inviteCode).toMatch(/^ocf1\./);

    const adminHdrs = await authHeaders(1);
    const cfg = (await (await page.request.get('/api/admin/friends', { headers: adminHdrs })).json()) as {
      links: { id: number; invite_expires?: string }[];
    };
    expect(cfg.links.length).toBe(1);
    const ttlHours = (new Date(cfg.links[0].invite_expires!).getTime() - Date.now()) / (3600 * 1000);
    expect(ttlHours).toBeGreaterThan(23);
    expect(ttlHours).toBeLessThan(25);

    // Peer joins and exchanges on the real hub.
    const peer = await peerJoinAndExchange(inviteCode, 'Unknown Family');

    // Sync via UI -> link moves to Needs confirmation, shows peer name, hides sharing controls.
    await section.getByRole('button', { name: 'Update now' }).click();
    await expect(section.getByText('Needs confirmation')).toBeVisible();
    await expect(section.getByText(/Unknown Family used your invite/)).toBeVisible();
    await expect(section.getByRole('button', { name: 'Confirm' })).toBeVisible();
    await expect(section.getByRole('button', { name: 'Decline' })).toBeVisible();
    await expect(section.getByText(/Sent to Unknown Family/)).toHaveCount(0);
    await expect(section.getByText(/Offered by Unknown Family/)).toHaveCount(0);

    // Privacy isolation before confirmation on kid endpoints.
    const lb = await (await page.request.get('/api/leaderboard', { headers: adminHdrs })).json();
    expect(JSON.stringify(lb)).not.toContain('Unknown Family');
    expect(JSON.stringify(lb)).not.toContain('PeerKid');
    const farms = await (await page.request.get('/api/friends/farms', { headers: adminHdrs })).json();
    expect(JSON.stringify(farms)).not.toContain('Unknown Family');
    expect(JSON.stringify(farms)).not.toContain('PeerKid');

    // Decline removes link and peer gets 410 Gone from the hub.
    await section.getByRole('button', { name: 'Decline' }).click();
    await expect(section.getByText('Declined. The link has been cancelled.')).toBeVisible();
    await expect(section.getByText('Unknown Family')).toHaveCount(0);

    const goneRes = await fetch(`${peer.inv.h}/v1/links/${encodeURIComponent(peer.inv.l)}/exchange`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${peer.token}` },
      body: JSON.stringify({}),
    });
    expect(goneRes.status).toBe(410);
  });

  test('parent confirms joining family and sharing controls activate', async ({ page }) => {
    await loginAsAdmin(page);
    await page.locator('nav button').last().click();
    const section = page.locator('section', { has: page.getByRole('heading', { name: 'Leaderboard & friend families' }) });
    await expect(section).toBeVisible({ timeout: 5_000 });

    await section.getByRole('button', { name: 'Make an invite' }).click();
    await expect(section.getByText('Waiting for them')).toBeVisible();
    const inviteCode = await section.getByLabel('Invite', { exact: true }).inputValue();

    await peerJoinAndExchange(inviteCode, 'The Smiths');

    await section.getByRole('button', { name: 'Update now' }).click();
    await expect(section.getByText('Needs confirmation')).toBeVisible();
    await expect(section.getByText(/The Smiths used your invite/)).toBeVisible();

    await section.getByRole('button', { name: 'Confirm' }).click();
    await expect(section.getByText('Confirmed. You can now choose who to share.')).toBeVisible();
    await expect(section.getByText('Linked', { exact: true })).toBeVisible();
    await expect(section.getByText('Sent to The Smiths')).toBeVisible();
    await expect(section.getByText('Offered by The Smiths')).toBeVisible();
    await expect(section.getByRole('button', { name: 'Confirm' })).toHaveCount(0);
  });
});

