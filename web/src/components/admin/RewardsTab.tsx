import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Trash2, Pencil, X } from 'lucide-react';
import { api } from '../../api';
import type { User, Reward, StreakRewardItem } from '../../types';
import { Avatar, Icon } from '../../design';
import { IconPicker, IconWell, personColor } from './pickers';
import ui from './ui.module.css';
import styles from './RewardsTab.module.css';

const RewardAssignmentEditor: React.FC<{
  reward: Reward;
  users: User[];
  onSave: () => void;
}> = ({ reward, users, onSave }) => {
  const { t } = useTranslation();
  const [assignments, setAssignments] = useState<{ user_id: number; custom_cost: string; enabled: boolean }[]>(
    users.map(u => {
      const existing = reward.assignments?.find(a => a.user_id === u.id);
      return {
        user_id: u.id,
        custom_cost: existing?.custom_cost?.toString() || '',
        enabled: !!existing,
      };
    })
  );
  const [saving, setSaving] = useState(false);

  const toggle = (userId: number) => {
    setAssignments(prev => prev.map(a => a.user_id === userId ? { ...a, enabled: !a.enabled } : a));
  };

  const setCost = (userId: number, val: string) => {
    setAssignments(prev => prev.map(a => a.user_id === userId ? { ...a, custom_cost: val } : a));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const enabled = assignments.filter(a => a.enabled);
      // If all kids are enabled with no custom costs, clear assignments (= available to all)
      const allEnabled = enabled.length === users.length && enabled.every(a => !a.custom_cost);
      const payload = allEnabled ? [] : enabled.map(a => ({
        user_id: a.user_id,
        custom_cost: a.custom_cost ? parseInt(a.custom_cost) : undefined,
      }));
      await api.rewards.setAssignments(reward.id, payload);
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  const anyAssigned = assignments.some(a => a.enabled);

  return (
    <div className={clsx(ui.inset, styles.assign)}>
      <p className={ui.help}>
        {anyAssigned ? t('admin.rewardsTab.assignmentHintRestricted') : t('admin.rewardsTab.assignmentHintAll')}
      </p>
      {assignments.map(a => {
        const user = users.find(u => u.id === a.user_id);
        if (!user) return null;
        return (
          <div key={a.user_id} className={styles.assignRow}>
            <label className={ui.check}>
              <input type="checkbox" checked={a.enabled} onChange={() => toggle(a.user_id)} />
              <Avatar name={user.name} color={personColor(user)} size="sm" />
              <span className={ui.checkLabel}>{user.name}</span>
            </label>
            {a.enabled && (
              <label className={styles.assignCost}>
                <input
                  className={ui.input}
                  type="number"
                  min="1"
                  value={a.custom_cost}
                  onChange={e => setCost(a.user_id, e.target.value)}
                  placeholder={t('admin.rewardsTab.customCostPlaceholder', { cost: reward.cost })}
                  aria-label={t('admin.rewardsTab.customCostLabel', { name: user.name })}
                />
                <span>{t('admin.rewardsTab.ptsLabel')}</span>
              </label>
            )}
          </div>
        );
      })}
      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnPrimary} onClick={handleSave} disabled={saving}>
          <Icon name="check" /> {t('admin.rewardsTab.saveAssignments')}
        </button>
      </div>
    </div>
  );
};

const RewardForm: React.FC<{
  reward: Reward | null;
  onSave: () => void;
  onCancel: () => void;
}> = ({ reward, onSave, onCancel }) => {
  const { t } = useTranslation();
  const [name, setName] = useState(reward?.name || '');
  const [description, setDescription] = useState(reward?.description || '');
  const [icon, setIcon] = useState(reward?.icon || '');
  const [cost, setCost] = useState(reward?.cost?.toString() || '50');
  const [stock, setStock] = useState(reward?.stock?.toString() || '');
  const [shareable, setShareable] = useState(reward?.shareable ?? false);
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const data = {
        name,
        description,
        icon,
        cost: parseInt(cost) || 0,
        stock: stock ? parseInt(stock) : undefined,
        active: true,
        shareable,
      };
      if (reward) {
        await api.rewards.update(reward.id, data);
      } else {
        await api.rewards.create(data);
      }
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <form className={clsx(ui.card, ui.formGrid)} onSubmit={handleSubmit}>
      <div className={ui.sectionHead}>
        <h3 className={ui.sectionTitle}>{reward ? t('admin.rewardsTab.editRewardTitle') : t('admin.rewardsTab.newRewardTitle')}</h3>
        <button type="button" className={ui.iconBtn} onClick={onCancel} aria-label={t('admin.rewardsTab.cancel')}><X aria-hidden /></button>
      </div>

      <label className={ui.field}>
        <span className={ui.label}>{t('admin.rewardsTab.fieldName')}</span>
        <input className={ui.input} value={name} onChange={e => setName(e.target.value)} required placeholder={t('admin.rewardsTab.fieldNamePlaceholder')} />
      </label>

      <IconPicker value={icon} onChange={setIcon} cat="bonus" label={t('admin.rewardsTab.fieldIcon')} />

      <label className={ui.field}>
        <span className={ui.label}>{t('admin.rewardsTab.fieldDescription')}</span>
        <input className={ui.input} value={description} onChange={e => setDescription(e.target.value)} placeholder={t('admin.rewardsTab.fieldDescriptionPlaceholder')} />
      </label>

      <div className={ui.formRow}>
        <label className={ui.field}>
          <span className={ui.label} title={t('admin.rewardsTab.fieldCostTitle')}>{t('admin.rewardsTab.fieldCost')}</span>
          <input className={ui.input} type="number" min="1" value={cost} onChange={e => setCost(e.target.value)} />
        </label>
        <label className={ui.field}>
          <span className={ui.label} title={t('admin.rewardsTab.fieldStockTitle')}>{t('admin.rewardsTab.fieldStock')}</span>
          <input className={ui.input} type="number" min="0" value={stock} onChange={e => setStock(e.target.value)} placeholder="∞" />
        </label>
      </div>

      <label className={ui.check} title={t('admin.rewardsTab.fieldShareableTitle')}>
        <input type="checkbox" checked={shareable} onChange={e => setShareable(e.target.checked)} />
        <span className={ui.checkLabel}>{t('admin.rewardsTab.fieldShareableLabel')}</span>
      </label>

      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnGhost} onClick={onCancel}>{t('admin.rewardsTab.cancel')}</button>
        <button type="submit" className={ui.btnPrimary} disabled={saving || !name || !cost}>
          <Icon name="check" /> {reward ? t('admin.rewardsTab.update') : t('admin.rewardsTab.create')}
        </button>
      </div>
    </form>
  );
};

const StreakRewardForm: React.FC<{ onSave: () => void; onCancel: () => void }> = ({ onSave, onCancel }) => {
  const { t } = useTranslation();
  const [days, setDays] = useState('7');
  const [points, setPoints] = useState('25');
  const [label, setLabel] = useState('');
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.streaks.createReward({
        streak_days: parseInt(days) || 0,
        bonus_points: parseInt(points) || 0,
        label: label || `${days}-Day Streak!`,
      });
      onSave();
    } catch (err) {
      console.error(err);
    }
    setSaving(false);
  };

  return (
    <form className={clsx(ui.card, ui.formGrid)} onSubmit={handleSubmit}>
      <div className={ui.formRow}>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.rewardsTab.streakFieldDays')}</span>
          <input className={ui.input} type="number" min="1" value={days} onChange={e => setDays(e.target.value)} />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.rewardsTab.streakFieldBonusPts')}</span>
          <input className={ui.input} type="number" min="1" value={points} onChange={e => setPoints(e.target.value)} />
        </label>
        <label className={ui.field}>
          <span className={ui.label}>{t('admin.rewardsTab.streakFieldLabel')}</span>
          <input className={ui.input} value={label} onChange={e => setLabel(e.target.value)} placeholder={t('admin.rewardsTab.streakFieldLabelPlaceholder')} />
        </label>
      </div>
      <div className={ui.actionsEnd}>
        <button type="button" className={ui.btnGhost} onClick={onCancel}>{t('admin.rewardsTab.cancel')}</button>
        <button type="submit" className={ui.btnPrimary} disabled={saving || !days || !points}>
          <Icon name="check" /> {t('admin.rewardsTab.addMilestone')}
        </button>
      </div>
    </form>
  );
};

export const RewardsTab: React.FC = () => {
  const { t } = useTranslation();
  const [rewards, setRewards] = useState<Reward[]>([]);
  const [streakRewards, setStreakRewards] = useState<StreakRewardItem[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingReward, setEditingReward] = useState<Reward | null>(null);
  const [showStreakForm, setShowStreakForm] = useState(false);
  const [expandedAssignments, setExpandedAssignments] = useState<number | null>(null);

  const load = useCallback(async () => {
    const [r, sr, u] = await Promise.all([api.rewards.listAll(), api.streaks.listRewards(), api.users.list()]);
    setRewards(r);
    setStreakRewards(sr);
    setUsers(u);
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleDeleteReward = async (id: number) => {
    await api.rewards.delete(id);
    load();
  };

  const handleDeleteStreakReward = async (id: number) => {
    await api.streaks.deleteReward(id);
    load();
  };

  const toggleAssignments = (id: number) => {
    setExpandedAssignments(expandedAssignments === id ? null : id);
  };

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.rewardsTab.rewardsStoreTitle')}</h2>
          <p className={ui.pageSub}>{t('admin.rewardsTab.subtitle')}</p>
        </div>
        <button type="button" className={ui.btnPrimary} onClick={() => { setEditingReward(null); setShowForm(true); }}>
          <Icon name="plus" /> {t('admin.rewardsTab.addReward')}
        </button>
      </div>

      {showForm && (
        <RewardForm
          key={editingReward?.id ?? 'new'}
          reward={editingReward}
          onSave={() => { setShowForm(false); setEditingReward(null); load(); }}
          onCancel={() => { setShowForm(false); setEditingReward(null); }}
        />
      )}

      {rewards.length === 0 ? (
        <p className={ui.emptyInline}>{t('admin.rewardsTab.noRewards')}</p>
      ) : (
        <div className={ui.list}>
          {rewards.map(r => {
            const assigned = (r.assignments ?? [])
              .map(a => users.find(u => u.id === a.user_id))
              .filter((u): u is User => !!u);
            const open = expandedAssignments === r.id;
            return (
              <div key={r.id} className={styles.item}>
                <div className={clsx(ui.row, !r.active && ui.rowMuted)}>
                  <IconWell icon={r.icon} cat="bonus" />
                  <div className={ui.rowMain}>
                    <span className={ui.rowTitle}>{r.name}</span>
                    {r.description && <span className={ui.rowDesc}>{r.description}</span>}
                    <span className={ui.rowMeta}>
                      <span><Icon name="star" /> {r.cost} {t('admin.rewardsTab.pts')}</span>
                      <span>{r.stock !== null && r.stock !== undefined ? t('admin.rewardsTab.inStock', { count: r.stock }) : t('admin.rewardsTab.unlimited')}</span>
                      {r.shareable && <span><Icon name="people" /> {t('admin.rewardsTab.shareable')}</span>}
                      {!r.active && <span className={ui.badgeOutline}>{t('admin.rewardsTab.inactive')}</span>}
                    </span>
                    <button type="button" className={styles.assignToggle} aria-expanded={open} onClick={() => toggleAssignments(r.id)}>
                      {assigned.length > 0 ? (
                        <span className={styles.stack} aria-hidden>
                          {assigned.slice(0, 5).map(u => <Avatar key={u.id} name={u.name} color={personColor(u)} size="sm" />)}
                        </span>
                      ) : <Icon name="people" />}
                      {assigned.length > 0
                        ? t('admin.rewardsTab.kidsAssigned', { count: assigned.length })
                        : t('admin.rewardsTab.allKids')}
                      <Icon name="chev" className={clsx(styles.caret, open && styles.caretOpen)} />
                    </button>
                  </div>
                  <div className={ui.rowActions}>
                    <button type="button" className={ui.iconBtn} aria-label={t('admin.rewardsTab.ariaEditReward')} title={t('admin.rewardsTab.ariaEditReward')} onClick={() => { setEditingReward(r); setShowForm(true); }}>
                      <Pencil aria-hidden />
                    </button>
                    <button type="button" className={clsx(ui.iconBtn, ui.iconBtnDanger)} aria-label={t('admin.rewardsTab.ariaDeleteReward')} title={t('admin.rewardsTab.ariaDeleteReward')} onClick={() => handleDeleteReward(r.id)}>
                      <Trash2 aria-hidden />
                    </button>
                  </div>
                </div>
                {open && (
                  <div className={styles.assignWrap}>
                    <RewardAssignmentEditor reward={r} users={users} onSave={load} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <section className={ui.section}>
        <div className={ui.pageHead}>
          <div>
            <h2 className={ui.pageTitle}>{t('admin.rewardsTab.streakMilestonesTitle')}</h2>
            <p className={ui.pageSub}>{t('admin.rewardsTab.streakSubtitle')}</p>
          </div>
          {!showStreakForm && (
            <button type="button" className={ui.btnPrimary} onClick={() => setShowStreakForm(true)}>
              <Icon name="plus" /> {t('admin.rewardsTab.addMilestone')}
            </button>
          )}
        </div>

        {showStreakForm && <StreakRewardForm onSave={() => { setShowStreakForm(false); load(); }} onCancel={() => setShowStreakForm(false)} />}

        {streakRewards.length === 0 ? (
          <p className={ui.emptyInline}>{t('admin.rewardsTab.noStreakMilestones')}</p>
        ) : (
          <div className={ui.list}>
            {streakRewards.map(sr => (
              <div key={sr.id} className={ui.row}>
                <span className={styles.streakBadge}>
                  <Icon name="flame" fill />
                  {t('admin.rewardsTab.streakDays', { count: sr.streak_days })}
                </span>
                <div className={ui.rowMain}>
                  <span className={ui.rowTitle}>{sr.label || `${sr.streak_days}-Day Streak`}</span>
                  <span className={ui.rowMeta}>
                    <span><Icon name="star" /> +{sr.bonus_points} {t('admin.rewardsTab.bonusPts')}</span>
                  </span>
                </div>
                <div className={ui.rowActions}>
                  <button type="button" className={clsx(ui.iconBtn, ui.iconBtnDanger)} aria-label={t('admin.rewardsTab.ariaDeleteStreakReward')} title={t('admin.rewardsTab.ariaDeleteStreakReward')} onClick={() => handleDeleteStreakReward(sr.id)}>
                    <Trash2 aria-hidden />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
};
