import React from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Icon } from './Icon';
import type { IconName } from './icons';
import type { Cat, PersonColor } from './types';

/* ---------------- Greeting ---------------- */

export interface GreetingProps {
  /** "Morning", "Afternoon", "Evening"… Defaults to the localized "Hi". */
  salutation?: string;
  /** The person's first name. */
  name: string;
  className?: string;
}

/** The greeting line at the top of a person's own screen. */
export function Greeting({ salutation, name, className }: GreetingProps) {
  const { t } = useTranslation();
  return (
    <h1 className={clsx('oc-greeting', className)}>
      {salutation ?? t('design.greeting.hi')}, <em>{name}.</em>
    </h1>
  );
}

/* ---------------- Avatar ---------------- */

const BLOBS = [
  '58% 42% 52% 48% / 46% 56% 44% 54%',
  '44% 56% 40% 60% / 58% 42% 58% 42%',
  '52% 48% 62% 38% / 40% 52% 48% 60%',
  '48% 52% 44% 56% / 54% 46% 56% 44%',
];

/** The Sunroom blob for a name: stable per person. */
export function blobFor(name: string): string {
  return BLOBS[(name.codePointAt(0) ?? 0) % BLOBS.length];
}

export type AvatarSize = 'sm' | 'md' | 'lg';

export interface AvatarProps {
  name: string;
  /** The person's colour. Without one, the avatar falls back to surface-sunk. */
  color?: PersonColor | null;
  /** sm 32px, md 40px (default), lg 96px. */
  size?: AvatarSize;
  /** An optional picture (e.g. the person's chosen avatar). It sits on their
   *  colour in the skin's avatar shape; the initial shows until it loads. */
  src?: string | null;
  className?: string;
  style?: React.CSSProperties;
}

/**
 * A person's initial on their own colour. Decorative: always pair it with the
 * name in text nearby.
 */
export function Avatar({ name, color, size = 'md', src, className, style }: AvatarProps) {
  const initial = Array.from(name.trim())[0]?.toUpperCase() ?? '?';
  const [failed, setFailed] = React.useState<string | null>(null);
  const showImg = !!src && failed !== src;
  return (
    <span
      className={clsx('oc-avatar', `oc-avatar--${size}`, showImg && 'oc-avatar--img', className)}
      data-person={color || undefined}
      aria-hidden
      style={{ '--blob': blobFor(name), ...style } as React.CSSProperties}
    >
      {initial}
      {showImg && <img src={src} alt="" draggable={false} onError={() => setFailed(src)} />}
    </span>
  );
}

/* ---------------- PointsChip ---------------- */

export interface PointsChipProps {
  points: number;
  className?: string;
}

/** A person's point balance: a star and a number. */
export function PointsChip({ points, className }: PointsChipProps) {
  const { t } = useTranslation();
  return (
    <span className={clsx('oc-points', className)} role="img" aria-label={t('design.points.label', { count: points })}>
      <Icon name="star" />
      {points}
    </span>
  );
}

/* ---------------- CategoryMark / CategoryHeader ---------------- */

export interface CategoryMarkProps {
  cat: Cat;
  /** Filled (in DayProgress' shape row) or empty. */
  done?: boolean;
  className?: string;
}

/** The category's shape: Must do ●, Every day ■, Bonus ★. */
export function CategoryMark({ cat, done, className }: CategoryMarkProps) {
  return (
    <svg className={clsx('oc-mark', `oc-mark--${cat}`, done && 'is-done', className)} viewBox="0 0 24 24" aria-hidden focusable="false">
      {cat === 'essential' && <circle cx="12" cy="12" r="9.5" />}
      {cat === 'daily' && <rect x="2.5" y="2.5" width="19" height="19" rx="4.5" />}
      {cat === 'bonus' && <path d="M12 1.8l3.1 6.4 7 1-5.1 4.9 1.2 7L12 17.8l-6.2 3.3 1.2-7-5.1-4.9 7-1z" />}
    </svg>
  );
}

export interface CategoryHeaderProps {
  cat: Cat;
  /** "2 of 3", "+15"… */
  count?: React.ReactNode;
  /** Don't override: the names are shared across skins. */
  label?: string;
  /** Heading level for the label. Defaults to h2. */
  as?: 'h2' | 'h3' | 'h4' | 'div';
  className?: string;
}

/** The label above a group of chores: shape, name and a count. */
export function CategoryHeader({ cat, count, label, as: Tag = 'h2', className }: CategoryHeaderProps) {
  const { t } = useTranslation();
  return (
    <div className={clsx('oc-cathead', className)}>
      <Tag className="oc-cathead__l">
        <CategoryMark cat={cat} />
        {label ?? t(`design.category.${cat}`)}
      </Tag>
      {count != null && <span className="oc-cathead__n">{count}</span>}
    </div>
  );
}

/* ---------------- Button ---------------- */

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** primary (accent fill) or quiet (underlined text). */
  variant?: 'primary' | 'quiet';
  /** Full width. */
  block?: boolean;
  /** A trailing icon, e.g. `chev` for "Next". */
  icon?: IconName;
}

/** The one primary action on a screen, or a quiet way back. */
export function Button({ variant = 'primary', block, icon, className, children, type = 'button', ...rest }: ButtonProps) {
  return (
    <button type={type} className={clsx('oc-btn', `oc-btn--${variant}`, block && 'oc-btn--block', className)} {...rest}>
      {children}
      {icon && <Icon name={icon} />}
    </button>
  );
}

/* ---------------- FamilyMember ---------------- */

export interface FamilyMemberProps {
  name: string;
  color?: PersonColor | null;
  done: number;
  total: number;
  className?: string;
}

/** One person's progress today, for family views. Never a ranking. */
export function FamilyMember({ name, color, done, total, className }: FamilyMemberProps) {
  const { t } = useTranslation();
  const pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
  return (
    <div className={clsx('oc-member', className)} data-person={color || undefined}>
      <Avatar name={name} color={color} size="sm" />
      <span className="oc-member__n" aria-hidden>{name}</span>
      <span className="oc-member__bar" aria-hidden>
        <i style={{ width: `${pct}%` }} />
      </span>
      <span className="oc-member__c" aria-hidden>{t('design.family.count', { done, total })}</span>
      <span className="oc-visually-hidden">{t('design.family.label', { name, done, total })}</span>
    </div>
  );
}
