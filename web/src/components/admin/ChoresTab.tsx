import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Trash2 } from 'lucide-react';
import { api } from '../../api';
import type { Chore, User } from '../../types';
import { CategoryHeader, Icon, catFromCategory } from '../../design';
import CreateChoreWizard from '../CreateChoreWizard/CreateChoreWizard';
import EditChoreModal from '../EditChoreModal/EditChoreModal';
import { ScheduleManager } from './ScheduleManager';
import { TriggerManager } from './TriggerManager';
import { CHORE_CATEGORIES, IconWell } from './pickers';
import ui from './ui.module.css';

export const ChoresTab: React.FC = () => {
  const { t } = useTranslation();
  const [chores, setChores] = useState<Chore[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  const [wizardOpen, setWizardOpen] = useState(false);

  const load = useCallback(async () => {
    const [c, u] = await Promise.all([api.chores.list(), api.users.list()]);
    setChores(c);
    setUsers(u);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (chore: Chore) => {
    if (!confirm(t('admin.choresTab.confirmDelete', { name: chore.title }))) return;
    await api.chores.delete(chore.id);
    load();
  };

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.choresTab.heading')}</h2>
          <p className={ui.pageSub}>{t('admin.choresTab.subtitle', { count: chores.length })}</p>
        </div>
        <button type="button" className={ui.btnPrimary} onClick={() => setWizardOpen(true)}>
          <Icon name="plus" /> {t('admin.choresTab.addChore')}
        </button>
      </div>

      {editingChore && (
        <EditChoreModal
          key={editingChore.id}
          chore={editingChore}
          isOpen={!!editingChore}
          onClose={() => { setEditingChore(null); load(); }}
          onSaved={load}
          users={users}
          renderSchedules={(choreId, users) => <ScheduleManager choreId={choreId} users={users} />}
          renderTriggers={(choreId, users) => <TriggerManager choreId={choreId} users={users} />}
        />
      )}

      <CreateChoreWizard
        isOpen={wizardOpen}
        onClose={() => setWizardOpen(false)}
        onComplete={() => {
          setWizardOpen(false);
          load();
        }}
        users={users}
      />

      {chores.length === 0 && (
        <div className={ui.empty}>
          <Icon name="broom" />
          <p>{t('admin.choresTab.empty')}</p>
        </div>
      )}

      {CHORE_CATEGORIES.map(category => {
        const items = chores.filter(c => c.category === category);
        if (items.length === 0) return null;
        const cat = catFromCategory(category);
        return (
          <section key={category} className={ui.section}>
            <CategoryHeader cat={cat} as="h3" count={items.length} className={ui.catHead} />
            <div className={ui.list}>
              {items.map(chore => (
                <div key={chore.id} className={ui.row}>
                  <button
                    type="button"
                    className={ui.rowButton}
                    onClick={() => setEditingChore(chore)}
                    title={t('admin.choresTab.editTitle')}
                  >
                    <IconWell icon={chore.icon} cat={cat} />
                    <span className={ui.rowMain}>
                      <span className={ui.rowTitle}>{chore.title}</span>
                      {chore.description && <span className={ui.rowDesc}>{chore.description}</span>}
                      <span className={ui.rowMeta}>
                        <span><Icon name="star" /> {t('admin.choresTab.points', { count: chore.points_value })}</span>
                        {chore.estimated_minutes ? (
                          <span><Icon name="clock" /> {t('admin.choresTab.minutes', { count: chore.estimated_minutes })}</span>
                        ) : null}
                        {chore.requires_approval && (
                          <span title={t('admin.choresTab.requiresApprovalTitle')}><Icon name="check" /> {t('admin.choresTab.requiresApprovalLabel')}</span>
                        )}
                        {chore.requires_photo && (
                          <span title={t('admin.choresTab.requiresPhotoTitle')}><Icon name="camera" /> {t('admin.choresTab.requiresPhotoLabel')}</span>
                        )}
                      </span>
                    </span>
                    <Icon name="chev" />
                  </button>
                  <div className={ui.rowActions}>
                    <button
                      type="button"
                      className={`${ui.iconBtn} ${ui.iconBtnDanger}`}
                      title={t('admin.choresTab.deleteTitle')}
                      aria-label={t('admin.choresTab.deleteAriaLabel', { name: chore.title })}
                      onClick={() => handleDelete(chore)}
                    >
                      <Trash2 aria-hidden />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
};
