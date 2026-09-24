import React from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Icon } from './Icon';
import { resolveChoreIcon } from './choreIcon';
import type { Cat, ChoreState } from './types';

export interface ChoreRowProps {
  /** essential (Must do), daily (Every day) or bonus. */
  cat?: Cat;
  /** A line icon name, or the chore's DB icon (emoji); resolved via resolveChoreIcon. */
  icon?: string | null;
  title: string;
  /** A time window, a countdown or a status line. Waiting/locked have defaults. */
  meta?: React.ReactNode;
  /** Appended to the meta line as "· 10 pts". */
  points?: number;
  state?: ChoreState;
  /** Sets the meta in `urgent`. */
  urgent?: boolean;
  /** Adds a camera to the meta line. */
  photo?: boolean;
  /** Adds a speaker button for young readers (hidden once done). */
  readAloud?: boolean;
  /** Tap on the check. Not called while locked or busy. */
  onToggle?: () => void;
  onReadAloud?: () => void;
  /** Override the check's accessible name (defaults: "Mark complete" / "Mark incomplete"). */
  checkLabel?: string;
  /** A toggle is in flight: the check is disabled and dimmed. */
  busy?: boolean;
  /** Extra controls (e.g. a photo upload button) placed before the check. */
  actions?: React.ReactNode;
  className?: string;
  id?: string;
}

/**
 * One chore: icon well, title, meta line and a check on the right. The root
 * keeps a class containing `choreCard` and the check is a real button with
 * aria-label "Mark complete" / "Mark incomplete", which the e2e suite relies on.
 */
export function ChoreRow({
  cat = 'daily', icon, title, meta, points, state = 'todo', urgent, photo, readAloud,
  onToggle, onReadAloud, checkLabel, busy, actions, className, id,
}: ChoreRowProps) {
  const { t } = useTranslation();
  const iconName = resolveChoreIcon(icon, cat);
  const checked = state === 'done' || state === 'waiting';
  const locked = state === 'locked';

  let metaLine: React.ReactNode;
  if (state === 'waiting') {
    metaLine = <span className="oc-chore__waiting">{meta ?? t('design.chore.waiting')}</span>;
  } else if (locked) {
    metaLine = meta ?? t('design.chore.locked');
  } else {
    const pts = points != null ? t('design.chore.points', { count: points }) : null;
    if (meta != null || pts || photo) {
      metaLine = (
        <>
          {photo && <Icon name="camera" />}
          {meta != null && (urgent ? <span className="oc-chore__urgent">{meta}</span> : <span>{meta}</span>)}
          {pts && <span>{meta != null ? `· ${pts}` : pts}</span>}
        </>
      );
    }
  }

  const label = checkLabel ?? (locked
    ? t('design.chore.lockedLabel')
    : checked ? t('design.chore.markIncomplete') : t('design.chore.markComplete'));

  return (
    <div
      id={id}
      className={clsx('oc-chore', 'oc-choreCard', `oc-chore--${state}`, busy && 'oc-chore--busy', className)}
      data-cat={cat}
      data-state={state}
    >
      <span className="oc-chore__well">
        <Icon name={iconName} />
      </span>
      <span className="oc-chore__txt">
        <span className="oc-chore__title">{title}</span>
        {metaLine != null && <span className="oc-chore__meta">{metaLine}</span>}
      </span>
      {readAloud && state !== 'done' && (
        <button type="button" className="oc-chore__say" onClick={onReadAloud} aria-label={t('design.chore.readAloud', { title })}>
          <span className="oc-chore__say-ring"><Icon name="sound" /></span>
        </button>
      )}
      {actions && <span className="oc-chore__actions">{actions}</span>}
      <button
        type="button"
        className="oc-chore__check"
        aria-label={label}
        disabled={locked || busy}
        onClick={locked || busy ? undefined : onToggle}
      >
        <span className="oc-chore__ring">
          {state === 'done' && <Icon name="check" />}
          {locked && <Icon name="lock" />}
        </span>
      </button>
    </div>
  );
}
