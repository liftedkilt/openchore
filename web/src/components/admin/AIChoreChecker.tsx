import React, { useState } from 'react';
import { api } from '../../api';
import styles from '../../pages/AdminDashboard.module.css';
import { Check, Camera, Volume2, Loader2 } from 'lucide-react';
import clsx from 'clsx';

export const AIChoreChecker: React.FC = () => {
  const [choreTitle, setChoreTitle] = useState('');
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [step, setStep] = useState<'idle' | 'uploading' | 'analyzing' | 'generating_audio' | 'done' | 'error'>('idle');
  const [result, setResult] = useState<{
    complete: boolean;
    confidence: number;
    feedback: string;
    feedback_audio: string;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [playingAudio, setPlayingAudio] = useState(false);
  const [retryingAudio, setRetryingAudio] = useState(false);

  const handlePhotoChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setPhotoFile(file);
    setResult(null);
    setError(null);
    setStep('idle');
    const reader = new FileReader();
    reader.onload = () => setPhotoPreview(reader.result as string);
    reader.readAsDataURL(file);
  };

  const handleTest = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!choreTitle || !photoFile) return;

    setResult(null);
    setError(null);

    try {
      setStep('uploading');
      const { url } = await api.chores.upload(photoFile);

      setStep('analyzing');
      const res = await api.admin.testAIReview(choreTitle, url);
      setResult(res);

      setStep('done');
    } catch (err: any) {
      setError(err.message || 'Test failed');
      setStep('error');
    }
  };

  const handlePlayAudio = () => {
    if (!result?.feedback_audio) return;
    setPlayingAudio(true);
    const audio = new Audio(result.feedback_audio);
    audio.onended = () => setPlayingAudio(false);
    audio.onerror = () => setPlayingAudio(false);
    audio.play().catch(() => setPlayingAudio(false));
  };

  const handleRetryAudio = async () => {
    if (!result?.feedback) return;
    setRetryingAudio(true);
    try {
      const { audio_url } = await api.admin.synthesizeTTS(result.feedback);
      setResult({ ...result, feedback_audio: audio_url });
    } catch (err: unknown) {
      setError(`TTS failed: ${err instanceof Error ? err.message : 'Unknown error'}`);
    } finally {
      setRetryingAudio(false);
    }
  };

  const stepLabels = [
    { key: 'uploading', label: 'Uploading photo' },
    { key: 'analyzing', label: 'AI analyzing photo' },
    { key: 'generating_audio', label: 'Generating audio' },
  ];
  const activeStepIndex = stepLabels.findIndex(s => s.key === step);
  const isWorking = step === 'uploading' || step === 'analyzing' || step === 'generating_audio';

  return (
    <div className={styles.form}>
      <div className={styles.formHeader}>
        <h3>AI Chore Checker</h3>
      </div>
      <p className={styles.sectionDesc}>
        Type a chore name, snap a photo, and see how the AI evaluates it — including Kokoro TTS audio.
      </p>

      <form onSubmit={handleTest}>
        <div className={styles.formGrid}>
          <div className={styles.formGroup}>
            <label className={styles.label}>Chore Name</label>
            <input
              className={styles.input}
              value={choreTitle}
              onChange={e => setChoreTitle(e.target.value)}
              placeholder="e.g. Pick Up Toys, Make Bed, Clean Kitchen"
              disabled={isWorking}
            />
          </div>

          <div className={styles.formGroup}>
            <label className={styles.label}>Photo</label>
            <label className={styles.photoUploadLabel} style={{ cursor: isWorking ? 'default' : 'pointer', opacity: isWorking ? 0.5 : 1 }}>
              <Camera size={16} />
              {photoFile ? photoFile.name : 'Choose photo...'}
              <input type="file" accept="image/*" capture="environment" onChange={handlePhotoChange} style={{ display: 'none' }} disabled={isWorking} />
            </label>
          </div>
        </div>

        {photoPreview && (
          <div className={styles.photoPreview}>
            <img src={photoPreview} alt="Preview" />
          </div>
        )}

        <div className={styles.formActions}>
          <button type="submit" className={styles.saveBtn} disabled={!choreTitle || !photoFile || isWorking}>
            {isWorking ? <><Loader2 size={16} className={styles.spinning} /> Working...</> : 'Test Review'}
          </button>
        </div>
      </form>

      {isWorking && (
        <div style={{ marginTop: '1rem' }}>
          {stepLabels.map((s, i) => {
            const isActive = s.key === step;
            const isDone = i < activeStepIndex || step === 'done';
            return (
              <div key={s.key} className={styles.stepItem} style={{
                color: isActive ? 'var(--color-primary, #38bdf8)' : isDone ? 'var(--text-secondary)' : 'var(--text-tertiary, rgba(255,255,255,0.3))',
              }}>
                {isActive ? <Loader2 size={14} className={styles.spinning} /> : isDone ? <Check size={14} /> : <div style={{ width: 14, height: 14 }} />}
                <span>{s.label}</span>
              </div>
            );
          })}
        </div>
      )}

      {error && (
        <div className={clsx(styles.statusBox, styles.statusBoxError)}>
          {error}
        </div>
      )}

      {result && step === 'done' && (
        <div className={clsx(styles.statusBox, result.complete ? styles.statusBoxSuccess : styles.statusBoxReject)}>
          <div className={styles.flexRow} style={{ marginBottom: '0.5rem', fontWeight: 600 }}>
            <span style={{ fontSize: '1.2rem' }}>{result.complete ? '✅' : '❌'}</span>
            <span>{result.complete ? 'Approved' : 'Rejected'}</span>
            <span style={{ marginLeft: 'auto', fontWeight: 400, opacity: 0.7 }}>
              Confidence: {(result.confidence * 100).toFixed(0)}%
            </span>
          </div>
          <div className={styles.flexRow}>
            <span style={{ flex: 1 }}>{result.feedback}</span>
            {result.feedback_audio ? (
              <button
                onClick={handlePlayAudio}
                disabled={playingAudio}
                className={styles.audioPlayBtn}
                aria-label="Listen to feedback"
              >
                {playingAudio ? <Loader2 size={16} className={styles.spinning} /> : <Volume2 size={16} />}
              </button>
            ) : (
              <button
                onClick={handleRetryAudio}
                disabled={retryingAudio}
                className={styles.audioPlayBtn}
                aria-label="Generate audio"
              >
                {retryingAudio ? <Loader2 size={16} className={styles.spinning} /> : <Volume2 size={16} />}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
