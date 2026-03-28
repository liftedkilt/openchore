import React, { useEffect, useState, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, fetchAsUser } from '../api';
import type { User, ScheduledChore, UserStreakData, PointsData } from '../types';
import styles from './AmbientDashboard.module.css';
import { Flame } from 'lucide-react';
import { localDateStr } from '../utils';

// Assign each kid a distinct color
const KID_COLORS = ['#38bdf8', '#a78bfa', '#f472b6', '#34d399', '#fb923c', '#facc15'];

interface TimelinePoint {
  time: Date;
  cumPct: number;
}

interface KidData {
  user: User;
  completed: number;
  total: number;
  percent: number;
  pointsToday: number;
  totalBalance: number;
  streak: number;
  timeline: TimelinePoint[];
}

const ProgressRing: React.FC<{ percent: number }> = ({ percent }) => {
  // Uses a fixed viewBox; actual size is controlled by the parent container via CSS
  const vb = 130;
  const strokeWidth = 5;
  const radius = (vb - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (Math.min(percent, 100) / 100) * circumference;
  const color = percent >= 100 ? '#22c55e' : percent >= 50 ? '#38bdf8' : '#f59e0b';

  return (
    <svg viewBox={`0 0 ${vb} ${vb}`} className={styles.progressRing}>
      <circle cx={vb / 2} cy={vb / 2} r={radius}
        stroke="rgba(255,255,255,0.06)" strokeWidth={strokeWidth} fill="none" />
      <circle cx={vb / 2} cy={vb / 2} r={radius}
        stroke={color} strokeWidth={strokeWidth} fill="none"
        strokeDasharray={circumference} strokeDashoffset={offset}
        strokeLinecap="round" transform={`rotate(-90 ${vb / 2} ${vb / 2})`}
        style={{ transition: 'stroke-dashoffset 0.8s ease' }} />
    </svg>
  );
};

// Combined line chart showing all kids' cumulative completion % over time today
const TimelineChart: React.FC<{ kids: KidData[]; colors: string[] }> = ({ kids, colors }) => {
  const width = 700;
  const height = 200;
  const padL = 44;
  const padR = 16;
  const padT = 28; // room for legend
  const padB = 28;
  const chartW = width - padL - padR;
  const chartH = height - padT - padB;

  // X axis: 6am to 10pm (16 hours)
  const startHour = 6;
  const endHour = 22;
  const hourSpan = endHour - startHour;

  const now = new Date();
  const currentHour = now.getHours() + now.getMinutes() / 60;
  const clampedHour = Math.min(Math.max(currentHour, startHour), endHour);
  const nowX = padL + ((clampedHour - startHour) / hourSpan) * chartW;

  const hourToX = (h: number) => padL + ((Math.min(Math.max(h, startHour), endHour) - startHour) / hourSpan) * chartW;
  const pctToY = (p: number) => padT + chartH - (Math.min(Math.max(p, 0), 100) / 100) * chartH;

  // Hour labels every 2 hours
  const hourLabels: { h: number; label: string }[] = [];
  for (let h = startHour; h <= endHour; h += 2) {
    const label = h < 12 ? `${h}a` : h === 12 ? '12p' : `${h - 12}p`;
    hourLabels.push({ h, label });
  }

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className={styles.chartSvg} preserveAspectRatio="xMidYMid meet">
      {/* Grid lines */}
      {[0, 25, 50, 75, 100].map(p => (
        <g key={p}>
          <line x1={padL} y1={pctToY(p)} x2={padL + chartW} y2={pctToY(p)}
            stroke="rgba(255,255,255,0.06)" strokeWidth={1} />
          <text x={padL - 8} y={pctToY(p) + 3.5} textAnchor="end"
            fontSize="9" fill="rgba(255,255,255,0.3)" fontWeight="600">{p}%</text>
        </g>
      ))}

      {/* Hour labels */}
      {hourLabels.map(({ h, label }) => (
        <text key={h} x={hourToX(h)} y={height - 6} textAnchor="middle"
          fontSize="9" fill="rgba(255,255,255,0.3)" fontWeight="600">{label}</text>
      ))}

      {/* "Now" indicator */}
      {currentHour >= startHour && currentHour <= endHour && (
        <line x1={nowX} y1={padT} x2={nowX} y2={padT + chartH}
          stroke="rgba(255,255,255,0.15)" strokeWidth={1} strokeDasharray="4,3" />
      )}

      {/* Lines per kid — always render a line, even if no timeline data */}
      {kids.map((kid, idx) => {
        if (kid.total === 0) return null;
        const color = colors[idx % colors.length];

        // Build SVG path points
        const svgPoints: { x: number; y: number }[] = [];

        // Always start at 0% at the beginning of the day
        svgPoints.push({ x: hourToX(startHour), y: pctToY(0) });

        if (kid.timeline.length > 0) {
          // We have completion timestamps — plot them as a step function
          let prevPct = 0;
          for (const pt of kid.timeline) {
            const h = pt.time.getHours() + pt.time.getMinutes() / 60;
            const x = hourToX(h);
            // Horizontal line at previous % up to this time (step function)
            svgPoints.push({ x, y: pctToY(prevPct) });
            // Then step up to new %
            svgPoints.push({ x, y: pctToY(pt.cumPct) });
            prevPct = pt.cumPct;
          }
          // Extend horizontally to current time
          svgPoints.push({ x: nowX, y: pctToY(prevPct) });
        } else {
          // No timeline data — show current % as a flat line from start to now
          // This handles the case where completed_at isn't available
          svgPoints.push({ x: nowX, y: pctToY(kid.percent) });
        }

        const pathD = svgPoints.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ');
        const lastY = svgPoints[svgPoints.length - 1].y;

        return (
          <g key={kid.user.id}>
            <path d={pathD} fill="none" stroke={color} strokeWidth={2.5}
              strokeLinecap="round" strokeLinejoin="round" opacity={0.85} />
            {/* Dot at current position */}
            <circle cx={nowX} cy={lastY} r={4.5}
              fill={color} stroke="#0f172a" strokeWidth={2} />
          </g>
        );
      })}

      {/* Legend at top */}
      {kids.map((kid, idx) => {
        if (kid.total === 0) return null;
        const color = colors[idx % colors.length];
        const lx = padL + idx * 110;
        return (
          <g key={kid.user.id}>
            <line x1={lx} y1={12} x2={lx + 14} y2={12}
              stroke={color} strokeWidth={2.5} strokeLinecap="round" />
            <text x={lx + 20} y={15} fontSize="10" fill="rgba(255,255,255,0.6)" fontWeight="600">
              {kid.user.name}
            </text>
          </g>
        );
      })}
    </svg>
  );
};

export const AmbientDashboard: React.FC = () => {
  const navigate = useNavigate();
  const [kids, setKids] = useState<KidData[]>([]);
  const [currentTime, setCurrentTime] = useState(new Date());
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    try {
      const allUsers: User[] = await api.users.list();
      const childUsers = allUsers.filter(u => u.role === 'child' && !u.paused);
      const today = localDateStr(new Date());

      const results = await Promise.allSettled(
        childUsers.map(async (kid) => {
          const [chores, streakData, pointsData] = await Promise.all([
            fetchAsUser<ScheduledChore[]>(kid.id, `/users/${kid.id}/chores?view=daily&date=${today}`),
            fetchAsUser<UserStreakData>(kid.id, `/users/${kid.id}/streak`),
            fetchAsUser<PointsData>(kid.id, `/users/${kid.id}/points`),
          ]);

          const completed = chores.filter(c => c.completed).length;
          const total = chores.length;
          const pointsToday = chores.filter(c => c.completed).reduce((sum, c) => sum + c.points_value, 0);

          // Build timeline from completed_at timestamps
          const completedChores = chores
            .filter(c => c.completed && c.completed_at)
            .map(c => {
              const t = new Date(c.completed_at!);
              return { time: t, valid: !isNaN(t.getTime()) };
            })
            .filter(c => c.valid)
            .sort((a, b) => a.time.getTime() - b.time.getTime());

          const timeline: TimelinePoint[] = [];
          let cumCompleted = 0;
          for (const cc of completedChores) {
            cumCompleted++;
            timeline.push({
              time: cc.time,
              cumPct: total > 0 ? Math.round((cumCompleted / total) * 100) : 0,
            });
          }

          return {
            user: kid,
            completed,
            total,
            percent: total > 0 ? Math.round((completed / total) * 100) : 0,
            pointsToday,
            totalBalance: pointsData.balance,
            streak: streakData.current_streak,
            timeline,
          };
        })
      );

      setKids(results
        .filter((r): r is PromiseFulfilledResult<KidData> => r.status === 'fulfilled')
        .map(r => r.value)
      );
    } catch (err) {
      console.error('Ambient fetch error:', err);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 45000);
    return () => clearInterval(interval);
  }, [fetchData]);

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  // Wake lock
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

  const timeStr = currentTime.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
  const dateStr = currentTime.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });

  // Stable color assignment by user ID order
  const sortedKids = useMemo(() => [...kids].sort((a, b) => a.user.id - b.user.id), [kids]);
  const colorMap = useMemo(() => {
    const map = new Map<number, string>();
    sortedKids.forEach((k, i) => map.set(k.user.id, KID_COLORS[i % KID_COLORS.length]));
    return map;
  }, [sortedKids]);

  if (loading) return <div className={styles.container} />;

  // Sort by completion percentage (leaders first) for card display
  const sorted = [...kids].sort((a, b) => b.percent - a.percent);
  const chartColors = sortedKids.map(k => colorMap.get(k.user.id) || KID_COLORS[0]);

  return (
    <div className={styles.container} onClick={() => navigate('/login')}>
      <header className={styles.header}>
        <div className={styles.clock}>{timeStr}</div>
        <div className={styles.date}>{dateStr}</div>
      </header>

      <div className={styles.grid}>
        {sorted.map((kid, i) => {
          const allDone = kid.completed === kid.total && kid.total > 0;
          const isLeader = i === 0 && kid.percent > 0;
          const color = colorMap.get(kid.user.id) || KID_COLORS[0];

          return (
            <div key={kid.user.id} className={`${styles.card} ${allDone ? styles.cardDone : ''} ${isLeader ? styles.cardLeader : ''}`}>
              <div className={styles.avatarWrap}>
                <ProgressRing percent={kid.percent} />
                <div className={styles.avatarInner}>
                  {kid.user.avatar_url
                    ? <img src={kid.user.avatar_url} alt={kid.user.name} className={styles.avatarImg} />
                    : <div className={styles.avatarPlaceholder} />}
                </div>
              </div>
              <h2 className={styles.name}>{kid.user.name}</h2>
              <div className={styles.completionCount}>{kid.completed}/{kid.total}</div>
              <div className={styles.completionLabel}>chores done</div>
              <div className={styles.statsRow}>
                {kid.streak > 0 && (
                  <span className={styles.streak}><Flame size={14} /> {kid.streak}d</span>
                )}
                <span className={styles.points}>{kid.pointsToday} pts today</span>
              </div>
              {/* Color indicator matching chart line */}
              <div className={styles.colorDot} style={{ backgroundColor: color }} />
            </div>
          );
        })}
      </div>

      {/* Combined timeline chart */}
      <div className={styles.chartPanel}>
        <h3 className={styles.chartTitle}>Today's Progress</h3>
        <TimelineChart kids={sortedKids} colors={chartColors} />
      </div>
    </div>
  );
};
