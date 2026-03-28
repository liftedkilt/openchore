import { describe, it, expect } from 'vitest';
import {
  getTimePeriod,
  groupChoresByTimePeriod,
  isTimePeriodActive,
  isTimePeriodPast,
  TIME_PERIOD_CONFIG,
} from './timeGrouping';
import type { ScheduledChore } from './types';

// Helper to create a minimal ScheduledChore for testing
function makeChore(overrides: Partial<ScheduledChore> = {}): ScheduledChore {
  return {
    schedule_id: 1,
    chore_id: 1,
    title: 'Test Chore',
    description: '',
    category: 'required',
    points_value: 10,
    missed_penalty_value: 0,
    requires_approval: false,
    requires_photo: false,
    assignment_type: 'daily',
    expiry_penalty: 'no_points',
    expiry_penalty_value: 0,
    available: true,
    expired: false,
    completed: false,
    date: '2026-03-28',
    ...overrides,
  };
}

// ── getTimePeriod ──────────────────────────────────────────────────

describe('getTimePeriod', () => {
  it('returns morning for undefined available_at', () => {
    expect(getTimePeriod(undefined)).toBe('morning');
  });

  it('returns morning for empty string available_at', () => {
    expect(getTimePeriod('')).toBe('morning');
  });

  it('returns morning for "06:00"', () => {
    expect(getTimePeriod('06:00')).toBe('morning');
  });

  it('returns morning for "08:00"', () => {
    expect(getTimePeriod('08:00')).toBe('morning');
  });

  it('returns morning for "11:59"', () => {
    expect(getTimePeriod('11:59')).toBe('morning');
  });

  it('returns afternoon for "12:00" (boundary)', () => {
    expect(getTimePeriod('12:00')).toBe('afternoon');
  });

  it('returns afternoon for "14:30"', () => {
    expect(getTimePeriod('14:30')).toBe('afternoon');
  });

  it('returns afternoon for "16:59"', () => {
    expect(getTimePeriod('16:59')).toBe('afternoon');
  });

  it('returns evening for "17:00" (boundary)', () => {
    expect(getTimePeriod('17:00')).toBe('evening');
  });

  it('returns evening for "19:00"', () => {
    expect(getTimePeriod('19:00')).toBe('evening');
  });

  it('returns evening for "23:59"', () => {
    expect(getTimePeriod('23:59')).toBe('evening');
  });

  it('returns evening for non-numeric input (NaN falls through all comparisons)', () => {
    expect(getTimePeriod('invalid')).toBe('evening');
  });
});

// ── groupChoresByTimePeriod ────────────────────────────────────────

describe('groupChoresByTimePeriod', () => {
  it('returns empty array for empty input', () => {
    expect(groupChoresByTimePeriod([])).toEqual([]);
  });

  it('groups mixed chores into correct buckets', () => {
    const chores = [
      makeChore({ schedule_id: 1, title: 'Brush teeth', available_at: '07:00' }),
      makeChore({ schedule_id: 2, title: 'Homework', available_at: '15:00' }),
      makeChore({ schedule_id: 3, title: 'Read book', available_at: '19:00' }),
      makeChore({ schedule_id: 4, title: 'Make bed', available_at: '08:00' }),
    ];

    const groups = groupChoresByTimePeriod(chores);

    expect(groups).toHaveLength(3);

    expect(groups[0].key).toBe('morning');
    expect(groups[0].label).toBe('Morning');
    expect(groups[0].chores).toHaveLength(2);
    expect(groups[0].chores.map(c => c.title)).toEqual(['Brush teeth', 'Make bed']);

    expect(groups[1].key).toBe('afternoon');
    expect(groups[1].chores).toHaveLength(1);
    expect(groups[1].chores[0].title).toBe('Homework');

    expect(groups[2].key).toBe('evening');
    expect(groups[2].chores).toHaveLength(1);
    expect(groups[2].chores[0].title).toBe('Read book');
  });

  it('filters out empty groups', () => {
    const chores = [
      makeChore({ schedule_id: 1, title: 'Morning chore', available_at: '09:00' }),
      makeChore({ schedule_id: 2, title: 'Another morning', available_at: '10:00' }),
    ];

    const groups = groupChoresByTimePeriod(chores);

    expect(groups).toHaveLength(1);
    expect(groups[0].key).toBe('morning');
    expect(groups[0].chores).toHaveLength(2);
  });

  it('puts chores with no available_at in morning', () => {
    const chores = [
      makeChore({ schedule_id: 1, title: 'No time', available_at: undefined }),
      makeChore({ schedule_id: 2, title: 'Evening task', available_at: '20:00' }),
    ];

    const groups = groupChoresByTimePeriod(chores);

    expect(groups).toHaveLength(2);
    expect(groups[0].key).toBe('morning');
    expect(groups[0].chores[0].title).toBe('No time');
    expect(groups[1].key).toBe('evening');
  });

  it('returns groups in morning/afternoon/evening order', () => {
    const chores = [
      makeChore({ schedule_id: 1, available_at: '20:00' }),
      makeChore({ schedule_id: 2, available_at: '08:00' }),
      makeChore({ schedule_id: 3, available_at: '14:00' }),
    ];

    const groups = groupChoresByTimePeriod(chores);
    expect(groups.map(g => g.key)).toEqual(['morning', 'afternoon', 'evening']);
  });

  it('includes startHour and nextStartHour in each group', () => {
    const chores = [
      makeChore({ schedule_id: 1, available_at: '08:00' }),
      makeChore({ schedule_id: 2, available_at: '14:00' }),
      makeChore({ schedule_id: 3, available_at: '20:00' }),
    ];

    const groups = groupChoresByTimePeriod(chores);

    expect(groups[0].startHour).toBe(0);
    expect(groups[0].nextStartHour).toBe(12);

    expect(groups[1].startHour).toBe(12);
    expect(groups[1].nextStartHour).toBe(17);

    expect(groups[2].startHour).toBe(17);
    expect(groups[2].nextStartHour).toBe(24);
  });
});

// ── isTimePeriodActive ─────────────────────────────────────────────

describe('isTimePeriodActive', () => {
  // Morning: startHour=0, nextStartHour=12
  it('morning is active at 9am', () => {
    const at9am = new Date(2026, 2, 28, 9, 0, 0);
    expect(isTimePeriodActive(0, 12, at9am)).toBe(true);
  });

  it('morning is not active at 2pm', () => {
    const at2pm = new Date(2026, 2, 28, 14, 0, 0);
    expect(isTimePeriodActive(0, 12, at2pm)).toBe(false);
  });

  // Afternoon: startHour=12, nextStartHour=17
  it('afternoon is active at 2pm', () => {
    const at2pm = new Date(2026, 2, 28, 14, 0, 0);
    expect(isTimePeriodActive(12, 17, at2pm)).toBe(true);
  });

  it('afternoon is active at boundary hour 12', () => {
    const atNoon = new Date(2026, 2, 28, 12, 0, 0);
    expect(isTimePeriodActive(12, 17, atNoon)).toBe(true);
  });

  it('afternoon is not active at 6pm', () => {
    const at6pm = new Date(2026, 2, 28, 18, 0, 0);
    expect(isTimePeriodActive(12, 17, at6pm)).toBe(false);
  });

  // Evening: startHour=17, nextStartHour=24
  it('evening is active at 7pm', () => {
    const at7pm = new Date(2026, 2, 28, 19, 0, 0);
    expect(isTimePeriodActive(17, 24, at7pm)).toBe(true);
  });

  it('evening is active at boundary hour 17', () => {
    const at5pm = new Date(2026, 2, 28, 17, 0, 0);
    expect(isTimePeriodActive(17, 24, at5pm)).toBe(true);
  });

  it('evening is not active at 9am', () => {
    const at9am = new Date(2026, 2, 28, 9, 0, 0);
    expect(isTimePeriodActive(17, 24, at9am)).toBe(false);
  });

  it('morning is active at midnight (boundary hour 0)', () => {
    const atMidnight = new Date(2026, 2, 28, 0, 0, 0);
    expect(isTimePeriodActive(0, 12, atMidnight)).toBe(true);
  });
});

// ── isTimePeriodPast ───────────────────────────────────────────────

describe('isTimePeriodPast', () => {
  // Morning has nextStartHour=12
  it('morning is not past at 9am', () => {
    const at9am = new Date(2026, 2, 28, 9, 0, 0);
    expect(isTimePeriodPast(12, at9am)).toBe(false);
  });

  it('morning is past at 2pm', () => {
    const at2pm = new Date(2026, 2, 28, 14, 0, 0);
    expect(isTimePeriodPast(12, at2pm)).toBe(true);
  });

  it('morning is past at boundary hour 12', () => {
    const atNoon = new Date(2026, 2, 28, 12, 0, 0);
    expect(isTimePeriodPast(12, atNoon)).toBe(true);
  });

  // Afternoon has nextStartHour=17
  it('afternoon is not past at 2pm', () => {
    const at2pm = new Date(2026, 2, 28, 14, 0, 0);
    expect(isTimePeriodPast(17, at2pm)).toBe(false);
  });

  it('afternoon is past at 6pm', () => {
    const at6pm = new Date(2026, 2, 28, 18, 0, 0);
    expect(isTimePeriodPast(17, at6pm)).toBe(true);
  });

  it('afternoon is past at boundary hour 17', () => {
    const at5pm = new Date(2026, 2, 28, 17, 0, 0);
    expect(isTimePeriodPast(17, at5pm)).toBe(true);
  });

  // Evening has nextStartHour=24
  it('evening is never past at 7pm', () => {
    const at7pm = new Date(2026, 2, 28, 19, 0, 0);
    expect(isTimePeriodPast(24, at7pm)).toBe(false);
  });

  it('evening is never past at 11:59pm', () => {
    const at1159pm = new Date(2026, 2, 28, 23, 59, 0);
    expect(isTimePeriodPast(24, at1159pm)).toBe(false);
  });

  it('evening is never past in the morning', () => {
    const at9am = new Date(2026, 2, 28, 9, 0, 0);
    expect(isTimePeriodPast(24, at9am)).toBe(false);
  });
});
