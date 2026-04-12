import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { User, PointTransaction } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Star, Gift, Coins, Flame, Undo2, Activity } from 'lucide-react';
import clsx from 'clsx';

export const ActivityTab: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [transactions, setTransactions] = useState<PointTransaction[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const usrs = await api.users.list();
      const children = usrs.filter((u: User) => u.role === 'child');
      setUsers(children);

      // Fetch transactions for all children
      const allTxns = await Promise.all(
        children.map(async (u: User) => {
          const data = await api.points.getForUser(u.id);
          return data.transactions.map(t => ({ ...t, user_id: u.id }));
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

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || `User ${id}`;

  const formatTime = (dateStr: string) => {
    const d = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    const diffHr = Math.floor(diffMs / 3600000);

    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    if (diffHr < 24) return `${diffHr}h ago`;

    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) + ' ' +
      d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  };

  const getReasonLabel = (reason: string) => {
    switch (reason) {
      case 'chore_complete': return 'Chore completed';
      case 'chore_uncomplete': return 'Chore undone';
      case 'reward_redeem': return 'Reward redeemed';
      case 'streak_bonus': return 'Streak bonus';
      case 'admin_adjust': return 'Admin adjustment';
      case 'expiry_penalty': return 'Late penalty';
      case 'points_decay': return 'Points decay';
      case 'missed_chore': return 'Missed chore penalty';
      default: return reason;
    }
  };

  const getReasonIcon = (reason: string) => {
    switch (reason) {
      case 'chore_complete': return <Star size={14} style={{ color: '#22c55e' }} />;
      case 'chore_uncomplete': return <Undo2 size={14} style={{ color: '#ef4444' }} />;
      case 'reward_redeem': return <Gift size={14} style={{ color: '#a78bfa' }} />;
      case 'streak_bonus': return <Flame size={14} style={{ color: '#f59e0b' }} />;
      case 'admin_adjust': return <Coins size={14} style={{ color: '#38bdf8' }} />;
      default: return <Activity size={14} />;
    }
  };

  const handleUndo = async (txn: PointTransaction) => {
    if (txn.reason === 'reward_redeem' && txn.reference_id) {
      await api.rewards.undoRedemption(txn.reference_id);
    } else {
      const note = `Undo: ${txn.note || getReasonLabel(txn.reason)}`;
      await api.points.adjust(txn.user_id, -txn.amount, note);
    }
    load();
  };

  if (loading) return <p className={styles.emptyText}>Loading...</p>;

  return (
    <div>
      <h2 className={styles.sectionTitle}>Activity Log</h2>
      <p className={styles.sectionSubtitle}>{transactions.length} events</p>

      <div className={styles.activityList}>
        {transactions.length === 0 && <p className={styles.emptyText}>No activity yet</p>}
        {transactions.map(txn => (
          <div key={`${txn.user_id}-${txn.id}`} className={styles.activityItem}>
            <div className={styles.activityIcon}>{getReasonIcon(txn.reason)}</div>
            <div className={styles.activityInfo}>
              <div className={styles.activityMain}>
                <span className={styles.activityUser}>{getUserName(txn.user_id)}</span>
                <span className={styles.activityReason}>{getReasonLabel(txn.reason)}</span>
              </div>
              {txn.note && <div className={styles.activityNote}>{txn.note}</div>}
              <div className={styles.activityTime}>{formatTime(txn.created_at)}</div>
            </div>
            <div className={clsx(styles.activityAmount, txn.amount > 0 ? styles.activityAmountPos : styles.activityAmountNeg)}>
              {txn.amount > 0 ? '+' : ''}{txn.amount}
            </div>
            <button
              className={clsx(styles.iconBtn, styles.iconBtnSm)}
              title="Undo this event"
              aria-label="Undo this event"
              onClick={() => handleUndo(txn)}
            >
              <Undo2 size={14} />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
};
