import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import type { Chore, User, ChoreSchedule, Reward, PointBalance, PointTransaction, StreakRewardItem, Theme, Webhook, WebhookDelivery, UserDecayConfig } from '../types';
import { DAY_NAMES } from '../types';
import styles from './AdminDashboard.module.css';
import { ArrowLeft, Plus, Trash2, Edit2, X, Save, Users, ListChecks, Clock, Star, ChevronDown, ChevronUp, CalendarPlus, Gift, Coins, Flame, Undo2, Activity, Settings, Check } from 'lucide-react';
import clsx from 'clsx';

type Tab = 'chores' | 'approvals' | 'users' | 'rewards' | 'points' | 'activity' | 'settings';

export const AdminDashboard: React.FC = () => {
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>('chores');
  const [ready, setReady] = useState(false);
  const [pendingCount, setPendingCount] = useState(0);

  // Fetch pending count periodically
  useEffect(() => {
    if (!ready) return;
    const fetchCount = () => api.chores.listPending().then(p => setPendingCount(p.length)).catch(() => {});
    fetchCount();
    const interval = setInterval(fetchCount, 30000);
    return () => clearInterval(interval);
  }, [ready]);

  // Clear admin session when navigating away via browser back button
  useEffect(() => {
    const handlePopState = () => {
      sessionStorage.removeItem('openchore_admin');
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  useEffect(() => {
    if (!sessionStorage.getItem('openchore_admin')) {
      navigate('/admin', { replace: true });
      return;
    }
    const ensureAdminUser = async () => {
      // Double-check in case sessionStorage was cleared between the outer check and this running
      if (!sessionStorage.getItem('openchore_admin')) {
        navigate('/admin', { replace: true });
        return;
      }
      try {
        const users = await api.users.list();
        const admin = users.find((u: User) => u.role === 'admin');
        if (admin) {
          localStorage.setItem('openchore_user', JSON.stringify(admin));
          setReady(true);
        } else {
          // No admin exists — redirect to setup
          localStorage.removeItem('openchore_user');
          sessionStorage.removeItem('openchore_admin');
          navigate('/setup', { replace: true });
        }
      } catch {
        navigate('/login', { replace: true });
      }
    };
    ensureAdminUser();
  }, [navigate]);

  // Block render if not authenticated (synchronous check + useEffect redirect)
  if (!ready || !sessionStorage.getItem('openchore_admin')) return null;

  return (
    <div className={styles.wrapper}>
      <header className={styles.header}>
        <button className={styles.backBtn} onClick={() => { sessionStorage.removeItem('openchore_admin'); navigate('/login'); }}>
          <ArrowLeft size={18} />
        </button>
        <h1 className={styles.title}>Admin</h1>
      </header>

      <nav className={styles.nav}>
        <button className={clsx(styles.navItem, tab === 'chores' && styles.navItemActive)} onClick={() => setTab('chores')}>
          <ListChecks size={16} /> Chores
        </button>
        <button className={clsx(styles.navItem, tab === 'approvals' && styles.navItemActive)} onClick={() => setTab('approvals')}>
          <Activity size={16} /> 
          Approvals
          {pendingCount > 0 && <span className={styles.navBadge}>{pendingCount}</span>}
        </button>
        <button className={clsx(styles.navItem, tab === 'rewards' && styles.navItemActive)} onClick={() => setTab('rewards')}>
          <Gift size={16} /> Rewards
        </button>
        <button className={clsx(styles.navItem, tab === 'points' && styles.navItemActive)} onClick={() => setTab('points')}>
          <Coins size={16} /> Points
        </button>
        <button className={clsx(styles.navItem, tab === 'activity' && styles.navItemActive)} onClick={() => setTab('activity')}>
          <Undo2 size={16} /> Log
        </button>
        <button className={clsx(styles.navItem, tab === 'users' && styles.navItemActive)} onClick={() => setTab('users')}>
          <Users size={16} /> People
        </button>
        <button className={clsx(styles.navItem, tab === 'settings' && styles.navItemActive)} onClick={() => setTab('settings')}>
          <Settings size={16} />
        </button>
      </nav>

      <main className={styles.content}>
        {tab === 'chores' && <ChoresTab />}
        {tab === 'approvals' && <ApprovalsTab onCountChange={setPendingCount} />}
        {tab === 'users' && <UsersTab />}
        {tab === 'rewards' && <RewardsTab />}
        {tab === 'points' && <PointsTab />}
        {tab === 'activity' && <ActivityTab />}
        {tab === 'settings' && <SettingsTab />}
      </main>
    </div>
  );
};

// =================== CHORES TAB ===================

const ChoresTab: React.FC = () => {
  const [chores, setChores] = useState<Chore[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  const [expandedChore, setExpandedChore] = useState<number | null>(null);
  const [autoAssign, setAutoAssign] = useState(false);

  const load = useCallback(async () => {
    const [c, u] = await Promise.all([api.chores.list(), api.users.list()]);
    setChores(c);
    setUsers(u);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id: number) => {
    await api.chores.delete(id);
    load();
  };

  const handleEdit = (chore: Chore) => {
    setEditingChore(chore);
    setShowForm(true);
  };

  const handleSaved = (newChoreId?: number) => {
    setShowForm(false);
    setEditingChore(null);
    load();
    if (newChoreId) {
      setExpandedChore(newChoreId);
      setAutoAssign(true);
    }
  };

  const childUsers = users.filter(u => u.role === 'child');

  return (
    <div>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>All Chores</h2>
        <button className={styles.addBtn} onClick={() => { setEditingChore(null); setShowForm(true); }}>
          <Plus size={18} /> Add Chore
        </button>
      </div>

      {showForm && (
        <ChoreForm
          chore={editingChore}
          users={childUsers}
          onSave={handleSaved}
          onCancel={() => { setShowForm(false); setEditingChore(null); }}
        />
      )}

      <div className={styles.list}>
        {chores.map(chore => (
          <div key={chore.id} className={styles.listItem}>
            <div className={styles.listItemMain} onClick={() => { setExpandedChore(expandedChore === chore.id ? null : chore.id); setAutoAssign(false); }}>
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
                <button className={styles.iconBtn} title="Quick Assign" onClick={(e) => { e.stopPropagation(); setExpandedChore(chore.id); setAutoAssign(true); }}>
                  <CalendarPlus size={16} />
                </button>
                <button className={styles.iconBtn} onClick={(e) => { e.stopPropagation(); handleEdit(chore); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} onClick={(e) => { e.stopPropagation(); handleDelete(chore.id); }}>
                  <Trash2 size={16} />
                </button>
                {expandedChore === chore.id ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
              </div>
            </div>
            {expandedChore === chore.id && (
              <ScheduleManager choreId={chore.id} users={childUsers} autoOpen={autoAssign} onOpened={() => setAutoAssign(false)} />
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

// =================== APPROVALS TAB ===================

const ApprovalsTab: React.FC<{ onCountChange: (count: number) => void }> = ({ onCountChange }) => {
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
    if (!confirm('Are you sure you want to reject this completion?')) return;
    await api.chores.reject(id);
    load();
  };

  if (loading) return <p className={styles.emptyText}>Loading...</p>;

  return (
    <div>
      <h2 className={styles.sectionTitle}>Pending Approvals</h2>
      <p className={styles.sectionSubtitle}>{pending.length} chores waiting for review</p>

      <div className={styles.list}>
        {pending.length === 0 && (
          <div className={styles.emptyState}>
            <Check size={48} className={styles.emptyIcon} />
            <p>All caught up! No pending approvals.</p>
          </div>
        )}
        {pending.map(p => (
          <div key={p.id} className={styles.approvalCard}>
            <div className={styles.approvalInfo}>
              <div className={styles.approvalHeader}>
                <span className={styles.approvalUser}>{p.child_name}</span>
                <span className={styles.approvalDate}>{p.completion_date}</span>
              </div>
              <h3 className={styles.approvalTitle}>{p.chore_title}</h3>
              {p.photo_url && (
                <div className={styles.approvalPhoto}>
                  <img src={p.photo_url} alt="Proof" onClick={() => window.open(p.photo_url, '_blank')} />
                </div>
              )}
            </div>
            <div className={styles.approvalActions}>
              <button className={styles.approveBtn} onClick={() => handleApprove(p.id)}>
                <Check size={18} /> Approve
              </button>
              <button className={styles.rejectBtn} onClick={() => handleReject(p.id)}>
                <X size={18} /> Reject
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

// =================== CHORE FORM ===================

const ChoreForm: React.FC<{
  chore: Chore | null;
  users: User[];
  onSave: (newChoreId?: number) => void;
  onCancel: () => void;
}> = ({ chore, users, onSave, onCancel }) => {
  const [title, setTitle] = useState(chore?.title || '');
  const [description, setDescription] = useState(chore?.description || '');
  const [category, setCategory] = useState<string>(chore?.category || 'core');
  const [points, setPoints] = useState(chore?.points_value?.toString() || '5');
  const [minutes, setMinutes] = useState(chore?.estimated_minutes?.toString() || '5');
  const [requiresApproval, setRequiresApproval] = useState(chore?.requires_approval || false);
  const [requiresPhoto, setRequiresPhoto] = useState(chore?.requires_photo || false);
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent, andAssign = false) => {
    e.preventDefault();
    setSaving(true);
    try {
      const data = {
        title,
        description,
        category,
        points_value: parseInt(points) || 0,
        estimated_minutes: parseInt(minutes) || undefined,
        requires_approval: requiresApproval,
        requires_photo: requiresPhoto,
      };
      if (chore) {
        await api.chores.update(chore.id, data);
        onSave();
      } else {
        const created = await api.chores.create(data);
        onSave(andAssign ? created.id : undefined);
      }
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.formHeader}>
        <h3>{chore ? 'Edit Chore' : 'New Chore'}</h3>
        <button type="button" className={styles.iconBtn} onClick={onCancel}><X size={18} /></button>
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Title</label>
          <input className={styles.input} value={title} onChange={e => setTitle(e.target.value)} required placeholder="e.g. Feed the Cats" />
        </div>

        <div className={styles.formGroup}>
          <label className={styles.label}>Description</label>
          <input className={styles.input} value={description} onChange={e => setDescription(e.target.value)} placeholder="What needs to be done?" />
        </div>

        <div className={styles.formRow}>
          <div className={styles.formGroup}>
            <label className={styles.label} title="Required: must be done to unlock core points. Core: daily tasks. Bonus: optional extras.">Category</label>
            <select className={styles.input} value={category} onChange={e => setCategory(e.target.value)}>
              <option value="required">Required</option>
              <option value="core">Core</option>
              <option value="bonus">Bonus</option>
            </select>
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label}>Points</label>
            <input className={styles.input} type="number" min="0" value={points} onChange={e => setPoints(e.target.value)} />
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label}>Minutes</label>
            <input className={styles.input} type="number" min="1" value={minutes} onChange={e => setMinutes(e.target.value)} />
          </div>
        </div>

        <div className={styles.formRow} style={{ gap: '2rem', padding: '0.5rem 0' }}>
          <label className={styles.checkboxLabel}>
            <input type="checkbox" checked={requiresApproval} onChange={e => setRequiresApproval(e.target.checked)} />
            <span>Requires Parent Approval</span>
          </label>
          <label className={styles.checkboxLabel}>
            <input type="checkbox" checked={requiresPhoto} onChange={e => setRequiresPhoto(e.target.checked)} />
            <span>Requires Photo Proof</span>
          </label>
        </div>
      </div>

      <div className={styles.formActions}>
        <button type="button" className={styles.btnSecondary} onClick={onCancel}>Cancel</button>
        {chore ? (
          <button type="submit" className={styles.btnPrimary} disabled={saving || !title}>
            <Save size={16} /> Update
          </button>
        ) : (
          <>
            <button type="submit" className={styles.btnSecondary} disabled={saving || !title} style={{ fontWeight: 700 }}>
              <Save size={16} /> Create
            </button>
            <button type="button" className={styles.btnPrimary} disabled={saving || !title} onClick={(e) => handleSubmit(e, true)}>
              <CalendarPlus size={16} /> Create & Assign
            </button>
          </>
        )}
      </div>
    </form>
  );
};

// =================== SCHEDULE MANAGER ===================

const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6];
const WEEKDAYS = [1, 2, 3, 4, 5];
const WEEKENDS = [0, 6];

const ScheduleManager: React.FC<{
  choreId: number;
  users: User[];
  autoOpen?: boolean;
  onOpened?: () => void;
}> = ({ choreId, users, autoOpen, onOpened }) => {
  const [schedules, setSchedules] = useState<ChoreSchedule[]>([]);
  const [adding, setAdding] = useState(false);
  const [selectedUsers, setSelectedUsers] = useState<number[]>(users[0] ? [users[0].id] : []);
  const [schedType, setSchedType] = useState<'recurring' | 'oneoff' | 'interval'>('recurring');
  const [selectedDays, setSelectedDays] = useState<number[]>([]);
  const [specificDate, setSpecificDate] = useState('');
  const [availableAt, setAvailableAt] = useState('');
  const [dueBy, setDueBy] = useState('');
  const [expiryPenalty, setExpiryPenalty] = useState<'block' | 'no_points' | 'penalty'>('block');
  const [expiryPenaltyValue, setExpiryPenaltyValue] = useState('5');
  const [intervalDays, setIntervalDays] = useState('2');
  const [intervalStart, setIntervalStart] = useState(() => {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;
  });
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    const s = await api.chores.listSchedules(choreId);
    setSchedules(s);
  }, [choreId]);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    if (autoOpen && !adding) {
      setAdding(true);
      onOpened?.();
    }
  }, [autoOpen, adding, onOpened]);

  const toggleDay = (d: number) => {
    setSelectedDays(prev => prev.includes(d) ? prev.filter(x => x !== d) : [...prev, d]);
  };

  const toggleUser = (id: number) => {
    setSelectedUsers(prev => prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]);
  };

  const setDayPreset = (days: number[]) => {
    setSelectedDays(prev => {
      const same = prev.length === days.length && days.every(d => prev.includes(d));
      return same ? [] : days;
    });
  };

  const handleAdd = async () => {
    if (selectedUsers.length === 0) return;
    setSaving(true);

    try {
      for (const userId of selectedUsers) {
        const penaltyFields = dueBy ? {
          expiry_penalty: expiryPenalty,
          ...(expiryPenalty === 'penalty' ? { expiry_penalty_value: parseInt(expiryPenaltyValue) || 0 } : {}),
        } : {};

        if (schedType === 'recurring') {
          for (const day of selectedDays) {
            await api.chores.createSchedule(choreId, {
              assigned_to: userId,
              day_of_week: day,
              available_at: availableAt || undefined,
              due_by: dueBy || undefined,
              ...penaltyFields,
            });
          }
        } else if (schedType === 'interval') {
          const interval = parseInt(intervalDays);
          if (!interval || interval < 1 || !intervalStart) continue;
          await api.chores.createSchedule(choreId, {
            assigned_to: userId,
            recurrence_interval: interval,
            recurrence_start: intervalStart,
            available_at: availableAt || undefined,
            due_by: dueBy || undefined,
            ...penaltyFields,
          });
        } else {
          if (!specificDate) continue;
          await api.chores.createSchedule(choreId, {
            assigned_to: userId,
            specific_date: specificDate,
            available_at: availableAt || undefined,
            due_by: dueBy || undefined,
            ...penaltyFields,
          });
        }
      }

      setAdding(false);
      setSelectedDays([]);
      setSpecificDate('');
      setAvailableAt('');
      setDueBy('');
      setExpiryPenalty('block');
      setExpiryPenaltyValue('5');
      setSelectedUsers(users[0] ? [users[0].id] : []);
      load();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  const handleDelete = async (schedId: number) => {
    try {
      await api.chores.deleteSchedule(choreId, schedId);
    } catch (err) {
      console.error('Failed to delete schedule:', err);
    }
    load();
  };

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || `User ${id}`;

  const canAdd = selectedUsers.length > 0 && (
    schedType === 'recurring' ? selectedDays.length > 0 :
    schedType === 'oneoff' ? !!specificDate :
    parseInt(intervalDays) >= 1 && !!intervalStart
  );

  return (
    <div className={styles.scheduleSection}>
      <div className={styles.scheduleHeader}>
        <span className={styles.scheduleTitle}>Schedules</span>
        <button className={styles.addBtnSmall} onClick={() => setAdding(!adding)}>
          {adding ? <X size={14} /> : <Plus size={14} />}
        </button>
      </div>

      {adding && (
        <div className={styles.scheduleForm}>
          <div className={styles.formGroup}>
            <label className={styles.label}>Assign to</label>
            <div className={styles.userPicker}>
              {users.map(u => (
                <button
                  key={u.id}
                  type="button"
                  className={clsx(styles.userPickerBtn, selectedUsers.includes(u.id) && styles.userPickerBtnActive)}
                  onClick={() => toggleUser(u.id)}
                >
                  {u.name}
                </button>
              ))}
              {users.length > 1 && (
                <button
                  type="button"
                  className={clsx(styles.userPickerBtn, styles.userPickerBtnAll, selectedUsers.length === users.length && styles.userPickerBtnActive)}
                  onClick={() => setSelectedUsers(selectedUsers.length === users.length ? [] : users.map(u => u.id))}
                >
                  All
                </button>
              )}
            </div>
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label} title="How often this chore repeats: weekly on specific days, every N days, or a single date.">Schedule type</label>
            <select className={styles.input} value={schedType} onChange={e => setSchedType(e.target.value as 'recurring' | 'oneoff' | 'interval')}>
              <option value="recurring">Weekly (pick days)</option>
              <option value="interval">Every N days</option>
              <option value="oneoff">One-off (specific date)</option>
            </select>
          </div>

          {schedType === 'recurring' ? (
            <div className={styles.formGroup}>
              <div className={styles.dayPicker}>
                {DAY_NAMES.map((name, i) => (
                  <button
                    key={i}
                    type="button"
                    className={clsx(styles.dayBtn, selectedDays.includes(i) && styles.dayBtnActive)}
                    onClick={() => toggleDay(i)}
                  >
                    {name}
                  </button>
                ))}
              </div>
              <div className={styles.dayPresets}>
                <button type="button" className={clsx(styles.presetBtn, selectedDays.length === 7 && styles.presetBtnActive)} onClick={() => setDayPreset(ALL_DAYS)}>Every day</button>
                <button type="button" className={clsx(styles.presetBtn, selectedDays.length === 5 && WEEKDAYS.every(d => selectedDays.includes(d)) && styles.presetBtnActive)} onClick={() => setDayPreset(WEEKDAYS)}>Weekdays</button>
                <button type="button" className={clsx(styles.presetBtn, selectedDays.length === 2 && WEEKENDS.every(d => selectedDays.includes(d)) && styles.presetBtnActive)} onClick={() => setDayPreset(WEEKENDS)}>Weekends</button>
              </div>
            </div>
          ) : schedType === 'interval' ? (
            <div className={styles.formRow}>
              <div className={styles.formGroup}>
                <label className={styles.label}>Every</label>
                <div className={styles.intervalInput}>
                  <input className={styles.input} type="number" min="1" max="365" value={intervalDays} onChange={e => setIntervalDays(e.target.value)} />
                  <span className={styles.intervalSuffix}>days</span>
                </div>
              </div>
              <div className={styles.formGroup}>
                <label className={styles.label}>Starting from</label>
                <input className={styles.input} type="date" value={intervalStart} onChange={e => setIntervalStart(e.target.value)} />
              </div>
            </div>
          ) : (
            <div className={styles.formGroup}>
              <label className={styles.label}>Date</label>
              <input className={styles.input} type="date" value={specificDate} onChange={e => setSpecificDate(e.target.value)} />
            </div>
          )}

          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label} title="Chore won't appear until this time. Leave blank for no time lock.">Available at (time lock)</label>
              <input className={styles.input} type="time" value={availableAt} onChange={e => setAvailableAt(e.target.value)} />
              <span className={styles.helpText}>Chore hidden until this time</span>
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label} title="Chore expires after this time. Leave blank for no deadline.">Due by (deadline)</label>
              <input className={styles.input} type="time" value={dueBy} onChange={e => setDueBy(e.target.value)} />
              <span className={styles.helpText}>Expires after this time</span>
            </div>
          </div>

          {dueBy && (
            <div className={styles.formGroup}>
              <label className={styles.label} title="What happens when a child tries to complete this chore after the deadline.">If completed late</label>
              <div className={styles.formRow}>
                <select className={styles.input} value={expiryPenalty} onChange={e => setExpiryPenalty(e.target.value as 'block' | 'no_points' | 'penalty')}>
                  <option value="block">Cannot complete (breaks streak)</option>
                  <option value="no_points">Can complete, but 0 points</option>
                  <option value="penalty">Can complete, but deduct points</option>
                </select>
                {expiryPenalty === 'penalty' && (
                  <input className={styles.input} type="number" min="1" placeholder="Points to deduct" value={expiryPenaltyValue} onChange={e => setExpiryPenaltyValue(e.target.value)} style={{ maxWidth: '140px' }} />
                )}
              </div>
            </div>
          )}

          <button className={styles.btnPrimary} onClick={handleAdd} disabled={!canAdd || saving}>
            <Save size={14} /> Assign{selectedUsers.length > 1 ? ` to ${selectedUsers.length} kids` : ''}
          </button>
        </div>
      )}

      <div className={styles.scheduleList}>
        {schedules.length === 0 && <p className={styles.emptyText}>No schedules yet</p>}
        {(() => {
          type Group = { key: string; userName: string; userId: number; availableAt?: string; dueBy?: string; expiryPenalty?: string; expiryPenaltyValue?: number; scheduleIds: number[]; } & (
            | { type: 'recurring'; days: number[] }
            | { type: 'interval'; interval: number; start: string }
            | { type: 'oneoff'; date: string }
          );
          const groups: Group[] = [];
          for (const s of schedules) {
            const time = s.available_at || '';
            if (s.recurrence_interval) {
              groups.push({ key: `${s.id}`, userName: getUserName(s.assigned_to), userId: s.assigned_to, availableAt: s.available_at ?? undefined, dueBy: s.due_by ?? undefined, expiryPenalty: s.expiry_penalty, expiryPenaltyValue: s.expiry_penalty_value, scheduleIds: [s.id], type: 'interval', interval: s.recurrence_interval, start: s.recurrence_start || '' });
            } else if (s.day_of_week != null) {
              const gKey = `${s.assigned_to}-weekly-${time}`;
              const existing = groups.find(g => g.key === gKey && g.type === 'recurring');
              if (existing && existing.type === 'recurring') {
                existing.days.push(s.day_of_week);
                existing.scheduleIds.push(s.id);
              } else {
                groups.push({ key: gKey, userName: getUserName(s.assigned_to), userId: s.assigned_to, availableAt: s.available_at ?? undefined, dueBy: s.due_by ?? undefined, expiryPenalty: s.expiry_penalty, expiryPenaltyValue: s.expiry_penalty_value, scheduleIds: [s.id], type: 'recurring', days: [s.day_of_week] });
              }
            } else if (s.specific_date) {
              groups.push({ key: `${s.id}`, userName: getUserName(s.assigned_to), userId: s.assigned_to, availableAt: s.available_at ?? undefined, dueBy: s.due_by ?? undefined, expiryPenalty: s.expiry_penalty, expiryPenaltyValue: s.expiry_penalty_value, scheduleIds: [s.id], type: 'oneoff', date: s.specific_date });
            }
          }
          const handleDeleteGroup = async (ids: number[]) => {
            try {
              await Promise.all(ids.map(id => api.chores.deleteSchedule(choreId, id)));
            } catch (err) {
              console.error('Failed to delete some schedules:', err);
            }
            load();
          };
          const formatDays = (days: number[]) => {
            const sorted = [...days].sort((a, b) => a - b);
            if (sorted.length === 7) return 'Every day';
            if (sorted.length === 5 && [1,2,3,4,5].every(d => sorted.includes(d))) return 'Weekdays';
            if (sorted.length === 2 && sorted[0] === 0 && sorted[1] === 6) return 'Weekends';
            return sorted.map(d => DAY_NAMES[d]).join(' ');
          };
          return groups.map(g => (
            <div key={g.key} className={styles.scheduleItem}>
              <span className={styles.scheduleUser}>{g.userName}</span>
              <span className={styles.scheduleDays}>
                {g.type === 'recurring' ? formatDays(g.days)
                  : g.type === 'interval' ? `Every ${g.interval}d from ${g.start}`
                  : g.date}
              </span>
              {g.availableAt && <span className={styles.scheduleTime}>from {g.availableAt}</span>}
              {g.dueBy && <span className={styles.scheduleTime} style={{ color: 'var(--status-required)' }}>due {g.dueBy}</span>}
              {g.dueBy && g.expiryPenalty && g.expiryPenalty !== 'block' && (
                <span className={styles.scheduleTime} style={{ color: 'var(--text-muted)', fontSize: '0.7rem' }}>
                  {g.expiryPenalty === 'no_points' ? '(0 pts if late)' : `(-${g.expiryPenaltyValue} pts if late)`}
                </span>
              )}
              <button className={clsx(styles.iconBtn, styles.iconBtnDanger, styles.iconBtnSm)} onClick={() => handleDeleteGroup(g.scheduleIds)}>
                <Trash2 size={14} />
              </button>
            </div>
          ));
        })()}
      </div>
    </div>
  );
};

// =================== REWARDS TAB ===================

const RewardsTab: React.FC = () => {
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
                <button className={styles.iconBtn} onClick={() => { setEditingReward(r); setShowForm(true); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} onClick={() => handleDeleteReward(r.id)}>
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
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} onClick={() => handleDeleteStreakReward(sr.id)}>
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

// =================== POINTS TAB ===================

const PointsTab: React.FC = () => {
  const [balances, setBalances] = useState<(PointBalance & { name: string })[]>([]);
  const [users, setUsers] = useState<User[]>([]);
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
      <h2 className={styles.sectionTitle}>Point Balances</h2>

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
              {adjustUser === b.user_id ? 'Cancel' : 'Adjust'}
            </button>

            {adjustUser === b.user_id && (
              <div className={styles.adjustForm}>
                <div className={styles.formRow}>
                  <div className={styles.formGroup}>
                    <label className={styles.label}>Amount (+/-)</label>
                    <input
                      className={styles.input}
                      type="number"
                      value={adjustAmount}
                      onChange={e => setAdjustAmount(e.target.value)}
                      placeholder="+10 or -5"
                    />
                  </div>
                  <div className={styles.formGroup} style={{ flex: 2 }}>
                    <label className={styles.label}>Reason</label>
                    <input
                      className={styles.input}
                      value={adjustNote}
                      onChange={e => setAdjustNote(e.target.value)}
                      placeholder="Why?"
                    />
                  </div>
                </div>
                <button className={styles.btnPrimary} onClick={handleAdjust} disabled={saving || !adjustAmount}>
                  <Save size={14} /> Apply
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

// =================== ACTIVITY TAB ===================

const ActivityTab: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [transactions, setTransactions] = useState<PointTransaction[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const usrs = await api.users.list();
      const children = usrs.filter((u: User) => u.role === 'child');
      setUsers(children);

      // Fetch transactions for all children
      const allTxns = await Promise.all(
        children.map(async (u: User) => {
          const data = await api.points.getForUser(u.id);
          return data.transactions.map(t => ({ ...t, user_id: u.id }));
        })
      );
      // Flatten and sort by date descending
      const flat = allTxns.flat().sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
      setTransactions(flat);
    } catch (err) {
      console.error(err);
    }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || `User ${id}`;

  const formatTime = (dateStr: string) => {
    const d = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    const diffHr = Math.floor(diffMs / 3600000);

    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    if (diffHr < 24) return `${diffHr}h ago`;

    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) + ' ' +
      d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  };

  const getReasonLabel = (reason: string) => {
    switch (reason) {
      case 'chore_complete': return 'Chore completed';
      case 'chore_uncomplete': return 'Chore undone';
      case 'reward_redeem': return 'Reward redeemed';
      case 'streak_bonus': return 'Streak bonus';
      case 'admin_adjust': return 'Admin adjustment';
      case 'expiry_penalty': return 'Late penalty';
      case 'points_decay': return 'Points decay';
      default: return reason;
    }
  };

  const getReasonIcon = (reason: string) => {
    switch (reason) {
      case 'chore_complete': return <Star size={14} style={{ color: '#22c55e' }} />;
      case 'chore_uncomplete': return <Undo2 size={14} style={{ color: '#ef4444' }} />;
      case 'reward_redeem': return <Gift size={14} style={{ color: '#a78bfa' }} />;
      case 'streak_bonus': return <Flame size={14} style={{ color: '#f59e0b' }} />;
      case 'admin_adjust': return <Coins size={14} style={{ color: '#38bdf8' }} />;
      default: return <Activity size={14} />;
    }
  };

  const handleUndo = async (txn: PointTransaction) => {
    if (txn.reason === 'reward_redeem' && txn.reference_id) {
      await api.rewards.undoRedemption(txn.reference_id);
    } else {
      const note = `Undo: ${txn.note || getReasonLabel(txn.reason)}`;
      await api.points.adjust(txn.user_id, -txn.amount, note);
    }
    load();
  };

  if (loading) return <p className={styles.emptyText}>Loading...</p>;

  return (
    <div>
      <h2 className={styles.sectionTitle}>Activity Log</h2>
      <p className={styles.sectionSubtitle}>{transactions.length} events</p>

      <div className={styles.activityList}>
        {transactions.length === 0 && <p className={styles.emptyText}>No activity yet</p>}
        {transactions.map(txn => (
          <div key={`${txn.user_id}-${txn.id}`} className={styles.activityItem}>
            <div className={styles.activityIcon}>{getReasonIcon(txn.reason)}</div>
            <div className={styles.activityInfo}>
              <div className={styles.activityMain}>
                <span className={styles.activityUser}>{getUserName(txn.user_id)}</span>
                <span className={styles.activityReason}>{getReasonLabel(txn.reason)}</span>
              </div>
              {txn.note && <div className={styles.activityNote}>{txn.note}</div>}
              <div className={styles.activityTime}>{formatTime(txn.created_at)}</div>
            </div>
            <div className={clsx(styles.activityAmount, txn.amount > 0 ? styles.activityAmountPos : styles.activityAmountNeg)}>
              {txn.amount > 0 ? '+' : ''}{txn.amount}
            </div>
            <button
              className={clsx(styles.iconBtn, styles.iconBtnSm)}
              title="Undo this event"
              onClick={() => handleUndo(txn)}
            >
              <Undo2 size={14} />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
};

// =================== SETTINGS TAB ===================

const EXPORT_SECTIONS = [
  { id: 'users', label: 'Users' },
  { id: 'chores', label: 'Chores & Schedules' },
  { id: 'rewards', label: 'Rewards' },
  { id: 'streak_rewards', label: 'Streak Rewards' },
  { id: 'settings', label: 'Settings' },
];

const ExportConfigSection: React.FC = () => {
  const [selected, setSelected] = useState<Set<string>>(new Set(EXPORT_SECTIONS.map(s => s.id)));
  const [exporting, setExporting] = useState(false);

  const toggle = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleExport = async () => {
    if (selected.size === 0) return;
    setExporting(true);
    try {
      const blob = await api.admin.exportConfig(Array.from(selected));
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'config.yaml';
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      console.error('Export failed', e);
    }
    setExporting(false);
  };

  return (
    <div className={styles.form} style={{ marginTop: '1.5rem' }}>
      <div className={styles.formHeader}>
        <h3>Export Configuration</h3>
      </div>
      <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.75rem' }}>
        Download a <code>config.yaml</code> reflecting the current database state. Use this to bootstrap a fresh instance.
      </p>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '1rem' }}>
        {EXPORT_SECTIONS.map(s => (
          <label key={s.id} style={{ display: 'flex', alignItems: 'center', gap: '0.3rem', fontSize: '0.85rem', cursor: 'pointer' }}>
            <input type="checkbox" checked={selected.has(s.id)} onChange={() => toggle(s.id)} />
            {s.label}
          </label>
        ))}
      </div>
      <div className={styles.formActions}>
        <button className={styles.btnPrimary} onClick={handleExport} disabled={exporting || selected.size === 0}>
          <Save size={16} /> {exporting ? 'Exporting...' : 'Download config.yaml'}
        </button>
      </div>
    </div>
  );
};

const SettingsTab: React.FC = () => {
  const [currentPin, setCurrentPin] = useState('');
  const [newPin, setNewPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Discord state
  const [discordUrl, setDiscordUrl] = useState('');
  const [discordSaving, setDiscordSaving] = useState(false);
  const [discordMessage, setDiscordMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Webhooks state
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [showWebhookForm, setShowWebhookForm] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState('');
  const [webhookSecret, setWebhookSecret] = useState('');
  const [webhookSelectedEvents, setWebhookSelectedEvents] = useState<Set<string>>(new Set());
  const [expandedWebhook, setExpandedWebhook] = useState<number | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);

  const WEBHOOK_EVENTS = [
    { id: 'chore.completed', label: 'Completed', icon: '✅' },
    { id: 'chore.uncompleted', label: 'Uncompleted', icon: '↩️' },
    { id: 'chore.expired', label: 'Expired', icon: '⏰' },
    { id: 'chore.missed', label: 'Missed', icon: '❌' },
    { id: 'reward.redeemed', label: 'Redeemed', icon: '🎁' },
    { id: 'daily.complete', label: 'Daily Done', icon: '🌟' },
    { id: 'streak.milestone', label: 'Streak', icon: '🔥' },
    { id: 'points.decayed', label: 'Decay', icon: '📉' },
  ];

  const allEventsSelected = webhookSelectedEvents.size === 0 || webhookSelectedEvents.size === WEBHOOK_EVENTS.length;
  const toggleEvent = (id: string) => {
    setWebhookSelectedEvents(prev => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  };
  const eventsToString = () => allEventsSelected ? '*' : Array.from(webhookSelectedEvents).join(',');

  const loadWebhooks = useCallback(async () => {
    try {
      const wh = await api.webhooks.list();
      setWebhooks(wh);
    } catch (e) { console.error(e); }
  }, []);

  useEffect(() => { loadWebhooks(); }, [loadWebhooks]);

  // Load initial settings
  useEffect(() => {
    // We don't have a bulk settings API, so we fetch what we need
    // For now, let's just assume we can fetch specific settings if needed
    // or add a new endpoint. Since we're here, let's add a quick fetch for base_url.
    fetch('/api/admin/settings/base_url')
      .then(r => r.json())
      .then(data => setBaseUrl(data.value || ''))
      .catch(() => {});
    fetch('/api/admin/settings/discord_webhook_url')
      .then(r => r.json())
      .then(data => setDiscordUrl(data.value || ''))
      .catch(() => {});
  }, []);

  const handleSaveBaseUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await fetch('/api/admin/settings/base_url', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: baseUrl })
      });
      setMessage({ type: 'success', text: 'Base URL updated' });
    } catch {
      setMessage({ type: 'error', text: 'Failed to update Base URL' });
    }
    setSaving(false);
  };

  const handleSaveDiscordUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setDiscordSaving(true);
    setDiscordMessage(null);
    try {
      await fetch('/api/admin/settings/discord_webhook_url', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'X-User-ID': JSON.parse(localStorage.getItem('openchore_user') || '{}').id?.toString() || '' },
        body: JSON.stringify({ value: discordUrl })
      });
      setDiscordMessage({ type: 'success', text: discordUrl ? 'Discord webhook URL saved' : 'Discord notifications disabled' });
    } catch {
      setDiscordMessage({ type: 'error', text: 'Failed to save Discord webhook URL' });
    }
    setDiscordSaving(false);
  };

  const handleTestDiscord = async () => {
    if (!discordUrl) return;
    setDiscordSaving(true);
    setDiscordMessage(null);
    try {
      const resp = await fetch(discordUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          embeds: [{
            title: 'OpenChore Test',
            description: 'Discord notifications are working!',
            color: 0x22c55e,
            timestamp: new Date().toISOString(),
          }]
        })
      });
      if (resp.ok) {
        setDiscordMessage({ type: 'success', text: 'Test message sent to Discord!' });
      } else {
        setDiscordMessage({ type: 'error', text: `Discord returned status ${resp.status}` });
      }
    } catch {
      setDiscordMessage({ type: 'error', text: 'Failed to reach Discord webhook URL' });
    }
    setDiscordSaving(false);
  };

  const handleChangePin = async (e: React.FormEvent) => {
    e.preventDefault();
    setMessage(null);

    if (newPin.length < 4) {
      setMessage({ type: 'error', text: 'New PIN must be at least 4 digits' });
      return;
    }
    if (newPin !== confirmPin) {
      setMessage({ type: 'error', text: 'New PINs do not match' });
      return;
    }

    setSaving(true);
    try {
      await api.admin.updatePasscode(currentPin, newPin);
      setMessage({ type: 'success', text: 'PIN updated successfully' });
      setCurrentPin('');
      setNewPin('');
      setConfirmPin('');
    } catch {
      setMessage({ type: 'error', text: 'Failed — check your current PIN' });
    }
    setSaving(false);
  };

  const handleCreateWebhook = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!webhookUrl) return;
    try {
      await api.webhooks.create({ url: webhookUrl, secret: webhookSecret || undefined, events: eventsToString() });
      setWebhookUrl('');
      setWebhookSecret('');
      setWebhookSelectedEvents(new Set());
      setShowWebhookForm(false);
      loadWebhooks();
    } catch (e) { console.error(e); }
  };

  const handleDeleteWebhook = async (id: number) => {
    try {
      await api.webhooks.delete(id);
      loadWebhooks();
    } catch (e) { console.error(e); }
  };

  const handleToggleWebhook = async (wh: Webhook) => {
    try {
      await api.webhooks.update(wh.id, { active: !wh.active });
      loadWebhooks();
    } catch (e) { console.error(e); }
  };

  const handleExpandWebhook = async (id: number) => {
    if (expandedWebhook === id) {
      setExpandedWebhook(null);
      return;
    }
    setExpandedWebhook(id);
    try {
      const d = await api.webhooks.listDeliveries(id);
      setDeliveries(d);
    } catch (e) { console.error(e); }
  };

  return (
    <div>
      <h2 className={styles.sectionTitle}>Settings</h2>

      <form className={styles.form} onSubmit={handleSaveBaseUrl}>
        <div className={styles.formHeader}>
          <h3>System Base URL</h3>
        </div>
        <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.75rem' }}>
          The public URL of this server (e.g. <code>https://chores.example.com</code>). Used for QR codes and notifications.
        </p>
        <div className={styles.formGroup}>
          <input
            className={styles.input}
            value={baseUrl}
            onChange={e => setBaseUrl(e.target.value)}
            placeholder="https://your-domain.com"
          />
        </div>
        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={saving}>
            <Save size={16} /> Save Base URL
          </button>
        </div>
      </form>

      <form className={styles.form} onSubmit={handleSaveDiscordUrl}>
        <div className={styles.formHeader}>
          <h3>Discord Notifications</h3>
        </div>
        <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.75rem' }}>
          Get notified in Discord when chores are completed, approved, or rejected. Paste a Discord webhook URL below.
        </p>
        <div className={styles.formGroup}>
          <input
            className={styles.input}
            value={discordUrl}
            onChange={e => setDiscordUrl(e.target.value)}
            placeholder="https://discord.com/api/webhooks/..."
          />
        </div>
        {discordMessage && (
          <p style={{ fontSize: '0.85rem', fontWeight: 600, color: discordMessage.type === 'success' ? '#22c55e' : '#ef4444', marginTop: '0.25rem', marginBottom: '0.25rem' }}>
            {discordMessage.text}
          </p>
        )}
        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={discordSaving}>
            <Save size={16} /> Save
          </button>
          {discordUrl && (
            <button type="button" className={styles.btnSecondary} disabled={discordSaving} onClick={handleTestDiscord}>
              Send Test
            </button>
          )}
        </div>
      </form>

      <form className={styles.form} onSubmit={handleChangePin}>
        <div className={styles.formHeader}>
          <h3>Change Admin PIN</h3>
        </div>

        <div className={styles.formGrid}>
          <div className={styles.formGroup}>
            <label className={styles.label}>Current PIN</label>
            <input
              className={styles.input}
              type="password"
              inputMode="numeric"
              pattern="[0-9]*"
              value={currentPin}
              onChange={e => setCurrentPin(e.target.value.replace(/\D/g, ''))}
              placeholder="Enter current PIN"
              required
            />
          </div>
          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label}>New PIN</label>
              <input
                className={styles.input}
                type="password"
                inputMode="numeric"
                pattern="[0-9]*"
                value={newPin}
                onChange={e => setNewPin(e.target.value.replace(/\D/g, ''))}
                placeholder="4+ digits"
                required
              />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>Confirm New PIN</label>
              <input
                className={styles.input}
                type="password"
                inputMode="numeric"
                pattern="[0-9]*"
                value={confirmPin}
                onChange={e => setConfirmPin(e.target.value.replace(/\D/g, ''))}
                placeholder="Re-enter"
                required
              />
            </div>
          </div>
        </div>

        {message && (
          <p style={{ fontSize: '0.85rem', fontWeight: 600, color: message.type === 'success' ? '#22c55e' : '#ef4444', marginTop: '0.5rem' }}>
            {message.text}
          </p>
        )}

        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={saving || !currentPin || !newPin || !confirmPin}>
            <Save size={16} /> Update PIN
          </button>
        </div>
      </form>

      {/* Export Config Section */}
      <ExportConfigSection />

      {/* Webhooks Section */}
      <div className={styles.form} style={{ marginTop: '1.5rem' }}>
        <div className={styles.formHeader}>
          <h3>Webhooks</h3>
          <button className={styles.btnSmall} onClick={() => setShowWebhookForm(f => !f)}>
            <Plus size={14} /> Add
          </button>
        </div>
        <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.75rem' }}>
          Send events to external services (Home Assistant, Discord, etc.)
        </p>

        {showWebhookForm && (
          <form onSubmit={handleCreateWebhook} style={{ marginBottom: '1rem' }}>
            <div className={styles.formGrid}>
              <div className={styles.formGroup}>
                <label className={styles.label}>URL</label>
                <input className={styles.input} value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} placeholder="https://..." required />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.label}>Secret (optional HMAC signing key)</label>
                <input className={styles.input} value={webhookSecret} onChange={e => setWebhookSecret(e.target.value)} placeholder="Leave blank for unsigned" />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.label}>Events {allEventsSelected && <span style={{ fontWeight: 400, color: 'var(--text-secondary)' }}>(all)</span>}</label>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem', marginTop: '0.3rem' }}>
                  {WEBHOOK_EVENTS.map(ev => {
                    const selected = webhookSelectedEvents.has(ev.id) || allEventsSelected;
                    return (
                      <button
                        key={ev.id}
                        type="button"
                        onClick={() => toggleEvent(ev.id)}
                        style={{
                          display: 'flex', alignItems: 'center', gap: '0.3rem',
                          padding: '0.35rem 0.65rem', borderRadius: '100px',
                          fontSize: '0.75rem', fontWeight: 600,
                          border: `2px solid ${selected ? 'var(--accent-blue)' : 'rgba(255,255,255,0.08)'}`,
                          background: selected ? 'rgba(56,189,248,0.12)' : 'rgba(255,255,255,0.03)',
                          color: selected ? 'var(--accent-blue)' : 'var(--text-secondary)',
                          cursor: 'pointer', transition: 'all 0.15s ease',
                        }}
                      >
                        <span>{ev.icon}</span> {ev.label}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>
            <div className={styles.formActions}>
              <button type="submit" className={styles.btnPrimary}><Save size={14} /> Create</button>
              <button type="button" className={styles.btnSecondary} onClick={() => setShowWebhookForm(false)}>Cancel</button>
            </div>
          </form>
        )}

        {webhooks.length === 0 && !showWebhookForm && (
          <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', fontStyle: 'italic' }}>No webhooks configured</p>
        )}

        {webhooks.map(wh => (
          <div key={wh.id} className={styles.listItem} style={{ marginBottom: '0.5rem' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span style={{
                  width: 8, height: 8, borderRadius: '50%',
                  background: wh.active ? '#22c55e' : '#6b7280',
                  flexShrink: 0
                }} />
                <span style={{ fontSize: '0.85rem', fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {wh.url}
                </span>
              </div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.2rem', display: 'flex', gap: '0.3rem', flexWrap: 'wrap', alignItems: 'center' }}>
                {wh.events === '*' ? (
                  <span>All events</span>
                ) : (
                  wh.events.split(',').map(e => {
                    const ev = WEBHOOK_EVENTS.find(we => we.id === e.trim());
                    return <span key={e} style={{ background: 'rgba(255,255,255,0.06)', padding: '0.1rem 0.4rem', borderRadius: '100px' }}>{ev ? `${ev.icon} ${ev.label}` : e.trim()}</span>;
                  })
                )}
                {wh.secret && <span>• Signed</span>}
              </div>
            </div>
            <div style={{ display: 'flex', gap: '0.4rem', flexShrink: 0 }}>
              <button className={styles.btnSmall} onClick={() => handleExpandWebhook(wh.id)}>
                {expandedWebhook === wh.id ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
              </button>
              <button className={styles.btnSmall} onClick={() => handleToggleWebhook(wh)}>
                {wh.active ? 'Disable' : 'Enable'}
              </button>
              <button className={clsx(styles.btnSmall, styles.btnDanger)} onClick={() => handleDeleteWebhook(wh.id)}>
                <Trash2 size={14} />
              </button>
            </div>
            {expandedWebhook === wh.id && (
              <div style={{ width: '100%', marginTop: '0.5rem' }}>
                <h4 style={{ fontSize: '0.8rem', fontWeight: 700, marginBottom: '0.4rem', color: 'var(--text-secondary)' }}>Recent Deliveries</h4>
                {deliveries.length === 0 ? (
                  <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', fontStyle: 'italic' }}>No deliveries yet</p>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.3rem', maxHeight: '200px', overflowY: 'auto' }}>
                    {deliveries.map(d => (
                      <div key={d.id} style={{
                        fontSize: '0.75rem', padding: '0.4rem 0.5rem',
                        background: 'rgba(255,255,255,0.03)', borderRadius: '4px',
                        display: 'flex', gap: '0.5rem', alignItems: 'center'
                      }}>
                        <span style={{
                          width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
                          background: d.status_code && d.status_code >= 200 && d.status_code < 300 ? '#22c55e' : '#ef4444'
                        }} />
                        <span style={{ fontWeight: 600 }}>{d.event}</span>
                        <span style={{ color: 'var(--text-secondary)' }}>{d.status_code || 'err'}</span>
                        <span style={{ color: 'var(--text-secondary)', marginLeft: 'auto' }}>
                          {new Date(d.created_at).toLocaleString()}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}

      </div>
    </div>
  );
};

// =================== USERS TAB ===================

const UsersTab: React.FC = () => {
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
          <div key={u.id} className={styles.listItem}>
            <div className={styles.listItemMain}>
              <div className={styles.userAvatar}>
                {u.avatar_url ? <img src={u.avatar_url} alt={u.name} /> : <div className={styles.userAvatarPlaceholder} />}
              </div>
              <div className={styles.listItemInfo}>
                <h3 className={styles.listItemTitle}>{u.name}</h3>
                <div className={styles.listItemMeta}>
                  <span className={clsx(styles.badge, u.role === 'admin' ? styles.badge_admin : styles.badge_child)}>{u.role}</span>
                  {u.age && <span>Age {u.age}</span>}
                </div>
              </div>
              <div className={styles.listItemActions}>
                {u.role === 'child' && (
                  <button className={styles.iconBtn} onClick={() => setExpandedDecay(expandedDecay === u.id ? null : u.id)} title="Points decay settings">
                    <Clock size={16} />
                  </button>
                )}
                <button className={styles.iconBtn} onClick={() => { setEditingUser(u); setShowForm(true); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} onClick={() => handleDelete(u.id)}>
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
          <label className={styles.label} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
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

// =================== USER FORM ===================

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const data: Partial<User> = {
        name,
        role: role as 'admin' | 'child',
        age: age ? parseInt(age) : undefined,
        theme: userTheme,
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
        <div className={styles.formGroup}>
          <label className={styles.label}>Theme</label>
          <select className={styles.input} value={userTheme} onChange={e => setUserTheme(e.target.value as Theme)}>
            <option value="default">🌊 Classic</option>
            <option value="quest">⚔️ Quest</option>
            <option value="galaxy">🚀 Galaxy</option>
            <option value="forest">🌲 Forest</option>
          </select>
        </div>
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
