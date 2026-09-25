import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import { DAY_NAMES } from '../../types';
import type { User } from '../../types';
import { localDateStr, toggleInArray } from '../../utils';
import { useAIStatus } from '../../hooks/useAIStatus';
import { Icon, catFromCategory } from '../../design';
import { CategoryLabel, CategoryPicker, IconPicker, IconWell, PersonName, PersonToggle } from '../admin/pickers';
import styles from './CreateChoreWizard.module.css';

interface ChoreData {
  title: string;
  description: string;
  category: 'required' | 'core' | 'bonus';
  icon: string;
  points: number;
  missedPenalty: number;
  estimatedMinutes: number;
  requiresApproval: boolean;
  requiresPhoto: boolean;
  photoSource: 'child' | 'external' | 'both';
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

const DAY_LABELS = DAY_NAMES;
const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6];
const WEEKDAYS = [1, 2, 3, 4, 5];
const WEEKENDS = [0, 6];

const defaultChoreData: ChoreData = {
  title: '', description: '', category: 'core', icon: '', points: 5, missedPenalty: 0,
  estimatedMinutes: 5, requiresApproval: false, requiresPhoto: false, photoSource: 'child',
};

const defaultScheduleData: ScheduleData = {
  selectedUsers: [], scheduleType: 'weekly', selectedDays: [],
  interval: 2, intervalStart: localDateStr(new Date()),
  specificDate: localDateStr(new Date()),
  availableAt: '', dueBy: '', expiryPenalty: 'block', expiryPenaltyValue: 5,
};

const CreateChoreWizard: React.FC<Props> = ({ isOpen, onClose, onComplete, users }) => {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const [chore, setChore] = useState<ChoreData>({ ...defaultChoreData });
  const [schedule, setSchedule] = useState<ScheduleData>({ ...defaultScheduleData });
  const [skipSchedule, setSkipSchedule] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');

  // AI description drafting (only offered when the server has AI configured)
  const aiStatus = useAIStatus();
  const [generatingDesc, setGeneratingDesc] = useState(false);

  const handleGenerateDescription = async () => {
    if (!chore.title.trim()) return;
    setGeneratingDesc(true);
    try {
      const resp = await api.admin.generateDescription(chore.title.trim(), chore.category);
      setChore(c => ({ ...c, description: resp.description }));
    } catch {
      // silently fail — AI is optional
    } finally {
      setGeneratingDesc(false);
    }
  };

  const reset = () => {
    setStep(0);
    setChore({ ...defaultChoreData });
    setSchedule({ ...defaultScheduleData });
    setSkipSchedule(false);
    setCreating(false);
    setError('');
    setGeneratingDesc(false);
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
      selectedUsers: toggleInArray(s.selectedUsers, id),
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
      selectedDays: toggleInArray(s.selectedDays, d),
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
        missed_penalty_value: chore.missedPenalty || 0,
        estimated_minutes: chore.estimatedMinutes || undefined,
        requires_approval: chore.requiresApproval,
        requires_photo: chore.requiresPhoto,
        photo_source: chore.requiresPhoto ? chore.photoSource : 'child',
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
          setError(t('admin.createChore.error.partialSchedule', { details: errors.join('; ') }));
        }
      }

      onComplete(created.id);
      reset();
    } catch (e: unknown) {
      setError(e instanceof Error && e.message ? e.message : t('admin.createChore.error.createFailed'));
      setCreating(false);
    }
  };

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || t('admin.createChore.unknown');
  const cat = catFromCategory(chore.category);

  // --- STEP 1: Chore Details ---
  const renderStep0 = () => (
    <div className={styles.formGrid}>
      <label className={styles.formGroup}>
        <span className={styles.label}>{t('admin.createChore.field.titleLabel')}</span>
        <input className={styles.input} value={chore.title} onChange={e => setChore(c => ({ ...c, title: e.target.value }))} placeholder={t('admin.createChore.field.titlePlaceholder')} />
      </label>

      <IconPicker value={chore.icon} onChange={icon => setChore(c => ({ ...c, icon }))} cat={cat} label={t('admin.createChore.field.iconLabel')} />

      <div className={styles.formGroup}>
        <div className={styles.labelRow}>
          <label className={styles.label} htmlFor="wizard-description">{t('admin.createChore.field.descriptionLabel')}</label>
          {aiStatus.ai.configured && chore.title.trim() && (
            <button
              type="button"
              className={styles.aiBtn}
              onClick={handleGenerateDescription}
              disabled={generatingDesc}
              title={t('admin.createChore.ai.generateTitle')}
            >
              {generatingDesc ? <span className={styles.spinnerSmall} aria-hidden /> : <Icon name="spark" />}
              {generatingDesc ? t('admin.createChore.ai.generating') : t('admin.createChore.ai.aiLabel')}
            </button>
          )}
        </div>
        <input id="wizard-description" className={styles.input} value={chore.description} onChange={e => setChore(c => ({ ...c, description: e.target.value }))} placeholder={t('admin.createChore.field.descriptionPlaceholder')} />
      </div>

      <CategoryPicker value={chore.category} onChange={category => setChore(c => ({ ...c, category }))} label={t('admin.createChore.field.categoryLabel')} />

      <div className={styles.formRow}>
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.field.pointsLabel')}</span>
          <input className={styles.input} type="number" min={0} value={chore.points} onChange={e => setChore(c => ({ ...c, points: parseInt(e.target.value) || 0 }))} />
        </label>
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.field.penaltyLabel')}</span>
          <input className={styles.input} type="number" min={0} value={chore.missedPenalty} onChange={e => setChore(c => ({ ...c, missedPenalty: parseInt(e.target.value) || 0 }))} placeholder="0" />
        </label>
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.field.minutesLabel')}</span>
          <input className={styles.input} type="number" min={0} value={chore.estimatedMinutes} onChange={e => setChore(c => ({ ...c, estimatedMinutes: parseInt(e.target.value) || 0 }))} />
        </label>
      </div>

      <div>
        <label className={styles.checkRow}>
          <input type="checkbox" checked={chore.requiresApproval} onChange={e => setChore(c => ({ ...c, requiresApproval: e.target.checked }))} />
          <span className={styles.checkLabel}>{t('admin.createChore.field.requiresApproval')}</span>
        </label>
        <label className={styles.checkRow}>
          <input type="checkbox" checked={chore.requiresPhoto} onChange={e => setChore(c => ({ ...c, requiresPhoto: e.target.checked }))} />
          <span className={styles.checkLabel}>{t('admin.createChore.field.requiresPhoto')}</span>
        </label>
      </div>
      {chore.requiresPhoto && (
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.field.photoSourceLabel')}</span>
          <select className={styles.input} value={chore.photoSource} onChange={e => setChore(c => ({ ...c, photoSource: e.target.value as ChoreData['photoSource'] }))}>
            <option value="child">{t('admin.createChore.photoSource.child')}</option>
            <option value="external">{t('admin.createChore.photoSource.external')}</option>
            <option value="both">{t('admin.createChore.photoSource.both')}</option>
          </select>
        </label>
      )}
    </div>
  );

  const allSelected = users.length > 0 && schedule.selectedUsers.length === users.length;
  const presetOn = (days: number[]) => schedule.selectedDays.length === days.length && days.every(d => schedule.selectedDays.includes(d));

  // --- STEP 2: Schedule ---
  const renderStep1 = () => (
    <div className={styles.formGrid}>
      <p className={styles.helpText}>{t('admin.createChore.schedule.helpText')}</p>

      <div className={styles.formGroup}>
        <span className={styles.label}>{t('admin.createChore.schedule.assignTo')}</span>
        <div className={styles.chips}>
          <button type="button" className={styles.chip} aria-pressed={allSelected} onClick={toggleAllUsers}>{t('admin.createChore.schedule.allUsers')}</button>
          {users.map(u => (
            <PersonToggle key={u.id} user={u} pressed={schedule.selectedUsers.includes(u.id)} onClick={() => toggleUser(u.id)} />
          ))}
        </div>
      </div>

      <label className={styles.formGroup}>
        <span className={styles.label}>{t('admin.createChore.schedule.typeLabel')}</span>
        <select className={styles.input} value={schedule.scheduleType} onChange={e => setSchedule(s => ({ ...s, scheduleType: e.target.value as ScheduleData['scheduleType'] }))}>
          <option value="weekly">{t('admin.createChore.scheduleType.weekly')}</option>
          <option value="interval">{t('admin.createChore.scheduleType.interval')}</option>
          <option value="oneoff">{t('admin.createChore.scheduleType.oneoff')}</option>
        </select>
      </label>

      {schedule.scheduleType === 'weekly' && (
        <div className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.schedule.daysLabel')}</span>
          <div className={styles.dayPicker}>
            {DAY_LABELS.map((d, i) => (
              <button key={i} type="button" className={styles.dayBtn} aria-pressed={schedule.selectedDays.includes(i)} onClick={() => toggleDay(i)}>{d}</button>
            ))}
          </div>
          <div className={styles.chips}>
            <button type="button" className={styles.chip} aria-pressed={presetOn(ALL_DAYS)} onClick={() => setDayPreset(ALL_DAYS)}>{t('admin.createChore.preset.everyDay')}</button>
            <button type="button" className={styles.chip} aria-pressed={presetOn(WEEKDAYS)} onClick={() => setDayPreset(WEEKDAYS)}>{t('admin.createChore.preset.weekdays')}</button>
            <button type="button" className={styles.chip} aria-pressed={presetOn(WEEKENDS)} onClick={() => setDayPreset(WEEKENDS)}>{t('admin.createChore.preset.weekends')}</button>
          </div>
        </div>
      )}

      {schedule.scheduleType === 'interval' && (
        <div className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.schedule.repeatEvery')}</span>
          <div className={styles.intervalInput}>
            <input className={styles.input} type="number" min={1} value={schedule.interval} onChange={e => setSchedule(s => ({ ...s, interval: parseInt(e.target.value) || 1 }))} aria-label={t('admin.createChore.schedule.repeatEvery')} />
            <span className={styles.intervalSuffix}>{t('admin.createChore.schedule.intervalSuffix')}</span>
            <input className={styles.input} type="date" value={schedule.intervalStart} onChange={e => setSchedule(s => ({ ...s, intervalStart: e.target.value }))} aria-label={t('admin.createChore.schedule.startDate')} />
          </div>
        </div>
      )}

      {schedule.scheduleType === 'oneoff' && (
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.schedule.dateLabel')}</span>
          <input className={styles.input} type="date" value={schedule.specificDate} onChange={e => setSchedule(s => ({ ...s, specificDate: e.target.value }))} />
        </label>
      )}

      <div className={styles.formRow}>
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.schedule.availableAt')}</span>
          <input className={styles.input} type="time" value={schedule.availableAt} onChange={e => setSchedule(s => ({ ...s, availableAt: e.target.value }))} />
        </label>
        <label className={styles.formGroup}>
          <span className={styles.label}>{t('admin.createChore.schedule.dueBy')}</span>
          <input className={styles.input} type="time" value={schedule.dueBy} onChange={e => setSchedule(s => ({ ...s, dueBy: e.target.value }))} />
        </label>
      </div>

      {schedule.dueBy && (
        <div className={styles.formRow}>
          <label className={styles.formGroup}>
            <span className={styles.label}>{t('admin.createChore.schedule.ifMissed')}</span>
            <select className={styles.input} value={schedule.expiryPenalty} onChange={e => setSchedule(s => ({ ...s, expiryPenalty: e.target.value as ScheduleData['expiryPenalty'] }))}>
              <option value="block">{t('admin.createChore.expiryPenalty.block')}</option>
              <option value="no_points">{t('admin.createChore.expiryPenalty.noPoints')}</option>
              <option value="penalty">{t('admin.createChore.expiryPenalty.penalty')}</option>
            </select>
          </label>
          {schedule.expiryPenalty === 'penalty' && (
            <label className={styles.formGroup}>
              <span className={styles.label}>{t('admin.createChore.schedule.deductLabel')}</span>
              <input className={styles.input} type="number" min={0} value={schedule.expiryPenaltyValue} onChange={e => setSchedule(s => ({ ...s, expiryPenaltyValue: parseInt(e.target.value) || 0 }))} />
            </label>
          )}
        </div>
      )}
    </div>
  );

  // --- STEP 3: Review ---
  const renderStep2 = () => {
    let when = '';
    if (schedule.scheduleType === 'weekly') {
      when = schedule.selectedDays.slice().sort((a, b) => a - b).map(d => DAY_LABELS[d]).join(', ');
    } else if (schedule.scheduleType === 'interval') {
      when = t('admin.createChore.review.intervalWhen', { interval: schedule.interval, start: schedule.intervalStart });
    } else {
      when = schedule.specificDate;
    }

    return (
      <div className={styles.review}>
        {error && <p className={styles.error} role="alert">{error}</p>}

        <section className={styles.reviewSection}>
          <div className={styles.reviewHeader}>
            <h3 className={styles.reviewTitle}>{t('admin.createChore.review.choreDetails')}</h3>
            <button type="button" className={styles.editLink} onClick={() => setStep(0)}>{t('admin.createChore.review.edit')}</button>
          </div>
          <div className={styles.reviewChore}>
            <IconWell icon={chore.icon} cat={cat} />
            <div className={styles.reviewChoreText}>
              <span className={styles.reviewChoreTitle}>{chore.title}</span>
              {chore.description && <span className={styles.reviewChoreDesc}>{chore.description}</span>}
            </div>
          </div>
          <dl className={styles.reviewList}>
            <div className={styles.reviewRow}>
              <dt className={styles.reviewLabel}>{t('admin.createChore.review.categoryLabel')}</dt>
              <dd className={styles.reviewValue}><CategoryLabel category={chore.category} /></dd>
            </div>
            <div className={styles.reviewRow}>
              <dt className={styles.reviewLabel}>{t('admin.createChore.review.pointsLabel')}</dt>
              <dd className={styles.reviewValue}>{chore.points} {t('admin.createChore.review.pts')}{chore.missedPenalty > 0 && ` / −${chore.missedPenalty} ${t('admin.createChore.review.penalty')}`}</dd>
            </div>
            {chore.estimatedMinutes > 0 && (
              <div className={styles.reviewRow}>
                <dt className={styles.reviewLabel}>{t('admin.createChore.review.timeLabel')}</dt>
                <dd className={styles.reviewValue}>{chore.estimatedMinutes} {t('admin.createChore.review.min')}</dd>
              </div>
            )}
            {(chore.requiresApproval || chore.requiresPhoto) && (
              <div className={styles.reviewRow}>
                <dt className={styles.reviewLabel}>{t('admin.createChore.review.flagsLabel')}</dt>
                <dd className={styles.reviewValue}>
                  {[
                    chore.requiresApproval && t('admin.createChore.review.flagApproval'),
                    chore.requiresPhoto && t('admin.createChore.review.flagPhoto', { source: chore.photoSource === 'child' ? t('admin.createChore.review.photoSourceChild') : chore.photoSource === 'external' ? t('admin.createChore.review.photoSourceExternal') : t('admin.createChore.review.photoSourceBoth') }),
                  ].filter(Boolean).join(', ')}
                </dd>
              </div>
            )}
          </dl>
        </section>

        <section className={styles.reviewSection}>
          <div className={styles.reviewHeader}>
            <h3 className={styles.reviewTitle}>{t('admin.createChore.review.scheduleTitle')}</h3>
            <button type="button" className={styles.editLink} onClick={() => { setSkipSchedule(false); setStep(1); }}>{t('admin.createChore.review.edit')}</button>
          </div>
          {!skipSchedule ? (
            <dl className={styles.reviewList}>
              <div className={styles.reviewRow}>
                <dt className={styles.reviewLabel}>{t('admin.createChore.review.assigned')}</dt>
                <dd className={styles.reviewValue}>
                  <span className={styles.reviewPeople}>
                    {schedule.selectedUsers.map(id => (
                      <PersonName key={id} user={users.find(u => u.id === id)} name={getUserName(id)} />
                    ))}
                  </span>
                </dd>
              </div>
              <div className={styles.reviewRow}>
                <dt className={styles.reviewLabel}>{t('admin.createChore.review.when')}</dt>
                <dd className={styles.reviewValue}>{when}</dd>
              </div>
              {schedule.availableAt && (
                <div className={styles.reviewRow}>
                  <dt className={styles.reviewLabel}>{t('admin.createChore.review.available')}</dt>
                  <dd className={styles.reviewValue}>{schedule.availableAt}</dd>
                </div>
              )}
              {schedule.dueBy && (
                <div className={styles.reviewRow}>
                  <dt className={styles.reviewLabel}>{t('admin.createChore.review.dueBy')}</dt>
                  <dd className={styles.reviewValue}>{schedule.dueBy}</dd>
                </div>
              )}
            </dl>
          ) : (
            <p className={styles.noSchedule}>{t('admin.createChore.review.noSchedule')}</p>
          )}
        </section>
      </div>
    );
  };

  const stepTitles = [
    t('admin.createChore.step.details'),
    t('admin.createChore.step.schedule'),
    t('admin.createChore.step.review'),
  ];

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={t('admin.createChore.modalTitle')} maxWidth="600px">
      <ol className={styles.stepper}>
        {stepTitles.map((label, i) => (
          <li key={i} className={clsx(styles.step, i === step && styles.stepActive, i < step && styles.stepComplete)} aria-current={i === step ? 'step' : undefined}>
            <span className={styles.stepDot} aria-hidden>
              {i < step ? <Icon name="check" /> : i + 1}
            </span>
            <span className={styles.stepLabel}>{label}</span>
          </li>
        ))}
      </ol>

      {step === 0 && renderStep0()}
      {step === 1 && renderStep1()}
      {step === 2 && renderStep2()}

      <div className={styles.nav}>
        <div className={styles.navLeft}>
          {step > 0 && (
            <button type="button" className={styles.btnQuiet} onClick={() => setStep(step - 1)}>
              <Icon name="back" /> {t('admin.createChore.nav.back')}
            </button>
          )}
        </div>
        <div className={styles.navRight}>
          {step === 1 && (
            <button type="button" className={styles.btnQuiet} onClick={() => { setSkipSchedule(true); setStep(2); }}>
              {t('admin.createChore.nav.skip')}
            </button>
          )}
          {step < 2 && (
            <button type="button" className={styles.btnPrimary} disabled={step === 0 ? !canNext0 : !canNext1} onClick={() => setStep(step + 1)}>
              {t('admin.createChore.nav.next')} <Icon name="chev" />
            </button>
          )}
          {step === 2 && (
            <button type="button" className={styles.btnPrimary} onClick={handleCreate} disabled={creating}>
              {creating ? <><span className={styles.spinner} aria-hidden /> {t('admin.createChore.nav.creating')}</> : t('admin.createChore.nav.createChore')}
            </button>
          )}
        </div>
      </div>
    </Modal>
  );
};

export default CreateChoreWizard;
