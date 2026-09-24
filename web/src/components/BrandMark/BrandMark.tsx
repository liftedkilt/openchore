import clsx from 'clsx';
import styles from './BrandMark.module.css';

const DOTS = ['coral', 'mint', 'butter', 'sky'] as const;

export interface BrandMarkProps {
  /** Show the "openchore" wordmark beside the dots. Default true. */
  wordmark?: boolean;
  className?: string;
}

/**
 * The OpenChore logo: four overlapping dots, one per person colour, and the
 * wordmark. People are the only colour in House, so the mark is too.
 */
export function BrandMark({ wordmark = true, className }: BrandMarkProps) {
  return (
    <span className={clsx(styles.brand, className)}>
      <span className={styles.dots} aria-hidden>
        {DOTS.map((c) => <i key={c} style={{ background: `var(--person-${c})` }} />)}
      </span>
      <span className={wordmark ? undefined : 'oc-visually-hidden'}>openchore</span>
    </span>
  );
}

export default BrandMark;
