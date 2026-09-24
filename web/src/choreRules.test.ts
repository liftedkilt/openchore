import { describe, it, expect } from 'vitest';
import { dayGates, isChoreDone } from './choreRules';
import type { ScheduledChore } from './types';

type C = Pick<ScheduledChore, 'category' | 'available' | 'completed' | 'completion_status'>;
const c = (category: C['category'], p: Partial<C> = {}): C => ({ category, available: true, completed: false, ...p });
const done = { completed: true, completion_status: 'approved' as const };

describe('isChoreDone', () => {
  it('counts every completion but a rejection', () => {
    expect(isChoreDone(c('core', done))).toBe(true);
    expect(isChoreDone(c('core', { completed: true, completion_status: 'pending' }))).toBe(true);
    expect(isChoreDone(c('core', { completed: true, completion_status: 'excused' }))).toBe(true);
    expect(isChoreDone(c('core', { completed: true, completion_status: 'rejected' }))).toBe(false);
    expect(isChoreDone(c('core'))).toBe(false);
  });
});

describe('dayGates', () => {
  it('opens Bonus when every available Must do and Every day chore is done', () => {
    expect(dayGates([c('required', done), c('core', done), c('bonus')])).toEqual({ requiredDone: true, bonusOpen: true });
    expect(dayGates([c('required', done), c('core'), c('bonus')])).toEqual({ requiredDone: true, bonusOpen: false });
    expect(dayGates([c('required'), c('core', done)])).toEqual({ requiredDone: false, bonusOpen: false });
  });

  it("ignores chores that haven't opened yet", () => {
    expect(dayGates([c('required', done), c('required', { available: false })]).bonusOpen).toBe(true);
  });

  it('is open on a day with only bonus chores', () => {
    expect(dayGates([c('bonus')]).bonusOpen).toBe(true);
    expect(dayGates([]).requiredDone).toBe(true);
  });
});
