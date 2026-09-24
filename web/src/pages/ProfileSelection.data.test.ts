import { describe, expect, it } from 'vitest';
import type { ScheduledChore, User } from '../types';
import { dayFraction, orderFamily, stripMembers, summarizeDay } from './ProfileSelection.data';
import { firstFreeColor } from './SetupWizard.data';

function chore(category: ScheduledChore['category'], completed: boolean, completed_at?: string): ScheduledChore {
  return { category, completed, completed_at } as ScheduledChore;
}

function user(id: number, name: string, role: User['role'], paused = false): User {
  return { id, name, role, paused } as User;
}

describe('dayFraction', () => {
  it('maps 7 am – 9 pm onto 0–1 and clamps outside it', () => {
    expect(dayFraction(new Date(2026, 8, 24, 7, 0))).toBe(0);
    expect(dayFraction(new Date(2026, 8, 24, 14, 0))).toBeCloseTo(0.5);
    expect(dayFraction(new Date(2026, 8, 24, 21, 0))).toBe(1);
    expect(dayFraction(new Date(2026, 8, 24, 5, 0))).toBe(0);
    expect(dayFraction(new Date(2026, 8, 24, 23, 0))).toBe(1);
  });
});

describe('summarizeDay', () => {
  it('counts done chores and orders items Must do, Every day, Bonus', () => {
    const at = new Date(2026, 8, 24, 14, 0).toISOString();
    const s = summarizeDay([
      chore('bonus', false),
      chore('core', true, at),
      chore('required', true),
      chore('required', false),
    ], 40);
    expect(s.total).toBe(4);
    expect(s.done).toBe(2);
    expect(s.balance).toBe(40);
    expect(s.items.map((x) => x.cat)).toEqual(['essential', 'essential', 'daily', 'bonus']);
    const daily = s.items.find((x) => x.cat === 'daily');
    expect(daily?.done).toBe(true);
    expect(daily?.at).toBeCloseTo(0.5);
  });

  it('handles a day with no chores', () => {
    expect(summarizeDay([])).toEqual({ items: [], done: 0, total: 0, balance: null });
  });
});

describe('orderFamily and stripMembers', () => {
  const users = [
    user(1, 'Alex', 'admin'),
    user(4, 'Lily', 'child'),
    user(3, 'Emma', 'child'),
    user(2, 'Jamie', 'admin'),
    user(6, 'Zed', 'child', true),
  ];

  it('lists kids then grown-ups, each by name', () => {
    const { kids, parents } = orderFamily(users);
    expect(kids.map((u) => u.name)).toEqual(['Emma', 'Lily', 'Zed']);
    expect(parents.map((u) => u.name)).toEqual(['Alex', 'Jamie']);
  });

  it('shows everyone not paused who has chores today, parents included', () => {
    const { kids, parents } = orderFamily(users);
    const day = (total: number) => ({ items: [], done: 0, total, balance: null });
    const strip = stripMembers([...kids, ...parents], { 1: day(3), 2: day(0), 3: day(5), 4: day(2), 6: day(4) });
    expect(strip.map((u) => u.name)).toEqual(['Emma', 'Lily', 'Alex']);
  });
});

describe('firstFreeColor', () => {
  it('picks the first colour nobody has', () => {
    expect(firstFreeColor([])).toBe('coral');
    expect(firstFreeColor(['coral', 'mint'])).toBe('butter');
    expect(firstFreeColor(['mint', undefined])).toBe('coral');
  });

  it('falls back to the first colour when all are taken', () => {
    expect(firstFreeColor(['coral', 'mint', 'butter', 'sky', 'rose', 'leaf', 'lilac', 'sand'])).toBe('coral');
  });
});
