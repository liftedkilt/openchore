import React, { useRef } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { ChoreRow, Icon } from '../../design';
import type { ScheduledChore } from '../../types';
import type { ChoreView } from './choreView';
import s from './kid.module.css';

interface ChoreItemProps {
  chore: ScheduledChore;
  view: ChoreView;
  busy?: boolean;
  /** Read-aloud is on for this person. */
  tts?: boolean;
  onToggle?: () => void;
  onPhoto?: () => void;
  /** Speak text (or play a recorded clip). */
  onSpeak?: (text: string, audioUrl?: string) => void;
}

const SWIPE_DISTANCE = 100;

/** DOM id of a chore's row, for "Next" to scroll to it. */
export const choreDomId = (c: ScheduledChore) => `chore-${c.schedule_id}-${c.date}`;

/**
 * One chore on a person's screen: the design's ChoreRow, plus what the old
 * card also carried — the description, the photo AI's feedback, a camera
 * for adding photo proof, and swipe right to finish / left to undo.
 */
export function ChoreItem({ chore, view, busy, tts, onToggle, onPhoto, onSpeak }: ChoreItemProps) {
  const { t } = useTranslation();
  const wrapRef = useRef<HTMLDivElement>(null);
  const swipe = useRef<{ x: number; y: number; active: boolean; dragging: boolean }>({ x: 0, y: 0, active: false, dragging: false });

  const interactive = view.canToggle && view.state !== 'locked' && !!onToggle;
  const readOnly = !view.canToggle && view.state !== 'locked';
  const done = view.state === 'done' || view.state === 'waiting';

  const readAloud = () => {
    const text = chore.tts_description || (chore.title + (chore.description ? `. ${chore.description}` : ''));
    onSpeak?.(text, chore.tts_audio_url);
  };

  // --- Swipe: right to finish, left to undo (touch only) ---
  const onTouchStart = (e: React.TouchEvent) => {
    if (!interactive || busy) return;
    const p = e.touches[0];
    swipe.current = { x: p.clientX, y: p.clientY, active: true, dragging: false };
  };
  const onTouchMove = (e: React.TouchEvent) => {
    const st = swipe.current;
    const row = wrapRef.current?.firstElementChild as HTMLElement | null;
    if (!st.active || !row) return;
    const dx = e.touches[0].clientX - st.x;
    const dy = e.touches[0].clientY - st.y;
    // Vertical scrolling wins if it clearly dominates the first move.
    if (!st.dragging && Math.abs(dy) > Math.abs(dx) && Math.abs(dy) > 10) {
      st.active = false;
      return;
    }
    const dir = done ? -1 : 1;
    const travel = Math.max(0, Math.min(dx * dir, 120));
    if (travel > 10) {
      st.dragging = true;
      row.style.transform = `translateX(${travel * dir}px)`;
      wrapRef.current?.style.setProperty('--swipe', String(Math.min(travel / SWIPE_DISTANCE, 1)));
    }
  };
  const onTouchEnd = (e: React.TouchEvent) => {
    const st = swipe.current;
    const row = wrapRef.current?.firstElementChild as HTMLElement | null;
    if (!st.active) return;
    st.active = false;
    if (row) row.style.transform = '';
    wrapRef.current?.style.removeProperty('--swipe');
    const dx = e.changedTouches[0].clientX - st.x;
    if (st.dragging && dx * (done ? -1 : 1) >= SWIPE_DISTANCE) onToggle?.();
  };

  const row = (
    <ChoreRow
      id={choreDomId(chore)}
      cat={view.cat}
      icon={chore.icon}
      title={chore.title}
      meta={view.meta}
      points={view.points}
      state={view.state}
      urgent={view.urgent}
      photo={view.photo}
      readAloud={tts}
      onReadAloud={readAloud}
      onToggle={interactive ? onToggle : undefined}
      checkLabel={readOnly ? (done ? t('kid.chore.readonlyDone') : t('kid.chore.readonlyTodo')) : view.checkLabel}
      busy={busy}
      actions={view.needsPhotoProof && onPhoto ? (
        <button type="button" className={s.rowAction} onClick={onPhoto} aria-label={t('kid.chore.uploadPhotoProof')}>
          <Icon name="camera" />
        </button>
      ) : undefined}
    />
  );

  return (
    <div
      ref={wrapRef}
      className={clsx(s.item, interactive && s.itemSwipe)}
      data-swipe={interactive ? (done ? 'undo' : 'done') : undefined}
      onTouchStart={onTouchStart}
      onTouchMove={onTouchMove}
      onTouchEnd={onTouchEnd}
    >
      {readOnly ? <fieldset disabled className={s.readonly}>{row}</fieldset> : row}
      {chore.description && view.state !== 'done' && (
        <p className={s.caption}>{chore.description}</p>
      )}
      {view.note && (
        <div className={s.note} data-kind={view.note.kind} role={view.note.kind === 'rejected' ? 'alert' : undefined}>
          <Icon name={view.note.kind === 'rejected' ? 'camera' : 'spark'} />
          <span className={s.noteText}>{view.note.text}</span>
          <button
            type="button"
            className={s.noteListen}
            onClick={() => onSpeak?.(view.note!.text, view.note!.audioUrl)}
            aria-label={t('kid.chore.listenToFeedback')}
          >
            <Icon name="sound" />
          </button>
        </div>
      )}
    </div>
  );
}
