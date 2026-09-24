import type { CSSProperties } from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { CategoryMark } from './primitives';
import type { Cat } from './types';

export interface DayProgressItem {
  cat: Cat;
  done?: boolean;
  /** When a done chore was finished, 0–1 across the day (for the sun arc's pins). */
  at?: number;
}

export interface DayProgressProps {
  /** One per chore for the day. */
  items: DayProgressItem[];
  /** Where the sun sits, 0–1 across the day. Defaults to 0.5. */
  now?: number;
  /** Arc end labels. Default to the localized "7 am" / "9 pm". */
  from?: string;
  to?: string;
  className?: string;
}

const ARC = 'M12 92 A144 80 0 0 1 300 92';
const clamp01 = (n: number) => Math.min(1, Math.max(0, n));
function arcPoint(f: number): [number, number] {
  const a = Math.PI * (1 - clamp01(f));
  return [156 + 144 * Math.cos(a), 92 - 80 * Math.sin(a)];
}

function Spark({ style }: { style: CSSProperties }) {
  return (
    <svg className="oc-prog__spark" viewBox="0 0 24 24" style={style} aria-hidden focusable="false">
      <path d="M12 3c.7 5 3.9 8.2 9 9-5.1.8-8.3 4-9 9-.7-5-3.9-8.2-9-9 5.1-.8 8.3-4 9-9z" />
    </svg>
  );
}

/**
 * The hero on Today. All three variants render (sun arc, shape row, ring);
 * the --show-arc / --show-shapes / --show-ring dials pick one. The written
 * count is always present, so the graphic never carries the meaning alone.
 */
export function DayProgress({ items, now = 0.5, from, to, className }: DayProgressProps) {
  const { t } = useTranslation();
  const done = items.filter((x) => x.done).length;
  const total = items.length;
  const sun = arcPoint(now);
  const ringPct = total ? (done / total) * 100 : 0;

  return (
    <div className={clsx('oc-prog', className)}>
      <div className="oc-prog__arcwrap">
        <svg className="oc-prog__arc" viewBox="0 0 312 100" aria-hidden focusable="false">
          <path className="oc-prog__track" d={ARC} />
          <path className="oc-prog__fill" d={ARC} pathLength={100} strokeDasharray={`${clamp01(now) * 100} 100`} />
          {items.map((x, i) => {
            if (!x.done || x.at == null) return null;
            const [cx, cy] = arcPoint(x.at);
            return <circle key={i} className="oc-prog__pin" cx={cx} cy={cy} r={7} />;
          })}
          <circle className="oc-prog__halo" cx={sun[0]} cy={sun[1]} r={20} />
          <circle className="oc-prog__sun" cx={sun[0]} cy={sun[1]} r={11} />
        </svg>
        <div className="oc-prog__arcnum">
          <b>{done}<span>{t('design.progress.of', { total })}</span></b>
          <small>{t('design.progress.doneToday')}</small>
        </div>
        <div className="oc-prog__ends" aria-hidden>
          <span>{from ?? t('design.progress.from')}</span>
          <span>{to ?? t('design.progress.to')}</span>
        </div>
      </div>

      <div className="oc-prog__shapes">
        {items.map((x, i) => <CategoryMark key={i} cat={x.cat} done={x.done} />)}
        <b aria-label={t('design.progress.label', { done, total })}>{t('design.progress.count', { done, total })}</b>
      </div>

      <div className="oc-prog__ringwrap">
        <svg className="oc-prog__ring" viewBox="0 0 212 212" aria-hidden focusable="false">
          <circle className="oc-prog__rtrack" cx={106} cy={106} r={90} />
          {done > 0 && (
            <circle className="oc-prog__rfill" cx={106} cy={106} r={90} pathLength={100} strokeDasharray={`${ringPct} 100`} />
          )}
        </svg>
        <Spark style={{ right: -6, top: 6, width: 20, height: 20 }} />
        <Spark style={{ right: 18, top: -10, width: 11, height: 11, opacity: 0.7 }} />
        <Spark style={{ left: -14, bottom: 30, width: 13, height: 13, opacity: 0.5 }} />
        <div className="oc-prog__ringnum">
          <b>{done}<span>{t('design.progress.slash', { total })}</span></b>
          <small>{t('design.progress.doneToday')}</small>
        </div>
      </div>
    </div>
  );
}
