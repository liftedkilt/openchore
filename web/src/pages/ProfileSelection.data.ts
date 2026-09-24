// Data for the family picker (/login). Everything here comes from the public,
// read-only endpoints the wall display (/ambient) already uses without a
// session: /users, /users/{id}/chores and /users/{id}/points.
import { useCallback, useEffect, useState } from 'react';
import { fetchPublicUserData } from '../api';
import { CATS, catFromCategory, type DayProgressItem } from '../design';
import type { PointsData, ScheduledChore, User } from '../types';
import { localDateStr } from '../utils';

/** The day the doors' sun arc spans, in hours (the arc reads "7 am" – "9 pm"). */
export const DAY_FROM_HOUR = 7;
export const DAY_TO_HOUR = 21;

/** Where `d` falls in the day's window, 0–1 (clamped). */
export function dayFraction(d: Date, from = DAY_FROM_HOUR, to = DAY_TO_HOUR): number {
  const h = d.getHours() + d.getMinutes() / 60 + d.getSeconds() / 3600;
  return Math.min(1, Math.max(0, (h - from) / (to - from)));
}

export interface PersonToday {
  /** One per chore today, for DayProgress. */
  items: DayProgressItem[];
  done: number;
  total: number;
  /** Point balance, when it could be read. */
  balance: number | null;
}

/** Summarise one person's chores for today, in category order (Must do, Every day, Bonus). */
export function summarizeDay(chores: ScheduledChore[], balance: number | null = null): PersonToday {
  const rank = (x: DayProgressItem) => CATS.indexOf(x.cat);
  const items = chores.map((c): DayProgressItem => {
    let at: number | undefined;
    if (c.completed && c.completed_at) {
      const when = new Date(c.completed_at);
      if (!Number.isNaN(when.getTime())) at = dayFraction(when);
    }
    return { cat: catFromCategory(c.category), done: c.completed, at };
  }).sort((a, b) => rank(a) - rank(b));
  const done = items.filter((x) => x.done).length;
  return { items, done, total: items.length, balance };
}

/**
 * Who appears where, in the family's order (the same order the picker has
 * always used: kids first, then grown-ups, each by name).
 */
export function orderFamily(users: User[]): { kids: User[]; parents: User[] } {
  const byName = (a: User, b: User) => a.name.localeCompare(b.name);
  return {
    kids: users.filter((u) => u.role === 'child').sort(byName),
    parents: users.filter((u) => u.role === 'admin').sort(byName),
  };
}

/**
 * The family strip: everyone not paused who has chores today. Parents take
 * part, so they show whenever they have chores of their own.
 */
export function stripMembers(ordered: User[], today: Record<number, PersonToday>): User[] {
  return ordered.filter((u) => !u.paused && (today[u.id]?.total ?? 0) > 0);
}

const REFRESH_MS = 60_000;

/**
 * Today's progress and balance for each person, refreshed every minute while
 * the picker is open. Paused people are skipped (they have nothing today).
 */
export function useFamilyToday(users: User[]): Record<number, PersonToday> {
  const [today, setToday] = useState<Record<number, PersonToday>>({});

  const load = useCallback(async () => {
    const date = localDateStr(new Date());
    const people = users.filter((u) => !u.paused);
    const results = await Promise.allSettled(
      people.map(async (u) => {
        const [chores, points] = await Promise.all([
          fetchPublicUserData<ScheduledChore[]>(`/users/${u.id}/chores?view=daily&date=${date}`),
          fetchPublicUserData<PointsData>(`/users/${u.id}/points`).catch(() => null),
        ]);
        return [u.id, summarizeDay(chores, points?.balance ?? null)] as const;
      }),
    );
    const next: Record<number, PersonToday> = {};
    for (const r of results) {
      if (r.status === 'fulfilled') next[r.value[0]] = r.value[1];
    }
    setToday(next);
  }, [users]);

  useEffect(() => {
    if (users.length === 0) return;
    load();
    const timer = setInterval(load, REFRESH_MS);
    return () => clearInterval(timer);
  }, [users, load]);

  return today;
}
