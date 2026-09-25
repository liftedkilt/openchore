import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Loader2 } from 'lucide-react';
import { api } from '../../api';
import type { AIReviewResult } from '../../types';
import { Icon } from '../../design';
import ui from './ui.module.css';
import styles from './PhotoReviewTester.module.css';

/**
 * Settings → AI: lets a parent try the configured model on a photo before
 * trusting it with the approval queue. Nothing is saved.
 */
export const PhotoReviewTester: React.FC = () => {
  const { t } = useTranslation();
  const [choreTitle, setChoreTitle] = useState('');
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [working, setWorking] = useState(false);
  const [result, setResult] = useState<AIReviewResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handlePhotoChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setPhotoFile(file);
    setResult(null);
    setError(null);
    const reader = new FileReader();
    reader.onload = () => setPhotoPreview(reader.result as string);
    reader.readAsDataURL(file);
  };

  const handleTest = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!choreTitle || !photoFile) return;
    setResult(null);
    setError(null);
    setWorking(true);
    try {
      const { url } = await api.chores.upload(photoFile);
      setResult(await api.admin.testAIReview(choreTitle, url));
    } catch (err: unknown) {
      setError(err instanceof Error && err.message ? err.message : t('admin.photoReviewTester.errorTestFailed'));
    } finally {
      setWorking(false);
    }
  };

  return (
    <form className={clsx(ui.card, ui.section)} onSubmit={handleTest} aria-labelledby="photo-review-tester">
      <h3 className={ui.sectionTitle} id="photo-review-tester"><Icon name="camera" /> {t('admin.photoReviewTester.heading')}</h3>
      <p className={ui.sectionDesc}>{t('admin.photoReviewTester.description')}</p>

      <label className={ui.field}>
        <span className={ui.label}>{t('admin.photoReviewTester.labelChoreName')}</span>
        <input
          className={ui.input}
          value={choreTitle}
          onChange={e => setChoreTitle(e.target.value)}
          placeholder={t('admin.photoReviewTester.placeholderChoreName')}
          disabled={working}
        />
      </label>

      <div className={ui.field}>
        <span className={ui.label}>{t('admin.photoReviewTester.labelPhoto')}</span>
        <label className={clsx(styles.upload, working && styles.uploadDisabled)}>
          {photoPreview
            ? <img src={photoPreview} alt={t('admin.photoReviewTester.photoPreviewAlt')} />
            : <Icon name="camera" />}
          <span className={styles.uploadText}>{photoFile ? photoFile.name : t('admin.photoReviewTester.choosePhoto')}</span>
          <input type="file" accept="image/*" capture="environment" onChange={handlePhotoChange} className={ui.srOnlyText} disabled={working} />
        </label>
      </div>

      <div className={ui.actionsEnd}>
        <button type="submit" className={ui.btnGhost} disabled={!choreTitle || !photoFile || working}>
          {working
            ? <><Loader2 aria-hidden className={ui.spin} /> {t('admin.photoReviewTester.buttonWorking')}</>
            : <><Icon name="spark" /> {t('admin.photoReviewTester.buttonTest')}</>}
        </button>
      </div>

      <div aria-live="polite">
        {error && <p className={ui.msgError} role="alert">{error}</p>}
        {result && (
          <div className={styles.result}>
            <div className={styles.verdict}>
              {/* Words carry the verdict; the mark only repeats it. */}
              <span className={clsx(styles.verdictIcon, !result.complete && styles.verdictNo)} aria-hidden>
                <Icon name={result.complete ? 'check' : 'clock'} />
              </span>
              <span className={styles.verdictText}>
                {result.complete ? t('admin.photoReviewTester.looksDone') : t('admin.photoReviewTester.looksNotDone')}
              </span>
              <span className={styles.confidence}>
                {t('admin.photoReviewTester.confidence', { value: Math.round(result.confidence * 100) })}
              </span>
            </div>
            {result.feedback && <p className={styles.feedback}>{result.feedback}</p>}
            <p className={styles.outcome}>
              {result.would_approve ? t('admin.photoReviewTester.wouldApprove') : t('admin.photoReviewTester.wouldWait')}
              {' · '}
              {t('admin.photoReviewTester.elapsed', { seconds: (result.elapsed_ms / 1000).toFixed(1) })}
            </p>
          </div>
        )}
      </div>
    </form>
  );
};
