import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import styles from '../../pages/AdminDashboard.module.css';
import { X, Check } from 'lucide-react';

export const ApprovalsTab: React.FC<{ onCountChange: (count: number) => void }> = ({ onCountChange }) => {
  const { t } = useTranslation();
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
    if (!confirm(t('admin.approvalsTab.rejectConfirm'))) return;
    await api.chores.reject(id);
    load();
  };

  if (loading) return <p className={styles.emptyText}>{t('admin.approvalsTab.loading')}</p>;

  return (
    <div>
      <h2 className={styles.sectionTitle}>{t('admin.approvalsTab.title')}</h2>
      <p className={styles.sectionSubtitle}>{t('admin.approvalsTab.waitingForReview', { count: pending.length })}</p>

      <div className={styles.list}>
        {pending.length === 0 && (
          <div className={styles.emptyState}>
            <Check size={48} className={styles.emptyIcon} />
            <p>{t('admin.approvalsTab.emptyState')}</p>
          </div>
        )}
        {pending.map(p => (
          <div key={p.id} className={styles.approvalCard}>
            <div className={styles.approvalInfo}>
              <div className={styles.approvalHeader}>
                <span className={styles.approvalUser}>{p.child_name}</span>
                <span className={styles.approvalDate}>{new Date(p.completion_date + 'T00:00:00').toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}</span>
              </div>
              <h3 className={styles.approvalTitle}>{p.chore_title}</h3>
              {p.photo_url && (
                <div className={styles.approvalPhoto}>
                  <img src={p.photo_url} alt={t('admin.approvalsTab.photoAlt')} onClick={() => window.open(p.photo_url, '_blank')} />
                </div>
              )}
            </div>
            <div className={styles.approvalActions}>
              <button className={styles.approveBtn} onClick={() => handleApprove(p.id)}>
                <Check size={18} /> {t('admin.approvalsTab.approveButton')}
              </button>
              <button className={styles.rejectBtn} onClick={() => handleReject(p.id)}>
                <X size={18} /> {t('admin.approvalsTab.rejectButton')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
