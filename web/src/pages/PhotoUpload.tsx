import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api } from '../api';
import type { User } from '../types';
import styles from './PhotoUpload.module.css';
import { Camera, Check, AlertCircle, Loader2 } from 'lucide-react';

export const PhotoUpload: React.FC = () => {
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
      setError(err.message || 'Failed to upload photo');
    } finally {
      setLoading(false);
    }
  };

  if (!scheduleId || !date || !userId) {
    return (
      <div className={styles.container}>
        <div className={styles.errorBox}>
          <AlertCircle size={48} />
          <h1>Invalid Link</h1>
          <p>This upload link is missing information.</p>
        </div>
      </div>
    );
  }

  if (done) {
    return (
      <div className={styles.container}>
        <div className={styles.successBox}>
          <div className={styles.checkCircle}><Check size={48} /></div>
          <h1>Photo Uploaded!</h1>
          <p>Great job{user ? `, ${user.name}` : ''}! You can close this tab now.</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <h1>Photo Proof</h1>
        {user && <p>Uploading for {user.name}</p>}
      </header>

      <div className={styles.content}>
        <div className={styles.uploadCard}>
          <Camera size={64} className={styles.cameraIcon} />
          <h2>Take a picture</h2>
          <p>Show your completed chore to earn your points!</p>
          
          <label className={styles.uploadBtn}>
            {loading ? <Loader2 className={styles.spinner} /> : 'Open Camera'}
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
