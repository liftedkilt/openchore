import { describe, it, expect, beforeAll } from 'vitest';
import i18n from '../../i18n';
import type { ScheduledChore, RewardCommitment, PointTransaction } from '../../types';
import { choreView, type ChoreViewContext } from './choreView';
import { daysToGoal, goalProgress } from './goals';
import { weekDates } from './WeekScreen';

const TODAY = '2026-09-24';
const at = (h: number, m = 0) => { const d = new Date(2026, 8, 24, h, m); return d; };

function chore(p: Partial<ScheduledChore> = {}): ScheduledChore {
  return {
    schedule_id: 1, chore_id: 1, title: 'Make Bed', description: '', category: 'required', points_value: 5,
    missed_penalty_value: 0, requires_approval: false, requires_photo: false, assignment_type: 'individual',
    expiry_penalty: 'block', expiry_penalty_value: 0, available: true, expired: false, completed: false,
    date: TODAY, ...p,
  };
}

let ctx: ChoreViewContext;
beforeAll(async () => {
  await i18n.changeLanguage('en');
  ctx = { now: at(15), today: TODAY, bonusOpen: true, requiredDone: true, t: i18n.t, lang: 'en-US' };
});

describe('choreView', () => {
  it('a plain chore is to do, with its points', () => {
    const v = choreView(chore(), ctx);
    expect(v).toMatchObject({ state: 'todo', canToggle: true, points: 5, urgent: false });
  });

  it('counts down and turns urgent near the deadline', () => {
    const v = choreView(chore({ available_at: '14:00', due_by: '15:44' }), ctx);
    expect(v.urgent).toBe(true);
    expect(v.meta).toBe('44 min left');
  });

  it('shows a window when the deadline is further off', () => {
    const v = choreView(chore({ available_at: '14:00', due_by: '18:00' }), ctx);
    expect(v.urgent).toBe(false);
    expect(v.meta).toMatch(/2:00\s?PM – 6:00\s?PM/);
  });

  it('locks a chore that is not open yet, with a countdown when close', () => {
    const soon = choreView(chore({ available: false, available_at: '16:30' }), ctx);
    expect(soon).toMatchObject({ state: 'locked', canToggle: false, meta: 'Opens in 1 h 30 min' });
    const later = choreView(chore({ available: false, available_at: '20:00' }), ctx);
    expect(later.meta).toMatch(/^Opens at 8:00\s?PM$/);
  });

  it('handles each expiry penalty', () => {
    const blocked = choreView(chore({ expired: true, due_by: '09:00', expiry_penalty: 'block' }), ctx);
    expect(blocked).toMatchObject({ state: 'locked', canToggle: false });
    expect(blocked.meta).toMatch(/^Missed\. Closed at 9:00\s?AM$/);
    const noPts = choreView(chore({ expired: true, expiry_penalty: 'no_points' }), ctx);
    expect(noPts).toMatchObject({ state: 'todo', urgent: true, meta: 'Late. No points now', canToggle: true });
    const penalty = choreView(chore({ expired: true, expiry_penalty: 'penalty', expiry_penalty_value: 3 }), ctx);
    expect(penalty.meta).toBe('Late. Costs 3 pts');
  });

  it('maps completion states', () => {
    expect(choreView(chore({ completed: true, completion_status: 'pending' }), ctx).state).toBe('waiting');
    const rejected = choreView(chore({ completed: true, completion_status: 'rejected' }), ctx);
    expect(rejected).toMatchObject({ state: 'todo', retry: true, urgent: true, checkLabel: 'Mark complete' });
    expect(choreView(chore({ completed: true, completion_status: 'excused' }), ctx)).toMatchObject({ state: 'done', meta: 'Excused today' });
    const sibling = choreView(chore({ completed: true, completion_status: 'approved', completed_by_sibling: true, completed_by_name: 'Lily' }), ctx);
    expect(sibling).toMatchObject({ state: 'done', meta: 'Done by Lily' });
  });

  it('holds Every day points until Must do is done', () => {
    const v = choreView(chore({ category: 'core', points_value: 10, completed: true, completion_status: 'approved' }), { ...ctx, requiredDone: false });
    expect(v).toMatchObject({ state: 'done', meta: 'Done. 10 pts after Must do' });
  });

  it('locks Bonus until everything else is done', () => {
    const b = chore({ category: 'bonus' });
    const locked = choreView(b, { ...ctx, bonusOpen: false });
    expect(locked).toMatchObject({ state: 'locked', canToggle: false });
    expect(locked.meta).toBeUndefined(); // ChoreRow's "Opens when everything else is done"
    expect(choreView(b, ctx).state).toBe('todo');
    // A bonus already done stays done (and can be undone).
    expect(choreView({ ...b, completed: true, completion_status: 'approved' }, { ...ctx, bonusOpen: false }).state).toBe('done');
  });

  it('carries photo checks; the AI photo note is for grown-ups only', () => {
    const photo = choreView(chore({ requires_photo: true, photo_source: 'child' }), ctx);
    expect(photo).toMatchObject({ photo: true, meta: 'Photo check' });
    // Finished without a photo (or with one the AI doubts): it waits for a grown-up.
    const waiting = choreView(chore({ requires_photo: true, completed: true, completion_status: 'pending', ai_feedback: 'The bed is still messy' }), ctx);
    expect(waiting).toMatchObject({ state: 'waiting', urgent: false });
    expect(waiting).not.toHaveProperty('note');
    const approved = choreView(chore({ completed: true, completion_status: 'approved', ai_feedback: 'Looks great' }), ctx);
    expect(approved.state).toBe('done');
    expect(approved).not.toHaveProperty('note');
    const proof = choreView(chore({ completed: true, completion_status: 'approved', requires_photo: true, photo_source: 'external' }), ctx);
    expect(proof.needsPhotoProof).toBe(true);
  });

  it('is read-only on other days', () => {
    expect(choreView(chore({ date: '2026-09-23' }), ctx)).toMatchObject({ canToggle: false, meta: 'Not done' });
    expect(choreView(chore({ date: '2026-09-25', completed: false }), ctx).canToggle).toBe(false);
  });
});

describe('goals', () => {
  const goal = (p: Partial<RewardCommitment> = {}): RewardCommitment => ({
    id: 7, user_id: 3, reward_id: 1, target_cost: 100, amount_saved: 40, auto_contribute_percent: 0,
    status: 'active', created_at: '', ...p,
  });
  const tx = (daysAgo: number, amount: number, ref = 7): PointTransaction => ({
    id: daysAgo, user_id: 3, amount, reason: 'commit_to_goal', reference_id: ref,
    created_at: new Date(Date.UTC(2026, 8, 24 - daysAgo, 12)).toISOString(),
  });
  const now = new Date(Date.UTC(2026, 8, 24, 12));

  it('reads a shared pot from the pool', () => {
    const p = goalProgress(goal({ pool: { id: 1, reward_id: 1, target_cost: 300, amount_saved: 300, status: 'active', created_at: '' } }));
    expect(p).toMatchObject({ shared: true, target: 300, saved: 300, mine: 40, funded: true, remaining: 0, pct: 100 });
  });

  it('estimates days at the recent pace', () => {
    // 30 points over 6 days = 5/day; 60 to go → 12 days.
    expect(daysToGoal(goal(), [tx(6, -10), tx(3, -10), tx(1, -10)], now)).toBe(12);
  });

  it('says nothing without enough history', () => {
    expect(daysToGoal(goal(), [tx(1, -10)], now)).toBeNull();
    expect(daysToGoal(goal(), [tx(1, -10, 99), tx(2, -10, 99)], now)).toBeNull();
    expect(daysToGoal(goal(), [tx(30, -10), tx(20, -10)], now)).toBeNull();
  });
});

describe('weekDates', () => {
  it('runs Monday to Sunday', () => {
    expect(weekDates('2026-09-24')).toEqual([
      '2026-09-21', '2026-09-22', '2026-09-23', '2026-09-24', '2026-09-25', '2026-09-26', '2026-09-27',
    ]);
    expect(weekDates('2026-09-27')[0]).toBe('2026-09-21');
  });
});
