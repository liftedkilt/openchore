import React, { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';
import clsx from 'clsx';
import type { PersonColor, ThemeId } from '../../design';
import styles from './Modal.module.css';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  maxWidth?: string;
  /**
   * The theme to draw the dialog in. By default the modal takes the theme
   * (and person) of the scope it is rendered inside, even though it portals
   * to document.body. Pass this to force one, e.g. from a kid's SkinScope.
   */
  theme?: ThemeId;
  /** The person colour for a person's own skin (Tint lets it lead). */
  person?: PersonColor | null;
  /** Extra class on the dialog card. */
  className?: string;
}

interface Scope {
  theme?: string;
  person?: string;
}

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * A dialog on the theme of whatever opened it. It portals to document.body
 * (so no ancestor can clip it) and copies the nearest `data-theme` /
 * `data-person` onto its own root, so tokens and dials still apply.
 */
const Modal: React.FC<ModalProps> = ({ isOpen, onClose, title, children, maxWidth = '600px', theme, person, className }) => {
  const { t } = useTranslation();
  const anchorRef = useRef<HTMLSpanElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const [scope, setScope] = useState<Scope>({});
  const titleId = useId();
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Read the scope the modal was opened from, before the first paint.
  useLayoutEffect(() => {
    if (!isOpen) return;
    const themed = anchorRef.current?.closest<HTMLElement>('[data-theme]');
    const personed = anchorRef.current?.closest<HTMLElement>('[data-person]');
    setScope({ theme: themed?.dataset.theme, person: personed?.dataset.person });
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onCloseRef.current();
        return;
      }
      if (e.key !== 'Tab' || !cardRef.current) return;
      // Keep focus inside the dialog.
      const items = Array.from(cardRef.current.querySelectorAll<HTMLElement>(FOCUSABLE));
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && (document.activeElement === first || document.activeElement === cardRef.current)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', handleKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    // Focus the dialog unless something inside asked for it (autoFocus).
    if (cardRef.current && !cardRef.current.contains(document.activeElement)) {
      cardRef.current.focus({ preventScroll: true });
    }
    return () => {
      document.removeEventListener('keydown', handleKey);
      document.body.style.overflow = prevOverflow;
      previouslyFocused?.focus?.({ preventScroll: true });
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (cardRef.current && !cardRef.current.contains(e.target as Node)) {
      onClose();
    }
  };

  const dataTheme = theme ?? scope.theme;
  const dataPerson = person === undefined ? scope.person : person || undefined;

  return (
    <>
      <span ref={anchorRef} hidden />
      {createPortal(
        <div
          className={styles.overlay}
          data-theme={dataTheme}
          data-person={dataPerson}
          onClick={handleOverlayClick}
        >
          <div
            className={clsx(styles.card, className)}
            ref={cardRef}
            style={{ maxWidth }}
            role="dialog"
            aria-modal="true"
            aria-labelledby={title ? titleId : undefined}
            tabIndex={-1}
          >
            {title && (
              <div className={styles.header}>
                <h2 className={styles.title} id={titleId}>{title}</h2>
                <button type="button" className={styles.closeBtn} onClick={onClose} aria-label={t('admin.modal.close')}>
                  <X aria-hidden />
                </button>
              </div>
            )}
            <div className={styles.body}>{children}</div>
          </div>
        </div>,
        document.body,
      )}
    </>
  );
};

export default Modal;
