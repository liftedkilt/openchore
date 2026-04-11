import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { User, Theme, UserDecayConfig } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Edit2, X, Save, Clock, Pause, Play, KeyRound } from 'lucide-react';
import clsx from 'clsx';

const DecayConfigEditor: React.FC<{ userId: number }> = ({ userId }) => {
  const [config, setConfig] = useState<UserDecayConfig | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [rate, setRate] = useState('5');
  const [intervalHours, setIntervalHours] = useState('24');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api.decay.getConfig(userId).then(cfg => {
      setConfig(cfg);
      setEnabled(cfg.enabled);
      setRate(cfg.decay_rate.toString());
      setIntervalHours(cfg.decay_interval_hours.toString());
    });
  }, [userId]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const updated = await api.decay.setConfig(userId, {
        enabled,
        decay_rate: parseInt(rate) || 5,
        decay_interval_hours: parseInt(intervalHours) || 24,
      });
      setConfig(updated);
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  if (!config) return <div className={styles.scheduleSection} style={{ padding: '0.5rem' }}>Loading...</div>;

  return (
    <div className={styles.scheduleSection}>
      <div className={styles.scheduleHeader}>
        <span className={styles.scheduleTitle}>Points Decay</span>
      </div>
      <div className={styles.scheduleForm}>
        <div className={styles.formGroup}>
          <label className={clsx(styles.label, styles.flexRow)}>
            <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
            Enable points decay
          </label>
          <span className={styles.helpText}>When enabled, points are deducted if non-bonus chores were not all completed the previous day.</span>
        </div>
        {enabled && (
          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label}>Points to deduct</label>
              <input className={styles.input} type="number" min="1" value={rate} onChange={e => setRate(e.target.value)} />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>Check every (hours)</label>
              <input className={styles.input} type="number" min="1" value={intervalHours} onChange={e => setIntervalHours(e.target.value)} />
            </div>
          </div>
        )}
        <button className={styles.btnPrimary} onClick={handleSave} disabled={saving} style={{ marginTop: '0.5rem' }}>
          <Save size={14} /> Save
        </button>
      </div>
    </div>
  );
};

const UserForm: React.FC<{
  user: User | null;
  onSave: () => void;
  onCancel: () => void;
}> = ({ user, onSave, onCancel }) => {
  const [name, setName] = useState(user?.name || '');
  const [role, setRole] = useState(user?.role || 'child');
  const [age, setAge] = useState(user?.age?.toString() || '');
  const [userTheme, setUserTheme] = useState<Theme>(user?.theme || 'default');
  const [saving, setSaving] = useState(false);

  const isChild = role === 'child';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const data: Partial<User> = {
        name,
        role: role as 'admin' | 'child',
        age: age ? parseInt(age) : undefined,
        theme: isChild ? userTheme : 'default',
        avatar_url: `https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=${encodeURIComponent(name)}`,
      };
      if (user) {
        await api.users.update(user.id, data);
      } else {
        await api.users.create(data);
      }
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.formHeader}>
        <h3>{user ? 'Edit Person' : 'New Person'}</h3>
        <button type="button" className={styles.iconBtn} onClick={onCancel}><X size={18} /></button>
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formRow}>
          <div className={styles.formGroup}>
            <label className={styles.label}>Name</label>
            <input className={styles.input} value={name} onChange={e => setName(e.target.value)} required placeholder="Name" />
          </div>
          <div className={styles.formGroup}>
            <label className={styles.label}>Role</label>
            <select className={styles.input} value={role} onChange={e => setRole(e.target.value)}>
              <option value="child">Child</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <div className={styles.formGroup}>
            <label className={styles.label}>Age</label>
            <input className={styles.input} type="number" min="1" max="99" value={age} onChange={e => setAge(e.target.value)} placeholder="Optional" />
          </div>
        </div>
        {isChild && (
          <div className={styles.formGroup}>
            <label className={styles.label}>Theme</label>
            <select className={styles.input} value={userTheme} onChange={e => setUserTheme(e.target.value as Theme)}>
              <option value="default">🌊 Classic</option>
              <option value="quest">⚔️ Quest</option>
              <option value="galaxy">🚀 Galaxy</option>
              <option value="forest">🌲 Forest</option>
            </select>
          </div>
        )}
      </div>

      <div className={styles.formActions}>
        <button type="button" className={styles.btnSecondary} onClick={onCancel}>Cancel</button>
        <button type="submit" className={styles.btnPrimary} disabled={saving || !name}>
          <Save size={16} /> {user ? 'Update' : 'Create'}
        </button>
      </div>
    </form>
  );
};

export const UsersTab: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [expandedDecay, setExpandedDecay] = useState<number | null>(null);

  const load = useCallback(async () => {
    const u = await api.users.list();
    setUsers(u);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id: number) => {
    await api.users.delete(id);
    load();
  };

  const handleTogglePause = async (user: User) => {
    try {
      if (user.paused) {
        await api.users.unpause(user.id);
      } else {
        await api.users.pause(user.id);
      }
      load();
    } catch (err) {
      console.error(err);
    }
  };

  const handleClearPin = async (user: User) => {
    if (!confirm(`Reset ${user.name}'s profile PIN? They will be able to log in without a PIN until they set a new one.`)) return;
    try {
      await api.users.clearPin(user.id);
      load();
    } catch (err) {
      console.error(err);
    }
  };

  const handleSaved = () => {
    setShowForm(false);
    setEditingUser(null);
    load();
  };

  return (
    <div>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Family Members</h2>
        <button className={styles.addBtn} onClick={() => { setEditingUser(null); setShowForm(true); }}>
          <Plus size={18} /> Add Person
        </button>
      </div>

      {showForm && (
        <UserForm
          user={editingUser}
          onSave={handleSaved}
          onCancel={() => { setShowForm(false); setEditingUser(null); }}
        />
      )}

      <div className={styles.list}>
        {users.map(u => (
          <div key={u.id} className={clsx(styles.listItem, u.paused && styles.listItemPaused)}>
            <div className={styles.listItemMain}>
              <div className={styles.userAvatar}>
                {u.avatar_url ? <img src={u.avatar_url} alt={u.name} /> : <div className={styles.userAvatarPlaceholder} />}
              </div>
              <div className={styles.listItemInfo}>
                <h3 className={styles.listItemTitle}>{u.name}</h3>
                <div className={styles.listItemMeta}>
                  <span className={clsx(styles.badge, u.role === 'admin' ? styles.badge_admin : styles.badge_child)}>{u.role}</span>
                  {u.paused && <span className={clsx(styles.badge, styles.badge_paused)}>Paused</span>}
                  {u.has_pin && <span className={clsx(styles.badge, styles.badge_child)}>PIN</span>}
                  {u.age && <span>Age {u.age}</span>}
                </div>
              </div>
              <div className={styles.listItemActions}>
                {u.has_pin && (
                  <button
                    className={styles.iconBtn}
                    onClick={() => handleClearPin(u)}
                    title="Reset profile PIN"
                    aria-label="Reset profile PIN"
                  >
                    <KeyRound size={16} />
                  </button>
                )}
                {u.role === 'child' && (
                  <button
                    className={clsx(styles.iconBtn, u.paused && styles.iconBtnActive)}
                    onClick={() => handleTogglePause(u)}
                    title={u.paused ? 'Unpause (resume chores)' : 'Pause (vacation/sick mode)'}
                    aria-label={u.paused ? 'Unpause user' : 'Pause user'}
                  >
                    {u.paused ? <Play size={16} /> : <Pause size={16} />}
                  </button>
                )}
                {u.role === 'child' && (
                  <button className={styles.iconBtn} onClick={() => setExpandedDecay(expandedDecay === u.id ? null : u.id)} title="Points decay settings" aria-label="Points decay settings">
                    <Clock size={16} />
                  </button>
                )}
                <button className={styles.iconBtn} aria-label="Edit user" onClick={() => { setEditingUser(u); setShowForm(true); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} aria-label="Delete user" onClick={() => handleDelete(u.id)}>
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
            {expandedDecay === u.id && u.role === 'child' && (
              <DecayConfigEditor userId={u.id} />
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
