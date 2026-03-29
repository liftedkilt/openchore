import React, { useState } from 'react';
import { Check, ChevronRight, ChevronLeft } from 'lucide-react';
import clsx from 'clsx';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import type { User } from '../../types';
import styles from './CreateChoreWizard.module.css';

interface ChoreData {
  title: string;
  description: string;
  category: 'required' | 'core' | 'bonus';
  icon: string;
  points: number;
  estimatedMinutes: number;
  requiresApproval: boolean;
  requiresPhoto: boolean;
}

interface ScheduleData {
  selectedUsers: number[];
  scheduleType: 'weekly' | 'interval' | 'oneoff';
  selectedDays: number[];
  interval: number;
  intervalStart: string;
  specificDate: string;
  availableAt: string;
  dueBy: string;
  expiryPenalty: 'block' | 'no_points' | 'penalty';
  expiryPenaltyValue: number;
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onComplete: (choreId: number) => void;
  users: User[];
}

const DAY_LABELS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

const defaultChoreData: ChoreData = {
  title: '', description: '', category: 'core', icon: '', points: 5,
  estimatedMinutes: 5, requiresApproval: false, requiresPhoto: false,
};

const defaultScheduleData: ScheduleData = {
  selectedUsers: [], scheduleType: 'weekly', selectedDays: [],
  interval: 2, intervalStart: new Date().toISOString().slice(0, 10),
  specificDate: new Date().toISOString().slice(0, 10),
  availableAt: '', dueBy: '', expiryPenalty: 'block', expiryPenaltyValue: 5,
};

const CreateChoreWizard: React.FC<Props> = ({ isOpen, onClose, onComplete, users }) => {
  const [step, setStep] = useState(0);
  const [chore, setChore] = useState<ChoreData>({ ...defaultChoreData });
  const [schedule, setSchedule] = useState<ScheduleData>({ ...defaultScheduleData });
  const [skipSchedule, setSkipSchedule] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');

  const reset = () => {
    setStep(0);
    setChore({ ...defaultChoreData });
    setSchedule({ ...defaultScheduleData });
    setSkipSchedule(false);
    setCreating(false);
    setError('');
  };

  const handleClose = () => { reset(); onClose(); };

  const canNext0 = chore.title.trim().length > 0;
  const canNext1 = skipSchedule || (schedule.selectedUsers.length > 0 && (
    (schedule.scheduleType === 'weekly' && schedule.selectedDays.length > 0) ||
    (schedule.scheduleType === 'interval' && schedule.interval > 0) ||
    (schedule.scheduleType === 'oneoff' && schedule.specificDate)
  ));

  const toggleUser = (id: number) => {
    setSchedule(s => ({
      ...s,
      selectedUsers: s.selectedUsers.includes(id)
        ? s.selectedUsers.filter(u => u !== id)
        : [...s.selectedUsers, id],
    }));
  };

  const toggleAllUsers = () => {
    const allIds = users.map(u => u.id);
    setSchedule(s => ({
      ...s,
      selectedUsers: s.selectedUsers.length === allIds.length ? [] : allIds,
    }));
  };

  const toggleDay = (d: number) => {
    setSchedule(s => ({
      ...s,
      selectedDays: s.selectedDays.includes(d)
        ? s.selectedDays.filter(x => x !== d)
        : [...s.selectedDays, d],
    }));
  };

  const setDayPreset = (days: number[]) => {
    setSchedule(s => ({ ...s, selectedDays: days }));
  };

  const handleCreate = async () => {
    setCreating(true);
    setError('');
    try {
      const created = await api.chores.create({
        title: chore.title.trim(),
        description: chore.description.trim(),
        category: chore.category,
        icon: chore.icon,
        points_value: chore.points,
        estimated_minutes: chore.estimatedMinutes || undefined,
        requires_approval: chore.requiresApproval,
        requires_photo: chore.requiresPhoto,
      });

      if (!skipSchedule && schedule.selectedUsers.length > 0) {
        const penaltyFields = schedule.dueBy
          ? { expiry_penalty: schedule.expiryPenalty, expiry_penalty_value: schedule.expiryPenalty === 'penalty' ? schedule.expiryPenaltyValue : 0 }
          : {};
        const common = {
          assignment_type: 'individual' as const,
          available_at: schedule.availableAt || undefined,
          due_by: schedule.dueBy || undefined,
          points_multiplier: 1,
          ...penaltyFields,
        };

        const promises: Promise<{ userId: number }>[] = [];
        for (const userId of schedule.selectedUsers) {
          if (schedule.scheduleType === 'weekly') {
            for (const day of schedule.selectedDays) {
              promises.push(
                api.chores.createSchedule(created.id, { assigned_to: userId, day_of_week: day, ...common }).then(() => ({ userId }))
              );
            }
          } else if (schedule.scheduleType === 'interval') {
            promises.push(
              api.chores.createSchedule(created.id, { assigned_to: userId, recurrence_interval: schedule.interval, recurrence_start: schedule.intervalStart, ...common }).then(() => ({ userId }))
            );
          } else {
            promises.push(
              api.chores.createSchedule(created.id, { assigned_to: userId, specific_date: schedule.specificDate, ...common }).then(() => ({ userId }))
            );
          }
        }

        const results = await Promise.allSettled(promises);
        const errors = results
          .filter((r): r is PromiseRejectedResult => r.status === 'rejected')
          .map(r => r.reason?.message || 'Unknown error');
        if (errors.length > 0) {
          setError(`Chore created, but some schedules failed: ${errors.join('; ')}`);
        }
      }

      onComplete(created.id);
      reset();
    } catch (e: any) {
      setError(e.message || 'Failed to create chore');
      setCreating(false);
    }
  };

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || 'Unknown';

  // --- STEP 1: Chore Details ---
  const renderStep0 = () => (
    <div className={styles.formGrid}>
      <div className={styles.formRow}>
        <div className={styles.formGroup} style={{ flex: 3 }}>
          <label className={styles.label}>Title *</label>
          <input className={styles.input} value={chore.title} onChange={e => setChore(c => ({ ...c, title: e.target.value }))} placeholder="e.g. Empty the dishwasher" />
        </div>
        <div className={styles.formGroup} style={{ flex: 0, minWidth: '70px' }}>
          <label className={styles.label}>Icon</label>
          <input className={styles.input} value={chore.icon} onChange={e => setChore(c => ({ ...c, icon: e.target.value }))} placeholder="🧹" style={{ textAlign: 'center' }} />
        </div>
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>Description</label>
        <input className={styles.input} value={chore.description} onChange={e => setChore(c => ({ ...c, description: e.target.value }))} placeholder="Optional details..." />
      </div>

      <div className={styles.formRow}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Category</label>
          <select className={styles.input} value={chore.category} onChange={e => setChore(c => ({ ...c, category: e.target.value as ChoreData['category'] }))}>
            <option value="required">Required</option>
            <option value="core">Core</option>
            <option value="bonus">Bonus</option>
          </select>
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Points</label>
          <input className={styles.input} type="number" min={0} value={chore.points} onChange={e => setChore(c => ({ ...c, points: parseInt(e.target.value) || 0 }))} />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Minutes</label>
          <input className={styles.input} type="number" min={0} value={chore.estimatedMinutes} onChange={e => setChore(c => ({ ...c, estimatedMinutes: parseInt(e.target.value) || 0 }))} />
        </div>
      </div>

      <div className={styles.checkRow}>
        <input type="checkbox" checked={chore.requiresApproval} onChange={e => setChore(c => ({ ...c, requiresApproval: e.target.checked }))} />
        <span className={styles.checkLabel}>Requires parent approval</span>
      </div>
      <div className={styles.checkRow}>
        <input type="checkbox" checked={chore.requiresPhoto} onChange={e => setChore(c => ({ ...c, requiresPhoto: e.target.checked }))} />
        <span className={styles.checkLabel}>Requires photo proof</span>
      </div>
    </div>
  );

  // --- STEP 2: Schedule ---
  const renderStep1 = () => (
    <div className={styles.formGrid}>
      <p className={styles.helpText}>Add a schedule so this chore appears on kids' dashboards. You can skip this and add schedules later.</p>

      <div className={styles.formGroup}>
        <label className={styles.label}>Assign to</label>
        <div className={styles.userPicker}>
          <button type="button" className={clsx(styles.userPickerBtn, schedule.selectedUsers.length === users.length && styles.userPickerBtnActive)} onClick={toggleAllUsers}>All</button>
          {users.map(u => (
            <button key={u.id} type="button" className={clsx(styles.userPickerBtn, schedule.selectedUsers.includes(u.id) && styles.userPickerBtnActive)} onClick={() => toggleUser(u.id)}>
              {u.name}
            </button>
          ))}
        </div>
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>Schedule type</label>
        <select className={styles.input} value={schedule.scheduleType} onChange={e => setSchedule(s => ({ ...s, scheduleType: e.target.value as ScheduleData['scheduleType'] }))}>
          <option value="weekly">Weekly (pick days)</option>
          <option value="interval">Every N days</option>
          <option value="oneoff">One-off date</option>
        </select>
      </div>

      {schedule.scheduleType === 'weekly' && (
        <>
          <div className={styles.formGroup}>
            <label className={styles.label}>Days</label>
            <div className={styles.dayPicker}>
              {DAY_LABELS.map((d, i) => (
                <button key={i} type="button" className={clsx(styles.dayBtn, schedule.selectedDays.includes(i) && styles.dayBtnActive)} onClick={() => toggleDay(i)}>{d}</button>
              ))}
            </div>
            <div className={styles.presets}>
              <button type="button" className={styles.presetBtn} onClick={() => setDayPreset([0,1,2,3,4,5,6])}>Every day</button>
              <button type="button" className={styles.presetBtn} onClick={() => setDayPreset([1,2,3,4,5])}>Weekdays</button>
              <button type="button" className={styles.presetBtn} onClick={() => setDayPreset([0,6])}>Weekends</button>
            </div>
          </div>
        </>
      )}

      {schedule.scheduleType === 'interval' && (
        <div className={styles.formGroup}>
          <label className={styles.label}>Repeat every</label>
          <div className={styles.intervalInput}>
            <input className={styles.input} type="number" min={1} value={schedule.interval} onChange={e => setSchedule(s => ({ ...s, interval: parseInt(e.target.value) || 1 }))} />
            <span className={styles.intervalSuffix}>days, starting</span>
            <input className={styles.input} type="date" value={schedule.intervalStart} onChange={e => setSchedule(s => ({ ...s, intervalStart: e.target.value }))} style={{ width: 'auto' }} />
          </div>
        </div>
      )}

      {schedule.scheduleType === 'oneoff' && (
        <div className={styles.formGroup}>
          <label className={styles.label}>Date</label>
          <input className={styles.input} type="date" value={schedule.specificDate} onChange={e => setSchedule(s => ({ ...s, specificDate: e.target.value }))} />
        </div>
      )}

      <div className={styles.formRow}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Available at</label>
          <input className={styles.input} type="time" value={schedule.availableAt} onChange={e => setSchedule(s => ({ ...s, availableAt: e.target.value }))} />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Due by</label>
          <input className={styles.input} type="time" value={schedule.dueBy} onChange={e => setSchedule(s => ({ ...s, dueBy: e.target.value }))} />
        </div>
      </div>

      {schedule.dueBy && (
        <div className={styles.formRow}>
          <div className={styles.formGroup}>
            <label className={styles.label}>If missed</label>
            <select className={styles.input} value={schedule.expiryPenalty} onChange={e => setSchedule(s => ({ ...s, expiryPenalty: e.target.value as ScheduleData['expiryPenalty'] }))}>
              <option value="block">Block completion</option>
              <option value="no_points">0 points</option>
              <option value="penalty">Deduct points</option>
            </select>
          </div>
          {schedule.expiryPenalty === 'penalty' && (
            <div className={styles.formGroup}>
              <label className={styles.label}>Deduct</label>
              <input className={styles.input} type="number" min={0} value={schedule.expiryPenaltyValue} onChange={e => setSchedule(s => ({ ...s, expiryPenaltyValue: parseInt(e.target.value) || 0 }))} />
            </div>
          )}
        </div>
      )}
    </div>
  );

  // --- STEP 3: Review ---
  const renderStep2 = () => {
    const scheduleDesc = () => {
      if (skipSchedule) return null;
      const names = schedule.selectedUsers.map(getUserName).join(', ');
      let when = '';
      if (schedule.scheduleType === 'weekly') {
        when = schedule.selectedDays.map(d => DAY_LABELS[d]).join(', ');
      } else if (schedule.scheduleType === 'interval') {
        when = `Every ${schedule.interval} days from ${schedule.intervalStart}`;
      } else {
        when = schedule.specificDate;
      }
      return { names, when };
    };

    const sd = scheduleDesc();

    return (
      <div>
        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.reviewSection}>
          <div className={styles.reviewHeader}>
            <span className={styles.reviewTitle}>Chore Details</span>
            <button className={styles.editLink} onClick={() => setStep(0)}>Edit</button>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Title</span>
            <span className={styles.reviewValue}>{chore.icon} {chore.title}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Category</span>
            <span className={clsx(styles.badge, styles[`badge_${chore.category}`])}>{chore.category}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Points</span>
            <span className={styles.reviewValue}>{chore.points} pts</span>
          </div>
          {chore.estimatedMinutes > 0 && (
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>Time</span>
              <span className={styles.reviewValue}>{chore.estimatedMinutes} min</span>
            </div>
          )}
          {chore.description && (
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>Desc</span>
              <span className={styles.reviewValue}>{chore.description}</span>
            </div>
          )}
          {(chore.requiresApproval || chore.requiresPhoto) && (
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>Flags</span>
              <span className={styles.reviewValue}>
                {[chore.requiresApproval && 'Approval', chore.requiresPhoto && 'Photo'].filter(Boolean).join(', ')}
              </span>
            </div>
          )}
        </div>

        <div className={styles.reviewSection}>
          <div className={styles.reviewHeader}>
            <span className={styles.reviewTitle}>Schedule</span>
            <button className={styles.editLink} onClick={() => { setSkipSchedule(false); setStep(1); }}>Edit</button>
          </div>
          {sd ? (
            <>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Assigned</span>
                <span className={styles.reviewValue}>{sd.names}</span>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>When</span>
                <span className={styles.reviewValue}>{sd.when}</span>
              </div>
              {schedule.availableAt && (
                <div className={styles.reviewRow}>
                  <span className={styles.reviewLabel}>Available</span>
                  <span className={styles.reviewValue}>{schedule.availableAt}</span>
                </div>
              )}
              {schedule.dueBy && (
                <div className={styles.reviewRow}>
                  <span className={styles.reviewLabel}>Due by</span>
                  <span className={styles.reviewValue}>{schedule.dueBy}</span>
                </div>
              )}
            </>
          ) : (
            <p className={styles.noSchedule}>No schedule — you can add one later.</p>
          )}
        </div>
      </div>
    );
  };

  const stepTitles = ['Details', 'Schedule', 'Review'];

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="New Chore" maxWidth="560px">
      {/* Step indicator */}
      <div className={styles.stepper}>
        {stepTitles.map((label, i) => (
          <div key={i} className={clsx(styles.step, i === step && styles.stepActive, i < step && styles.stepComplete)}>
            <div className={styles.stepDot}>
              {i < step ? <Check size={14} /> : i + 1}
            </div>
            <span className={styles.stepLabel}>{label}</span>
          </div>
        ))}
      </div>

      {/* Step content */}
      {step === 0 && renderStep0()}
      {step === 1 && renderStep1()}
      {step === 2 && renderStep2()}

      {/* Navigation */}
      <div className={styles.nav}>
        <div className={styles.navLeft}>
          {step > 0 && (
            <button className={styles.btnSecondary} onClick={() => setStep(step - 1)}>
              <ChevronLeft size={16} /> Back
            </button>
          )}
        </div>
        <div className={styles.navRight}>
          {step === 1 && (
            <button className={styles.skipBtn} onClick={() => { setSkipSchedule(true); setStep(2); }}>
              Skip
            </button>
          )}
          {step < 2 && (
            <button className={styles.btnPrimary} disabled={step === 0 ? !canNext0 : !canNext1} onClick={() => setStep(step + 1)}>
              Next <ChevronRight size={16} />
            </button>
          )}
          {step === 2 && (
            <button className={styles.btnPrimary} onClick={handleCreate} disabled={creating}>
              {creating ? <><span className={styles.spinner} /> Creating...</> : 'Create Chore'}
            </button>
          )}
        </div>
      </div>
    </Modal>
  );
};

export default CreateChoreWizard;
