// The weekly leaderboard: the share of this week's chore points each person
// has earned, with the people friend families share mixed in. It's a share,
// never a points total (every family prices chores differently); see
// docs/friends.md.
import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { api } from '../../../api';
import { Icon } from '../../../design';
import type { FriendFarm, Leaderboard, LeaderboardEntry, Pet, User } from '../../../types';
import { Sheet } from '../Sheet';
import { FarmAnimal, petPicture } from '../farm/FarmAnimal';
import { FarmScene } from '../farm/FarmScene';
import fs from '../farm/petSheet.module.css';
import s from './board.module.css';

const isMe = (e: LeaderboardEntry, user: User) => e.user_id === user.id;

function Row({ e, user, compact }: { e: LeaderboardEntry; user: User; compact?: boolean }) {
  const { t } = useTranslation();
  const me = isMe(e, user);
  const counted = e.assigned > 0;
  return (
    <li
      className={clsx(s.row, me && s.me, e.rank === 0 && s.unranked)}
      style={{ ['--who' as string]: e.color ? `var(--person-${e.color})` : undefined }}
    >
      <span className={s.rank} aria-label={e.rank ? t('kid.board.place', { rank: e.rank }) : t('kid.board.notRanked')}>
        {e.rank || '–'}
      </span>
      <span className={s.who}>
        <span className={s.name}>
          {e.name}
          {me && <span className={s.youTag}>{t('kid.board.you')}</span>}
        </span>
        {(e.family || (!compact && (e.streak > 1 || e.paused))) && (
          <span className={s.sub}>
            {e.family}
            {!compact && e.paused && <span>{t('kid.board.paused')}</span>}
            {!compact && e.streak > 1 && (
              <span className={s.streak}><Icon name="flame" /> {t('kid.board.streak', { count: e.streak })}</span>
            )}
          </span>
        )}
      </span>
      <span className={s.score}>
        {/* The share of chore points earned, as a score out of 100. */}
        <span className={s.percent}>
          {counted ? <><span className={s.unit}>{t('kid.board.scoreLabel')}</span> {e.percent}</> : '–'}
        </span>
        <span className={s.count}>
          {counted ? t('kid.board.score', { done: e.done, assigned: e.assigned }) : t('kid.board.noneYet')}
          {e.bonus > 0 && <span className={s.bonus}> · {t('kid.board.bonus', { count: e.bonus })}</span>}
        </span>
      </span>
      <span className={s.bar} aria-hidden="true"><span style={{ width: `${counted ? e.percent : 0}%` }} /></span>
    </li>
  );
}

/** The card on Today: the top three, plus you if you're further down. */
export function LeaderboardCard({ board, user, onOpen }: { board: Leaderboard; user: User; onOpen: () => void }) {
  const { t } = useTranslation();
  if (!board.enabled || board.entries.length < 2) return null;
  const top = board.entries.slice(0, 3);
  const mine = board.entries.find(e => isMe(e, user));
  const rows = mine && !top.includes(mine) ? [...top, mine] : top;
  return (
    <section className={s.card} aria-labelledby="today-board">
      <div className={fs.farmCardHead}>
        <h2 id="today-board" className={fs.farmCardTitle}>{t('kid.board.title')}</h2>
        <button type="button" className={fs.farmCardOpen} onClick={onOpen}>{t('kid.board.open')}</button>
      </div>
      <ol className={s.list}>
        {rows.map(e => <Row key={e.key} e={e} user={user} compact />)}
      </ol>
    </section>
  );
}

function asOf(iso: string | undefined, lang: string) {
  if (!iso) return '';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString(lang, { weekday: 'short', hour: 'numeric', minute: '2-digit' });
}

/** A friend family's farm: look, don't touch. */
function FriendFarmView({ farm }: { farm: FriendFarm }) {
  const { t, i18n } = useTranslation();
  const [selected, setSelected] = useState<Pet | null>(null);
  const when = asOf(farm.snapshot_at, i18n.language);
  return (
    <>
      <FarmScene pets={farm.pets} onSelect={setSelected} />
      {selected && (
        <div className={fs.otherCard} role="status">
          <FarmAnimal species={selected.species} color={selected.color} mood={selected.mood} accessory={selected.accessory} {...petPicture(selected)} className={fs.otherArt} />
          <p>
            <strong>{t('kid.farm.belongsTo', { name: selected.name, owner: selected.owner_name ?? '' })}</strong>
            <span>{t(`kid.farm.mood.${selected.mood}`, { name: selected.name })}</span>
          </p>
        </div>
      )}
      {farm.pets.length === 0 && <p className={s.help}>{t('kid.board.farmEmpty')}</p>}
      {when && <p className={s.help}>{t('kid.board.asOf', { when })}</p>}
    </>
  );
}

export function LeaderboardSheet({ user, initial, onClose }: { user: User; initial: Leaderboard | null; onClose: () => void }) {
  const { t } = useTranslation();
  const [week, setWeek] = useState<'this' | 'last'>('this');
  const [board, setBoard] = useState<Leaderboard | null>(initial);
  const [farms, setFarms] = useState<FriendFarm[]>([]);
  const [visiting, setVisiting] = useState<number | null>(null);

  useEffect(() => {
    let live = true;
    api.leaderboard.get(week).then(b => { if (live) setBoard(b); }).catch(() => {});
    return () => { live = false; };
  }, [week]);
  useEffect(() => {
    api.friends.farms().then(r => setFarms(r.farms)).catch(() => {});
  }, []);

  const farm = farms.find(f => f.link_id === visiting);
  if (farm) {
    return (
      <Sheet
        title={t('kid.board.farmOf', { family: farm.family })}
        onClose={onClose}
        className={fs.sheet}
        leading={
          <button type="button" className={s.back} onClick={() => setVisiting(null)} aria-label={t('kid.sheet.back')}>
            <Icon name="back" />
          </button>
        }
      >
        <FriendFarmView farm={farm} />
      </Sheet>
    );
  }

  const entries = board?.entries ?? [];
  const ranked = entries.filter(e => e.rank > 0);
  const waiting = entries.filter(e => e.rank === 0);
  return (
    <Sheet title={t('kid.board.sheetTitle')} onClose={onClose} className={s.sheet}>
      <div className={s.weeks} role="tablist" aria-label={t('kid.board.sheetTitle')}>
        {(['this', 'last'] as const).map(w => (
          <button key={w} type="button" role="tab" aria-selected={week === w} className={clsx(s.week, week === w && s.weekOn)} onClick={() => setWeek(w)}>
            {t(`kid.board.${w}Week`)}
          </button>
        ))}
      </div>

      {ranked.length > 0 && <ol className={s.list}>{ranked.map(e => <Row key={e.key} e={e} user={user} />)}</ol>}
      {ranked.length === 0 && <p className={s.help}>{t('kid.board.none')}</p>}

      {waiting.length > 0 && (
        <>
          <h3 className={s.heading}>{t('kid.board.notRanked')}</h3>
          <p className={s.help}>{t('kid.board.notRankedHelp', { count: board?.min_assigned ?? 5 })}</p>
          <ol className={s.list}>{waiting.map(e => <Row key={e.key} e={e} user={user} />)}</ol>
        </>
      )}

      <p className={s.help}>{t('kid.board.how')}</p>

      {farms.length > 0 && (
        <>
          <h3 className={s.heading}>{t('kid.board.farms')}</h3>
          <div className={s.farms}>
            {farms.map(f => (
              <button key={f.link_id} type="button" className={s.farmBtn} onClick={() => setVisiting(f.link_id)}>
                <span className={s.farmArt} aria-hidden="true">
                  {f.pets.slice(0, 3).map(p => (
                    <FarmAnimal key={p.id} species={p.species} color={p.color} mood={p.mood} accessory={p.accessory} {...petPicture(p)} />
                  ))}
                </span>
                <span className={s.farmName}>{f.family}</span>
                <span className={s.farmCount}>{t('kid.board.animals', { count: f.pets.length })}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </Sheet>
  );
}
