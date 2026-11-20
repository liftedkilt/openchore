// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { currentFarmSeason, seasonOverride } from './season';

const july = new Date(2026, 6, 4, 12);
const december = new Date(2026, 11, 10, 12);
const visit = (url: string) => window.history.replaceState(null, '', url.startsWith('/') ? url : `/${url}`);

describe('season preview override', () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    visit('');
  });
  afterEach(() => vi.restoreAllMocks());

  it('follows the calendar with no override', () => {
    expect(seasonOverride()).toBeUndefined();
    expect(currentFarmSeason(july)).toBeNull();
    expect(currentFarmSeason(december)).toBe('christmas');
  });

  it('forces the season on or off from the URL', () => {
    visit('?season=christmas');
    expect(currentFarmSeason(july)).toBe('christmas');
    visit('?season=none');
    expect(currentFarmSeason(december)).toBeNull();
  });

  it('remembers the override for the tab after navigating away', () => {
    visit('?season=christmas');
    seasonOverride();
    visit('/chores');
    expect(currentFarmSeason(july)).toBe('christmas');
  });

  it('auto clears the remembered override', () => {
    visit('?season=christmas');
    seasonOverride();
    visit('?season=auto');
    expect(currentFarmSeason(july)).toBeNull();
    visit('');
    expect(currentFarmSeason(july)).toBeNull();
  });

  it('ignores unknown values and keeps the remembered one', () => {
    visit('?season=christmas');
    seasonOverride();
    visit('?season=easter');
    expect(currentFarmSeason(july)).toBe('christmas');
  });

  it('still honours the URL when storage is blocked', () => {
    const blocked = () => { throw new DOMException('blocked', 'SecurityError'); };
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(blocked);
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(blocked);
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(blocked);
    visit('?season=christmas');
    expect(currentFarmSeason(july)).toBe('christmas');
    visit('?season=auto');
    expect(currentFarmSeason(december)).toBe('christmas');
    visit('');
    expect(currentFarmSeason(july)).toBeNull();
  });
});
