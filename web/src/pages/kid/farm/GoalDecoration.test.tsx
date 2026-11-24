// @vitest-environment jsdom
import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../../i18n';
import en from '../../../i18n/locales/en/translation.json';
import de from '../../../i18n/locales/de/translation.json';
import type { GoalCelebration, Pet } from '../../../types';
import { SkinScope } from '../../../design/Scopes';
import type { Skin } from '../../../design/types';
import { FarmScene } from './FarmScene';

const SAMPLE_PET: Pet = {
  id: 1,
  user_id: 3,
  name: 'Biscuit',
  species: 'dog',
  color: 'amber',
  accessory: '',
  items: [],
  health: 100,
  fullness: 90,
  joy: 90,
  mood: 'happy',
  sick: false,
  hiding: false,
  can_play: true,
  owner_name: 'Emma',
  owner_color: 'coral',
};

const GOAL_SMITHS: GoalCelebration = {
  link_id: 1,
  family: 'The Smiths',
  week: 'this',
  week_start: '2026-06-08',
  target: 85,
  score: 90,
};

const GOAL_GARCIAS: GoalCelebration = {
  link_id: 2,
  family: 'The Garcias',
  week: 'last',
  week_start: '2026-06-01',
  target: 80,
  score: 84,
};

const origMatchMedia = window.matchMedia;

beforeAll(async () => {
  if (!i18n.isInitialized) await i18n.init();
  await i18n.changeLanguage('en');
});

afterEach(async () => {
  cleanup();
  window.sessionStorage.clear();
  window.history.replaceState(null, '', '/');
  Object.defineProperty(window, 'matchMedia', { writable: true, configurable: true, value: origMatchMedia });
  vi.restoreAllMocks();
  await i18n.changeLanguage('en');
});

describe('GoalDecoration inside FarmScene', () => {
  it('stays hidden when goalCelebrations is omitted or empty', () => {
    const { rerender } = render(<FarmScene pets={[SAMPLE_PET]} meId={3} />);
    expect(screen.queryByTestId('farm-goal-decoration')).toBeNull();

    rerender(<FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[]} />);
    expect(screen.queryByTestId('farm-goal-decoration')).toBeNull();
  });

  it('renders bunting and accessible note pill for a single reached goal on own farm (full mode)', () => {
    const { container } = render(
      <FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[GOAL_SMITHS]} />,
    );

    const pill = screen.getByTestId('farm-goal-decoration');
    expect(pill.getAttribute('role')).toBe('note');
    expect(pill.textContent).toContain('Team goal with The Smiths: 85!');
    expect(pill.getAttribute('aria-label')).toBe(
      'Celebration bunting: team goal of 85 reached with The Smiths.',
    );
    expect(pill.getAttribute('title')).toBe(
      'Celebration bunting: team goal of 85 reached with The Smiths.',
    );

    // Two decorative bunting SVGs (both aria-hidden="true") with 10 total pennant polygons.
    const buntingSvgs = Array.from(container.querySelectorAll('svg[aria-hidden="true"]'))
      .filter(svg => svg.querySelectorAll('polygon').length === 5);
    expect(buntingSvgs).toHaveLength(2);
  });

  it('renders compact note pill for 1 and 2 reached goals without popover button', () => {
    const { rerender } = render(
      <FarmScene pets={[SAMPLE_PET]} meId={3} compact goalCelebrations={[GOAL_SMITHS]} />,
    );
    const pill1 = screen.getByTestId('farm-goal-decoration');
    expect(pill1.tagName).toBe('DIV');
    expect(pill1.getAttribute('role')).toBe('note');
    expect(pill1.textContent).toContain('Team goal!');
    expect(pill1.getAttribute('aria-label')).toContain('The Smiths');

    rerender(
      <FarmScene pets={[SAMPLE_PET]} meId={3} compact goalCelebrations={[GOAL_SMITHS, GOAL_GARCIAS]} />,
    );
    const pill2 = screen.getByTestId('farm-goal-decoration');
    expect(pill2.tagName).toBe('DIV');
    expect(pill2.getAttribute('role')).toBe('note');
    expect(pill2.textContent).toContain('2 team goals!');
    expect(pill2.getAttribute('aria-label')).toContain('The Smiths (85)');
    expect(pill2.getAttribute('aria-label')).toContain('The Garcias (80)');
  });

  it('renders expandable popover for multiple reached goals in full mode and closes on Escape and outside click', async () => {
    const user = userEvent.setup();
    render(
      <div>
        <button type="button">Outside</button>
        <FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[GOAL_SMITHS, GOAL_GARCIAS]} />
      </div>,
    );

    const btn = screen.getByTestId('farm-goal-decoration');
    expect(btn.tagName).toBe('BUTTON');
    expect(btn.getAttribute('aria-expanded')).toBe('false');
    expect(btn.textContent).toContain('2 team goals reached!');
    expect(screen.queryByRole('list')).toBeNull();

    // Open popover
    await user.click(btn);
    expect(btn.getAttribute('aria-expanded')).toBe('true');
    const list = screen.getByRole('list');
    expect(list.id).toBe(btn.getAttribute('aria-controls'));
    expect(list.textContent).toContain('The Smiths: goal of 85 reached together!');
    expect(list.textContent).toContain('The Garcias: goal of 80 reached together!');

    // Close on Escape
    await user.keyboard('{Escape}');
    expect(btn.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('list')).toBeNull();

    // Reopen and close on outside pointerdown
    await user.click(btn);
    expect(btn.getAttribute('aria-expanded')).toBe('true');
    fireEvent.pointerDown(screen.getByRole('button', { name: 'Outside' }));
    expect(btn.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('list')).toBeNull();
  });

  it('uses friend-farm copy and ARIA label when friendFarm is true', () => {
    render(
      <FarmScene pets={[SAMPLE_PET]} goalCelebrations={[GOAL_SMITHS]} friendFarm />,
    );
    const pill = screen.getByTestId('farm-goal-decoration');
    expect(pill.getAttribute('role')).toBe('note');
    expect(pill.textContent).toContain('Team goal reached: 85!');
    expect(pill.getAttribute('aria-label')).toBe(
      'Celebration bunting: team goal of 85 reached together with The Smiths.',
    );
  });

  it('renders cleanly alongside the Christmas winter backdrop', () => {
    window.history.replaceState(null, '', '/?season=christmas');
    const { container } = render(
      <FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[GOAL_SMITHS]} />,
    );

    const scene = container.querySelector('[data-season="christmas"]');
    expect(scene).not.toBeNull();
    expect(screen.getByTestId('farm-goal-decoration')).toBeTruthy();
  });

  it('renders cleanly across all 3 skins and under prefers-reduced-motion', () => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: vi.fn((query: string) => ({
        matches: query.includes('prefers-reduced-motion'),
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    const skins: Skin[] = ['sunroom', 'blocks', 'tint'];
    for (const skin of skins) {
      const { container, unmount } = render(
        <SkinScope skin={skin} color="coral">
          <FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[GOAL_SMITHS]} />
        </SkinScope>,
      );
      expect(container.querySelector(`[data-theme="${skin}"]`)).not.toBeNull();
      expect(screen.getByTestId('farm-goal-decoration').textContent).toContain('Team goal with The Smiths: 85!');
      unmount();
    }
  });

  it('maintains 1:1 EN/DE translation parity and resolves German copy cleanly', async () => {
    const enKeys = Object.keys(en.kid.farm.goal).sort();
    const deKeys = Object.keys(de.kid.farm.goal).sort();
    expect(deKeys).toEqual(enKeys);

    await i18n.changeLanguage('de');
    const { rerender } = render(
      <FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[GOAL_SMITHS]} />,
    );
    expect(screen.getByTestId('farm-goal-decoration').textContent).toContain('Teamziel mit The Smiths: 85!');
    expect(screen.getByTestId('farm-goal-decoration').getAttribute('aria-label')).toContain('Festwimpel:');

    rerender(<FarmScene pets={[SAMPLE_PET]} meId={3} goalCelebrations={[GOAL_SMITHS]} friendFarm />);
    expect(screen.getByTestId('farm-goal-decoration').textContent).toContain('Teamziel erreicht: 85!');

    rerender(<FarmScene pets={[SAMPLE_PET]} meId={3} compact goalCelebrations={[GOAL_SMITHS, GOAL_GARCIAS]} />);
    expect(screen.getByTestId('farm-goal-decoration').textContent).toContain('2 Teamziele!');
  });
});
