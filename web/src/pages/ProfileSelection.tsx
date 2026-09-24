import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, APIError } from '../api';
import { useAuth } from '../AuthContext';
import type { AuthProvider, User } from '../types';
import styles from './ProfileSelection.module.css';
import { UserCircle, Monitor, Lock, ArrowLeft, LogIn, AlertCircle } from 'lucide-react';
import PinPad from '../components/PinPad/PinPad';

// What the tapped profile needs before it can sign in.
type Step =
  | { kind: 'pin' }
  | { kind: 'oidc' }
  | { kind: 'claim'; legacyPasscode: boolean };

const AUTH_ERROR_KEYS: Record<string, string> = {
  not_linked: 'profile.authError.notLinked',
  wrong_account: 'profile.authError.wrongAccount',
  provider_error: 'profile.authError.providerError',
  provider_unavailable: 'profile.authError.providerUnavailable',
  invalid_state: 'profile.authError.invalidState',
  exchange_failed: 'profile.authError.exchangeFailed',
  session_required: 'profile.authError.sessionRequired',
  linked_to_other_profile: 'profile.authError.linkedToOther',
};

export const ProfileSelection: React.FC = () => {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [pendingUser, setPendingUser] = useState<User | null>(null);
  const [step, setStep] = useState<Step | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [legacyPasscode, setLegacyPasscode] = useState('');
  const [newPin, setNewPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const { signIn } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // Errors bounced back from an OIDC provider (/login?auth_error=...).
  const authError = searchParams.get('auth_error');
  const authErrorProvider = searchParams.get('provider');

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
    api.auth.providers().then(setProviders).catch(() => setProviders([]));
  }, [navigate]);

  const providerName = useMemo(() => {
    const byId = new Map(providers.map(p => [p.id, p.name]));
    return (id: string) => byId.get(id) ?? id;
  }, [providers]);

  const reset = () => {
    setPendingUser(null);
    setStep(null);
    setError('');
    setLegacyPasscode('');
    setNewPin('');
    setConfirmPin('');
  };

  const describeError = (e: unknown): string => {
    if (e instanceof APIError) {
      const code = e.data?.code;
      if (code === 'incorrect_pin') return t('profile.incorrectPin');
      if (code === 'incorrect_passcode') return t('profile.incorrectPasscode');
      if (code === 'locked_out') {
        return t('profile.lockedOut', { seconds: e.data?.retry_after_seconds ?? 30 });
      }
      return e.message;
    }
    return t('profile.couldNotVerifyPin');
  };

  const attemptLogin = async (user: User, extra: { pin?: string; legacy_passcode?: string; new_pin?: string } = {}) => {
    setBusy(true);
    try {
      const auth = await api.auth.login({ user_id: user.id, ...extra });
      signIn(auth);
      navigate('/');
    } catch (e) {
      if (e instanceof APIError && e.status === 403 && e.data?.code === 'admin_setup_required') {
        setError('');
        setStep({ kind: 'claim', legacyPasscode: !!e.data?.legacy_passcode });
      } else if (e instanceof APIError && e.status === 403 && e.data?.code === 'oidc_required') {
        setError('');
        setStep({ kind: 'oidc' });
      } else {
        setError(describeError(e));
      }
    } finally {
      setBusy(false);
    }
  };

  const handleSelect = (user: User) => {
    setError('');
    if (authError) setSearchParams({}, { replace: true });
    setPendingUser(user);
    if (user.has_pin) {
      setStep({ kind: 'pin' });
      return;
    }
    if (user.auth_providers.length > 0) {
      setStep({ kind: 'oidc' });
      return;
    }
    attemptLogin(user);
  };

  const handleClaim = (e: React.FormEvent) => {
    e.preventDefault();
    if (!pendingUser || !step || step.kind !== 'claim') return;
    if (!/^\d{4,8}$/.test(newPin)) {
      setError(t('profile.claim.pinFormat'));
      return;
    }
    if (newPin !== confirmPin) {
      setError(t('profile.claim.pinMismatch'));
      return;
    }
    attemptLogin(pendingUser, {
      new_pin: newPin,
      ...(step.legacyPasscode ? { legacy_passcode: legacyPasscode } : {}),
    });
  };

  if (users.length === 0) return null; // Let the redirect handle it

  if (pendingUser && step) {
    const oidcButtons = pendingUser.auth_providers.length > 0 && (
      <div className={styles.oidcButtons}>
        {step.kind === 'pin' && <div className={styles.divider}><span>{t('profile.or')}</span></div>}
        {pendingUser.auth_providers.map(id => (
          <a
            key={id}
            className={styles.oidcBtn}
            href={api.auth.oidcLoginURL(id, pendingUser.id)}
          >
            <LogIn size={18} />
            {t('profile.continueWith', { provider: providerName(id) })}
          </a>
        ))}
      </div>
    );

    return (
      <div className={styles.container}>
        <button className={styles.backBtn} onClick={reset}>
          <ArrowLeft size={20} /> {t('profile.back')}
        </button>
        <div className={styles.pinPrompt}>
          <div className={styles.pinAvatar}>
            {pendingUser.avatar_url
              ? <img src={pendingUser.avatar_url} alt={pendingUser.name} />
              : <UserCircle size={80} className={styles.placeholder} />}
          </div>
          <h1 className={styles.pinName}>{pendingUser.name}</h1>

          {step.kind === 'pin' && (
            <PinPad
              prompt={t('profile.enterPin')}
              error={error}
              onSubmit={(pin) => attemptLogin(pendingUser, { pin })}
            />
          )}

          {step.kind === 'oidc' && (
            <p className={styles.stepHint}>{t('profile.signInWithLinked')}</p>
          )}

          {oidcButtons}

          {step.kind !== 'pin' && error && (
            <p className={styles.errorText} role="alert">{error}</p>
          )}

          {step.kind === 'claim' && (
            <form className={styles.claimForm} onSubmit={handleClaim}>
              <p className={styles.stepHint}>
                {step.legacyPasscode ? t('profile.claim.introLegacy') : t('profile.claim.intro')}
              </p>
              {step.legacyPasscode && (
                <label className={styles.field}>
                  <span>{t('profile.claim.legacyPasscode')}</span>
                  <input
                    type="password"
                    autoComplete="current-password"
                    value={legacyPasscode}
                    onChange={e => setLegacyPasscode(e.target.value)}
                    required
                  />
                </label>
              )}
              <label className={styles.field}>
                <span>{t('profile.claim.newPin')}</span>
                <input
                  type="password"
                  inputMode="numeric"
                  autoComplete="new-password"
                  value={newPin}
                  onChange={e => setNewPin(e.target.value.replace(/\D/g, '').slice(0, 8))}
                  required
                />
              </label>
              <label className={styles.field}>
                <span>{t('profile.claim.confirmPin')}</span>
                <input
                  type="password"
                  inputMode="numeric"
                  autoComplete="new-password"
                  value={confirmPin}
                  onChange={e => setConfirmPin(e.target.value.replace(/\D/g, '').slice(0, 8))}
                  required
                />
              </label>
              <button type="submit" className={styles.primaryBtn} disabled={busy}>
                {t('profile.claim.submit')}
              </button>
            </form>
          )}
        </div>
      </div>
    );
  }

  // Show kids first, then parents
  const sorted = [...users].sort((a, b) => {
    if (a.role !== b.role) return a.role === 'child' ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  const kids = sorted.filter(u => u.role === 'child');
  const admins = sorted.filter(u => u.role === 'admin');
  const isLocked = (u: User) => u.has_pin || u.auth_providers.length > 0;

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>{t('profile.welcomeBack')}</h1>
      <p className={styles.subtitle}>{t('profile.whoIsDoingChores')}</p>

      {authError && (
        <div className={styles.errorBanner} role="alert">
          <AlertCircle size={18} />
          <span>
            {t(AUTH_ERROR_KEYS[authError] ?? 'profile.authError.generic', {
              provider: authErrorProvider ? providerName(authErrorProvider) : '',
            })}
          </span>
        </div>
      )}
      {!pendingUser && error && (
        <div className={styles.errorBanner} role="alert">
          <AlertCircle size={18} />
          <span>{error}</span>
        </div>
      )}

      <div className={styles.grid}>
        {kids.map(u => (
          <button key={u.id} className={styles.card} onClick={() => handleSelect(u)} disabled={busy} aria-label={t('profile.selectProfile', { name: u.name })}>
            <div className={styles.avatarWrapper}>
              {u.avatar_url ? (
                <img src={u.avatar_url} alt={u.name} className={styles.avatar} />
              ) : (
                <UserCircle size={80} className={styles.placeholder} />
              )}
              {isLocked(u) && (
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
              <button key={u.id} className={styles.adminCard} onClick={() => handleSelect(u)} disabled={busy} aria-label={t('profile.selectProfile', { name: u.name })}>
                <div className={styles.adminAvatar}>
                  {u.avatar_url ? <img src={u.avatar_url} alt={u.name} /> : <UserCircle size={32} />}
                </div>
                <span className={styles.adminName}>{u.name}</span>
                {isLocked(u) && <Lock size={12} className={styles.adminLock} />}
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
      </div>
    </div>
  );
};
