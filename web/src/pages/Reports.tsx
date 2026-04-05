import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { BarChart } from '../components/charts/BarChart';
import { LineChart } from '../components/charts/LineChart';
import styles from './Reports.module.css';
import { ArrowLeft, ChevronLeft, ChevronRight, Users, TrendingUp, BarChart3, Coins, Calendar, AlertTriangle, Sparkles } from 'lucide-react';
import clsx from 'clsx';

type Period = 'week' | 'month' | 'year';

interface KidSummary {
  user_id: number;
  name: string;
  avatar_url: string;
  total_assigned: number;
  total_completed: number;
  total_missed: number;
  completion_rate: number;
  points_earned: number;
  current_streak: number;
}

interface MissedChore {
  chore_id: number;
  chore_name: string;
  miss_count: number;
  kids: string[];
}

interface TrendDay {
  date: string;
  completed: number;
  assigned: number;
}

interface CategoryStat {
  category: string;
  total_assigned: number;
  total_completed: number;
  completion_rate: number;
}

interface PointsSummary {
  user_id: number;
  name: string;
  points_earned: number;
  points_decayed: number;
  points_spent: number;
}

interface DayOfWeekStat {
  day_of_week: number;
  day_name: string;
  total_assigned: number;
  total_completed: number;
  completion_rate: number;
}

interface ReportsData {
  period: string;
  start_date: string;
  end_date: string;
  kids: KidSummary[];
  most_missed: MissedChore[];
  trend: TrendDay[];
  categories: CategoryStat[];
  points: PointsSummary[];
  day_of_week: DayOfWeekStat[];
}

function formatDateRange(start: string, end: string): string {
  const s = new Date(start + 'T00:00:00');
  const e = new Date(end + 'T00:00:00');
  const opts: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric' };
  const sStr = s.toLocaleDateString('en-US', opts);
  const eStr = e.toLocaleDateString('en-US', { ...opts, year: 'numeric' });
  return `${sStr} - ${eStr}`;
}

function shiftDate(dateStr: string, period: Period, direction: number): string {
  const d = new Date(dateStr + 'T00:00:00');
  switch (period) {
    case 'week':
      d.setDate(d.getDate() + 7 * direction);
      break;
    case 'month':
      d.setMonth(d.getMonth() + direction);
      break;
    case 'year':
      d.setFullYear(d.getFullYear() + direction);
      break;
  }
  return d.toISOString().slice(0, 10);
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10);
}

function rateColor(rate: number): string {
  if (rate >= 80) return styles.rateGreen;
  if (rate >= 50) return styles.rateYellow;
  return styles.rateRed;
}

const categoryColors: Record<string, string> = {
  required: '#ef4444',
  core: '#3b82f6',
  bonus: '#f59e0b',
};

const categoryStyleMap: Record<string, string> = {
  required: styles.categoryRequired,
  core: styles.categoryCore,
  bonus: styles.categoryBonus,
};

export const Reports: React.FC = () => {
  const navigate = useNavigate();
  const [period, setPeriod] = useState<Period>('week');
  const [date, setDate] = useState(todayStr());
  const [data, setData] = useState<ReportsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [aiSummaries, setAiSummaries] = useState<Record<number, string>>({});
  const [summaryLoading, setSummaryLoading] = useState<Record<number, boolean>>({});

  const handleGenerateSummary = async (userId: number) => {
    setSummaryLoading(prev => ({ ...prev, [userId]: true }));
    try {
      const resp = await api.admin.getAISummary(userId, period, date);
      setAiSummaries(prev => ({ ...prev, [userId]: resp.summary }));
    } catch {
      setAiSummaries(prev => ({ ...prev, [userId]: 'Failed to generate summary. AI services may not be available.' }));
    } finally {
      setSummaryLoading(prev => ({ ...prev, [userId]: false }));
    }
  };

  const fetchReports = useCallback(async () => {
    setLoading(true);
    setAiSummaries({});
    try {
      const resp = await api.reports.get(period, date);
      setData(resp);
    } catch {
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [period, date]);

  useEffect(() => {
    if (!sessionStorage.getItem('openchore_admin')) {
      navigate('/admin', { replace: true });
      return;
    }
    fetchReports();
  }, [fetchReports, navigate]);

  const handlePeriodChange = (p: Period) => {
    setPeriod(p);
    setDate(todayStr());
  };

  return (
    <div className={styles.wrapper}>
      <header className={styles.header}>
        <button className={styles.backBtn} onClick={() => navigate('/admin/dashboard')}>
          <ArrowLeft size={18} />
        </button>
        <h1 className={styles.title}>Reports</h1>
      </header>

      {/* Period selector */}
      <div className={styles.periodNav}>
        <button className={styles.arrowBtn} onClick={() => setDate(shiftDate(date, period, -1))}>
          <ChevronLeft size={18} />
        </button>
        <div className={styles.periodTabs}>
          {(['week', 'month', 'year'] as Period[]).map(p => (
            <button
              key={p}
              className={clsx(styles.periodTab, period === p && styles.periodTabActive)}
              onClick={() => handlePeriodChange(p)}
            >
              {p.charAt(0).toUpperCase() + p.slice(1)}
            </button>
          ))}
        </div>
        <button className={styles.arrowBtn} onClick={() => setDate(shiftDate(date, period, 1))}>
          <ChevronRight size={18} />
        </button>
      </div>

      {data && (
        <div className={styles.dateLabel}>
          {formatDateRange(data.start_date, data.end_date)}
        </div>
      )}

      {loading ? (
        <div className={styles.loading}>Loading reports...</div>
      ) : !data ? (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}><BarChart3 size={48} /></div>
          <div>Failed to load reports</div>
        </div>
      ) : (
        <div className={styles.content}>
          {/* Kid Scorecards */}
          <div className={styles.card}>
            <div className={styles.cardTitle}>
              <Users size={16} className={styles.cardTitleIcon} />
              Kid Scorecards
            </div>
            {data.kids.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', textAlign: 'center', padding: '1rem' }}>No data for this period</div>
            ) : (
              <div className={styles.kidGrid}>
                {data.kids.map(kid => (
                  <div key={kid.user_id} className={styles.kidCard}>
                    <div className={styles.kidCardMain}>
                      <div className={styles.kidAvatar}>
                        {kid.avatar_url ? (
                          <img src={kid.avatar_url} alt={kid.name} />
                        ) : (
                          <div className={styles.kidAvatarPlaceholder} />
                        )}
                      </div>
                      <div className={styles.kidInfo}>
                        <div className={styles.kidName}>{kid.name}</div>
                        <div className={styles.kidStats}>
                          <span className={styles.kidStat}>{kid.total_completed}/{kid.total_assigned} done</span>
                          <span className={styles.kidStat}>{kid.points_earned} pts</span>
                          <span className={styles.kidStat}>{kid.current_streak}d streak</span>
                        </div>
                      </div>
                      <div className={clsx(styles.kidRate, rateColor(kid.completion_rate))}>
                        {Math.round(kid.completion_rate)}%
                      </div>
                    </div>
                    {aiSummaries[kid.user_id] ? (
                      <div className={styles.aiSummaryCard}>
                        <div className={styles.aiSummaryText}>{aiSummaries[kid.user_id]}</div>
                      </div>
                    ) : (
                      <button
                        className={styles.aiSummaryBtn}
                        onClick={() => handleGenerateSummary(kid.user_id)}
                        disabled={summaryLoading[kid.user_id]}
                      >
                        {summaryLoading[kid.user_id] ? (
                          <><span className={styles.spinnerSmall} /> Generating...</>
                        ) : (
                          <><Sparkles size={12} /> AI Summary</>
                        )}
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Completion Trend */}
          <div className={styles.card}>
            <div className={styles.cardTitle}>
              <TrendingUp size={16} className={styles.cardTitleIcon} />
              Completion Trend
            </div>
            <LineChart
              height={200}
              series={[
                {
                  data: data.trend.map(t => ({ label: t.date, value: t.completed })),
                  color: '#22c55e',
                  label: 'Completed',
                  filled: true,
                },
                {
                  data: data.trend.map(t => ({ label: t.date, value: t.assigned })),
                  color: 'var(--accent-blue)',
                  label: 'Assigned',
                  filled: false,
                },
              ]}
            />
          </div>

          {/* Most Missed Chores */}
          <div className={styles.card}>
            <div className={styles.cardTitle}>
              <AlertTriangle size={16} className={styles.cardTitleIcon} />
              Most Missed Chores
            </div>
            {data.most_missed.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', textAlign: 'center', padding: '1rem' }}>No missed chores!</div>
            ) : (
              <BarChart
                data={data.most_missed.map(m => ({
                  label: m.chore_name,
                  value: m.miss_count,
                  color: '#ef4444',
                }))}
                horizontal
                height={data.most_missed.length * 36}
                barColor="#ef4444"
              />
            )}
          </div>

          {/* Category Breakdown */}
          <div className={styles.card}>
            <div className={styles.cardTitle}>
              <BarChart3 size={16} className={styles.cardTitleIcon} />
              Category Breakdown
            </div>
            {data.categories.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', textAlign: 'center', padding: '1rem' }}>No data</div>
            ) : (
              <div className={styles.categoryList}>
                {data.categories.map(cat => (
                  <div key={cat.category} className={styles.categoryRow}>
                    <span className={clsx(styles.categoryLabel, categoryStyleMap[cat.category])}>
                      {cat.category}
                    </span>
                    <div className={styles.categoryBar}>
                      <div
                        className={styles.categoryFill}
                        style={{
                          width: `${Math.round(cat.completion_rate)}%`,
                          background: categoryColors[cat.category] || 'var(--accent-blue)',
                        }}
                      />
                    </div>
                    <span className={styles.categoryRate}>
                      {Math.round(cat.completion_rate)}%
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Points Flow */}
          <div className={styles.card}>
            <div className={styles.cardTitle}>
              <Coins size={16} className={styles.cardTitleIcon} />
              Points Flow
            </div>
            {data.points.length === 0 ? (
              <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', textAlign: 'center', padding: '1rem' }}>No data</div>
            ) : (
              <div className={styles.pointsGrid}>
                {data.points.map(p => (
                  <div key={p.user_id} className={styles.pointsRow}>
                    <span className={styles.pointsName}>{p.name}</span>
                    <div className={styles.pointsValues}>
                      <span className={styles.pointsEarned}>+{p.points_earned}</span>
                      {p.points_decayed > 0 && (
                        <span className={styles.pointsDecayed}>-{p.points_decayed} decay</span>
                      )}
                      {p.points_spent > 0 && (
                        <span className={styles.pointsSpent}>-{p.points_spent} spent</span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Day of Week */}
          <div className={styles.card}>
            <div className={styles.cardTitle}>
              <Calendar size={16} className={styles.cardTitleIcon} />
              Best / Worst Days
            </div>
            <BarChart
              data={data.day_of_week.map(d => ({
                label: d.day_name,
                value: Math.round(d.completion_rate),
                color: d.completion_rate >= 80 ? '#22c55e' : d.completion_rate >= 50 ? '#f59e0b' : '#ef4444',
              }))}
              height={180}
              showValues
            />
          </div>
        </div>
      )}
    </div>
  );
};
