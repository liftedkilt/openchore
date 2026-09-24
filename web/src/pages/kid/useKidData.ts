// Data and actions behind a person's own screens (Today, Week, Rewards).
// The business rules live on the server; this keeps the old dashboard's
// client behaviour: double-tap guards, photo flow fallbacks, AI feedback,
// optimistic auto-save and the same error toasts.
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../../api';
import type {
  PointsData, RedemptionHistory, Reward, ScheduledChore, User, UserStreakData,
} from '../../types';
import { localDateStr } from '../../utils';

export interface AIFeedback { text: string; audioUrl?: string }

export interface ToggleResult {
  /** The chore was finished (show the celebration). */
  finished?: boolean;
  /** A photo is needed first: open the photo sheet. */
  needsPhoto?: boolean;
}

export function useKidData(user: User | null, opts: { week: boolean; rewards: boolean }) {
  const { t } = useTranslation();
  const userId = user?.id;
  const [today, setToday] = useState(() => localDateStr(new Date()));
  const [chores, setChores] = useState<ScheduledChore[] | null>(null);
  const [weekChores, setWeekChores] = useState<ScheduledChore[] | null>(null);
  const [streak, setStreak] = useState<UserStreakData | null>(null);
  const [points, setPoints] = useState<PointsData | null>(null);
  const [rewards, setRewards] = useState<Reward[] | null>(null);
  const [redemptions, setRedemptions] = useState<RedemptionHistory[]>([]);
  const [people, setPeople] = useState<User[]>([]);
  const [togglingIds, setTogglingIds] = useState<Set<number>>(new Set());
  const [aiFeedback, setAiFeedback] = useState<Record<number, AIFeedback>>({});
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const showToast = useCallback((message: string) => {
    setToast(message);
    clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 3000);
  }, []);
  useEffect(() => () => clearTimeout(toastTimer.current), []);

  // Roll over at midnight so a tablet left on Today shows the new day.
  useEffect(() => {
    const id = setInterval(() => {
      const d = localDateStr(new Date());
      setToday(prev => (prev === d ? prev : d));
    }, 60_000);
    return () => clearInterval(id);
  }, []);

  const loadChores = useCallback(async () => {
    if (!userId) return;
    try {
      setChores(await api.users.getChores(userId, 'daily', today));
    } catch (e) {
      console.error(e);
      setChores(prev => prev ?? []);
    }
  }, [userId, today]);

  const loadWeek = useCallback(async () => {
    if (!userId) return;
    try {
      setWeekChores(await api.users.getChores(userId, 'weekly', today));
    } catch (e) {
      console.error(e);
      setWeekChores(prev => prev ?? []);
    }
  }, [userId, today]);

  const loadExtras = useCallback(async () => {
    if (!userId) return;
    try {
      const [s, p] = await Promise.all([api.streaks.getForUser(userId), api.points.getForUser(userId)]);
      setStreak(s);
      setPoints(p);
    } catch (e) {
      console.error(e);
    }
  }, [userId]);

  const loadRewards = useCallback(async () => {
    if (!userId) return;
    try {
      const [r, h] = await Promise.all([api.rewards.list(), api.rewards.listRedemptions(userId)]);
      setRewards(r);
      setRedemptions(h);
    } catch (e) {
      console.error(e);
      setRewards(prev => prev ?? []);
    }
  }, [userId]);

  useEffect(() => { loadChores(); }, [loadChores]);
  useEffect(() => { loadExtras(); }, [loadExtras]);
  useEffect(() => { if (opts.week) loadWeek(); }, [opts.week, loadWeek]);
  useEffect(() => { if (opts.rewards) loadRewards(); }, [opts.rewards, loadRewards]);
  // Family pots show each contributor in their colour: the profile list is
  // public and carries everyone's colour key.
  const hasPot = !!points?.active_commitments.some(c => c.pool);
  useEffect(() => {
    if (!hasPot || people.length) return;
    api.users.list().then(setPeople).catch(() => {});
  }, [hasPot, people.length]);

  const reloadChores = useCallback(async () => {
    await Promise.all([loadChores(), opts.week ? loadWeek() : Promise.resolve()]);
  }, [loadChores, loadWeek, opts.week]);

  const refreshAll = useCallback(async () => {
    await Promise.all([reloadChores(), loadExtras()]);
  }, [reloadChores, loadExtras]);

  const clearFeedback = useCallback((scheduleId: number) => {
    setAiFeedback(prev => {
      if (!(scheduleId in prev)) return prev;
      const next = { ...prev };
      delete next[scheduleId];
      return next;
    });
  }, []);

  const setFeedback = useCallback((scheduleId: number, fb: AIFeedback) => {
    setAiFeedback(prev => ({ ...prev, [scheduleId]: fb }));
  }, []);

  const togglingRef = useRef(togglingIds);
  togglingRef.current = togglingIds;

  /**
   * Tap on a chore's check. Completes or uncompletes it, and reports whether
   * the photo sheet or the celebration should open.
   */
  const toggle = useCallback(async (chore: ScheduledChore, opts2: { retry?: boolean } = {}): Promise<ToggleResult> => {
    if (chore.date !== today) return {};
    // Double-tap guard: drop taps while a request for this chore is in flight.
    if (togglingRef.current.has(chore.schedule_id)) return {};
    setTogglingIds(prev => new Set(prev).add(chore.schedule_id));
    try {
      if (chore.completed && !opts2.retry) {
        await api.chores.uncomplete(chore.schedule_id, chore.date);
        await refreshAll();
        return {};
      }
      if (opts2.retry) {
        // A grown-up said no: clear that try, then start a fresh one.
        await api.chores.uncomplete(chore.schedule_id, chore.date);
      }
      const needsPhoto = chore.requires_photo && (chore.photo_source || 'child') === 'child';
      try {
        // With a photo required this may still succeed: the server revives an
        // earlier approved completion from today (photo and AI review kept).
        await api.chores.complete(chore.schedule_id, chore.date);
      } catch (e) {
        if (needsPhoto && e instanceof APIError && e.status === 400) {
          if (opts2.retry) await refreshAll();
          return { needsPhoto: true };
        }
        throw e;
      }
      clearFeedback(chore.schedule_id);
      await refreshAll();
      return { finished: true };
    } catch (err) {
      if (err instanceof APIError && err.status === 422 && err.data?.ai_review) {
        setFeedback(chore.schedule_id, { text: err.data.ai_review.feedback, audioUrl: err.data.ai_review.feedback_audio });
        await reloadChores();
      } else if (err instanceof APIError && (err.status === 400 || err.status === 422)) {
        // Validation errors surface through the photo sheet or AI feedback.
        console.error(err);
      } else {
        console.error(err);
        showToast(err instanceof APIError && err.data?.error ? String(err.data.error) : t('kid.errors.couldntSave'));
      }
      return {};
    } finally {
      setTogglingIds(prev => {
        if (!prev.has(chore.schedule_id)) return prev;
        const next = new Set(prev);
        next.delete(chore.schedule_id);
        return next;
      });
    }
  }, [today, refreshAll, reloadChores, clearFeedback, setFeedback, showToast, t]);

  // --- Rewards and goals ---

  const commitmentFor = useCallback(
    (rewardId: number) => points?.active_commitments.find(c => c.reward_id === rewardId) ?? null,
    [points],
  );

  const [redeemingId, setRedeemingId] = useState<number | null>(null);
  const [redeemedId, setRedeemedId] = useState<number | null>(null);
  const [savingTowardId, setSavingTowardId] = useState<number | null>(null);
  const [busyGoals, setBusyGoals] = useState<Set<number>>(new Set());

  const markGoal = (id: number, on: boolean) => setBusyGoals(prev => {
    const next = new Set(prev);
    if (on) next.add(id); else next.delete(id);
    return next;
  });

  /** Returns true when the reward was redeemed. */
  const redeem = useCallback(async (reward: Reward): Promise<boolean> => {
    if (!points) return false;
    const commitment = commitmentFor(reward.id);
    const funded = commitment?.pool
      ? commitment.pool.amount_saved >= commitment.pool.target_cost
      : commitment ? commitment.amount_saved >= commitment.target_cost : false;
    if (!commitment && points.balance < reward.effective_cost) return false;
    if (commitment && !funded) return false;
    setRedeemingId(reward.id);
    try {
      await api.rewards.redeem(reward.id);
      setRedeemedId(reward.id);
      showToast(t('kid.rewards.redeemedToast', { name: reward.name }));
      await Promise.all([loadExtras(), loadRewards()]);
      setTimeout(() => setRedeemedId(prev => (prev === reward.id ? null : prev)), 2000);
      return true;
    } catch (e) {
      console.error('Redeem error:', e);
      showToast(e instanceof APIError ? e.message : t('kid.errors.redemptionFailed'));
      return false;
    } finally {
      setRedeemingId(null);
    }
  }, [points, commitmentFor, showToast, loadExtras, loadRewards, t]);

  const saveToward = useCallback(async (reward: Reward) => {
    if (!points) return;
    // Personal goals occupy a single slot; shared rewards stack freely.
    if (!reward.shareable && points.active_commitments.some(c => !c.shared_pool_id)) {
      showToast(t('kid.errors.alreadyHasGoal'));
      return;
    }
    setSavingTowardId(reward.id);
    try {
      await api.commitments.commit(reward.id, 0);
      showToast(reward.shareable
        ? t('kid.goal.joinedToast', { name: reward.name })
        : t('kid.goal.savingToast', { name: reward.name }));
      await loadExtras();
    } catch (e) {
      console.error('Save toward error:', e);
      showToast(e instanceof APIError ? e.message : t('kid.errors.couldNotStartSaving'));
    } finally {
      setSavingTowardId(null);
    }
  }, [points, showToast, loadExtras, t]);

  /** Returns true when the points went in. */
  const contribute = useCallback(async (commitmentId: number, raw: string): Promise<boolean> => {
    if (!points) return false;
    const amount = parseInt(raw, 10);
    if (!Number.isFinite(amount) || amount <= 0) {
      showToast(t('kid.errors.enterPoints'));
      return false;
    }
    if (amount > points.balance) {
      showToast(t('kid.errors.notEnoughPoints', { balance: points.balance }));
      return false;
    }
    markGoal(commitmentId, true);
    try {
      await api.commitments.contribute(commitmentId, amount);
      await loadExtras();
      showToast(t('kid.goal.savedMore', { count: amount }));
      return true;
    } catch (e) {
      console.error('Contribute error:', e);
      showToast(e instanceof APIError ? e.message : t('kid.errors.couldNotSave'));
      return false;
    } finally {
      markGoal(commitmentId, false);
    }
  }, [points, showToast, loadExtras, t]);

  const setAutoContribute = useCallback(async (commitmentId: number, percent: number) => {
    // Optimistic so the slider feels responsive.
    setPoints(prev => prev && ({
      ...prev,
      active_commitments: prev.active_commitments.map(c =>
        c.id === commitmentId ? { ...c, auto_contribute_percent: percent } : c),
    }));
    try {
      await api.commitments.setAutoContribute(commitmentId, percent);
    } catch (e) {
      console.error('Auto-contribute error:', e);
      await loadExtras();
    }
  }, [loadExtras]);

  const breakCommitment = useCallback(async (commitmentId: number, shared: boolean) => {
    if (!window.confirm(shared ? t('kid.goal.leaveConfirm') : t('kid.goal.stopConfirm'))) return;
    markGoal(commitmentId, true);
    try {
      await api.commitments.break(commitmentId);
      await loadExtras();
      showToast(shared ? t('kid.goal.leftToast') : t('kid.goal.stoppedToast'));
    } catch (e) {
      console.error('Break commitment error:', e);
      showToast(t('kid.errors.couldNotCancelGoal'));
    } finally {
      markGoal(commitmentId, false);
    }
  }, [showToast, loadExtras, t]);

  return {
    today, chores, weekChores, streak, points, rewards, redemptions, people,
    togglingIds, aiFeedback, toast, showToast,
    reloadChores, refreshAll, loadRewards, clearFeedback, setFeedback, toggle,
    commitmentFor, redeem, redeemingId, redeemedId, saveToward, savingTowardId,
    contribute, setAutoContribute, breakCommitment, busyGoals,
  };
}

export type KidData = ReturnType<typeof useKidData>;
