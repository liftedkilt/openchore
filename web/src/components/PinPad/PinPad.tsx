import React, { useCallback, useEffect, useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Icon } from '../../design';
import styles from './PinPad.module.css';

interface PinPadProps {
  /** Text shown above the dots (e.g. "Enter your PIN"). */
  prompt?: string;
  /** Optional error message to display under the dots. */
  error?: string;
  /** Called when the user has entered `length` digits. */
  onSubmit: (pin: string) => void;
  /** Length of the PIN, defaults to 4. */
  length?: number;
  /** When set, clears the current input (e.g. after the parent reports an error). */
  resetKey?: number;
}

const DIGITS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', 'del'];

/**
 * An on-screen PIN pad. It has no theme of its own: it reads the tokens and
 * dials of whatever scope it sits in (House, House Dark or a person's skin).
 */
export const PinPad: React.FC<PinPadProps> = ({ prompt, error, onSubmit, length = 4, resetKey }) => {
  const { t } = useTranslation();
  const [code, setCode] = useState('');
  const [shaking, setShaking] = useState(false);
  const promptId = useId();

  // Clear the input shortly after an error so the dots show what went wrong first.
  useEffect(() => {
    if (error) {
      setShaking(true);
      const timer = setTimeout(() => { setShaking(false); setCode(''); }, 500);
      return () => clearTimeout(timer);
    }
  }, [error]);

  useEffect(() => {
    setCode('');
  }, [resetKey]);

  const handleDigit = useCallback((digit: string) => {
    setCode(prev => (prev.length >= length ? prev : prev + digit));
  }, [length]);

  // Submit from an effect rather than inside the state updater: updaters must
  // be pure (React may run them twice), and submitting usually updates the
  // parent's state.
  const onSubmitRef = useRef(onSubmit);
  onSubmitRef.current = onSubmit;
  useEffect(() => {
    if (code.length === length) onSubmitRef.current(code);
  }, [code, length]);

  const handleDelete = useCallback(() => {
    setCode(prev => prev.slice(0, -1));
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key >= '0' && e.key <= '9') {
        handleDigit(e.key);
      } else if (e.key === 'Backspace' || e.key === 'Delete') {
        handleDelete();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleDigit, handleDelete]);

  return (
    <div className={clsx(styles.wrapper, error && styles.hasError)}>
      {prompt && <p className={styles.prompt} id={promptId}>{prompt}</p>}
      <div className={clsx(styles.dots, shaking && styles.shake)} aria-hidden>
        {Array.from({ length }).map((_, i) => (
          <span key={i} className={clsx(styles.dot, i < code.length && styles.dotFilled)} />
        ))}
      </div>
      <p className="oc-visually-hidden" aria-live="polite">
        {t('entry.pinPad.progress', { count: code.length, total: length })}
      </p>
      <p className={styles.error} role="alert">{error}</p>
      <div className={styles.keypad} role="group" aria-labelledby={prompt ? promptId : undefined}>
        {DIGITS.map((d, i) => {
          if (d === '') return <span key={i} aria-hidden />;
          if (d === 'del') {
            return (
              <button
                key={i}
                type="button"
                className={clsx(styles.key, styles.keyQuiet)}
                onClick={handleDelete}
                disabled={code.length === 0}
                aria-label={t('common.pinPad.deleteAriaLabel')}
              >
                <Icon name="back" size={26} />
              </button>
            );
          }
          return (
            <button key={i} type="button" className={styles.key} onClick={() => handleDigit(d)}>
              {d}
            </button>
          );
        })}
      </div>
    </div>
  );
};

export default PinPad;
