import React, { useId, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { Avatar, Button, Icon, isPersonColor, resolveChoreIcon, type PersonColor } from '../../design';
import type { PointTransaction, RewardCommitment, User } from '../../types';
import { daysToGoal, goalProgress } from './goals';
import s from './kid.module.css';

const rewardIcon = (icon: string | undefined, shared: boolean) => {
  const name = resolveChoreIcon(icon, 'bonus');
  return name === 'spark' ? (shared ? 'people' : 'gift') : name;
};

/** The saved amount as "43 / 5000 pts": one text node group, as tests read it. */
function Amounts({ saved, target }: { saved: number; target: number }) {
  const { t } = useTranslation();
  return <span className={s.goalAmounts}>{t('kid.goal.amounts', { saved, target })}</span>;
}

/** A progress bar. Shared pots show each saver's part in their own colour. */
function GoalBar({ c, me, people }: { c: RewardCommitment; me: User; people: User[] }) {
  const p = goalProgress(c);
  if (!p.shared || !c.pool?.contributors?.length || p.target <= 0) {
    return (
      <span className={s.bar} data-person={isPersonColor(me.color) ? me.color : undefined} aria-hidden>
        <i style={{ width: `${p.pct}%` }} />
      </span>
    );
  }
  return (
    <span className={s.bar} aria-hidden>
      {c.pool.contributors.filter(x => x.amount_saved > 0).map(x => {
        const color = people.find(u => u.id === x.user_id)?.color;
        return (
          <i
            key={x.user_id}
            data-person={isPersonColor(color) ? color : undefined}
            style={{ width: `${Math.min(100, (x.amount_saved / p.target) * 100)}%` }}
          />
        );
      })}
    </span>
  );
}

/** The goal nudge on Today: tap to open Rewards. */
export function GoalNudge({ c, me, people = [] }: { c: RewardCommitment; me: User; people?: User[] }) {
  const { t } = useTranslation();
  const p = goalProgress(c);
  return (
    <Link to="/rewards" className={clsx(s.goal, s.goalNudge)}>
      <span className={s.goalHead}>
        <span className={s.well}><Icon name={rewardIcon(c.reward_icon, p.shared)} /></span>
        <span className={s.goalTitles}>
          <span className={s.goalKicker}>{p.shared ? t('kid.goal.familyGoal') : t('kid.goal.savingToward')}</span>
          <span className={s.goalName}>{c.reward_name}</span>
        </span>
        <Icon name="chev" />
      </span>
      <GoalBar c={c} me={me} people={people} />
      <span className={s.goalFoot}>
        <Amounts saved={p.saved} target={p.target} />
        <span className={p.funded ? s.goalReady : undefined}>
          {p.funded ? t('kid.goal.ready') : t('kid.goal.toGo', { count: p.remaining })}
        </span>
      </span>
    </Link>
  );
}

interface GoalCardProps {
  c: RewardCommitment;
  me: User;
  people: User[];
  balance: number;
  transactions: PointTransaction[];
  busy: boolean;
  redeeming: boolean;
  onRedeem: () => void;
  onContribute: (amount: string) => Promise<boolean>;
  onAutoSave: (percent: number) => void;
  onBreak: () => void;
}

/** A savings goal (personal) or a family pot (shared), with every control. */
export function GoalCard({ c, me, people, balance, transactions, busy, redeeming, onRedeem, onContribute, onAutoSave, onBreak }: GoalCardProps) {
  const { t } = useTranslation();
  const p = goalProgress(c);
  const [amount, setAmount] = useState('');
  const sliderId = useId();
  const amountId = useId();
  const days = daysToGoal(c, transactions);

  const colorOf = (userId: number): PersonColor | null => {
    const col = people.find(u => u.id === userId)?.color;
    return isPersonColor(col) ? col : null;
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (await onContribute(amount)) setAmount('');
  };

  return (
    <article className={s.goal}>
      <div className={s.goalHead}>
        <span className={s.well}><Icon name={rewardIcon(c.reward_icon, p.shared)} /></span>
        <span className={s.goalTitles}>
          <span className={s.goalKicker}>{p.shared ? t('kid.goal.familyGoal') : t('kid.goal.savingToward')}</span>
          <h3 className={s.goalName}>{c.reward_name}</h3>
        </span>
        {p.funded && <span className={s.badge}>{t('kid.goal.ready')}</span>}
      </div>

      <GoalBar c={c} me={me} people={people} />
      <div className={s.goalFoot}>
        <Amounts saved={p.saved} target={p.target} />
        <span>
          {p.funded
            ? t('kid.goal.fullyFunded')
            : days != null
              ? t('kid.goal.pace', { count: days <= 13 ? days : Math.round(days / 7), context: days <= 13 ? 'days' : 'weeks' })
              : t('kid.goal.toGo', { count: p.remaining })}
        </span>
      </div>

      {p.shared && c.pool?.contributors && c.pool.contributors.length > 0 && (
        <ul className={s.savers} aria-label={t('kid.goal.savers')}>
          {c.pool.contributors.map(x => (
            <li key={x.user_id} className={s.saver}>
              <Avatar name={x.user_name} color={colorOf(x.user_id)} size="sm" />
              <span className={s.saverName}>{x.user_id === me.id ? t('kid.goal.you') : x.user_name}</span>
              <span className={s.saverAmt}>{t('kid.goal.saverAmount', { count: x.amount_saved })}</span>
            </li>
          ))}
        </ul>
      )}

      <div className={s.autoSave}>
        <label htmlFor={sliderId} className={s.autoSaveLabel}>
          {t('kid.goal.autoSave')}
          <small>{t('kid.goal.autoSaveHint')}</small>
        </label>
        <input
          id={sliderId}
          type="range"
          min={0}
          max={100}
          step={5}
          value={c.auto_contribute_percent}
          onChange={e => onAutoSave(parseInt(e.target.value, 10))}
          className={s.slider}
          aria-valuetext={`${c.auto_contribute_percent}%`}
        />
        <output htmlFor={sliderId} className={s.autoSaveValue}>{c.auto_contribute_percent}%</output>
      </div>

      {!p.funded && (
        <form className={s.addRow} onSubmit={submit}>
          <label htmlFor={amountId} className="oc-visually-hidden">{t('kid.goal.addPoints')}</label>
          <input
            id={amountId}
            type="number"
            inputMode="numeric"
            min={1}
            max={balance}
            placeholder={t('kid.goal.addPlaceholder', { max: balance })}
            value={amount}
            onChange={e => setAmount(e.target.value)}
            className={s.input}
          />
          <button type="submit" className={s.smallBtn} disabled={busy || !amount}>
            <Icon name="plus" />
            {t('kid.goal.save')}
          </button>
        </form>
      )}

      <div className={s.goalActions}>
        {p.funded && (
          <Button onClick={onRedeem} disabled={redeeming}>{t('kid.goal.redeemNow')}</Button>
        )}
        <Button variant="quiet" onClick={onBreak} disabled={busy}>
          {p.shared ? t('kid.goal.leave') : t('kid.goal.stop')}
        </Button>
      </div>
    </article>
  );
}
