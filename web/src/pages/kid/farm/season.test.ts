import { describe, expect, it } from 'vitest';
import { farmSeason, isChristmasSeason, parseSeasonParam } from './season';

const at = (y: number, m: number, d: number, h = 12) => new Date(y, m - 1, d, h);

describe('isChristmasSeason', () => {
  it('starts five weeks before Christmas', () => {
    expect(isChristmasSeason(at(2026, 11, 19, 23))).toBe(false);
    expect(isChristmasSeason(at(2026, 11, 20, 0))).toBe(true);
  });

  it('covers December', () => {
    expect(isChristmasSeason(at(2026, 12, 1))).toBe(true);
    expect(isChristmasSeason(at(2026, 12, 25))).toBe(true);
    expect(isChristmasSeason(at(2026, 12, 31, 23))).toBe(true);
  });

  it('ends one week after Christmas, across the new year', () => {
    expect(isChristmasSeason(at(2027, 1, 1, 23))).toBe(true);
    expect(isChristmasSeason(at(2027, 1, 2, 0))).toBe(false);
  });

  it('is off the rest of the year', () => {
    expect(isChristmasSeason(at(2026, 7, 4))).toBe(false);
    expect(isChristmasSeason(at(2026, 10, 31))).toBe(false);
    expect(isChristmasSeason(at(2027, 2, 14))).toBe(false);
  });

  it('works in a leap year', () => {
    expect(isChristmasSeason(at(2028, 11, 19))).toBe(false);
    expect(isChristmasSeason(at(2028, 11, 20))).toBe(true);
  });
});

describe('farmSeason', () => {
  it('names the season', () => {
    expect(farmSeason(at(2026, 12, 10))).toBe('christmas');
    expect(farmSeason(at(2026, 6, 10))).toBeNull();
  });
});

describe('parseSeasonParam', () => {
  it('accepts the preview values in any case', () => {
    expect(parseSeasonParam('christmas')).toBe('christmas');
    expect(parseSeasonParam(' Christmas ')).toBe('christmas');
    expect(parseSeasonParam('none')).toBe('none');
    expect(parseSeasonParam('AUTO')).toBe('auto');
  });

  it('ignores anything else', () => {
    expect(parseSeasonParam(null)).toBeUndefined();
    expect(parseSeasonParam('')).toBeUndefined();
    expect(parseSeasonParam('easter')).toBeUndefined();
  });
});
