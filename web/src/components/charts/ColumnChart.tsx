import { useId } from 'react';
import clsx from 'clsx';
import { useWidth } from './useWidth';
import styles from './Charts.module.css';

export interface Column {
  label: string;
  value: number;
  /** The written value above the column ("80%"). Defaults to the number. */
  valueLabel?: string;
  /** Emphasise this column (drawn in ink; the others in muted ink). */
  strong?: boolean;
}

export interface ColumnChartProps {
  /** The chart's accessible name. */
  title: string;
  description?: string;
  data: Column[];
  /** The value a full column stands for. Defaults to the largest value. */
  max?: number;
  height?: number;
  className?: string;
}

const PAD_T = 22;
const PAD_B = 26;

/** Vertical columns with their value written on top, in the theme's inks. */
export function ColumnChart({ title, description, data, max, height = 180, className }: ColumnChartProps) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const uid = useId();
  const top = max ?? Math.max(1, ...data.map((d) => d.value));
  const innerH = height - PAD_T - PAD_B;
  const slot = width / Math.max(1, data.length);
  const barW = Math.min(44, slot * 0.6);

  return (
    <figure className={clsx(styles.figure, className)}>
      <div ref={ref} className={styles.plot}>
        <svg className={styles.svg} width={width} height={height} viewBox={`0 0 ${width} ${height}`}
          role="img" aria-labelledby={`${uid}-t`} aria-describedby={description ? `${uid}-d` : undefined}>
          <title id={`${uid}-t`}>{title}</title>
          {description && <desc id={`${uid}-d`}>{description}</desc>}
          <line className={styles.grid} x1={0} x2={width} y1={PAD_T + innerH} y2={PAD_T + innerH} />
          {data.map((d, i) => {
            const h = Math.max(d.value > 0 ? 4 : 0, (Math.min(d.value, top) / top) * innerH);
            const x = slot * i + (slot - barW) / 2;
            const y = PAD_T + innerH - h;
            const cx = x + barW / 2;
            return (
              <g key={i}>
                <path
                  className={d.strong ? styles.colStrong : styles.col}
                  d={h > 0 ? roundedTop(x, y, barW, h, Math.min(6, barW / 2, h)) : ''}
                >
                  <title>{`${d.label}: ${d.valueLabel ?? d.value}`}</title>
                </path>
                <text className={styles.colValue} x={cx} y={y - 6} textAnchor="middle">{d.valueLabel ?? d.value}</text>
                <text className={styles.axis} x={cx} y={height - 8} textAnchor="middle">{d.label}</text>
              </g>
            );
          })}
        </svg>
      </div>
    </figure>
  );
}

/** A column with rounded top corners, anchored square to the baseline. */
function roundedTop(x: number, y: number, w: number, h: number, r: number): string {
  return `M${x} ${y + h} V${y + r} Q${x} ${y} ${x + r} ${y} H${x + w - r} Q${x + w} ${y} ${x + w} ${y + r} V${y + h} Z`;
}
