import React, { useEffect, useId, useRef } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Icon } from '../../design';
import s from './Sheet.module.css';

export interface SheetProps {
  title: React.ReactNode;
  onClose: () => void;
  children: React.ReactNode;
  /** Shown left of the title, e.g. a back button inside a sub-view. */
  leading?: React.ReactNode;
  className?: string;
}

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * A dialog in the person's own skin: a bottom sheet on phones, a centred card
 * on wider screens. Rendered inline (no portal) so it stays inside the
 * SkinScope and picks up the skin's tokens and dials.
 */
export function Sheet({ title, onClose, children, leading, className }: SheetProps) {
  const { t } = useTranslation();
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    const returnTo = document.activeElement as HTMLElement | null;
    panelRef.current?.focus();
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== 'Tab' || !panelRef.current) return;
      // Keep focus inside the sheet.
      const items = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE))
        .filter(el => el.offsetParent !== null || el === document.activeElement);
      if (!items.length) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && (document.activeElement === first || document.activeElement === panelRef.current)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = prevOverflow;
      returnTo?.focus?.();
    };
  }, []);

  return (
    <div className={s.backdrop} onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div
        ref={panelRef}
        className={clsx(s.panel, className)}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
      >
        <div className={s.grip} aria-hidden />
        <header className={s.head}>
          {leading}
          <h2 id={titleId} className={s.title}>{title}</h2>
          <button type="button" className={s.close} onClick={onClose} aria-label={t('kid.sheet.close')}>
            <Icon name="plus" />
          </button>
        </header>
        <div className={s.body}>{children}</div>
      </div>
    </div>
  );
}
