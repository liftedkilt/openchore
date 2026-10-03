import React, { useId } from 'react';
import clsx from 'clsx';
import { Trans, useTranslation } from 'react-i18next';
import { Icon } from './Icon';
import { Button } from './primitives';

export interface CelebrationProps {
  points: number;
  /** The chore just finished. */
  title: string;
  /** Days in a row; omit or 0 for none. */
  streak?: number;
  /** The next chore's title; omit at the end of the day. */
  next?: string;
  /** Override the headline ("Nailed it!"). */
  headline?: React.ReactNode;
  onNext?: () => void;
  onBack?: () => void;
  className?: string;
}

/**
 * The moment after a chore is ticked. Fill the screen with it. The copy is the
 * same in every skin; the skin's dials pick the decoration (sun, confetti or
 * ring). Entrance motion is skipped under prefers-reduced-motion.
 */
export function Celebration({ points, title, streak, next, headline, onNext, onBack, className }: CelebrationProps) {
  const { t } = useTranslation();
  const headId = useId();
  return (
    <section className={clsx('oc-cele', className)} aria-labelledby={headId} aria-live="polite">
      <svg className="oc-cele__rays" viewBox="0 0 360 712" preserveAspectRatio="xMidYMin slice" aria-hidden focusable="false">
        <circle className="sun" cx={300} cy={70} r={120} />
        <circle className="dash" cx={300} cy={70} r={150} />
        <circle className="dash" cx={300} cy={70} r={182} />
      </svg>
      <svg className="oc-cele__confetti" viewBox="0 0 360 712" aria-hidden focusable="false">
        <circle className="c1" cx={40} cy={50} r={34} />
        <rect className="c2" x={262} y={20} width={64} height={64} rx={12} transform="rotate(18 294 52)" />
        <path className="c3" d="M300 290l38 66h-76z" transform="rotate(-14 300 330)" />
        <rect className="c4" x={-20} y={290} width={86} height={34} rx={17} transform="rotate(-24 23 307)" />
        <path className="sq" d="M150 64c10-12 20 12 30 0s20 12 30 0" />
        <circle className="c5" cx={318} cy={200} r={12} />
        <rect className="c1" x={18} y={520} width={46} height={46} rx={10} transform="rotate(-12 41 543)" />
        <circle className="c2" cx={330} cy={520} r={22} />
        <path className="c5" d="M82 480l7 15 16 2-12 11 3 16-14-8-14 8 3-16-12-11 16-2z" />
      </svg>
      <div className="oc-cele__body">
        <h2 className="oc-cele__head" id={headId}>
          {headline ?? <Trans i18nKey="design.celebration.headline" components={{ em: <em /> }} />}
        </h2>
        <div className="oc-cele__card">
          {/* The ring frames the card, so it is drawn around it rather than
              across the screen: it stays centred on the points at any size. */}
          <svg className="oc-cele__ringdeco" viewBox="-200 -200 400 400" aria-hidden focusable="false">
            <circle className="glow" r={190} />
            <circle className="trk" r={150} />
            <circle className="arc" r={150} pathLength={100} strokeDasharray="100 100" />
            <path className="sp" d="M138 -180c.7 5 3.9 8.2 9 9-5.1.8-8.3 4-9 9-.7-5-3.9-8.2-9-9 5.1-.8 8.3-4 9-9z" />
            <path className="sp" d="M-140 140c.5 3.6 2.8 5.9 6.5 6.5-3.7.6-6 2.9-6.5 6.5-.5-3.6-2.8-5.9-6.5-6.5 3.7-.6 6-2.9 6.5-6.5z" />
          </svg>
          <div className="oc-cele__pts">
            <span aria-hidden>{t('design.celebration.points', { count: points })}</span>
            <span className="oc-visually-hidden">{t('design.celebration.pointsLabel', { count: points })}</span>
          </div>
          <div className="oc-cele__what">{title}</div>
          {!!streak && (
            <div className="oc-cele__streak">
              <Icon name="flame" />
              {t('design.celebration.streak', { count: streak })}
            </div>
          )}
        </div>
      </div>
      <div className="oc-cele__foot">
        {next && (
          <Button block icon="chev" onClick={onNext}>
            {t('design.celebration.next', { title: next })}
          </Button>
        )}
        <Button variant="quiet" onClick={onBack}>{t('design.celebration.back')}</Button>
      </div>
    </section>
  );
}
