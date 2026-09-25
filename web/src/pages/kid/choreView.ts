// Maps a ScheduledChore from the API onto the design's ChoreRow: its state,
// meta line and whether the check may be tapped. Pure, so every chore state
// the old dashboard handled is covered by choreView.test.ts.
import type { TFunction } from 'i18next';
import { catFromCategory, type Cat, type ChoreState } from '../../design';
import type { ScheduledChore } from '../../types';

/** Minutes before `due_by` at which a countdown turns urgent. */
export const URGENT_MINUTES = 60;
/** How far ahead a not-yet-open chore shows a countdown instead of a time. */
export const COUNTDOWN_MINUTES = 120;

export interface ChoreViewContext {
  now: Date;
  /** Today, YYYY-MM-DD local. */
  today: string;
  /** Every Must do and Every day chore today is done. */
  bonusOpen: boolean;
  /** Every Must do chore today is done (Every day points pay out). */
  requiredDone: boolean;
  t: TFunction;
  lang?: string;
}

export interface ChoreView {
  cat: Cat;
  state: ChoreState;
  meta?: string;
  urgent: boolean;
  /** Shows a camera in the meta line. */
  photo: boolean;
  /** Appended to the meta as "· 10 pts". */
  points?: number;
  /** The check may be tapped (today, and not closed). */
  canToggle: boolean;
  /** Override of the check's accessible name. */
  checkLabel?: string;
  /** Rejected by a grown-up: a tap clears the old try and starts again. */
  retry: boolean;
  /** Done, but a photo still has to be added from another device. */
  needsPhotoProof: boolean;
}

/** "HH:MM" today as a Date. */
export function atTime(hhmm: string, now: Date): Date {
  const [h, m] = hhmm.split(':').map(Number);
  const d = new Date(now);
  d.setHours(h || 0, m || 0, 0, 0);
  return d;
}

export function formatClock(d: Date, lang?: string): string {
  return d.toLocaleTimeString(lang, { hour: 'numeric', minute: '2-digit' });
}

/** "44 min" / "2 h 5 min" */
export function formatDuration(minutes: number, t: TFunction): string {
  const m = Math.max(1, Math.ceil(minutes));
  if (m < 60) return t('kid.time.minutes', { count: m });
  const h = Math.floor(m / 60);
  const rest = m % 60;
  return rest ? t('kid.time.hoursMinutes', { h, m: rest }) : t('kid.time.hours', { count: h });
}

// The Bonus / Every day gates are shared with the wall display.
export { isChoreDone as isDone, dayGates as gates } from '../../choreRules';

export function choreView(c: ScheduledChore, ctx: ChoreViewContext): ChoreView {
  const { t, now, lang } = ctx;
  const cat = catFromCategory(c.category);
  const isToday = c.date === ctx.today;
  const photo = !!c.requires_photo && (c.photo_source ?? 'child') !== 'external';
  const base: ChoreView = {
    cat, state: 'todo', urgent: false, photo, canToggle: isToday, retry: false, needsPhotoProof: false,
  };

  const doneAt = c.completed_at ? new Date(c.completed_at) : null;
  const doneAtLine = doneAt && !Number.isNaN(doneAt.getTime())
    ? t('kid.chore.doneAt', { time: formatClock(doneAt, lang) })
    : t('kid.chore.done');

  // --- Finished in some way ---
  if (c.completed) {
    base.needsPhotoProof = isToday && !!c.requires_photo &&
      (c.photo_source === 'both' || c.photo_source === 'external') && !c.photo_url;
    if (c.completed_by_sibling) {
      return { ...base, state: 'done', meta: c.completed_by_name ? t('kid.chore.doneBy', { name: c.completed_by_name }) : doneAtLine };
    }
    switch (c.completion_status) {
      case 'pending':
        return { ...base, state: 'waiting' };
      case 'rejected':
        return {
          ...base, state: 'todo', urgent: true, meta: t('kid.chore.rejected'), retry: true,
          checkLabel: t('design.chore.markComplete'), points: c.points_value,
        };
      case 'excused':
        return { ...base, state: 'done', meta: t('kid.chore.excused') };
      default:
        if (isToday && c.category === 'core' && !ctx.requiredDone) {
          return { ...base, state: 'done', meta: t('kid.chore.pointsPending', { count: c.points_value }) };
        }
        return { ...base, state: 'done', meta: doneAtLine };
    }
  }

  // --- Another day (the Week tab): read-only ---
  if (!isToday) {
    const span = timeWindow(c, now, t, lang);
    if (c.date < ctx.today) return { ...base, canToggle: false, meta: t('kid.chore.notDone') };
    return { ...base, canToggle: false, meta: span, points: c.points_value };
  }

  // --- Missed its window ---
  if (c.expired) {
    const closed = c.due_by ? formatClock(atTime(c.due_by, now), lang) : '';
    if (c.expiry_penalty === 'block') {
      return {
        ...base, state: 'locked', canToggle: false, photo: false,
        meta: closed ? t('kid.chore.closedAt', { time: closed }) : t('kid.chore.closed'),
        checkLabel: t('kid.chore.closedLabel'),
      };
    }
    const meta = c.expiry_penalty === 'penalty'
      ? t('kid.chore.latePenalty', { count: c.expiry_penalty_value })
      : t('kid.chore.lateNoPoints');
    return { ...base, urgent: true, meta };
  }

  // --- Not open yet ---
  if (!c.available) {
    const opens = c.available_at ? atTime(c.available_at, now) : null;
    const mins = opens ? (opens.getTime() - now.getTime()) / 60000 : Infinity;
    const meta = opens && mins > 0 && mins <= COUNTDOWN_MINUTES
      ? t('kid.chore.opensIn', { time: formatDuration(mins, t) })
      : opens ? t('kid.chore.opensAt', { time: formatClock(opens, lang) }) : t('kid.chore.later');
    return {
      ...base, state: 'locked', canToggle: false, photo: false, meta,
      checkLabel: opens ? t('kid.chore.opensAtLabel', { time: formatClock(opens, lang) }) : t('kid.chore.later'),
    };
  }

  // --- Bonus waits for everything else ---
  if (c.category === 'bonus' && !ctx.bonusOpen) {
    return { ...base, state: 'locked', canToggle: false, photo: false };
  }

  // --- To do ---
  if (c.due_by) {
    const due = atTime(c.due_by, now);
    const left = (due.getTime() - now.getTime()) / 60000;
    if (left > 0 && left <= URGENT_MINUTES) {
      return { ...base, urgent: true, meta: t('kid.chore.left', { time: formatDuration(left, t) }), points: c.points_value };
    }
  }
  let meta = timeWindow(c, now, t, lang);
  if (!meta && photo) meta = t('design.chore.photo');
  if (!meta && c.estimated_minutes) meta = t('kid.chore.about', { time: formatDuration(c.estimated_minutes, t) });
  return { ...base, meta, points: c.points_value };
}

/** "7:00 – 9:00 am", "Before 9:00 am", "From 5:00 pm" or nothing. */
export function timeWindow(
  c: Pick<ScheduledChore, 'available_at' | 'due_by'>, now: Date, t: TFunction, lang?: string,
): string | undefined {
  const from = c.available_at ? formatClock(atTime(c.available_at, now), lang) : null;
  const to = c.due_by ? formatClock(atTime(c.due_by, now), lang) : null;
  if (from && to) return `${from} – ${to}`;
  if (to) return t('kid.chore.before', { time: to });
  if (from) return t('kid.chore.from', { time: from });
  return undefined;
}
