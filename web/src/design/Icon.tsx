import React from 'react';
import clsx from 'clsx';
import { ICONS, type IconName, type IconShape } from './icons';

export interface IconProps extends Omit<React.SVGProps<SVGSVGElement>, 'name' | 'fill'> {
  name: IconName;
  /** Width and height in px. Defaults to the CSS size (22px). */
  size?: number;
  /** Solid fill, for the star and flame. */
  fill?: boolean;
  className?: string;
}

function renderShape(s: IconShape, i: number) {
  switch (s.tag) {
    case 'path':
      return <path key={i} d={s.d} fill={s.fill} />;
    case 'circle':
      return <circle key={i} cx={s.cx} cy={s.cy} r={s.r} fill={s.fill} />;
    case 'rect':
      return <rect key={i} x={s.x} y={s.y} width={s.width} height={s.height} rx={s.rx} fill={s.fill} />;
  }
}

/**
 * A line icon. Takes `currentColor` and the skin's stroke weight. Decorative
 * by default (aria-hidden); pass `aria-hidden={false}` plus `aria-label` and
 * `role="img"` if it must carry meaning on its own.
 */
export function Icon({ name, size, fill, className, style, ...rest }: IconProps) {
  const shapes: IconShape[] = ICONS[name] ?? [];
  return (
    <svg
      viewBox="0 0 24 24"
      aria-hidden
      focusable="false"
      {...rest}
      className={clsx('oc-icon', fill && 'oc-icon--fill', className)}
      style={size ? { width: size, height: size, ...style } : style}
    >
      {shapes.map(renderShape)}
    </svg>
  );
}
