import React, { useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Icon } from '../../design';
import type { ScheduledChore, User } from '../../types';
import { localDateStr } from '../../utils';
import { isDone, type ChoreView } from './choreView';
import { ChoreGroups, Notice, PausedNotice } from './parts';
import type { KidData } from './useKidData';
import s from './kid.module.css';

/** Monday-first dates (YYYY-MM-DD) of the week that holds `day`. */
export function weekDates(day: string): string[] {
  const d = new Date(`${day}T00:00:00`);
  const offset = (d.getDay() + 6) % 7; // Monday = 0
  d.setDate(d.getDate() - offset);
  return Array.from({ length: 7 }, (_, i) => {
    const x = new Date(d);
    x.setDate(d.getDate() + i);
    return localDateStr(x);
  });
}

type DayKind = 'full' | 'partial' | 'future' | 'none';

function DayMark({ kind, pct }: { kind: DayKind; pct: number }) {
  return (
    <svg className={s.dayMark} data-kind={kind} viewBox="0 0 36 36" aria-hidden focusable="false">
      {kind === 'full' && (
        <>
          <circle className={s.dayFull} cx="18" cy="18" r="16" />
          <path className={s.dayTick} d="M11.5 18.5l4.5 4.5 8.5-9" />
        </>
      )}
      {kind === 'partial' && (
        <>
          <circle className={s.dayTrack} cx="18" cy="18" r="14.5" />
          {pct > 0 && (
            <circle
              className={s.dayArc}
              cx="18" cy="18" r="14.5"
              pathLength={100}
              strokeDasharray={`${pct} 100`}
              transform="rotate(-90 18 18)"
            />
          )}
        </>
      )}
      {kind === 'future' && <circle className={s.dayDashed} cx="18" cy="18" r="14.5" />}
      {kind === 'none' && <circle className={s.dayNone} cx="18" cy="18" r="3" />}
    </svg>
  );
}

interface WeekProps {
  user: User;
  data: KidData;
  topBar: React.ReactNode;
  viewOf: (c: ScheduledChore) => ChoreView;
  tts: boolean;
  onToggle: (c: ScheduledChore, view: ChoreView) => void;
  onPhoto: (c: ScheduledChore) => void;
  onSpeak: (text: string, audioUrl?: string) => void;
}

export function WeekScreen({ user, data, topBar, viewOf, tts, onToggle, onPhoto, onSpeak }: WeekProps) {
  const { t, i18n } = useTranslation();
  const [selected, setSelected] = useState(data.today);
  const dates = weekDates(data.today);
  const all = data.weekChores ?? [];
  const streak = data.streak;
  const lang = i18n.language;
  const day = dates.includes(selected) ? selected : data.today;
  const dayChores = all.filter(c => c.date === day);
  const dayLabel = new Date(`${day}T00:00:00`).toLocaleDateString(lang, { weekday: 'long', month: 'long', day: 'numeric' });

  return (
    <div className={s.week}>
      {topBar}
      <h1 className={s.screenTitle}>{t('kid.week.title')}</h1>

      <div className={s.strip} role="group" aria-label={t('kid.week.days')}>
        {dates.map(date => {
          const d = new Date(`${date}T00:00:00`);
          const list = all.filter(c => c.date === date);
          const done = list.filter(isDone).length;
          const total = list.length;
          const isToday = date === data.today;
          const kind: DayKind = total === 0 ? 'none'
            : done === total ? 'full'
            : date > data.today ? 'future' : 'partial';
          const long = d.toLocaleDateString(lang, { weekday: 'long', day: 'numeric' });
          return (
            <button
              key={date}
              type="button"
              className={clsx(s.day, isToday && s.dayToday, date === day && s.daySelected)}
              aria-pressed={date === day}
              aria-current={isToday ? 'date' : undefined}
              aria-label={total ? t('kid.week.dayLabel', { day: long, done, total }) : t('kid.week.dayEmpty', { day: long })}
              onClick={() => setSelected(date)}
            >
              <span className={s.dayName} aria-hidden>{d.toLocaleDateString(lang, { weekday: 'short' })}</span>
              <DayMark kind={kind} pct={total ? (done / total) * 100 : 0} />
              <span className={s.dayNum} aria-hidden>{d.getDate()}</span>
            </button>
          );
        })}
      </div>

      <section className={s.streak} aria-label={t('kid.week.streak')}>
        <span className={s.streakIcon}><Icon name="flame" fill size={26} /></span>
        <div className={s.streakMain}>
          <span className={s.streakNum}>{t('kid.week.streakDays', { count: streak?.current_streak ?? 0 })}</span>
          <span className={s.streakSub}>{t('kid.week.best', { count: streak?.longest_streak ?? 0 })}</span>
        </div>
        {streak?.next_reward && (
          <p className={s.streakNext}>
            {t('kid.week.nextMilestone', {
              count: streak.next_reward.days_remaining,
              label: streak.next_reward.label,
              points: streak.next_reward.bonus_points,
            })}
          </p>
        )}
      </section>

      <h2 className={s.dayTitle}>{dayLabel}</h2>
      {user.paused ? (
        <PausedNotice />
      ) : data.weekChores == null ? null : dayChores.length === 0 ? (
        <Notice icon="cal" title={t('kid.week.emptyTitle')}>{t('kid.week.emptyText')}</Notice>
      ) : (
        <ChoreGroups
          chores={dayChores}
          viewOf={viewOf}
          togglingIds={data.togglingIds}
          tts={tts}
          onToggle={onToggle}
          onPhoto={onPhoto}
          onSpeak={onSpeak}
          headingLevel="h3"
        />
      )}
    </div>
  );
}
