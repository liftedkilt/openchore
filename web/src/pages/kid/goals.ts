import type { PointTransaction, RewardCommitment } from '../../types';

/** Look-back window for the savings pace. */
export const PACE_WINDOW_DAYS = 14;

export interface GoalProgress {
  shared: boolean;
  target: number;
  /** Everyone's savings for a shared pot, otherwise mine. */
  saved: number;
  /** My own part of it. */
  mine: number;
  remaining: number;
  pct: number;
  funded: boolean;
}

export function goalProgress(c: RewardCommitment): GoalProgress {
  const shared = !!c.pool;
  const target = shared ? c.pool!.target_cost : c.target_cost;
  const saved = shared ? c.pool!.amount_saved : c.amount_saved;
  return {
    shared,
    target,
    saved,
    mine: c.amount_saved,
    remaining: Math.max(0, target - saved),
    pct: target > 0 ? Math.min(100, Math.round((saved / target) * 100)) : 0,
    funded: saved >= target,
  };
}

/**
 * Days until a personal goal is funded at the recent saving pace: the
 * `commit_to_goal` rows for this goal (manual saves and auto-save) over the
 * last two weeks. Null when there isn't enough history to say, when the goal
 * is funded, or for a shared pot (siblings' pace isn't visible here).
 */
export function daysToGoal(c: RewardCommitment, txs: PointTransaction[], now = new Date()): number | null {
  const p = goalProgress(c);
  if (p.shared || p.funded || p.remaining <= 0) return null;
  const since = now.getTime() - PACE_WINDOW_DAYS * 86_400_000;
  const rows = txs.filter(tx =>
    tx.reason === 'commit_to_goal' && tx.reference_id === c.id && new Date(tx.created_at).getTime() >= since);
  if (rows.length < 2) return null;
  const saved = rows.reduce((sum, tx) => sum + Math.abs(tx.amount), 0);
  const oldest = Math.min(...rows.map(tx => new Date(tx.created_at).getTime()));
  // At least three days of history, so one busy afternoon isn't a trend.
  const days = Math.max(3, (now.getTime() - oldest) / 86_400_000);
  const perDay = saved / days;
  if (perDay <= 0) return null;
  return Math.max(1, Math.ceil(p.remaining / perDay));
}
