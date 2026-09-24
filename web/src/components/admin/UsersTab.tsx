import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../../api';
import type { User, Theme, UserDecayConfig } from '../../types';
import styles from '../../pages/AdminDashboard.module.css';
import { Plus, Trash2, Edit2, X, Save, Clock, Pause, Play, KeyRound, Link2 } from 'lucide-react';
import LinkedAccountsModal from '../LinkedAccounts/LinkedAccountsModal';
import clsx from 'clsx';

const DecayConfigEditor: React.FC<{ userId: number }> = ({ userId }) => {
  const { t } = useTranslation();
  const [config, setConfig] = useState<UserDecayConfig | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [rate, setRate] = useState('5');
  const [intervalHours, setIntervalHours] = useState('24');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api.decay.getConfig(userId).then(cfg => {
      setConfig(cfg);
      setEnabled(cfg.enabled);
      setRate(cfg.decay_rate.toString());
      setIntervalHours(cfg.decay_interval_hours.toString());
    });
  }, [userId]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const updated = await api.decay.setConfig(userId, {
        enabled,
        decay_rate: parseInt(rate) || 5,
        decay_interval_hours: parseInt(intervalHours) || 24,
      });
      setConfig(updated);
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  if (!config) return <div className={styles.scheduleSection} style={{ padding: '0.5rem' }}>{t('admin.usersTab.loading')}</div>;

  return (
    <div className={styles.scheduleSection}>
      <div className={styles.scheduleHeader}>
        <span className={styles.scheduleTitle}>{t('admin.usersTab.decayTitle')}</span>
      </div>
      <div className={styles.scheduleForm}>
        <div className={styles.formGroup}>
          <label className={clsx(styles.label, styles.flexRow)}>
            <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
            {t('admin.usersTab.decayEnableLabel')}
          </label>
          <span className={styles.helpText}>{t('admin.usersTab.decayHelpText')}</span>
        </div>
        {enabled && (
          <div className={styles.formRow}>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.usersTab.decayPointsLabel')}</label>
              <input className={styles.input} type="number" min="1" value={rate} onChange={e => setRate(e.target.value)} />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.label}>{t('admin.usersTab.decayIntervalLabel')}</label>
              <input className={styles.input} type="number" min="1" value={intervalHours} onChange={e => setIntervalHours(e.target.value)} />
            </div>
          </div>
        )}
        <button className={styles.btnPrimary} onClick={handleSave} disabled={saving} style={{ marginTop: '0.5rem' }}>
          <Save size={14} /> {t('admin.usersTab.saveBtn')}
        </button>
      </div>
    </div>
  );
};

const UserForm: React.FC<{
  user: User | null;
  onSave: () => void;
  onCancel: () => void;
}> = ({ user, onSave, onCancel }) => {
  const { t } = useTranslation();
  const [name, setName] = useState(user?.name || '');
  const [role, setRole] = useState<'admin' | 'child'>(user?.role || 'child');
  const [age, setAge] = useState(user?.age?.toString() || '');
  const [userTheme, setUserTheme] = useState<Theme>(user?.theme || 'default');
  const [pin, setPin] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // Parents always need a way to sign in: a PIN, or a linked account.
  const hasCredential = !!user && (user.has_pin || user.auth_providers.length > 0);
  const pinRequired = role === 'admin' && !hasCredential;
  const showPin = !user || pinRequired;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    if (pin && !/^\d{4,8}$/.test(pin)) {
      setError(t('admin.usersTab.pinFormat'));
      return;
    }
    if (pinRequired && !pin) {
      setError(t('admin.usersTab.pinRequiredForParent'));
      return;
    }
    setSaving(true);
    try {
      const data: Partial<User> & { pin?: string } = {
        name,
        role,
        age: age ? parseInt(age) : undefined,
        theme: userTheme,
        avatar_url: `https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=${encodeURIComponent(name)}`,
      };
      if (user) {
        // Promoting someone without a credential: give them the PIN first.
        if (pin) await api.users.setPin(user.id, pin);
        await api.users.update(user.id, data);
      } else {
        if (pin) data.pin = pin;
        await api.users.create(data);
      }
      onSave();
    } catch (err) {
      setError(err instanceof APIError ? err.message : String(err));
    }
    setSaving(false);
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.formHeader}>
        <h3>{user ? t('admin.usersTab.formEditTitle') : t('admin.usersTab.formNewTitle')}</h3>
        <button type="button" className={styles.iconBtn} onClick={onCancel}><X size={18} /></button>
      </div>

      <div className={styles.formGrid}>
        <div className={styles.formRow}>
          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.usersTab.fieldName')}</label>
            <input className={styles.input} value={name} onChange={e => setName(e.target.value)} required placeholder={t('admin.usersTab.fieldNamePlaceholder')} />
          </div>
          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.usersTab.fieldRole')}</label>
            <select className={styles.input} value={role} onChange={e => setRole(e.target.value as 'admin' | 'child')}>
              <option value="child">{t('admin.usersTab.roleChild')}</option>
              <option value="admin">{t('admin.usersTab.roleAdmin')}</option>
            </select>
          </div>
          <div className={styles.formGroup}>
            <label className={styles.label}>{t('admin.usersTab.fieldAge')}</label>
            <input className={styles.input} type="number" min="1" max="99" value={age} onChange={e => setAge(e.target.value)} placeholder={t('admin.usersTab.fieldAgePlaceholder')} />
          </div>
        </div>
        {showPin && (
          <div className={styles.formGroup}>
            <label className={styles.label}>
              {pinRequired ? t('admin.usersTab.fieldPinRequired') : t('admin.usersTab.fieldPinOptional')}
            </label>
            <input
              className={styles.input}
              type="password"
              inputMode="numeric"
              autoComplete="new-password"
              value={pin}
              onChange={e => setPin(e.target.value.replace(/\D/g, '').slice(0, 8))}
              placeholder={t('admin.usersTab.fieldPinPlaceholder')}
              required={pinRequired}
            />
            <span className={styles.helpText}>{t('admin.usersTab.fieldPinHelp')}</span>
          </div>
        )}
        <div className={styles.formGroup}>
          <label className={styles.label}>{t('admin.usersTab.fieldTheme')}</label>
          <select className={styles.input} value={userTheme} onChange={e => setUserTheme(e.target.value as Theme)}>
            <option value="default">{t('admin.usersTab.themeDefault')}</option>
            <option value="quest">{t('admin.usersTab.themeQuest')}</option>
            <option value="galaxy">{t('admin.usersTab.themeGalaxy')}</option>
            <option value="forest">{t('admin.usersTab.themeForest')}</option>
          </select>
        </div>
      </div>

      {error && (
        <p className={clsx(styles.feedbackMsg, styles.feedbackMsgError)} role="alert">{error}</p>
      )}

      <div className={styles.formActions}>
        <button type="button" className={styles.btnSecondary} onClick={onCancel}>{t('admin.usersTab.cancelBtn')}</button>
        <button type="submit" className={styles.btnPrimary} disabled={saving || !name}>
          <Save size={16} /> {user ? t('admin.usersTab.updateBtn') : t('admin.usersTab.createBtn')}
        </button>
      </div>
    </form>
  );
};

export const UsersTab: React.FC = () => {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [expandedDecay, setExpandedDecay] = useState<number | null>(null);
  const [accountsFor, setAccountsFor] = useState<User | null>(null);
  const [listError, setListError] = useState('');

  const load = useCallback(async () => {
    const u = await api.users.list();
    setUsers(u);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDelete = async (id: number) => {
    setListError('');
    try {
      await api.users.delete(id);
    } catch (err) {
      setListError(err instanceof APIError ? err.message : String(err));
    }
    load();
  };

  const handleTogglePause = async (user: User) => {
    try {
      if (user.paused) {
        await api.users.unpause(user.id);
      } else {
        await api.users.pause(user.id);
      }
      load();
    } catch (err) {
      console.error(err);
    }
  };

  const handleClearPin = async (user: User) => {
    if (!confirm(t('admin.usersTab.confirmResetPin', { name: user.name }))) return;
    setListError('');
    try {
      await api.users.clearPin(user.id);
      load();
    } catch (err) {
      setListError(err instanceof APIError ? err.message : String(err));
    }
  };

  const handleSaved = () => {
    setShowForm(false);
    setEditingUser(null);
    load();
  };

  return (
    <div>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>{t('admin.usersTab.sectionTitle')}</h2>
        <button className={styles.addBtn} onClick={() => { setEditingUser(null); setShowForm(true); }}>
          <Plus size={18} /> {t('admin.usersTab.addPersonBtn')}
        </button>
      </div>

      {showForm && (
        <UserForm
          user={editingUser}
          onSave={handleSaved}
          onCancel={() => { setShowForm(false); setEditingUser(null); }}
        />
      )}

      {listError && (
        <p className={clsx(styles.feedbackMsg, styles.feedbackMsgError)} role="alert">{listError}</p>
      )}

      {accountsFor && (
        <LinkedAccountsModal
          user={accountsFor}
          self={false}
          onClose={() => setAccountsFor(null)}
          onChanged={() => load()}
        />
      )}

      <div className={styles.list}>
        {users.map(u => (
          <div key={u.id} className={clsx(styles.listItem, u.paused && styles.listItemPaused)}>
            <div className={styles.listItemMain}>
              <div className={styles.userAvatar}>
                {u.avatar_url ? <img src={u.avatar_url} alt={u.name} /> : <div className={styles.userAvatarPlaceholder} />}
              </div>
              <div className={styles.listItemInfo}>
                <h3 className={styles.listItemTitle}>{u.name}</h3>
                <div className={styles.listItemMeta}>
                  <span className={clsx(styles.badge, u.role === 'admin' ? styles.badge_admin : styles.badge_child)}>
                    {u.role === 'admin' ? t('admin.usersTab.roleAdmin') : t('admin.usersTab.roleChild')}
                  </span>
                  {u.paused && <span className={clsx(styles.badge, styles.badge_paused)}>{t('admin.usersTab.badgePaused')}</span>}
                  {u.has_pin && <span className={clsx(styles.badge, styles.badge_child)}>{t('admin.usersTab.badgePin')}</span>}
                  {u.auth_providers.length > 0 && (
                    <span className={clsx(styles.badge, styles.badge_child)}>{t('admin.usersTab.badgeLinked', { count: u.auth_providers.length })}</span>
                  )}
                  {u.age && <span>{t('admin.usersTab.ageDisplay', { age: u.age })}</span>}
                </div>
              </div>
              <div className={styles.listItemActions}>
                {u.has_pin && (
                  <button
                    className={styles.iconBtn}
                    onClick={() => handleClearPin(u)}
                    title={t('admin.usersTab.resetPinTitle')}
                    aria-label={t('admin.usersTab.resetPinTitle')}
                  >
                    <KeyRound size={16} />
                  </button>
                )}
                {u.auth_providers.length > 0 && (
                  <button
                    className={styles.iconBtn}
                    onClick={() => setAccountsFor(u)}
                    title={t('admin.usersTab.linkedAccountsTitle')}
                    aria-label={t('admin.usersTab.linkedAccountsTitle')}
                  >
                    <Link2 size={16} />
                  </button>
                )}
                <button
                  className={clsx(styles.iconBtn, u.paused && styles.iconBtnActive)}
                  onClick={() => handleTogglePause(u)}
                  title={u.paused ? t('admin.usersTab.unpauseTitle') : t('admin.usersTab.pauseTitle')}
                  aria-label={u.paused ? t('admin.usersTab.unpauseAriaLabel') : t('admin.usersTab.pauseAriaLabel')}
                >
                  {u.paused ? <Play size={16} /> : <Pause size={16} />}
                </button>
                <button className={styles.iconBtn} onClick={() => setExpandedDecay(expandedDecay === u.id ? null : u.id)} title={t('admin.usersTab.decaySettingsTitle')} aria-label={t('admin.usersTab.decaySettingsTitle')}>
                  <Clock size={16} />
                </button>
                <button className={styles.iconBtn} aria-label={t('admin.usersTab.editUserAriaLabel')} onClick={() => { setEditingUser(u); setShowForm(true); }}>
                  <Edit2 size={16} />
                </button>
                <button className={clsx(styles.iconBtn, styles.iconBtnDanger)} aria-label={t('admin.usersTab.deleteUserAriaLabel')} onClick={() => handleDelete(u.id)}>
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
            {expandedDecay === u.id && (
              <DecayConfigEditor userId={u.id} />
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
