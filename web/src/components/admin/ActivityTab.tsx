import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Undo2 } from 'lucide-react';
import { api } from '../../api';
import type { User, PointTransaction } from '../../types';
import { Avatar, Icon, type IconName } from '../../design';
import { personColor } from './pickers';
import ui from './ui.module.css';
import styles from './ActivityTab.module.css';

// Goal savings rows carry machine notes ("auto:completion:34", "manual",
// "auto_revert:completion:34") that the store matches on, so they're turned
// into plain words here instead of being shown raw.
const GOAL_NOTE_KEY: { prefix: string; key: string }[] = [
  { prefix: 'auto:completion:', key: 'autoSaved' },
  { prefix: 'auto_revert:completion:', key: 'autoReturned' },
  { prefix: 'manual', key: 'manualSaved' },
];

// Undo happens on the server (POST /points/transactions/{id}/undo), and the
// entry then drops out of the history, so the log never gains "Undo:" rows.
// Rows that are themselves an undo (chore unchecked, savings returned) have
// no Undo: checking the chore again is how you reverse those.
// activeGoals holds the ids of active savings goals: a saving can only be
// taken back out of one of those (the server refuses the rest).
const canUndo = (txn: PointTransaction, activeGoals: Map<number, string>) => {
  switch (txn.reason) {
    case 'chore_complete':
    case 'reward_redeem':
    case 'admin_adjust':
    case 'streak_bonus':
      return true;
    case 'expiry_penalty':
    case 'missed_chore':
    case 'points_decay':
      return txn.amount < 0;
    case 'commit_to_goal':
      return txn.amount < 0 && txn.reference_id != null && activeGoals.has(txn.reference_id);
    default:
      return false;
  }
};

const REASON_ICON: Partial<Record<PointTransaction['reason'], IconName>> = {
  chore_complete: 'check',
  chore_uncomplete: 'back',
  reward_redeem: 'gift',
  streak_bonus: 'flame',
  admin_adjust: 'star',
  expiry_penalty: 'clock',
  missed_chore: 'clock',
  points_decay: 'moon',
  commit_to_goal: 'rocket',
  goal_break: 'rocket',
};

export const ActivityTab: React.FC = () => {
  const { t, i18n } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [transactions, setTransactions] = useState<PointTransaction[]>([]);
  const [goalNames, setGoalNames] = useState<Map<number, string>>(new Map());
  const [undoError, setUndoError] = useState('');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      // Parents take part too, so everyone's activity is listed.
      const usrs = await api.users.list();
      setUsers(usrs);

      const names = new Map<number, string>();
      const allTxns = await Promise.all(
        usrs.map(async (u: User) => {
          const data = await api.points.getForUser(u.id);
          for (const c of data.active_commitments ?? []) {
            if (c.reward_name) names.set(c.id, c.reward_name);
          }
          return data.transactions.map(tx => ({ ...tx, user_id: u.id }));
        })
      );
      setGoalNames(names);
      // Flatten and sort by date descending
      const flat = allTxns.flat().sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      setTransactions(flat);
    } catch (err) {
      console.error(err);
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const formatTime = (dateStr: string) => {
    const d = new Date(dateStr);
    const diffMs = Date.now() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    const diffHr = Math.floor(diffMs / 3600000);

    if (diffMin < 1) return t('admin.activityTab.justNow');
    if (diffMin < 60) return t('admin.activityTab.minutesAgo', { count: diffMin });
    if (diffHr < 24) return t('admin.activityTab.hoursAgo', { count: diffHr });

    return d.toLocaleDateString(i18n.language, { month: 'short', day: 'numeric' }) + ' ' +
      d.toLocaleTimeString(i18n.language, { hour: 'numeric', minute: '2-digit' });
  };

  const getReasonLabel = (reason: string) => {
    const key = `admin.activityTab.reason.${reason}`;
    return i18n.exists(key) ? t(key) : reason;
  };

  const isGoalRow = (txn: PointTransaction) =>
    txn.reason === 'commit_to_goal' || txn.reason === 'goal_break';

  // "Saved to Gift Card" when the goal is still active, else the generic label.
  const getTxnLabel = (txn: PointTransaction) => {
    const goal = isGoalRow(txn) && txn.reference_id ? goalNames.get(txn.reference_id) : undefined;
    if (goal) return t(`admin.activityTab.goalReason.${txn.reason}`, { goal });
    return getReasonLabel(txn.reason);
  };

  const getTxnNote = (txn: PointTransaction) => {
    if (!txn.note || !isGoalRow(txn)) return txn.note;
    const match = GOAL_NOTE_KEY.find(n => txn.note!.startsWith(n.prefix));
    return match ? t(`admin.activityTab.goalNote.${match.key}`) : txn.note;
  };

  const handleUndo = async (txn: PointTransaction) => {
    setUndoError('');
    try {
      await api.points.undo(txn.id);
    } catch (err) {
      setUndoError(err instanceof Error ? err.message : String(err));
    }
    load();
  };

  if (loading) return <p className={ui.emptyInline}>{t('admin.activityTab.loading')}</p>;

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.activityTab.title')}</h2>
          <p className={ui.pageSub}>{t('admin.activityTab.eventCount', { count: transactions.length })}</p>
        </div>
      </div>

      {undoError && <p className={ui.msgError} role="alert">{undoError}</p>}

      {transactions.length === 0 ? (
        <div className={ui.empty}>
          <Icon name="clock" />
          <p>{t('admin.activityTab.empty')}</p>
        </div>
      ) : (
        <ul className={clsx(ui.list, styles.list)}>
          {transactions.map(txn => {
            const user = users.find(u => u.id === txn.user_id);
            const name = user?.name ?? `User ${txn.user_id}`;
            return (
              <li key={`${txn.user_id}-${txn.id}`} className={ui.row}>
                <span className={styles.avatar}>
                  <Avatar name={name} color={personColor(user)} size="sm" />
                  <span className={styles.reasonIcon} aria-hidden>
                    <Icon name={REASON_ICON[txn.reason] ?? 'star'} />
                  </span>
                </span>
                <div className={ui.rowMain}>
                  <span className={styles.line}>
                    <span className={styles.user}>{name}</span>
                    <span className={styles.reason}>{getTxnLabel(txn)}</span>
                  </span>
                  {txn.note && <span className={ui.rowDesc}>{getTxnNote(txn)}</span>}
                  <span className={styles.time}>{formatTime(txn.created_at)}</span>
                </div>
                <span className={clsx(styles.amount, txn.amount < 0 && styles.amountNeg)}>
                  {txn.amount > 0 ? '+' : txn.amount < 0 ? '−' : ''}{Math.abs(txn.amount)}
                </span>
                {canUndo(txn, goalNames) ? (
                  <button
                    type="button"
                    className={ui.iconBtn}
                    title={t('admin.activityTab.undoTitle')}
                    aria-label={t('admin.activityTab.undoTitle')}
                    onClick={() => handleUndo(txn)}
                  >
                    <Undo2 aria-hidden />
                  </button>
                ) : (
                  <span className={styles.undoSpacer} aria-hidden />
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};
