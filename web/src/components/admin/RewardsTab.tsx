import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { User, Reward, StreakRewardItem } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Edit2, X, Save, Users, Star, ChevronDown, ChevronUp, Flame } from 'lucide-react';
import clsx from 'clsx';

const RewardAssignmentEditor: React.FC<{
  reward: Reward;
  users: User[];
  onSave: () => void;
}> = ({ reward, users, onSave }) => {
  const [assignments, setAssignments] = useState<{ user_id: number; custom_cost: string; enabled: boolean }[]>(
    users.map(u => {
      const existing = reward.assignments?.find(a => a.user_id === u.id);
      return {
        user_id: u.id,
        custom_cost: existing?.custom_cost?.toString() || '',
        enabled: !!existing,
      };
    })
  );
  const [saving, setSaving] = useState(false);

  const toggle = (userId: number) => {
    setAssignments(prev => prev.map(a => a.user_id === userId ? { ...a, enabled: !a.enabled } : a));
  };

  const setCost = (userId: number, val: string) => {
    setAssignments(prev => prev.map(a => a.user_id === userId ? { ...a, custom_cost: val } : a));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const enabled = assignments.filter(a => a.enabled);
      // If all kids are enabled with no custom costs, clear assignments (= available to all)
      const allEnabled = enabled.length === users.length && enabled.every(a => !a.custom_cost);
      const payload = allEnabled ? [] : enabled.map(a => ({
        user_id: a.user_id,
        custom_cost: a.custom_cost ? parseInt(a.custom_cost) : undefined,
      }));
      await api.rewards.setAssignments(reward.id, payload);
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  const anyAssigned = assignments.some(a => a.enabled);

  return (
    <div className={styles.assignmentEditor}>
      <div className={styles.assignmentHint}>
        {anyAssigned ? 'Only checked kids can see this reward.' : 'No restrictions — all kids can see this reward.'}
      </div>
      {assignments.map(a => {
        const user = users.find(u => u.id === a.user_id);
        if (!user) return null;
        return (
          <div key={a.user_id} className={styles.assignmentRow}>
            <label className={styles.assignmentCheck}>
              <input type="checkbox" checked={a.enabled} onChange={() => toggle(a.user_id)} />
              <span>{user.name}</span>
            </label>
            {a.enabled && (
              <div className={styles.assignmentCost}>
                <input
                  className={styles.input}
                  type="number"
                  min="1"
                  value={a.custom_cost}
                  onChange={e => setCost(a.user_id, e.target.value)}
                  placeholder={`${reward.cost} (default)`}
                />
                <span className={styles.assignmentCostLabel}>pts</span>
              </div>
            )}
          </div>
        );
      })}
      <button className={styles.btnPrimary} onClick={handleSave} disabled={saving} style={{ marginTop: '0.5rem' }}>
        <Save size={14} /> Save Assignments
      </button>
    </div>
  );
};

const RewardForm: React.FC<{
  reward: Reward | null;
  users: User[];
  onSave: () => void;
  onCancel: () => void;
}> = ({ reward, onSave, onCancel }) => {
  const [name, setName] = useState(reward?.name || '');
  const [description, setDescription] = useState(reward?.description || '');
  const [icon, setIcon] = useState(reward?.icon || '');
  const [cost, setCost] = useState(reward?.cost?.toString() || '50');
  const [stock, setStock] = useState(reward?.stock?.toString() || '');
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const data = {
        name,
        description,
        icon,
        cost: parseInt(cost) || 0,
        stock: stock ? parseInt(stock) : undefined,
        active: true,
      };
      if (reward) {
        await api.rewards.update(reward.id, data);
      } else {
        await api.rewards.create(data);
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
        <h3>{reward ? 'Edit Reward' : 'New Reward'}</h3>
        <button type="button" className={styles.iconBtn} onClick={onCancel}><X size={18} /></button>
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formRow}>
          <div className={styles.formGroup} style={{ flex: 3 }}>
            <label className={styles.label}>Name</label>
            <input className={styles.input} value={name} onChange={e => setName(e.target.value)} required placeholder="e.g. Ice Cream Trip" />
          </div>
          <div className={styles.formGroup} style={{ flex: 1 }}>
            <label className={styles.label}>Icon</label>
            <input className={styles.input} value={icon} onChange={e => setIcon(e.target.value)} placeholder="emoji" style={{ textAlign: 'center', fontSize: '1.5rem' }} />
          </div>
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label}>Description</label>
          <input className={styles.input} value={description} onChange={e => setDescription(e.target.value)} placeholder="What do they get?" />
        </div>

        <div className={styles.formRow}>
          <div className={styles.formGroup}>
            <label className={styles.label} title="How many points the child needs to redeem this reward.">Cost (pts)</label>
            <input className={styles.input} type="number" min="1" value={cost} onChange={e => setCost(e.target.value)} />
          </div>
          <div className={styles.formGroup}>
            <label className={styles.label} title="Limit how many times this reward can be redeemed. Leave blank for unlimited.">Stock (blank = unlimited)</label>
            <input className={styles.input} type="number" min="0" value={stock} onChange={e => setStock(e.target.value)} placeholder="∞" />
          </div>
        </div>
      </div>

      <div className={styles.formActions}>
        <button type="button" className={styles.btnSecondary} onClick={onCancel}>Cancel</button>
        <button type="submit" className={styles.btnPrimary} disabled={saving || !name || !cost}>
          <Save size={16} /> {reward ? 'Update' : 'Create'}
        </button>
      </div>
    </form>
  );
};

const StreakRewardForm: React.FC<{ onSave: () => void }> = ({ onSave }) => {
  const [days, setDays] = useState('7');
  const [points, setPoints] = useState('25');
  const [label, setLabel] = useState('');
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.streaks.createReward({
        streak_days: parseInt(days) || 0,
        bonus_points: parseInt(points) || 0,
        label: label || `${days}-Day Streak!`,
      });
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.formRow}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Days</label>
          <input className={styles.input} type="number" min="1" value={days} onChange={e => setDays(e.target.value)} />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Bonus Pts</label>
          <input className={styles.input} type="number" min="1" value={points} onChange={e => setPoints(e.target.value)} />
        </div>
        <div className={styles.formGroup} style={{ flex: 2 }}>
          <label className={styles.label}>Label</label>
          <input className={styles.input} value={label} onChange={e => setLabel(e.target.value)} placeholder="e.g. Week Warrior!" />
        </div>
      </div>
      <div className={styles.formActions}>
        <button type="submit" className={styles.btnPrimary} disabled={saving || !days || !points}>
          <Save size={16} /> Add Milestone
        </button>
      </div>
    </form>
  );
};

export const RewardsTab: React.FC = () => {
  const [rewards, setRewards] = useState<Reward[]>([]);
  const [streakRewards, setStreakRewards] = useState<StreakRewardItem[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingReward, setEditingReward] = useState<Reward | null>(null);
  const [showStreakForm, setShowStreakForm] = useState(false);
  const [expandedAssignments, setExpandedAssignments] = useState<number | null>(null);

  const load = useCallback(async () => {
    const [r, sr, u] = await Promise.all([api.rewards.listAll(), api.streaks.listRewards(), api.users.list()]);
    setRewards(r);
    setStreakRewards(sr);
    setUsers(u.filter((u: User) => u.role === 'child'));
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDeleteReward = async (id: number) => {
    await api.rewards.delete(id);
    load();
  };

  const handleDeleteStreakReward = async (id: number) => {
    await api.streaks.deleteReward(id);
    load();
  };

  const toggleAssignments = (id: number) => {
    setExpandedAssignments(expandedAssignments === id ? null : id);
  };

  return (
    <div>
      {/* Rewards */}
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Rewards Store</h2>
        <button className={styles.addBtn} onClick={() => { setEditingReward(null); setShowForm(true); }}>
          <Plus size={18} /> Add Reward
        </button>
      </div>

      {showForm && (
        <RewardForm
          reward={editingReward}
          users={users}
          onSave={() => { setShowForm(false); setEditingReward(null); load(); }}
          onCancel={() => { setShowForm(false); setEditingReward(null); }}
        />
      )}

      <div className={styles.list}>
        {rewards.length === 0 && <p className={styles.emptyText}>No rewards yet</p>}
        {rewards.map(r => (
          <div key={r.id} className={styles.listItem}>
            <div className={styles.listItemMain}>
              {r.icon && <span className={styles.rewardIconLg}>{r.icon}</span>}
              <div className={styles.listItemInfo}>
                <h3 className={styles.listItemTitle}>{r.name}</h3>
                {r.description && <p className={styles.listItemDesc}>{r.description}</p>}
                <div className={styles.listItemMeta}>
                  <span><Star size={12} /> {r.cost} pts</span>
                  <span>{r.stock !== null && r.stock !== undefined ? `${r.stock} in stock` : 'Unlimited'}</span>
                  <span className={r.active ? styles.statusActive : styles.statusInactive}>
                    {r.active ? 'Active' : 'Inactive'}
                  </span>
                </div>
                <button className={styles.assignmentToggle} onClick={() => toggleAssignments(r.id)}>
                  <Users size={12} />
                  {r.assignments && r.assignments.length > 0
                    ? `${r.assignments.length} kid${r.assignments.length > 1 ? 's' : ''} assigned`
                    : 'All kids (no restrictions)'}
                  {expandedAssignments === r.id ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                </button>
              </div>
              <div className={styles.listItemActions}>
                <button className={styles.iconBtn} aria-label="Edit reward" onClick={() => { setEditingReward(r); setShowForm(true); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} aria-label="Delete reward" onClick={() => handleDeleteReward(r.id)}>
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
            {expandedAssignments === r.id && (
              <RewardAssignmentEditor reward={r} users={users} onSave={load} />
            )}
          </div>
        ))}
      </div>

      {/* Streak Milestones */}
      <div className={styles.sectionHeader} style={{ marginTop: '2rem' }}>
        <h2 className={styles.sectionTitle}>
          <Flame size={18} style={{ color: '#f59e0b', marginRight: '0.4rem' }} />
          Streak Milestones
        </h2>
        <button className={styles.addBtn} onClick={() => setShowStreakForm(!showStreakForm)}>
          {showStreakForm ? <X size={18} /> : <Plus size={18} />}
          {showStreakForm ? 'Cancel' : 'Add'}
        </button>
      </div>

      {showStreakForm && <StreakRewardForm onSave={() => { setShowStreakForm(false); load(); }} />}

      <div className={styles.list}>
        {streakRewards.length === 0 && <p className={styles.emptyText}>No streak milestones yet</p>}
        {streakRewards.map(sr => (
          <div key={sr.id} className={styles.listItem}>
            <div className={styles.listItemMain}>
              <div className={styles.streakBadge}>{sr.streak_days}d</div>
              <div className={styles.listItemInfo}>
                <h3 className={styles.listItemTitle}>{sr.label || `${sr.streak_days}-Day Streak`}</h3>
                <div className={styles.listItemMeta}>
                  <span><Star size={12} /> +{sr.bonus_points} bonus pts</span>
                </div>
              </div>
              <div className={styles.listItemActions}>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} aria-label="Delete streak reward" onClick={() => handleDeleteStreakReward(sr.id)}>
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
