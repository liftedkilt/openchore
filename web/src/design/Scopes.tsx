import React, { useEffect, useState, useSyncExternalStore } from 'react';
import clsx from 'clsx';
import { msUntilNextMinute, resolveHouseTheme } from './theme';
import type { HouseMode, HouseTheme, PersonColor, Skin } from './types';

type DivProps = Omit<React.HTMLAttributes<HTMLDivElement>, 'color'>;

export interface SkinScopeProps extends DivProps {
  /** The person's skin. Use resolveSkin(user.theme, user.age). */
  skin: Skin;
  /** The person's colour. In Tint it leads (accent, ring, done check). */
  color?: PersonColor | null;
  /** Render Tint's glow in the person's colour (dial-controlled). Default true. */
  glow?: boolean;
  /**
   * This scope is a "door" inside a House frame (the family picker): it dims
   * with House Dark so no skin glares at night.
   */
  door?: boolean;
  children?: React.ReactNode;
}

/**
 * A person's own screen root (or a door on a shared screen). Writes
 * `data-theme` and `data-person` together so the skin's dials and tokens, and
 * Tint's person lead, apply to everything inside.
 */
export function SkinScope({ skin, color, glow = true, door, className, children, ...rest }: SkinScopeProps) {
  return (
    <div
      {...rest}
      data-theme={skin}
      data-person={color || undefined}
      className={clsx('oc-scope', door && 'oc-scope--door', className)}
    >
      {glow && <div className="oc-glow" aria-hidden />}
      {children}
    </div>
  );
}

/* ---------------- House ---------------- */

const DARK_QUERY = '(prefers-color-scheme: dark)';

function subscribeDark(cb: () => void) {
  if (typeof window === 'undefined' || !window.matchMedia) return () => {};
  const mq = window.matchMedia(DARK_QUERY);
  mq.addEventListener('change', cb);
  return () => mq.removeEventListener('change', cb);
}
const getDark = () => typeof window !== 'undefined' && !!window.matchMedia && window.matchMedia(DARK_QUERY).matches;

/** Whether the system prefers a dark colour scheme (live). */
export function usePrefersDark(): boolean {
  return useSyncExternalStore(subscribeDark, getDark, () => false);
}

/** The current time, updated on every wall-clock minute. */
export function useMinuteClock(enabled = true): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setTimeout>;
    const tick = () => {
      const d = new Date();
      setNow(d);
      timer = setTimeout(tick, msUntilNextMinute(d) + 50);
    };
    timer = setTimeout(tick, msUntilNextMinute(new Date()) + 50);
    return () => clearTimeout(timer);
  }, [enabled]);
  return now;
}

/**
 * House or House Dark for shared screens.
 * - mode `light` / `dark` force it.
 * - `auto` + persistent (personal / OIDC session) follows prefers-color-scheme.
 * - `auto` otherwise (shared device, ambient display) follows the schedule:
 *   dark 19:30 – 07:00 local time, re-evaluated each minute.
 */
export function useHouseTheme(mode: HouseMode = 'auto', persistent = false): HouseTheme {
  const prefersDark = usePrefersDark();
  const now = useMinuteClock(mode === 'auto' && !persistent);
  return resolveHouseTheme({ mode, persistent, prefersDark, now });
}

export interface HouseScopeProps extends DivProps {
  mode?: HouseMode;
  /** A personal (persistent) session: follow the system setting. */
  persistent?: boolean;
  children?: React.ReactNode;
}

/** The neutral frame for shared screens: picker, ambient, admin, reports. */
export function HouseScope({ mode = 'auto', persistent = false, className, children, ...rest }: HouseScopeProps) {
  const theme = useHouseTheme(mode, persistent);
  return (
    <div {...rest} data-theme={theme} className={clsx('oc-scope', className)}>
      {children}
    </div>
  );
}
