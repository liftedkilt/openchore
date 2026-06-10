import { describe, it, expect, beforeAll } from 'vitest';
import i18n from './index';

beforeAll(async () => {
  if (!i18n.isInitialized) {
    await i18n.init();
  }
});

describe('i18n configuration', () => {
  it('resolves a known key to its English string', () => {
    expect(i18n.t('common.back')).toBe('Back');
  });

  it('falls back to English for an unsupported language', async () => {
    await i18n.changeLanguage('zz-ZZ');
    expect(i18n.resolvedLanguage).toBe('en');
    expect(i18n.t('common.save')).toBe('Save');
  });

  it('interpolates variables', () => {
    expect(i18n.t('test.greeting', { name: 'Sam' })).toBe('Hi, Sam');
  });

  it('selects singular vs plural', () => {
    expect(i18n.t('test.points', { count: 1 })).toBe('1 point');
    expect(i18n.t('test.points', { count: 3 })).toBe('3 points');
  });
});
