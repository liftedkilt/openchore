// Small pieces shared by the Today, Week and Rewards screens.
import React from 'react';
import { useTranslation } from 'react-i18next';
import { Avatar, CategoryHeader, Icon, PointsChip, catFromCategory, isPersonColor, type Cat, type IconName } from '../../design';
import type { ScheduledChore, User } from '../../types';
import { isDone, type ChoreView } from './choreView';
import { ChoreItem } from './ChoreItem';
import s from './kid.module.css';

export const CAT_ORDER: readonly Cat[] = ['essential', 'daily', 'bonus', 'optional'];

/** The row at the top of every personal screen: me (opens settings), Manage, balance. */
export function TopBar({ user, balance, onMe, onManage }: {
  user: User;
  balance: number;
  onMe: () => void;
  /** Grown-ups only: jump to the admin screens. */
  onManage?: () => void;
}) {
  const { t, i18n } = useTranslation();
  const weekday = new Date().toLocaleDateString(i18n.language, { weekday: 'long' });
  return (
    <div className={s.topbar}>
      <button type="button" className={s.me} onClick={onMe} aria-label={t('kid.me.open', { name: user.name })}>
        <Avatar name={user.name} color={isPersonColor(user.color) ? user.color : null} src={user.avatar_url || null} />
        <span className={s.meDay}>{weekday}</span>
      </button>
      <div className={s.topbarEnd}>
        {onManage && (
          <button type="button" className={s.manage} onClick={onManage}>
            <Icon name="people" />
            <span>{t('kid.me.manage')}</span>
          </button>
        )}
        <PointsChip points={balance} />
      </div>
    </div>
  );
}

export interface ChoreGroupsProps {
  chores: ScheduledChore[];
  viewOf: (c: ScheduledChore) => ChoreView;
  togglingIds?: Set<number>;
  tts?: boolean;
  onToggle?: (c: ScheduledChore, view: ChoreView) => void;
  onPhoto?: (c: ScheduledChore) => void;
  onSpeak?: (text: string, audioUrl?: string) => void;
  headingLevel?: 'h2' | 'h3';
}

/** Chores under their category headers, always Must do → Every day → Bonus. */
export function ChoreGroups({ chores, viewOf, togglingIds, tts, onToggle, onPhoto, onSpeak, headingLevel = 'h2' }: ChoreGroupsProps) {
  const { t } = useTranslation();
  return (
    <>
      {CAT_ORDER.map(cat => {
        const list = chores.filter(c => catFromCategory(c.category) === cat);
        if (!list.length) return null;
        const views = list.map(viewOf);
        // Same rule as the day progress (a sibling's waiting take isn't done).
        const done = list.filter(isDone).length;
        const bonusPts = list.reduce((sum, c) => sum + (c.points_value || 0), 0);
        const count = cat === 'bonus'
          ? t('kid.today.bonusCount', { count: bonusPts })
          : t('design.progress.count', { done, total: list.length });
        return (
          <section key={cat} className={s.group}>
            <CategoryHeader cat={cat} count={count} as={headingLevel} />
            {list.map((c, i) => (
              <ChoreItem
                key={`${c.schedule_id}-${c.date}`}
                chore={c}
                view={views[i]}
                busy={togglingIds?.has(c.schedule_id)}
                tts={tts}
                onToggle={onToggle ? () => onToggle(c, views[i]) : undefined}
                onPhoto={onPhoto ? () => onPhoto(c) : undefined}
                onSpeak={onSpeak}
              />
            ))}
          </section>
        );
      })}
    </>
  );
}

/** "Add a task" + "My tasks", under the chore lists. */
export function OwnTaskEntry({ onAdd, onList }: { onAdd: () => void; onList: () => void }) {
  const { t } = useTranslation();
  return (
    <div className={s.ownEntry}>
      <button type="button" className={s.secondaryAction} onClick={onAdd}>
        <Icon name="plus" /> {t('kid.own.add')}
      </button>
      <button type="button" className={s.linkBtn} onClick={onList}>{t('kid.own.myTasks')}</button>
    </div>
  );
}

/** A quiet centred message: empty lists, breaks, nothing to redeem. */
export function Notice({ icon, title, children }: { icon: IconName; title: string; children?: React.ReactNode }) {
  return (
    <div className={s.notice}>
      <span className={s.noticeWell}><Icon name={icon} size={28} /></span>
      <h2 className={s.noticeTitle}>{title}</h2>
      {children && <p className={s.noticeText}>{children}</p>}
    </div>
  );
}

/** A paused profile: chores are off, nothing decays. */
export function PausedNotice() {
  const { t } = useTranslation();
  return <Notice icon="moon" title={t('kid.paused.title')}>{t('kid.paused.text')}</Notice>;
}
