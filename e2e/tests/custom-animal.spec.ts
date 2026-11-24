import path from 'path';
import { test, expect } from '@playwright/test';
import { authHeaders, selectUser } from './helpers/setup';

// A kid swaps their drawn farm animal for their own picture (the example
// duck strip from the starter kit), then goes back. Farm pets are off by
// default, so this turns them on and puts everything back afterwards.
const EMMA = 3;
const STRIP = path.join(__dirname, '../../web/public/sprites/sample-duck-strip.png');

test.describe('Custom animal', () => {
  test.beforeAll(async ({ request }) => {
    const admin = await authHeaders(1);
    await request.put('/api/admin/pets', { headers: admin, data: { enabled: true } });
    await request.delete(`/api/admin/users/${EMMA}/pet`, { headers: admin });
    const adopt = await request.post('/api/me/pet', { headers: await authHeaders(EMMA), data: { name: 'Quackers', species: 'chicken' } });
    expect(adopt.ok()).toBeTruthy();
    await request.post('/api/points/adjust', { headers: admin, data: { user_id: EMMA, amount: 200, note: 'e2e custom animal' } });
  });

  test.afterAll(async ({ request }) => {
    const admin = await authHeaders(1);
    await request.delete(`/api/admin/users/${EMMA}/pet`, { headers: admin });
    await request.put('/api/admin/pets', { headers: admin, data: { enabled: false } });
  });

  test('the starter kit is served', async ({ request }) => {
    for (const file of ['blank-64.png', 'guide-layer-64.png', 'guide.png', 'sample-duck.png',
      'blank-strip.png', 'guide-layer-strip.png', 'guide-strip.png', 'sample-duck-strip.png']) {
      const resp = await request.get(`/sprites/${file}`);
      expect(resp.status(), file).toBe(200);
      expect(resp.headers()['content-type'], file).toContain('image/png');
    }
  });

  test('the starter pack is served', async ({ request }) => {
    for (const animal of ['donkey', 'duck', 'goose', 'turkey', 'rabbit', 'alpaca', 'rooster', 'mouse']) {
      const resp = await request.get(`/sprites/pack/${animal}-strip.png`);
      expect(resp.status(), animal).toBe(200);
      expect(resp.headers()['content-type'], animal).toContain('image/png');
      const body = await resp.body();
      expect(body.byteLength, animal).toBeGreaterThan(0);
      expect(body.byteLength, animal).toBeLessThanOrEqual(48 * 1024);
    }
  });

  test('a kid uses their own picture, then goes back to the drawn animal', async ({ page }) => {
    await selectUser(page, 'Emma');
    await page.getByRole('button', { name: 'Visit Quackers' }).click();
    const farm = page.getByRole('dialog', { name: 'The farm' });
    await farm.getByRole('button', { name: /Use my own picture/ }).click();

    const sheet = page.getByRole('dialog', { name: 'Use my own picture' });
    // Nothing chosen yet: can't confirm.
    await expect(sheet.getByRole('button', { name: /Get it for/ })).toBeDisabled();
    await sheet.locator('input[type=file]').setInputFiles(STRIP);
    await expect(sheet.getByText('6 pictures found in a row.')).toBeVisible();
    await expect(sheet.getByText('Very sick')).toBeVisible();
    await sheet.getByRole('button', { name: /Get it for/ }).click();
    await expect(sheet).toBeHidden();

    // The server kept all six frames, and the farm draws the picture.
    const mine = await (await page.request.get('/api/me/pet', { headers: await authHeaders(EMMA) })).json();
    expect(mine.pet.sprite).toMatch(/^[a-f0-9]{16,64}$/);
    expect(mine.pet.sprite_frames).toBe(6);
    await expect(farm.locator(`image[href="/uploads/pets/${mine.pet.sprite}.png"]`).first()).toBeAttached();

    // Going back is free and asks twice.
    const back = farm.getByRole('button', { name: 'Go back to the drawn animal' });
    await back.click();
    await farm.getByRole('button', { name: /Really\?/ }).click();
    await expect(back).toHaveCount(0);
    const after = await (await page.request.get('/api/me/pet', { headers: await authHeaders(EMMA) })).json();
    expect(after.pet.sprite ?? '').toBe('');
  });

  test('a kid picks a starter-pack animal, sees it on the farm, and can go back', async ({ page }) => {
    await selectUser(page, 'Emma');
    await page.getByRole('button', { name: 'Visit Quackers' }).click();
    const farm = page.getByRole('dialog', { name: 'The farm' });
    await farm.getByRole('button', { name: /Use my own picture/ }).click();

    const sheet = page.getByRole('dialog', { name: 'Use my own picture' });
    const confirm = sheet.getByRole('button', { name: /Get it for/ });
    await expect(confirm).toBeDisabled();

    const alpaca = sheet.getByRole('radio', { name: 'Alpaca' });
    await alpaca.click();
    await expect(alpaca).toHaveAttribute('aria-checked', 'true');
    await expect(sheet.getByText('6 pictures found in a row.')).toBeVisible();
    await expect(sheet.getByText('Very sick')).toBeVisible();
    await expect(confirm).toBeEnabled();

    await confirm.click();
    await expect(sheet).toBeHidden();

    const mine = await (await page.request.get('/api/me/pet', { headers: await authHeaders(EMMA) })).json();
    expect(mine.pet.sprite).toMatch(/^[a-f0-9]{16,64}$/);
    expect(mine.pet.sprite_frames).toBe(6);
    await expect(farm.locator(`image[href="/uploads/pets/${mine.pet.sprite}.png"]`).first()).toBeAttached();

    // Clean revert back to the drawn animal.
    const back = farm.getByRole('button', { name: 'Go back to the drawn animal' });
    await back.click();
    await farm.getByRole('button', { name: /Really\?/ }).click();
    await expect(back).toHaveCount(0);
    const after = await (await page.request.get('/api/me/pet', { headers: await authHeaders(EMMA) })).json();
    expect(after.pet.sprite ?? '').toBe('');
  });
});
