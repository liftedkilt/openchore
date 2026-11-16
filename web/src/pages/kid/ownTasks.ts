// Pure helpers for the "my own tasks" sheet: the form's state, turning it
// into the API's `when`, reading it back from schedules, and a short summary.
import type { TFunction } from 'i18next';
import type { ChoreSchedule, OwnChoreWhen } from '../../types';
import { localDateStr } from '../../utils';

export type RepeatKind = 'once' | 'weekly' | 'interval';

export interface OwnTaskForm {
  title: string;
  icon: string;
  repeat: RepeatKind;
  /** One day (once). */
  date: string;
  /** Weekly days, 0 = Sunday. */
  days: number[];
  /** Every N days. */
  interval: number;
  /** First day of a repeating task. */
  start: string;
  /** Optional last day of a repeating task ('' = keeps going). */
  end: string;
}

/** Monday-first, as the Week screen shows days. */
export const WEEK_ORDER = [1, 2, 3, 4, 5, 6, 0] as const;

export function emptyForm(today: string, date = today): OwnTaskForm {
  return { title: '', icon: '', repeat: 'once', date, days: [], interval: 2, start: today, end: '' };
}

export function addDays(day: string, n: number): string {
  const d = new Date(`${day}T00:00:00`);
  d.setDate(d.getDate() + n);
  return localDateStr(d);
}

/** Why the form can't be saved yet, as a translation key; null when it can. */
export function formProblem(f: OwnTaskForm, today: string): string | null {
  if (!f.title.trim()) return 'kid.own.problem.title';
  switch (f.repeat) {
    case 'once':
      if (!f.date) return 'kid.own.problem.date';
      if (f.date < today) return 'kid.own.problem.past';
      return null;
    case 'weekly':
      if (!f.days.length) return 'kid.own.problem.days';
      break;
    case 'interval':
      if (!(f.interval >= 1 && f.interval <= 365)) return 'kid.own.problem.interval';
      break;
  }
  if (f.start && f.start < today) return 'kid.own.problem.past';
  if (f.end && f.end < (f.start || today)) return 'kid.own.problem.end';
  return null;
}

export function whenFromForm(f: OwnTaskForm): OwnChoreWhen {
  if (f.repeat === 'once') return { specific_date: f.date };
  const range = { start_date: f.start || undefined, end_date: f.end || undefined };
  if (f.repeat === 'weekly') return { days_of_week: [...f.days].sort((a, b) => a - b), ...range };
  return { recurrence_interval: f.interval, ...range };
}

/** Read a saved task's schedules back into the form. */
export function formFromSchedules(
  base: Pick<OwnTaskForm, 'title' | 'icon'>, schedules: ChoreSchedule[], today: string,
): OwnTaskForm {
  const f = { ...emptyForm(today), ...base };
  const once = schedules.find(s => s.specific_date);
  const every = schedules.find(s => s.recurrence_interval);
  const weekly = schedules.filter(s => s.day_of_week != null && !s.specific_date && !s.recurrence_interval);
  const later = (d?: string) => (d && d > today ? d : today);
  if (every) {
    return { ...f, repeat: 'interval', interval: every.recurrence_interval!, start: later(every.recurrence_start), end: every.end_date ?? '' };
  }
  if (weekly.length) {
    return {
      ...f, repeat: 'weekly', days: weekly.map(s => s.day_of_week!),
      start: later(weekly[0].start_date), end: weekly[0].end_date ?? '',
    };
  }
  if (once) return { ...f, repeat: 'once', date: once.specific_date! };
  return f;
}

/** A short weekday name for day-of-week n (0 = Sunday). */
export function weekdayName(n: number, lang: string | undefined, style: 'short' | 'long' = 'short'): string {
  // 2026-11-15 is a Sunday.
  return new Date(2026, 10, 15 + n).toLocaleDateString(lang, { weekday: style });
}

export function formatDay(day: string, lang?: string): string {
  return new Date(`${day}T00:00:00`).toLocaleDateString(lang, { weekday: 'short', month: 'short', day: 'numeric' });
}

/** "Sat, Dec 5", "Mon, Wed", "Every day", "Every 3 days" … */
export function summarize(schedules: ChoreSchedule[], today: string, t: TFunction, lang?: string): string {
  const f = formFromSchedules({ title: '', icon: '' }, schedules, today);
  if (!schedules.length) return t('kid.own.summary.none');
  const from = f.start > today ? ` · ${t('kid.own.summary.from', { date: formatDay(f.start, lang) })}` : '';
  const until = f.end ? ` · ${t('kid.own.summary.until', { date: formatDay(f.end, lang) })}` : '';
  switch (f.repeat) {
    case 'once':
      return f.date === today ? t('kid.own.summary.today') : formatDay(f.date, lang);
    case 'interval':
      return t('kid.own.summary.everyN', { count: f.interval }) + from + until;
    case 'weekly': {
      const days = WEEK_ORDER.filter(d => f.days.includes(d));
      const label = days.length === 7 ? t('kid.own.summary.everyDay') : days.map(d => weekdayName(d, lang)).join(', ');
      return label + from + until;
    }
  }
}
