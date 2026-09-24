import type React from 'react';
import clsx from 'clsx';
import styles from './Charts.module.css';

export interface BarListItem {
  key: string | number;
  /** The row's name, always written out. */
  label: string;
  /** Something before the name: an Avatar, a CategoryMark… (decorative). */
  lead?: React.ReactNode;
  value: number;
  /** The written value ("80%", "+120"). Defaults to the number. */
  valueLabel?: string;
  /** A CSS colour for the bar, normally a token. Defaults to ink. */
  color?: string;
  /** A second line under the name. */
  note?: React.ReactNode;
}

export interface BarListProps {
  /** Names the list for assistive tech. */
  label: string;
  items: BarListItem[];
  /** The value a full bar stands for. Defaults to the largest value. */
  max?: number;
  className?: string;
}

/**
 * Horizontal bars as a plain list: name, bar, written value. The value is
 * always text, so a pale person colour never has to carry it alone.
 */
export function BarList({ label, items, max, className }: BarListProps) {
  const top = max ?? Math.max(1, ...items.map((i) => i.value));
  return (
    <ul className={clsx(styles.barList, className)} aria-label={label}>
      {items.map((item) => {
        const pct = top > 0 ? Math.max(0, Math.min(100, (item.value / top) * 100)) : 0;
        return (
          <li key={item.key} className={styles.barRow}>
            <span className={styles.barName}>
              {item.lead}
              <span className={styles.barText}>
                <span className={styles.barLabel}>{item.label}</span>
                {item.note && <span className={styles.barNote}>{item.note}</span>}
              </span>
            </span>
            <span className={styles.barTrack} aria-hidden>
              <i style={{ width: `${pct}%`, background: item.color ?? 'var(--ink)' }} />
            </span>
            <span className={styles.barValue}>{item.valueLabel ?? item.value}</span>
          </li>
        );
      })}
    </ul>
  );
}
