import React from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Icon, resolveChoreIcon } from '../../design';
import type { Reward, User } from '../../types';
import { GoalCard } from './GoalCard';
import { Notice } from './parts';
import type { KidData } from './useKidData';
import s from './kid.module.css';

const rewardIcon = (icon?: string) => {
  const name = resolveChoreIcon(icon, 'bonus');
  return name === 'spark' ? 'gift' : name;
};

interface RewardsProps {
  user: User;
  data: KidData;
  topBar: React.ReactNode;
  onRedeemed: () => void;
}

export function RewardsScreen({ user, data, topBar, onRedeemed }: RewardsProps) {
  const { t, i18n } = useTranslation();
  const balance = data.points?.balance ?? 0;
  const committed = data.points?.committed ?? 0;
  const commitments = data.points?.active_commitments ?? [];
  const hasPersonalGoal = commitments.some(c => !c.shared_pool_id);
  const rewards = data.rewards;

  const redeem = async (reward: Reward) => {
    if (await data.redeem(reward)) onRedeemed();
  };

  return (
    <div className={s.rewards}>
      <div className={s.rewardsSide}>
        {topBar}
        <h1 className={s.screenTitle}>{t('kid.rewards.title')}</h1>
        <div className={s.balance}>
          <span className={s.balanceNum}>
            <Icon name="star" fill className={s.balanceStar} />
            <span>{balance}</span>
          </span>
          <span className={s.balanceSub}>
            {committed > 0 ? t('kid.rewards.spendableSaved', { count: balance, saved: committed }) : t('kid.rewards.spendable', { count: balance })}
          </span>
        </div>

        {commitments.length > 0 && (
          <section className={s.goals} aria-label={t('kid.goal.title')}>
            {commitments.map(c => (
              <GoalCard
                key={c.id}
                c={c}
                me={user}
                people={data.people}
                balance={balance}
                transactions={data.points?.transactions ?? []}
                busy={data.busyGoals.has(c.id)}
                redeeming={data.redeemingId === c.reward_id}
                onRedeem={() => {
                  const r = rewards?.find(x => x.id === c.reward_id);
                  if (r) redeem(r);
                }}
                onContribute={(amount) => data.contribute(c.id, amount)}
                onAutoSave={(pct) => data.setAutoContribute(c.id, pct)}
                onBreak={() => data.breakCommitment(c.id, !!c.pool)}
              />
            ))}
          </section>
        )}
      </div>

      <div className={s.rewardsMain}>
        <h2 className={s.sectionTitle}>{t('kid.rewards.shop')}</h2>
        {rewards == null ? null : rewards.length === 0 ? (
          <Notice icon="gift" title={t('kid.rewards.emptyTitle')}>{t('kid.rewards.emptyText')}</Notice>
        ) : (
          <ul className={s.rewardList}>
            {rewards.map(reward => {
              const commitment = data.commitmentFor(reward.id);
              const committedTo = !!commitment;
              const shared = reward.shareable;
              const target = commitment?.pool ? commitment.pool.target_cost : (commitment?.target_cost ?? reward.effective_cost);
              const saved = commitment?.pool ? commitment.pool.amount_saved : (commitment?.amount_saved ?? 0);
              const funded = committedTo && saved >= target;
              const canAfford = !shared && balance >= reward.effective_cost;
              const outOfStock = reward.stock != null && reward.stock <= 0;
              const redeeming = data.redeemingId === reward.id;
              const redeemed = data.redeemedId === reward.id;
              const savingToward = data.savingTowardId === reward.id;
              // Personal: offer "Save toward" when it's out of reach and there's
              // no personal goal yet. Shared: "Join" until you're in the pot.
              const showSaveToward = shared
                ? !committedTo && !outOfStock
                : !committedTo && !hasPersonalGoal && reward.effective_cost > balance && !outOfStock;
              const redeemEnabled = shared ? funded : committedTo ? funded : canAfford;
              const label = redeemed ? t('kid.rewards.redeemed')
                : redeeming ? t('kid.rewards.redeeming')
                : outOfStock ? t('kid.rewards.gone')
                : committedTo ? (funded ? t('kid.rewards.redeem') : t('kid.rewards.progress', { saved, target }))
                : canAfford ? t('kid.rewards.redeem')
                : shared ? t('kid.rewards.familyGoal')
                : t('kid.rewards.more', { count: reward.effective_cost - balance });
              const ready = redeemEnabled && !outOfStock;

              return (
                <li key={reward.id} className={s.reward}>
                  <span className={s.well}><Icon name={rewardIcon(reward.icon)} /></span>
                  <div className={s.rewardText}>
                    <h3 className={s.rewardName}>
                      {reward.name}
                      {shared && <span className={s.tag}><Icon name="people" />{t('kid.rewards.familyTag')}</span>}
                      {committedTo && !shared && <span className={s.tag}>{t('kid.rewards.goalTag')}</span>}
                    </h3>
                    {reward.description && <p className={s.rewardDesc}>{reward.description}</p>}
                    <p className={s.rewardMeta}>
                      <span className={s.cost}><Icon name="star" fill />{t('kid.rewards.cost', { count: reward.effective_cost })}</span>
                      {reward.stock != null && <span>{t('kid.rewards.stock', { count: reward.stock })}</span>}
                    </p>
                  </div>
                  <div className={s.rewardActions}>
                    <button
                      type="button"
                      className={clsx(s.redeem, ready && s.redeemReady, redeemed && s.redeemDone)}
                      disabled={!redeemEnabled || outOfStock || redeeming || redeemed}
                      onClick={() => redeem(reward)}
                    >
                      {redeemed && <Icon name="check" />}
                      {!ready && !redeemed && !committedTo && !outOfStock && <Icon name="lock" />}
                      {label}
                    </button>
                    {showSaveToward && (
                      <button
                        type="button"
                        className={s.saveToward}
                        onClick={() => data.saveToward(reward)}
                        disabled={savingToward}
                      >
                        {savingToward ? t('kid.rewards.redeeming') : shared ? t('kid.rewards.join') : t('kid.rewards.saveToward')}
                      </button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        {data.redemptions.length > 0 && (
          <section className={s.history} aria-labelledby="kid-history">
            <h2 id="kid-history" className={s.sectionTitle}>{t('kid.rewards.history')}</h2>
            <ul className={s.historyList}>
              {data.redemptions.map(r => (
                <li key={r.id} className={s.historyItem}>
                  <span className={s.wellSm}><Icon name={rewardIcon(r.reward_icon)} /></span>
                  <span className={s.historyName}>{r.reward_name}</span>
                  <span className={s.historyCost}>{t('kid.rewards.spent', { count: r.points_spent })}</span>
                  <time className={s.historyDate} dateTime={r.created_at}>
                    {new Date(r.created_at).toLocaleDateString(i18n.language, { month: 'short', day: 'numeric' })}
                  </time>
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </div>
  );
}
