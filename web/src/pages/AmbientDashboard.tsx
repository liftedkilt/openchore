import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, fetchPublicUserData } from '../api';
import type { User, ScheduledChore, UserStreakData, PointsData } from '../types';
import { localDateStr } from '../utils';
import {
  Avatar, ChoreRow, DayProgress, FamilyMember, HouseScope, Icon, SkinScope,
  catFromCategory, resolveSkin, salutationFor, useMinuteClock,
  type Cat, type ChoreState, type DayProgressItem, type Skin,
} from '../design';
import { LineChart, type LineSeries, type Tick } from '../components/charts/LineChart';
import { personColorVar } from '../components/charts/personColor';
import styles from './AmbientDashboard.module.css';

interface PersonDay {
  user: User;
  chores: ScheduledChore[];
  completed: number;
  total: number;
  pointsToday: number;
  balance: number;
  streak: number;
  /** Completion times, oldest first. */
  doneAt: Date[];
}

const REFRESH_MS = 45_000;
/** How many of a person's next chores their door lists (more are clipped if they don't fit). */
const NEXT_ROWS = 3;
/** DayProgress' day, matching its default "7 am" / "9 pm" labels. */
const DAY_FROM_H = 7;
const DAY_TO_H = 21;
/** The family chart's day. */
const CHART_FROM_H = 6;
const CHART_TO_H = 22;
const CAT_ORDER: Record<Cat, number> = { essential: 0, daily: 1, bonus: 2 };

/**
 * Door hero sizing is layout, not theme (the gallery does the same): each
 * skin draws a different hero, so each gets the width and scale that fits a
 * door. The door's container query scales it up when doors are wide.
 */
const HERO_FIT: Record<Skin, { w: number; s: number }> = {
  sunroom: { w: 312, s: 0.62 },
  blocks: { w: 236, s: 0.82 },
  tint: { w: 312, s: 0.52 },
};

const hoursOf = (d: Date) => d.getHours() + d.getMinutes() / 60;
const dayFraction = (d: Date) => (hoursOf(d) - DAY_FROM_H) / (DAY_TO_H - DAY_FROM_H);

/** "HH:MM" today, as a Date. */
function atToday(hhmm: string | undefined, now: Date): Date | null {
  if (!hhmm) return null;
  const [h, m] = hhmm.split(':').map(Number);
  if (!Number.isFinite(h) || !Number.isFinite(m)) return null;
  const d = new Date(now);
  d.setHours(h, m, 0, 0);
  return d;
}

/**
 * Hide the children of a box that don't fully fit inside it, so a door shows
 * as many whole rows as its height allows and never a clipped one.
 */
function useFitRows<T extends HTMLElement>(deps: unknown[]) {
  const ref = useRef<T>(null);
  useLayoutEffect(() => {
    const box = ref.current;
    if (!box) return;
    const fit = () => {
      const kids = Array.from(box.children) as HTMLElement[];
      kids.forEach((k) => { k.hidden = false; });
      const bottom = box.getBoundingClientRect().bottom;
      kids.forEach((k, i) => { k.hidden = i > 0 && k.getBoundingClientRect().bottom > bottom + 1; });
    };
    fit();
    if (typeof ResizeObserver === 'undefined') return;
    const ro = new ResizeObserver(fit);
    ro.observe(box);
    return () => ro.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return ref;
}

async function loadPerson(user: User, today: string): Promise<PersonDay> {
  const [chores, streakData, pointsData] = await Promise.all([
    fetchPublicUserData<ScheduledChore[]>(`/users/${user.id}/chores?view=daily&date=${today}`),
    fetchPublicUserData<UserStreakData>(`/users/${user.id}/streak`),
    fetchPublicUserData<PointsData>(`/users/${user.id}/points`),
  ]);
  const done = chores.filter((c) => c.completed);
  const doneAt = done
    .map((c) => (c.completed_at ? new Date(c.completed_at) : null))
    .filter((d): d is Date => !!d && !isNaN(d.getTime()))
    .sort((a, b) => a.getTime() - b.getTime());
  return {
    user,
    chores,
    completed: done.length,
    total: chores.length,
    pointsToday: done.reduce((sum, c) => sum + c.points_value, 0),
    balance: pointsData.balance,
    streak: streakData.current_streak,
    doneAt,
  };
}

export const AmbientDashboard: React.FC = () => {
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const [people, setPeople] = useState<PersonDay[]>([]);
  const [loading, setLoading] = useState(true);
  const now = useMinuteClock();

  const fetchData = useCallback(async () => {
    try {
      const users: User[] = await api.users.list();
      const today = localDateStr(new Date());
      const results = await Promise.allSettled(users.filter((u) => !u.paused).map((u) => loadPerson(u, today)));
      setPeople(
        results
          .filter((r): r is PromiseFulfilledResult<PersonDay> => r.status === 'fulfilled')
          .map((r) => r.value)
          // Everyone takes part: kids always show; grown-ups on days they have chores.
          .filter((p) => p.user.role === 'child' || p.total > 0)
          // The family's order, never a ranking.
          .sort((a, b) => a.user.id - b.user.id),
      );
    } catch (err) {
      console.error('Ambient fetch error:', err);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, REFRESH_MS);
    return () => clearInterval(interval);
  }, [fetchData]);

  // Keep the wall display awake.
  useEffect(() => {
    let wakeLock: WakeLockSentinel | null = null;
    const request = async () => {
      try { wakeLock = await navigator.wakeLock.request('screen'); } catch { /* unsupported */ }
    };
    request();
    const onVisibility = () => { if (document.visibilityState === 'visible') request(); };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      wakeLock?.release();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, []);

  const lang = i18n.language;
  const fmtTime = useCallback(
    (d: Date) => d.toLocaleTimeString(lang, { hour: 'numeric', minute: '2-digit' }),
    [lang],
  );
  const start = () => navigate('/login');

  return (
    <HouseScope mode="auto" persistent={false} className={styles.wall} onClick={start}>
      {!loading && (
        <>
          <header className={styles.hello}>
            <div className={styles.helloText}>
              <div className={styles.top}>
                <span className={styles.brand}>
                  <span className={styles.logo} aria-hidden>
                    {(['coral', 'mint', 'butter', 'sky'] as const).map((c) => <i key={c} style={{ background: `var(--person-${c})` }} />)}
                  </span>
                  openchore
                </span>
                <span className={styles.date}>
                  {now.toLocaleDateString(lang, { weekday: 'long', month: 'long', day: 'numeric' })}
                </span>
              </div>
              <h1 className={styles.greeting}>
                {t(`wall.greeting.${salutationFor(now)}`)}{' '}
                <button type="button" className={styles.startBtn} onClick={(e) => { e.stopPropagation(); start(); }}>
                  {t('wall.tapToStart')}
                </button>
              </h1>
            </div>
            <time className={styles.clock} dateTime={now.toISOString()} data-testid="wall-clock">
              {fmtTime(now)}
            </time>
          </header>

          <div className={styles.doors}>
            {people.map((p) => <Door key={p.user.id} p={p} now={now} fmtTime={fmtTime} />)}
          </div>

          {people.length > 0 && (
            <section className={styles.family} aria-labelledby="wall-family">
              <div className={styles.members}>
                <h2 id="wall-family" className={styles.label}>{t('wall.familyToday')}</h2>
                <div className={styles.memberGrid}>
                  {people.map((p) => (
                    <FamilyMember key={p.user.id} name={p.user.name} color={p.user.color} done={p.completed} total={p.total} />
                  ))}
                </div>
              </div>
              <DayChart people={people} now={now} />
            </section>
          )}
        </>
      )}
    </HouseScope>
  );
};

/* ---------------- A person's door ---------------- */

function Door({ p, now, fmtTime }: { p: PersonDay; now: Date; fmtTime: (d: Date) => string }) {
  const { t } = useTranslation();
  const skin = resolveSkin(p.user.theme, p.user.age);
  const left = p.total - p.completed;
  const nameId = `wall-door-${p.user.id}`;

  // One item per chore, finished ones pinned where they happened on the arc.
  const items: DayProgressItem[] = useMemo(() => {
    const times = [...p.doneAt];
    return [...p.chores]
      .sort((a, b) => Number(b.completed) - Number(a.completed) || CAT_ORDER[catFromCategory(a.category)] - CAT_ORDER[catFromCategory(b.category)])
      .map((c) => {
        const at = c.completed ? times.shift() : undefined;
        return { cat: catFromCategory(c.category), done: c.completed, at: at ? dayFraction(at) : undefined };
      });
  }, [p.chores, p.doneAt]);

  // Bonus only opens once every Must do and Every day chore is done.
  const bonusOpen = p.chores.every((c) => c.category === 'bonus' || c.completed || !c.available);
  const next = p.chores
    .filter((c) => !c.completed && !c.expired)
    .sort((a, b) =>
      Number(b.available) - Number(a.available)
      || CAT_ORDER[catFromCategory(a.category)] - CAT_ORDER[catFromCategory(b.category)]
      || (a.due_by ?? '99').localeCompare(b.due_by ?? '99'))
    .slice(0, NEXT_ROWS);

  const rowFor = (c: ScheduledChore) => {
    const cat = catFromCategory(c.category);
    const state: ChoreState = cat === 'bonus' && !bonusOpen ? 'locked' : 'todo';
    let meta: string | undefined;
    let urgent = false;
    const opens = atToday(c.available_at, now);
    const due = atToday(c.due_by, now);
    if (!c.available && opens && opens > now) {
      meta = t('wall.opensAt', { time: fmtTime(opens) });
    } else if (due) {
      meta = t('wall.dueBy', { time: fmtTime(due) });
      urgent = due.getTime() - now.getTime() < 60 * 60 * 1000;
    }
    return (
      <ChoreRow
        key={`${c.schedule_id}-${c.chore_id}`}
        readOnly
        cat={cat}
        icon={c.icon}
        title={c.title}
        meta={meta}
        urgent={urgent}
        points={c.points_value}
        photo={c.requires_photo}
        state={state}
      />
    );
  };

  const fit = HERO_FIT[skin];
  const rowsRef = useFitRows<HTMLDivElement>([next.map((c) => c.schedule_id).join(), bonusOpen]);

  return (
    <SkinScope skin={skin} color={p.user.color} door className={styles.door} role="group" aria-labelledby={nameId}>
      <div className={styles.doorHead}>
        {p.user.avatar_url
          ? <img src={p.user.avatar_url} alt="" className={styles.photo} />
          : <Avatar name={p.user.name} color={p.user.color} size="lg" className={styles.avatar} />}
        <h2 id={nameId} className={styles.name}>{p.user.name}</h2>
        <div className={styles.meta}>
          <span className={left === 0 && p.total > 0 ? styles.allDone : undefined}>
            {p.total === 0 ? t('wall.nothingToday') : left === 0 ? t('wall.allDone') : t('wall.toGo', { count: left })}
          </span>
          <span aria-hidden>·</span>
          <span className={styles.stars} role="img" aria-label={t('design.points.label', { count: p.balance })}>
            <Icon name="star" />{p.balance}
          </span>
        </div>
        <div className={styles.meta2}>
          {p.streak > 0 && (
            <span className={styles.streak}><Icon name="flame" />{t('wall.streak', { count: p.streak })}</span>
          )}
          <span>{t('wall.ptsToday', { count: p.pointsToday })}</span>
        </div>
      </div>

      {p.total > 0 && (
        <div className={styles.hero}>
          <div className={styles.heroFit} style={{ width: fit.w, '--hero-s': fit.s } as React.CSSProperties}>
            <DayProgress items={items} now={dayFraction(now)} />
          </div>
        </div>
      )}

      {next.length > 0 && (
        <div className={styles.next}>
          <h3 className={styles.label}>{t('wall.nextUp')}</h3>
          <div ref={rowsRef} className={styles.rows}>{next.map(rowFor)}</div>
        </div>
      )}
    </SkinScope>
  );
}

/* ---------------- The family's day, hour by hour ---------------- */

function DayChart({ people, now }: { people: PersonDay[]; now: Date }) {
  const { t, i18n } = useTranslation();
  const nowH = Math.min(Math.max(hoursOf(now), CHART_FROM_H), CHART_TO_H);

  const series: LineSeries[] = people
    .filter((p) => p.total > 0)
    .map((p) => {
      const pct = (n: number) => Math.round((n / p.total) * 100);
      const points = [{ x: CHART_FROM_H, y: 0 }];
      if (p.doneAt.length) {
        p.doneAt.forEach((d, i) => points.push({ x: hoursOf(d), y: pct(i + 1) }));
      } else if (p.completed > 0) {
        // Done, but without completion times: hold today's level.
        points.push({ x: CHART_FROM_H, y: pct(p.completed) });
      }
      points.push({ x: nowH, y: pct(p.completed) });
      return { key: String(p.user.id), label: p.user.name, color: personColorVar(p.user.color), points, step: true };
    });

  const hourLabel = (h: number) => {
    const d = new Date(now);
    d.setHours(h, 0, 0, 0);
    return d.toLocaleTimeString(i18n.language, { hour: 'numeric' });
  };
  const xTicks: Tick[] = [];
  for (let h = CHART_FROM_H; h <= CHART_TO_H; h += 4) xTicks.push({ value: h, label: hourLabel(h) });
  const yTicks: Tick[] = [0, 50, 100].map((v) => ({ value: v, label: `${v}%` }));

  const summary = people
    .filter((p) => p.total > 0)
    .map((p) => t('wall.chartPerson', { name: p.user.name, pct: Math.round((p.completed / p.total) * 100) }))
    .join(', ');

  return (
    <div className={styles.chart}>
      <div className={styles.label} aria-hidden>{t('wall.chartTitle')}</div>
      <LineChart
        title={t('wall.chartTitle')}
        description={t('wall.chartDesc', { summary })}
        series={series}
        xDomain={[CHART_FROM_H, CHART_TO_H]}
        yDomain={[0, 100]}
        xTicks={xTicks}
        yTicks={yTicks}
        marker={{ x: nowH }}
        maxDots={0}
        hideLegend
        height={148}
      />
    </div>
  );
}
