import React, { useCallback, useEffect, useState } from 'react';
import { Delete } from 'lucide-react';
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

export const PinPad: React.FC<PinPadProps> = ({ prompt, error, onSubmit, length = 4, resetKey }) => {
  const [code, setCode] = useState('');
  const [shaking, setShaking] = useState(false);

  // Reset the input when the parent bumps resetKey or the error changes.
  useEffect(() => {
    if (error) {
      setShaking(true);
      const t = setTimeout(() => { setShaking(false); setCode(''); }, 500);
      return () => clearTimeout(t);
    }
  }, [error]);

  useEffect(() => {
    setCode('');
  }, [resetKey]);

  const handleDigit = useCallback((digit: string) => {
    setCode(prev => {
      if (prev.length >= length) return prev;
      const next = prev + digit;
      if (next.length === length) {
        onSubmit(next);
      }
      return next;
    });
  }, [length, onSubmit]);

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

  const digits = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', 'del'];

  return (
    <div className={styles.wrapper}>
      {prompt && <p className={styles.prompt}>{prompt}</p>}
      <div className={`${styles.dots} ${shaking ? styles.shake : ''}`}>
        {Array.from({ length }).map((_, i) => (
          <div
            key={i}
            className={`${styles.dot} ${i < code.length ? styles.dotFilled : ''} ${error ? styles.dotError : ''}`}
          />
        ))}
      </div>
      {error && <p className={styles.error}>{error}</p>}
      <div className={styles.keypad}>
        {digits.map((d, i) => {
          if (d === '') return <div key={i} className={styles.keyEmpty} />;
          if (d === 'del') {
            return (
              <button key={i} type="button" className={styles.key} onClick={handleDelete} aria-label="Delete">
                <Delete size={22} />
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
