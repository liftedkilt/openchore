import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Trash2, Pencil, X, Pause, Play, KeyRound, Link2, TrendingDown } from 'lucide-react';
import { api, APIError } from '../../api';
import type { User, UserDecayConfig } from '../../types';
import { Avatar, Icon, resolveSkin, type PersonColor, type Skin } from '../../design';
import LinkedAccountsModal from '../LinkedAccounts/LinkedAccountsModal';
import { ColorPicker, SkinPicker, personColor } from './pickers';
import ui from './ui.module.css';
import styles from './UsersTab.module.css';

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

  if (!config) return <div className={ui.inset}><p className={ui.emptyInline}>{t('admin.usersTab.loading')}</p></div>;

  return (
    <div className={ui.inset}>
      <h4 className={ui.eyebrow}>{t('admin.usersTab.decayTitle')}</h4>
      <div>
        <label className={ui.check}>
          <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
          <span className={ui.checkLabel}>{t('admin.usersTab.decayEnableLabel')}</span>
        </label>
        <p className={ui.help}>{t('admin.usersTab.decayHelpText')}</p>
      </div>
      {enabled && (
        <div className={ui.formRow}>
          <label className={ui.field}>
            <span className={ui.label}>{t('admin.usersTab.decayPointsLabel')}</span>
            <input className={ui.input} type="number" min="1" value={rate} onChange={e => setRate(e.target.value)} />
          </label>
          <label className={ui.field}>
            <span className={ui.label}>{t('admin.usersTab.decayIntervalLabel')}</span>
            <input className={ui.input} type="number" min="1" value={intervalHours} onChange={e => setIntervalHours(e.target.value)} />
          </label>
        </div>
      )}
      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnPrimary} onClick={handleSave} disabled={saving}>
          <Icon name="check" /> {t('admin.usersTab.saveBtn')}
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
  // A stored skin wins; otherwise suggest one by age (Blocks under 8).
  const [skinChosen, setSkinChosen] = useState<Skin | null>(user?.theme ? resolveSkin(user.theme, user.age) : null);
  const [color, setColor] = useState<PersonColor | undefined>(personColor(user));
  const [pin, setPin] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const skin: Skin = skinChosen ?? resolveSkin('', age ? parseInt(age) : undefined);

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
        theme: skin,
        // Empty on create: the server assigns the next free colour.
        ...(color ? { color } : {}),
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
    <form className={clsx(ui.card, ui.formGrid)} onSubmit={handleSubmit}>
      <div className={ui.sectionHead}>
        <h3 className={ui.sectionTitle}>
          <Avatar name={name || '?'} color={color} size="sm" />
          {user ? t('admin.usersTab.formEditTitle') : t('admin.usersTab.formNewTitle')}
        </h3>
        <button type="button" className={ui.iconBtn} onClick={onCancel} aria-label={t('admin.usersTab.cancelBtn')}><X aria-hidden /></button>
      </div>

      <div className={ui.formRow}>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.usersTab.fieldName')}</span>
          <input className={ui.input} value={name} onChange={e => setName(e.target.value)} required placeholder={t('admin.usersTab.fieldNamePlaceholder')} />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.usersTab.fieldRole')}</span>
          <select className={ui.input} value={role} onChange={e => setRole(e.target.value as 'admin' | 'child')}>
            <option value="child">{t('admin.usersTab.roleChild')}</option>
            <option value="admin">{t('admin.usersTab.roleAdmin')}</option>
          </select>
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.usersTab.fieldAge')}</span>
          <input className={ui.input} type="number" min="1" max="99" value={age} onChange={e => setAge(e.target.value)} placeholder={t('admin.usersTab.fieldAgePlaceholder')} />
        </label>
      </div>

      {showPin && (
        <label className={ui.field}>
          <span className={ui.label}>
            {pinRequired ? t('admin.usersTab.fieldPinRequired') : t('admin.usersTab.fieldPinOptional')}
          </span>
          <input
            className={ui.input}
            type="password"
            inputMode="numeric"
            autoComplete="new-password"
            value={pin}
            onChange={e => setPin(e.target.value.replace(/\D/g, '').slice(0, 8))}
            placeholder={t('admin.usersTab.fieldPinPlaceholder')}
            required={pinRequired}
          />
          <span className={ui.help}>{t('admin.usersTab.fieldPinHelp')}</span>
        </label>
      )}

      <ColorPicker value={color} onChange={setColor} label={t('admin.usersTab.fieldColor')} />

      <div className={ui.field}>
        <SkinPicker value={skin} onChange={setSkinChosen} color={color} name={name} label={t('admin.usersTab.fieldSkin')} />
        {!skinChosen && <span className={ui.help}>{t('admin.usersTab.skinSuggested')}</span>}
      </div>

      {error && (
        <p className={ui.msgError} role="alert">{error}</p>
      )}

      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnGhost} onClick={onCancel}>{t('admin.usersTab.cancelBtn')}</button>
        <button type="submit" className={ui.btnPrimary} disabled={saving || !name}>
          <Icon name="check" /> {user ? t('admin.usersTab.updateBtn') : t('admin.usersTab.createBtn')}
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

  const handleDelete = async (user: User) => {
    setListError('');
    try {
      await api.users.delete(user.id);
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
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.usersTab.sectionTitle')}</h2>
          <p className={ui.pageSub}>{t('admin.usersTab.subtitle')}</p>
        </div>
        <button type="button" className={ui.btnPrimary} onClick={() => { setEditingUser(null); setShowForm(true); }}>
          <Icon name="plus" /> {t('admin.usersTab.addPersonBtn')}
        </button>
      </div>

      {showForm && (
        <UserForm
          key={editingUser?.id ?? 'new'}
          user={editingUser}
          onSave={handleSaved}
          onCancel={() => { setShowForm(false); setEditingUser(null); }}
        />
      )}

      {listError && (
        <p className={ui.msgError} role="alert">{listError}</p>
      )}

      {accountsFor && (
        <LinkedAccountsModal
          user={accountsFor}
          self={false}
          onClose={() => setAccountsFor(null)}
          onChanged={() => load()}
        />
      )}

      <div className={ui.list}>
        {users.map(u => {
          const color = personColor(u);
          const skin = resolveSkin(u.theme, u.age);
          return (
            <div key={u.id} className={styles.item}>
              <div className={clsx(ui.row, ui.rowWrap, u.paused && ui.rowMuted)}>
                <Avatar name={u.name} color={color} size="md" />
                <div className={ui.rowMain}>
                  <span className={ui.rowTitle}>{u.name}</span>
                  <span className={ui.rowMeta}>
                    <span className={ui.badge}>
                      {u.role === 'admin' ? t('admin.usersTab.roleAdmin') : t('admin.usersTab.roleChild')}
                    </span>
                    {u.paused && <span className={ui.badgeOutline}>{t('admin.usersTab.badgePaused')}</span>}
                    {u.has_pin && <span className={ui.badgeOutline}><Icon name="lock" /> {t('admin.usersTab.badgePin')}</span>}
                    {u.auth_providers.length > 0 && (
                      <span className={ui.badgeOutline}>{t('admin.usersTab.badgeLinked', { count: u.auth_providers.length })}</span>
                    )}
                    {u.age ? <span>{t('admin.usersTab.ageDisplay', { age: u.age })}</span> : null}
                    <span>{t('admin.usersTab.skinDisplay', { skin: t(`admin.skins.${skin}.name`) })}</span>
                  </span>
                </div>
                <div className={clsx(ui.rowActions, styles.actions)}>
                  {u.has_pin && (
                    <button
                      type="button"
                      className={ui.iconBtn}
                      onClick={() => handleClearPin(u)}
                      title={t('admin.usersTab.resetPinTitle')}
                      aria-label={t('admin.usersTab.resetPinTitle')}
                    >
                      <KeyRound aria-hidden />
                    </button>
                  )}
                  {u.auth_providers.length > 0 && (
                    <button
                      type="button"
                      className={ui.iconBtn}
                      onClick={() => setAccountsFor(u)}
                      title={t('admin.usersTab.linkedAccountsTitle')}
                      aria-label={t('admin.usersTab.linkedAccountsTitle')}
                    >
                      <Link2 aria-hidden />
                    </button>
                  )}
                  <button
                    type="button"
                    className={ui.iconBtn}
                    aria-pressed={u.paused}
                    onClick={() => handleTogglePause(u)}
                    title={u.paused ? t('admin.usersTab.unpauseTitle') : t('admin.usersTab.pauseTitle')}
                    aria-label={u.paused ? t('admin.usersTab.unpauseAriaLabel') : t('admin.usersTab.pauseAriaLabel')}
                  >
                    {u.paused ? <Play aria-hidden /> : <Pause aria-hidden />}
                  </button>
                  <button
                    type="button"
                    className={ui.iconBtn}
                    aria-expanded={expandedDecay === u.id}
                    onClick={() => setExpandedDecay(expandedDecay === u.id ? null : u.id)}
                    title={t('admin.usersTab.decaySettingsTitle')}
                    aria-label={t('admin.usersTab.decaySettingsTitle')}
                  >
                    <TrendingDown aria-hidden />
                  </button>
                  <button
                    type="button"
                    className={ui.iconBtn}
                    title={t('admin.usersTab.editUserAriaLabel')}
                    aria-label={t('admin.usersTab.editUserAriaLabel')}
                    onClick={() => { setEditingUser(u); setShowForm(true); window.scrollTo({ top: 0, behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }); }}
                  >
                    <Pencil aria-hidden />
                  </button>
                  <button
                    type="button"
                    className={clsx(ui.iconBtn, ui.iconBtnDanger)}
                    title={t('admin.usersTab.deleteUserAriaLabel')}
                    aria-label={t('admin.usersTab.deleteUserAriaLabel')}
                    onClick={() => handleDelete(u)}
                  >
                    <Trash2 aria-hidden />
                  </button>
                </div>
              </div>
              {expandedDecay === u.id && (
                <div className={styles.decay}>
                  <DecayConfigEditor userId={u.id} />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
