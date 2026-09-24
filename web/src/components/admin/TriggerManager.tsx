import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Trash2, Pencil, X, Pause, Play, Link2, Copy } from 'lucide-react';
import { api } from '../../api';
import type { User, ChoreTrigger } from '../../types';
import { Icon } from '../../design';
import ui from './ui.module.css';
import styles from './managers.module.css';

export const TriggerManager: React.FC<{
  choreId: number;
  users: User[];
}> = ({ choreId, users }) => {
  const [triggers, setTriggers] = useState<ChoreTrigger[]>([]);
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [defaultAssignedTo, setDefaultAssignedTo] = useState<number | ''>('');
  const [defaultDueBy, setDefaultDueBy] = useState('');
  const [defaultAvailableAt, setDefaultAvailableAt] = useState('');
  const [cooldownMinutes, setCooldownMinutes] = useState('0');
  const [assignmentType, setAssignmentType] = useState('individual');
  const [copied, setCopied] = useState<number | null>(null);

  const { t } = useTranslation();

  const load = useCallback(async () => {
    const data = await api.triggers.listForChore(choreId);
    setTriggers(data);
  }, [choreId]);

  useEffect(() => { load(); }, [load]);

  const handleAdd = async () => {
    await api.triggers.create(choreId, {
      default_assigned_to: defaultAssignedTo ? Number(defaultAssignedTo) : undefined,
      default_due_by: defaultDueBy || undefined,
      default_available_at: defaultAvailableAt || undefined,
      cooldown_minutes: parseInt(cooldownMinutes) || 0,
      assignment_type: assignmentType,
    });
    setAdding(false);
    resetForm();
    load();
  };

  const handleUpdate = async (id: number) => {
    await api.triggers.update(id, {
      default_assigned_to: defaultAssignedTo ? Number(defaultAssignedTo) : undefined,
      default_due_by: defaultDueBy || undefined,
      default_available_at: defaultAvailableAt || undefined,
      cooldown_minutes: parseInt(cooldownMinutes) || 0,
      assignment_type: assignmentType,
    });
    setEditingId(null);
    resetForm();
    load();
  };

  const handleToggle = async (trigger: ChoreTrigger) => {
    await api.triggers.update(trigger.id, {
      default_assigned_to: trigger.default_assigned_to,
      default_due_by: trigger.default_due_by,
      default_available_at: trigger.default_available_at,
      cooldown_minutes: trigger.cooldown_minutes,
      assignment_type: trigger.assignment_type,
      enabled: !trigger.enabled,
    });
    load();
  };

  const handleDelete = async (id: number) => {
    await api.triggers.delete(id);
    load();
  };

  const startEdit = (trigger: ChoreTrigger) => {
    setAdding(false);
    setEditingId(trigger.id);
    setDefaultAssignedTo(trigger.default_assigned_to ?? '');
    setDefaultDueBy(trigger.default_due_by ?? '');
    setDefaultAvailableAt(trigger.default_available_at ?? '');
    setCooldownMinutes(String(trigger.cooldown_minutes));
    setAssignmentType(trigger.assignment_type || 'individual');
  };

  const resetForm = () => {
    setDefaultAssignedTo('');
    setDefaultDueBy('');
    setDefaultAvailableAt('');
    setCooldownMinutes('0');
    setAssignmentType('individual');
  };

  const copyUrl = (uuid: string, id: number) => {
    const url = `${window.location.origin}/api/hooks/trigger/${uuid}`;
    navigator.clipboard.writeText(url);
    setCopied(id);
    setTimeout(() => setCopied(null), 2000);
  };

  const getUserName = (id: number) => users.find(u => u.id === id)?.name || `User ${id}`;

  const triggerForm = (
    <>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.triggerManager.assignmentTypeLabel')}</span>
        <select className={ui.input} value={assignmentType} onChange={e => setAssignmentType(e.target.value)}>
          <option value="individual">{t('admin.triggerManager.assignmentTypeIndividual')}</option>
          <option value="fcfs">{t('admin.triggerManager.assignmentTypeFcfs')}</option>
        </select>
        <span className={ui.help}>
          {assignmentType === 'fcfs' ? t('admin.triggerManager.helpFcfs') : t('admin.triggerManager.helpIndividual')}
        </span>
      </label>
      {assignmentType !== 'fcfs' && (
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.triggerManager.defaultAssignedToLabel')}</span>
          <select className={ui.input} value={defaultAssignedTo} onChange={e => setDefaultAssignedTo(e.target.value ? Number(e.target.value) : '')}>
            <option value="">{t('admin.triggerManager.defaultAssignedToNone')}</option>
            {users.map(u => <option key={u.id} value={u.id}>{u.name}</option>)}
          </select>
        </label>
      )}
      <div className={ui.formRow}>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.triggerManager.defaultAvailableAtLabel')}</span>
          <input className={ui.input} type="time" value={defaultAvailableAt} onChange={e => setDefaultAvailableAt(e.target.value)} />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.triggerManager.defaultDueByLabel')}</span>
          <input className={ui.input} type="time" value={defaultDueBy} onChange={e => setDefaultDueBy(e.target.value)} />
        </label>
      </div>
      <label className={ui.field}>
        <span className={ui.label}>{t('admin.triggerManager.cooldownLabel')}</span>
        <input className={ui.input} type="number" min="0" value={cooldownMinutes} onChange={e => setCooldownMinutes(e.target.value)} />
        <span className={ui.help}>{t('admin.triggerManager.cooldownHelp')}</span>
      </label>
    </>
  );

  return (
    <section className={styles.manager}>
      <div className={ui.sectionHead}>
        <h3 className={ui.sectionTitle}><Link2 aria-hidden className={styles.titleIcon} /> {t('admin.triggerManager.sectionTitle')}</h3>
        <button
          type="button"
          className={ui.btnGhost}
          aria-expanded={adding}
          onClick={() => { setAdding(!adding); setEditingId(null); if (!adding) resetForm(); }}
        >
          {adding ? <X aria-hidden /> : <Icon name="plus" />}
          {adding ? t('admin.triggerManager.cancelBtn') : t('admin.triggerManager.addBtn')}
        </button>
      </div>

      {adding && (
        <div className={ui.inset}>
          {triggerForm}
          <div className={ui.actionsEnd}>
            <button type="button" className={ui.btnPrimary} onClick={handleAdd}>
              <Icon name="check" /> {t('admin.triggerManager.createTriggerBtn')}
            </button>
          </div>
        </div>
      )}

      {triggers.length === 0 && !adding && (
        <p className={ui.emptyInline}>{t('admin.triggerManager.emptyState')}</p>
      )}
      {triggers.length > 0 && (
        <div className={styles.list}>
          {triggers.map(trigger => (
            editingId === trigger.id ? (
              <div key={trigger.id} className={ui.inset}>
                {triggerForm}
                <div className={ui.actionsEnd}>
                  <button type="button" className={ui.btnGhost} onClick={() => { setEditingId(null); resetForm(); }}>
                    {t('admin.triggerManager.cancelBtn')}
                  </button>
                  <button type="button" className={ui.btnPrimary} onClick={() => handleUpdate(trigger.id)}>
                    <Icon name="check" /> {t('admin.triggerManager.saveBtn')}
                  </button>
                </div>
              </div>
            ) : (
              <div key={trigger.id} className={clsx(styles.item, !trigger.enabled && ui.rowMuted)}>
                <div className={ui.rowMain}>
                  <button
                    type="button"
                    className={styles.url}
                    onClick={() => copyUrl(trigger.uuid, trigger.id)}
                    title={t('admin.triggerManager.clickToCopy')}
                  >
                    /api/hooks/trigger/{trigger.uuid.substring(0, 8)}…
                  </button>
                  <div className={ui.rowMeta}>
                    {trigger.assignment_type === 'fcfs' && <span className={ui.badge}>FCFS</span>}
                    {trigger.default_assigned_to && <span>{t('admin.triggerManager.metaAssigned', { name: getUserName(trigger.default_assigned_to) })}</span>}
                    {trigger.default_due_by && <span>{t('admin.triggerManager.metaDue', { time: trigger.default_due_by })}</span>}
                    {trigger.cooldown_minutes > 0 && <span>{t('admin.triggerManager.metaCooldown', { minutes: trigger.cooldown_minutes })}</span>}
                    {!trigger.enabled && <span className={ui.badgeOutline}>{t('admin.triggerManager.paused')}</span>}
                  </div>
                </div>
                <div className={ui.rowActions}>
                  <button
                    type="button"
                    className={ui.iconBtn}
                    title={t('admin.triggerManager.copyUrlTitle')}
                    aria-label={t('admin.triggerManager.copyUrlAriaLabel')}
                    onClick={() => copyUrl(trigger.uuid, trigger.id)}
                  >
                    {copied === trigger.id ? <Icon name="check" /> : <Copy aria-hidden />}
                  </button>
                  <button
                    type="button"
                    className={ui.iconBtn}
                    title={trigger.enabled ? t('admin.triggerManager.disableTitle') : t('admin.triggerManager.enableTitle')}
                    aria-label={trigger.enabled ? t('admin.triggerManager.disableAriaLabel') : t('admin.triggerManager.enableAriaLabel')}
                    onClick={() => handleToggle(trigger)}
                  >
                    {trigger.enabled ? <Pause aria-hidden /> : <Play aria-hidden />}
                  </button>
                  <button type="button" className={ui.iconBtn} title={t('admin.triggerManager.editTitle')} aria-label={t('admin.triggerManager.editAriaLabel')} onClick={() => startEdit(trigger)}>
                    <Pencil aria-hidden />
                  </button>
                  <button type="button" className={clsx(ui.iconBtn, ui.iconBtnDanger)} title={t('admin.triggerManager.deleteTitle')} aria-label={t('admin.triggerManager.deleteAriaLabel')} onClick={() => handleDelete(trigger.id)}>
                    <Trash2 aria-hidden />
                  </button>
                </div>
              </div>
            )
          ))}
        </div>
      )}
    </section>
  );
};
