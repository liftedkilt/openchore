import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import { Icon } from '../../design';
import type { PendingCompletion, User } from '../../types';
import { ApprovalList } from './ApprovalList';
import ui from './ui.module.css';

export const ApprovalsTab: React.FC<{ onCountChange: (count: number) => void }> = ({ onCountChange }) => {
  const { t } = useTranslation();
  const [pending, setPending] = useState<PendingCompletion[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [data, u] = await Promise.all([api.chores.listPending(), api.users.list().catch(() => [] as User[])]);
      setPending(data);
      setUsers(u);
      onCountChange(data.length);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  }, [onCountChange]);

  useEffect(() => { load(); }, [load]);

  if (loading) return <p className={ui.emptyInline}>{t('admin.approvalsTab.loading')}</p>;

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.approvalsTab.title')}</h2>
          <p className={ui.pageSub}>{t('admin.approvalsTab.waitingForReview', { count: pending.length })}</p>
        </div>
      </div>

      {pending.length === 0 ? (
        <div className={ui.empty}>
          <Icon name="check" />
          <p>{t('admin.approvalsTab.emptyState')}</p>
        </div>
      ) : (
        <ApprovalList pending={pending} users={users} onChanged={load} />
      )}
    </div>
  );
};
