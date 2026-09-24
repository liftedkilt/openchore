import { describe, it, expect } from 'vitest';
import { resolveSkin, resolveHouseTheme, isScheduledDark, msUntilNextMinute, salutationFor } from './theme';

const at = (h: number, m: number, s = 0) => new Date(2026, 8, 24, h, m, s);

describe('resolveSkin', () => {
  it('keeps a stored skin', () => {
    expect(resolveSkin('sunroom', 5)).toBe('sunroom');
    expect(resolveSkin('blocks', 12)).toBe('blocks');
    expect(resolveSkin('tint')).toBe('tint');
  });

  it('resolves an empty theme by age', () => {
    expect(resolveSkin('', 5)).toBe('blocks');
    expect(resolveSkin(undefined, 7)).toBe('blocks');
    expect(resolveSkin(null, 8)).toBe('sunroom');
    expect(resolveSkin('', 13)).toBe('sunroom');
  });

  it('defaults to Sunroom with no age', () => {
    expect(resolveSkin('')).toBe('sunroom');
    expect(resolveSkin(undefined, null)).toBe('sunroom');
    expect(resolveSkin('', 0)).toBe('sunroom');
  });

  it('maps legacy theme names', () => {
    expect(resolveSkin('default', 5)).toBe('sunroom');
    expect(resolveSkin('quest')).toBe('blocks');
    expect(resolveSkin('galaxy')).toBe('tint');
    expect(resolveSkin('forest')).toBe('sunroom');
  });

  it('never resolves to House', () => {
    expect(resolveSkin('house', 5)).toBe('blocks');
    expect(resolveSkin('house-dark', 10)).toBe('sunroom');
  });
});

describe('House schedule', () => {
  it.each([
    [6, 59, true],
    [7, 0, false],
    [12, 0, false],
    [19, 29, false],
    [19, 30, true],
    [23, 59, true],
    [0, 0, true],
  ])('%i:%i dark=%s', (h, m, dark) => {
    expect(isScheduledDark(at(h, m))).toBe(dark);
  });

  it('forces light and dark', () => {
    expect(resolveHouseTheme({ mode: 'light', now: at(22, 0) })).toBe('house');
    expect(resolveHouseTheme({ mode: 'dark', now: at(12, 0) })).toBe('house-dark');
  });

  it('follows the schedule on shared devices', () => {
    expect(resolveHouseTheme({ mode: 'auto', now: at(16, 15), prefersDark: true })).toBe('house');
    expect(resolveHouseTheme({ mode: 'auto', now: at(20, 40), prefersDark: false })).toBe('house-dark');
  });

  it('follows the system setting on personal devices', () => {
    expect(resolveHouseTheme({ mode: 'auto', persistent: true, prefersDark: true, now: at(12, 0) })).toBe('house-dark');
    expect(resolveHouseTheme({ mode: 'auto', persistent: true, prefersDark: false, now: at(22, 0) })).toBe('house');
  });

  it('re-evaluates on the next minute', () => {
    expect(msUntilNextMinute(at(19, 29, 45))).toBe(15_000);
    expect(msUntilNextMinute(at(19, 29, 0))).toBe(60_000);
  });
});

describe('salutationFor', () => {
  it('greets by time of day', () => {
    expect(salutationFor(at(8, 0))).toBe('morning');
    expect(salutationFor(at(12, 0))).toBe('afternoon');
    expect(salutationFor(at(18, 0))).toBe('evening');
  });
});
