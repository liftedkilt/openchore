import React, { useState, useEffect, useCallback, useRef, type TouchEvent as ReactTouchEvent } from 'react';
import { useAuth } from '../AuthContext';
import { useTheme } from '../ThemeContext';
import { api } from '../api';
import type { ScheduledChore, UserStreakData, PointsData, Reward, RedemptionHistory, Theme } from '../types';
import styles from './Dashboard.module.css';
import { CheckCircle, Clock, Calendar, Star, LogOut, LayoutDashboard, Lock, Flame, Trophy, Zap, Gift, ShoppingBag, Palette, ShieldCheck, CircleCheck, Sparkles, Swords, Scroll, Coins, Rocket, Orbit, Telescope, TreePine, Sprout, Leaf, X, Loader2, Volume2, VolumeX, Undo2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import clsx from 'clsx';
import confetti from 'canvas-confetti';
import { localDateStr } from '../utils';
import { getTimePeriod, groupChoresByTimePeriod, isTimePeriodActive, isTimePeriodPast } from '../timeGrouping';
import type { TimePeriod } from '../timeGrouping';
import { QRCodeSVG } from 'qrcode.react';
import { useThemeSound } from '../hooks/useThemeSound';
import { useTextToSpeech } from '../hooks/useTextToSpeech';

const QRCodeModal: React.FC<{
  chore: ScheduledChore;
  userId: number;
  baseUrl?: string;
  onClose: () => void;
  onComplete: () => void;
}> = ({ chore, userId, baseUrl, onClose, onComplete }) => {
  // Poll for completion status
  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const chores = await api.users.getChores(userId, 'daily', chore.date);
        const updated = chores.find(c => c.schedule_id === chore.schedule_id);
        if (updated?.completed) {
          onComplete();
        }
      } catch (e) {
        console.error('Polling failed:', e);
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [chore, userId, onComplete]);

  const origin = baseUrl || window.location.origin;
  const uploadUrl = `${origin}/upload?scheduleId=${chore.schedule_id}&date=${chore.date}&userId=${userId}`;

  return (
    <div className={styles.modalOverlay}>
      <div className={styles.qrModal}>
        <button className={styles.closeBtn} onClick={onClose}><X size={24} /></button>
        <h2>Photo Proof Needed</h2>
        <p>Scan this code with another iPad or phone to take a picture of your work!</p>
        
        <div className={styles.qrWrapper}>
          <QRCodeSVG value={uploadUrl} size={256} marginSize={4} />
        </div>

        <div className={styles.qrStatus}>
          <Loader2 className={styles.spinner} size={20} />
          <span>Waiting for photo...</span>
        </div>

        <p className={styles.qrHelp}>Your chore will be finished automatically once you upload the photo.</p>
      </div>
    </div>
  );
};

// Map icon string names from ThemeConfig to actual components
const CATEGORY_ICON_MAP: Record<string, React.FC<{ size?: number }>> = {
  'shield-check': ShieldCheck,
  'circle-check': CircleCheck,
  'sparkles': Sparkles,
  'swords': Swords,
  'scroll': Scroll,
  'coins': Coins,
  'rocket': Rocket,
  'orbit': Orbit,
  'telescope': Telescope,
  'tree-pine': TreePine,
  'sprout': Sprout,
  'leaf': Leaf,
};

export const Dashboard: React.FC = () => {
  const { user, setUser } = useAuth();
  const { theme, setTheme, config } = useTheme();
  const { playComplete, playAllDone } = useThemeSound();
  const { speak, stop } = useTextToSpeech();
  const prevProgressRef = useRef(0);
  const [ttsEnabled, setTtsEnabled] = useState(() => {
    const saved = localStorage.getItem(`openchore_tts_${user?.id}`);
    if (saved !== null) return saved === '1';
    return user?.age !== undefined && user.age <= 7;
  });
  const [view, setView] = useState<'daily' | 'weekly' | 'rewards'>('daily');
  const [chores, setChores] = useState<ScheduledChore[]>([]);
  const [streakData, setStreakData] = useState<UserStreakData | null>(null);
  const [pointsData, setPointsData] = useState<PointsData | null>(null);
  const [rewards, setRewards] = useState<Reward[]>([]);
  const [redemptions, setRedemptions] = useState<RedemptionHistory[]>([]);
  const [redeemingId, setRedeemingId] = useState<number | null>(null);
  const [redeemedId, setRedeemedId] = useState<number | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [showThemePicker, setShowThemePicker] = useState(false);
  const [showAvatarPicker, setShowAvatarPicker] = useState(false);
  const [qrChore, setQrChore] = useState<ScheduledChore | null>(null);
  const [systemBaseUrl, setSystemBaseUrl] = useState<string>('');
  const navigate = useNavigate();

  // Load system settings (only if user is authenticated)
  useEffect(() => {
    if (!user) return;
    api.admin.getSetting('base_url')
      .then(data => setSystemBaseUrl(data.value))
      .catch(() => {});
  }, [user]);

  const [date] = useState(localDateStr(new Date()));
  const todayStr = localDateStr(new Date());

  const loadChores = useCallback(async () => {
    if (user) {
      try {
        const data = await api.users.getChores(user.id, view === 'rewards' ? 'daily' : view, date);
        setChores(data);
      } catch (e) {
        console.error(e);
      }
    }
  }, [user, view, date]);

  const loadExtras = useCallback(async () => {
    if (!user) return;
    try {
      const [streak, points] = await Promise.all([
        api.streaks.getForUser(user.id),
        api.points.getForUser(user.id),
      ]);
      setStreakData(streak);
      setPointsData(points);
    } catch (e) {
      console.error(e);
    }
  }, [user]);

  const loadRewards = useCallback(async () => {
    if (!user) return;
    try {
      const [r, h] = await Promise.all([
        api.rewards.list(),
        api.rewards.listRedemptions(user.id),
      ]);
      setRewards(r);
      setRedemptions(h);
    } catch (e) {
      console.error(e);
    }
  }, [user]);

  useEffect(() => { loadChores(); }, [loadChores]);
  useEffect(() => { loadExtras(); }, [loadExtras]);
  useEffect(() => {
    if (view === 'rewards') loadRewards();
  }, [view, loadRewards]);

  const onChoreFinished = useCallback(async () => {
    setQrChore(null);
    confetti({
      particleCount: 150,
      spread: 70,
      origin: { y: 0.6 },
      colors: config.confettiColors,
    });
    playComplete();
    if (navigator.vibrate) navigator.vibrate(50);
    await loadChores();
    await loadExtras();
  }, [config.confettiColors, playComplete, loadChores, loadExtras]);

  const handleToggleComplete = async (chore: ScheduledChore) => {
    if (chore.date !== todayStr) return;
    try {
      if (chore.completed) {
        await api.chores.uncomplete(chore.schedule_id, chore.date);
      } else {
        if (chore.requires_photo) {
          setQrChore(chore);
          return;
        }
        await api.chores.complete(chore.schedule_id, chore.date);
        onChoreFinished();
        return; // onChoreFinished handles reload
      }
      await loadChores();
      await loadExtras();
    } catch (e) {
      console.error(e);
    }
  };

  const showToast = (message: string) => {
    setToast(message);
    setTimeout(() => setToast(null), 3000);
  };

  const handleRedeem = async (reward: Reward) => {
    if (!pointsData || pointsData.balance < reward.effective_cost) return;
    setRedeemingId(reward.id);
    try {
      await api.rewards.redeem(reward.id);
      // Optimistically update balance so it feels instant
      setPointsData(prev => prev ? { ...prev, balance: prev.balance - reward.effective_cost } : prev);
      setRedeemingId(null);
      setRedeemedId(reward.id);
      confetti({
        particleCount: 100,
        spread: 60,
        origin: { y: 0.5 },
        colors: config.confettiColors,
      });
      playComplete();
      if (navigator.vibrate) navigator.vibrate(50);
      showToast(`${reward.icon || '🎁'} ${reward.name} redeemed!`);
      // Refresh real data in background
      await Promise.all([loadExtras(), loadRewards()]);
      setTimeout(() => setRedeemedId(null), 2000);
    } catch (e) {
      console.error('Redeem error:', e);
      setRedeemingId(null);
      showToast('Redemption failed — try again');
    }
  };

  const logout = () => {
    setUser(null);
    navigate('/login');
  };

  const getGreeting = () => {
    const hour = new Date().getHours();
    if (hour < 12) return config.greetings.morning;
    if (hour < 17) return config.greetings.afternoon;
    return config.greetings.evening;
  };

  const getFormattedDate = () => {
    const d = new Date();
    return d.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });
  };

  const formatDateLabel = (dateStr: string) => {
    const d = new Date(dateStr + 'T00:00:00');
    const dayShort = d.toLocaleDateString('en-US', { weekday: 'short' });
    const dayNum = d.getDate();
    const isToday = dateStr === todayStr;
    return { dayShort, dayNum, isToday };
  };

  // --- Points & Progress ---
  const dailyChores = chores.filter(c => c.date === todayStr);
  const availableRequiredChores = dailyChores.filter(c => c.category === 'required' && c.available);
  const allRequiredDone = availableRequiredChores.every(c => c.completed);

  const completedCount = dailyChores.filter(c => c.completed).length;
  const availableCount = dailyChores.filter(c => c.available || c.completed).length;
  const progressPercent = availableCount > 0 ? Math.round((completedCount / availableCount) * 100) : 0;

  // Play allDone sound when progress reaches 100%
  useEffect(() => {
    if (progressPercent === 100 && prevProgressRef.current < 100 && prevProgressRef.current > 0) {
      playAllDone();
    }
    prevProgressRef.current = progressPercent;
  }, [progressPercent, playAllDone]);

  const calculatePoints = (choresList: ScheduledChore[]) => {
    const daily = choresList.filter(c => c.date === todayStr);
    const availReq = daily.filter(c => c.category === 'required' && c.available);
    const allReqDone = availReq.every(c => c.completed);

    let earned = 0;
    let pending = 0;

    daily.forEach(c => {
      if (!c.completed) return;
      const pts = c.points_value || 0;
      if (c.category === 'bonus' || c.category === 'required') {
        earned += pts;
      } else if (c.category === 'core') {
        if (allReqDone) earned += pts;
        else pending += pts;
      }
    });

    const possible = daily.reduce((sum, c) => sum + (c.points_value || 0), 0);
    return { earned, pending, possible };
  };

  const { earned, pending, possible } = calculatePoints(chores);

  const getCountdown = (availableAt: string) => {
    const now = new Date();
    const [hrs, mins] = availableAt.split(':').map(Number);
    const target = new Date();
    target.setHours(hrs, mins, 0, 0);
    const diff = target.getTime() - now.getTime();
    if (diff <= 0) return null;
    const h = Math.floor(diff / (1000 * 60 * 60));
    const m = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  };

  // --- Grouping ---
  const groupChoresByDate = () => {
    const groups: { [date: string]: ScheduledChore[] } = {};
    const d = new Date(date + 'T00:00:00');
    const day = d.getDay();
    const diff = d.getDate() - day + (day === 0 ? -6 : 1);
    const monday = new Date(d.setDate(diff));
    for (let i = 0; i < 7; i++) {
      const current = new Date(monday);
      current.setDate(monday.getDate() + i);
      const dStr = localDateStr(current);
      groups[dStr] = [];
    }
    chores.forEach(chore => {
      if (groups[chore.date]) groups[chore.date].push(chore);
    });
    return groups;
  };

  const groupChoresByCategory = (list: ScheduledChore[]) => {
    const order = ['required', 'core', 'bonus'] as const;
    return order
      .map(cat => ({ category: cat, label: config.labels[cat], chores: list.filter(c => c.category === cat) }))
      .filter(g => g.chores.length > 0);
  };

  // Time period grouping functions imported from '../timeGrouping'

  // --- Swipe-to-complete/uncomplete ---
  const swipeRef = useRef<{ startX: number; startY: number; cardEl: HTMLDivElement | null; choreKey: string; locked: boolean }>({ startX: 0, startY: 0, cardEl: null, choreKey: '', locked: false });

  const handleTouchStart = (e: ReactTouchEvent<HTMLDivElement>, choreKey: string) => {
    const touch = e.touches[0];
    swipeRef.current = { startX: touch.clientX, startY: touch.clientY, cardEl: e.currentTarget, choreKey, locked: false };
  };

  const handleTouchMove = (e: ReactTouchEvent<HTMLDivElement>, chore: ScheduledChore) => {
    const ref = swipeRef.current;
    if (!ref.cardEl) return;
    const dx = e.touches[0].clientX - ref.startX;
    const dy = e.touches[0].clientY - ref.startY;
    // Lock to vertical scrolling if that's the dominant direction on first significant move
    if (!ref.locked && Math.abs(dy) > Math.abs(dx) && Math.abs(dy) > 10) {
      ref.cardEl = null;
      return;
    }
    ref.locked = true;
    // Right swipe to complete, left swipe to uncomplete
    const direction = chore.completed ? -1 : 1;
    const raw = dx * direction;
    const clamped = Math.max(0, Math.min(raw, 120));
    if (clamped > 10) {
      const progress = Math.min(clamped / 100, 1);
      ref.cardEl.style.transform = `translateX(${dx > 0 ? clamped : -clamped}px)`;
      const hintEl = ref.cardEl.parentElement?.querySelector(`.${styles.swipeHint}`) as HTMLElement | null;
      if (hintEl) hintEl.style.opacity = String(progress);
      ref.cardEl.style.background = chore.completed
        ? `rgba(239, 68, 68, ${progress * 0.15})`
        : `rgba(74, 222, 128, ${progress * 0.15})`;
    }
  };

  const handleSwipeEnd = (e: ReactTouchEvent<HTMLDivElement>, chore: ScheduledChore) => {
    const { startX, cardEl } = swipeRef.current;
    if (!cardEl) return;
    const dx = e.changedTouches[0].clientX - startX;
    cardEl.style.transform = '';
    cardEl.style.background = '';
    const hintEl = cardEl.parentElement?.querySelector(`.${styles.swipeHint}`) as HTMLElement | null;
    if (hintEl) hintEl.style.opacity = '';
    swipeRef.current = { startX: 0, startY: 0, cardEl: null, choreKey: '', locked: false };
    const direction = chore.completed ? -1 : 1;
    if (dx * direction >= 100) {
      handleToggleComplete(chore);
    }
  };

  // --- Renders ---
  const renderChoreCard = (chore: ScheduledChore, isWeekly = false) => {
    const isToday = chore.date === todayStr;
    const isLocked = !chore.available && !chore.completed;
    const isExpired = chore.expired && !chore.completed;
    const isPointsLocked = isToday && chore.completed && chore.category === 'core' && !allRequiredDone;

    if (isWeekly) {
      return (
        <div
          key={`${chore.schedule_id}-${chore.date}`}
          className={clsx(
            styles.calendarChoreItem,
            chore.completed && styles.calendarChoreCompleted,
            !isToday && styles.calendarChorePast,
            isLocked && styles.calendarChoreLocked,
            isPointsLocked && styles.calendarChorePointsLocked
          )}
          onClick={() => isToday && !isLocked && handleToggleComplete(chore)}
        >
          <div className={clsx(styles.calendarStatus, chore.completed && styles.calendarStatusCompleted)}>
            {chore.completed ? <CheckCircle size={14} /> : (isLocked ? <Lock size={10} /> : null)}
          </div>
          <div className={styles.calendarChoreContent}>
            <span className={styles.calendarChoreTitle}>{chore.title}</span>
            {isLocked && chore.available_at && (
              <span className={styles.calendarCountdown}>{chore.available_at}</span>
            )}
          </div>
          <div className={styles.calendarChorePoints}>
            {isPointsLocked ? (
              <span className={styles.pointsLocked}><Lock size={8} /> {chore.points_value}</span>
            ) : (
              <span className={styles.pointsValue}><Star size={8} /> {chore.points_value}</span>
            )}
          </div>
        </div>
      );
    }

    const canSwipeComplete = isToday && !chore.completed && chore.available && !(isExpired && chore.expiry_penalty === 'block');
    const canSwipeUndo = isToday && chore.completed;
    const canSwipe = canSwipeComplete || canSwipeUndo;
    const choreKey = `${chore.schedule_id}-${chore.date}`;

    return (
      <div key={choreKey} className={clsx(styles.choreCardWrap, canSwipe && styles.choreCardSwipeable)}>
        {canSwipe && (
          <div className={clsx(styles.swipeHint, canSwipeUndo ? styles.swipeHintUndo : styles.swipeHintDone)} style={canSwipeUndo ? { right: '1.25rem', left: 'auto' } : undefined}>
            {canSwipeUndo ? <><Undo2 size={20} /> Undo</> : <><CheckCircle size={20} /> Done!</>}
          </div>
        )}
        <div
          className={clsx(
            styles.choreCard,
            chore.completed && styles.choreCardCompleted,
            isLocked && styles.choreCardLocked,
            isExpired && styles.choreCardExpired,
            isPointsLocked && styles.choreCardPointsLocked
          )}
          onTouchStart={canSwipe ? (e) => handleTouchStart(e, choreKey) : undefined}
          onTouchMove={canSwipe ? (e) => handleTouchMove(e, chore) : undefined}
          onTouchEnd={canSwipe ? (e) => handleSwipeEnd(e, chore) : undefined}
        >
        <div className={styles.choreInfo}>
          <h3 className={styles.choreTitle}>
            {chore.icon && <span className={styles.choreIcon}>{chore.icon}</span>}
            {chore.title}
            {isLocked && <Lock size={14} className={styles.titleLockIcon} />}
            {ttsEnabled && (
              <button
                className={styles.ttsBtn}
                onClick={(e) => { e.stopPropagation(); speak(chore.title + (chore.description ? '. ' + chore.description : '')); }}
                aria-label={`Read ${chore.title} aloud`}
              >
                <Volume2 size={16} />
              </button>
            )}
          </h3>
          {chore.description && (
            <p className={styles.choreDescription}>{chore.description}</p>
          )}
          <div className={styles.choreMeta}>
            <span className={styles.metaItem}><Clock size={14} /> {chore.estimated_minutes || 5}m</span>
            <span className={clsx(styles.metaItem, isPointsLocked && styles.pointsLocked)}>
              {isPointsLocked ? (
                <><Lock size={12} /> {chore.points_value} pts pending</>
              ) : (
                <><Star size={14} /> {chore.points_value} pts</>
              )}
            </span>
            {isExpired && chore.due_by && (
              <span className={styles.expiredBadge}>
                {chore.expiry_penalty === 'block'
                  ? `Expired at ${chore.due_by}`
                  : chore.expiry_penalty === 'no_points'
                  ? `Late (0 pts)`
                  : `Late (-${chore.expiry_penalty_value} pts)`}
              </span>
            )}
          </div>
        </div>

        <div className={styles.actionArea}>
          {isLocked ? (
            <div className={styles.countdownBox}>
              <Lock size={14} className={styles.countdownIcon} />
              <span className={styles.countdownTime}>{getCountdown(chore.available_at || "")}</span>
            </div>
          ) : isExpired && chore.expiry_penalty === 'block' ? (
            <div className={styles.countdownBox}>
              <X size={14} className={styles.countdownIcon} />
              <span className={styles.countdownTime}>Expired</span>
            </div>
          ) : (
            <button
              onClick={() => handleToggleComplete(chore)}
              className={clsx(styles.completeBtn, chore.completed && styles.completeBtnActive)}
              aria-label={chore.completed ? "Mark incomplete" : "Mark complete"}
            >
              {chore.completed ? <CheckCircle size={32} /> : <div className={styles.circle} />}
            </button>
          )}
        </div>
      </div>
      </div>
    );
  };

  const renderDailyView = () => {
    const allChores = chores;
    const timePeriods = groupChoresByTimePeriod(allChores);

    return (
      <div className={styles.choreGrid}>
        {timePeriods.map(period => {
          const active = isTimePeriodActive(period.startHour, period.nextStartHour);
          const past = isTimePeriodPast(period.nextStartHour);
          const future = !active && !past;
          const available = period.chores.filter(c => c.available || c.completed);
          const upcoming = period.chores.filter(c => !c.available && !c.completed);
          const categoryGroups = groupChoresByCategory(available);
          const completedInPeriod = period.chores.filter(c => c.completed).length;
          const totalInPeriod = period.chores.length;

          return (
            <div
              key={period.key}
              className={clsx(
                styles.timePeriodSection,
                active && styles.timePeriodActive,
                future && styles.timePeriodFuture,
                past && styles.timePeriodPast
              )}
            >
              <div className={styles.timePeriodHeader}>
                <span className={styles.timePeriodEmoji}>{period.emoji}</span>
                <span className={styles.timePeriodLabel}>{period.label}</span>
                <span className={styles.timePeriodCount}>
                  {completedInPeriod}/{totalInPeriod}
                </span>
              </div>

              <div className={styles.timePeriodBody}>
                {categoryGroups.map(group => (
                  <div key={group.category} className={styles.categoryGroup}>
                    <div className={styles.categoryHeader}>
                      {(() => {
                        const IconComp = CATEGORY_ICON_MAP[config.categoryIcons[group.category]];
                        return IconComp
                          ? <span className={styles[`catIcon_${group.category}`]}><IconComp size={14} /></span>
                          : <span className={clsx(styles.categoryDot, styles[`dot_${group.category}`])} />;
                      })()}
                      <span className={styles.categoryLabel}>{group.label}</span>
                      <span className={styles.categoryCount}>
                        {group.chores.filter(c => c.completed).length}/{group.chores.length}
                      </span>
                    </div>
                    {group.chores.map(chore => renderChoreCard(chore))}
                  </div>
                ))}

                {upcoming.length > 0 && (
                  <div className={styles.categoryGroup}>
                    <div className={styles.sectionDivider}>
                      <span className={styles.dividerLine} />
                      <Clock size={14} className={styles.dividerIcon} />
                      <span className={styles.dividerText}>Coming up later</span>
                      <span className={styles.dividerLine} />
                    </div>
                    {upcoming.map(chore => renderChoreCard(chore))}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  const renderWeeklyView = () => {
    const groups = groupChoresByDate();
    const sortedDates = Object.keys(groups).sort();

    return (
      <div className={styles.calendarGrid}>
        {sortedDates.map(dateStr => {
          const { dayShort, dayNum, isToday } = formatDateLabel(dateStr);
          const dayChores = groups[dateStr];
          const done = dayChores.filter(c => c.completed).length;
          const total = dayChores.length;
          return (
            <div key={dateStr} className={clsx(styles.calendarColumn, isToday && styles.calendarColumnToday)}>
              <div className={styles.calendarHeader}>
                <span className={styles.calendarDayName}>{dayShort}</span>
                <span className={styles.calendarDayNum}>{dayNum}</span>
                {total > 0 && (
                  <span className={clsx(styles.calendarProgress, done === total && styles.calendarProgressDone)}>
                    {done}/{total}
                  </span>
                )}
              </div>
              <div className={styles.calendarChores}>
                {dayChores.map(chore => renderChoreCard(chore, true))}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  const renderRewardsView = () => {
    const balance = pointsData?.balance ?? 0;

    return (
      <div className={styles.rewardsView}>
        <div className={styles.rewardsBalance}>
          <Star size={20} className={styles.rewardsBalanceIcon} />
          <span className={styles.rewardsBalanceAmount}>{balance}</span>
          <span className={styles.rewardsBalanceLabel}>points available</span>
        </div>

        {rewards.length === 0 ? (
          <div className={styles.empty}>
            <Gift size={48} className={styles.emptyIcon} />
            <h3>No rewards yet</h3>
            <p>Ask a parent to add some rewards!</p>
          </div>
        ) : (
          <div className={styles.rewardsGrid}>
            {rewards.map(reward => {
              const canAfford = balance >= reward.effective_cost;
              const outOfStock = reward.stock !== null && reward.stock !== undefined && reward.stock <= 0;
              const isRedeeming = redeemingId === reward.id;

              return (
                <div key={reward.id} className={clsx(styles.rewardCard, !canAfford && styles.rewardCardLocked)}>
                  {reward.icon && <div className={styles.rewardIcon}>{reward.icon}</div>}
                  <div className={styles.rewardInfo}>
                    <h3 className={styles.rewardName}>{reward.name}</h3>
                    {reward.description && (
                      <p className={styles.rewardDesc}>{reward.description}</p>
                    )}
                    <div className={styles.rewardMeta}>
                      <span className={styles.rewardCost}>
                        <Star size={12} /> {reward.effective_cost} pts
                      </span>
                      {reward.stock !== null && reward.stock !== undefined && (
                        <span className={styles.rewardStock}>
                          {reward.stock} left
                        </span>
                      )}
                    </div>
                  </div>
                  <button
                    className={clsx(
                      styles.redeemBtn,
                      canAfford && !outOfStock && styles.redeemBtnActive,
                      redeemedId === reward.id && styles.redeemBtnSuccess
                    )}
                    disabled={!canAfford || outOfStock || isRedeeming || redeemedId === reward.id}
                    onClick={() => handleRedeem(reward)}
                  >
                    {redeemedId === reward.id ? <><CheckCircle size={14} /> Redeemed!</> : isRedeeming ? '...' : outOfStock ? 'Gone' : canAfford ? 'Redeem' : `Need ${reward.effective_cost - balance}`}
                  </button>
                </div>
              );
            })}
          </div>
        )}

        {redemptions.length > 0 && (
          <div className={styles.redemptionHistory}>
            <h3 className={styles.redemptionTitle}>
              <ShoppingBag size={16} /> Recent Redemptions
            </h3>
            <div className={styles.redemptionList}>
              {redemptions.map(r => (
                <div key={r.id} className={styles.redemptionItem}>
                  {r.reward_icon && <span className={styles.redemptionIcon}>{r.reward_icon}</span>}
                  <span className={styles.redemptionName}>{r.reward_name}</span>
                  <span className={styles.redemptionCost}>-{r.points_spent} pts</span>
                  <span className={styles.redemptionDate}>
                    {new Date(r.created_at).toLocaleDateString()}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    );
  };

  const renderProgressBar = () => {
    const allDone = completedCount === availableCount && availableCount > 0;
    return (
      <div className={styles.progressSection}>
        <div className={styles.progressHeader}>
          <div className={styles.progressLabel}>
            {allDone ? (
              <><Trophy size={16} className={styles.trophyIcon} /> {config.messages.allDone}</>
            ) : (
              <>{completedCount} of {availableCount} complete</>
            )}
          </div>
          <span className={styles.progressPercent}>{progressPercent}%</span>
        </div>
        <div className={styles.progressTrack}>
          <div
            className={clsx(styles.progressFill, allDone && styles.progressFillDone)}
            style={{ width: `${progressPercent}%` }}
          />
        </div>
      </div>
    );
  };

  const renderPointsSummary = () => (
    <div className={styles.statsRow}>
      {/* Total Balance */}
      <div className={clsx(styles.statCard, styles.statCardBalance)}>
        <Star size={18} className={styles.statIconBalance} />
        <div className={styles.statInfo}>
          <span className={styles.statValue}>{pointsData?.balance ?? 0}</span>
          <span className={styles.statLabel}>Balance</span>
        </div>
      </div>

      {/* Streak */}
      {streakData && streakData.current_streak > 0 && (
        <div className={clsx(styles.statCard, styles.statCardStreak)}>
          <Flame size={18} className={styles.statIconStreak} />
          <div className={styles.statInfo}>
            <span className={styles.statValue}>{streakData.current_streak}d</span>
            <span className={styles.statLabel}>{config.messages.streakLabel}</span>
          </div>
        </div>
      )}

      {/* Today's earned */}
      <div className={styles.statCard}>
        <Zap size={18} className={styles.statIconEarned} />
        <div className={styles.statInfo}>
          <span className={styles.statValue}>{earned}</span>
          <span className={styles.statLabel}>Today</span>
        </div>
      </div>

      {pending > 0 && (
        <div className={clsx(styles.statCard, styles.statCardPending)}>
          <Lock size={18} className={styles.statIconPending} />
          <div className={styles.statInfo}>
            <span className={styles.statValue}>{pending}</span>
            <span className={styles.statLabel}>Pending</span>
          </div>
        </div>
      )}
    </div>
  );

  // Streak milestone banner
  const renderStreakBanner = () => {
    if (!streakData || !streakData.next_reward) return null;
    const { next_reward } = streakData;
    return (
      <div className={styles.streakBanner}>
        <Flame size={16} className={styles.streakBannerIcon} />
        <span className={styles.streakBannerText}>
          {next_reward.days_remaining} day{next_reward.days_remaining !== 1 ? 's' : ''} to <strong>{next_reward.label}</strong> (+{next_reward.bonus_points} pts)
        </span>
      </div>
    );
  };

  return (
    <div className={clsx(styles.wrapper, view === 'weekly' && styles.wrapperWide)}>
      {toast && (
        <div className={styles.toast}>
          {toast}
        </div>
      )}
      <header className={styles.header}>
        <div className={styles.userProfile}>
          <button className={styles.avatarMini} onClick={() => { setShowAvatarPicker(!showAvatarPicker); setShowThemePicker(false); }} aria-label="Change avatar">
            {user?.avatar_url ? <img src={user.avatar_url} alt={user.name} /> : <div className={styles.avatarPlaceholder} />}
          </button>
          <div>
            <h2 className={styles.greeting}>{getGreeting()}, {user?.name}!</h2>
            <p className={styles.dateText}>{getFormattedDate()}</p>
          </div>
        </div>
        <div className={styles.headerActions}>
          <button
            onClick={() => {
              const next = !ttsEnabled;
              setTtsEnabled(next);
              localStorage.setItem(`openchore_tts_${user?.id}`, next ? '1' : '0');
              if (!next) stop();
            }}
            className={clsx(styles.ttsToggle, ttsEnabled && styles.ttsToggleActive)}
            aria-label={ttsEnabled ? 'Disable read aloud' : 'Enable read aloud'}
          >
            {ttsEnabled ? <Volume2 size={20} /> : <VolumeX size={20} />}
          </button>
          <button onClick={() => { setShowThemePicker(!showThemePicker); setShowAvatarPicker(false); }} className={styles.themeBtn} aria-label="Change theme">
            <Palette size={20} />
          </button>
          <button onClick={logout} className={styles.logoutBtn} aria-label="Logout">
            <LogOut size={20} />
          </button>
        </div>
      </header>

      {showThemePicker && (
        <ThemePicker
          current={theme}
          onSelect={(t) => { setTheme(t); setShowThemePicker(false); }}
        />
      )}

      {showAvatarPicker && user && (
        <AvatarPicker
          currentUrl={user.avatar_url}
          userName={user.name}
          onSelect={async (url) => {
            try {
              const updated = await api.users.updateAvatar(user.id, url);
              setUser({ ...user, ...updated });
            } catch (e) {
              console.error('Failed to update avatar:', e);
            }
            setShowAvatarPicker(false);
          }}
          onClose={() => setShowAvatarPicker(false)}
        />
      )}

      {qrChore && user && (
        <QRCodeModal
          chore={qrChore}
          userId={user.id}
          baseUrl={systemBaseUrl}
          onClose={() => setQrChore(null)}
          onComplete={onChoreFinished}
        />
      )}

      {user?.paused ? (
        <>
          <div className={styles.pausedBanner}>
            <div className={styles.pausedIcon}>🏖️</div>
            <h2 className={styles.pausedTitle}>You're on a break!</h2>
            <p className={styles.pausedText}>Enjoy your time off. Your chores are paused and no points will be deducted while you're away.</p>
          </div>

          <nav className={styles.nav}>
            <button
              className={clsx(styles.navItem, view === 'rewards' && styles.navItemActive)}
              onClick={() => setView('rewards')}
            >
              <ShoppingBag size={18} />
              Rewards
            </button>
          </nav>

          {view === 'rewards' && (
            <main className={styles.content}>
              {renderRewardsView()}
            </main>
          )}
        </>
      ) : (
        <>
          {view === 'daily' && renderProgressBar()}
          {view === 'daily' && renderPointsSummary()}
          {view === 'daily' && renderStreakBanner()}

          <nav className={styles.nav}>
            <button
              className={clsx(styles.navItem, view === 'daily' && styles.navItemActive)}
              onClick={() => setView('daily')}
            >
              <LayoutDashboard size={18} />
              Today
            </button>
            <button
              className={clsx(styles.navItem, view === 'weekly' && styles.navItemActive)}
              onClick={() => setView('weekly')}
            >
              <Calendar size={18} />
              Week
            </button>
            <button
              className={clsx(styles.navItem, view === 'rewards' && styles.navItemActive)}
              onClick={() => setView('rewards')}
            >
              <ShoppingBag size={18} />
              Rewards
            </button>
          </nav>

          <main className={styles.content}>
            {view === 'rewards' ? (
              renderRewardsView()
            ) : chores.length === 0 ? (
              <div className={styles.empty}>
                <Trophy size={48} className={styles.emptyIcon} />
                <h3>{config.messages.allDone}</h3>
                <p>{config.messages.empty}</p>
              </div>
            ) : (
              view === 'daily' ? renderDailyView() : renderWeeklyView()
            )}
          </main>
        </>
      )}
    </div>
  );
};

const THEME_OPTIONS: { id: Theme; name: string; icon: string; preview: string }[] = [
  { id: 'default', name: 'Classic', icon: '🌊', preview: '#38bdf8' },
  { id: 'quest', name: 'Quest', icon: '⚔️', preview: '#fbbf24' },
  { id: 'galaxy', name: 'Galaxy', icon: '🚀', preview: '#a855f7' },
  { id: 'forest', name: 'Forest', icon: '🌲', preview: '#4ade80' },
];

const ThemePicker: React.FC<{ current: Theme; onSelect: (t: Theme) => void }> = ({ current, onSelect }) => (
  <div className={styles.themePicker}>
    {THEME_OPTIONS.map(t => (
      <button
        key={t.id}
        className={clsx(styles.themeOption, current === t.id && styles.themeOptionActive)}
        onClick={() => onSelect(t.id)}
      >
        <div className={styles.themePreview} style={{ backgroundColor: t.preview }}>
          <span className={styles.themeEmoji}>{t.icon}</span>
        </div>
        <span className={styles.themeName}>{t.name}</span>
      </button>
    ))}
  </div>
);

const AVATAR_STYLES = [
  { id: 'avataaars-neutral', label: 'Classic' },
  { id: 'adventurer-neutral', label: 'Adventure' },
  { id: 'big-ears-neutral', label: 'Big Ears' },
  { id: 'bottts-neutral', label: 'Robots' },
  { id: 'fun-emoji', label: 'Emoji' },
  { id: 'lorelei-neutral', label: 'Lorelei' },
  { id: 'croodles-neutral', label: 'Doodle' },
  { id: 'pixel-art-neutral', label: 'Pixel' },
  { id: 'thumbs', label: 'Thumbs' },
  { id: 'notionists-neutral', label: 'Sketch' },
  { id: 'shapes', label: 'Shapes' },
  { id: 'glass', label: 'Glass' },
];

const avatarUrl = (style: string, seed: string) =>
  `https://api.dicebear.com/9.x/${style}/svg?seed=${encodeURIComponent(seed)}`;

const AvatarPicker: React.FC<{
  currentUrl: string;
  userName: string;
  onSelect: (url: string) => void;
  onClose: () => void;
}> = ({ currentUrl, userName, onSelect, onClose }) => {
  const [selectedStyle, setSelectedStyle] = useState(
    (() => {
      const match = currentUrl?.match(/dicebear\.com\/\d+\.x\/([^/]+)\//);
      return match?.[1] || 'avataaars';
    })()
  );
  const [shuffleKey, setShuffleKey] = useState('');

  const seed = shuffleKey || userName;
  const previewUrl = avatarUrl(selectedStyle, seed);

  const randomize = () => {
    setShuffleKey(`${userName}-${Math.random().toString(36).slice(2, 8)}`);
  };

  return (
    <div className={styles.avatarPicker}>
      <div className={styles.avatarPickerHeader}>
        <h3 className={styles.avatarPickerTitle}>Choose Your Look</h3>
        <button className={styles.avatarPickerClose} onClick={onClose}>
          <span>✕</span>
        </button>
      </div>

      <div className={styles.avatarPickerPreview}>
        <img src={previewUrl} alt="Avatar preview" className={styles.avatarPickerImg} />
      </div>

      <div className={styles.avatarStyleGrid}>
        {AVATAR_STYLES.map(s => (
          <button
            key={s.id}
            className={clsx(styles.avatarStyleBtn, selectedStyle === s.id && styles.avatarStyleBtnActive)}
            onClick={() => setSelectedStyle(s.id)}
          >
            <img src={avatarUrl(s.id, seed)} alt={s.label} className={styles.avatarStylePreview} />
            <span className={styles.avatarStyleLabel}>{s.label}</span>
          </button>
        ))}
      </div>

      <div className={styles.avatarPickerActions}>
        <button className={styles.avatarRandomBtn} onClick={randomize}>
          Shuffle
        </button>
        <button
          className={styles.avatarSaveBtn}
          onClick={() => onSelect(previewUrl)}
          disabled={previewUrl === currentUrl}
        >
          Save
        </button>
      </div>
    </div>
  );
};
