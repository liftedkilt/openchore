import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { api } from '../api';
import { useAuth } from '../AuthContext';
import type { User } from '../types';
import { localDateStr } from '../utils';
import { Avatar, CategoryMark, HouseScope, Icon, catFromCategory, type Cat, type PersonColor } from '../design';
import { LineChart, type Tick } from '../components/charts/LineChart';
import { ColumnChart } from '../components/charts/ColumnChart';
import { BarList } from '../components/charts/BarList';
import { personColorVar } from '../components/charts/personColor';
import { useAIStatus } from '../hooks/useAIStatus';
import styles from './Reports.module.css';

type Period = 'week' | 'month' | 'year';
const PERIODS: Period[] = ['week', 'month', 'year'];

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

const parseDay = (s: string) => new Date(`${s}T00:00:00`);

function shiftDate(dateStr: string, period: Period, direction: number): string {
  const d = parseDay(dateStr);
  if (period === 'week') d.setDate(d.getDate() + 7 * direction);
  else if (period === 'month') d.setMonth(d.getMonth() + direction);
  else d.setFullYear(d.getFullYear() + direction);
  return localDateStr(d);
}

/** Localized short weekday for 0 = Sunday … 6 = Saturday. */
function weekdayName(dow: number, lang: string): string {
  // 2026-09-20 is a Sunday.
  return new Date(2026, 8, 20 + dow).toLocaleDateString(lang, { weekday: 'short' });
}

const pct = (n: number) => `${Math.round(n)}%`;
const CAT_ORDER: Record<Cat, number> = { essential: 0, daily: 1, bonus: 2 };

/** One trend row per day from the period's start to its end or today, whichever is first. */
function fillDays(data: ReportsData): TrendDay[] {
  const byDate = new Map(data.trend.map((d) => [d.date, d]));
  const today = localDateStr(new Date());
  const last = data.end_date < today ? data.end_date : today;
  const out: TrendDay[] = [];
  for (let d = parseDay(data.start_date); localDateStr(d) <= last && out.length < 400; d.setDate(d.getDate() + 1)) {
    const key = localDateStr(d);
    out.push(byDate.get(key) ?? { date: key, completed: 0, assigned: 0 });
  }
  // A past period with gaps in the data still shows what came back.
  return out.length ? out : data.trend;
}

export const Reports: React.FC = () => {
  const navigate = useNavigate();
  const { session } = useAuth();
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const [period, setPeriod] = useState<Period>('week');
  const [date, setDate] = useState(() => localDateStr(new Date()));
  const [data, setData] = useState<ReportsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [colors, setColors] = useState<Record<number, PersonColor | undefined>>({});
  const [aiSummaries, setAiSummaries] = useState<Record<number, string>>({});
  const [summaryLoading, setSummaryLoading] = useState<Record<number, boolean>>({});
  // Summaries need an AI model on the server; without one the button goes.
  const aiStatus = useAIStatus();

  // People's colours: every per-person series is drawn in its person's colour.
  useEffect(() => {
    api.users.list()
      .then((users: User[]) => setColors(Object.fromEntries(users.map((u) => [u.id, u.color]))))
      .catch(() => setColors({}));
  }, []);

  const handleGenerateSummary = async (userId: number) => {
    setSummaryLoading((prev) => ({ ...prev, [userId]: true }));
    try {
      const resp = await api.admin.getAISummary(userId, period, date);
      setAiSummaries((prev) => ({ ...prev, [userId]: resp.summary }));
    } catch (e) {
      const msg = e instanceof Error && e.message ? e.message : t('reports.failedToGenerateSummary');
      setAiSummaries((prev) => ({ ...prev, [userId]: msg }));
    } finally {
      setSummaryLoading((prev) => ({ ...prev, [userId]: false }));
    }
  };

  const fetchReports = useCallback(async () => {
    setLoading(true);
    setAiSummaries({});
    try {
      setData(await api.reports.get(period, date));
    } catch {
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [period, date]);

  useEffect(() => {
    fetchReports();
  }, [fetchReports]);

  const handlePeriodChange = (p: Period) => {
    setPeriod(p);
    setDate(localDateStr(new Date()));
  };

  const range = useMemo(() => {
    if (!data) return '';
    const s = parseDay(data.start_date).toLocaleDateString(lang, { month: 'short', day: 'numeric' });
    const e = parseDay(data.end_date).toLocaleDateString(lang, { month: 'short', day: 'numeric', year: 'numeric' });
    return `${s} – ${e}`;
  }, [data, lang]);

  return (
    <HouseScope mode="auto" persistent={!!session?.persistent} className={styles.page}>
      <div className={styles.inner}>
        <header className={styles.top}>
          <button type="button" className={styles.iconBtn} onClick={() => navigate('/admin/dashboard')} aria-label={t('reports.back')}>
            <Icon name="back" />
          </button>
          <h1 className={styles.title}>{t('reports.title')}</h1>
        </header>

        <div className={styles.periodBar}>
          <div className={styles.segmented} role="group" aria-label={t('reports.periodLabel')}>
            {PERIODS.map((p) => (
              <button
                key={p}
                type="button"
                className={clsx(styles.segment, period === p && styles.segmentOn)}
                aria-pressed={period === p}
                onClick={() => handlePeriodChange(p)}
              >
                {t(`reports.period_${p}`)}
              </button>
            ))}
          </div>
          <div className={styles.stepper}>
            <button type="button" className={styles.iconBtn} onClick={() => setDate(shiftDate(date, period, -1))} aria-label={t('reports.previous')}>
              <Icon name="back" />
            </button>
            <span className={styles.range} aria-live="polite">{range}</span>
            <button type="button" className={styles.iconBtn} onClick={() => setDate(shiftDate(date, period, 1))} aria-label={t('reports.next')}>
              <Icon name="chev" />
            </button>
          </div>
        </div>

        {loading ? (
          <p className={styles.status}>{t('reports.loadingReports')}</p>
        ) : !data ? (
          <p className={styles.status}>{t('reports.failedToLoadReports')}</p>
        ) : (
          <ReportBody
            data={data}
            colors={colors}
            aiSummaries={aiSummaries}
            summaryLoading={summaryLoading}
            onSummary={aiStatus.ai.configured ? handleGenerateSummary : undefined}
          />
        )}
      </div>
    </HouseScope>
  );
};

interface ReportBodyProps {
  data: ReportsData;
  colors: Record<number, PersonColor | undefined>;
  aiSummaries: Record<number, string>;
  summaryLoading: Record<number, boolean>;
  /** Writes a person's AI summary; absent when no AI is configured. */
  onSummary?: (userId: number) => void;
}

function ReportBody({ data, colors, aiSummaries, summaryLoading, onSummary }: ReportBodyProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const empty = <p className={styles.empty}>{t('reports.noData')}</p>;

  // Completion trend: the family's done vs. assigned for every day of the
  // period so far (days without chores count as zero).
  const trend = useMemo(() => fillDays(data), [data]);
  const n = trend.length;
  const dayLabel = (s: string) => parseDay(s).toLocaleDateString(lang, { month: 'short', day: 'numeric' });
  const step = Math.max(1, Math.ceil(n / 6));
  const xTicks: Tick[] = trend
    .map((d, i) => ({ value: i, label: dayLabel(d.date) }))
    .filter((_, i) => i % step === 0);
  const doneTotal = trend.reduce((s, d) => s + d.completed, 0);
  const assignedTotal = trend.reduce((s, d) => s + d.assigned, 0);

  // All seven days, Monday first, with the best and the hardest named in words.
  const week = [1, 2, 3, 4, 5, 6, 0].map((dow) => data.day_of_week.find((d) => d.day_of_week === dow)
    ?? { day_of_week: dow, day_name: '', total_assigned: 0, total_completed: 0, completion_rate: 0 });
  const days = week.filter((d) => d.total_assigned > 0);
  const best = days.length ? days.reduce((a, b) => (b.completion_rate > a.completion_rate ? b : a)) : null;
  const worst = days.length ? days.reduce((a, b) => (b.completion_rate < a.completion_rate ? b : a)) : null;

  return (
    <div className={styles.grid}>
      <section className={clsx(styles.card, styles.wide)} aria-labelledby="rep-people">
        <h2 id="rep-people" className={styles.cardTitle}>{t('reports.scorecards')}</h2>
        {data.kids.length === 0 ? (
          <p className={styles.empty}>{t('reports.noDataForPeriod')}</p>
        ) : (
          <ul className={styles.people}>
            {data.kids.map((kid) => (
              <li key={kid.user_id} className={styles.person} data-person={colors[kid.user_id] || undefined}>
                <div className={styles.personMain}>
                  {kid.avatar_url
                    ? <img src={kid.avatar_url} alt="" className={styles.photo} />
                    : <Avatar name={kid.name} color={colors[kid.user_id]} />}
                  <div className={styles.personText}>
                    <h3 className={styles.personName}>{kid.name}</h3>
                    <p className={styles.personStats}>
                      <span>{t('reports.doneCount', { completed: kid.total_completed, assigned: kid.total_assigned })}</span>
                      <span>{t('reports.pointsEarned', { count: kid.points_earned })}</span>
                      <span className={styles.streak}><Icon name="flame" />{t('reports.streakDays', { count: kid.current_streak })}</span>
                    </p>
                  </div>
                  <div className={styles.rate}>
                    {pct(kid.completion_rate)}
                  </div>
                </div>
                <span className={styles.personBar} aria-hidden><i style={{ width: pct(Math.min(100, kid.completion_rate)) }} /></span>
                {aiSummaries[kid.user_id] ? (
                  <p className={styles.summary}>{aiSummaries[kid.user_id]}</p>
                ) : onSummary && (
                  <button
                    type="button"
                    className={styles.summaryBtn}
                    onClick={() => onSummary(kid.user_id)}
                    disabled={summaryLoading[kid.user_id]}
                  >
                    <Icon name="spark" />
                    {summaryLoading[kid.user_id] ? t('reports.generating') : t('reports.aiSummary')}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className={clsx(styles.card, styles.wide)} aria-labelledby="rep-trend">
        <h2 id="rep-trend" className={styles.cardTitle}>{t('reports.completionTrend')}</h2>
        {n === 0 ? empty : (
          <LineChart
            title={t('reports.completionTrend')}
            description={t('reports.trendDesc', { from: dayLabel(data.start_date), to: dayLabel(data.end_date), done: doneTotal, assigned: assignedTotal })}
            series={[
              {
                key: 'done', label: t('reports.seriesCompleted'), color: 'var(--ink)', area: true,
                points: trend.map((d, i) => ({ x: i, y: d.completed })),
              },
              {
                key: 'assigned', label: t('reports.seriesAssigned'), color: 'var(--ink-muted)', dashed: true,
                points: trend.map((d, i) => ({ x: i, y: d.assigned })),
              },
            ]}
            xDomain={[0, Math.max(1, n - 1)]}
            xTicks={xTicks}
            maxDots={31}
            pointTitle={(s, p) => `${dayLabel(trend[p.x].date)} · ${s.label}: ${p.y}`}
          />
        )}
      </section>

      <section className={styles.card} aria-labelledby="rep-rate">
        <h2 id="rep-rate" className={styles.cardTitle}>{t('reports.byPerson')}</h2>
        {data.kids.length === 0 ? empty : (
          <BarList
            label={t('reports.byPerson')}
            max={100}
            items={data.kids.map((k) => ({
              key: k.user_id,
              label: k.name,
              lead: <Avatar name={k.name} color={colors[k.user_id]} size="sm" />,
              value: k.completion_rate,
              valueLabel: pct(k.completion_rate),
              color: personColorVar(colors[k.user_id]),
            }))}
          />
        )}
      </section>

      <section className={styles.card} aria-labelledby="rep-points">
        <h2 id="rep-points" className={styles.cardTitle}>{t('reports.pointsFlow')}</h2>
        {data.points.length === 0 ? empty : (
          <BarList
            label={t('reports.pointsFlow')}
            items={data.points.map((p) => ({
              key: p.user_id,
              label: p.name,
              lead: <Avatar name={p.name} color={colors[p.user_id]} size="sm" />,
              value: p.points_earned,
              valueLabel: `+${p.points_earned}`,
              color: personColorVar(colors[p.user_id]),
              note: [
                p.points_decayed > 0 ? t('reports.pointsDecayed', { count: p.points_decayed }) : null,
                p.points_spent > 0 ? t('reports.pointsSpent', { count: p.points_spent }) : null,
              ].filter(Boolean).join(' · ') || undefined,
            }))}
          />
        )}
      </section>

      <section className={styles.card} aria-labelledby="rep-missed">
        <h2 id="rep-missed" className={styles.cardTitle}>{t('reports.mostMissedChores')}</h2>
        {data.most_missed.length === 0 ? (
          <p className={styles.empty}>{t('reports.noMissedChores')}</p>
        ) : (
          <BarList
            label={t('reports.mostMissedChores')}
            items={data.most_missed.map((m) => ({
              key: m.chore_id,
              label: m.chore_name,
              value: m.miss_count,
              valueLabel: t('reports.missedTimes', { count: m.miss_count }),
              color: 'var(--ink-muted)',
              note: m.kids?.length ? m.kids.join(', ') : undefined,
            }))}
          />
        )}
      </section>

      <section className={styles.card} aria-labelledby="rep-cats">
        <h2 id="rep-cats" className={styles.cardTitle}>{t('reports.categoryBreakdown')}</h2>
        {data.categories.length === 0 ? empty : (
          <BarList
            label={t('reports.categoryBreakdown')}
            max={100}
            items={[...data.categories].sort((a, b) => CAT_ORDER[catFromCategory(a.category)] - CAT_ORDER[catFromCategory(b.category)]).map((c) => {
              const cat = catFromCategory(c.category);
              return {
                key: c.category,
                label: t(`design.category.${cat}`),
                lead: <CategoryMark cat={cat} className={styles.mark} />,
                value: c.completion_rate,
                valueLabel: pct(c.completion_rate),
                note: t('reports.doneCount', { completed: c.total_completed, assigned: c.total_assigned }),
              };
            })}
          />
        )}
      </section>

      <section className={clsx(styles.card, styles.wide)} aria-labelledby="rep-days">
        <h2 id="rep-days" className={styles.cardTitle}>{t('reports.bestWorstDays')}</h2>
        {days.length === 0 ? empty : (
          <>
            <ColumnChart
              title={t('reports.bestWorstDays')}
              description={best && worst ? t('reports.daysSummary', {
                best: weekdayName(best.day_of_week, lang), bestRate: pct(best.completion_rate),
                worst: weekdayName(worst.day_of_week, lang), worstRate: pct(worst.completion_rate),
              }) : undefined}
              max={100}
              data={week.map((d) => ({
                label: weekdayName(d.day_of_week, lang),
                value: d.completion_rate,
                valueLabel: d.total_assigned > 0 ? pct(d.completion_rate) : '–',
                strong: !!best && d.day_of_week === best.day_of_week,
              }))}
            />
            {best && worst && best !== worst && (
              <p className={styles.caption} aria-hidden>
                {t('reports.daysSummary', {
                  best: weekdayName(best.day_of_week, lang), bestRate: pct(best.completion_rate),
                  worst: weekdayName(worst.day_of_week, lang), worstRate: pct(worst.completion_rate),
                })}
              </p>
            )}
          </>
        )}
      </section>
    </div>
  );
}
