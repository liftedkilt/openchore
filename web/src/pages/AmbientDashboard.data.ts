// Row selection for the wall display's doors: which of a person's chores
// show under "Next up", in what order and with what meta line. Pure, so it
// can be unit-tested without the page.
import type { ScheduledChore } from '../types';
import { catFromCategory, type Cat, type ChoreState } from '../design';
import { dayGates, isChoreDone } from '../choreRules';

export const CAT_ORDER: Record<Cat, number> = { essential: 0, daily: 1, bonus: 2 };

/** What the meta line says: a deadline, an opening time or a missed deadline. */
export type NextMeta =
  | { kind: 'due'; at: Date }
  | { kind: 'opens'; at: Date }
  | { kind: 'late'; at: Date }
  | null;

export interface NextRow {
  chore: ScheduledChore;
  cat: Cat;
  state: ChoreState;
  meta: NextMeta;
  urgent: boolean;
}

/** "HH:MM" on the same day as `now`, as a Date. */
export function atToday(hhmm: string | undefined, now: Date): Date | null {
  if (!hhmm) return null;
  const [h, m] = hhmm.split(':').map(Number);
  if (!Number.isFinite(h) || !Number.isFinite(m)) return null;
  const d = new Date(now);
  d.setHours(h, m, 0, 0);
  return d;
}


const HOUR = 60 * 60 * 1000;

/**
 * Everything a person still has to do today, most useful first:
 * 1. what they can do now (a deadline within the hour is urgent),
 * 2. what is late but can still be done (expired without the `block` penalty),
 * 3. what opens later today, with its "From …" time,
 * 4. locked bonus chores.
 * Done chores (see isChoreDone: a rejected chore is not done) and expired
 * chores that can no longer be completed are left out. Bonus locks by the
 * same dayGates rule as the kid's Today screen.
 * Within a group: Must do, Every day, Bonus, then by time.
 */
export function nextRows(chores: ScheduledChore[], now: Date): NextRow[] {
  const open = dayGates(chores).bonusOpen;
  const rows: (NextRow & { group: number; sortAt: number })[] = [];
  for (const c of chores) {
    if (isChoreDone(c)) continue;
    if (c.expired && c.expiry_penalty === 'block') continue;
    const cat = catFromCategory(c.category);
    const locked = cat === 'bonus' && !open;
    const opens = atToday(c.available_at, now);
    const due = atToday(c.due_by, now);
    let group: number;
    let meta: NextMeta = null;
    let urgent = false;
    if (locked) {
      group = 3;
    } else if (!c.available && opens && opens > now) {
      group = 2;
      meta = { kind: 'opens', at: opens };
    } else if (c.expired) {
      group = 1;
      urgent = true;
      meta = due ? { kind: 'late', at: due } : null;
    } else {
      group = 0;
      if (due) {
        meta = { kind: 'due', at: due };
        urgent = due.getTime() - now.getTime() < HOUR;
      }
    }
    const sortAt = (meta?.at ?? due ?? opens)?.getTime() ?? Number.MAX_SAFE_INTEGER;
    rows.push({ chore: c, cat, state: locked ? 'locked' : 'todo', meta, urgent, group, sortAt });
  }
  rows.sort((a, b) => a.group - b.group || CAT_ORDER[a.cat] - CAT_ORDER[b.cat] || a.sortAt - b.sortAt);
  return rows.map(({ group: _g, sortAt: _s, ...r }) => r);
}
