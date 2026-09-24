import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Loader2, X } from 'lucide-react';
import { api } from '../../api';
import { Icon } from '../../design';
import ui from './ui.module.css';
import styles from './AIChoreChecker.module.css';

type Step = 'idle' | 'uploading' | 'analyzing' | 'generating_audio' | 'done' | 'error';

export const AIChoreChecker: React.FC = () => {
  const { t } = useTranslation();
  const [choreTitle, setChoreTitle] = useState('');
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [step, setStep] = useState<Step>('idle');
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
    } catch (err: unknown) {
      setError(err instanceof Error && err.message ? err.message : t('admin.aiChoreChecker.errorTestFailed'));
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
      setError(t('admin.aiChoreChecker.errorTtsFailed', { message: err instanceof Error ? err.message : t('admin.aiChoreChecker.errorUnknown') }));
    } finally {
      setRetryingAudio(false);
    }
  };

  const stepLabels = [
    { key: 'uploading', label: t('admin.aiChoreChecker.stepUploading') },
    { key: 'analyzing', label: t('admin.aiChoreChecker.stepAnalyzing') },
    { key: 'generating_audio', label: t('admin.aiChoreChecker.stepGeneratingAudio') },
  ];
  const activeStepIndex = stepLabels.findIndex(s => s.key === step);
  const isWorking = step === 'uploading' || step === 'analyzing' || step === 'generating_audio';

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.aiChoreChecker.heading')}</h2>
          <p className={ui.pageSub}>{t('admin.aiChoreChecker.description')}</p>
        </div>
      </div>

      <div className={styles.layout}>
        <form className={clsx(ui.card, ui.formGrid)} onSubmit={handleTest}>
          <label className={ui.field}>
            <span className={ui.label}>{t('admin.aiChoreChecker.labelChoreName')}</span>
            <input
              className={ui.input}
              value={choreTitle}
              onChange={e => setChoreTitle(e.target.value)}
              placeholder={t('admin.aiChoreChecker.placeholderChoreName')}
              disabled={isWorking}
            />
          </label>

          <div className={ui.field}>
            <span className={ui.label}>{t('admin.aiChoreChecker.labelPhoto')}</span>
            <label className={clsx(styles.upload, isWorking && styles.uploadDisabled)}>
              {photoPreview
                ? <img src={photoPreview} alt={t('admin.aiChoreChecker.photoPreviewAlt')} />
                : <Icon name="camera" />}
              <span className={styles.uploadText}>{photoFile ? photoFile.name : t('admin.aiChoreChecker.choosePhoto')}</span>
              <input type="file" accept="image/*" capture="environment" onChange={handlePhotoChange} className={ui.srOnlyText} disabled={isWorking} />
            </label>
          </div>

          <div className={ui.actionsEnd}>
            <button type="submit" className={ui.btnPrimary} disabled={!choreTitle || !photoFile || isWorking}>
              {isWorking
                ? <><Loader2 aria-hidden className={ui.spin} /> {t('admin.aiChoreChecker.buttonWorking')}</>
                : <><Icon name="spark" /> {t('admin.aiChoreChecker.buttonTestReview')}</>}
            </button>
          </div>
        </form>

        <div className={styles.results} aria-live="polite">
          {isWorking && (
            <ol className={clsx(ui.card, styles.steps)}>
              {stepLabels.map((s, i) => {
                const isActive = s.key === step;
                const isDone = i < activeStepIndex;
                return (
                  <li key={s.key} className={clsx(styles.step, isActive && styles.stepActive, isDone && styles.stepDone)}>
                    {isActive ? <Loader2 aria-hidden className={ui.spin} /> : isDone ? <Icon name="check" /> : <span className={styles.stepDot} aria-hidden />}
                    <span>{s.label}</span>
                  </li>
                );
              })}
            </ol>
          )}

          {error && (
            <p className={clsx(ui.card, ui.msgError)} role="alert">{error}</p>
          )}

          {result && step === 'done' && (
            <div className={clsx(ui.card, styles.result)}>
              <div className={styles.verdict}>
                <span className={clsx(styles.verdictIcon, !result.complete && styles.verdictNo)} aria-hidden>
                  {result.complete ? <Icon name="check" /> : <X />}
                </span>
                <span className={styles.verdictText}>
                  {result.complete ? t('admin.aiChoreChecker.resultApproved') : t('admin.aiChoreChecker.resultRejected')}
                </span>
                <span className={styles.confidence}>
                  {t('admin.aiChoreChecker.confidence', { value: (result.confidence * 100).toFixed(0) })}
                </span>
              </div>
              <div className={styles.feedback}>
                <p>{result.feedback}</p>
                {result.feedback_audio ? (
                  <button
                    type="button"
                    onClick={handlePlayAudio}
                    disabled={playingAudio}
                    className={ui.iconBtn}
                    aria-label={t('admin.aiChoreChecker.ariaListenFeedback')}
                    title={t('admin.aiChoreChecker.ariaListenFeedback')}
                  >
                    {playingAudio ? <Loader2 aria-hidden className={ui.spin} /> : <Icon name="sound" />}
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={handleRetryAudio}
                    disabled={retryingAudio}
                    className={ui.iconBtn}
                    aria-label={t('admin.aiChoreChecker.ariaGenerateAudio')}
                    title={t('admin.aiChoreChecker.ariaGenerateAudio')}
                  >
                    {retryingAudio ? <Loader2 aria-hidden className={ui.spin} /> : <Icon name="sound" />}
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
