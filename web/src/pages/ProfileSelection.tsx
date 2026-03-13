import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { useAuth } from '../AuthContext';
import type { User } from '../types';
import styles from './ProfileSelection.module.css';
import { UserCircle, Settings, Monitor } from 'lucide-react';

export const ProfileSelection: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const { setUser } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    api.users.list().then(setUsers).catch(console.error);
  }, []);

  const handleSelect = (user: User) => {
    setUser(user);
    navigate('/');
  };

  // Show kids first, then admins
  const sorted = [...users].sort((a, b) => {
    if (a.role !== b.role) return a.role === 'child' ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  const kids = sorted.filter(u => u.role === 'child');
  const admins = sorted.filter(u => u.role === 'admin');

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>Welcome back!</h1>
      <p className={styles.subtitle}>Who's doing chores today?</p>

      <div className={styles.grid}>
        {kids.map(u => (
          <button key={u.id} className={styles.card} onClick={() => handleSelect(u)}>
            <div className={styles.avatarWrapper}>
              {u.avatar_url ? (
                <img src={u.avatar_url} alt={u.name} className={styles.avatar} />
              ) : (
                <UserCircle size={80} className={styles.placeholder} />
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
              <button key={u.id} className={styles.adminCard} onClick={() => handleSelect(u)}>
                <div className={styles.adminAvatar}>
                  {u.avatar_url ? <img src={u.avatar_url} alt={u.name} /> : <UserCircle size={32} />}
                </div>
                <span className={styles.adminName}>{u.name}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className={styles.bottomBtns}>
        <button className={styles.settingsBtn} onClick={() => navigate('/ambient')}>
          <Monitor size={18} />
          <span>Wall Display</span>
        </button>
        <button className={styles.settingsBtn} onClick={() => navigate('/admin')}>
          <Settings size={18} />
          <span>Manage</span>
        </button>
      </div>
    </div>
  );
};
