import { describe, expect, it } from 'vitest';
import type { TFunction } from 'i18next';
import type { ChoreSchedule } from '../../types';
import { addDays, emptyForm, formFromSchedules, formProblem, summarize, whenFromForm } from './ownTasks';

const today = '2026-11-16';
const t = ((k: string, o?: Record<string, unknown>) => (o ? `${k}:${JSON.stringify(o)}` : k)) as unknown as TFunction;
const sched = (x: Partial<ChoreSchedule>): ChoreSchedule => ({
  id: 1, chore_id: 1, assigned_to: 2, assignment_type: 'individual', expiry_penalty: 'no_points',
  expiry_penalty_value: 0, points_multiplier: 1, created_at: '', ...x,
});

describe('own task form', () => {
  it('needs a title and a day that is not in the past', () => {
    expect(formProblem(emptyForm(today), today)).toBe('kid.own.problem.title');
    expect(formProblem({ ...emptyForm(today), title: 'Pack', date: addDays(today, -1) }, today)).toBe('kid.own.problem.past');
    expect(formProblem({ ...emptyForm(today), title: 'Pack', date: addDays(today, 21) }, today)).toBeNull();
  });

  it('needs days for weekly and a sane interval', () => {
    const f = { ...emptyForm(today), title: 'Swim' };
    expect(formProblem({ ...f, repeat: 'weekly' }, today)).toBe('kid.own.problem.days');
    expect(formProblem({ ...f, repeat: 'interval', interval: 0 }, today)).toBe('kid.own.problem.interval');
    expect(formProblem({ ...f, repeat: 'weekly', days: [1], start: addDays(today, 5), end: addDays(today, 2) }, today)).toBe('kid.own.problem.end');
  });

  it('builds the API when', () => {
    const f = { ...emptyForm(today), title: 'x' };
    expect(whenFromForm({ ...f, date: '2026-12-07' })).toEqual({ specific_date: '2026-12-07' });
    expect(whenFromForm({ ...f, repeat: 'weekly', days: [5, 1], start: today })).toEqual({ days_of_week: [1, 5], start_date: today, end_date: undefined });
    expect(whenFromForm({ ...f, repeat: 'interval', interval: 3, start: '2026-11-20', end: '2026-12-20' }))
      .toEqual({ recurrence_interval: 3, start_date: '2026-11-20', end_date: '2026-12-20' });
  });

  it('reads saved schedules back', () => {
    const base = { title: 'x', icon: '' };
    expect(formFromSchedules(base, [sched({ specific_date: '2026-12-07' })], today)).toMatchObject({ repeat: 'once', date: '2026-12-07' });
    expect(formFromSchedules(base, [sched({ day_of_week: 1 }), sched({ day_of_week: 3, start_date: '2026-11-30' })].reverse(), today))
      .toMatchObject({ repeat: 'weekly', days: [3, 1], start: '2026-11-30' });
    // A start in the past reads as today, so saving it again is allowed.
    expect(formFromSchedules(base, [sched({ recurrence_interval: 2, recurrence_start: '2026-11-01' })], today))
      .toMatchObject({ repeat: 'interval', interval: 2, start: today });
  });

  it('summarizes', () => {
    expect(summarize([sched({ specific_date: today })], today, t)).toBe('kid.own.summary.today');
    expect(summarize([0, 1, 2, 3, 4, 5, 6].map(d => sched({ day_of_week: d })), today, t)).toBe('kid.own.summary.everyDay');
    expect(summarize([sched({ recurrence_interval: 3, recurrence_start: today })], today, t)).toBe('kid.own.summary.everyN:{"count":3}');
  });
});
