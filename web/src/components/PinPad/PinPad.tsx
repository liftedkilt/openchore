import React, { useCallback, useEffect, useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Icon } from '../../design';
import styles from './PinPad.module.css';

/** PINs are 4-8 digits (the server enforces the same rule). */
export const MIN_PIN_LENGTH = 4;
export const MAX_PIN_LENGTH = 8;

interface PinPadProps {
  /** Text shown above the dots (e.g. "Enter your PIN"). */
  prompt?: string;
  /** Quieter line under the prompt (e.g. "4 to 8 digits"). */
  hint?: string;
  /** Optional error message to display under the dots. */
  error?: string;
  /** Called with the PIN once it is complete. */
  onSubmit: (pin: string) => void;
  /**
   * How many digits the PIN has, when known: the pad submits as soon as the
   * last one is entered. Without it, any 4-8 digit PIN can be entered and
   * the check key (or Enter) submits it.
   */
  length?: number;
  /** When set, clears the current input (e.g. after the parent reports an error). */
  resetKey?: number;
}

/**
 * An on-screen PIN pad. It has no theme of its own: it reads the tokens and
 * dials of whatever scope it sits in (House, House Dark or a person's skin).
 */
export const PinPad: React.FC<PinPadProps> = ({ prompt, hint, error, onSubmit, length, resetKey }) => {
  const { t } = useTranslation();
  const [code, setCode] = useState('');
  const [shaking, setShaking] = useState(false);
  const promptId = useId();

  const fixed = length !== undefined && length >= MIN_PIN_LENGTH && length <= MAX_PIN_LENGTH ? length : undefined;
  const maxLength = fixed ?? MAX_PIN_LENGTH;
  const canSubmit = fixed === undefined && code.length >= MIN_PIN_LENGTH;

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
    setCode(prev => (prev.length >= maxLength ? prev : prev + digit));
  }, [maxLength]);

  // Submit from an effect rather than inside the state updater: updaters must
  // be pure (React may run them twice), and submitting usually updates the
  // parent's state.
  const onSubmitRef = useRef(onSubmit);
  onSubmitRef.current = onSubmit;
  useEffect(() => {
    if (fixed !== undefined && code.length === fixed) onSubmitRef.current(code);
  }, [code, fixed]);

  // Without a known length the person says when they're done. Each entry is
  // submitted once, so a double tap (or tap + Enter) doesn't count twice.
  const submitted = useRef<string | null>(null);
  useEffect(() => {
    if (code !== submitted.current) submitted.current = null;
  }, [code]);
  const handleSubmit = useCallback(() => {
    if (!canSubmit || submitted.current === code) return;
    submitted.current = code;
    onSubmitRef.current(code);
  }, [canSubmit, code]);

  const handleDelete = useCallback(() => {
    setCode(prev => prev.slice(0, -1));
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key >= '0' && e.key <= '9') {
        handleDigit(e.key);
      } else if (e.key === 'Backspace' || e.key === 'Delete') {
        handleDelete();
      } else if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement)) {
        // On a focused key, Enter presses that key instead.
        handleSubmit();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleDigit, handleDelete, handleSubmit]);

  // A known length shows exactly that many dots; otherwise they grow with
  // the entry, starting from the shortest PIN.
  const dots = fixed ?? Math.max(MIN_PIN_LENGTH, code.length);

  return (
    <div className={clsx(styles.wrapper, error && styles.hasError)}>
      {prompt && <p className={styles.prompt} id={promptId}>{prompt}</p>}
      {hint && <p className={styles.hint}>{hint}</p>}
      <div className={clsx(styles.dots, shaking && styles.shake)} aria-hidden>
        {Array.from({ length: dots }).map((_, i) => (
          <span key={i} className={clsx(styles.dot, i < code.length && styles.dotFilled)} />
        ))}
      </div>
      <p className="oc-visually-hidden" aria-live="polite">
        {fixed !== undefined
          ? t('entry.pinPad.progress', { count: code.length, total: fixed })
          : t('entry.pinPad.progressOpen', { count: code.length })}
      </p>
      <p className={styles.error} role="alert">{error}</p>
      <div className={styles.keypad} role="group" aria-labelledby={prompt ? promptId : undefined}>
        {['1', '2', '3', '4', '5', '6', '7', '8', '9'].map(d => (
          <button key={d} type="button" className={styles.key} onClick={() => handleDigit(d)}>
            {d}
          </button>
        ))}
        {fixed === undefined ? (
          <button
            type="button"
            className={clsx(styles.key, canSubmit ? styles.keySubmit : styles.keyQuiet)}
            onClick={handleSubmit}
            disabled={!canSubmit}
            aria-label={t('common.pinPad.submitAriaLabel')}
          >
            <Icon name="check" size={26} />
          </button>
        ) : (
          <span aria-hidden />
        )}
        <button type="button" className={styles.key} onClick={() => handleDigit('0')}>
          0
        </button>
        <button
          type="button"
          className={clsx(styles.key, styles.keyQuiet)}
          onClick={handleDelete}
          disabled={code.length === 0}
          aria-label={t('common.pinPad.deleteAriaLabel')}
        >
          <Icon name="back" size={26} />
        </button>
      </div>
    </div>
  );
};

export default PinPad;
