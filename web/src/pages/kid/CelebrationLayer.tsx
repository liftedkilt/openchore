import { useEffect, useRef } from 'react';
import { Celebration } from '../../design';
import s from './kid.module.css';

/** How long the celebration stays before it steps aside on its own. */
export const CELEBRATION_MS = 3200;

interface CelebrationLayerProps {
  points: number;
  title: string;
  streak?: number;
  next?: string;
  onNext: () => void;
  onBack: () => void;
}

/**
 * The moment after a chore is ticked, over the whole screen. It never blocks:
 * a tap anywhere, Escape or a few seconds' wait returns to the list, and
 * focus goes back where it was.
 */
export function CelebrationLayer({ points, title, streak, next, onNext, onBack }: CelebrationLayerProps) {
  const ref = useRef<HTMLDivElement>(null);
  const onBackRef = useRef(onBack);
  onBackRef.current = onBack;

  useEffect(() => {
    const returnTo = document.activeElement as HTMLElement | null;
    // Keyboard users land on the way back; the tap target stays where it was.
    const buttons = ref.current?.querySelectorAll<HTMLButtonElement>('button');
    buttons?.[buttons.length - 1]?.focus({ preventScroll: true });
    let timer = setTimeout(() => onBackRef.current(), CELEBRATION_MS);
    // Someone reading or tabbing through it gets to finish.
    const hold = () => {
      clearTimeout(timer);
      timer = setTimeout(() => onBackRef.current(), CELEBRATION_MS * 2);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onBackRef.current();
      else if (e.key === 'Tab') hold();
    };
    document.addEventListener('keydown', onKey);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('keydown', onKey);
      if (returnTo && document.contains(returnTo)) returnTo.focus({ preventScroll: true });
    };
  }, []);

  return (
    <div
      ref={ref}
      className={s.celebrate}
      onClick={(e) => {
        // Taps on the buttons do their own thing; anywhere else goes back.
        if (!(e.target as HTMLElement).closest('button')) onBack();
      }}
    >
      <Celebration points={points} title={title} streak={streak || undefined} next={next} onNext={onNext} onBack={onBack} />
    </div>
  );
}
