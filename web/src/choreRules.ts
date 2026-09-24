// Shared, pure rules about a person's chores for the day, so every screen
// that shows them (Today, Week, the wall display) agrees.
import type { ScheduledChore } from './types';

type Chore = Pick<ScheduledChore, 'category' | 'available' | 'completed' | 'completion_status'>;

/** Done for the day: completed in any way except a grown-up's rejection. */
export function isChoreDone(c: Chore): boolean {
  return c.completed && c.completion_status !== 'rejected';
}

/**
 * The day's gates, over the chores that are open now (available, or already
 * done). A chore that hasn't opened yet doesn't hold a gate; the server
 * credits held-back points once a later chore closes it.
 * - `requiredDone`: every Must do is done, so Every day points pay out.
 * - `bonusOpen`: every Must do and Every day chore is done, so Bonus opens.
 */
export function dayGates(todayChores: readonly Chore[]): { requiredDone: boolean; bonusOpen: boolean } {
  const open = todayChores.filter(c => c.available || c.completed);
  return {
    requiredDone: open.filter(c => c.category === 'required').every(isChoreDone),
    bonusOpen: open.filter(c => c.category !== 'bonus').every(isChoreDone),
  };
}
