import React, { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, APIError, setBearerToken } from '../api';
import type { User } from '../types';
import { Avatar, Button, HouseScope, Icon, SkinScope, isPersonColor, resolveSkin } from '../design';
import styles from './PhotoUpload.module.css';

/**
 * Photo proof, opened on another device from the QR code on a chore. It
 * renders in the chore owner's skin once it knows who that is, and in House
 * until then (or if it can't tell).
 */
export const PhotoUpload: React.FC = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const scheduleId = parseInt(searchParams.get('scheduleId') || '');
  const date = searchParams.get('date') || '';
  const userId = parseInt(searchParams.get('userId') || '');
  // Short-lived token from the QR code: it can upload a photo and complete
  // this one chore, nothing else. Without it, fall back to this device's own
  // session (e.g. the link was opened on the same tablet).
  const uploadToken = searchParams.get('t');
  const fileRef = useRef<HTMLInputElement>(null);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [lookedUp, setLookedUp] = useState(!userId);

  useEffect(() => {
    if (!userId) return;
    api.users.get(userId)
      .then(setUser)
      .catch(console.error)
      .finally(() => setLookedUp(true));
  }, [userId]);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setLoading(true);
    setError(null);
    try {
      setBearerToken(uploadToken);
      const { url } = await api.chores.upload(file);
      await api.chores.complete(scheduleId, date, url);
      setDone(true);
    } catch (err) {
      const feedback = err instanceof APIError && err.status === 422 ? err.data?.ai_review?.feedback : undefined;
      setError(feedback || (err as Error)?.message || t('kid.upload.failed'));
    } finally {
      setLoading(false);
    }
  };

  let body: React.ReactNode;
  if (!scheduleId || !date || !userId) {
    body = (
      <div className={styles.card}>
        <span className={styles.well} data-kind="error"><Icon name="camera" size={30} /></span>
        <h1 className={styles.title}>{t('kid.upload.invalidTitle')}</h1>
        <p className={styles.text}>{t('kid.upload.invalidBody')}</p>
      </div>
    );
  } else if (done) {
    body = (
      <div className={styles.card} role="status">
        <span className={styles.well} data-kind="done"><Icon name="check" size={34} /></span>
        <h1 className={styles.title}>{t('kid.upload.doneTitle')}</h1>
        <p className={styles.text}>
          {user ? t('kid.upload.doneBodyNamed', { name: user.name.split(' ')[0] }) : t('kid.upload.doneBody')}
        </p>
      </div>
    );
  } else {
    body = (
      <>
        <header className={styles.head}>
          {user && <Avatar name={user.name} color={isPersonColor(user.color) ? user.color : null} src={user.avatar_url || null} />}
          <div>
            <p className={styles.kicker}>{t('kid.upload.kicker')}</p>
            {user && <p className={styles.for}>{t('kid.upload.for', { name: user.name })}</p>}
          </div>
        </header>
        <div className={styles.card}>
          <span className={styles.well}><Icon name="camera" size={30} /></span>
          <h1 className={styles.title}>{t('kid.upload.takeTitle')}</h1>
          <p className={styles.text}>{t('kid.upload.takeHint')}</p>
          <Button block icon={loading ? undefined : 'chev'} onClick={() => fileRef.current?.click()} disabled={loading} aria-busy={loading}>
            {loading ? t('kid.upload.uploading') : t('kid.upload.openCamera')}
          </Button>
          <input ref={fileRef} type="file" accept="image/*" capture="environment" onChange={handleFileChange} hidden />
          {error && <p className={styles.error} role="alert">{error}</p>}
        </div>
      </>
    );
  }

  const page = <main className={styles.page}>{lookedUp ? body : null}</main>;

  return user
    ? <SkinScope skin={resolveSkin(user.theme, user.age)} color={isPersonColor(user.color) ? user.color : null} className={styles.root}>{page}</SkinScope>
    : <HouseScope persistent className={styles.root}>{page}</HouseScope>;
};
