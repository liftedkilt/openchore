import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../api';
import { useAuth } from '../AuthContext';
import type { User } from '../types';
import styles from './ProfileSelection.module.css';
import { UserCircle, Settings, Monitor, Lock, ArrowLeft } from 'lucide-react';
import PinPad from '../components/PinPad/PinPad';

export const ProfileSelection: React.FC = () => {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [pendingUser, setPendingUser] = useState<User | null>(null);
  const [pinError, setPinError] = useState('');
  const { setUser } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    document.body.className = 'theme-default';
  }, []);

  useEffect(() => {
    api.users.list().then(data => {
      setUsers(data);
      if (data.length === 0) {
        navigate('/setup');
      }
    }).catch(console.error);
  }, [navigate]);

  const finalizeLogin = (user: User) => {
    setUser(user);
    navigate('/');
  };

  const handleSelect = (user: User) => {
    if (user.has_pin) {
      setPinError('');
      setPendingUser(user);
      return;
    }
    finalizeLogin(user);
  };

  const handlePinSubmit = async (pin: string) => {
    if (!pendingUser) return;
    try {
      await api.users.verifyPin(pendingUser.id, pin);
      finalizeLogin(pendingUser);
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setPinError(t('profile.incorrectPin'));
      } else {
        setPinError(t('profile.couldNotVerifyPin'));
      }
    }
  };

  if (users.length === 0) return null; // Let the redirect handle it

  if (pendingUser) {
    return (
      <div className={styles.container}>
        <button
          className={styles.backBtn}
          onClick={() => { setPendingUser(null); setPinError(''); }}
        >
          <ArrowLeft size={20} /> {t('profile.back')}
        </button>
        <div className={styles.pinPrompt}>
          <div className={styles.pinAvatar}>
            {pendingUser.avatar_url
              ? <img src={pendingUser.avatar_url} alt={pendingUser.name} />
              : <UserCircle size={80} className={styles.placeholder} />}
          </div>
          <h1 className={styles.pinName}>{pendingUser.name}</h1>
          <PinPad
            prompt={t('profile.enterPin')}
            error={pinError}
            onSubmit={handlePinSubmit}
          />
        </div>
      </div>
    );
  }

  // Show kids first, then admins
  const sorted = [...users].sort((a, b) => {
    if (a.role !== b.role) return a.role === 'child' ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  const kids = sorted.filter(u => u.role === 'child');
  const admins = sorted.filter(u => u.role === 'admin');

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>{t('profile.welcomeBack')}</h1>
      <p className={styles.subtitle}>{t('profile.whoIsDoingChores')}</p>

      <div className={styles.grid}>
        {kids.map(u => (
          <button key={u.id} className={styles.card} onClick={() => handleSelect(u)} role="button" aria-label={t('profile.selectProfile', { name: u.name })}>
            <div className={styles.avatarWrapper}>
              {u.avatar_url ? (
                <img src={u.avatar_url} alt={u.name} className={styles.avatar} />
              ) : (
                <UserCircle size={80} className={styles.placeholder} />
              )}
              {u.has_pin && (
                <div className={styles.lockBadge} aria-label={t('profile.pinProtected')}>
                  <Lock size={14} />
                </div>
              )}
            </div>
            <span className={styles.name}>{u.name}</span>
          </button>
        ))}
      </div>

      {admins.length > 0 && (
        <div className={styles.adminSection}>
          <div className={styles.adminRow}>
            {admins.map(u => (
              <button key={u.id} className={styles.adminCard} onClick={() => handleSelect(u)} role="button" aria-label={t('profile.selectProfile', { name: u.name })}>
                <div className={styles.adminAvatar}>
                  {u.avatar_url ? <img src={u.avatar_url} alt={u.name} /> : <UserCircle size={32} />}
                </div>
                <span className={styles.adminName}>{u.name}</span>
                {u.has_pin && <Lock size={12} className={styles.adminLock} />}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className={styles.bottomBtns}>
        <button className={styles.settingsBtn} onClick={() => navigate('/ambient')}>
          <Monitor size={18} />
          <span>{t('profile.wallDisplay')}</span>
        </button>
        <button className={styles.settingsBtn} onClick={() => navigate('/admin')}>
          <Settings size={18} />
          <span>{t('profile.manage')}</span>
        </button>
      </div>
    </div>
  );
};
