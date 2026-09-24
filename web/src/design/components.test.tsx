// @vitest-environment jsdom
import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';
import i18n from '../i18n';
import { ChoreRow } from './ChoreRow';
import { TabBar } from './TabBar';
import { SkinScope } from './Scopes';

beforeAll(async () => {
  await i18n.changeLanguage('en');
});
afterEach(cleanup);

describe('ChoreRow', () => {
  it('keeps the e2e hooks: a choreCard root and a real Mark complete button', () => {
    const onToggle = vi.fn();
    const { container } = render(<ChoreRow title="Feed the cats" icon="🐱" points={5} onToggle={onToggle} />);
    const root = container.firstElementChild!;
    expect(root.tagName).toBe('DIV');
    expect(root.className).toContain('choreCard');
    const btn = screen.getByRole('button', { name: 'Mark complete' });
    fireEvent.click(btn);
    expect(onToggle).toHaveBeenCalledOnce();
    expect(screen.getByText(/5 pts/)).toBeTruthy();
  });

  it('labels done and waiting rows Mark incomplete', () => {
    render(<><ChoreRow title="A" state="done" /><ChoreRow title="B" state="waiting" /></>);
    expect(screen.getAllByRole('button', { name: 'Mark incomplete' })).toHaveLength(2);
    expect(screen.getByText('Waiting for a grown-up')).toBeTruthy();
  });

  it('disables the check on a locked bonus and explains the lock', () => {
    const onToggle = vi.fn();
    render(<ChoreRow cat="bonus" title="Water the garden" state="locked" onToggle={onToggle} />);
    const btn = screen.getByRole('button', { name: 'Locked' }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(screen.getByText('Opens when everything else is done')).toBeTruthy();
  });

  it('accepts a translated check label', () => {
    render(<ChoreRow title="A" checkLabel="Als erledigt markieren" />);
    expect(screen.getByRole('button', { name: 'Als erledigt markieren' })).toBeTruthy();
  });
});

describe('TabBar', () => {
  it('renders links with hrefs and reports navigation', () => {
    const onNavigate = vi.fn();
    render(<TabBar active="week" hrefs={{ today: '/', week: '/week', rewards: '/rewards' }} onNavigate={onNavigate} />);
    const week = screen.getByRole('link', { name: 'Week' });
    expect(week.getAttribute('aria-current')).toBe('page');
    fireEvent.click(screen.getByRole('link', { name: 'Rewards' }));
    expect(onNavigate).toHaveBeenCalledWith('rewards');
  });

  it('renders buttons without hrefs', () => {
    render(<TabBar />);
    expect(screen.getAllByRole('button')).toHaveLength(3);
  });
});

describe('SkinScope', () => {
  it('writes data-theme and data-person together', () => {
    const { container } = render(<SkinScope skin="tint" color="mint">x</SkinScope>);
    const el = container.firstElementChild!;
    expect(el.getAttribute('data-theme')).toBe('tint');
    expect(el.getAttribute('data-person')).toBe('mint');
  });
});
