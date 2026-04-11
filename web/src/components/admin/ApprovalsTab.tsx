import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import styles from '../../pages/AdminDashboard.module.css';
import { X, Check } from 'lucide-react';

export const ApprovalsTab: React.FC<{ onCountChange: (count: number) => void }> = ({ onCountChange }) => {
  const [pending, setPending] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await api.chores.listPending();
      setPending(data);
      onCountChange(data.length);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  }, [onCountChange]);

  useEffect(() => { load(); }, [load]);

  const handleApprove = async (id: number) => {
    await api.chores.approve(id);
    load();
  };

  const handleReject = async (id: number) => {
    if (!confirm('Are you sure you want to reject this completion?')) return;
    await api.chores.reject(id);
    load();
  };

  if (loading) return <p className={styles.emptyText}>Loading...</p>;

  return (
    <div>
      <h2 className={styles.sectionTitle}>Pending Approvals</h2>
      <p className={styles.sectionSubtitle}>{pending.length} chores waiting for review</p>

      <div className={styles.list}>
        {pending.length === 0 && (
          <div className={styles.emptyState}>
            <Check size={48} className={styles.emptyIcon} />
            <p>All caught up! No pending approvals.</p>
          </div>
        )}
        {pending.map(p => (
          <div key={p.id} className={styles.approvalCard}>
            <div className={styles.approvalInfo}>
              <div className={styles.approvalHeader}>
                <span className={styles.approvalUser}>{p.child_name}</span>
                <span className={styles.approvalDate}>{p.completion_date}</span>
              </div>
              <h3 className={styles.approvalTitle}>{p.chore_title}</h3>
              {p.photo_url && (
                <div className={styles.approvalPhoto}>
                  <img src={p.photo_url} alt="Proof" onClick={() => window.open(p.photo_url, '_blank')} />
                </div>
              )}
            </div>
            <div className={styles.approvalActions}>
              <button className={styles.approveBtn} onClick={() => handleApprove(p.id)}>
                <Check size={18} /> Approve
              </button>
              <button className={styles.rejectBtn} onClick={() => handleReject(p.id)}>
                <X size={18} /> Reject
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
