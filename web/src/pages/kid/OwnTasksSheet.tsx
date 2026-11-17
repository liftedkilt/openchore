// "My own tasks": a person adds reminders to their own list — today, a day
// on the calendar, every week or every few days. They're always Optional and
// never worth points (a grown-up can give a chore points from Manage).
import { useEffect, useId, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { CategoryMark, Icon, ICON_NAMES, resolveChoreIcon, type IconName } from '../../design';
import { api } from '../../api';
import type { OwnChore } from '../../types';
import { Sheet } from './Sheet';
import {
  WEEK_ORDER, addDays, emptyForm, formFromSchedules, formProblem, summarize, weekdayName, whenFromForm,
  type OwnTaskForm, type RepeatKind,
} from './ownTasks';
import s from './kid.module.css';

const TASK_ICONS: IconName[] = ICON_NAMES.filter(n => !['back', 'chev', 'plus', 'people', 'lock', 'swap'].includes(n));
const REPEATS: RepeatKind[] = ['once', 'weekly', 'interval'];

export interface OwnTasksSheetProps {
  today: string;
  /** Open straight into a new task for this day (e.g. the day picked on Week). */
  startWith?: { date: string } | null;
  onClose: () => void;
  /** Something was added, changed or removed: reload the chore lists. */
  onChanged: () => void;
}

export function OwnTasksSheet({ today, startWith, onClose, onChanged }: OwnTasksSheetProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const [tasks, setTasks] = useState<OwnChore[] | null>(null);
  const [loadError, setLoadError] = useState(false);
  // null: the list; 'new' or a task: the form.
  const [editing, setEditing] = useState<OwnChore | 'new' | null>(startWith ? 'new' : null);
  const [form, setForm] = useState<OwnTaskForm>(() => emptyForm(today, startWith?.date && startWith.date >= today ? startWith.date : today));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const ids = { title: useId(), icon: useId(), repeat: useId(), days: useId(), date: useId(), start: useId(), end: useId(), every: useId() };

  const load = () => {
    api.ownChores.list()
      .then(list => { setTasks(list); setLoadError(false); })
      .catch(() => setLoadError(true));
  };
  useEffect(load, []);

  const openNew = () => {
    setForm(emptyForm(today));
    setError('');
    setConfirmDelete(false);
    setEditing('new');
  };
  const openEdit = (task: OwnChore) => {
    setForm(formFromSchedules({ title: task.title, icon: task.icon ?? '' }, task.schedules, today));
    setError('');
    setConfirmDelete(false);
    setEditing(task);
  };
  const backToList = () => { setEditing(null); setError(''); };
  const set = (patch: Partial<OwnTaskForm>) => setForm(f => ({ ...f, ...patch }));

  const problem = formProblem(form, today);

  const save = async () => {
    if (problem) { setError(t(problem)); return; }
    setSaving(true);
    setError('');
    const body = { title: form.title.trim(), icon: form.icon, when: whenFromForm(form) };
    try {
      if (editing === 'new') await api.ownChores.create(body);
      else if (editing) await api.ownChores.update(editing.id, body);
      onChanged();
      load();
      setEditing(null);
    } catch (e) {
      setError(e instanceof Error && e.message ? e.message : t('kid.errors.couldntSave'));
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!editing || editing === 'new') return;
    setSaving(true);
    try {
      await api.ownChores.delete(editing.id);
      onChanged();
      load();
      setEditing(null);
    } catch (e) {
      setError(e instanceof Error && e.message ? e.message : t('kid.errors.couldntSave'));
    } finally {
      setSaving(false);
    }
  };

  // ---------- The list ----------
  if (editing === null) {
    return (
      <Sheet title={t('kid.own.title')} onClose={onClose}>
        <p className={s.sheetLead}><span>{t('kid.own.lead')}</span></p>
        {loadError && <p className={s.errorLine} role="alert">{t('kid.own.loadError')}</p>}
        {tasks && tasks.length === 0 && <p className={s.helpLine}>{t('kid.own.empty')}</p>}
        {tasks && tasks.length > 0 && (
          <ul className={s.settingsList} aria-label={t('kid.own.title')}>
            {tasks.map(task => (
              <li key={task.id} className={s.ownItem}>
                <button
                  type="button"
                  className={s.settingRow}
                  onClick={() => task.editable && openEdit(task)}
                  disabled={!task.editable}
                  aria-label={task.editable ? t('kid.own.editLabel', { title: task.title }) : undefined}
                >
                  <span className={s.wellSm} aria-hidden><Icon name={resolveChoreIcon(task.icon, 'optional')} /></span>
                  <span className={s.settingText}>
                    <span>{task.title}</span>
                    <small>{task.editable ? summarize(task.schedules, today, t, lang) : t('kid.own.locked')}</small>
                  </span>
                  {task.editable && <Icon name="chev" />}
                </button>
              </li>
            ))}
          </ul>
        )}
        <div className={s.sheetActions}>
          <button type="button" className={s.primaryAction} onClick={openNew}>
            <Icon name="plus" /> {t('kid.own.add')}
          </button>
        </div>
      </Sheet>
    );
  }

  // ---------- The form ----------
  const isNew = editing === 'new';
  const tomorrow = addDays(today, 1);
  const back = tasks || !isNew ? (
    <button type="button" className={s.backBtn} onClick={backToList} aria-label={t('kid.own.backToList')}>
      <Icon name="back" />
    </button>
  ) : undefined;

  return (
    <Sheet title={isNew ? t('kid.own.newTitle') : t('kid.own.editTitle')} onClose={onClose} leading={back}>
      <p className={s.ownCat}>
        <CategoryMark cat="optional" /> {t('kid.own.alwaysOptional')}
      </p>

      <label className={s.ownField} htmlFor={ids.title}>
        <span className={s.ownLabel}>{t('kid.own.whatLabel')}</span>
        <input
          id={ids.title}
          className={s.input}
          value={form.title}
          maxLength={80}
          autoFocus
          placeholder={t('kid.own.whatPlaceholder')}
          onChange={e => set({ title: e.target.value })}
        />
      </label>

      <div className={s.ownField}>
        <span className={s.ownLabel} id={ids.icon}>{t('kid.own.iconLabel')}</span>
        <div className={clsx(s.pickerGrid, s.ownIcons)} role="radiogroup" aria-labelledby={ids.icon}>
          {TASK_ICONS.map(name => (
            <button
              key={name}
              type="button"
              role="radio"
              aria-checked={form.icon === name}
              aria-label={t(`admin.icons.${name}`)}
              className={clsx(s.pickerItem, s.ownIcon, form.icon === name && s.pickerItemOn)}
              onClick={() => set({ icon: form.icon === name ? '' : name })}
            >
              <Icon name={name} />
            </button>
          ))}
        </div>
      </div>

      <div className={s.ownField}>
        <span className={s.ownLabel} id={ids.repeat}>{t('kid.own.whenLabel')}</span>
        <div className={s.ownSegments} role="radiogroup" aria-labelledby={ids.repeat}>
          {REPEATS.map(r => (
            <button
              key={r}
              type="button"
              role="radio"
              aria-checked={form.repeat === r}
              className={clsx(s.ownSegment, form.repeat === r && s.ownSegmentOn)}
              onClick={() => set({ repeat: r })}
            >
              {t(`kid.own.repeat.${r}`)}
            </button>
          ))}
        </div>
      </div>

      {form.repeat === 'once' && (
        <div className={s.ownField}>
          <label className={s.ownLabel} htmlFor={ids.date}>{t('kid.own.dayLabel')}</label>
          <div className={s.ownChips}>
            <button type="button" className={clsx(s.ownChip, form.date === today && s.ownChipOn)} aria-pressed={form.date === today} onClick={() => set({ date: today })}>
              {t('kid.own.today')}
            </button>
            <button type="button" className={clsx(s.ownChip, form.date === tomorrow && s.ownChipOn)} aria-pressed={form.date === tomorrow} onClick={() => set({ date: tomorrow })}>
              {t('kid.own.tomorrow')}
            </button>
          </div>
          <input id={ids.date} className={s.input} type="date" min={today} value={form.date} onChange={e => set({ date: e.target.value })} />
        </div>
      )}

      {form.repeat === 'weekly' && (
        <div className={s.ownField}>
          <span className={s.ownLabel} id={ids.days}>{t('kid.own.daysLabel')}</span>
          <div className={s.ownDays} role="group" aria-labelledby={ids.days}>
            {WEEK_ORDER.map(d => {
              const on = form.days.includes(d);
              return (
                <button
                  key={d}
                  type="button"
                  aria-pressed={on}
                  aria-label={weekdayName(d, lang, 'long')}
                  className={clsx(s.ownDay, on && s.ownDayOn)}
                  onClick={() => set({ days: on ? form.days.filter(x => x !== d) : [...form.days, d] })}
                >
                  {weekdayName(d, lang)}
                </button>
              );
            })}
          </div>
          <div className={s.ownChips}>
            <button type="button" className={s.ownChip} onClick={() => set({ days: [0, 1, 2, 3, 4, 5, 6] })}>{t('kid.own.everyDay')}</button>
            <button type="button" className={s.ownChip} onClick={() => set({ days: [1, 2, 3, 4, 5] })}>{t('kid.own.schoolDays')}</button>
            <button type="button" className={s.ownChip} onClick={() => set({ days: [0, 6] })}>{t('kid.own.weekends')}</button>
          </div>
        </div>
      )}

      {form.repeat === 'interval' && (
        <div className={s.ownField}>
          <label className={s.ownLabel} htmlFor={ids.every}>{t('kid.own.everyLabel')}</label>
          <div className={s.addRow}>
            <input
              id={ids.every}
              className={clsx(s.input, s.ownNumber)}
              type="number"
              inputMode="numeric"
              min={1}
              max={365}
              value={form.interval || ''}
              onChange={e => set({ interval: parseInt(e.target.value, 10) || 0 })}
            />
            <span className={s.ownSuffix}>{t('kid.own.days', { count: form.interval || 0 })}</span>
          </div>
        </div>
      )}

      {form.repeat !== 'once' && (
        <div className={s.ownRange}>
          <label className={s.ownField} htmlFor={ids.start}>
            <span className={s.ownLabel}>{t('kid.own.startLabel')}</span>
            <input id={ids.start} className={s.input} type="date" min={today} value={form.start} onChange={e => set({ start: e.target.value })} />
          </label>
          <label className={s.ownField} htmlFor={ids.end}>
            <span className={s.ownLabel}>{t('kid.own.endLabel')}</span>
            <input id={ids.end} className={s.input} type="date" min={form.start || today} value={form.end} onChange={e => set({ end: e.target.value })} />
          </label>
        </div>
      )}

      {error && <p className={s.errorLine} role="alert">{error}</p>}

      <div className={s.sheetActions}>
        <button type="button" className={s.primaryAction} onClick={save} disabled={saving}>
          {saving ? <span className={s.spinner} aria-hidden /> : <Icon name="check" />}
          {isNew ? t('kid.own.save') : t('kid.own.saveChanges')}
        </button>
      </div>

      {!isNew && (
        <div className={s.ownDelete}>
          {confirmDelete ? (
            <>
              <span>{t('kid.own.deleteConfirm')}</span>
              <button type="button" className={s.linkBtn} onClick={remove} disabled={saving}>{t('kid.own.deleteYes')}</button>
              <button type="button" className={s.linkBtn} onClick={() => setConfirmDelete(false)}>{t('kid.own.deleteNo')}</button>
            </>
          ) : (
            <button type="button" className={s.linkBtn} onClick={() => setConfirmDelete(true)}>{t('kid.own.delete')}</button>
          )}
        </div>
      )}
    </Sheet>
  );
}
