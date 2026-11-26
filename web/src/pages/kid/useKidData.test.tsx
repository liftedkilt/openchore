// @vitest-environment jsdom
import { describe, it, expect, beforeAll, beforeEach, afterEach, vi } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import i18n from '../../i18n';
import { api } from '../../api';
import type { ScheduledChore, User } from '../../types';
import { localDateStr } from '../../utils';
import { useKidData } from './useKidData';

// Every API call the hook makes resolves to something empty; the tests only
// look at complete / uncomplete.
vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>();
  const list = () => vi.fn().mockResolvedValue([]);
  const none = () => vi.fn().mockResolvedValue(null);
  return {
    ...actual,
    api: {
      ...actual.api,
      chores: { ...actual.api.chores, complete: vi.fn().mockResolvedValue({}), uncomplete: vi.fn().mockResolvedValue(undefined) },
      users: { ...actual.api.users, getChores: list(), list: list() },
      streaks: { ...actual.api.streaks, getForUser: none() },
      points: { ...actual.api.points, getForUser: none() },
      rewards: { ...actual.api.rewards, list: list(), listRedemptions: list() },
      pets: { ...actual.api.pets, farm: none() },
      leaderboard: { ...actual.api.leaderboard, get: none() },
      friends: { ...actual.api.friends, visitors: none() },
    },
  };
});

const bob = { id: 2, name: 'Bob', role: 'child' } as User;

function chore(over: Partial<ScheduledChore>): ScheduledChore {
  return {
    schedule_id: 7, chore_id: 1, title: 'Empty dryer', description: '', category: 'core',
    points_value: 10, missed_penalty_value: 0, requires_approval: false, requires_photo: false,
    photo_source: 'child', auto_complete: false, assignment_type: 'fcfs', expiry_penalty: 'block',
    expiry_penalty_value: 0, available: true, expired: false, completed: false,
    date: localDateStr(new Date()),
    ...over,
  } as ScheduledChore;
}

async function tap(c: ScheduledChore, opts: { retry?: boolean } = {}) {
  const { result } = renderHook(() => useKidData(bob, { week: false, rewards: false }));
  await act(async () => { await result.current.toggle(c, opts); });
  return result;
}

describe('useKidData toggle on a first-come chore', () => {
  beforeAll(async () => { await i18n.changeLanguage('en'); });
  beforeEach(() => { vi.clearAllMocks(); });
  afterEach(() => { cleanup(); });

  it("doesn't uncheck a chore a sibling did, and says why", async () => {
    const result = await tap(chore({ completed: true, completion_status: 'approved', completed_by_sibling: true, completed_by_name: 'Alice' }));
    expect(api.chores.uncomplete).not.toHaveBeenCalled();
    expect(result.current.toast).toBe('Alice did this one. Only they or a grown-up can undo it.');
  });

  it("doesn't try to complete a chore a sibling's waiting completion has taken", async () => {
    const result = await tap(chore({ completed: false, completed_by_sibling: true, completed_by_name: 'Alice' }));
    expect(api.chores.complete).not.toHaveBeenCalled();
    expect(api.chores.uncomplete).not.toHaveBeenCalled();
    expect(result.current.toast).toMatch(/^Alice got to this one first/);
  });

  it("a stale 'try again' doesn't clear or redo a chore a sibling has since taken", async () => {
    // Bob's try was rejected, then Alice's went in; the screen hadn't refreshed.
    const result = await tap(
      chore({ completed: true, completion_status: 'rejected', completed_by_sibling: true, completed_by_name: 'Alice' }),
      { retry: true },
    );
    expect(api.chores.uncomplete).not.toHaveBeenCalled();
    expect(api.chores.complete).not.toHaveBeenCalled();
    expect(result.current.toast).toMatch(/^Alice got to this one first/);
  });

  it('lets the kid who did it uncheck their own', async () => {
    await tap(chore({ completed: true, completion_status: 'approved' }));
    expect(api.chores.uncomplete).toHaveBeenCalledWith(7, localDateStr(new Date()));
  });
});
