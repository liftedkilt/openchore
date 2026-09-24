import { useId } from 'react';
import clsx from 'clsx';
import { useWidth } from './useWidth';
import { spreadLabels } from './personColor';
import styles from './Charts.module.css';

export interface LinePoint {
  x: number;
  y: number;
}

export interface LineSeries {
  key: string;
  /** Shown in the legend and at the line's end. */
  label: string;
  /** Any CSS colour expression, normally a token: `var(--person-mint)`. */
  color: string;
  points: LinePoint[];
  /** Dashed stroke (a second encoding beside colour). */
  dashed?: boolean;
  /** Tint the area under the line. */
  area?: boolean;
  /** Draw as steps (value holds until the next point). */
  step?: boolean;
}

export interface Tick {
  value: number;
  label: string;
}

export interface LineChartProps {
  /** The chart's accessible name. */
  title: string;
  /** A one- or two-sentence summary for screen readers. */
  description?: string;
  series: LineSeries[];
  xDomain: [number, number];
  yDomain?: [number, number];
  xTicks: Tick[];
  yTicks?: Tick[];
  /** A vertical "now" rule. */
  marker?: { x: number; label?: string };
  /** Hover text for a point. */
  pointTitle?: (s: LineSeries, p: LinePoint) => string;
  /** Draw point dots (with hover titles) when a series has this many points or fewer. */
  maxDots?: number;
  height?: number;
  /** Hide the legend row (e.g. when the lines are labelled elsewhere). */
  hideLegend?: boolean;
  className?: string;
}

/** A round step (1, 2 or 5 × 10^n, at least 1) near `raw`. */
export function niceStep(raw: number): number {
  if (!(raw > 1)) return 1;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const f = raw / mag;
  return (f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10) * mag;
}

const PAD_T = 12;
const PAD_B = 26;
const PAD_L = 40;
const LABEL_GAP = 16;

/**
 * A line chart drawn in the current theme's tokens: gridlines in `--line`,
 * axis text in `--ink-muted`, series in whatever colour they carry (people's
 * colours for per-person series). Every series is named twice — in the legend
 * and at the end of its line — so no one has to match colours.
 */
export function LineChart({
  title, description, series, xDomain, yDomain, xTicks, yTicks, marker, pointTitle,
  maxDots = 40, height = 220, hideLegend, className,
}: LineChartProps) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const uid = useId();
  const titleId = `${uid}-t`;
  const descId = `${uid}-d`;

  const longest = Math.max(0, ...series.map((s) => s.label.length));
  const padR = series.length ? Math.min(140, 22 + longest * 7.5) : 12;
  const innerW = Math.max(40, width - PAD_L - padR);
  const innerH = Math.max(40, height - PAD_T - PAD_B);

  const allY = series.flatMap((s) => s.points.map((p) => p.y));
  const autoStep = niceStep(Math.max(1, ...allY) / 4);
  const [y0, y1] = yDomain ?? [0, Math.ceil(Math.max(1, ...allY) / autoStep) * autoStep];
  const [x0, x1] = xDomain;
  const sx = (x: number) => PAD_L + ((Math.min(Math.max(x, x0), x1) - x0) / Math.max(1e-9, x1 - x0)) * innerW;
  const sy = (y: number) => PAD_T + innerH - ((Math.min(Math.max(y, y0), y1) - y0) / Math.max(1e-9, y1 - y0)) * innerH;
  const baseY = PAD_T + innerH;

  const grid: Tick[] = yTicks ?? Array.from({ length: Math.round((y1 - y0) / autoStep) + 1 }, (_, i) => {
    const v = y0 + i * autoStep;
    return { value: v, label: String(v) };
  });

  const paths = series.map((s) => {
    const pts: [number, number][] = [];
    s.points.forEach((p, i) => {
      if (s.step && i > 0) pts.push([sx(p.x), sy(s.points[i - 1].y)]);
      pts.push([sx(p.x), sy(p.y)]);
    });
    const d = pts.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`).join(' ');
    const area = s.area && pts.length > 1
      ? `${d} L${pts[pts.length - 1][0].toFixed(1)} ${baseY} L${pts[0][0].toFixed(1)} ${baseY} Z`
      : null;
    const last = pts[pts.length - 1];
    return { s, d, area, last };
  });

  const ends = paths.filter((p) => p.last);
  const labelYs = spreadLabels(ends.map((p) => p.last![1]), LABEL_GAP, PAD_T + 4, baseY);

  return (
    <figure className={clsx(styles.figure, className)}>
      {!hideLegend && series.length > 1 && (
        <ul className={styles.legend} aria-hidden>
          {series.map((s) => (
            <li key={s.key}>
              <svg width="18" height="10" aria-hidden focusable="false">
                <line x1="1" y1="5" x2="17" y2="5" style={{ stroke: s.color }} strokeWidth="3" strokeLinecap="round"
                  strokeDasharray={s.dashed ? '3 4' : undefined} />
              </svg>
              {s.label}
            </li>
          ))}
        </ul>
      )}
      <div ref={ref} className={styles.plot}>
        <svg
          className={styles.svg}
          width={width}
          height={height}
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-labelledby={titleId}
          aria-describedby={description ? descId : undefined}
        >
          <title id={titleId}>{title}</title>
          {description && <desc id={descId}>{description}</desc>}

          {grid.map((t) => (
            <g key={t.value}>
              <line className={styles.grid} x1={PAD_L} x2={PAD_L + innerW} y1={sy(t.value)} y2={sy(t.value)} />
              <text className={styles.axis} x={PAD_L - 8} y={sy(t.value)} textAnchor="end" dominantBaseline="middle">{t.label}</text>
            </g>
          ))}
          {xTicks.map((t, i) => (
            <text key={i} className={styles.axis} x={sx(t.value)} y={height - 8} textAnchor="middle">{t.label}</text>
          ))}

          {marker && marker.x >= x0 && marker.x <= x1 && (
            <g>
              <line className={styles.marker} x1={sx(marker.x)} x2={sx(marker.x)} y1={PAD_T} y2={baseY} />
              {marker.label && (
                <text className={styles.axis} x={sx(marker.x)} y={PAD_T - 2} textAnchor="middle">{marker.label}</text>
              )}
            </g>
          )}

          {paths.map(({ s, area }) => area && (
            <path key={`${s.key}-a`} d={area} className={styles.area} style={{ fill: s.color }} />
          ))}
          {paths.map(({ s, d }) => (
            <path key={s.key} d={d} className={styles.line} style={{ stroke: s.color }}
              strokeDasharray={s.dashed ? '5 5' : undefined} />
          ))}
          {series.map((s) => s.points.length <= maxDots && s.points.map((p, i) => (
            <circle key={`${s.key}-${i}`} className={styles.dot} cx={sx(p.x)} cy={sy(p.y)} r={4} style={{ fill: s.color }}>
              {pointTitle && <title>{pointTitle(s, p)}</title>}
            </circle>
          )))}

          {ends.map(({ s, last }, i) => (
            <g key={`${s.key}-end`}>
              <circle className={styles.dot} cx={last![0]} cy={last![1]} r={5} style={{ fill: s.color }} />
              <text className={styles.endLabel} x={last![0] + 12} y={labelYs[i]} dominantBaseline="middle">{s.label}</text>
            </g>
          ))}
        </svg>
      </div>
    </figure>
  );
}
