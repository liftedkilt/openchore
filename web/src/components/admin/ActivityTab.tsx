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
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      // Parents take part too, so everyone's activity is listed.
      const usrs = await api.users.list();
      setUsers(usrs);

      const allTxns = await Promise.all(
        usrs.map(async (u: User) => {
          const data = await api.points.getForUser(u.id);
          return data.transactions.map(tx => ({ ...tx, user_id: u.id }));
        })
      );
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

  const handleUndo = async (txn: PointTransaction) => {
    if (txn.reason === 'reward_redeem' && txn.reference_id) {
      await api.rewards.undoRedemption(txn.reference_id);
    } else {
      // An adjustment, so the undo is itself a point_transactions row.
      const note = `Undo: ${txn.note || getReasonLabel(txn.reason)}`;
      await api.points.adjust(txn.user_id, -txn.amount, note);
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
                    <span className={styles.reason}>{getReasonLabel(txn.reason)}</span>
                  </span>
                  {txn.note && <span className={ui.rowDesc}>{txn.note}</span>}
                  <span className={styles.time}>{formatTime(txn.created_at)}</span>
                </div>
                <span className={clsx(styles.amount, txn.amount < 0 && styles.amountNeg)}>
                  {txn.amount > 0 ? '+' : txn.amount < 0 ? '−' : ''}{Math.abs(txn.amount)}
                </span>
                <button
                  type="button"
                  className={ui.iconBtn}
                  title={t('admin.activityTab.undoTitle')}
                  aria-label={t('admin.activityTab.undoTitle')}
                  onClick={() => handleUndo(txn)}
                >
                  <Undo2 aria-hidden />
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};
