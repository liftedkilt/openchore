// Seasonal dressing for the farm scene. The season follows the device's own
// calendar date, so the farm turns wintry at local midnight wherever it runs.
import { useEffect, useState } from 'react';

export type FarmSeason = 'christmas' | null;

const DAY_MS = 24 * 60 * 60 * 1000;
/** Christmas season: five weeks before Christmas Day to one week after. */
const CHRISTMAS_BEFORE_DAYS = 35;
const CHRISTMAS_AFTER_DAYS = 7;

/** The local calendar day as a UTC day number, so DST never shifts it. */
function dayNumber(year: number, month: number, day: number): number {
  return Date.UTC(year, month, day) / DAY_MS;
}

/** Whether `date` (local time) falls in the Christmas season, both ends included. */
export function isChristmasSeason(date: Date): boolean {
  const today = dayNumber(date.getFullYear(), date.getMonth(), date.getDate());
  // Early January belongs to the previous year's Christmas.
  return [date.getFullYear(), date.getFullYear() - 1].some(year => {
    const christmas = dayNumber(year, 11, 25);
    return today >= christmas - CHRISTMAS_BEFORE_DAYS && today <= christmas + CHRISTMAS_AFTER_DAYS;
  });
}

export function farmSeason(date: Date = new Date()): FarmSeason {
  return isChristmasSeason(date) ? 'christmas' : null;
}

/**
 * Preview override, for checking seasonal art on any day: add `?season=christmas`
 * (force on), `?season=none` (force off) or `?season=auto` (back to the
 * calendar) to any page URL. The choice is kept for the browser tab, so moving
 * around the app keeps it.
 */
export const SEASON_PARAM = 'season';
const SEASON_STORAGE_KEY = 'openchore.farmSeason';
type SeasonOverride = 'christmas' | 'none';

/** Parses a `season` URL value: an override, 'auto' to clear it, or undefined if not recognised. */
export function parseSeasonParam(value: string | null): SeasonOverride | 'auto' | undefined {
  const v = value?.trim().toLowerCase();
  return v === 'christmas' || v === 'none' || v === 'auto' ? v : undefined;
}

/** Remembers (or with no value, forgets) the override for this tab. Best effort: storage can be blocked. */
function rememberOverride(value: SeasonOverride | undefined): void {
  try {
    if (value) window.sessionStorage.setItem(SEASON_STORAGE_KEY, value);
    else window.sessionStorage.removeItem(SEASON_STORAGE_KEY);
  } catch {
    // Blocked storage (embedded views, blocked site data, old private modes): the URL still applies.
  }
}

function storedOverride(): SeasonOverride | undefined {
  try {
    const stored = parseSeasonParam(window.sessionStorage.getItem(SEASON_STORAGE_KEY));
    return stored === 'auto' ? undefined : stored;
  } catch {
    return undefined;
  }
}

/** Reads the override from the URL (remembering it for the tab) or from the tab's memory. */
export function seasonOverride(): SeasonOverride | undefined {
  if (typeof window === 'undefined') return undefined;
  // The URL always wins, even when it can't be remembered.
  const fromUrl = parseSeasonParam(new URLSearchParams(window.location.search).get(SEASON_PARAM));
  if (fromUrl === 'auto') {
    rememberOverride(undefined);
    return undefined;
  }
  if (fromUrl) {
    rememberOverride(fromUrl);
    return fromUrl;
  }
  return storedOverride();
}

/** The season to draw now: the preview override if set, else the calendar. */
export function currentFarmSeason(date: Date = new Date()): FarmSeason {
  const forced = seasonOverride();
  if (forced) return forced === 'christmas' ? 'christmas' : null;
  return farmSeason(date);
}

/** The current season, rechecked hourly and on wake so an always-on dashboard turns over on its own. */
export function useFarmSeason(): FarmSeason {
  const [season, setSeason] = useState<FarmSeason>(() => currentFarmSeason());
  useEffect(() => {
    const recheck = () => setSeason(currentFarmSeason());
    const id = setInterval(recheck, 60 * 60 * 1000);
    // Timers are paused while a tablet sleeps, so also recheck the moment the screen comes back.
    const onVisible = () => { if (document.visibilityState === 'visible') recheck(); };
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      clearInterval(id);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, []);
  return season;
}
