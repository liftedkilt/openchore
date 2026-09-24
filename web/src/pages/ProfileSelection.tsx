import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api, APIError } from '../api';
import { useAuth } from '../AuthContext';
import type { AuthProvider, User } from '../types';
import {
  Avatar, blobFor, Button, DayProgress, FamilyMember, HouseScope, Icon, SkinScope,
  resolveSkin, salutationFor, useMinuteClock, type AvatarSize, type Skin,
} from '../design';
import { BrandMark } from '../components/BrandMark/BrandMark';
import PinPad from '../components/PinPad/PinPad';
import {
  dayFraction, orderFamily, stripMembers, useFamilyToday, type PersonToday,
} from './ProfileSelection.data';
import styles from './ProfileSelection.module.css';

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

const isLocked = (u: User) => u.has_pin || u.auth_providers.length > 0;

/**
 * The strip repeats every name the doors already show. A trailing word joiner
 * (invisible, never read aloud) keeps each door the only element whose text is
 * exactly the person's name, which is how name lookups (and the e2e helpers)
 * find the way in.
 */
const stripName = (name: string) => `${name}⁠`;

/** A person's avatar: their photo when they have one, else their initial. */
function PersonAvatar({ user, size = 'md', className }: { user: User; size?: AvatarSize; className?: string }) {
  const [broken, setBroken] = useState(false);
  const avatar = <Avatar name={user.name} color={user.color} size={size} className={className} />;
  if (!user.avatar_url || broken) return avatar;
  return (
    <span className={styles.photo} style={{ '--blob': blobFor(user.name) } as React.CSSProperties}>
      {avatar}
      <img src={user.avatar_url} alt="" onError={() => setBroken(true)} />
    </span>
  );
}

/* ---------------- Doors ---------------- */

// Hero sizing is layout, not theme: each skin's hero has its own footprint.
const HERO_CLASS: Record<Skin, string> = {
  sunroom: styles.heroArc,
  blocks: styles.heroShapes,
  tint: styles.heroRing,
};

function DoorMeta({ user, today }: { user: User; today?: PersonToday }) {
  const { t } = useTranslation();
  let left: string | null = null;
  let allDone = false;
  if (user.paused) left = t('entry.picker.paused');
  else if (today && today.total === 0) left = t('entry.picker.nothingToday');
  else if (today && today.done >= today.total) { left = t('entry.picker.allDone'); allDone = true; }
  else if (today) left = t('entry.picker.toGo', { count: today.total - today.done });
  const balance = today?.balance;
  if (left == null && balance == null) return <p className={styles.doorMeta} />;
  return (
    <p className={styles.doorMeta}>
      {left != null && <span className={clsx(allDone && styles.allDone)}>{left}</span>}
      {left != null && balance != null && <span aria-hidden>·</span>}
      {balance != null && (
        <span className={styles.balance} role="img" aria-label={t('design.points.label', { count: balance })}>
          <Icon name="star" fill />
          {balance}
        </span>
      )}
    </p>
  );
}

interface DoorProps {
  user: User;
  today?: PersonToday;
  now: number;
  disabled: boolean;
  onSelect: (u: User) => void;
}

/** One person's door: drawn in their own skin inside the House frame. */
function Door({ user, today, now, disabled, onSelect }: DoorProps) {
  const { t } = useTranslation();
  const skin = resolveSkin(user.theme, user.age);
  return (
    <li className={styles.doorSlot}>
      <SkinScope skin={skin} color={user.color} door className={styles.door}>
        <PersonAvatar user={user} size="lg" />
        <h2 className={styles.doorName}>
          <button
            type="button"
            className={styles.doorBtn}
            onClick={() => onSelect(user)}
            disabled={disabled}
            aria-label={t('profile.selectProfile', { name: user.name })}
          >
            {user.name}
          </button>
          {isLocked(user) && (
            <span className={styles.lock} role="img" aria-label={t('profile.pinProtected')}>
              <Icon name="lock" />
            </span>
          )}
        </h2>
        <DoorMeta user={user} today={today} />
        {!user.paused && today && today.total > 0 && (
          <div className={styles.heroZone}>
            <div className={clsx(styles.hero, HERO_CLASS[skin])}>
              <DayProgress items={today.items} now={now} />
            </div>
          </div>
        )}
      </SkinScope>
    </li>
  );
}

interface GrownUpsDoorProps {
  parents: User[];
  disabled: boolean;
  onSelect: (u: User) => void;
}

/** The grown-ups share one House-styled door; each parent signs in from it. */
function GrownUpsDoor({ parents, disabled, onSelect }: GrownUpsDoorProps) {
  const { t } = useTranslation();
  return (
    <li className={clsx(styles.doorSlot, styles.grown)}>
      <span className={styles.stack} aria-hidden>
        {parents.slice(0, 3).map((p) => <PersonAvatar key={p.id} user={p} size="lg" />)}
      </span>
      <h2 className={clsx(styles.doorName, styles.grownName)}>
        <Icon name="lock" />
        {t('entry.picker.grownUps')}
      </h2>
      <p className={styles.doorMeta}>{t('entry.picker.grownUpsHint')}</p>
      <div className={styles.parents}>
        {parents.map((p) => (
          <button
            key={p.id}
            type="button"
            className={styles.parentBtn}
            onClick={() => onSelect(p)}
            disabled={disabled}
            aria-label={t('profile.selectProfile', { name: p.name })}
          >
            <PersonAvatar user={p} size="sm" />
            <span className={styles.parentName}>{p.name}</span>
            <Icon name="chev" />
          </button>
        ))}
      </div>
    </li>
  );
}

/* ---------------- Page ---------------- */

export const ProfileSelection: React.FC = () => {
  const { t, i18n } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [pendingUser, setPendingUser] = useState<User | null>(null);
  const [step, setStep] = useState<Step | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [legacyPasscode, setLegacyPasscode] = useState('');
  const [newPin, setNewPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const { signIn, session } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const clock = useMinuteClock();
  const today = useFamilyToday(users);
  // A personal (OIDC) session follows the system setting; a shared device
  // (or no session yet) follows the evening schedule.
  const persistent = session?.persistent ?? false;

  // Errors bounced back from an OIDC provider (/login?auth_error=...).
  const authError = searchParams.get('auth_error');
  const authErrorProvider = searchParams.get('provider');

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

  const { kids, parents } = useMemo(() => orderFamily(users), [users]);

  if (users.length === 0) return null; // Let the redirect handle it

  /* ----- Signing in as one person ----- */
  if (pendingUser && step) {
    const content = (
      <div className={styles.signIn}>
        <button type="button" className={styles.back} onClick={reset}>
          <Icon name="back" />
          {t('profile.back')}
        </button>
        <div className={styles.signInBody}>
          <PersonAvatar user={pendingUser} size="lg" />
          <h1 className={styles.signInName}>{pendingUser.name}</h1>

          {step.kind === 'pin' && (
            <PinPad
              prompt={t('profile.enterPin')}
              error={error}
              onSubmit={(pin) => attemptLogin(pendingUser, { pin })}
            />
          )}

          {step.kind === 'oidc' && (
            <p className={styles.hint}>{t('profile.signInWithLinked')}</p>
          )}

          {pendingUser.auth_providers.length > 0 && (
            <div className={styles.oidc}>
              {step.kind === 'pin' && <div className={styles.divider}><span>{t('profile.or')}</span></div>}
              {pendingUser.auth_providers.map(id => (
                <a key={id} className={styles.oidcBtn} href={api.auth.oidcLoginURL(id, pendingUser.id)}>
                  {t('profile.continueWith', { provider: providerName(id) })}
                  <Icon name="chev" />
                </a>
              ))}
            </div>
          )}

          {step.kind !== 'pin' && error && (
            <p className={styles.error} role="alert">{error}</p>
          )}

          {step.kind === 'claim' && (
            <form className={styles.claim} onSubmit={handleClaim}>
              <p className={styles.hint}>
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
              <Button type="submit" block disabled={busy}>
                {t('profile.claim.submit')}
              </Button>
            </form>
          )}
        </div>
      </div>
    );

    // A kid signs in inside their own skin; grown-ups in the House frame.
    return pendingUser.role === 'child' ? (
      <SkinScope
        skin={resolveSkin(pendingUser.theme, pendingUser.age)}
        color={pendingUser.color}
        className={styles.screen}
      >
        {content}
      </SkinScope>
    ) : (
      <HouseScope persistent={persistent} className={styles.screen}>{content}</HouseScope>
    );
  }

  /* ----- Who's here? ----- */
  const now = dayFraction(clock);
  const locale = i18n.resolvedLanguage || undefined;
  const dateLabel = t('entry.picker.dateTime', {
    date: clock.toLocaleDateString(locale, { weekday: 'long', month: 'long', day: 'numeric' }),
    time: clock.toLocaleTimeString(locale, { hour: 'numeric', minute: '2-digit' }),
  });
  const strip = stripMembers([...kids, ...parents], today);

  return (
    <HouseScope persistent={persistent} className={styles.screen}>
      <div className={styles.page}>
        <header className={styles.top}>
          <BrandMark />
          <time className={styles.date} dateTime={clock.toISOString()}>{dateLabel}</time>
        </header>

        <h1 className={styles.hello}>
          {t(`entry.picker.hello.${salutationFor(clock)}`)}{' '}
          <span>{t('entry.picker.whosHere')}</span>
        </h1>

        {authError && (
          <p className={styles.banner} role="alert">
            {t(AUTH_ERROR_KEYS[authError] ?? 'profile.authError.generic', {
              provider: authErrorProvider ? providerName(authErrorProvider) : '',
            })}
          </p>
        )}
        {error && (
          <p className={styles.banner} role="alert">{error}</p>
        )}

        <ul className={styles.doors} aria-label={t('entry.picker.doorsLabel')}>
          {kids.map(u => (
            <Door key={u.id} user={u} today={today[u.id]} now={now} disabled={busy} onSelect={handleSelect} />
          ))}
          {parents.length > 0 && (
            <GrownUpsDoor parents={parents} disabled={busy} onSelect={handleSelect} />
          )}
        </ul>

        {strip.length > 0 && (
          <section className={styles.family} aria-labelledby="family-today">
            <h2 id="family-today" className={styles.familyTitle}>{t('entry.picker.familyToday')}</h2>
            {strip.map(u => (
              <FamilyMember
                key={u.id}
                name={stripName(u.name)}
                color={u.color}
                done={today[u.id].done}
                total={today[u.id].total}
              />
            ))}
          </section>
        )}

        <footer className={styles.foot}>
          <button type="button" className={styles.wall} onClick={() => navigate('/ambient')}>
            <Icon name="screen" />
            {t('entry.picker.wallDisplay')}
          </button>
        </footer>
      </div>
    </HouseScope>
  );
};
