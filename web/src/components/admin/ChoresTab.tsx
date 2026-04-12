import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { Chore, User } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Edit2, Clock, Star, Activity } from 'lucide-react';
import clsx from 'clsx';
import CreateChoreWizard from '../CreateChoreWizard/CreateChoreWizard';
import EditChoreModal from '../EditChoreModal/EditChoreModal';
import { ScheduleManager } from './ScheduleManager';
import { TriggerManager } from './TriggerManager';

export const ChoresTab: React.FC = () => {
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

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(`Delete chore "${name}"? This action cannot be undone.`)) return;
    await api.chores.delete(id);
    load();
  };

  const handleEdit = (chore: Chore) => {
    setEditingChore(chore);
  };

  return (
    <div>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>All Chores</h2>
        <button className={styles.addBtn} onClick={() => setWizardOpen(true)}>
          <Plus size={18} /> Add Chore
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

      <div className={styles.list}>
        {chores.map(chore => (
          <div key={chore.id} className={styles.listItem}>
            <div className={styles.listItemMain} onClick={() => handleEdit(chore)}>
              <div className={styles.listItemInfo}>
                <div className={styles.listItemHeader}>
                  <span className={clsx(styles.badge, styles[`badge_${chore.category}`])}>{chore.category}</span>
                  <h3 className={styles.listItemTitle}>{chore.title}</h3>
                </div>
                {chore.description && <p className={styles.listItemDesc}>{chore.description}</p>}
                <div className={styles.listItemMeta}>
                  <span><Star size={12} /> {chore.points_value} pts</span>
                  {chore.estimated_minutes && <span><Clock size={12} /> {chore.estimated_minutes}m</span>}
                  {chore.requires_approval && <span title="Requires Approval"><Activity size={12} /> Approval</span>}
                  {chore.requires_photo && <span title="Requires Photo"><Clock size={12} /> Photo</span>}
                </div>
              </div>
              <div className={styles.listItemActions}>
                <button className={styles.iconBtn} title="Edit" aria-label="Edit chore" onClick={(e) => { e.stopPropagation(); handleEdit(chore); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} title="Delete" aria-label="Delete chore" onClick={(e) => { e.stopPropagation(); handleDelete(chore.id, chore.name); }}>
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
