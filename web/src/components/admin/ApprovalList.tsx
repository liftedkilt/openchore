import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';
import { api } from '../../api';
import { Icon, catFromCategory } from '../../design';
import type { PendingCompletion, User } from '../../types';
import { CategoryLabel, IconWell, PersonName } from './pickers';
import ui from './ui.module.css';
import styles from './ApprovalList.module.css';

interface Props {
  pending: PendingCompletion[];
  users: User[];
  /** Called after an approve or reject so the parent can reload. */
  onChanged: () => void;
}

function formatDay(date: string, lang: string): string {
  return new Date(date + 'T00:00:00').toLocaleDateString(lang, { weekday: 'short', month: 'short', day: 'numeric' });
}

/**
 * Chores waiting for a grown-up: who, what, the photo and the photo check's
 * feedback, with Approve / Reject. Parents open the app mostly for this.
 */
export const ApprovalList: React.FC<Props> = ({ pending, users, onChanged }) => {
  const { t, i18n } = useTranslation();
  const [busy, setBusy] = useState<number | null>(null);
  const [error, setError] = useState('');

  const act = async (id: number, action: 'approve' | 'reject') => {
    if (action === 'reject' && !confirm(t('admin.approvalsTab.rejectConfirm'))) return;
    setBusy(id);
    setError('');
    try {
      if (action === 'approve') await api.chores.approve(id);
      else await api.chores.reject(id);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    setBusy(null);
    onChanged();
  };

  return (
    <div className={styles.list}>
      {error && <p className={ui.msgError} role="alert">{error}</p>}
      {pending.map(p => {
        const assignee = users.find(u => u.id === p.assigned_user_id);
        const byOther = p.completed_by != null && p.completed_by !== p.assigned_user_id;
        const cat = catFromCategory(p.category);
        return (
          <article key={p.id} className={styles.card} aria-label={t('admin.approvalsTab.cardLabel', { title: p.chore_title, name: assignee?.name ?? p.child_name })}>
            <div className={styles.head}>
              <PersonName user={assignee} name={p.child_name} />
              <span className={styles.when}>{formatDay(p.completion_date, i18n.language)}</span>
            </div>

            <div className={styles.chore}>
              <IconWell icon={p.icon} cat={cat} />
              <div className={ui.rowMain}>
                <h3 className={styles.title}>{p.chore_title}</h3>
                <div className={ui.rowMeta}>
                  {p.category && <span><CategoryLabel category={p.category} /></span>}
                  {p.points_value != null && (
                    <span><Icon name="star" />{t('admin.choresTab.points', { count: p.points_value })}</span>
                  )}
                  {byOther && <span>{t('admin.approvalsTab.tickedOffBy', { name: p.child_name })}</span>}
                </div>
              </div>
            </div>

            {p.photo_url && (
              <a className={styles.photo} href={p.photo_url} target="_blank" rel="noreferrer">
                <img src={p.photo_url} alt={t('admin.approvalsTab.photoAlt')} loading="lazy" />
                <span className={styles.photoHint}><Icon name="camera" />{t('admin.approvalsTab.openPhoto')}</span>
              </a>
            )}

            {p.ai_feedback && (
              <div className={styles.ai}>
                <Icon name="spark" />
                <div>
                  <span className={styles.aiLabel}>{t('admin.approvalsTab.aiFeedback')}</span>
                  <p className={styles.aiText}>{p.ai_feedback}</p>
                </div>
              </div>
            )}

            <div className={styles.actions}>
              <button
                type="button"
                className={ui.btnPrimary}
                onClick={() => act(p.id, 'approve')}
                disabled={busy === p.id}
              >
                <Icon name="check" /> {t('admin.approvalsTab.approveButton')}
              </button>
              <button
                type="button"
                className={ui.btnGhost}
                onClick={() => act(p.id, 'reject')}
                disabled={busy === p.id}
              >
                <X aria-hidden /> {t('admin.approvalsTab.rejectButton')}
              </button>
            </div>
          </article>
        );
      })}
    </div>
  );
};
