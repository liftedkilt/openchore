import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Check, ChevronRight, ChevronLeft, Sparkles } from 'lucide-react';
import clsx from 'clsx';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import { DAY_NAMES } from '../../types';
import type { User } from '../../types';
import { localDateStr, toggleInArray } from '../../utils';
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

  // AI description generation
  const [generatingDesc, setGeneratingDesc] = useState(false);

  // AI point suggestion
  const [aiSuggestion, setAiSuggestion] = useState<{ points: number; estimated_minutes: number; reasoning: string } | null>(null);
  const [suggestingPoints, setSuggestingPoints] = useState(false);
  const suggestDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  const fetchPointSuggestion = useCallback(async (title: string, description: string, category: string) => {
    if (!title.trim()) return;
    setSuggestingPoints(true);
    try {
      const resp = await api.admin.suggestPoints(title.trim(), description.trim(), category);
      setAiSuggestion(resp);
    } catch {
      setAiSuggestion(null);
    } finally {
      setSuggestingPoints(false);
    }
  }, []);

  // Debounced auto-trigger for point suggestions when title+category are filled
  useEffect(() => {
    if (suggestDebounceRef.current) clearTimeout(suggestDebounceRef.current);
    if (!chore.title.trim() || !chore.category) {
      setAiSuggestion(null);
      return;
    }
    suggestDebounceRef.current = setTimeout(() => {
      fetchPointSuggestion(chore.title, chore.description, chore.category);
    }, 1000);
    return () => {
      if (suggestDebounceRef.current) clearTimeout(suggestDebounceRef.current);
    };
  }, [chore.title, chore.description, chore.category, fetchPointSuggestion]);

  const applyAiSuggestion = () => {
    if (!aiSuggestion) return;
    setChore(c => ({
      ...c,
      points: aiSuggestion.points,
      estimatedMinutes: aiSuggestion.estimated_minutes,
    }));
  };

  const reset = () => {
    setStep(0);
    setChore({ ...defaultChoreData });
    setSchedule({ ...defaultScheduleData });
    setSkipSchedule(false);
    setCreating(false);
    setError('');
    setAiSuggestion(null);
    setGeneratingDesc(false);
    setSuggestingPoints(false);
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
    } catch (e: any) {
      setError(e.message || t('admin.createChore.error.createFailed'));
      setCreating(false);
    }
  };

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || t('admin.createChore.unknown');

  // --- STEP 1: Chore Details ---
  const renderStep0 = () => (
    <div className={styles.formGrid}>
      <div className={styles.formRow}>
        <div className={styles.formGroup} style={{ flex: 3 }}>
          <label className={styles.label}>{t('admin.createChore.field.titleLabel')}</label>
          <input className={styles.input} value={chore.title} onChange={e => setChore(c => ({ ...c, title: e.target.value }))} placeholder={t('admin.createChore.field.titlePlaceholder')} />
        </div>
        <div className={styles.formGroup} style={{ flex: 0, minWidth: '70px' }}>
          <label className={styles.label}>{t('admin.createChore.field.iconLabel')}</label>
          <input className={styles.input} value={chore.icon} onChange={e => setChore(c => ({ ...c, icon: e.target.value }))} placeholder="🧹" style={{ textAlign: 'center' }} />
        </div>
      </div>

      <div className={styles.formGroup}>
        <div className={styles.labelRow}>
          <label className={styles.label}>{t('admin.createChore.field.descriptionLabel')}</label>
          {chore.title.trim() && (
            <button
              type="button"
              className={styles.aiBtn}
              onClick={handleGenerateDescription}
              disabled={generatingDesc}
              title={t('admin.createChore.ai.generateTitle')}
            >
              {generatingDesc ? <span className={styles.spinnerSmall} /> : <Sparkles size={12} />}
              {generatingDesc ? t('admin.createChore.ai.generating') : t('admin.createChore.ai.aiLabel')}
            </button>
          )}
        </div>
        <input className={styles.input} value={chore.description} onChange={e => setChore(c => ({ ...c, description: e.target.value }))} placeholder={t('admin.createChore.field.descriptionPlaceholder')} />
      </div>

      <div className={styles.formRow}>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.field.categoryLabel')}</label>
          <select className={styles.input} value={chore.category} onChange={e => setChore(c => ({ ...c, category: e.target.value as ChoreData['category'] }))}>
            <option value="required">{t('admin.createChore.category.required')}</option>
            <option value="core">{t('admin.createChore.category.core')}</option>
            <option value="bonus">{t('admin.createChore.category.bonus')}</option>
          </select>
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.field.pointsLabel')}</label>
          <input className={styles.input} type="number" min={0} value={chore.points} onChange={e => setChore(c => ({ ...c, points: parseInt(e.target.value) || 0 }))} />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.field.penaltyLabel')}</label>
          <input className={styles.input} type="number" min={0} value={chore.missedPenalty} onChange={e => setChore(c => ({ ...c, missedPenalty: parseInt(e.target.value) || 0 }))} placeholder="0" />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.field.minutesLabel')}</label>
          <input className={styles.input} type="number" min={0} value={chore.estimatedMinutes} onChange={e => setChore(c => ({ ...c, estimatedMinutes: parseInt(e.target.value) || 0 }))} />
        </div>
      </div>

      {(suggestingPoints || aiSuggestion) && (
        <div className={styles.aiSuggestionRow}>
          {suggestingPoints ? (
            <span className={styles.aiSuggestionText}><span className={styles.spinnerSmall} /> {t('admin.createChore.ai.suggesting')}</span>
          ) : aiSuggestion && (
            <>
              <span className={styles.aiSuggestionText}>
                <Sparkles size={12} /> {t('admin.createChore.ai.suggestion', { points: aiSuggestion.points, minutes: aiSuggestion.estimated_minutes })}
              </span>
              <button type="button" className={styles.aiApplyBtn} onClick={applyAiSuggestion}>{t('admin.createChore.ai.apply')}</button>
            </>
          )}
        </div>
      )}

      <div className={styles.checkRow}>
        <input type="checkbox" checked={chore.requiresApproval} onChange={e => setChore(c => ({ ...c, requiresApproval: e.target.checked }))} />
        <span className={styles.checkLabel}>{t('admin.createChore.field.requiresApproval')}</span>
      </div>
      <div className={styles.checkRow}>
        <input type="checkbox" checked={chore.requiresPhoto} onChange={e => setChore(c => ({ ...c, requiresPhoto: e.target.checked }))} />
        <span className={styles.checkLabel}>{t('admin.createChore.field.requiresPhoto')}</span>
      </div>
      {chore.requiresPhoto && (
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.field.photoSourceLabel')}</label>
          <select className={styles.input} value={chore.photoSource} onChange={e => setChore(c => ({ ...c, photoSource: e.target.value as ChoreData['photoSource'] }))}>
            <option value="child">{t('admin.createChore.photoSource.child')}</option>
            <option value="external">{t('admin.createChore.photoSource.external')}</option>
            <option value="both">{t('admin.createChore.photoSource.both')}</option>
          </select>
        </div>
      )}
    </div>
  );

  // --- STEP 2: Schedule ---
  const renderStep1 = () => (
    <div className={styles.formGrid}>
      <p className={styles.helpText}>{t('admin.createChore.schedule.helpText')}</p>

      <div className={styles.formGroup}>
        <label className={styles.label}>{t('admin.createChore.schedule.assignTo')}</label>
        <div className={styles.userPicker}>
          <button type="button" className={clsx(styles.userPickerBtn, schedule.selectedUsers.length === users.length && styles.userPickerBtnActive)} onClick={toggleAllUsers}>{t('admin.createChore.schedule.allUsers')}</button>
          {users.map(u => (
            <button key={u.id} type="button" className={clsx(styles.userPickerBtn, schedule.selectedUsers.includes(u.id) && styles.userPickerBtnActive)} onClick={() => toggleUser(u.id)}>
              {u.name}
            </button>
          ))}
        </div>
      </div>

      <div className={styles.formGroup}>
        <label className={styles.label}>{t('admin.createChore.schedule.typeLabel')}</label>
        <select className={styles.input} value={schedule.scheduleType} onChange={e => setSchedule(s => ({ ...s, scheduleType: e.target.value as ScheduleData['scheduleType'] }))}>
          <option value="weekly">{t('admin.createChore.scheduleType.weekly')}</option>
          <option value="interval">{t('admin.createChore.scheduleType.interval')}</option>
          <option value="oneoff">{t('admin.createChore.scheduleType.oneoff')}</option>
        </select>
      </div>

      {schedule.scheduleType === 'weekly' && (
        <>
          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.createChore.schedule.daysLabel')}</label>
            <div className={styles.dayPicker}>
              {DAY_LABELS.map((d, i) => (
                <button key={i} type="button" className={clsx(styles.dayBtn, schedule.selectedDays.includes(i) && styles.dayBtnActive)} onClick={() => toggleDay(i)}>{d}</button>
              ))}
            </div>
            <div className={styles.presets}>
              <button type="button" className={styles.presetBtn} onClick={() => setDayPreset([0,1,2,3,4,5,6])}>{t('admin.createChore.preset.everyDay')}</button>
              <button type="button" className={styles.presetBtn} onClick={() => setDayPreset([1,2,3,4,5])}>{t('admin.createChore.preset.weekdays')}</button>
              <button type="button" className={styles.presetBtn} onClick={() => setDayPreset([0,6])}>{t('admin.createChore.preset.weekends')}</button>
            </div>
          </div>
        </>
      )}

      {schedule.scheduleType === 'interval' && (
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.schedule.repeatEvery')}</label>
          <div className={styles.intervalInput}>
            <input className={styles.input} type="number" min={1} value={schedule.interval} onChange={e => setSchedule(s => ({ ...s, interval: parseInt(e.target.value) || 1 }))} />
            <span className={styles.intervalSuffix}>{t('admin.createChore.schedule.intervalSuffix')}</span>
            <input className={styles.input} type="date" value={schedule.intervalStart} onChange={e => setSchedule(s => ({ ...s, intervalStart: e.target.value }))} style={{ width: 'auto' }} />
          </div>
        </div>
      )}

      {schedule.scheduleType === 'oneoff' && (
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.schedule.dateLabel')}</label>
          <input className={styles.input} type="date" value={schedule.specificDate} onChange={e => setSchedule(s => ({ ...s, specificDate: e.target.value }))} />
        </div>
      )}

      <div className={styles.formRow}>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.schedule.availableAt')}</label>
          <input className={styles.input} type="time" value={schedule.availableAt} onChange={e => setSchedule(s => ({ ...s, availableAt: e.target.value }))} />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.createChore.schedule.dueBy')}</label>
          <input className={styles.input} type="time" value={schedule.dueBy} onChange={e => setSchedule(s => ({ ...s, dueBy: e.target.value }))} />
        </div>
      </div>

      {schedule.dueBy && (
        <div className={styles.formRow}>
          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.createChore.schedule.ifMissed')}</label>
            <select className={styles.input} value={schedule.expiryPenalty} onChange={e => setSchedule(s => ({ ...s, expiryPenalty: e.target.value as ScheduleData['expiryPenalty'] }))}>
              <option value="block">{t('admin.createChore.expiryPenalty.block')}</option>
              <option value="no_points">{t('admin.createChore.expiryPenalty.noPoints')}</option>
              <option value="penalty">{t('admin.createChore.expiryPenalty.penalty')}</option>
            </select>
          </div>
          {schedule.expiryPenalty === 'penalty' && (
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.createChore.schedule.deductLabel')}</label>
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
        when = t('admin.createChore.review.intervalWhen', { interval: schedule.interval, start: schedule.intervalStart });
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
            <span className={styles.reviewTitle}>{t('admin.createChore.review.choreDetails')}</span>
            <button className={styles.editLink} onClick={() => setStep(0)}>{t('admin.createChore.review.edit')}</button>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>{t('admin.createChore.review.titleLabel')}</span>
            <span className={styles.reviewValue}>{chore.icon} {chore.title}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>{t('admin.createChore.review.categoryLabel')}</span>
            <span className={clsx(styles.badge, styles[`badge_${chore.category}`])}>{chore.category}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>{t('admin.createChore.review.pointsLabel')}</span>
            <span className={styles.reviewValue}>{chore.points} {t('admin.createChore.review.pts')}{chore.missedPenalty > 0 && ` / −${chore.missedPenalty} ${t('admin.createChore.review.penalty')}`}</span>
          </div>
          {chore.estimatedMinutes > 0 && (
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>{t('admin.createChore.review.timeLabel')}</span>
              <span className={styles.reviewValue}>{chore.estimatedMinutes} {t('admin.createChore.review.min')}</span>
            </div>
          )}
          {chore.description && (
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>{t('admin.createChore.review.descLabel')}</span>
              <span className={styles.reviewValue}>{chore.description}</span>
            </div>
          )}
          {(chore.requiresApproval || chore.requiresPhoto) && (
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>{t('admin.createChore.review.flagsLabel')}</span>
              <span className={styles.reviewValue}>
                {[
                  chore.requiresApproval && t('admin.createChore.review.flagApproval'),
                  chore.requiresPhoto && t('admin.createChore.review.flagPhoto', { source: chore.photoSource === 'child' ? t('admin.createChore.review.photoSourceChild') : chore.photoSource === 'external' ? t('admin.createChore.review.photoSourceExternal') : t('admin.createChore.review.photoSourceBoth') }),
                ].filter(Boolean).join(', ')}
              </span>
            </div>
          )}
        </div>

        <div className={styles.reviewSection}>
          <div className={styles.reviewHeader}>
            <span className={styles.reviewTitle}>{t('admin.createChore.review.scheduleTitle')}</span>
            <button className={styles.editLink} onClick={() => { setSkipSchedule(false); setStep(1); }}>{t('admin.createChore.review.edit')}</button>
          </div>
          {sd ? (
            <>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>{t('admin.createChore.review.assigned')}</span>
                <span className={styles.reviewValue}>{sd.names}</span>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>{t('admin.createChore.review.when')}</span>
                <span className={styles.reviewValue}>{sd.when}</span>
              </div>
              {schedule.availableAt && (
                <div className={styles.reviewRow}>
                  <span className={styles.reviewLabel}>{t('admin.createChore.review.available')}</span>
                  <span className={styles.reviewValue}>{schedule.availableAt}</span>
                </div>
              )}
              {schedule.dueBy && (
                <div className={styles.reviewRow}>
                  <span className={styles.reviewLabel}>{t('admin.createChore.review.dueBy')}</span>
                  <span className={styles.reviewValue}>{schedule.dueBy}</span>
                </div>
              )}
            </>
          ) : (
            <p className={styles.noSchedule}>{t('admin.createChore.review.noSchedule')}</p>
          )}
        </div>
      </div>
    );
  };

  const stepTitles = [
    t('admin.createChore.step.details'),
    t('admin.createChore.step.schedule'),
    t('admin.createChore.step.review'),
  ];

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={t('admin.createChore.modalTitle')} maxWidth="560px">
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
              <ChevronLeft size={16} /> {t('admin.createChore.nav.back')}
            </button>
          )}
        </div>
        <div className={styles.navRight}>
          {step === 1 && (
            <button className={styles.skipBtn} onClick={() => { setSkipSchedule(true); setStep(2); }}>
              {t('admin.createChore.nav.skip')}
            </button>
          )}
          {step < 2 && (
            <button className={styles.btnPrimary} disabled={step === 0 ? !canNext0 : !canNext1} onClick={() => setStep(step + 1)}>
              {t('admin.createChore.nav.next')} <ChevronRight size={16} />
            </button>
          )}
          {step === 2 && (
            <button className={styles.btnPrimary} onClick={handleCreate} disabled={creating}>
              {creating ? <><span className={styles.spinner} /> {t('admin.createChore.nav.creating')}</> : t('admin.createChore.nav.createChore')}
            </button>
          )}
        </div>
      </div>
    </Modal>
  );
};

export default CreateChoreWizard;
