import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { RefreshCw, Loader2 } from 'lucide-react';
import { api } from '../../api';
import type { User, ScheduledChore, UserStreakData, PointBalance, PendingCompletion } from '../../types';
import { Avatar, CategoryHeader, CategoryMark, Icon, catFromCategory } from '../../design';
import { localDateStr } from '../../utils';
import { ApprovalList } from './ApprovalList';
import { IconWell, personColor } from './pickers';
import ui from './ui.module.css';
import styles from './KidsStatusTab.module.css';

interface KidStatus {
  user: User;
  chores: ScheduledChore[];
  balance: number;
  streak: number;
  pendingApprovals: number;
  loadError: boolean;
}

interface Breakdown {
  requiredCompleted: number;
  requiredTotal: number;
  coreCompleted: number;
  coreTotal: number;
  bonusCompleted: number;
  bonusTotal: number;
  // overdue is restricted to required+core (bonus is never "overdue" for the
  // purpose of alerts — it's opt-in and doesn't block the bonus gate).
  overdue: number;
  pendingOnToday: number;
}

function breakdownFor(chores: ScheduledChore[]): Breakdown {
  let requiredCompleted = 0;
  let requiredTotal = 0;
  let coreCompleted = 0;
  let coreTotal = 0;
  let bonusCompleted = 0;
  let bonusTotal = 0;
  let overdue = 0;
  let pendingOnToday = 0;
  for (const c of chores) {
    if (c.category === 'required') {
      requiredTotal += 1;
      if (c.completed) requiredCompleted += 1;
    } else if (c.category === 'core') {
      coreTotal += 1;
      if (c.completed) coreCompleted += 1;
    } else {
      bonusTotal += 1;
      if (c.completed) bonusCompleted += 1;
    }
    if (!c.completed && c.expired && c.category !== 'bonus') overdue += 1;
    if (c.completion_status === 'pending') pendingOnToday += 1;
  }
  return {
    requiredCompleted,
    requiredTotal,
    coreCompleted,
    coreTotal,
    bonusCompleted,
    bonusTotal,
    overdue,
    pendingOnToday,
  };
}

export const KidsStatusTab: React.FC<{ onPendingChange?: (count: number) => void }> = ({ onPendingChange }) => {
  const { t, i18n } = useTranslation();
  const [kids, setKids] = useState<KidStatus[]>([]);
  const [allUsers, setAllUsers] = useState<User[]>([]);
  const [pending, setPending] = useState<PendingCompletion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [refreshing, setRefreshing] = useState(false);

  const today = localDateStr(new Date());

  const load = useCallback(async () => {
    setRefreshing(true);
    setError(null);
    try {
      const [users, balances, pendingList] = await Promise.all([
        api.users.list(),
        api.points.getAllBalances().catch(() => [] as PointBalance[]),
        api.chores.listPending().catch(() => [] as PendingCompletion[]),
      ]);
      setAllUsers(users);
      setPending(pendingList);
      onPendingChange?.(pendingList.length);

      // Kids first, then parents (who take part too and show up once they
      // have chores of their own).
      const children = users
        .filter((u: User) => !u.paused || u.role === 'child')
        .sort((a, b) => (a.role === b.role ? a.name.localeCompare(b.name) : a.role === 'child' ? -1 : 1));

      // Attribute pending approvals to the assignee (the kid the chore
      // belongs to), not the completer. Matching by name would collapse
      // duplicate names and would miss the sibling-completing case.
      const pendingByAssignee = new Map<number, number>();
      for (const p of pendingList) {
        pendingByAssignee.set(p.assigned_user_id, (pendingByAssignee.get(p.assigned_user_id) || 0) + 1);
      }

      const results = await Promise.all(
        children.map(async (kid): Promise<KidStatus> => {
          try {
            const [chores, streakData] = await Promise.all([
              api.users.getChores(kid.id, 'daily', today),
              api.streaks.getForUser(kid.id).catch<UserStreakData>(() => ({
                current_streak: 0,
                longest_streak: 0,
                earned_rewards: [],
              })),
            ]);
            const bal = balances.find(b => b.user_id === kid.id)?.balance ?? 0;
            return {
              user: kid,
              chores,
              balance: bal,
              streak: streakData.current_streak,
              pendingApprovals: pendingByAssignee.get(kid.id) || 0,
              loadError: false,
            };
          } catch (e) {
            console.error('Failed to load kid status', kid.id, e);
            return {
              user: kid,
              chores: [],
              balance: balances.find(b => b.user_id === kid.id)?.balance ?? 0,
              streak: 0,
              pendingApprovals: pendingByAssignee.get(kid.id) || 0,
              loadError: true,
            };
          }
        }),
      );

      setKids(results.filter(k => k.user.role === 'child' || k.chores.length > 0));
    } catch {
      setError(t('admin.kidsStatusTab.loadError'));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [today, t, onPendingChange]);

  const handleExcuse = async (e: React.MouseEvent, chore: ScheduledChore) => {
    e.stopPropagation();
    const promptMsg = t('admin.kidsStatusTab.excusePrompt', { title: chore.title }) || `Excuse "${chore.title}" for today? Enter optional reason:`;
    const reason = window.prompt(promptMsg);
    if (reason === null) return; // user cancelled prompt
    try {
      await api.chores.excuse(chore.schedule_id, chore.date, reason);
      load();
    } catch (err) {
      console.error('Failed to excuse chore:', err);
    }
  };

  // Parents can tick a chore off (or undo it) on someone's behalf — the
  // assignee is credited, and no photo is needed since the parent vouches.
  const [toggling, setToggling] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const handleToggle = async (e: React.MouseEvent, kid: KidStatus, chore: ScheduledChore) => {
    e.stopPropagation();
    const key = `${chore.schedule_id}-${chore.date}`;
    if (toggling) return;
    if (chore.completed && !window.confirm(t('admin.kidsStatusTab.confirmUndo', { title: chore.title, name: kid.user.name }))) {
      return;
    }
    setToggling(key);
    setActionError(null);
    try {
      if (chore.completed) {
        await api.chores.uncomplete(chore.schedule_id, chore.date);
      } else {
        await api.chores.complete(chore.schedule_id, chore.date);
      }
      await load();
    } catch (err) {
      setActionError(t('admin.kidsStatusTab.toggleError', {
        title: chore.title,
        error: err instanceof Error ? err.message : String(err),
      }));
    } finally {
      setToggling(null);
    }
  };

  useEffect(() => { load(); }, [load]);

  const toggleExpand = (id: number) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  if (loading) return <p className={ui.emptyInline}>{t('admin.kidsStatusTab.loading')}</p>;

  const dateLabel = new Date().toLocaleDateString(i18n.language, { weekday: 'long', month: 'long', day: 'numeric' });

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.kidsStatusTab.heading')}</h2>
          <p className={ui.pageSub}>{t('admin.kidsStatusTab.subtitle', { date: dateLabel })}</p>
        </div>
        <button
          type="button"
          className={ui.btnGhost}
          onClick={load}
          disabled={refreshing}
          title={t('admin.kidsStatusTab.refreshTitle')}
        >
          <RefreshCw aria-hidden className={refreshing ? ui.spin : undefined} /> {t('admin.kidsStatusTab.refreshLabel')}
        </button>
      </div>

      {error && <p className={ui.msgError} role="alert">{error}</p>}

      {pending.length > 0 && (
        <section className={ui.section} aria-labelledby="waiting-for-you">
          <div className={ui.sectionHead}>
            <h3 className={ui.sectionTitle} id="waiting-for-you">
              <Icon name="clock" />
              {t('admin.kidsStatusTab.waitingHeading')}
              <span className={ui.badgeWaiting}>{pending.length}</span>
            </h3>
          </div>
          <ApprovalList pending={pending} users={allUsers} onChanged={load} />
        </section>
      )}

      {kids.length === 0 && (
        <div className={ui.empty}>
          <Icon name="people" />
          <p>{t('admin.kidsStatusTab.noChildren')}</p>
        </div>
      )}

      <div className={styles.grid}>
        {kids.map(kid => {
          const b = breakdownFor(kid.chores);
          // The bar shows Must do + Every day only. Bonus points are gated on
          // those (CLAUDE.md), so bonus progress only shows once it's open.
          const gatedTotal = b.requiredTotal + b.coreTotal;
          const gatedCompleted = b.requiredCompleted + b.coreCompleted;
          const gatedPct = gatedTotal > 0 ? (gatedCompleted / gatedTotal) * 100 : 0;
          const allRequiredAndCoreDone = gatedTotal > 0 && gatedCompleted === gatedTotal;
          const bonusUnlocked = allRequiredAndCoreDone && b.bonusTotal > 0;
          const hasAlert = b.overdue > 0;
          const isExpanded = expanded.has(kid.user.id);
          const totalChores = gatedTotal + b.bonusTotal;
          const detailsId = `kid-${kid.user.id}-details`;
          const color = personColor(kid.user);
          const counts: { cat: 'required' | 'core' | 'bonus'; done: number; total: number }[] = [
            { cat: 'required', done: b.requiredCompleted, total: b.requiredTotal },
            { cat: 'core', done: b.coreCompleted, total: b.coreTotal },
            { cat: 'bonus', done: b.bonusCompleted, total: b.bonusTotal },
          ];

          return (
            <div
              key={kid.user.id}
              className={clsx(styles.card, kid.user.paused && styles.cardPaused)}
              data-person={color}
            >
              <button
                type="button"
                className={styles.header}
                onClick={() => toggleExpand(kid.user.id)}
                aria-expanded={isExpanded}
                aria-controls={detailsId}
              >
                <span className={styles.who}>
                  <Avatar name={kid.user.name} color={color} size="md" />
                  <span className={styles.nameBlock}>
                    <span className={styles.name}>{kid.user.name}</span>
                    <span className={styles.tags}>
                      {kid.user.paused && <span className={ui.badgeOutline}>{t('admin.kidsStatusTab.paused')}</span>}
                      {hasAlert && (
                        <span className={ui.badgeUrgent}>
                          <Icon name="clock" /> {t('admin.kidsStatusTab.overdue', { count: b.overdue })}
                        </span>
                      )}
                      {!hasAlert && allRequiredAndCoreDone && (
                        <span className={ui.badge}>
                          <Icon name="check" /> {t('admin.kidsStatusTab.allDone')}
                        </span>
                      )}
                    </span>
                  </span>
                  <Icon name="chev" className={clsx(styles.caret, isExpanded && styles.caretOpen)} />
                </span>

                {totalChores === 0 ? (
                  <span className={styles.noChores}>{t('admin.kidsStatusTab.noChoresScheduled')}</span>
                ) : (
                  <>
                    <span className={styles.progressRow}>
                      <span className={styles.bar}>
                        {gatedTotal > 0 && <i style={{ width: `${gatedPct}%` }} />}
                      </span>
                      <span className={styles.progressCount}>
                        {t('admin.kidsStatusTab.progress', { done: gatedCompleted, total: gatedTotal })}
                      </span>
                    </span>
                    <span className={styles.cats}>
                      {counts.filter(c => c.total > 0).map(c => (
                        <span key={c.cat} className={styles.catCount}>
                          <CategoryMark cat={catFromCategory(c.cat)} done={c.done === c.total} />
                          <span className={ui.srOnlyText}>{t(`design.category.${catFromCategory(c.cat)}`)}</span>
                          {c.done}/{c.total}
                        </span>
                      ))}
                    </span>
                    {b.bonusTotal > 0 && !bonusUnlocked && (
                      <span className={styles.hint}>
                        <Icon name="lock" /> {t('admin.kidsStatusTab.bonusHint')}
                      </span>
                    )}
                  </>
                )}

                <span className={styles.stats}>
                  <span><Icon name="flame" fill className={styles.flame} /> {t('admin.kidsStatusTab.streak', { count: kid.streak })}</span>
                  <span><Icon name="star" fill className={styles.star} /> {t('admin.kidsStatusTab.points', { count: kid.balance })}</span>
                  {kid.pendingApprovals > 0 && (
                    <span className={styles.waiting}>
                      <Icon name="clock" /> {t('admin.kidsStatusTab.awaitingApproval', { count: kid.pendingApprovals })}
                    </span>
                  )}
                  {b.pendingOnToday > 0 && b.pendingOnToday !== kid.pendingApprovals && (
                    <span className={styles.waiting}>
                      <Icon name="clock" /> {t('admin.kidsStatusTab.pendingToday', { count: b.pendingOnToday })}
                    </span>
                  )}
                </span>
              </button>

              {isExpanded && (
                <div id={detailsId} className={styles.details}>
                  {kid.loadError && (
                    <p className={ui.msgError}>{t('admin.kidsStatusTab.childLoadError')}</p>
                  )}
                  {!kid.loadError && kid.chores.length === 0 && (
                    <p className={ui.emptyInline}>{t('admin.kidsStatusTab.noChoresScheduledDetail')}</p>
                  )}
                  {!kid.loadError && (['required', 'core', 'bonus'] as const).map(category => {
                    const items = kid.chores.filter(c => c.category === category);
                    if (items.length === 0) return null;
                    const cat = catFromCategory(category);
                    const done = items.filter(c => c.completed).length;
                    return (
                      <div key={category}>
                        <CategoryHeader cat={cat} as="h4" count={t('admin.kidsStatusTab.catCount', { done, total: items.length })} className={styles.catHead} />
                        <ul className={styles.choreList}>
                          {items.map(c => {
                            const isOverdue = !c.completed && c.expired && c.category !== 'bonus';
                            const isPending = c.completion_status === 'pending';
                            const isExcused = c.completion_status === 'excused';
                            const key = `${c.schedule_id}-${c.date}`;
                            const label = c.completed
                              ? t('admin.kidsStatusTab.markNotDone', { title: c.title, name: kid.user.name })
                              : t('admin.kidsStatusTab.markDone', { title: c.title, name: kid.user.name });
                            return (
                              <li key={key} className={clsx(styles.chore, c.completed && styles.choreDone)}>
                                {isExcused ? (
                                  <span className={clsx(styles.ring, styles.ringExcused)} aria-hidden>
                                    <Icon name="check" />
                                  </span>
                                ) : (
                                  <button
                                    type="button"
                                    className={styles.check}
                                    onClick={(e) => handleToggle(e, kid, c)}
                                    disabled={toggling !== null}
                                    aria-label={label}
                                    title={label}
                                  >
                                    <span className={clsx(
                                      styles.ring,
                                      c.completed && !isPending && styles.ringDone,
                                      isPending && styles.ringWaiting,
                                    )}>
                                      {toggling === key
                                        ? <Loader2 aria-hidden className={ui.spin} />
                                        : c.completed && <Icon name="check" />}
                                    </span>
                                  </button>
                                )}
                                <IconWell icon={c.icon} cat={cat} className={styles.choreWell} />
                                <span className={styles.choreText}>
                                  <span className={styles.choreTitle}>{c.title}</span>
                                  <span className={clsx(
                                    styles.choreStatus,
                                    isOverdue && styles.statusUrgent,
                                    isPending && styles.statusWaiting,
                                  )} title={isExcused ? c.ai_feedback || undefined : undefined}>
                                    {isExcused
                                      ? t('admin.kidsStatusTab.statusExcused')
                                      : c.completed && isPending
                                        ? t('admin.kidsStatusTab.statusPending')
                                        : c.completed
                                          ? t('admin.kidsStatusTab.statusDone', { points: c.points_value })
                                          : isOverdue
                                            ? t('admin.kidsStatusTab.statusOverdue')
                                            : t('admin.kidsStatusTab.statusIdle', { points: c.points_value })}
                                  </span>
                                </span>
                                {!c.completed && (
                                  <button
                                    type="button"
                                    className={styles.excuse}
                                    onClick={(e) => handleExcuse(e, c)}
                                    title={t('admin.kidsStatusTab.excuseButtonTooltip')}
                                  >
                                    {t('admin.kidsStatusTab.excuseButton')}
                                  </button>
                                )}
                              </li>
                            );
                          })}
                        </ul>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {actionError && (
        <p className={ui.msgError} role="alert">{actionError}</p>
      )}

      <p className={styles.tapHint}>{t('admin.kidsStatusTab.tapHint')}</p>
    </div>
  );
};
