import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import type { User, PointBalance } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Save, Star } from 'lucide-react';

export const PointsTab: React.FC = () => {
  const { t } = useTranslation();
  const [balances, setBalances] = useState<(PointBalance & { name: string })[]>([]);
  const [, setUsers] = useState<User[]>([]);
  const [adjustUser, setAdjustUser] = useState<number | null>(null);
  const [adjustAmount, setAdjustAmount] = useState('');
  const [adjustNote, setAdjustNote] = useState('');
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    const [bals, usrs] = await Promise.all([api.points.getAllBalances(), api.users.list()]);
    const children = usrs.filter((u: User) => u.role === 'child');
    setUsers(children);
    setBalances(children.map(u => {
      const b = bals.find((b: PointBalance) => b.user_id === u.id);
      return { user_id: u.id, balance: b?.balance || 0, name: u.name };
    }));
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAdjust = async () => {
    if (!adjustUser || !adjustAmount) return;
    setSaving(true);
    try {
      await api.points.adjust(adjustUser, parseInt(adjustAmount), adjustNote || 'Admin adjustment');
      setAdjustUser(null);
      setAdjustAmount('');
      setAdjustNote('');
      load();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <div>
      <h2 className={styles.sectionTitle}>{t('admin.pointsTab.heading')}</h2>

      <div className={styles.balanceGrid}>
        {balances.map(b => (
          <div key={b.user_id} className={styles.balanceCard}>
            <div className={styles.balanceName}>{b.name}</div>
            <div className={styles.balanceAmount}>
              <Star size={16} className={styles.balanceIcon} />
              {b.balance}
            </div>
            <button
              className={styles.adjustBtn}
              onClick={() => setAdjustUser(adjustUser === b.user_id ? null : b.user_id)}
            >
              {adjustUser === b.user_id ? t('admin.pointsTab.cancelButton') : t('admin.pointsTab.adjustButton')}
            </button>

            {adjustUser === b.user_id && (
              <div className={styles.adjustForm}>
                <div className={styles.formRow}>
                  <div className={styles.formGroup}>
                    <label className={styles.label}>{t('admin.pointsTab.amountLabel')}</label>
                    <input
                      className={styles.input}
                      type="number"
                      value={adjustAmount}
                      onChange={e => setAdjustAmount(e.target.value)}
                      placeholder={t('admin.pointsTab.amountPlaceholder')}
                    />
                  </div>
                  <div className={styles.formGroup} style={{ flex: 2 }}>
                    <label className={styles.label}>{t('admin.pointsTab.reasonLabel')}</label>
                    <input
                      className={styles.input}
                      value={adjustNote}
                      onChange={e => setAdjustNote(e.target.value)}
                      placeholder={t('admin.pointsTab.reasonPlaceholder')}
                    />
                  </div>
                </div>
                <button className={styles.btnPrimary} onClick={handleAdjust} disabled={saving || !adjustAmount}>
                  <Save size={14} /> {t('admin.pointsTab.applyButton')}
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
