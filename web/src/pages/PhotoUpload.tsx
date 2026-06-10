import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api';
import type { User } from '../types';
import styles from './PhotoUpload.module.css';
import { Camera, Check, AlertCircle, Loader2 } from 'lucide-react';

export const PhotoUpload: React.FC = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const scheduleId = parseInt(searchParams.get('scheduleId') || '');
  const date = searchParams.get('date') || '';
  const userId = parseInt(searchParams.get('userId') || '');

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    if (userId) {
      api.users.get(userId).then(setUser).catch(console.error);
    }
  }, [userId]);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setLoading(true);
    setError(null);

    try {
      // 1. Set temporary user ID for auth header if not already logged in on this device
      // In a real app we'd use a signed token, but for family local app this is fine
      if (!localStorage.getItem('openchore_user') && userId) {
        localStorage.setItem('openchore_user', JSON.stringify({ id: userId }));
      }

      // 2. Upload photo
      const { url } = await api.chores.upload(file);

      // 3. Complete chore
      await api.chores.complete(scheduleId, date, url);
      
      setDone(true);
    } catch (err: any) {
      setError(err.message || t('photo.uploadFailed'));
    } finally {
      setLoading(false);
    }
  };

  if (!scheduleId || !date || !userId) {
    return (
      <div className={styles.container}>
        <div className={styles.errorBox}>
          <AlertCircle size={48} />
          <h1>{t('photo.invalidLinkTitle')}</h1>
          <p>{t('photo.invalidLinkBody')}</p>
        </div>
      </div>
    );
  }

  if (done) {
    return (
      <div className={styles.container}>
        <div className={styles.successBox}>
          <div className={styles.checkCircle}><Check size={48} /></div>
          <h1>{t('photo.successTitle')}</h1>
          <p>{user ? t('photo.successBodyNamed', { name: user.name }) : t('photo.successBody')}</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <h1>{t('photo.pageTitle')}</h1>
        {user && <p>{t('photo.uploadingFor', { name: user.name })}</p>}
      </header>

      <div className={styles.content}>
        <div className={styles.uploadCard}>
          <Camera size={64} className={styles.cameraIcon} />
          <h2>{t('photo.takePicture')}</h2>
          <p>{t('photo.takePictureHint')}</p>

          <label className={styles.uploadBtn}>
            {loading ? <Loader2 className={styles.spinner} /> : t('photo.openCamera')}
            <input 
              type="file" 
              accept="image/*" 
              capture="environment" 
              onChange={handleFileChange} 
              disabled={loading}
              hidden
            />
          </label>

          {error && <div className={styles.errorText}>{error}</div>}
        </div>
      </div>
    </div>
  );
};
