import React from 'react';
import { useTranslation } from 'react-i18next';
import { DayProgress, Greeting, catFromCategory, salutationFor, type DayProgressItem } from '../../design';
import type { ScheduledChore, User } from '../../types';
import { gates, isDone, type ChoreView } from './choreView';
import { GoalNudge } from './GoalCard';
import { CAT_ORDER, ChoreGroups, Notice, OwnTaskEntry, PausedNotice } from './parts';
import type { KidData } from './useKidData';
import { FarmScene } from './farm/FarmScene';
import { LeaderboardCard } from './board/Leaderboard';
import s from './kid.module.css';
import fs from './farm/petSheet.module.css';

/** The day runs 7 am – 9 pm on the sun arc. */
const DAY_FROM = 7;
const DAY_TO = 21;
export const dayFraction = (d: Date) =>
  Math.min(1, Math.max(0, (d.getHours() + d.getMinutes() / 60 - DAY_FROM) / (DAY_TO - DAY_FROM)));

interface TodayProps {
  user: User;
  data: KidData;
  now: Date;
  topBar: React.ReactNode;
  viewOf: (c: ScheduledChore) => ChoreView;
  tts: boolean;
  onToggle: (c: ScheduledChore, view: ChoreView) => void;
  onPhoto: (c: ScheduledChore) => void;
  onSpeak: (text: string, audioUrl?: string) => void;
  /** Open "my own tasks": a new one for `date`, or the list. */
  onOwnTasks?: (date?: string) => void;
  /** Open the family farm. */
  onFarm?: () => void;
  /** Open the weekly leaderboard. */
  onBoard?: () => void;
}

export function TodayScreen({ user, data, now, topBar, viewOf, tts, onToggle, onPhoto, onSpeak, onOwnTasks, onFarm, onBoard }: TodayProps) {
  const { t } = useTranslation();
  const chores = (data.chores ?? []).filter(c => c.date === data.today);
  const firstName = user.name.split(' ')[0];

  // One mark per chore, in list order, so the shape row reads like the list.
  const items: DayProgressItem[] = CAT_ORDER.flatMap(cat =>
    chores.filter(c => catFromCategory(c.category) === cat).map(c => {
      const done = isDone(c);
      const at = done && c.completed_at ? new Date(c.completed_at) : null;
      return { cat, done, at: at && !Number.isNaN(at.getTime()) ? dayFraction(at) : undefined };
    }));
  const done = items.filter(i => i.done).length;
  // Optional chores never hold up "all done" (unless they're all there is).
  const expected = items.some(i => i.cat !== 'optional') ? items.filter(i => i.cat !== 'optional') : items;
  const allDone = expected.length > 0 && expected.every(i => i.done);

  // Points today, as the old stats showed them: Every day points wait until
  // every Must do is done.
  const { requiredDone } = gates(chores);
  let earned = 0;
  let pending = 0;
  for (const c of chores) {
    if (!c.completed) continue;
    if (c.category === 'core' && !requiredDone) pending += c.points_value || 0;
    else earned += c.points_value || 0;
  }

  const commitments = data.points?.active_commitments ?? [];
  const loading = data.chores == null;

  return (
    <div className={s.today}>
      <section className={s.todayHero} aria-label={t('kid.today.overview')}>
        {topBar}
        <Greeting salutation={t(`design.greeting.${salutationFor(now)}`)} name={firstName} />
        {!user.paused && items.length > 0 && (
          <div role="group" aria-label={t('design.progress.label', { done, total: items.length })} className={s.progress}>
            <DayProgress items={items} now={dayFraction(now)} />
          </div>
        )}
        {!user.paused && items.length > 0 && (
          <p className={s.todayLine}>
            {allDone ? <strong>{t('kid.today.allDone')}</strong> : null}
            {earned > 0 && <span>{t('kid.today.earned', { count: earned })}</span>}
            {pending > 0 && <span>{t('kid.today.pending', { count: pending })}</span>}
          </p>
        )}
      </section>

      <section className={s.todayLists} aria-label={t('kid.today.listLabel')}>
        {user.paused ? (
          <PausedNotice />
        ) : loading ? null : chores.length === 0 ? (
          <Notice icon="sprout" title={t('kid.today.emptyTitle')}>{t('kid.today.emptyText')}</Notice>
        ) : (
          <ChoreGroups
            chores={chores}
            viewOf={viewOf}
            togglingIds={data.togglingIds}
            tts={tts}
            onToggle={onToggle}
            onPhoto={onPhoto}
            onSpeak={onSpeak}
          />
        )}
        {!user.paused && !loading && onOwnTasks && (
          <OwnTaskEntry onAdd={() => onOwnTasks(data.today)} onList={() => onOwnTasks()} />
        )}
      </section>

      {data.farm?.enabled && onFarm && (
        <section className={fs.farmCard} aria-labelledby="today-farm">
          <div className={fs.farmCardHead}>
            <h2 id="today-farm" className={fs.farmCardTitle}>{t('kid.farm.title')}</h2>
            <button type="button" className={fs.farmCardOpen} onClick={onFarm}>
              {data.myPet ? t('kid.farm.open', { name: data.myPet.name }) : t('kid.farm.adoptCta')}
            </button>
          </div>
          {data.myPet && (
            <p className={fs.farmCardLine}>{t(`kid.farm.mood.${data.myPet.mood}`, { name: data.myPet.name })}</p>
          )}
          <FarmScene pets={[...data.farm.pets, ...data.visitors]} meId={user.id} compact onSelect={onFarm} />
        </section>
      )}

      {data.leaderboard && onBoard && <LeaderboardCard board={data.leaderboard} user={user} onOpen={onBoard} />}

      {/* Goals: under the list on a phone, under the hero on a tablet. */}
      {commitments.length > 0 && (
        <section className={s.nudges} aria-label={t('kid.goal.title')}>
          {commitments.map(c => <GoalNudge key={c.id} c={c} me={user} people={data.people} />)}
        </section>
      )}
    </div>
  );
}
