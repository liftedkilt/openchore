import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { User, ChoreSchedule } from '../../types';
import { DAY_NAMES } from '../../types';
import { toggleInArray } from '../../utils';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, X, Save } from 'lucide-react';
import clsx from 'clsx';

const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6];
const WEEKDAYS = [1, 2, 3, 4, 5];
const WEEKENDS = [0, 6];

export const ScheduleManager: React.FC<{
  choreId: number;
  users: User[];
}> = ({ choreId, users }) => {
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
  const [intervalStart, setIntervalStart] = useState(() => new Date().toISOString().slice(0, 10));
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    const s = await api.chores.listSchedules(choreId);
    setSchedules(s);
  }, [choreId]);

  useEffect(() => { load(); }, [load]);


  const toggleDay = (d: number) => {
    setSelectedDays(prev => toggleInArray(prev, d));
  };

  const toggleUser = (id: number) => {
    setSelectedUsers(prev => toggleInArray(prev, id));
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
      const penaltyFields = dueBy ? {
        expiry_penalty: expiryPenalty,
        ...(expiryPenalty === 'penalty' ? { expiry_penalty_value: parseInt(expiryPenaltyValue) || 0 } : {}),
      } : {};
      const common = {
        available_at: availableAt || undefined,
        due_by: dueBy || undefined,
        ...penaltyFields,
      };

      const promises: Promise<unknown>[] = [];
      for (const userId of selectedUsers) {
        if (schedType === 'recurring') {
          for (const day of selectedDays) {
            promises.push(api.chores.createSchedule(choreId, { assigned_to: userId, day_of_week: day, ...common }));
          }
        } else if (schedType === 'interval') {
          const interval = parseInt(intervalDays);
          if (!interval || interval < 1 || !intervalStart) continue;
          promises.push(api.chores.createSchedule(choreId, { assigned_to: userId, recurrence_interval: interval, recurrence_start: intervalStart, ...common }));
        } else {
          if (!specificDate) continue;
          promises.push(api.chores.createSchedule(choreId, { assigned_to: userId, specific_date: specificDate, ...common }));
        }
      }
      const results = await Promise.allSettled(promises);
      const failures = results.filter(r => r.status === 'rejected');
      if (failures.length > 0) {
        console.error('Some schedules failed to create:', failures);
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
            const results = await Promise.allSettled(ids.map(id => api.chores.deleteSchedule(choreId, id)));
            const failures = results.filter(r => r.status === 'rejected');
            if (failures.length > 0) {
              console.error('Failed to delete some schedules:', failures);
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
              <button className={clsx(styles.iconBtn, styles.iconBtnDanger, styles.iconBtnSm)} aria-label="Delete schedule" onClick={() => handleDeleteGroup(g.scheduleIds)}>
                <Trash2 size={14} />
              </button>
            </div>
          ));
        })()}
      </div>
    </div>
  );
};
