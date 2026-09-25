// A person's own screens — Today, Week and Rewards — in their own skin.
// Kids and grown-ups alike: a parent's personal screen works the same way.
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, Navigate, Route, Routes, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { SkinScope, TabBar, useMinuteClock, type TabId } from '../../design';
import { useAuth } from '../../AuthContext';
import { useTheme } from '../../ThemeContext';
import { api } from '../../api';
import type { ScheduledChore } from '../../types';
import { useThemeSound } from '../../hooks/useThemeSound';
import { useTextToSpeech } from '../../hooks/useTextToSpeech';
import PinSettingsModal from '../../components/PinPad/PinSettingsModal';
import LinkedAccountsModal from '../../components/LinkedAccounts/LinkedAccountsModal';
import { choreView, gates, isDone, type ChoreView } from './choreView';
import { useKidData } from './useKidData';
import { TopBar, CAT_ORDER } from './parts';
import { TodayScreen } from './TodayScreen';
import { WeekScreen } from './WeekScreen';
import { RewardsScreen } from './RewardsScreen';
import { PhotoSheet } from './PhotoSheet';
import { MeSheet } from './MeSheet';
import { CelebrationLayer } from './CelebrationLayer';
import { choreDomId } from './ChoreItem';
import { catFromCategory } from '../../design';
import s from './kid.module.css';

const HREFS: Record<TabId, string> = { today: '/', week: '/week', rewards: '/rewards' };

const ttsKey = (id: number) => `openchore_tts_${id}`;
function readTts(id: number, age?: number): boolean {
  try {
    const saved = localStorage.getItem(ttsKey(id));
    if (saved !== null) return saved === '1';
  } catch { /* storage blocked */ }
  // Young readers get chores read aloud by default.
  return age !== undefined && age <= 7;
}

interface Celebrating { chore: ScheduledChore; points: number }

export const KidApp: React.FC = () => {
  const { t } = useTranslation();
  const { user, setUser, signOut, isAdmin, refresh } = useAuth();
  const { skin, color, setSkin, setColor } = useTheme();
  const { playComplete, playAllDone } = useThemeSound();
  const { speak, stop } = useTextToSpeech();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const now = useMinuteClock();

  const tab: TabId = location.pathname.startsWith('/week') ? 'week'
    : location.pathname.startsWith('/rewards') ? 'rewards' : 'today';
  const data = useKidData(user, { week: tab === 'week', rewards: tab === 'rewards' });

  const [tts, setTts] = useState(() => (user ? readTts(user.id, user.age) : false));
  const [meOpen, setMeOpen] = useState(false);
  const [pinOpen, setPinOpen] = useState(false);
  const [linkedOpen, setLinkedOpen] = useState(false);
  const [photoChore, setPhotoChore] = useState<ScheduledChore | null>(null);
  const [celebrating, setCelebrating] = useState<Celebrating | null>(null);
  const [baseUrl, setBaseUrl] = useState('');

  // The household's public URL for QR links (a grown-up setting; kids fall
  // back to this device's origin).
  useEffect(() => {
    if (!user) return;
    api.admin.getSetting('base_url').then(d => setBaseUrl(d.value)).catch(() => {});
  }, [user]);

  // Back from linking a sign-in (?linked=<provider> or ?auth_error=...).
  useEffect(() => {
    const linked = searchParams.get('linked');
    const authError = searchParams.get('auth_error');
    if (!linked && !authError) return;
    if (linked) {
      refresh();
      data.showToast(t('linkedAccounts.linkedToast'));
    } else if (authError === 'linked_to_other_profile') {
      data.showToast(t('profile.authError.linkedToOther'));
    } else {
      data.showToast(t('linkedAccounts.linkFailed'));
    }
    setSearchParams({}, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  // --- Chore views ---
  const todayChores = useMemo(
    () => (data.chores ?? []).filter(c => c.date === data.today),
    [data.chores, data.today],
  );
  const { bonusOpen, requiredDone } = gates(todayChores);
  const viewOf = useCallback((c: ScheduledChore): ChoreView => choreView(c, {
    now, today: data.today, bonusOpen, requiredDone, t,
  }), [now, data.today, bonusOpen, requiredDone, t]);

  // The whole day done: the skin's fanfare (not on first load).
  const doneCount = todayChores.filter(isDone).length;
  const allDone = todayChores.length > 0 && doneCount === todayChores.length;
  const prevDone = useRef<boolean | null>(null);
  useEffect(() => {
    if (data.chores == null) return;
    if (prevDone.current === false && allDone) playAllDone();
    prevDone.current = allDone;
  }, [allDone, data.chores, playAllDone]);

  const speakOrPlay = useCallback((text: string, audioUrl?: string) => {
    if (audioUrl) {
      new Audio(audioUrl).play().catch(() => speak(text));
      return;
    }
    speak(text);
  }, [speak]);

  const celebrate = useCallback((chore: ScheduledChore) => {
    playComplete();
    const late = chore.expired && chore.expiry_penalty !== 'block';
    setCelebrating({ chore, points: late ? 0 : chore.points_value || 0 });
  }, [playComplete]);

  const onToggle = useCallback(async (c: ScheduledChore, view: ChoreView) => {
    const result = await data.toggle(c, { retry: view.retry });
    if (result.needsPhoto) setPhotoChore(c);
    if (result.finished) celebrate(c);
  }, [data, celebrate]);

  // The next thing to do after a celebration: the first open chore in order.
  const nextChore = useMemo(() => {
    if (!celebrating) return null;
    for (const cat of CAT_ORDER) {
      for (const c of todayChores) {
        if (catFromCategory(c.category) !== cat || c.schedule_id === celebrating.chore.schedule_id) continue;
        const v = viewOf(c);
        if (v.state === 'todo' && v.canToggle) return c;
      }
    }
    return null;
  }, [celebrating, todayChores, viewOf]);

  const goToChore = (c: ScheduledChore) => {
    if (tab !== 'today') navigate('/');
    requestAnimationFrame(() => {
      const row = document.getElementById(choreDomId(c));
      row?.scrollIntoView({ block: 'center', behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
      row?.querySelector<HTMLButtonElement>('.oc-chore__check')?.focus({ preventScroll: true });
    });
  };

  const onPhotoDone = useCallback(async () => {
    const c = photoChore;
    setPhotoChore(null);
    if (!c) return;
    await data.refreshAll();
    if (!c.completed) celebrate(c);
  }, [photoChore, data, celebrate]);

  const setTtsPref = (on: boolean) => {
    if (!user) return;
    setTts(on);
    try { localStorage.setItem(ttsKey(user.id), on ? '1' : '0'); } catch { /* storage blocked */ }
    if (!on) stop();
  };

  const logout = async () => {
    await signOut();
    navigate('/login');
  };

  const toManage = () => navigate('/admin/dashboard');

  if (!user) return null;

  const topBar = (
    <TopBar
      user={user}
      balance={data.points?.balance ?? 0}
      onMe={() => setMeOpen(true)}
      onManage={isAdmin ? toManage : undefined}
    />
  );
  const shared = { user, data, topBar, viewOf, tts, onToggle, onPhoto: setPhotoChore, onSpeak: speakOrPlay };

  return (
    <SkinScope skin={skin} color={color} className={s.root}>
      <main className={s.main}>
        <Routes>
          <Route index element={<TodayScreen {...shared} now={now} />} />
          <Route path="week" element={<WeekScreen {...shared} />} />
          <Route path="rewards" element={<RewardsScreen user={user} data={data} topBar={topBar} onRedeemed={playComplete} />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>

      <TabBar
        active={tab}
        hrefs={HREFS}
        position="fixed"
        className={s.tabs}
        renderLink={({ tab: id, href, ...p }) => <Link key={id} to={href} {...p} />}
      />

      {data.toast && <div className={s.toast} role="status">{data.toast}</div>}

      {meOpen && (
        <MeSheet
          user={user}
          skin={skin}
          isAdmin={isAdmin}
          tts={tts}
          onClose={() => setMeOpen(false)}
          onSkin={(x) => { setSkin(x).catch(() => data.showToast(t('kid.errors.couldntSave'))); }}
          onColor={(c) => { setColor(c).catch(() => data.showToast(t('kid.errors.couldntSave'))); }}
          onAvatar={async (url) => {
            try {
              const updated = await api.users.updateAvatar(user.id, url);
              setUser({ ...user, ...updated });
            } catch (e) {
              console.error('Failed to update avatar:', e);
              data.showToast(t('kid.errors.couldntSave'));
            }
          }}
          onTts={setTtsPref}
          onPin={() => { setMeOpen(false); setPinOpen(true); }}
          onLinked={() => { setMeOpen(false); setLinkedOpen(true); }}
          onManage={toManage}
          onSignOut={logout}
        />
      )}

      {pinOpen && (
        <PinSettingsModal
          userId={user.id}
          hasPin={user.has_pin}
          onClose={() => setPinOpen(false)}
          onChanged={(hasPin) => setUser({ ...user, has_pin: hasPin })}
        />
      )}

      {linkedOpen && (
        <LinkedAccountsModal
          user={user}
          self
          onClose={() => setLinkedOpen(false)}
          onChanged={(providers) => setUser({ ...user, auth_providers: providers })}
          onSignedOutEverywhere={logout}
        />
      )}

      {photoChore && (
        <PhotoSheet
          chore={photoChore}
          userId={user.id}
          baseUrl={baseUrl}
          onClose={() => setPhotoChore(null)}
          onComplete={onPhotoDone}
        />
      )}

      {celebrating && (
        <CelebrationLayer
          points={celebrating.points}
          title={celebrating.chore.title}
          streak={data.streak?.current_streak}
          next={nextChore?.title}
          onNext={() => {
            setCelebrating(null);
            if (nextChore) goToChore(nextChore);
          }}
          onBack={() => setCelebrating(null)}
        />
      )}
    </SkinScope>
  );
};
