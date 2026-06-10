import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation();
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

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || t('admin.scheduleManager.userFallback', { id });

  const canAdd = selectedUsers.length > 0 && (
    schedType === 'recurring' ? selectedDays.length > 0 :
    schedType === 'oneoff' ? !!specificDate :
    parseInt(intervalDays) >= 1 && !!intervalStart
  );

  return (
    <div className={styles.scheduleSection}>
      <div className={styles.scheduleHeader}>
        <span className={styles.scheduleTitle}>{t('admin.scheduleManager.title')}</span>
        <button className={styles.addBtnSmall} onClick={() => setAdding(!adding)}>
          {adding ? <X size={14} /> : <Plus size={14} />}
        </button>
      </div>

      {adding && (
        <div className={styles.scheduleForm}>
          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.scheduleManager.labelAssignTo')}</label>
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
                  {t('admin.scheduleManager.btnAll')}
                </button>
              )}
            </div>
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label} title={t('admin.scheduleManager.scheduleTypeTitle')}>{t('admin.scheduleManager.labelScheduleType')}</label>
            <select className={styles.input} value={schedType} onChange={e => setSchedType(e.target.value as 'recurring' | 'oneoff' | 'interval')}>
              <option value="recurring">{t('admin.scheduleManager.optionRecurring')}</option>
              <option value="interval">{t('admin.scheduleManager.optionInterval')}</option>
              <option value="oneoff">{t('admin.scheduleManager.optionOneoff')}</option>
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
                <button type="button" className={clsx(styles.presetBtn, selectedDays.length === 7 && styles.presetBtnActive)} onClick={() => setDayPreset(ALL_DAYS)}>{t('admin.scheduleManager.presetEveryDay')}</button>
                <button type="button" className={clsx(styles.presetBtn, selectedDays.length === 5 && WEEKDAYS.every(d => selectedDays.includes(d)) && styles.presetBtnActive)} onClick={() => setDayPreset(WEEKDAYS)}>{t('admin.scheduleManager.presetWeekdays')}</button>
                <button type="button" className={clsx(styles.presetBtn, selectedDays.length === 2 && WEEKENDS.every(d => selectedDays.includes(d)) && styles.presetBtnActive)} onClick={() => setDayPreset(WEEKENDS)}>{t('admin.scheduleManager.presetWeekends')}</button>
              </div>
            </div>
          ) : schedType === 'interval' ? (
            <div className={styles.formRow}>
              <div className={styles.formGroup}>
                <label className={styles.label}>{t('admin.scheduleManager.labelEvery')}</label>
                <div className={styles.intervalInput}>
                  <input className={styles.input} type="number" min="1" max="365" value={intervalDays} onChange={e => setIntervalDays(e.target.value)} />
                  <span className={styles.intervalSuffix}>{t('admin.scheduleManager.suffixDays')}</span>
                </div>
              </div>
              <div className={styles.formGroup}>
                <label className={styles.label}>{t('admin.scheduleManager.labelStartingFrom')}</label>
                <input className={styles.input} type="date" value={intervalStart} onChange={e => setIntervalStart(e.target.value)} />
              </div>
            </div>
          ) : (
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.scheduleManager.labelDate')}</label>
              <input className={styles.input} type="date" value={specificDate} onChange={e => setSpecificDate(e.target.value)} />
            </div>
          )}

          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label} title={t('admin.scheduleManager.availableAtTitle')}>{t('admin.scheduleManager.labelAvailableAt')}</label>
              <input className={styles.input} type="time" value={availableAt} onChange={e => setAvailableAt(e.target.value)} />
              <span className={styles.helpText}>{t('admin.scheduleManager.helpAvailableAt')}</span>
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label} title={t('admin.scheduleManager.dueByTitle')}>{t('admin.scheduleManager.labelDueBy')}</label>
              <input className={styles.input} type="time" value={dueBy} onChange={e => setDueBy(e.target.value)} />
              <span className={styles.helpText}>{t('admin.scheduleManager.helpDueBy')}</span>
            </div>
          </div>

          {dueBy && (
            <div className={styles.formGroup}>
              <label className={styles.label} title={t('admin.scheduleManager.ifLateTitle')}>{t('admin.scheduleManager.labelIfLate')}</label>
              <div className={styles.formRow}>
                <select className={styles.input} value={expiryPenalty} onChange={e => setExpiryPenalty(e.target.value as 'block' | 'no_points' | 'penalty')}>
                  <option value="block">{t('admin.scheduleManager.optionBlock')}</option>
                  <option value="no_points">{t('admin.scheduleManager.optionNoPoints')}</option>
                  <option value="penalty">{t('admin.scheduleManager.optionPenalty')}</option>
                </select>
                {expiryPenalty === 'penalty' && (
                  <input className={styles.input} type="number" min="1" placeholder={t('admin.scheduleManager.placeholderPointsToDeduct')} value={expiryPenaltyValue} onChange={e => setExpiryPenaltyValue(e.target.value)} style={{ maxWidth: '140px' }} />
                )}
              </div>
            </div>
          )}

          <button className={styles.btnPrimary} onClick={handleAdd} disabled={!canAdd || saving}>
            <Save size={14} /> {selectedUsers.length > 1
              ? t('admin.scheduleManager.btnAssignMultiple', { count: selectedUsers.length })
              : t('admin.scheduleManager.btnAssign')}
          </button>
        </div>
      )}

      <div className={styles.scheduleList}>
        {schedules.length === 0 && <p className={styles.emptyText}>{t('admin.scheduleManager.emptySchedules')}</p>}
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
            if (sorted.length === 7) return t('admin.scheduleManager.presetEveryDay');
            if (sorted.length === 5 && [1,2,3,4,5].every(d => sorted.includes(d))) return t('admin.scheduleManager.presetWeekdays');
            if (sorted.length === 2 && sorted[0] === 0 && sorted[1] === 6) return t('admin.scheduleManager.presetWeekends');
            return sorted.map(d => DAY_NAMES[d]).join(' ');
          };
          return groups.map(g => (
            <div key={g.key} className={styles.scheduleItem}>
              <span className={styles.scheduleUser}>{g.userName}</span>
              <span className={styles.scheduleDays}>
                {g.type === 'recurring' ? formatDays(g.days)
                  : g.type === 'interval' ? t('admin.scheduleManager.intervalDisplay', { interval: g.interval, start: g.start })
                  : g.date}
              </span>
              {g.availableAt && <span className={styles.scheduleTime}>{t('admin.scheduleManager.displayFrom', { time: g.availableAt })}</span>}
              {g.dueBy && <span className={styles.scheduleTime} style={{ color: 'var(--status-required)' }}>{t('admin.scheduleManager.displayDue', { time: g.dueBy })}</span>}
              {g.dueBy && g.expiryPenalty && g.expiryPenalty !== 'block' && (
                <span className={styles.scheduleTime} style={{ color: 'var(--text-muted)', fontSize: '0.7rem' }}>
                  {g.expiryPenalty === 'no_points'
                    ? t('admin.scheduleManager.displayNoPointsIfLate')
                    : t('admin.scheduleManager.displayDeductIfLate', { points: g.expiryPenaltyValue })}
                </span>
              )}
              <button className={clsx(styles.iconBtn, styles.iconBtnDanger, styles.iconBtnSm)} aria-label={t('admin.scheduleManager.ariaDeleteSchedule')} onClick={() => handleDeleteGroup(g.scheduleIds)}>
                <Trash2 size={14} />
              </button>
            </div>
          ));
        })()}
      </div>
    </div>
  );
};
