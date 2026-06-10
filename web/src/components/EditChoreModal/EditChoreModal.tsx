import React, { useState, useRef, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Save, Check, Play, Pause, RefreshCw, Sparkles } from 'lucide-react';
import Modal from '../Modal/Modal';
import { api, APIError } from '../../api';
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
  const { t } = useTranslation();
  const [title, setTitle] = useState(chore.title);
  const [description, setDescription] = useState(chore.description);
  const [category, setCategory] = useState(chore.category);
  const [points, setPoints] = useState(chore.points_value);
  const [missedPenalty, setMissedPenalty] = useState(chore.missed_penalty_value || 0);
  const [minutes, setMinutes] = useState(chore.estimated_minutes || 0);
  const [icon, setIcon] = useState(chore.icon || '');
  const [requiresApproval, setRequiresApproval] = useState(chore.requires_approval);
  const [requiresPhoto, setRequiresPhoto] = useState(chore.requires_photo);
  const [photoSource, setPhotoSource] = useState<'child' | 'external' | 'both'>(chore.photo_source || 'child');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');

  // TTS editing state
  const [ttsDescription, setTtsDescription] = useState(chore.tts_description || '');
  const [ttsAudioURL, setTtsAudioURL] = useState(chore.tts_audio_url || '');
  const [ttsCacheBust, setTtsCacheBust] = useState(() => Date.now());
  const [ttsRegenerating, setTtsRegenerating] = useState(false);
  const [ttsGenerating, setTtsGenerating] = useState(false);
  const [ttsSaved, setTtsSaved] = useState(false);
  const [ttsError, setTtsError] = useState('');
  const [ttsPlaying, setTtsPlaying] = useState(false);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    const handleEnded = () => setTtsPlaying(false);
    const handlePause = () => setTtsPlaying(false);
    const handlePlay = () => setTtsPlaying(true);
    audio.addEventListener('ended', handleEnded);
    audio.addEventListener('pause', handlePause);
    audio.addEventListener('play', handlePlay);
    return () => {
      audio.removeEventListener('ended', handleEnded);
      audio.removeEventListener('pause', handlePause);
      audio.removeEventListener('play', handlePlay);
    };
  }, [ttsAudioURL]);

  const audioSrc = ttsAudioURL ? `${ttsAudioURL}?v=${ttsCacheBust}` : '';

  const handlePlayPause = () => {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.paused) {
      audio.play().catch(() => {
        setTtsError(t('admin.editChore.ttsPlayError'));
      });
    } else {
      audio.pause();
    }
  };

  const handleRegenerateTTS = async () => {
    setTtsRegenerating(true);
    setTtsError('');
    setTtsSaved(false);
    try {
      const resp = await api.chores.regenerateTTS(chore.id, ttsDescription.trim());
      setTtsDescription(resp.tts_description);
      setTtsAudioURL(resp.tts_audio_url);
      setTtsCacheBust(Date.now());
      setTtsSaved(true);
      onSaved();
      setTimeout(() => setTtsSaved(false), 2000);
    } catch (e) {
      const msg = e instanceof APIError ? (e.data?.error || e.message) : (e instanceof Error ? e.message : t('admin.editChore.ttsRegenerateError'));
      setTtsError(msg);
    }
    setTtsRegenerating(false);
  };

  const handleGenerateTTSDescription = async () => {
    setTtsGenerating(true);
    setTtsError('');
    try {
      const resp = await api.chores.generateTTSDescription(chore.id);
      setTtsDescription(resp.description);
    } catch (e) {
      const msg = e instanceof APIError ? (e.data?.error || e.message) : (e instanceof Error ? e.message : t('admin.editChore.ttsGenerateDescError'));
      setTtsError(msg);
    }
    setTtsGenerating(false);
  };

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
        missed_penalty_value: missedPenalty || 0,
        estimated_minutes: minutes || undefined,
        requires_approval: requiresApproval,
        requires_photo: requiresPhoto,
        photo_source: requiresPhoto ? photoSource : 'child',
      });
      setSaved(true);
      onSaved();
      setTimeout(() => setSaved(false), 2000);
    } catch (e: any) {
      setError(e.message || t('admin.editChore.saveError'));
    }
    setSaving(false);
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={t('admin.editChore.modalTitle', { title: chore.title })} maxWidth="600px">
      {/* --- Chore Details --- */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.sectionTitle}>{t('admin.editChore.sectionDetails')}</span>
        </div>
        <div className={styles.formGrid}>
          <div className={styles.formRow}>
            <div className={styles.formGroup} style={{ flex: 3 }}>
              <label className={styles.label}>{t('admin.editChore.labelTitle')}</label>
              <input className={styles.input} value={title} onChange={e => setTitle(e.target.value)} />
            </div>
            <div className={styles.formGroup} style={{ flex: 0, minWidth: '65px' }}>
              <label className={styles.label}>{t('admin.editChore.labelIcon')}</label>
              <input className={styles.input} value={icon} onChange={e => setIcon(e.target.value)} style={{ textAlign: 'center' }} />
            </div>
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.editChore.labelDescription')}</label>
            <input className={styles.input} value={description} onChange={e => setDescription(e.target.value)} />
          </div>

          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.editChore.labelCategory')}</label>
              <select className={styles.input} value={category} onChange={e => setCategory(e.target.value as Chore['category'])}>
                <option value="required">{t('admin.editChore.categoryRequired')}</option>
                <option value="core">{t('admin.editChore.categoryCore')}</option>
                <option value="bonus">{t('admin.editChore.categoryBonus')}</option>
              </select>
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.editChore.labelPoints')}</label>
              <input className={styles.input} type="number" min={0} value={points} onChange={e => setPoints(parseInt(e.target.value) || 0)} />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.editChore.labelPenalty')}</label>
              <input className={styles.input} type="number" min={0} value={missedPenalty} onChange={e => setMissedPenalty(parseInt(e.target.value) || 0)} placeholder="0" />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.editChore.labelMinutes')}</label>
              <input className={styles.input} type="number" min={0} value={minutes} onChange={e => setMinutes(parseInt(e.target.value) || 0)} />
            </div>
          </div>

          <div className={styles.checkRow}>
            <input type="checkbox" checked={requiresApproval} onChange={e => setRequiresApproval(e.target.checked)} />
            <span className={styles.checkLabel}>{t('admin.editChore.requiresApproval')}</span>
          </div>
          <div className={styles.checkRow}>
            <input type="checkbox" checked={requiresPhoto} onChange={e => setRequiresPhoto(e.target.checked)} />
            <span className={styles.checkLabel}>{t('admin.editChore.requiresPhoto')}</span>
          </div>
          {requiresPhoto && (
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.editChore.labelPhotoSource')}</label>
              <select className={styles.input} value={photoSource} onChange={e => setPhotoSource(e.target.value as 'child' | 'external' | 'both')}>
                <option value="child">{t('admin.editChore.photoSourceChild')}</option>
                <option value="external">{t('admin.editChore.photoSourceExternal')}</option>
                <option value="both">{t('admin.editChore.photoSourceBoth')}</option>
              </select>
            </div>
          )}

          <div className={styles.saveRow}>
            {saved && <span className={styles.saved}><Check size={14} /> {t('admin.editChore.savedLabel')}</span>}
            {error && <span className={styles.error}>{error}</span>}
            <button className={styles.btnPrimary} onClick={handleSave} disabled={saving || !title.trim()}>
              <Save size={14} /> {saving ? t('admin.editChore.savingLabel') : t('admin.editChore.saveDetailsBtn')}
            </button>
          </div>
        </div>
      </div>

      <hr className={styles.divider} />

      {/* --- TTS Audio --- */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.sectionTitle}>{t('admin.editChore.sectionTTS')}</span>
        </div>
        <div className={styles.formGrid}>
          {ttsAudioURL ? (
            <div className={styles.ttsPlayerRow}>
              <button
                type="button"
                className={styles.ttsPlayBtn}
                onClick={handlePlayPause}
                aria-label={ttsPlaying ? t('admin.editChore.ttsPauseAriaLabel') : t('admin.editChore.ttsPlayAriaLabel')}
                title={ttsPlaying ? t('admin.editChore.ttsPauseTitle') : t('admin.editChore.ttsPlayTitle')}
              >
                {ttsPlaying ? <Pause size={18} /> : <Play size={18} />}
              </button>
              <audio ref={audioRef} src={audioSrc} preload="none" />
              <span className={styles.ttsHint}>
                {ttsPlaying ? t('admin.editChore.ttsPlaying') : t('admin.editChore.ttsClickPreview')}
              </span>
            </div>
          ) : (
            <div className={styles.ttsHint}>{t('admin.editChore.ttsNoAudio')}</div>
          )}

          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.editChore.labelTTSDescription')}</label>
            <textarea
              className={styles.textarea}
              value={ttsDescription}
              onChange={e => setTtsDescription(e.target.value)}
              placeholder={t('admin.editChore.ttsDescriptionPlaceholder')}
              rows={3}
            />
          </div>

          <div className={styles.saveRow}>
            {ttsSaved && <span className={styles.saved}><Check size={14} /> {t('admin.editChore.ttsRegeneratedLabel')}</span>}
            {ttsError && <span className={styles.error}>{ttsError}</span>}
            <button
              type="button"
              className={styles.btnSecondary}
              onClick={handleGenerateTTSDescription}
              disabled={ttsGenerating || ttsRegenerating}
              title={t('admin.editChore.suggestTextTitle')}
            >
              <Sparkles size={14} /> {ttsGenerating ? t('admin.editChore.generatingLabel') : t('admin.editChore.suggestTextBtn')}
            </button>
            <button
              type="button"
              className={styles.btnPrimary}
              onClick={handleRegenerateTTS}
              disabled={ttsRegenerating || ttsGenerating || !ttsDescription.trim()}
            >
              <RefreshCw size={14} className={ttsRegenerating ? styles.spin : ''} />
              {' '}
              {ttsRegenerating ? t('admin.editChore.regeneratingLabel') : t('admin.editChore.regenerateAudioBtn')}
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
