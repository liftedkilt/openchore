import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import type { User, PointBalance } from '../../types';
import { Avatar, Icon } from '../../design';
import { personColor } from './pickers';
import ui from './ui.module.css';
import styles from './PointsTab.module.css';

export const PointsTab: React.FC = () => {
  const { t } = useTranslation();
  const [balances, setBalances] = useState<{ user: User; balance: number }[]>([]);
  const [adjustUser, setAdjustUser] = useState<number | null>(null);
  const [adjustAmount, setAdjustAmount] = useState('');
  const [adjustNote, setAdjustNote] = useState('');
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    const [bals, usrs] = await Promise.all([api.points.getAllBalances(), api.users.list()]);
    // Parents take part too, so everyone has a balance.
    setBalances(usrs.map(u => ({
      user: u,
      balance: bals.find((b: PointBalance) => b.user_id === u.id)?.balance || 0,
    })));
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleAdjust = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!adjustUser || !adjustAmount) return;
    setSaving(true);
    try {
      // Every change is written to point_transactions by the API.
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
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.pointsTab.heading')}</h2>
          <p className={ui.pageSub}>{t('admin.pointsTab.subtitle')}</p>
        </div>
      </div>

      <div className={ui.grid}>
        {balances.map(({ user, balance }) => {
          const open = adjustUser === user.id;
          const formId = `adjust-${user.id}`;
          return (
            <div key={user.id} className={ui.card} data-person={personColor(user)}>
              <div className={styles.head}>
                <Avatar name={user.name} color={personColor(user)} size="md" />
                <span className={styles.name}>{user.name}</span>
                <span className={styles.balance}>
                  <Icon name="star" fill />
                  <span aria-hidden>{balance}</span>
                  <span className={ui.srOnlyText}>{t('design.points.label', { count: balance })}</span>
                </span>
              </div>
              <button
                type="button"
                className={ui.btnGhost}
                aria-expanded={open}
                aria-controls={formId}
                onClick={() => setAdjustUser(open ? null : user.id)}
              >
                {open ? t('admin.pointsTab.cancelButton') : t('admin.pointsTab.adjustButton')}
              </button>

              {open && (
                <form id={formId} className={styles.form} onSubmit={handleAdjust}>
                  <div className={ui.formRow}>
                    <label className={ui.field}>
                      <span className={ui.label}>{t('admin.pointsTab.amountLabel')}</span>
                      <input
                        className={ui.input}
                        type="number"
                        value={adjustAmount}
                        onChange={e => setAdjustAmount(e.target.value)}
                        placeholder={t('admin.pointsTab.amountPlaceholder')}
                        autoFocus
                      />
                    </label>
                    <label className={ui.field}>
                      <span className={ui.label}>{t('admin.pointsTab.reasonLabel')}</span>
                      <input
                        className={ui.input}
                        value={adjustNote}
                        onChange={e => setAdjustNote(e.target.value)}
                        placeholder={t('admin.pointsTab.reasonPlaceholder')}
                      />
                    </label>
                  </div>
                  <button type="submit" className={ui.btnPrimary} disabled={saving || !adjustAmount}>
                    <Icon name="check" /> {t('admin.pointsTab.applyButton')}
                  </button>
                </form>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
