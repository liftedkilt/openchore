import { describe, expect, it } from 'vitest';
import type { ScheduledChore } from '../types';
import { atToday, bonusOpen, nextRows } from './AmbientDashboard.data';

let id = 0;
function chore(title: string, category: ScheduledChore['category'], extra: Partial<ScheduledChore> = {}): ScheduledChore {
  id += 1;
  return {
    schedule_id: id, chore_id: id, title, category, completed: false, available: true, expired: false,
    expiry_penalty: 'no_points', points_value: 5, ...extra,
  } as ScheduledChore;
}

const at = (h: number, m = 0) => new Date(2026, 8, 24, h, m);
const titles = (rows: ReturnType<typeof nextRows>) => rows.map((r) => r.chore.title);

describe('atToday', () => {
  it('reads HH:MM on the same day and rejects junk', () => {
    expect(atToday('19:30', at(16))?.getHours()).toBe(19);
    expect(atToday(undefined, at(16))).toBeNull();
    expect(atToday('soon', at(16))).toBeNull();
  });
});

describe('bonusOpen', () => {
  it('opens once every available Must do and Every day chore is done', () => {
    expect(bonusOpen([chore('A', 'required', { completed: true }), chore('B', 'bonus')])).toBe(true);
    expect(bonusOpen([chore('A', 'core'), chore('B', 'bonus')])).toBe(false);
    expect(bonusOpen([chore('Later', 'required', { available: false }), chore('B', 'bonus')])).toBe(true);
  });
});

describe('nextRows', () => {
  it('leaves out done chores and expired chores that are blocked', () => {
    const rows = nextRows([
      chore('Done', 'required', { completed: true }),
      chore('Gone', 'required', { expired: true, expiry_penalty: 'block', due_by: '09:00' }),
      chore('Open', 'core'),
    ], at(16));
    expect(titles(rows)).toEqual(['Open']);
  });

  it('puts what can be done now first, by category then deadline', () => {
    const rows = nextRows([
      chore('Every day', 'core'),
      chore('Must do late in the day', 'required', { due_by: '20:00' }),
      chore('Must do soon', 'required', { due_by: '16:30' }),
    ], at(16));
    expect(titles(rows)).toEqual(['Must do soon', 'Must do late in the day', 'Every day']);
    expect(rows[0].meta).toMatchObject({ kind: 'due' });
    expect(rows[0].urgent).toBe(true);
    expect(rows[1].urgent).toBe(false);
  });

  it('lists upcoming chores with their opening time when nothing is available now', () => {
    const rows = nextRows([
      chore('Feed cats (evening)', 'required', { available: false, available_at: '19:00', due_by: '21:00' }),
      chore('Brush teeth (evening)', 'required', { available: false, available_at: '20:00', due_by: '21:30' }),
    ], at(16, 15));
    expect(titles(rows)).toEqual(['Feed cats (evening)', 'Brush teeth (evening)']);
    expect(rows.every((r) => r.meta?.kind === 'opens' && r.state === 'todo')).toBe(true);
    expect((rows[0].meta as { at: Date }).at.getHours()).toBe(19);
  });

  it('keeps late chores that can still be done, after the ones on time', () => {
    const rows = nextRows([
      chore('Late', 'required', { expired: true, due_by: '09:00' }),
      chore('On time', 'core'),
      chore('Later', 'core', { available: false, available_at: '19:00' }),
    ], at(16));
    expect(titles(rows)).toEqual(['On time', 'Late', 'Later']);
    expect(rows[1]).toMatchObject({ meta: { kind: 'late' }, urgent: true });
  });

  it('locks bonus chores until everything else is done, and lists them last', () => {
    const rows = nextRows([
      chore('Bonus', 'bonus'),
      chore('Later', 'required', { available: false, available_at: '19:00' }),
      chore('Now', 'required'),
    ], at(16));
    expect(titles(rows)).toEqual(['Now', 'Later', 'Bonus']);
    expect(rows[2].state).toBe('locked');

    const open = nextRows([chore('Done', 'required', { completed: true }), chore('Bonus', 'bonus')], at(16));
    expect(open[0].state).toBe('todo');
  });
});
