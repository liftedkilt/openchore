import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { User, ScheduledChore, UserStreakData, PointBalance } from '../../types';
import adminStyles from '../../pages/AdminDashboard.module.css';
import styles from './KidsStatusTab.module.css';
import {
  Check,
  ChevronDown,
  Flame,
  Star,
  Clock,
  AlertTriangle,
  RefreshCw,
  Circle,
} from 'lucide-react';
import clsx from 'clsx';
import { localDateStr } from '../../utils';

interface PendingCompletionLite {
  child_name: string;
}

interface KidStatus {
  user: User;
  chores: ScheduledChore[];
  balance: number;
  streak: number;
  pendingApprovals: number;
  loadError: boolean;
}

interface Breakdown {
  coreCompleted: number;
  coreTotal: number;
  bonusCompleted: number;
  bonusTotal: number;
  overdue: number;
  pendingOnToday: number;
}

function breakdownFor(chores: ScheduledChore[]): Breakdown {
  let coreCompleted = 0;
  let coreTotal = 0;
  let bonusCompleted = 0;
  let bonusTotal = 0;
  let overdue = 0;
  let pendingOnToday = 0;
  for (const c of chores) {
    const isBonus = c.category === 'bonus';
    if (isBonus) {
      bonusTotal += 1;
      if (c.completed) bonusCompleted += 1;
    } else {
      coreTotal += 1;
      if (c.completed) coreCompleted += 1;
    }
    if (!c.completed && c.expired && c.category !== 'bonus') overdue += 1;
    if (c.completion_status === 'pending') pendingOnToday += 1;
  }
  return { coreCompleted, coreTotal, bonusCompleted, bonusTotal, overdue, pendingOnToday };
}

function initialsFor(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

export const KidsStatusTab: React.FC = () => {
  const [kids, setKids] = useState<KidStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [refreshing, setRefreshing] = useState(false);

  const today = localDateStr(new Date());

  const load = useCallback(async () => {
    setRefreshing(true);
    setError(null);
    try {
      const [users, balances, pending] = await Promise.all([
        api.users.list(),
        api.points.getAllBalances().catch(() => [] as PointBalance[]),
        api.chores.listPending().catch(() => [] as PendingCompletionLite[]),
      ]);

      const children = users
        .filter((u: User) => u.role === 'child')
        .sort((a, b) => a.name.localeCompare(b.name));

      // Count pending approvals per child name (pending API returns child_name only).
      const pendingByName = new Map<string, number>();
      for (const p of pending as PendingCompletionLite[]) {
        pendingByName.set(p.child_name, (pendingByName.get(p.child_name) || 0) + 1);
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
              pendingApprovals: pendingByName.get(kid.name) || 0,
              loadError: false,
            };
          } catch (e) {
            console.error('Failed to load kid status', kid.id, e);
            return {
              user: kid,
              chores: [],
              balance: balances.find(b => b.user_id === kid.id)?.balance ?? 0,
              streak: 0,
              pendingApprovals: pendingByName.get(kid.name) || 0,
              loadError: true,
            };
          }
        }),
      );

      setKids(results);
    } catch (e) {
      console.error(e);
      setError('Failed to load kids status. Try refreshing.');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [today]);

  useEffect(() => { load(); }, [load]);

  const toggleExpand = (id: number) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  if (loading) return <p className={adminStyles.emptyText}>Loading...</p>;

  return (
    <div>
      <div className={styles.refreshBar}>
        <div>
          <h2 className={adminStyles.sectionTitle}>Kids Status</h2>
          <p className={adminStyles.sectionSubtitle}>
            Today's progress at a glance — {new Date().toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' })}
          </p>
        </div>
        <button
          className={adminStyles.btnSmall}
          onClick={load}
          disabled={refreshing}
          title="Refresh"
        >
          <RefreshCw size={14} className={refreshing ? adminStyles.spinning : undefined} /> Refresh
        </button>
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {kids.length === 0 && (
        <div className={adminStyles.emptyState}>
          <p>No children configured. Add a child in the People tab to see their status here.</p>
        </div>
      )}

      <div className={styles.grid}>
        {kids.map(kid => {
          const b = breakdownFor(kid.chores);
          const corePct = b.coreTotal > 0 ? (b.coreCompleted / b.coreTotal) * 100 : 0;
          const bonusWidthOfRemaining = b.coreTotal > 0
            ? (1 - b.coreCompleted / b.coreTotal) * (b.bonusTotal > 0 ? (b.bonusCompleted / b.bonusTotal) : 0) * 100
            : b.bonusTotal > 0 ? (b.bonusCompleted / b.bonusTotal) * 100 : 0;
          const allCoreDone = b.coreTotal > 0 && b.coreCompleted === b.coreTotal;
          const hasAlert = b.overdue > 0;
          const isExpanded = expanded.has(kid.user.id);
          const totalCompleted = b.coreCompleted + b.bonusCompleted;
          const totalChores = b.coreTotal + b.bonusTotal;

          return (
            <div
              key={kid.user.id}
              className={clsx(
                styles.card,
                kid.user.paused && styles.cardPaused,
                hasAlert && styles.cardAlert,
                !hasAlert && allCoreDone && styles.cardDone,
              )}
            >
              <div
                className={styles.header}
                onClick={() => toggleExpand(kid.user.id)}
                role="button"
                tabIndex={0}
                onKeyDown={e => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    toggleExpand(kid.user.id);
                  }
                }}
              >
                <div className={styles.avatar}>
                  {kid.user.avatar_url
                    ? <img src={kid.user.avatar_url} alt={kid.user.name} />
                    : <div className={styles.avatarPlaceholder}>{initialsFor(kid.user.name)}</div>}
                </div>

                <div className={styles.info}>
                  <div className={styles.nameRow}>
                    <span className={styles.name}>{kid.user.name}</span>
                    {kid.user.paused && <span className={styles.pausedTag}>Paused</span>}
                    {hasAlert && (
                      <span className={styles.alertTag}>
                        <AlertTriangle size={11} /> {b.overdue} overdue
                      </span>
                    )}
                    {!hasAlert && allCoreDone && (
                      <span className={styles.doneTag}>
                        <Check size={11} /> All done
                      </span>
                    )}
                  </div>

                  <div className={styles.progressText}>
                    {totalChores === 0 ? (
                      <span className={styles.progressTextMuted}>No chores scheduled today</span>
                    ) : (
                      <>
                        <span>
                          <strong>{b.coreCompleted}</strong>
                          <span className={styles.progressTextMuted}>/{b.coreTotal}</span> core
                        </span>
                        {b.bonusTotal > 0 && (
                          <span className={styles.progressTextMuted}>
                            · <strong style={{ color: 'var(--text-primary)' }}>{b.bonusCompleted}</strong>/{b.bonusTotal} bonus
                          </span>
                        )}
                        <span className={styles.progressTextMuted}>· {totalCompleted}/{totalChores} total</span>
                      </>
                    )}
                  </div>

                  <div className={styles.progressBar}>
                    {b.coreTotal > 0 && (
                      <div
                        className={allCoreDone ? styles.progressFillDone : styles.progressFillCore}
                        style={{ width: `${corePct}%` }}
                      />
                    )}
                    {b.bonusTotal > 0 && (
                      <div
                        className={styles.progressFillBonus}
                        style={{ width: `${bonusWidthOfRemaining}%` }}
                      />
                    )}
                  </div>

                  <div className={styles.statsRow}>
                    <span className={clsx(styles.stat, styles.statStreak)}>
                      <Flame size={13} /> {kid.streak}d streak
                    </span>
                    <span className={clsx(styles.stat, styles.statPoints)}>
                      <Star size={13} /> {kid.balance} pts
                    </span>
                    {kid.pendingApprovals > 0 && (
                      <span className={clsx(styles.stat, styles.statPending)}>
                        <Clock size={13} /> {kid.pendingApprovals} awaiting approval
                      </span>
                    )}
                    {b.pendingOnToday > 0 && b.pendingOnToday !== kid.pendingApprovals && (
                      <span className={clsx(styles.stat, styles.statPending)}>
                        <Clock size={13} /> {b.pendingOnToday} pending today
                      </span>
                    )}
                  </div>
                </div>

                <ChevronDown
                  size={20}
                  className={clsx(styles.caret, isExpanded && styles.caretOpen)}
                />
              </div>

              {isExpanded && (
                <div className={styles.details}>
                  {kid.loadError && (
                    <div className={styles.error}>Couldn't load this child's chores.</div>
                  )}
                  {!kid.loadError && kid.chores.length === 0 && (
                    <div className={styles.emptyChore}>No chores scheduled today.</div>
                  )}
                  {!kid.loadError && kid.chores.length > 0 && (
                    <>
                      {(['required', 'core', 'bonus'] as const).map(cat => {
                        const items = kid.chores.filter(c => c.category === cat);
                        if (items.length === 0) return null;
                        const label = cat === 'required' ? 'Required' : cat === 'core' ? 'Core' : 'Bonus';
                        return (
                          <div key={cat} className={styles.categorySection}>
                            <span className={styles.categoryLabel}>{label}</span>
                            <div className={styles.choreList}>
                              {items.map(c => {
                                const isOverdue = !c.completed && c.expired && c.category !== 'bonus';
                                const isPending = c.completion_status === 'pending';
                                return (
                                  <div key={c.schedule_id + '-' + c.date} className={styles.choreItem}>
                                    {c.completed ? (
                                      <Check size={14} className={clsx(styles.choreIcon, styles.choreIconDone)} />
                                    ) : isOverdue ? (
                                      <AlertTriangle size={14} className={clsx(styles.choreIcon, styles.choreIconOverdue)} />
                                    ) : (
                                      <Circle size={14} className={styles.choreIcon} />
                                    )}
                                    <span className={clsx(styles.choreTitle, c.completed && styles.choreTitleDone)}>
                                      {c.title}
                                    </span>
                                    {c.completed && isPending && (
                                      <span className={clsx(styles.choreStatus, styles.statusPending)}>Pending</span>
                                    )}
                                    {c.completed && !isPending && (
                                      <span className={clsx(styles.choreStatus, styles.statusDone)}>
                                        +{c.points_value}
                                      </span>
                                    )}
                                    {!c.completed && isOverdue && (
                                      <span className={clsx(styles.choreStatus, styles.statusOverdue)}>Overdue</span>
                                    )}
                                    {!c.completed && !isOverdue && (
                                      <span className={clsx(styles.choreStatus, styles.statusIdle)}>
                                        {c.points_value} pts
                                      </span>
                                    )}
                                  </div>
                                );
                              })}
                            </div>
                          </div>
                        );
                      })}
                    </>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      <p className={styles.refreshHint} style={{ marginTop: '1rem', textAlign: 'center' }}>
        Tap a child to see their chores for today.
      </p>
    </div>
  );
};
