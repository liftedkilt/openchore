import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import type { AIReviewResult } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Camera, Loader2 } from 'lucide-react';
import clsx from 'clsx';

// Lets a parent try the configured model on a photo before trusting it with
// the approval queue. Nothing is saved.
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
    <form onSubmit={handleTest} style={{ marginTop: '1rem' }}>
      <h4>{t('admin.photoReviewTester.heading')}</h4>
      <p className={styles.sectionDesc}>{t('admin.photoReviewTester.description')}</p>
      <div className={styles.formGrid}>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.photoReviewTester.labelChoreName')}</label>
          <input
            className={styles.input}
            value={choreTitle}
            onChange={e => setChoreTitle(e.target.value)}
            placeholder={t('admin.photoReviewTester.placeholderChoreName')}
            disabled={working}
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.photoReviewTester.labelPhoto')}</label>
          <label className={styles.photoUploadLabel} style={{ cursor: working ? 'default' : 'pointer', opacity: working ? 0.5 : 1 }}>
            <Camera size={16} />
            {photoFile ? photoFile.name : t('admin.photoReviewTester.choosePhoto')}
            <input type="file" accept="image/*" capture="environment" onChange={handlePhotoChange} style={{ display: 'none' }} disabled={working} />
          </label>
        </div>
      </div>

      {photoPreview && (
        <div className={styles.photoPreview}>
          <img src={photoPreview} alt={t('admin.photoReviewTester.photoPreviewAlt')} />
        </div>
      )}

      <div className={styles.formActions}>
        <button type="submit" className={styles.btnSecondary} disabled={!choreTitle || !photoFile || working}>
          {working ? <><Loader2 size={16} className={styles.spinning} /> {t('admin.photoReviewTester.buttonWorking')}</> : t('admin.photoReviewTester.buttonTest')}
        </button>
      </div>

      {error && <div className={clsx(styles.statusBox, styles.statusBoxError)}>{error}</div>}

      {result && (
        <div className={clsx(styles.statusBox, result.complete ? styles.statusBoxSuccess : styles.statusBoxReject)}>
          <div className={styles.flexRow} style={{ marginBottom: '0.5rem', fontWeight: 600 }}>
            <span>{result.complete ? t('admin.photoReviewTester.looksDone') : t('admin.photoReviewTester.looksNotDone')}</span>
            <span style={{ marginLeft: 'auto', fontWeight: 400, opacity: 0.7 }}>
              {t('admin.photoReviewTester.confidence', { value: Math.round(result.confidence * 100) })}
            </span>
          </div>
          <div>{result.feedback}</div>
          <div style={{ marginTop: '0.5rem', fontSize: '0.85rem', opacity: 0.7 }}>
            {result.would_approve ? t('admin.photoReviewTester.wouldApprove') : t('admin.photoReviewTester.wouldWait')}
            {' · '}
            {t('admin.photoReviewTester.elapsed', { seconds: (result.elapsed_ms / 1000).toFixed(1) })}
          </div>
        </div>
      )}
    </form>
  );
};
