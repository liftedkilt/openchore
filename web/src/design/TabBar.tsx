import React from 'react';
import clsx from 'clsx';
import { useTranslation } from 'react-i18next';
import { Icon } from './Icon';
import type { IconName } from './icons';
import type { TabId } from './types';

export const TABS: readonly { id: TabId; icon: IconName }[] = [
  { id: 'today', icon: 'home' },
  { id: 'week', icon: 'cal' },
  { id: 'rewards', icon: 'gift' },
];

/** What a custom link renderer receives: spread it onto your link element. */
export interface TabLinkProps {
  tab: TabId;
  href: string;
  className: string;
  'aria-label': string;
  'aria-current'?: 'page';
  onClick?: (e: React.MouseEvent<HTMLElement>) => void;
  children: React.ReactNode;
}

export interface TabBarProps {
  /** The current tab. */
  active?: TabId;
  /** Link targets. With hrefs, tabs render as links; without, as buttons. */
  hrefs?: Partial<Record<TabId, string>>;
  /** Called on tap. With hrefs, a plain left click is prevented so you can route. */
  onNavigate?: (tab: TabId) => void;
  /**
   * Render each tab yourself, e.g. with react-router:
   * `renderLink={({ tab, href, ...p }) => <Link key={tab} to={href} {...p} />}`
   * Requires `hrefs`.
   */
  renderLink?: (props: TabLinkProps) => React.ReactNode;
  /**
   * absolute (default): at the bottom of the nearest positioned ancestor.
   * fixed: at the bottom of the viewport. static: in flow.
   */
  position?: 'absolute' | 'fixed' | 'static';
  className?: string;
}

/** The three tabs every skin shares: Today, Week and Rewards. */
export function TabBar({ active = 'today', hrefs, onNavigate, renderLink, position = 'absolute', className }: TabBarProps) {
  const { t } = useTranslation();
  return (
    <nav className={clsx('oc-tabs', position !== 'absolute' && `oc-tabs--${position}`, className)} aria-label={t('design.tabs.label')}>
      {TABS.map(({ id, icon }) => {
        const on = id === active;
        const label = t(`design.tabs.${id}`);
        const children = (
          <>
            <span className="oc-tab__pill">
              <Icon name={icon} />
              <span className="oc-tab__label" aria-hidden>{label}</span>
            </span>
            <span className="oc-tab__dot" aria-hidden />
          </>
        );
        const cls = clsx('oc-tab', on && 'is-on');
        const href = hrefs?.[id];
        if (href != null) {
          const onClick = onNavigate
            ? (e: React.MouseEvent<HTMLElement>) => {
                if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                if (!renderLink) e.preventDefault();
                onNavigate(id);
              }
            : undefined;
          const props: TabLinkProps = {
            tab: id, href, className: cls, 'aria-label': label,
            'aria-current': on ? 'page' : undefined, onClick, children,
          };
          if (renderLink) return <React.Fragment key={id}>{renderLink(props)}</React.Fragment>;
          const { tab: _tab, ...anchor } = props;
          return <a key={id} {...anchor} />;
        }
        return (
          <button
            key={id}
            type="button"
            className={cls}
            aria-label={label}
            aria-current={on ? 'page' : undefined}
            onClick={() => onNavigate?.(id)}
          >
            {children}
          </button>
        );
      })}
    </nav>
  );
}
