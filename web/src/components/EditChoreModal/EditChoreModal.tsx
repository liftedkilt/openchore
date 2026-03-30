import React, { useState } from 'react';
import { Save, Check } from 'lucide-react';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import type { Chore, User } from '../../types';
import styles from './EditChoreModal.module.css';

interface Props {
  chore: Chore;
  isOpen: boolean;
  onClose: () => void;
  onSaved: () => void;
  users: User[];
  /** Render the ScheduleManager component for this chore */
  renderSchedules: (choreId: number, users: User[]) => React.ReactNode;
  /** Render the TriggerManager component for this chore */
  renderTriggers: (choreId: number, users: User[]) => React.ReactNode;
}

const EditChoreModal: React.FC<Props> = ({ chore, isOpen, onClose, onSaved, users, renderSchedules, renderTriggers }) => {
  const [title, setTitle] = useState(chore.title);
  const [description, setDescription] = useState(chore.description);
  const [category, setCategory] = useState(chore.category);
  const [points, setPoints] = useState(chore.points_value);
  const [minutes, setMinutes] = useState(chore.estimated_minutes || 0);
  const [icon, setIcon] = useState(chore.icon || '');
  const [requiresApproval, setRequiresApproval] = useState(chore.requires_approval);
  const [requiresPhoto, setRequiresPhoto] = useState(chore.requires_photo);
  const [photoSource, setPhotoSource] = useState<'child' | 'external' | 'both'>(chore.photo_source || 'child');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');

  const handleSave = async () => {
    setSaving(true);
    setError('');
    setSaved(false);
    try {
      await api.chores.update(chore.id, {
        title: title.trim(),
        description: description.trim(),
        category,
        icon,
        points_value: points,
        estimated_minutes: minutes || undefined,
        requires_approval: requiresApproval,
        requires_photo: requiresPhoto,
        photo_source: requiresPhoto ? photoSource : 'child',
      });
      setSaved(true);
      onSaved();
      setTimeout(() => setSaved(false), 2000);
    } catch (e: any) {
      setError(e.message || 'Failed to save');
    }
    setSaving(false);
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`Edit: ${chore.title}`} maxWidth="600px">
      {/* --- Chore Details --- */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.sectionTitle}>Chore Details</span>
        </div>
        <div className={styles.formGrid}>
          <div className={styles.formRow}>
            <div className={styles.formGroup} style={{ flex: 3 }}>
              <label className={styles.label}>Title</label>
              <input className={styles.input} value={title} onChange={e => setTitle(e.target.value)} />
            </div>
            <div className={styles.formGroup} style={{ flex: 0, minWidth: '65px' }}>
              <label className={styles.label}>Icon</label>
              <input className={styles.input} value={icon} onChange={e => setIcon(e.target.value)} style={{ textAlign: 'center' }} />
            </div>
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label}>Description</label>
            <input className={styles.input} value={description} onChange={e => setDescription(e.target.value)} />
          </div>

          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label}>Category</label>
              <select className={styles.input} value={category} onChange={e => setCategory(e.target.value as Chore['category'])}>
                <option value="required">Required</option>
                <option value="core">Core</option>
                <option value="bonus">Bonus</option>
              </select>
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>Points</label>
              <input className={styles.input} type="number" min={0} value={points} onChange={e => setPoints(parseInt(e.target.value) || 0)} />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>Minutes</label>
              <input className={styles.input} type="number" min={0} value={minutes} onChange={e => setMinutes(parseInt(e.target.value) || 0)} />
            </div>
          </div>

          <div className={styles.checkRow}>
            <input type="checkbox" checked={requiresApproval} onChange={e => setRequiresApproval(e.target.checked)} />
            <span className={styles.checkLabel}>Requires parent approval</span>
          </div>
          <div className={styles.checkRow}>
            <input type="checkbox" checked={requiresPhoto} onChange={e => setRequiresPhoto(e.target.checked)} />
            <span className={styles.checkLabel}>Requires photo proof</span>
          </div>
          {requiresPhoto && (
            <div className={styles.formGroup}>
              <label className={styles.label}>Photo source</label>
              <select className={styles.input} value={photoSource} onChange={e => setPhotoSource(e.target.value as 'child' | 'external' | 'both')}>
                <option value="child">Child uploads photo</option>
                <option value="external">External system (e.g. camera)</option>
                <option value="both">External with manual fallback</option>
              </select>
            </div>
          )}

          <div className={styles.saveRow}>
            {saved && <span className={styles.saved}><Check size={14} /> Saved</span>}
            {error && <span className={styles.error}>{error}</span>}
            <button className={styles.btnPrimary} onClick={handleSave} disabled={saving || !title.trim()}>
              <Save size={14} /> {saving ? 'Saving...' : 'Save Details'}
            </button>
          </div>
        </div>
      </div>

      <hr className={styles.divider} />

      {/* --- Schedules --- */}
      {renderSchedules(chore.id, users)}

      <hr className={styles.divider} />

      {/* --- Triggers --- */}
      {renderTriggers(chore.id, users)}
    </Modal>
  );
};

export default EditChoreModal;
