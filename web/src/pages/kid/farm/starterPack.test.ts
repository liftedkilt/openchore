import { describe, it, expect, beforeAll } from 'vitest';
import i18n from '../../../i18n';
import { STARTER_PACK, STARTER_PACK_IDS } from './starterPack';

beforeAll(async () => {
  if (!i18n.isInitialized) await i18n.init();
});

describe('STARTER_PACK manifest', () => {
  it('defines the 8 unique starter-pack animals with 6-frame strip URLs', () => {
    expect(STARTER_PACK_IDS).toEqual([
      'donkey',
      'duck',
      'goose',
      'turkey',
      'rabbit',
      'alpaca',
      'rooster',
      'mouse',
    ]);
    expect(STARTER_PACK).toHaveLength(8);
    expect(new Set(STARTER_PACK.map(a => a.id)).size).toBe(8);

    for (const animal of STARTER_PACK) {
      expect(animal.frames).toBe(6);
      expect(animal.stripUrl).toBe(`/sprites/pack/${animal.id}-strip.png`);
      expect(animal.labelKey).toBe(`kid.farm.picture.pack.${animal.id}`);
    }
  });

  it('has complete EN and DE translations for every animal and picker string', () => {
    const en = i18n.getFixedT('en');
    const de = i18n.getFixedT('de');

    const uiKeys = [
      'kid.farm.picture.lead',
      'kid.farm.picture.packTitle',
      'kid.farm.picture.packLead',
      'kid.farm.picture.packError',
      'kid.farm.picture.uploadTitle',
    ] as const;

    for (const key of uiKeys) {
      const enVal = en(key);
      const deVal = de(key);
      expect(enVal, `en:${key}`).toBeTruthy();
      expect(enVal, `en:${key}`).not.toBe(key);
      expect(deVal, `de:${key}`).toBeTruthy();
      expect(deVal, `de:${key}`).not.toBe(key);
    }

    for (const animal of STARTER_PACK) {
      const enLabel = en(animal.labelKey);
      const deLabel = de(animal.labelKey);
      expect(enLabel, `en:${animal.labelKey}`).toBeTruthy();
      expect(enLabel, `en:${animal.labelKey}`).not.toBe(animal.labelKey);
      expect(deLabel, `de:${animal.labelKey}`).toBeTruthy();
      expect(deLabel, `de:${animal.labelKey}`).not.toBe(animal.labelKey);
    }
  });
});
