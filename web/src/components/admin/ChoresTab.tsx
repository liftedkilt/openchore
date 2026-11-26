import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Trash2 } from 'lucide-react';
import { api } from '../../api';
import type { Chore, ChoreSchedule, PossiblePoints, User } from '../../types';
import { localDateStr } from '../../utils';
import { Avatar, CategoryHeader, Icon, catFromCategory } from '../../design';
import CreateChoreWizard from '../CreateChoreWizard/CreateChoreWizard';
import EditChoreModal from '../EditChoreModal/EditChoreModal';
import { ScheduleManager } from './ScheduleManager';
import { TriggerManager } from './TriggerManager';
import { CHORE_CATEGORIES, IconWell, personColor } from './pickers';
import ui from './ui.module.css';
import styles from './ChoresTab.module.css';

/** A schedule still applies if it has an occurrence today or later. */
function isActive(s: ChoreSchedule, today: string): boolean {
  if (s.end_date && s.end_date < today) return false;
  if (s.specific_date && !s.recurrence_interval && s.specific_date < today) return false;
  return true;
}

export const ChoresTab: React.FC = () => {
  const { t } = useTranslation();
  const [chores, setChores] = useState<Chore[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  // null until loaded (or if loading failed): the rows then show no assignees
  // rather than a misleading "Not assigned".
  const [schedules, setSchedules] = useState<ChoreSchedule[] | null>(null);
  const [possible, setPossible] = useState<PossiblePoints[]>([]);
  const [possibleError, setPossibleError] = useState(false);
  const [wizardOpen, setWizardOpen] = useState(false);
  const today = localDateStr(new Date());
  // Saving and closing the editor can start overlapping loads; only the
  // latest one may set state.
  const loadSeq = useRef(0);

  const load = useCallback(async () => {
    const seq = ++loadSeq.current;
    const current = () => seq === loadSeq.current;
    // The extras load alongside the list but can fail on their own without
    // hiding it.
    const extras = Promise.allSettled([api.chores.listAllSchedules(), api.chores.possiblePoints(today)]);
    const [c, u] = await Promise.all([api.chores.list(), api.users.list()]);
    if (!current()) return;
    setChores(c);
    setUsers(u);
    const [s, p] = await extras;
    if (!current()) return;
    if (s.status === 'fulfilled') setSchedules(s.value);
    else console.error('Failed to load schedules', s.reason);
    if (p.status === 'fulfilled') {
      setPossible(p.value);
      setPossibleError(false);
    } else {
      console.error('Failed to load possible points', p.reason);
      setPossibleError(true);
    }
  }, [today]);

  // People with something to earn this week, in profile order.
  const possibleRows = useMemo(() => {
    const byUser = new Map(possible.map(p => [p.user_id, p]));
    return users.flatMap(user => {
      const p = byUser.get(user.id);
      return p && p.week > 0 ? [{ user, today: p.today, week: p.week }] : [];
    });
  }, [possible, users]);

  // Who each chore is assigned to, in profile order.
  const assignees = useMemo(() => {
    if (!schedules) return null;
    const ids = new Map<number, Set<number>>();
    for (const s of schedules) {
      if (!isActive(s, today)) continue;
      if (!ids.has(s.chore_id)) ids.set(s.chore_id, new Set());
      ids.get(s.chore_id)!.add(s.assigned_to);
    }
    const byChore = new Map<number, User[]>();
    for (const [choreId, set] of ids) byChore.set(choreId, users.filter(u => set.has(u.id)));
    return byChore;
  }, [schedules, users, today]);

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

      {possibleError && (
        <p className={ui.msgError} role="alert">{t('admin.choresTab.possibleError')}</p>
      )}

      {possibleRows.length > 0 && (
        <section className={ui.section} aria-labelledby="chores-possible-heading">
          <div>
            <h3 id="chores-possible-heading" className={ui.sectionTitle}>
              <Icon name="star" /> {t('admin.choresTab.possibleHeading')}
            </h3>
            <p className={ui.sectionDesc}>{t('admin.choresTab.possibleDesc')}</p>
          </div>
          <div className={ui.list}>
            {possibleRows.map(p => (
              <div key={p.user.id} className={ui.row}>
                <Avatar name={p.user.name} color={personColor(p.user)} size="sm" src={p.user.avatar_url || undefined} />
                <span className={`${ui.rowTitle} ${styles.possibleName}`}>{p.user.name}</span>
                <span className={styles.possibleStat}>
                  <span className={styles.possibleLabel}>{t('admin.choresTab.possibleToday')}</span>
                  <span className={styles.possibleValue}>{t('admin.choresTab.points', { count: p.today })}</span>
                </span>
                <span className={styles.possibleStat}>
                  <span className={styles.possibleLabel}>{t('admin.choresTab.possibleWeek')}</span>
                  <span className={styles.possibleValue}>{t('admin.choresTab.points', { count: p.week })}</span>
                </span>
              </div>
            ))}
          </div>
        </section>
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
                        {chore.auto_complete && (
                          <span title={t('admin.choresTab.autoCompleteTitle')}><Icon name="lock" /> {t('admin.choresTab.autoCompleteLabel')}</span>
                        )}
                        {chore.source === 'self' && (
                          <span title={t('admin.choresTab.addedByTitle')}>
                            <Icon name="people" /> {t('admin.choresTab.addedBy', { name: users.find(u => u.id === chore.created_by)?.name ?? '?' })}
                          </span>
                        )}
                      </span>
                    </span>
                    {assignees && <AssigneeAvatars people={assignees.get(chore.id) ?? []} />}
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

function AssigneeAvatars({ people }: { people: User[] }) {
  const { t } = useTranslation();
  if (people.length === 0) {
    return <span className={styles.unassigned}>{t('admin.choresTab.unassigned')}</span>;
  }
  const label = t('admin.choresTab.assignedTo', { names: people.map(p => p.name).join(', ') });
  return (
    <span className={styles.assignees} title={label}>
      {people.map(p => (
        <Avatar key={p.id} name={p.name} color={personColor(p)} size="sm" src={p.avatar_url || undefined} />
      ))}
      <span className={ui.srOnlyText}>{label}</span>
    </span>
  );
}
