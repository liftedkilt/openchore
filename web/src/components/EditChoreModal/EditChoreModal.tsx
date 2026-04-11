import React, { useState, useRef, useEffect } from 'react';
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
        setTtsError('Unable to play audio');
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
      const msg = e instanceof APIError ? (e.data?.error || e.message) : (e instanceof Error ? e.message : 'Failed to regenerate TTS');
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
      const msg = e instanceof APIError ? (e.data?.error || e.message) : (e instanceof Error ? e.message : 'Failed to generate description');
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
              <label className={styles.label}>Penalty</label>
              <input className={styles.input} type="number" min={0} value={missedPenalty} onChange={e => setMissedPenalty(parseInt(e.target.value) || 0)} placeholder="0" />
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

      {/* --- TTS Audio --- */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.sectionTitle}>Kokoro TTS</span>
        </div>
        <div className={styles.formGrid}>
          {ttsAudioURL ? (
            <div className={styles.ttsPlayerRow}>
              <button
                type="button"
                className={styles.ttsPlayBtn}
                onClick={handlePlayPause}
                aria-label={ttsPlaying ? 'Pause TTS audio' : 'Play TTS audio'}
                title={ttsPlaying ? 'Pause' : 'Play'}
              >
                {ttsPlaying ? <Pause size={18} /> : <Play size={18} />}
              </button>
              <audio ref={audioRef} src={audioSrc} preload="none" />
              <span className={styles.ttsHint}>
                {ttsPlaying ? 'Playing…' : 'Click to preview generated audio'}
              </span>
            </div>
          ) : (
            <div className={styles.ttsHint}>No TTS audio has been generated for this chore yet.</div>
          )}

          <div className={styles.formGroup}>
            <label className={styles.label}>Spoken description (TTS prompt)</label>
            <textarea
              className={styles.textarea}
              value={ttsDescription}
              onChange={e => setTtsDescription(e.target.value)}
              placeholder="What should the TTS voice say for this chore?"
              rows={3}
            />
          </div>

          <div className={styles.saveRow}>
            {ttsSaved && <span className={styles.saved}><Check size={14} /> Regenerated</span>}
            {ttsError && <span className={styles.error}>{ttsError}</span>}
            <button
              type="button"
              className={styles.btnSecondary}
              onClick={handleGenerateTTSDescription}
              disabled={ttsGenerating || ttsRegenerating}
              title="Generate a new spoken description with AI"
            >
              <Sparkles size={14} /> {ttsGenerating ? 'Generating…' : 'Suggest Text'}
            </button>
            <button
              type="button"
              className={styles.btnPrimary}
              onClick={handleRegenerateTTS}
              disabled={ttsRegenerating || ttsGenerating || !ttsDescription.trim()}
            >
              <RefreshCw size={14} className={ttsRegenerating ? styles.spin : ''} />
              {' '}
              {ttsRegenerating ? 'Regenerating…' : 'Save & Regenerate Audio'}
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
