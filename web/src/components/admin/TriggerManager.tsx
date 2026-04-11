import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../../api';
import type { User, ChoreTrigger } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Edit2, X, Save, Check, Pause, Play, Link2, Copy } from 'lucide-react';
import clsx from 'clsx';

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

  const load = useCallback(async () => {
    const t = await api.triggers.listForChore(choreId);
    setTriggers(t);
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
    <div className={styles.scheduleForm}>
      <div className={styles.formGroup}>
        <label className={styles.label}>Assignment type</label>
        <select className={styles.input} value={assignmentType} onChange={e => setAssignmentType(e.target.value)}>
          <option value="individual">Individual</option>
          <option value="fcfs">First-Come-First-Serve</option>
        </select>
        <span className={styles.helpText}>
          {assignmentType === 'fcfs' ? 'Assigned to all kids — first to complete wins' : 'Assigned to one person'}
        </span>
      </div>
      {assignmentType !== 'fcfs' && (
        <div className={styles.formGroup}>
          <label className={styles.label}>Default assigned to</label>
          <select className={styles.input} value={defaultAssignedTo} onChange={e => setDefaultAssignedTo(e.target.value ? Number(e.target.value) : '')}>
            <option value="">-- None (require param) --</option>
            {users.map(u => <option key={u.id} value={u.id}>{u.name}</option>)}
          </select>
        </div>
      )}
      <div className={styles.formRow}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Default available at</label>
          <input className={styles.input} type="time" value={defaultAvailableAt} onChange={e => setDefaultAvailableAt(e.target.value)} />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Default due by</label>
          <input className={styles.input} type="time" value={defaultDueBy} onChange={e => setDefaultDueBy(e.target.value)} />
        </div>
      </div>
      <div className={styles.formGroup}>
        <label className={styles.label}>Cooldown (minutes)</label>
        <input className={styles.input} type="number" min="0" value={cooldownMinutes} onChange={e => setCooldownMinutes(e.target.value)} />
        <span className={styles.helpText}>0 = no cooldown</span>
      </div>
    </div>
  );

  return (
    <div className={styles.scheduleSection}>
      <div className={styles.scheduleHeader}>
        <span className={styles.scheduleTitle}><Link2 size={14} /> Trigger URLs</span>
        <button className={styles.addBtnSmall} onClick={() => { setAdding(!adding); setEditingId(null); if (!adding) resetForm(); }}>
          {adding ? <X size={14} /> : <Plus size={14} />}
        </button>
      </div>

      {adding && (
        <>
          {triggerForm}
          <button className={styles.saveBtn} onClick={handleAdd}>
            <Save size={14} /> Create Trigger
          </button>
        </>
      )}

      <div className={styles.scheduleList}>
        {triggers.length === 0 && !adding && (
          <p className={styles.helpText} style={{ padding: '0.5rem 0' }}>No triggers yet. Add one to allow external systems to create chore assignments.</p>
        )}
        {triggers.map(trigger => (
          <div key={trigger.id} className={styles.scheduleItem} style={{ opacity: trigger.enabled ? 1 : 0.5 }}>
            {editingId === trigger.id ? (
              <>
                {triggerForm}
                <div className={styles.scheduleItemActions}>
                  <button className={styles.saveBtn} onClick={() => handleUpdate(trigger.id)}>
                    <Save size={14} /> Save
                  </button>
                  <button className={styles.iconBtn} onClick={() => { setEditingId(null); resetForm(); }}>
                    <X size={14} />
                  </button>
                </div>
              </>
            ) : (
              <>
                <div className={styles.triggerInfo}>
                  <code className={styles.triggerUrl} onClick={() => copyUrl(trigger.uuid, trigger.id)} title="Click to copy">
                    /api/hooks/trigger/{trigger.uuid.substring(0, 8)}...
                  </code>
                  <div className={styles.listItemMeta}>
                    {trigger.assignment_type === 'fcfs' && <span className={styles.fcfsBadge}>FCFS</span>}
                    {trigger.default_assigned_to && <span>Assigned: {getUserName(trigger.default_assigned_to)}</span>}
                    {trigger.default_due_by && <span>Due: {trigger.default_due_by}</span>}
                    {trigger.cooldown_minutes > 0 && <span>Cooldown: {trigger.cooldown_minutes}m</span>}
                  </div>
                </div>
                <div className={styles.scheduleItemActions}>
                  <button
                    className={styles.iconBtn}
                    title="Copy URL"
                    aria-label="Copy trigger URL"
                    onClick={() => copyUrl(trigger.uuid, trigger.id)}
                  >
                    {copied === trigger.id ? <Check size={14} /> : <Copy size={14} />}
                  </button>
                  <button
                    className={styles.iconBtn}
                    title={trigger.enabled ? 'Disable' : 'Enable'}
                    aria-label={trigger.enabled ? 'Disable trigger' : 'Enable trigger'}
                    onClick={() => handleToggle(trigger)}
                  >
                    {trigger.enabled ? <Pause size={14} /> : <Play size={14} />}
                  </button>
                  <button className={styles.iconBtn} title="Edit" aria-label="Edit trigger" onClick={() => startEdit(trigger)}>
                    <Edit2 size={14} />
                  </button>
                  <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} title="Delete" aria-label="Delete trigger" onClick={() => handleDelete(trigger.id)}>
                    <Trash2 size={14} />
                  </button>
                </div>
              </>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
