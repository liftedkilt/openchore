import { isSkin, type HouseMode, type HouseTheme, type Skin } from './types';

/** Legacy `users.theme` values, mapped the same way as migration 017. */
const LEGACY_SKIN: Record<string, Skin> = {
  default: 'sunroom',
  quest: 'blocks',
  galaxy: 'tint',
  forest: 'sunroom',
};

/** Age under which an unset skin resolves to Blocks. */
export const BLOCKS_UNDER_AGE = 8;

/**
 * The skin a person's own screens render in.
 * A stored skin wins; legacy names map to their new skin; an empty theme
 * resolves by age (under 8 → Blocks, otherwise Sunroom).
 */
export function resolveSkin(theme: string | null | undefined, age?: number | null): Skin {
  if (isSkin(theme)) return theme;
  if (theme && LEGACY_SKIN[theme]) return LEGACY_SKIN[theme];
  if (typeof age === 'number' && Number.isFinite(age) && age > 0 && age < BLOCKS_UNDER_AGE) return 'blocks';
  return 'sunroom';
}

/** Shared devices go dark from 19:30 until 07:00, local time. */
export const DARK_FROM_MINUTES = 19 * 60 + 30;
export const DARK_UNTIL_MINUTES = 7 * 60;

/** Whether the House schedule is in its dark window at `now` (local time). */
export function isScheduledDark(now: Date): boolean {
  const m = now.getHours() * 60 + now.getMinutes();
  return m >= DARK_FROM_MINUTES || m < DARK_UNTIL_MINUTES;
}

export interface HouseThemeInput {
  mode: HouseMode;
  /** A personal (persistent / OIDC) session: follow the system setting. */
  persistent?: boolean;
  /** `prefers-color-scheme: dark`, only consulted for persistent sessions. */
  prefersDark?: boolean;
  /** The current local time, only consulted for shared sessions. */
  now: Date;
}

/**
 * Pick House or House Dark.
 * - `light` / `dark` force a mode.
 * - `auto` on a personal device follows `prefers-color-scheme`.
 * - `auto` on a shared device (tap/PIN session, ambient display) follows the
 *   schedule: dark 19:30 – 07:00.
 */
export function resolveHouseTheme({ mode, persistent, prefersDark, now }: HouseThemeInput): HouseTheme {
  if (mode === 'light') return 'house';
  if (mode === 'dark') return 'house-dark';
  if (persistent) return prefersDark ? 'house-dark' : 'house';
  return isScheduledDark(now) ? 'house-dark' : 'house';
}

/** Milliseconds until the next wall-clock minute, for re-evaluating the schedule. */
export function msUntilNextMinute(now: Date): number {
  return 60_000 - (now.getSeconds() * 1000 + now.getMilliseconds());
}

export type Salutation = 'morning' | 'afternoon' | 'evening';

/** The greeting for a time of day: morning before 12, afternoon before 18. */
export function salutationFor(now: Date): Salutation {
  const h = now.getHours();
  if (h < 12) return 'morning';
  if (h < 18) return 'afternoon';
  return 'evening';
}
