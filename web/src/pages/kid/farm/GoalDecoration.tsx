// Celebratory farm decoration shown on both linked families' farms when an
// agreed weekly team goal is reached: two non-stretching sky bunting swags
// (clear of sun/moon, winter pines, and barn lights) plus an accessible badge
// pill that expands upward when multiple families' goals are reached.
import { useEffect, useId, useRef, useState } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import type { GoalCelebration } from '../../../types';
import s from './farm.module.css';

export interface GoalDecorationProps {
  goals: readonly GoalCelebration[];
  compact?: boolean;
  /** True when rendering a visited friend family's farm. */
  friendFarm?: boolean;
}

const LEFT_FLAGS: ReadonlyArray<{ pts: string; cls: string; delay: string }> = [
  { pts: '8,32 28,33 17,49', cls: s.goalFlagA, delay: '0s' },
  { pts: '34,32 54,30 45,46', cls: s.goalFlagB, delay: '-0.7s' },
  { pts: '61,28 81,23 73,41', cls: s.goalFlagC, delay: '-1.4s' },
  { pts: '88,20 108,13 102,32', cls: s.goalFlagD, delay: '-2.1s' },
  { pts: '116,10 136,2 130,22', cls: s.goalFlagA, delay: '-2.8s' },
];

const RIGHT_FLAGS: ReadonlyArray<{ pts: string; cls: string; delay: string }> = [
  { pts: '18,6 38,14 24,28', cls: s.goalFlagC, delay: '-0.4s' },
  { pts: '44,16 64,20 52,36', cls: s.goalFlagD, delay: '-1.1s' },
  { pts: '70,21 90,21 80,38', cls: s.goalFlagB, delay: '-1.8s' },
  { pts: '96,20 116,16 108,36', cls: s.goalFlagA, delay: '-2.5s' },
  { pts: '122,14 142,6 136,28', cls: s.goalFlagC, delay: '-3.2s' },
];

function Sparkle({ cx, cy, r = 4.2, alt = false }: { cx: number; cy: number; r?: number; alt?: boolean }) {
  const i = r * 0.32;
  const d = [
    `M${cx} ${cy - r}`,
    `L${cx + i} ${cy - i}`,
    `L${cx + r} ${cy}`,
    `L${cx + i} ${cy + i}`,
    `L${cx} ${cy + r}`,
    `L${cx - i} ${cy + i}`,
    `L${cx - r} ${cy}`,
    `L${cx - i} ${cy - i}Z`,
  ].join(' ');
  return <path d={d} className={clsx(s.goalSparkle, alt && s.goalSparkleAlt)} />;
}

function GoalStarIcon() {
  return (
    <svg className={s.goalPillIcon} viewBox="0 0 16 16" aria-hidden="true">
      <path
        d="M8 1.2l1.95 4.02 4.43.62-3.22 3.12.78 4.41L8 11.27 4.06 13.37l.78-4.41L1.62 5.84l4.43-.62Z"
        fill="var(--star)"
        stroke="var(--ink)"
        strokeWidth="1.2"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function GoalDecoration({ goals, compact = false, friendFarm = false }: GoalDecorationProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const badgeRef = useRef<HTMLDivElement>(null);
  const listId = useId();
  const multi = !compact && goals.length > 1;

  useEffect(() => {
    if (!multi && open) setOpen(false);
  }, [multi, open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    const onPointerDown = (e: PointerEvent) => {
      if (badgeRef.current && !badgeRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('pointerdown', onPointerDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('pointerdown', onPointerDown);
    };
  }, [open]);

  if (goals.length === 0) return null;

  const first = goals[0];
  const summaryList = goals.map(g => t('kid.farm.goal.item', { family: g.family, target: g.target })).join(', ');
  const ariaLabel = goals.length === 1
    ? friendFarm
      ? t('kid.farm.goal.ariaFriend', { family: first.family, target: first.target })
      : t('kid.farm.goal.ariaOne', { family: first.family, target: first.target })
    : t('kid.farm.goal.ariaMany', { count: goals.length, list: summaryList });

  const pillText = compact
    ? t('kid.farm.goal.badgeCompact', { count: goals.length })
    : goals.length === 1
      ? friendFarm
        ? t('kid.farm.goal.badgeFriend', { family: first.family, target: first.target })
        : t('kid.farm.goal.badgeOne', { family: first.family, target: first.target })
      : t('kid.farm.goal.badgeMany', { count: goals.length });

  return (
    <>
      <svg
        className={s.goalBuntingLeft}
        viewBox="0 0 160 56"
        preserveAspectRatio="xMinYMin meet"
        aria-hidden="true"
      >
        <path className={s.goalCord} d="M-4 32 Q68 38 152 -2" />
        {LEFT_FLAGS.map((f, idx) => (
          <polygon
            key={idx}
            points={f.pts}
            className={clsx(s.goalFlag, f.cls)}
            style={{ animationDelay: f.delay }}
          />
        ))}
        <Sparkle cx={38} cy={12} r={4.2} />
        <Sparkle cx={114} cy={24} r={3.4} alt />
      </svg>

      <svg
        className={s.goalBuntingRight}
        viewBox="0 0 160 52"
        preserveAspectRatio="xMidYMin meet"
        aria-hidden="true"
      >
        <path className={s.goalCord} d="M4 -2 Q80 30 156 -2" />
        {RIGHT_FLAGS.map((f, idx) => (
          <polygon
            key={idx}
            points={f.pts}
            className={clsx(s.goalFlag, f.cls)}
            style={{ animationDelay: f.delay }}
          />
        ))}
        <Sparkle cx={26} cy={26} r={3.4} alt />
        <Sparkle cx={132} cy={24} r={4} />
      </svg>

      <div className={s.goalBadge} ref={badgeRef}>
        {multi && open && (
          <ul id={listId} className={s.goalPopover} role="list">
            {goals.map(g => (
              <li key={g.link_id} className={s.goalPopoverItem}>
                <GoalStarIcon />
                <span>{t('kid.farm.goal.detail', { family: g.family, target: g.target })}</span>
              </li>
            ))}
          </ul>
        )}

        {multi ? (
          <button
            type="button"
            className={clsx(s.goalPill, s.goalPillBtn)}
            aria-expanded={open}
            aria-controls={listId}
            aria-label={ariaLabel}
            title={ariaLabel}
            data-testid="farm-goal-decoration"
            onClick={() => setOpen(o => !o)}
          >
            <GoalStarIcon />
            <span className={s.goalPillText}>{pillText}</span>
            <span className={s.goalCaret} aria-hidden="true">{open ? '▾' : '▴'}</span>
          </button>
        ) : (
          <div
            className={s.goalPill}
            role="note"
            aria-label={ariaLabel}
            title={ariaLabel}
            data-testid="farm-goal-decoration"
          >
            <GoalStarIcon />
            <span className={s.goalPillText}>{pillText}</span>
          </div>
        )}
      </div>
    </>
  );
}
